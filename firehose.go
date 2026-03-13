package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcalabro/atmos/streaming"
	"github.com/jcalabro/gt"
	"github.com/urfave/cli/v3"
)

func firehoseCmd() *cli.Command {
	return &cli.Command{
		Name:  "firehose",
		Usage: "Stream live firehose events",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "url", Value: "wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos", Usage: "Subscription WebSocket URL (repos or labels)"},
			&cli.IntFlag{Name: "cursor", Value: 0, Usage: "Resume from cursor position"},
			&cli.StringFlag{Name: "collection", Usage: "Filter by collection NSID"},
			&cli.StringFlag{Name: "action", Usage: "Filter by action (create/update/delete)"},
			&cli.BoolFlag{Name: "plain", Usage: "Plain JSON lines to stdout (no TUI)"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			subURL := c.String("url")
			cursor := c.Int("cursor")
			collection := c.String("collection")
			action := c.String("action")
			plain := c.Bool("plain")
			isLabels := strings.Contains(subURL, "subscribeLabels")

			opts := streaming.Options{URL: subURL}
			if cursor > 0 {
				opts.Cursor = gt.Some(int64(cursor))
			}

			// Auto-enable plain mode when stdout is not a terminal (e.g. piped).
			if !plain {
				if f, ok := c.Root().Writer.(*os.File); ok {
					stat, err := f.Stat()
					if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
						plain = true
					}
				}
			}

			if plain {
				if isLabels {
					return labelPlain(ctx, opts, c)
				}
				return firehosePlain(ctx, opts, collection, action, c)
			}

			if isLabels {
				return labelTUI(ctx, opts)
			}
			return firehoseTUI(ctx, opts, collection, action)
		},
	}
}

func firehosePlain(ctx context.Context, opts streaming.Options, collection, action string, c *cli.Command) error {
	client, err := streaming.NewClient(opts)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	enc := json.NewEncoder(c.Root().Writer)

	for evt, err := range client.Events(ctx) {
		if err != nil {
			return err
		}

		for op, err := range evt.Operations() {
			if err != nil {
				continue
			}
			if collection != "" && op.Collection != collection {
				continue
			}
			if action != "" && string(op.Action) != action {
				continue
			}

			record := map[string]any{
				"seq":        evt.Seq,
				"action":     string(op.Action),
				"collection": op.Collection,
				"repo":       op.Repo,
				"rkey":       op.RKey,
			}
			if err := enc.Encode(record); err != nil {
				return err
			}
		}
	}
	return nil
}

func labelPlain(ctx context.Context, opts streaming.Options, c *cli.Command) error {
	client, err := streaming.NewClient(opts)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	enc := json.NewEncoder(c.Root().Writer)

	for evt, err := range client.Events(ctx) {
		if err != nil {
			return err
		}

		for _, label := range evt.Labels() {
			record := map[string]any{
				"seq": evt.Seq,
				"src": label.Src,
				"uri": label.URI,
				"val": label.Val,
				"neg": label.Neg.ValOr(false),
				"cts": label.Cts,
			}
			if err := enc.Encode(record); err != nil {
				return err
			}
		}
	}
	return nil
}

func labelTUI(ctx context.Context, opts streaming.Options) error {
	m := newFirehoseModel(opts, "", "")
	m.isLabels = true
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		client, err := streaming.NewClient(opts)
		if err != nil {
			p.Send(tea.Quit())
			return
		}
		defer func() { _ = client.Close() }()

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		const flushInterval = 50 * time.Millisecond
		var batch []firehoseOp
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()

		flush := func() {
			if len(batch) > 0 {
				p.Send(firehoseBatchMsg(batch))
				batch = nil
			}
		}

		for evt, err := range client.Events(subCtx) {
			if err != nil {
				flush()
				p.Send(firehoseErrMsg{err: err})
				return
			}

			for _, label := range evt.Labels() {
				batch = append(batch, firehoseOp{
					seq:     evt.Seq,
					isLabel: true,
					src:     label.Src,
					uri:     label.URI,
					val:     label.Val,
					neg:     label.Neg.ValOr(false),
					cts:     label.Cts,
				})
			}

			select {
			case <-ticker.C:
				flush()
			default:
			}
		}
		flush()
	}()

	_, err := p.Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}
	return err
}

func firehoseTUI(ctx context.Context, opts streaming.Options, collection, action string) error {
	m := newFirehoseModel(opts, collection, action)
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Start streaming in background, batching ops to avoid flooding
	// the bubbletea message queue (which would starve key input).
	go func() {
		client, err := streaming.NewClient(opts)
		if err != nil {
			p.Send(tea.Quit())
			return
		}
		defer func() { _ = client.Close() }()

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		const flushInterval = 50 * time.Millisecond
		var batch []firehoseOp
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()

		flush := func() {
			if len(batch) > 0 {
				p.Send(firehoseBatchMsg(batch))
				batch = nil
			}
		}

		for evt, err := range client.Events(subCtx) {
			if err != nil {
				flush()
				p.Send(firehoseErrMsg{err: err})
				return
			}

			for op, err := range evt.Operations() {
				if err != nil {
					continue
				}
				if collection != "" && op.Collection != collection {
					continue
				}
				if action != "" && string(op.Action) != action {
					continue
				}
				batch = append(batch, firehoseOp{
					seq:        evt.Seq,
					action:     string(op.Action),
					collection: op.Collection,
					repo:       op.Repo,
					rkey:       op.RKey,
				})
			}

			// Flush if ticker has fired (non-blocking check).
			select {
			case <-ticker.C:
				flush()
			default:
			}
		}
		flush()
	}()

	_, err := p.Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}
	return err
}

