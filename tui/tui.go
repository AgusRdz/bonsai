package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/conventions"
	"github.com/AgusRdz/bonsai/git"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const gitTimeout = 5 * time.Second
const pushTimeout = 60 * time.Second

type panel int

const (
	panelMain panel = iota
	panelCommit
	panelEducation
	panelBranch
	panelConvention
)

type branchMode int

const (
	branchModeCreate branchMode = iota
	branchModeRename
)

type fileItem struct {
	entry    git.FileEntry
	category int // catStaged | catChanged | catUntracked
}

const (
	catStaged    = 0
	catChanged   = 1
	catUntracked = 2
)

type model struct {
	cfg            *config.Config
	git            *git.Runner
	status         *git.Status
	files          []fileItem // flat list of all selectable files
	cursor         int
	panel          panel
	commitMsg      textinput.Model
	branchInput    textinput.Model
	branchMode     branchMode
	convViolation  *conventions.Result
	convPanelShown bool // panel already shown for current branch violation
	edu            *educationPanel
	eduTimer       int
	width          int
	height         int
	ready          bool
	err            error  // startup/refresh error
	actionErr      error  // error from last action
	lastCmd        string // last git command run
	pushing        bool
}

// --- messages ---

type statusMsg *git.Status
type errMsg struct{ err error }
type actionDoneMsg struct {
	cmd string
	err error
}

// --- commands ---

func (m model) fetchStatus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		s, err := m.git.Status(ctx)
		if err != nil {
			return errMsg{err}
		}
		return statusMsg(s)
	}
}

func (m model) doAdd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Add(ctx, path)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doRestore(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Restore(ctx, path)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doCommit(msg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Commit(ctx, msg)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doPush() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.Push(ctx)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doCreateBranch(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CreateBranch(ctx, name)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doRename(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Rename(ctx, name)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

// --- init ---

func (m model) Init() tea.Cmd {
	return tea.Batch(m.fetchStatus(), textinput.Blink)
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case statusMsg:
		prevBranch := ""
		if m.status != nil {
			prevBranch = m.status.Branch
		}
		m.status = msg
		m.files = buildFileList(msg)
		if m.cursor >= len(m.files) {
			m.cursor = max(0, len(m.files)-1)
		}
		m.ready = true
		m.err = nil
		if msg.Branch != prevBranch {
			m.convPanelShown = false
		}
		if !m.convPanelShown && m.cfg.Conventions.Validation.Mode != "off" && len(m.cfg.Conventions.Branches) > 0 {
			result := conventions.Validate(msg.Branch, m.cfg.Conventions)
			if !result.Valid {
				m.convViolation = &result
				m.convPanelShown = true
				m.panel = panelConvention
			} else {
				m.convViolation = nil
			}
		}

	case errMsg:
		m.err = msg.err
		m.ready = true

	case actionDoneMsg:
		m.pushing = false
		m.lastCmd = msg.cmd
		m.actionErr = msg.err
		dur := m.cfg.Education.PanelDuration
		if m.cfg.Modes.Default != "pro" && dur > 0 {
			m.edu = newEduPanel(msg.cmd, msg.err)
			m.eduTimer = dur
			m.panel = panelEducation
			return m, tea.Batch(m.fetchStatus(), startEduTimer())
		}
		return m, m.fetchStatus()

	case eduTickMsg:
		if m.panel == panelEducation {
			m.eduTimer--
			if m.eduTimer <= 0 {
				m.panel = panelMain
				m.edu = nil
				return m, nil
			}
			return m, startEduTimer()
		}

	case tea.KeyMsg:
		if m.panel == panelEducation {
			return m.updateEduPanel(msg)
		}
		if m.panel == panelCommit {
			return m.updateCommitPanel(msg)
		}
		if m.panel == panelBranch {
			return m.updateBranchPanel(msg)
		}
		if m.panel == panelConvention {
			return m.updateConventionPanel(msg)
		}
		return m.updateMainPanel(msg)
	}

	if m.panel == panelCommit {
		var cmd tea.Cmd
		m.commitMsg, cmd = m.commitMsg.Update(msg)
		return m, cmd
	}
	if m.panel == panelBranch {
		var cmd tea.Cmd
		m.branchInput, cmd = m.branchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateMainPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.cfg.Keybindings
	switch msg.String() {
	case kb.Quit, "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}

	case " ", "enter":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category == catStaged {
			return m, m.doRestore(f.entry.Path)
		}
		return m, m.doAdd(f.entry.Path)

	case kb.Commit, "c":
		if m.status == nil || len(m.status.Staged) == 0 {
			m.actionErr = fmt.Errorf("nothing staged - use space to stage files first")
			break
		}
		ti := textinput.New()
		ti.Placeholder = "commit message"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = m.width - 6
		m.commitMsg = ti
		m.panel = panelCommit
		m.actionErr = nil

	case kb.Push, "p":
		if m.pushing {
			break
		}
		m.pushing = true
		m.actionErr = nil
		return m, m.doPush()

	case "b":
		ti := textinput.New()
		ti.Placeholder = "branch name"
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.branchInput = ti
		m.branchMode = branchModeCreate
		m.panel = panelBranch
		m.actionErr = nil
	}

	return m, nil
}

func (m model) updateCommitPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		message := strings.TrimSpace(m.commitMsg.Value())
		if message == "" {
			m.actionErr = fmt.Errorf("commit message cannot be empty")
			m.panel = panelMain
			return m, nil
		}
		m.panel = panelMain
		return m, m.doCommit(message)

	case "esc":
		m.panel = panelMain
		return m, nil
	}

	var cmd tea.Cmd
	m.commitMsg, cmd = m.commitMsg.Update(msg)
	return m, cmd
}

func (m model) updateEduPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
		m.edu = nil
	case "c":
		if m.edu != nil {
			// clipboard.WriteAll is a best-effort operation; ignore errors
			_ = writeClipboard(m.edu.cmd)
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateBranchPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.branchInput.Value())
		if name == "" {
			m.actionErr = fmt.Errorf("branch name cannot be empty")
			return m, nil
		}
		result := conventions.Validate(name, m.cfg.Conventions)
		if !result.Valid && m.cfg.Conventions.Validation.Mode == "strict" {
			m.actionErr = fmt.Errorf("branch name does not follow conventions (strict mode)")
			return m, nil
		}
		m.panel = panelMain
		m.actionErr = nil
		if m.branchMode == branchModeRename {
			return m, m.doRename(name)
		}
		return m, m.doCreateBranch(name)

	case "esc":
		m.panel = panelMain
		m.actionErr = nil
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m model) updateConventionPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		ti := textinput.New()
		ti.Placeholder = "new branch name"
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.branchInput = ti
		m.branchMode = branchModeRename
		m.panel = panelBranch
		m.actionErr = nil
	case "enter", "esc", m.cfg.Keybindings.Quit:
		m.convViolation = nil
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// --- view ---

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	if !m.ready {
		return "\n  " + styleDim.Render("loading...") + "\n"
	}

	if m.panel == panelEducation {
		return m.eduView()
	}
	if m.panel == panelCommit {
		return m.commitView()
	}
	if m.panel == panelBranch {
		return m.branchView()
	}
	if m.panel == panelConvention {
		return m.conventionView()
	}
	return m.mainView()
}

