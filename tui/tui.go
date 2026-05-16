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
	panelCommitDetail
	panelConflict
	panelTagList
	panelTagCreate
	panelResetPick
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
	catConflict  = -1 // shown first; not stageable until resolved
	catStaged    = 0
	catChanged   = 1
	catUntracked = 2
)

type model struct {
	cfg                *config.Config
	git                *git.Runner
	status             *git.Status
	files              []fileItem // flat list of all selectable files
	cursor             int
	panel              panel
	commitMsg          textinput.Model
	branchInput        textinput.Model
	branchMode         branchMode
	convViolation      *conventions.Result
	convPanelShown     bool // panel already shown for current branch violation
	logEntries         []git.LogEntry
	logCursor          int
	logOffset          int             // pagination: how many commits already loaded
	logHasMore         bool            // more commits available to load
	logFilter          string          // active filter query; empty = no filter
	logFilterInput     textinput.Model // search input field
	logFiltering       bool            // search input is focused
	branches           []git.Branch
	branchCursor       int
	diffLines          []string
	diffScroll         int
	diffTitle          string
	stashes            []git.StashEntry
	stashCursor        int
	confirmPrompt      string
	confirmCmd         tea.Cmd
	flowOptions        []flowOption
	flowPickCursor     int
	commitDetail       *git.CommitDetail
	commitDetailScroll int
	conflictPath       string
	conflictLines      []string
	conflictScroll     int
	tags               []git.TagEntry
	tagCursor          int
	edu                *educationPanel
	eduTimer           int
	width              int
	height             int
	ready              bool
	err                error  // startup/refresh error
	actionErr          error  // error from last action
	lastCmd            string // last git command run
	pushing            bool
	pulling            bool
}

// --- messages ---

type statusMsg *git.Status
type errMsg struct{ err error }
type actionDoneMsg struct {
	cmd string
	err error
}
type branchListMsg []git.Branch

type diffMsg struct {
	title string
	lines []string
}
type stashListMsg []git.StashEntry
type commitDetailMsg *git.CommitDetail

type logPageMsg struct {
	entries []git.LogEntry
	hasMore bool
	append  bool // true = append to existing list; false = replace
}
type conflictLinesMsg struct {
	path  string
	lines []string
}

type tagListMsg []git.TagEntry

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

const logPageSize = 100

func (m model) doFetchLog() tea.Cmd {
	return m.doFetchLogPage(0, false)
}

// doFetchLogPage fetches one page of commits. skip is the pagination offset;
// appendResults=true merges results into the existing list instead of replacing.
func (m model) doFetchLogPage(skip int, appendResults bool) tea.Cmd {
	filter := m.logFilter
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		opts := parseLogFilter(filter)
		opts.MaxCount = logPageSize + 1 // fetch one extra to detect hasMore
		opts.Skip = skip
		entries, err := m.git.LogOpts(ctx, opts)
		if err != nil || entries == nil {
			return logPageMsg{entries: []git.LogEntry{}, hasMore: false, append: appendResults}
		}
		hasMore := len(entries) > logPageSize
		if hasMore {
			entries = entries[:logPageSize]
		}
		return logPageMsg{entries: entries, hasMore: hasMore, append: appendResults}
	}
}

// parseLogFilter converts a user-typed filter string into LogOptions.
// Supported prefixes: author:, since: / after:, until: / before:
// Anything else is treated as a commit message grep.
func parseLogFilter(q string) git.LogOptions {
	q = strings.TrimSpace(q)
	switch {
	case strings.HasPrefix(q, "author:"):
		return git.LogOptions{Author: strings.TrimSpace(q[7:])}
	case strings.HasPrefix(q, "since:"), strings.HasPrefix(q, "after:"):
		v := q[strings.Index(q, ":")+1:]
		return git.LogOptions{Since: strings.TrimSpace(v)}
	case strings.HasPrefix(q, "until:"), strings.HasPrefix(q, "before:"):
		v := q[strings.Index(q, ":")+1:]
		return git.LogOptions{Until: strings.TrimSpace(v)}
	default:
		return git.LogOptions{Grep: q}
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

func (m model) doFetchCommitDetail(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		detail, err := m.git.ShowStat(ctx, hash)
		if err != nil {
			return commitDetailMsg(&git.CommitDetail{Hash: hash})
		}
		return commitDetailMsg(detail)
	}
}

func (m model) doReadConflict(path string) tea.Cmd {
	return func() tea.Msg {
		lines, err := m.git.ConflictLines(path)
		if err != nil {
			return conflictLinesMsg{path: path, lines: []string{"error reading file: " + err.Error()}}
		}
		return conflictLinesMsg{path: path, lines: lines}
	}
}

func (m model) doAcceptOurs(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AcceptOurs(ctx, path)
		return actionDoneMsg{cmd: "git checkout --ours -- " + path, err: err}
	}
}

