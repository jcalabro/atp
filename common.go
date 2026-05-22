package main

import (
	"os"
	"strconv"
	"strings"
)

// Minimal ANSI styling — replaces our previous lipgloss dependency.
//
// Only what we use: bold + a single SGR foreground color, with automatic
// no-op rendering when stdout isn't a TTY or NO_COLOR is set
// (https://no-color.org). Output is plain text in those cases so piping
// to a file or a downstream tool does the right thing without flags.

// useColor is decided once at startup. Cheap to read, branchless at call
// sites, and matches what users expect: colors interactively, plain text
// when redirected.
var useColor = func() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}()

type style struct {
	fg   int // SGR foreground code; 0 means "no foreground"
	bold bool
}

// Render wraps s in this style's ANSI escapes. When color is disabled,
// returns s unchanged so output stays clean for pipes and logs.
func (s style) Render(text string) string {
	if !useColor || (s.fg == 0 && !s.bold) {
		return text
	}
	var b strings.Builder
	b.WriteString("\x1b[")
	first := true
	if s.bold {
		b.WriteString("1")
		first = false
	}
	if s.fg != 0 {
		if !first {
			b.WriteByte(';')
		}
		b.WriteString("38;5;")
		b.WriteString(strconv.Itoa(s.fg))
	}
	b.WriteByte('m')
	b.WriteString(text)
	b.WriteString("\x1b[0m")
	return b.String()
}

// 256-color SGR indices; chosen to match the prior lipgloss palette.
const (
	colorGreen  = 10
	colorYellow = 11
	colorRed    = 9
	colorCyan   = 14
	colorGray   = 8
)

var (
	styleCreate = style{fg: colorGreen, bold: true}
	styleUpdate = style{fg: colorYellow, bold: true}
	styleDelete = style{fg: colorGray, bold: true}

	styleLabel = style{fg: colorCyan, bold: true}
	styleDim   = style{fg: colorGray}
	styleError = style{fg: colorRed, bold: true}
)

func actionStyle(action string) style {
	switch action {
	case "create":
		return styleCreate
	case "update":
		return styleUpdate
	case "delete":
		return styleDelete
	default:
		return styleDim
	}
}
