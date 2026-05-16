package tui

import (
	"fmt"
	"strings"

	"github.com/AgusRdz/bonsai/config"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	cfg    *config.Config
	state  gitState
	width  int
	height int
	ready  bool
	err    error
}

type stateMsg gitState
type errMsg struct{ err error }

func fetchState() tea.Msg {
	s, err := loadGitState()
	if err != nil {
		return errMsg{err}
	}
	return stateMsg(s)
}

func (m model) Init() tea.Cmd {
	return fetchState
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == m.cfg.Keybindings.Quit || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case stateMsg:
		m.state = gitState(msg)
		m.ready = true
	case errMsg:
		m.err = msg.err
		m.ready = true
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if !m.ready {
		return "\n  " + styleDim.Render("loading...") + "\n"
	}

	var b strings.Builder

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString("  " + styleChanged.Render("not a git repository") + "\n\n")
		b.WriteString("  " + styleDim.Render("open bonsai from inside a git repository") + "\n")
	} else {
		b.WriteString("\n")
		b.WriteString("  " + styleBranch.Render(" "+m.state.branch+" ") + "\n\n")

		if len(m.state.staged) > 0 {
			b.WriteString("  " + styleSection.Render(fmt.Sprintf("Staged (%d)", len(m.state.staged))) + "\n")
			for _, f := range m.state.staged {
				b.WriteString("    " + styleStaged.Render(f) + "\n")
			}
			b.WriteString("\n")
		}

		if len(m.state.changed) > 0 {
			b.WriteString("  " + styleSection.Render(fmt.Sprintf("Changed (%d)", len(m.state.changed))) + "\n")
			for _, f := range m.state.changed {
				b.WriteString("    " + styleChanged.Render(f) + "\n")
			}
			b.WriteString("\n")
		}

		if len(m.state.untracked) > 0 {
			b.WriteString("  " + styleSection.Render(fmt.Sprintf("Untracked (%d)", len(m.state.untracked))) + "\n")
			for _, f := range m.state.untracked {
				b.WriteString("    " + styleUntracked.Render(f) + "\n")
			}
			b.WriteString("\n")
		}

		if len(m.state.staged) == 0 && len(m.state.changed) == 0 && len(m.state.untracked) == 0 {
			b.WriteString("  " + styleDim.Render("nothing to commit, working tree clean") + "\n\n")
		}

		b.WriteString("  " + styleDim.Render("mode: "+m.cfg.Modes.Default) + "\n")
	}

	// pad remaining lines so command bar sticks to bottom
	content := b.String()
	lines := strings.Count(content, "\n")
	pad := m.height - lines - 1
	if pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	return content + m.commandBar()
}

func (m model) commandBar() string {
	kb := m.cfg.Keybindings
	parts := []string{
		fmt.Sprintf("[%s] graph", kb.Graph),
		fmt.Sprintf("[%s] commit", kb.Commit),
		fmt.Sprintf("[%s] branch", kb.Branch),
		fmt.Sprintf("[%s] push", kb.Push),
		"[?] more",
		fmt.Sprintf("[%s] quit", kb.Quit),
	}
	return styleDim.Render("  "+strings.Join(parts, "  ")) + "\n"
}

// Run starts the bonsai TUI.
func Run(cfg *config.Config) error {
	m := model{cfg: cfg}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