func (m model) doAcceptTheirs(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AcceptTheirs(ctx, path)
		return actionDoneMsg{cmd: "git checkout --theirs -- " + path, err: err}
	}
}

func (m model) doRemoveConflict(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoveConflict(ctx, path)
		return actionDoneMsg{cmd: "git rm -- " + path, err: err}
	}
}

func (m model) doFetchTags() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		tags, err := m.git.Tags(ctx)
		if err != nil || tags == nil {
			return tagListMsg([]git.TagEntry{})
		}
		return tagListMsg(tags)
	}
}

func (m model) doCreateTag(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CreateTag(ctx, name)
		return actionDoneMsg{cmd: "git tag " + name, err: err}
	}
}

func (m model) doDeleteTag(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DeleteTag(ctx, name)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doMerge(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Merge(ctx, branch)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doCherryPick(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CherryPick(ctx, hash)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doReset(mode string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Reset(ctx, mode)
		return actionDoneMsg{cmd: "git reset --" + mode + " HEAD~1", err: err}
	}
}

func (m model) doRebase(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Rebase(ctx, branch)
		return actionDoneMsg{cmd: "git rebase " + branch, err: err}
	}
}

func (m model) doRebaseContinue() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RebaseContinue(ctx)
		return actionDoneMsg{cmd: "git rebase --continue", err: err}
	}
}

func (m model) doRebaseAbort() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RebaseAbort(ctx)
		return actionDoneMsg{cmd: "git rebase --abort", err: err}
	}
}

func (m model) doMergeAbort() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.MergeAbort(ctx)
		return actionDoneMsg{cmd: "git merge --abort", err: err}
	}
}