// TUI model

const maxLines = 1000

type firehoseOp struct {
	seq        int64
	action     string
	collection string
	repo       string
	rkey       string

	// Label fields.
	isLabel bool
	src     string
	uri     string
	val     string
	neg     bool
	cts     string
}

// Batched message: delivers many ops at once to avoid flooding the bubbletea
// message queue and starving key input.
type firehoseBatchMsg []firehoseOp

type firehoseErrMsg struct {
	err error
}

type tickMsg time.Time

type firehoseModel struct {
	opts       streaming.Options
	collection string
	action     string
	isLabels   bool

	viewport viewport.Model
	lines    []string
	total    int64
	rate     float64
	cursor   int64
	startAt  time.Time
	lastTick time.Time
	tickEvts int64
	paused   bool
	connErr  error
	width    int
	height   int
	ready    bool
}

func newFirehoseModel(opts streaming.Options, collection, action string) *firehoseModel {
	return &firehoseModel{
		opts:       opts,
		collection: collection,
		action:     action,
		startAt:    time.Now(),
		lastTick:   time.Now(),
	}
}

func (m *firehoseModel) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *firehoseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p", " ":
			m.paused = !m.paused
			if !m.paused {
				m.viewport.GotoBottom()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 3
		footerHeight := 2
		contentHeight := m.height - headerHeight - footerHeight
		contentHeight = max(contentHeight, 1)

		if !m.ready {
			m.viewport = viewport.New(m.width, contentHeight)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = contentHeight
		}
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		if !m.paused {
			m.viewport.GotoBottom()
		}

	case firehoseBatchMsg:
		// Always count for stats, but only buffer lines when not paused.
		for _, op := range msg {
			m.total++
			m.tickEvts++
			m.cursor = op.seq
			if !m.paused {
				m.lines = append(m.lines, formatOp(op))
			}
		}
		if !m.paused {
			if len(m.lines) > maxLines {
				m.lines = m.lines[len(m.lines)-maxLines:]
			}
			if m.ready {
				m.viewport.SetContent(strings.Join(m.lines, "\n"))
				m.viewport.GotoBottom()
			}
		}

	case firehoseErrMsg:
		m.connErr = msg.err

	case tickMsg:
		elapsed := time.Since(m.lastTick).Seconds()
		if elapsed > 0 {
			m.rate = float64(m.tickEvts) / elapsed
		}
		m.tickEvts = 0
		m.lastTick = time.Now()
		cmds = append(cmds, tickCmd())
	}

	// Only forward key messages to viewport when paused (for scrolling).
	if km, ok := msg.(tea.KeyMsg); ok {
		if m.paused {
			switch km.String() {
			case "p", " ", "q", "ctrl+c":
				// already handled above
			default:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func formatOp(op firehoseOp) string {
	if op.isLabel {
		record := map[string]any{
			"seq": op.seq,
			"src": op.src,
			"uri": op.uri,
			"val": op.val,
			"neg": op.neg,
			"cts": op.cts,
		}
		raw, err := json.Marshal(record)
		if err != nil {
			return fmt.Sprintf("{\"error\":%q}", err.Error())
		}
		if op.neg {
			return styleDelete.Render(string(raw))
		}
		return styleLabel.Render(string(raw))
	}

	record := map[string]any{
		"seq":        op.seq,
		"action":     op.action,
		"collection": op.collection,
		"repo":       op.repo,
		"rkey":       op.rkey,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Sprintf("{\"error\":%q}", err.Error())
	}
	return actionStyle(op.action).Render(string(raw))
}

func (m *firehoseModel) View() string {
	if !m.ready {
		return "initializing..."
	}

	uptime := time.Since(m.startAt).Truncate(time.Second)

	status := "● Connected"
	statusStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	if m.connErr != nil {
		status = "✗ Error"
		statusStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	}
	if m.paused {
		status = "⏸ Paused"
		statusStyle = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	}

	header := styleStatusBar.Width(m.width).Render(
		fmt.Sprintf("%s  │  %.0f evt/s  │  %s total  │  %s",
			statusStyle.Render(status),
			m.rate,
			formatCount(m.total),
			uptime,
		),
	)

	pauseLabel := "pause"
	if m.paused {
		pauseLabel = "resume"
	}
	footer := styleHelp.Render(fmt.Sprintf("  q: quit  p/space: %s  ↑↓: scroll", pauseLabel))

	return header + "\n" + m.viewport.View() + "\n" + footer
}

func formatCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
