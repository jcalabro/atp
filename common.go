package main

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("10")
	colorYellow = lipgloss.Color("11")
	colorRed    = lipgloss.Color("9")
	colorCyan   = lipgloss.Color("14")
	colorGray   = lipgloss.Color("8")
	colorWhite  = lipgloss.Color("15")

	styleCreate = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleUpdate = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	styleDelete = lipgloss.NewStyle().Foreground(colorRed).Bold(true)

	styleLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleDim   = lipgloss.NewStyle().Foreground(colorGray)
	styleError = lipgloss.NewStyle().Foreground(colorRed).Bold(true)

	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(colorWhite).
			Padding(0, 1)

	styleHelp = lipgloss.NewStyle().Foreground(colorGray)
)

func actionStyle(action string) lipgloss.Style {
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
