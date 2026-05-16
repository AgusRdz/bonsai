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
	panelLog
	panelBranchList
	panelDiff
	panelStashList
	panelHelp
	panelConfirm
	panelFlowPick
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
	logEntries     []git.LogEntry
	logCursor      int
	branches       []git.Branch
	branchCursor   int
	diffLines      []string
	diffScroll     int
	diffTitle      string
	stashes        []git.StashEntry
	stashCursor    int
	confirmPrompt  string
	confirmCmd     tea.Cmd
	flowOptions    []flowOption
	flowPickCursor int
	edu            *educationPanel
	eduTimer       int
	width          int
	height         int
	ready          bool
	err            error  // startup/refresh error
	actionErr      error  // error from last action
	lastCmd        string // last git command run
	pushing        bool
	pulling        bool
}

// --- messages ---

type statusMsg *git.Status
type errMsg struct{ err error }
type actionDoneMsg struct {
	cmd string
	err error
}
type logMsg []git.LogEntry
type branchListMsg []git.Branch

type diffMsg struct {
	title string
	lines []string
}
type stashListMsg []git.StashEntry

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

func (m model) doPull() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.Pull(ctx)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doFetchLog() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.Log(ctx, 20)
		if err != nil || entries == nil {
			return logMsg([]git.LogEntry{})
		}
		return logMsg(entries)
	}
}

func (m model) doSwitch(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Switch(ctx, name)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doFetchBranches() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		branches, err := m.git.Branches(ctx)
		if err != nil || branches == nil {
			return branchListMsg([]git.Branch{})
		}
		return branchListMsg(branches)
	}
}

func (m model) doFetchDiff(path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		title := path
		if staged {
			title += "  (staged)"
		}
		content, err := m.git.Diff(ctx, path, staged)
		if err != nil || content == "" {
			return diffMsg{title: title, lines: []string{}}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: title, lines: lines}
	}
}