func (m model) doCherryPickAbort() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CherryPickAbort(ctx)
		return actionDoneMsg{cmd: "git cherry-pick --abort", err: err}
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

	case logPageMsg:
		if msg.append {
			m.logEntries = append(m.logEntries, msg.entries...)
		} else {
			m.logEntries = msg.entries
			m.logCursor = 0
			m.logOffset = 0
		}
		if !msg.append {
			m.logOffset = len(msg.entries)
		} else {
			m.logOffset += len(msg.entries)
		}
		m.logHasMore = msg.hasMore
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

	case commitDetailMsg:
		m.commitDetail = (*git.CommitDetail)(msg)
		m.commitDetailScroll = 0
		m.panel = panelCommitDetail

	case conflictLinesMsg:
		m.conflictPath = msg.path
		m.conflictLines = msg.lines
		m.conflictScroll = 0
		m.panel = panelConflict

	case tagListMsg:
		m.tags = []git.TagEntry(msg)
		m.tagCursor = 0
		m.panel = panelTagList

	case actionDoneMsg:
		m.pushing = false
		m.pulling = false
		m.lastCmd = msg.cmd
		m.actionErr = msg.err
		dur := m.cfg.Education.PanelDuration
		if m.cfg.Modes.Default != "pro" && dur > 0 {
			m.edu = newEduPanel(msg.cmd, msg.err)
			if m.cfg.Modes.Default == "standard" {
				// standard mode: command confirmation only, no explanation text.
				m.edu.explain = ""
			} else if hint := flowHint(msg.cmd, detectFlow(m.cfg)); hint != "" {
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
		if m.panel == panelCommitDetail {
			return m.updateCommitDetailPanel(msg)
		}
		if m.panel == panelConflict {
			return m.updateConflictPanel(msg)
		}
		if m.panel == panelResetPick {
			return m.updateResetPickPanel(msg)
		}
		if m.panel == panelTagList {
			return m.updateTagListPanel(msg)
		}
		if m.panel == panelTagCreate {
			return m.updateTagCreatePanel(msg)
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
	if m.panel == panelTagCreate {
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
		if f.category == catConflict {
			m.actionErr = fmt.Errorf("resolve this conflict first - press [d] to view and resolve")
			break
		}
		if f.category == catStaged {
			return m, m.doRestore(f.entry.Path)
		}
		return m, m.doAdd(f.entry.Path)

	case kb.Commit, "c":
		if m.status != nil && m.status.MergeState == "rebase" {
			if len(m.status.Conflicts) > 0 {
				m.actionErr = fmt.Errorf("resolve all conflicts first, then press [c] to continue the rebase")
				break
			}
			return m, m.doRebaseContinue()
		}
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
		if f.category == catConflict {
			m.conflictLines = nil
			m.conflictScroll = 0
			m.conflictPath = f.entry.Path
			return m, m.doReadConflict(f.entry.Path)
		}
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

	case kb.Undo, "z":
		m.panel = panelResetPick
		m.actionErr = nil

	case "t":
		m.tags = nil
		m.tagCursor = 0
		m.panel = panelTagList
		return m, m.doFetchTags()

	case "a":
		if m.status == nil || m.status.MergeState == "" {
			break
		}
		switch m.status.MergeState {
		case "rebase":
			m.confirmPrompt = "abort rebase? your branch will be restored to its state before the rebase"
			m.confirmCmd = m.doRebaseAbort()
		case "merge":
			m.confirmPrompt = "abort merge? all in-progress merge changes will be discarded"
			m.confirmCmd = m.doMergeAbort()
		case "cherry-pick":
			m.confirmPrompt = "abort cherry-pick? the operation will be cancelled"
			m.confirmCmd = m.doCherryPickAbort()
		}
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
	// When the filter input is focused, route most keys to it.
	if m.logFiltering {
		switch msg.String() {
		case "enter":
			// Commit the search.
			q := strings.TrimSpace(m.logFilterInput.Value())
			m.logFilter = q
			m.logFiltering = false
			m.logFilterInput.Blur()
			m.logEntries = nil
			return m, m.doFetchLogPage(0, false)
		case "esc":
			// Cancel search, restore previous filter state.
			m.logFiltering = false
			m.logFilterInput.SetValue(m.logFilter)
			m.logFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.logFilterInput, cmd = m.logFilterInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.logCursor > 0 {
			m.logCursor--
		}
	case "down", "j":
		if m.logCursor < len(m.logEntries)-1 {
			m.logCursor++
		}
	case "enter":
		if m.logCursor < len(m.logEntries) {
			entry := m.logEntries[m.logCursor]
			if entry.Hash != "" {
				m.commitDetail = nil
				m.commitDetailScroll = 0
				return m, m.doFetchCommitDetail(entry.Hash)
			}
		}
	case "m":
		// Load more commits (pagination).
		if m.logHasMore && m.logFilter == "" {
			return m, m.doFetchLogPage(m.logOffset, true)
		}
	case "/":
		// Open filter input.
		m.logFiltering = true
		m.logFilterInput.Focus()
	case "ctrl+/", "ctrl+r":
		// Clear active filter.
		m.logFilter = ""
		m.logFilterInput.SetValue("")
		m.logEntries = nil
		return m, m.doFetchLogPage(0, false)
	case "esc":
		if m.logFilter != "" {
			// First esc clears the filter.
			m.logFilter = ""
			m.logFilterInput.SetValue("")
			m.logEntries = nil
			return m, m.doFetchLogPage(0, false)
		}
		m.panel = panelMain
	case m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateCommitDetailPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 6
	if visibleLines < 1 {
		visibleLines = 1
	}
	var maxScroll int
	if m.commitDetail != nil {
		total := commitDetailLineCount(m.commitDetail)
		maxScroll = total - visibleLines
		if maxScroll < 0 {
			maxScroll = 0
		}
	}
	switch msg.String() {
	case "up", "k":
		if m.commitDetailScroll > 0 {
			m.commitDetailScroll--
		}
	case "down", "j":
		if m.commitDetailScroll < maxScroll {
			m.commitDetailScroll++
		}
	case "y":
		if m.commitDetail != nil {
			_ = clipboard.WriteAll(m.commitDetail.Hash)
		}
	case "p":
		if m.commitDetail != nil && m.commitDetail.Hash != "" {
			hash := m.commitDetail.Hash
			short := hash
			if len(short) > 7 {
				short = short[:7]
			}
			current := ""
			if m.status != nil {
				current = m.status.Branch
			}
			m.confirmPrompt = fmt.Sprintf("cherry-pick %s onto %s?", short, current)
			m.confirmCmd = m.doCherryPick(hash)
			m.panel = panelConfirm
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelLog
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func commitDetailLineCount(d *git.CommitDetail) int {
	if d == nil {
		return 0
	}
	n := 6 // hash + author + date + blank lines
	if d.Body != "" {
		n += strings.Count(d.Body, "\n") + 2
	}
	n += len(d.Stat) + 1
	return n
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
	case "m":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		if b.Current {
			break
		}
		current := ""
		if m.status != nil {
			current = m.status.Branch
		}
		m.confirmPrompt = fmt.Sprintf("merge %s into %s?", b.Name, current)
		m.confirmCmd = m.doMerge(b.Name)
		m.panel = panelConfirm
	case "r":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		if b.Current {
			break
		}
		current := ""
		if m.status != nil {
			current = m.status.Branch
		}
		m.confirmPrompt = fmt.Sprintf("rebase %s onto %s?", current, b.Name)
		m.confirmCmd = m.doRebase(b.Name)
		m.panel = panelConfirm
		m.actionErr = nil
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// diffViewport returns the number of visible lines and the maximum scroll
// offset for the diff panel. Both callers (updateDiffPanel and diffView) use
// this to avoid duplicating the calculation.
func (m model) diffViewport() (visibleLines, maxScroll int) {
	visibleLines = m.height - 5
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll = len(m.diffLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	return
}

func (m model) updateDiffPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, maxScroll := m.diffViewport()
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

func (m model) updateConflictPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 7
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(m.conflictLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Find the conflict code for the current file.
	code := ""
	if m.status != nil {
		for _, f := range m.status.Conflicts {
			if f.Path == m.conflictPath {
				code = f.Code
				break
			}
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.conflictScroll > 0 {
			m.conflictScroll--
		}
	case "down", "j":
		if m.conflictScroll < maxScroll {
			m.conflictScroll++
		}
	case "o":
		if code != "DD" {
			m.panel = panelMain
			return m, m.doAcceptOurs(m.conflictPath)
		}
	case "t":
		if code != "DD" {
			m.panel = panelMain
			return m, m.doAcceptTheirs(m.conflictPath)
		}
	case "r":
		if code == "DD" {
			m.panel = panelMain
			return m, m.doRemoveConflict(m.conflictPath)
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateResetPickPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		m.panel = panelMain
		return m, m.doReset("soft")
	case "m":
		m.panel = panelMain
		return m, m.doReset("mixed")
	case "h":
		m.confirmPrompt = "hard reset HEAD~1? uncommitted changes will be permanently discarded"
		m.confirmCmd = m.doReset("hard")
		m.panel = panelConfirm
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTagListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
		}
	case "down", "j":
		if m.tagCursor < len(m.tags)-1 {
			m.tagCursor++
		}
	case "n":
		ti := textinput.New()
		ti.Placeholder = "tag name"
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.branchInput = ti
		m.panel = panelTagCreate
		m.actionErr = nil
	case "d":
		if len(m.tags) == 0 {
			break
		}
		tag := m.tags[m.tagCursor]
		m.confirmPrompt = fmt.Sprintf("delete tag %s?", tag.Name)
		m.confirmCmd = m.doDeleteTag(tag.Name)
		m.panel = panelConfirm
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTagCreatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.branchInput.Value())
		if name == "" {
			m.actionErr = fmt.Errorf("tag name cannot be empty")
			return m, nil
		}
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doCreateTag(name)
	case "esc":
		m.panel = panelTagList
		m.actionErr = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
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
	if m.panel == panelCommitDetail {
		return m.commitDetailView()
	}
	if m.panel == panelConflict {
		return m.conflictView()
	}
	if m.panel == panelResetPick {
		return m.resetPickView()
	}
	if m.panel == panelTagList {
		return m.tagListView()
	}
	if m.panel == panelTagCreate {
		return m.tagCreateView()
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
		header += "  " + styleDim.Render("[mode:"+m.cfg.Modes.Default+"]")
		b.WriteString("  " + header + "\n\n")

		// Merge/cherry-pick/rebase banner.
		if m.status.MergeState != "" {
			var banner string
			switch m.status.MergeState {
			case "rebase":
				if len(m.status.Conflicts) > 0 {
					banner = fmt.Sprintf("rebase in progress - resolve %d conflict(s), then [c] to continue  [a] to abort", len(m.status.Conflicts))
				} else {
					banner = "rebase in progress - conflicts resolved, press [c] to continue  [a] to abort"
				}
			case "cherry-pick":
				if len(m.status.Conflicts) > 0 {
					banner = fmt.Sprintf("cherry-pick in progress - resolve %d conflict(s), then [c] to commit  [a] to abort", len(m.status.Conflicts))
				} else {
					banner = "cherry-pick in progress - conflicts resolved, press [c] to commit  [a] to abort"
				}
			default: // merge
				if len(m.status.Conflicts) > 0 {
					banner = fmt.Sprintf("merge in progress - resolve %d conflict(s), then [c] to commit  [a] to abort", len(m.status.Conflicts))
				} else {
					banner = "merge in progress - conflicts resolved, press [c] to commit  [a] to abort"
				}
			}
			b.WriteString("  " + styleChanged.Render(banner) + "\n\n")
		}

		m.renderConflictSection(&b)
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
		b.WriteString("  " + styleDim.Render("run 'bonsai config' to change mode or flow") + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + m.commandBar()
}

func (m model) renderConflictSection(b *strings.Builder) {
	if len(m.status.Conflicts) == 0 {
		return
	}
	offset := 0 // conflicts are always first in m.files

	b.WriteString("  " + styleChanged.Render(fmt.Sprintf("Conflicts (%d)", len(m.status.Conflicts))) + "\n")
	for i, f := range m.status.Conflicts {
		cursor := "  "
		if m.cursor == offset+i {
			cursor = styleSelected.Render("> ")
		}
		desc := git.ConflictDesc(f.Code)
		b.WriteString(cursor + "  " + styleChanged.Render("!  "+f.Path) + "  " + styleDim.Render(desc) + "\n")
	}
	b.WriteString("\n")
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
	return content + styleDim.Render("  [enter] switch  [m] merge  [r] rebase  [esc] cancel") + "\n"
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

	section("Config")
	b.WriteString("    " + styleDim.Render("bonsai config          open global config in editor") + "\n")
	b.WriteString("    " + styleDim.Render("bonsai config local    open per-project .bonsai.toml") + "\n")
	b.WriteString("    " + styleDim.Render("bonsai init            create .bonsai.toml template") + "\n")
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
		visibleLines, _ := m.diffViewport()
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

func (m model) commitDetailView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Commit Detail") + "\n\n")

	if m.commitDetail == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else {
		d := m.commitDetail
		// Build all lines into a slice so we can apply the viewport.
		var lines []string
		lines = append(lines, styleCmd.Render(d.Hash)+"  "+d.Subject)
		lines = append(lines, "")
		lines = append(lines, styleDim.Render("Author  ")+d.Author)
		lines = append(lines, styleDim.Render("Date    ")+d.Date)
		if d.Body != "" {
			lines = append(lines, "")
			for _, l := range strings.Split(d.Body, "\n") {
				lines = append(lines, styleDim.Render(l))
			}
		}
		if len(d.Stat) > 0 {
			lines = append(lines, "")
			lines = append(lines, styleSection.Render("Files changed"))
			for _, l := range d.Stat {
				lines = append(lines, renderStatLine(l))
			}
		}

		visibleLines := m.height - 6
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := m.commitDetailScroll
		end := start + visibleLines
		if end > len(lines) {
			end = len(lines)
		}
		for _, l := range lines[start:end] {
			b.WriteString("  " + l + "\n")
		}
	}

	b.WriteString("\n")
	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	bar := "  [↑↓] scroll  [esc] back  [y] copy hash  [p] cherry-pick"
	if m.commitDetail != nil {
		total := commitDetailLineCount(m.commitDetail)
		visibleLines := m.height - 6
		if visibleLines < 1 {
			visibleLines = 1
		}
		if total > visibleLines {
			bar += fmt.Sprintf("  (%d/%d)", m.commitDetailScroll+1, total)
		}
	}
	return content + styleDim.Render(bar) + "\n"
}

// renderStatLine colors the + and - bars in a diff-stat line.
func renderStatLine(line string) string {
	if !strings.Contains(line, "|") {
		// Summary line: "N files changed, M insertions(+), K deletions(-)"
		return styleDim.Render(line)
	}
	idx := strings.LastIndex(line, "|")
	name := line[:idx+1]
	bars := line[idx+1:]
	var colored string
	for _, ch := range bars {
		switch ch {
		case '+':
			colored += styleStaged.Render("+")
		case '-':
			colored += styleChanged.Render("-")
		default:
			colored += styleDim.Render(string(ch))
		}
	}
	return styleDim.Render(name) + colored
}

func (m model) logView() string {
	var b strings.Builder
	b.WriteString("\n")

	// Header with active filter badge.
	title := "Recent Commits"
	if m.logFilter != "" {
		title += "  " + styleCmd.Render("["+m.logFilter+"]")
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	// Filter input (shown when [/] is pressed).
	if m.logFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.logFilterInput.View() + "\n\n")
	}

	if m.logEntries == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.logEntries) == 0 {
		if m.logFilter != "" {
			b.WriteString("  " + styleDim.Render("no commits matched - press esc to clear filter") + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("no commits yet") + "\n")
		}
	} else {
		// Overhead: blank + title + optional filter input + blank footer lines.
		overhead := 6
		if m.logFiltering {
			overhead += 2
		}
		visibleLines := m.height - overhead
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := 0
		if m.logCursor >= visibleLines {
			start = m.logCursor - visibleLines + 1
		}
		end := start + visibleLines
		if end > len(m.logEntries) {
			end = len(m.logEntries)
		}
		for i := start; i < end; i++ {
			e := m.logEntries[i]
			if m.logCursor == i {
				b.WriteString("  " + styleSelected.Render(">") + " " + styleDim.Render(e.Line) + "\n")
			} else {
				b.WriteString("    " + styleDim.Render(e.Line) + "\n")
			}
		}
		if m.logHasMore && m.logFilter == "" {
			b.WriteString("    " + styleDim.Render("--- press [m] to load more ---") + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lineCount := strings.Count(content, "\n")
	if pad := m.height - lineCount - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	pos := ""
	if len(m.logEntries) > 0 {
		pos = fmt.Sprintf("  (%d", m.logCursor+1)
		if m.logHasMore {
			pos += fmt.Sprintf("/%d+", len(m.logEntries))
		} else {
			pos += fmt.Sprintf("/%d", len(m.logEntries))
		}
		pos += ")"
	}
	hint := "  [↑↓] scroll  [/] search  [enter] detail  [esc] back" + pos
	return content + styleDim.Render(hint) + "\n"
}

func (m model) conflictView() string {
	var b strings.Builder
	b.WriteString("\n")

	path := m.conflictPath
	code := ""
	if m.status != nil {
		for _, f := range m.status.Conflicts {
			if f.Path == path {
				code = f.Code
				break
			}
		}
	}

	desc := git.ConflictDesc(code)
	b.WriteString("  " + styleSection.Render("Conflict") + "  " + styleChanged.Render(path) + "  " + styleDim.Render(desc) + "\n\n")

	if code == "DD" {
		b.WriteString("  " + styleDim.Render("both sides deleted this file") + "\n")
		b.WriteString("  " + styleDim.Render("press [r] to accept the deletion and remove it from the index") + "\n")
	} else if m.conflictLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.conflictLines) == 0 {
		b.WriteString("  " + styleDim.Render("(empty file)") + "\n")
	} else {
		// Precompute the kind for each line:
		// 0=context, 1=ours marker, 2=ours content, 3=separator, 4=theirs content, 5=theirs marker
		kind := make([]int, len(m.conflictLines))
		state := 0 // 0=context, 2=in-ours, 4=in-theirs
		for i, line := range m.conflictLines {
			switch {
			case strings.HasPrefix(line, "<<<<<<<"):
				kind[i] = 1
				state = 2
			case line == "=======" && state == 2:
				kind[i] = 3
				state = 4
			case strings.HasPrefix(line, ">>>>>>>") && state == 4:
				kind[i] = 5
				state = 0
			case state == 2:
				kind[i] = 2
			case state == 4:
				kind[i] = 4
			default:
				kind[i] = 0
			}
		}

		visibleLines := m.height - 7
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := m.conflictScroll
		end := start + visibleLines
		if end > len(m.conflictLines) {
			end = len(m.conflictLines)
		}
		for i := start; i < end; i++ {
			line := m.conflictLines[i]
			var rendered string
			switch kind[i] {
			case 1:
				rendered = "  " + styleStaged.Render("<<<<<<< YOUR CHANGES")
			case 2:
				rendered = "  " + styleStaged.Render(line)
			case 3:
				rendered = "  " + styleDim.Render("======= (above: yours / below: incoming)")
			case 4:
				rendered = "  " + styleChanged.Render(line)
			case 5:
				rendered = "  " + styleChanged.Render(">>>>>>> INCOMING CHANGES")
			default:
				rendered = "  " + styleDim.Render(line)
			}
			b.WriteString(rendered + "\n")
		}
	}

	b.WriteString("\n")
	content := b.String()
	lineCount := strings.Count(content, "\n")
	if pad := m.height - lineCount - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	var bar string
	if code == "DD" {
		bar = "  [r] remove file  [esc] back"
	} else {
		bar = "  [o] keep ours (green)  [t] keep theirs (red)  [↑↓] scroll  [esc] back"
		visibleLines := m.height - 7
		if visibleLines < 1 {
			visibleLines = 1
		}
		if len(m.conflictLines) > visibleLines {
			bar += fmt.Sprintf("  (%d/%d)", m.conflictScroll+1, len(m.conflictLines))
		}
	}
	return content + styleDim.Render(bar) + "\n"
}

func (m model) resetPickView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Reset (undo last commit)") + "\n\n")
	b.WriteString("  " + styleCmd.Render("[s]") + "  " + styleDim.Render("soft  - commit removed, changes stay staged") + "\n")
	b.WriteString("  " + styleCmd.Render("[m]") + "  " + styleDim.Render("mixed - commit removed, changes stay unstaged") + "\n")
	b.WriteString("  " + styleCmd.Render("[h]") + "  " + styleChanged.Render("hard  - commit removed, changes permanently discarded") + "\n")
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [s] soft  [m] mixed  [h] hard  [esc] cancel") + "\n"
}

func (m model) tagListView() string {
	var b strings.Builder
	b.WriteString("\n")

	title := "Tags"
	if len(m.tags) > 0 {
		title = fmt.Sprintf("Tags (%d)", len(m.tags))
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.tags == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.tags) == 0 {
		b.WriteString("  " + styleDim.Render("no tags found") + "\n")
	} else {
		for i, tag := range m.tags {
			cursor := "  "
			if m.tagCursor == i {
				cursor = styleSelected.Render("> ")
			}
			b.WriteString(cursor + "  " + styleCmd.Render(tag.Name) + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [n] new tag  [d] delete  [esc] back") + "\n"
}

func (m model) tagCreateView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Create Tag") + "\n\n")
	b.WriteString("  " + m.branchInput.View() + "\n\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] create  [esc] cancel") + "\n"
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
		"[l] log",
		"[z] reset",
		"[t] tags",
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
	// Conflicts come first - they block commit and must be resolved.
	for _, f := range s.Conflicts {
		items = append(items, fileItem{entry: f, category: catConflict})
	}
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
	fi := textinput.New()
	fi.Placeholder = "message text  |  author:name  |  since:2026-01-01  |  until:2026-03-01"
	fi.CharLimit = 120
	m := model{cfg: cfg, git: g, logFilterInput: fi}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