func (m model) mainView() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString("  " + styleChanged.Render("git error") + "  " + styleDim.Render(m.err.Error()) + "\n\n")
		b.WriteString("  " + styleDim.Render("open bonsai from inside a git repository") + "\n")
	} else {
		b.WriteString("  " + styleBranch.Render(" "+m.status.Branch+" ") + "\n\n")

		m.renderSection(&b, "Staged", m.status.Staged, catStaged, styleStaged)
		m.renderSection(&b, "Changed", m.status.Changed, catChanged, styleChanged)
		m.renderSection(&b, "Untracked", m.status.Untracked, catUntracked, styleUntracked)

		if len(m.files) == 0 {
			b.WriteString("  " + styleDim.Render("nothing to commit, working tree clean") + "\n")
		}
		b.WriteString("\n")
	}

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n")
	} else if m.lastCmd != "" {
		b.WriteString("  " + styleDim.Render("$ "+m.lastCmd) + "\n")
	} else if m.convViolation != nil && m.cfg.Conventions.Validation.Mode == "warn" {
		b.WriteString("  " + styleChanged.Render("! "+m.convViolation.Branch+" does not follow conventions") + "\n")
	} else {
		b.WriteString("  " + styleDim.Render("mode: "+m.cfg.Modes.Default) + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + m.commandBar()
}

func (m model) renderSection(b *strings.Builder, title string, entries []git.FileEntry, cat int, style lipgloss.Style) {
	if len(entries) == 0 {
		return
	}

	// track the flat cursor offset for this category
	offset := 0
	for _, f := range m.files {
		if f.category == cat {
			break
		}
		offset++
	}

	b.WriteString("  " + styleSection.Render(fmt.Sprintf("%s (%d)", title, len(entries))) + "\n")
	for i, f := range entries {
		cursor := "  "
		if m.cursor == offset+i {
			cursor = styleSelected.Render("> ")
		}
		b.WriteString(cursor + "  " + style.Render(fileCode(f, cat)+"  "+f.Path) + "\n")
	}
	b.WriteString("\n")
}

func (m model) commitView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Commit") + "\n\n")
	b.WriteString("  " + m.commitMsg.View() + "\n\n")
	b.WriteString("  " + styleDim.Render("staged files that will be committed:") + "\n")
	for _, f := range m.status.Staged {
		b.WriteString("    " + styleStaged.Render(string(f.StagedCode())+"  "+f.Path) + "\n")
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] commit  [esc] cancel") + "\n"
}