func (m model) doStash() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Stash(ctx)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doStashPop(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashPop(ctx, ref)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doDiscard(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Discard(ctx, path)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doFetchStashList() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.StashList(ctx)
		if err != nil {
			return stashListMsg([]git.StashEntry{})
		}
		return stashListMsg(entries)
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

	case logMsg:
		m.logEntries = []git.LogEntry(msg)
		m.logCursor = 0
		m.panel = panelLog

	case branchListMsg:
		m.branches = []git.Branch(msg)
		m.branchCursor = 0
		for i, b := range m.branches {
			if b.Current {
				m.branchCursor = i
				break
			}
		}
		m.panel = panelBranchList

	case diffMsg:
		m.diffLines = msg.lines
		m.diffTitle = msg.title

	case stashListMsg:
		m.stashes = []git.StashEntry(msg)
		m.stashCursor = 0
		m.panel = panelStashList

	case actionDoneMsg:
		m.pushing = false
		m.pulling = false
		m.lastCmd = msg.cmd
		m.actionErr = msg.err
		dur := m.cfg.Education.PanelDuration
		if m.cfg.Modes.Default != "pro" && dur > 0 {
			m.edu = newEduPanel(msg.cmd, msg.err)
			if hint := flowHint(msg.cmd, detectFlow(m.cfg)); hint != "" {
				if m.edu.explain != "" {
					m.edu.explain += "\n\n" + hint
				} else {
					m.edu.explain = hint
				}
			}
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
		if m.panel == panelLog {
			return m.updateLogPanel(msg)
		}
		if m.panel == panelBranchList {
			return m.updateBranchListPanel(msg)
		}
		if m.panel == panelDiff {
			return m.updateDiffPanel(msg)
		}
		if m.panel == panelStashList {
			return m.updateStashListPanel(msg)
		}
		if m.panel == panelHelp {
			return m.updateHelpPanel(msg)
		}
		if m.panel == panelConfirm {
			return m.updateConfirmPanel(msg)
		}
		if m.panel == panelFlowPick {
			return m.updateFlowPickPanel(msg)
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

	case "P":
		if m.pulling || m.pushing {
			break
		}
		m.pulling = true
		m.actionErr = nil
		return m, m.doPull()

	case kb.Branch, "b":
		if detectFlow(m.cfg) == "gitflow" {
			m.flowOptions = gitflowOptions(m.cfg)
			m.flowPickCursor = 0
			m.panel = panelFlowPick
			m.actionErr = nil
			break
		}
		ti := textinput.New()
		ti.Placeholder = "branch name"
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.branchInput = ti
		m.branchMode = branchModeCreate
		m.panel = panelBranch
		m.actionErr = nil

	case "l":
		m.logEntries = nil
		m.logCursor = 0
		m.panel = panelLog
		return m, m.doFetchLog()

	case "B":
		m.branches = nil
		m.branchCursor = 0
		m.panel = panelBranchList
		return m, m.doFetchBranches()

	case "d":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category == catUntracked {
			m.actionErr = fmt.Errorf("untracked file has no diff - stage it first")
			break
		}
		m.diffLines = nil
		m.diffScroll = 0
		m.panel = panelDiff
		return m, m.doFetchDiff(f.entry.Path, f.category == catStaged)

	case "s":
		if len(m.files) == 0 {
			m.actionErr = fmt.Errorf("nothing to stash - working tree is clean")
			break
		}
		m.actionErr = nil
		return m, m.doStash()

	case "S":
		m.stashes = nil
		m.stashCursor = 0
		m.panel = panelStashList
		return m, m.doFetchStashList()

	case "?":
		m.panel = panelHelp

	case "x":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category != catChanged {
			m.actionErr = fmt.Errorf("only changed (unstaged) files can be discarded")
			break
		}
		m.confirmPrompt = fmt.Sprintf("discard all changes to %s? this cannot be undone", f.entry.Path)
		m.confirmCmd = m.doDiscard(f.entry.Path)
		m.panel = panelConfirm
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

func (m model) updateLogPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.logCursor > 0 {
			m.logCursor--
		}
	case "down", "j":
		if m.logCursor < len(m.logEntries)-1 {
			m.logCursor++
		}
	case "esc", "enter", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateBranchListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
		}
	case "down", "j":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
		}
	case "enter":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		if b.Current {
			m.panel = panelMain
			return m, nil
		}
		m.panel = panelMain
		return m, m.doSwitch(b.Name)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateDiffPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 5
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(m.diffLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch msg.String() {
	case "up", "k":
		if m.diffScroll > 0 {
			m.diffScroll--
		}
	case "down", "j":
		if m.diffScroll < maxScroll {
			m.diffScroll++
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateStashListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.stashCursor > 0 {
			m.stashCursor--
		}
	case "down", "j":
		if m.stashCursor < len(m.stashes)-1 {
			m.stashCursor++
		}
	case "enter":
		if len(m.stashes) == 0 {
			break
		}
		ref := m.stashes[m.stashCursor].Ref
		m.panel = panelMain
		return m, m.doStashPop(ref)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateFlowPickPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.flowPickCursor > 0 {
			m.flowPickCursor--
		}
	case "down", "j":
		if m.flowPickCursor < len(m.flowOptions)-1 {
			m.flowPickCursor++
		}
	case "enter":
		if len(m.flowOptions) == 0 {
			m.panel = panelMain
			return m, nil
		}
		opt := m.flowOptions[m.flowPickCursor]
		ti := textinput.New()
		ti.Placeholder = opt.example
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		ti.SetValue(opt.prefix)
		m.branchInput = ti
		m.branchMode = branchModeCreate
		m.panel = panelBranch
		m.actionErr = nil
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfirmPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		cmd := m.confirmCmd
		m.confirmCmd = nil
		m.panel = panelMain
		return m, cmd
	case "n", "N", "esc", m.cfg.Keybindings.Quit:
		m.confirmCmd = nil
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateHelpPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.panel = panelMain
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
	if m.panel == panelLog {
		return m.logView()
	}
	if m.panel == panelBranchList {
		return m.branchListView()
	}
	if m.panel == panelDiff {
		return m.diffView()
	}
	if m.panel == panelStashList {
		return m.stashListView()
	}
	if m.panel == panelHelp {
		return m.helpView()
	}
	if m.panel == panelConfirm {
		return m.confirmView()
	}
	if m.panel == panelFlowPick {
		return m.flowPickView()
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
		header := styleBranch.Render(" " + m.status.Branch + " ")
		if m.status.Ahead > 0 {
			header += "  " + styleDim.Render(fmt.Sprintf("↑%d", m.status.Ahead))
		}
		if m.status.Behind > 0 {
			header += "  " + styleChanged.Render(fmt.Sprintf("↓%d", m.status.Behind))
		}
		if flow := detectFlow(m.cfg); flow != "auto" {
			header += "  " + styleDim.Render("["+flow+"]")
		}
		b.WriteString("  " + header + "\n\n")

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
	} else if m.cfg.Modes.Default != "pro" {
		b.WriteString("  " + styleDim.Render(contextTip(m)) + "\n")
	} else {
		b.WriteString("  " + styleDim.Render("mode: pro") + "\n")
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

func (m model) branchListView() string {
	var b strings.Builder
	b.WriteString("\n")

	title := "Branches"
	if len(m.branches) > 0 {
		title = fmt.Sprintf("Branches (%d)", len(m.branches))
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.branches == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.branches) == 0 {
		b.WriteString("  " + styleDim.Render("no branches found") + "\n")
	} else {
		for i, br := range m.branches {
			cursor := "  "
			if m.branchCursor == i {
				cursor = styleSelected.Render("> ")
			}
			var name string
			if br.Current {
				name = styleBranch.Render(" "+br.Name+" ") + "  " + styleDim.Render("current")
			} else {
				name = styleDim.Render(br.Name)
			}
			b.WriteString(cursor + "  " + name + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] switch  [esc] cancel") + "\n"
}

func (m model) confirmView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleChanged.Render("! confirm") + "\n\n")
	b.WriteString("  " + styleDim.Render(m.confirmPrompt) + "\n\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [y] yes  [n / esc] cancel") + "\n"
}

func (m model) helpView() string {
	kb := m.cfg.Keybindings
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Keybindings") + "\n\n")

	section := func(title string) {
		b.WriteString("  " + styleDim.Render(title) + "\n")
	}
	row := func(key, desc string) {
		b.WriteString("    " + styleCmd.Render(fmt.Sprintf("%-14s", key)) + styleDim.Render(desc) + "\n")
	}

	section("Files")
	row("↑↓ / k/j", "navigate file list")
	row("space / enter", "stage or unstage selected file")
	row("d", "diff selected file (staged or unstaged)")
	row("x", "discard working tree changes (with confirmation)")
	b.WriteString("\n")

	section("Git")
	row(kb.Commit+" / c", "open commit panel")
	row(kb.Push+" / p", "push to remote")
	row("P", "pull from remote")
	row("s", "stash all changes")
	row("S", "view stash list and pop")
	b.WriteString("\n")

	section("Branches")
	row("b", "create new branch")
	row("B", "switch to another branch")
	row("l", "commit log (recent 20)")
	b.WriteString("\n")

	section("App")
	row("?", "this help panel")
	row(kb.Quit+" / ctrl+c", "quit")
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  press any key to close") + "\n"
}

func (m model) stashListView() string {
	var b strings.Builder
	b.WriteString("\n")

	title := "Stashes"
	if len(m.stashes) > 0 {
		title = fmt.Sprintf("Stashes (%d)", len(m.stashes))
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.stashes == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.stashes) == 0 {
		b.WriteString("  " + styleDim.Render("no stashes") + "\n")
	} else {
		for i, st := range m.stashes {
			cursor := "  "
			if m.stashCursor == i {
				cursor = styleSelected.Render("> ")
			}
			ref := styleCmd.Render(st.Ref)
			desc := styleDim.Render(st.Description)
			b.WriteString(cursor + "  " + ref + "  " + desc + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] pop  [esc] cancel") + "\n"
}

func (m model) diffView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Diff") + "  " + styleDim.Render(m.diffTitle) + "\n\n")

	if m.diffLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.diffLines) == 0 {
		b.WriteString("  " + styleDim.Render("no changes") + "\n")
	} else {
		visibleLines := m.height - 5
		if visibleLines < 1 {
			visibleLines = 1
		}
		end := m.diffScroll + visibleLines
		if end > len(m.diffLines) {
			end = len(m.diffLines)
		}
		for _, line := range m.diffLines[m.diffScroll:end] {
			b.WriteString(renderDiffLine(line) + "\n")
		}
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	pos := ""
	if len(m.diffLines) > 0 {
		pos = fmt.Sprintf("  (%d/%d)", m.diffScroll+1, len(m.diffLines))
	}
	return content + styleDim.Render("  [↑↓] scroll  [esc] back"+pos) + "\n"
}

func renderDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		return "  " + styleCmd.Render(line)
	case strings.HasPrefix(line, "+"):
		return "  " + styleStaged.Render(line)
	case strings.HasPrefix(line, "-"):
		return "  " + styleChanged.Render(line)
	default:
		return "  " + styleDim.Render(line)
	}
}

func (m model) flowPickView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Branch Type") + "\n\n")
	for i, opt := range m.flowOptions {
		cursor := "  "
		if m.flowPickCursor == i {
			cursor = styleSelected.Render("> ")
		}
		name := styleCmd.Render(fmt.Sprintf("%-10s", opt.name))
		prefix := styleDim.Render(opt.prefix)
		example := styleDim.Render("  e.g. " + opt.example)
		b.WriteString(cursor + "  " + name + "  " + prefix + example + "\n")
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] select  [esc] cancel") + "\n"
}

func (m model) logView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Recent Commits") + "\n\n")

	if m.logEntries == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.logEntries) == 0 {
		b.WriteString("  " + styleDim.Render("no commits yet") + "\n")
	} else {
		for i, e := range m.logEntries {
			if m.logCursor == i {
				b.WriteString("  " + styleSelected.Render(">") + " " + styleDim.Render(e.Line) + "\n")
			} else {
				b.WriteString("    " + styleDim.Render(e.Line) + "\n")
			}
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [↑↓] scroll  [esc] back") + "\n"
}

func contextTip(m model) string {
	if m.status == nil {
		return ""
	}
	s := m.status
	flow := detectFlow(m.cfg)
	nChanged := len(s.Changed) + len(s.Untracked)
	nStaged := len(s.Staged)
	switch {
	case s.Behind > 0:
		return fmt.Sprintf("tip: %d commit(s) available on remote - press [P] to pull", s.Behind)
	case nChanged > 0 && nStaged == 0:
		return fmt.Sprintf("tip: %d file(s) changed - navigate and press [space] to stage", nChanged)
	case nChanged > 0 && nStaged > 0:
		return fmt.Sprintf("tip: %d staged, %d unstaged - press [c] to commit or keep staging", nStaged, nChanged)
	case nStaged > 0:
		return fmt.Sprintf("tip: %d file(s) staged - press [c] to commit", nStaged)
	case s.Ahead > 0:
		switch flow {
		case "gitflow":
			return fmt.Sprintf("tip: %d commit(s) ready - push [p] and open a PR targeting develop", s.Ahead)
		case "trunk":
			return fmt.Sprintf("tip: %d commit(s) ahead - push and merge soon, keep branches short-lived", s.Ahead)
		case "githubflow", "forking":
			return fmt.Sprintf("tip: %d commit(s) ready - push [p] then open a PR to merge into main", s.Ahead)
		default:
			return fmt.Sprintf("tip: %d commit(s) ready - press [p] to push to remote", s.Ahead)
		}
	default:
		return "tip: working tree is clean - edit a file to get started"
	}
}

func (m model) commandBar() string {
	if m.pushing {
		return styleDim.Render("  pushing...") + "\n"
	}
	if m.pulling {
		return styleDim.Render("  pulling...") + "\n"
	}
	kb := m.cfg.Keybindings
	parts := []string{
		"[space] stage/unstage",
		fmt.Sprintf("[%s] commit", kb.Commit),
		fmt.Sprintf("[%s] push", kb.Push),
		"[P] pull",
		"[b/B] branch",
		"[?] help",
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
