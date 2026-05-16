package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleBranch = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2"))

	styleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))

	styleStaged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	styleChanged = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	styleUntracked = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))

	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)