func (m model) eduView() string {
	if m.edu == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")

	icon := styleStaged.Render("✓")
	titleStyle := styleStaged
	if !m.edu.success {
		icon = styleChanged.Render("✗")
		titleStyle = styleChanged
	}
	b.WriteString("  " + icon + "  " + titleStyle.Render(m.edu.title) + "\n\n")
	b.WriteString("  " + styleCmd.Render("$ "+m.edu.cmd) + "\n\n")

	if m.edu.explain != "" {
		b.WriteString("  " + m.edu.explain + "\n\n")
	}

	divider := strings.Repeat("-", min(m.width-4, 48))
	b.WriteString("  " + styleDim.Render(divider) + "\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	bar := fmt.Sprintf("  [enter] close  [c] copy command  (closes in %ds)", m.eduTimer)
	return content + styleDim.Render(bar) + "\n"
}

func (m model) branchView() string {
	var b strings.Builder
	b.WriteString("\n")

	title := "Create Branch"
	if m.branchMode == branchModeRename {
		title = "Rename Branch"
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")
	b.WriteString("  " + m.branchInput.View() + "\n\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	name := strings.TrimSpace(m.branchInput.Value())
	if name != "" && m.cfg.Conventions.Validation.Mode != "off" && len(m.cfg.Conventions.Branches) > 0 {
		result := conventions.Validate(name, m.cfg.Conventions)
		if result.Valid {
			b.WriteString("  " + styleStaged.Render("✓ valid") + "\n\n")
		} else if m.cfg.Conventions.Validation.Mode == "strict" {
			b.WriteString("  " + styleChanged.Render("✗ does not follow conventions (strict mode)") + "\n\n")
		} else {
			b.WriteString("  " + styleChanged.Render("! does not follow conventions") + "\n\n")
		}
	}

	rules := conventions.Rules(m.cfg.Conventions)
	if len(rules) > 0 {
		b.WriteString("  " + styleDim.Render("configured patterns:") + "\n")
		for _, r := range rules {
			hint := r.Rule.Prefix
			if r.Rule.Example != "" {
				hint += "  (e.g. " + r.Rule.Example + ")"
			}
			b.WriteString("  " + styleDim.Render("  "+r.Name+": "+hint) + "\n")
		}
		b.WriteString("\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] confirm  [esc] cancel") + "\n"
}

func (m model) conventionView() string {
	if m.convViolation == nil {
		return ""
	}
	v := m.convViolation
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleChanged.Render("! branch convention violation") + "\n\n")
	b.WriteString("  " + styleDim.Render("current branch: ") + styleChanged.Render(v.Branch) + "\n")
	b.WriteString("  " + styleDim.Render("this branch does not match any configured naming convention") + "\n\n")

	if len(v.Rules) > 0 {
		b.WriteString("  " + styleSection.Render("Expected patterns") + "\n")
		for _, r := range v.Rules {
			line := r.Name + ": " + r.Rule.Prefix
			if r.Rule.Example != "" {
				line += "  (e.g. " + r.Rule.Example + ")"
			}
			b.WriteString("  " + styleDim.Render("  "+line) + "\n")
		}
		b.WriteString("\n")
	}

	if m.cfg.Conventions.Validation.Mode == "strict" {
		b.WriteString("  " + styleChanged.Render("strict mode: rename this branch before creating branches or commits") + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [r] rename branch  [enter] dismiss") + "\n"
}

func (m model) commandBar() string {
	if m.pushing {
		return styleDim.Render("  pushing...") + "\n"
	}
	kb := m.cfg.Keybindings
	parts := []string{
		fmt.Sprintf("[%s] commit", kb.Commit),
		fmt.Sprintf("[%s] push", kb.Push),
		"[b] branch",
		"[space] stage/unstage",
		"[↑↓] navigate",
		fmt.Sprintf("[%s] quit", kb.Quit),
	}
	return styleDim.Render("  "+strings.Join(parts, "  ")) + "\n"
}

// --- helpers ---

// fileCode returns the display character for a file based on its category.
// Staged files show the index code; changed files show the working-tree code.
func fileCode(f git.FileEntry, cat int) string {
	if cat == catChanged {
		return string(f.UnstagedCode())
	}
	return string(f.StagedCode())
}

func buildFileList(s *git.Status) []fileItem {
	var items []fileItem
	for _, f := range s.Staged {
		items = append(items, fileItem{entry: f, category: catStaged})
	}
	for _, f := range s.Changed {
		items = append(items, fileItem{entry: f, category: catChanged})
	}
	for _, f := range s.Untracked {
		items = append(items, fileItem{entry: f, category: catUntracked})
	}
	return items
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// writeClipboard is isolated to make it easy to stub in tests and to contain
// the platform-specific clipboard dependency.
func writeClipboard(s string) error {
	return clipboard.WriteAll(s)
}

// Run starts the bonsai TUI.
func Run(cfg *config.Config) error {
	g := git.New()
	m := model{cfg: cfg, git: g}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
