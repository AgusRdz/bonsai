package changelog

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleVersion = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "14", Light: "6"})
	styleSubhead = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Dark: "11", Light: "3"})
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "246", Light: "240"})
)

type model struct {
	lines  []string
	scroll int
	height int
	width  int
}

// Run opens an interactive scrollable changelog viewer.
func Run(content string) error {
	m := model{lines: renderLines(content)}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		visible := m.height - 2
		if visible < 1 {
			visible = 1
		}
		max := len(m.lines) - visible
		if max < 0 {
			max = 0
		}
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			if m.scroll < max {
				m.scroll++
			}
		case "pgup", "b":
			m.scroll -= visible
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown", " ":
			m.scroll += visible
			if m.scroll > max {
				m.scroll = max
			}
		case "g":
			m.scroll = 0
		case "G":
			m.scroll = max
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.height == 0 {
		return ""
	}
	visible := m.height - 2
	if visible < 1 {
		visible = 1
	}
	end := m.scroll + visible
	if end > len(m.lines) {
		end = len(m.lines)
	}
	content := strings.Join(m.lines[m.scroll:end], "\n")
	footer := styleDim.Render("  ↑/↓ k/j  pgup/pgdn  g top  G bottom  q quit")
	return content + "\n" + footer
}

func renderLines(content string) []string {
	var lines []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(line, "## [") {
			// Guarantee a blank line before each version section.
			if len(lines) > 0 && lines[len(lines)-1] != "" {
				lines = append(lines, "")
			}
		}
		lines = append(lines, renderLine(line))
	}
	return lines
}

func renderLine(line string) string {
	switch {
	case strings.HasPrefix(line, "# "):
		return styleVersion.Render("  " + strings.TrimPrefix(line, "# "))
	case strings.HasPrefix(line, "## ["):
		return styleVersion.Render("  " + line)
	case strings.HasPrefix(line, "### "):
		return "    " + styleSubhead.Render(strings.TrimPrefix(line, "### "))
	case strings.HasPrefix(line, "- "):
		return "      · " + strings.TrimPrefix(line, "- ")
	case strings.HasPrefix(line, "(["):
		// Commit hash link line: ([abc1234](url)) — show only the short hash.
		inner := strings.TrimPrefix(line, "([")
		if idx := strings.Index(inner, "]"); idx >= 0 {
			hash := inner[:idx]
			if len(hash) > 8 {
				hash = hash[:8]
			}
			return "        " + styleDim.Render(hash)
		}
		return "        " + styleDim.Render(line)
	case strings.TrimSpace(line) == "":
		return ""
	default:
		return "  " + styleDim.Render(line)
	}
}
