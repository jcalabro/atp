package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jcalabro/atmos/streaming"
	"github.com/jcalabro/gt"
	"github.com/urfave/cli/v3"
)

func subscribeCmd() *cli.Command {
	return &cli.Command{
		Name:  "subscribe",
		Usage: "Stream live subscription events as JSON lines",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "url", Value: "wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos", Usage: "Subscription WebSocket URL (repos or labels)"},
			&cli.IntFlag{Name: "cursor", Value: 0, Usage: "Resume from cursor position"},
			&cli.StringFlag{Name: "collection", Usage: "Filter by collection NSID"},
			&cli.StringFlag{Name: "action", Usage: "Filter by action (create/update/delete)"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			opts := streaming.Options{
				URL: c.String("url"),
			}

			if cursor := c.Int("cursor"); cursor > 0 {
				opts.Cursor = gt.Some(int64(cursor))
			}

			client, err := streaming.NewClient(opts)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			w := c.Root().Writer
			if strings.Contains(opts.URL, "subscribeLabels") {
				return streamLabels(ctx, client, w)
			}
			return streamRepos(ctx, client, w, c.String("collection"), c.String("action"))
		},
	}
}

// streamRepos prints one JSON line per repo operation. Lines are colored
// by action on a TTY and emitted plain when stdout is redirected.
func streamRepos(ctx context.Context, client *streaming.Client, w io.Writer, collection, action string) error {
	for batch, err := range client.Events(ctx) {
		if err != nil {
			return err
		}

		for _, evt := range batch {
			for op, opErr := range evt.Operations() {
				if opErr != nil {
					continue
				}

				actStr := string(op.Action)
				if action != "" && actStr != action {
					continue
				}

				line, err := json.Marshal(map[string]any{
					"seq":        evt.Seq,
					"action":     actStr,
					"collection": string(op.Collection),
					"repo":       string(op.Repo),
					"rkey":       string(op.RKey),
				})
				if err != nil {
					return err
				}

				if _, err := fmt.Fprintln(w, actionStyle(actStr).Render(string(line))); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// streamLabels prints one JSON line per label, with negation labels (which
// revoke a previous label) colored as deletes and additions as labels.
func streamLabels(ctx context.Context, client *streaming.Client, w io.Writer) error {
	for batch, err := range client.Events(ctx) {
		if err != nil {
			return err
		}

		for _, evt := range batch {
			for _, label := range evt.Labels() {
				neg := label.Neg.ValOr(false)
				line, err := json.Marshal(map[string]any{
					"seq": evt.Seq,
					"src": label.Src,
					"uri": label.URI,
					"val": label.Val,
					"neg": neg,
					"cts": label.Cts,
				})
				if err != nil {
					return err
				}

				s := styleLabel
				if neg {
					s = styleDelete
				}

				if _, err := fmt.Fprintln(w, s.Render(string(line))); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
