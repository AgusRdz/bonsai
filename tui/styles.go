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

	// styleWarn is amber — a lesser warning than styleChanged's red.
	// Used for the "⚠ old" stash tag (age-based, not staleness).
	styleWarn = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "11", Light: "3"})

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

	styleHash = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "12", Light: "4"})

	styleAdded = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "10", Light: "2"})

	styleMerged = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "141", Light: "5"})

	styleSynced = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "10", Light: "2"})

	styleConflict = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "9", Light: "1"})

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Dark: "14", Light: "6"})

	// Intra-line diff: background highlight for the specific characters that
	// changed within a removed or added line.
	styleRemovedHL = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Dark: "88", Light: "224"}).
			Foreground(lipgloss.AdaptiveColor{Dark: "255", Light: "0"})

	styleAddedHL = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Dark: "22", Light: "194"}).
			Foreground(lipgloss.AdaptiveColor{Dark: "255", Light: "0"})
)
