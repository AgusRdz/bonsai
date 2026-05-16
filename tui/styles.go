package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleBranch = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "10", Light: "2"})

	styleSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "14", Light: "6"})

	styleStaged = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "10", Light: "2"})

	styleChanged = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "9", Light: "1"})

	styleUntracked = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "11", Light: "3"})

	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "13", Light: "5"})

	// styleDim uses a mid-gray visible on both dark and light terminals.
	// ANSI "8" (bright-black) is too close to black on many dark themes.
	styleDim = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "246", Light: "240"})

	styleCmd = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "12", Light: "4"})
)
