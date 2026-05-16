package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/conventions"
	"github.com/AgusRdz/bonsai/git"
	"github.com/AgusRdz/bonsai/usage"
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
	panelWorktreeList
	panelWorktreeAdd
	panelBlame
	panelBisect
	panelRebaseInteractive
	panelAmend
	panelConfigMenu
	panelConfigFile
	panelConfigRecommend
	panelConfigProfiles
	panelFetch
	panelRestore
	panelReflog
	panelRemoteList
	panelRemoteAdd
	panelRemoteRename
	panelSubmoduleList
	panelSubmoduleAdd
	panelNoteView
	panelHunkStage
	panelPushOpts
	panelMastery
	panelStashMsg
	panelFileHistory
	panelGraph
	panelEduMgr
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

type rebaseTodo struct {
	action string // "pick", "reword", "edit", "squash", "fixup", "drop"
	hash   string // abbreviated commit hash
	msg    string // commit subject
}

type configSection int

const (
	configSectionGlobal configSection = iota
	configSectionLocal
	configSectionGlobalIgnore
	configSectionLocalIgnore
)

type configRecommend struct {
	key       string // git config key
	value     string // recommended value
	desc      string // short description
	reasoning string // why this is recommended
	applied   bool   // whether already set to this value
}

type configProfile struct {
	gitdir string // e.g. ~/work/
	path   string // path to include config
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
	worktrees          []git.WorktreeEntry
	worktreeCursor     int
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
	blameLines         []git.BlameLine
	blameScroll        int
	blameTitle         string
	bisectState        *git.BisectState
	bisectLog          string
	bisectInput        textinput.Model
	bisectInputActive  bool
	rebaseTodos        []rebaseTodo
	rebaseCursor       int
	rebaseBase         string          // e.g. "HEAD~3"
	rebaseBaseInput    textinput.Model // input for entering the base ref
	rebaseStep         int             // 0 = enter base ref, 1 = edit todo list
	amendInput         textinput.Model
	amendField         int               // 0=menu, 1=message, 2=author, 3=date
	amendDetail        *git.CommitDetail // HEAD commit shown in the panel

	configMenuCursor      int
	configSection         configSection
	configFileLines       []string
	configFileScroll      int
	configFilePath        string
	configEntries         []git.ConfigEntry
	configRecommendations []configRecommend
	configRecommendCursor int
	configProfiles        []configProfile
	configProfileCursor   int
	configProfileInput    textinput.Model
	configProfileStep     int
	configProfileNewPath  string

	// fetch
	fetchCursor int

	// restore
	restoreFile  string
	restoreInput textinput.Model // source ref

	// clean
	cleanFiles []string

	// reflog
	reflogEntries []git.ReflogEntry
	reflogCursor  int

	// remotes
	remotes            []git.RemoteEntry
	remoteCursor       int
	remoteAddInputs    [2]textinput.Model // [0]=name [1]=url
	remoteAddStep      int
	remoteRenameInput  textinput.Model
	remoteRenameTarget string // name of remote being renamed

	// submodules
	submodules      []git.SubmoduleEntry
	submoduleCursor int
	submoduleInputs [2]textinput.Model // [0]=url [1]=path
	submoduleStep   int

	// notes
	noteCommit  string // hash of commit whose note is being viewed/edited
	noteContent string // current note text
	noteInput   textinput.Model
	noteEditing bool

	// hunk staging
	hunkFile    string
	hunkStaged  bool
	hunkFileHdr string
	hunkList    []git.Hunk
	hunkSel     []bool
	hunkCursor  int

	// push options
	pushOptCursor int

	// usage tracking
	usage     *usage.Data
	usagePath string

	// mastery question panel
	masteryKey    string
	masteryCursor int // 0 = suppress, 1 = keep showing

	// stash with message
	stashMsgInput textinput.Model

	// file history
	fileHistoryPath    string
	fileHistoryEntries []git.LogEntry
	fileHistoryCursor  int

	// commit detail origin: which panel to return to on esc
	commitDetailOrigin panel

	// branch graph
	graphLines  []string
	graphScroll int

	// branch rename from branch list
	branchRenameTarget string

	// education manager
	eduMgrKeys   []string // ordered list of command keys shown
	eduMgrCursor int
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

type worktreeListMsg []git.WorktreeEntry

type blameMsg struct {
	title string
	lines []git.BlameLine
}

type bisectStateMsg *git.BisectState
type bisectLogMsg string

type bisectActionMsg struct {
	cmd string
	err error
}

type rebaseTodosMsg struct {
	base  string
	lines []string // raw "hash message" lines from git log, oldest-first
}

type amendDetailMsg *git.CommitDetail

type configFileMsg struct {
	section configSection
	lines   []string
	path    string
	entries []git.ConfigEntry
}
type configRecommendMsg []configRecommend
type configProfilesMsg []configProfile
type editorDoneMsg struct{ err error }

type cleanPreviewMsg []string
type reflogMsg []git.ReflogEntry
type remotesMsg []git.RemoteEntry
type submodulesMsg []git.SubmoduleEntry
type noteMsg struct {
	commit  string
	content string
}

type hunkLoadMsg struct {
	fileHdr string
	hunks   []git.Hunk
	err     error
}

type fileHistoryMsg []git.LogEntry
type graphMsg string

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

var pushMenuOptions = []struct {
	label       string
	force       bool
	setUpstream bool
}{
	{"Push", false, false},
	{"Push --force-with-lease", true, false},
	{"Push --set-upstream origin <branch>", false, true},
}

func (m model) doPushWithOpts() tea.Cmd {
	opt := pushMenuOptions[m.pushOptCursor]
	remote := "origin"
	branch := ""
	if m.status != nil {
		branch = m.status.Branch
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.PushWithOptions(ctx, opt.force, opt.setUpstream, remote, branch)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doSaveUsage() tea.Cmd {
	data := m.usage
	path := m.usagePath
	return func() tea.Msg {
		_ = data.Save(path)
		return nil
	}
}

func (m model) doStashWithMsg(msg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashWithMessage(ctx, msg)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doFileHistory(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.FileLog(ctx, path, 200)
		if err != nil {
			return fileHistoryMsg(nil)
		}
		return fileHistoryMsg(entries)
	}
}

func (m model) doGraph() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		out, err := m.git.Graph(ctx, 300)
		if err != nil {
			return graphMsg("")
		}
		return graphMsg(out)
	}
}

func (m model) doStashApply(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashApply(ctx, ref)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doStashDrop(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashDrop(ctx, ref)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doDeleteBranch(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DeleteBranch(ctx, name, false)
		if err != nil {
			// if safe delete fails because branch is unmerged, suggest -D
			return actionDoneMsg{cmd: "git branch -d " + name,
				err: fmt.Errorf("%w (branch has unmerged work - use force delete)", err)}
		}
		return actionDoneMsg{cmd: "git branch -d " + name, err: nil}
	}
}

func (m model) doDeleteRemoteBranch(remote, branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DeleteRemoteBranch(ctx, remote, branch)
		return actionDoneMsg{cmd: "git push " + remote + " --delete " + branch, err: err}
	}
}

func (m model) doRenameBranch(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RenameBranch(ctx, oldName, newName)
		return actionDoneMsg{cmd: "git branch -m " + oldName + " " + newName, err: err}
	}
}

func (m model) doPushTag(remote, tag string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.PushTag(ctx, remote, tag)
		return actionDoneMsg{cmd: "git push " + remote + " " + tag, err: err}
	}
}

func (m model) doFetchDiffCommit(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		content, err := m.git.DiffCommit(ctx, hash)
		if err != nil || content == "" {
			return diffMsg{title: "diff " + hash, lines: []string{}}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: "diff " + hash, lines: lines}
	}
}

func (m model) doLoadHunks(path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		hdr, hunks, err := m.git.DiffHunks(ctx, path, staged)
		return hunkLoadMsg{fileHdr: hdr, hunks: hunks, err: err}
	}
}

func (m model) doApplyHunks() tea.Cmd {
	hdr := m.hunkFileHdr
	reverse := m.hunkStaged
	var selected []git.Hunk
	for i, h := range m.hunkList {
		if i < len(m.hunkSel) && m.hunkSel[i] {
			selected = append(selected, h)
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.ApplyHunks(ctx, hdr, selected, reverse)
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

func (m model) doFetchWorktrees() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.Worktrees(ctx)
		if err != nil || entries == nil {
			return worktreeListMsg([]git.WorktreeEntry{})
		}
		return worktreeListMsg(entries)
	}
}

func (m model) doAddWorktree(path, branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AddWorktree(ctx, path, branch)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doRemoveWorktree(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoveWorktree(ctx, path)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doBlame(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		lines, err := m.git.Blame(ctx, path)
		if err != nil || lines == nil {
			return blameMsg{title: path, lines: []git.BlameLine{}}
		}
		return blameMsg{title: path, lines: lines}
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

func (m model) doFetchBisectState() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		state, err := m.git.BisectStatus(ctx)
		if err != nil {
			return bisectStateMsg(&git.BisectState{})
		}
		return bisectStateMsg(state)
	}
}

func (m model) doFetchBisectLog() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		log, err := m.git.BisectLog(ctx)
		if err != nil {
			return bisectLogMsg("")
		}
		return bisectLogMsg(log)
	}
}

func (m model) doBisectStart() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.BisectStart(ctx)
		return bisectActionMsg{cmd: "git bisect start", err: err}
	}
}

func (m model) doBisectBad() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.BisectBad(ctx)
		return bisectActionMsg{cmd: "git bisect bad", err: err}
	}
}

func (m model) doBisectGood(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.BisectGood(ctx, hash)
		cmd := "git bisect good"
		if hash != "" {
			cmd += " " + hash
		}
		return bisectActionMsg{cmd: cmd, err: err}
	}
}

func (m model) doBisectReset() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.BisectReset(ctx)
		return bisectActionMsg{cmd: "git bisect reset", err: err}
	}
}

func (m model) doFetchRebaseTodos(base string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		lines, err := m.git.RebaseInteractiveCommits(ctx, base)
		if err != nil {
			return rebaseTodosMsg{base: base, lines: nil}
		}
		return rebaseTodosMsg{base: base, lines: lines}
	}
}

func (m model) doRebaseInteractive() tea.Cmd {
	todos := m.rebaseTodos
	base := m.rebaseBase
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		todoLines := make([]string, len(todos))
		for i, todo := range todos {
			todoLines[i] = fmt.Sprintf("%s %s %s", todo.action, todo.hash, todo.msg)
		}
		err := m.git.RebaseInteractive(ctx, base, todoLines)
		return actionDoneMsg{cmd: "git rebase -i " + base, err: err}
	}
}

func (m model) doFetchAmendDetail() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		d, err := m.git.ShowStat(ctx, "HEAD")
		if err != nil {
			return amendDetailMsg(&git.CommitDetail{})
		}
		return amendDetailMsg(d)
	}
}

func (m model) doAmendMessage(msg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendMessage(ctx, msg)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doAmendAuthor(author string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendAuthor(ctx, author)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doAmendDate(date string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendDate(ctx, date)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doAmendNoEdit() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendNoEdit(ctx)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doLoadConfigSection(sec configSection) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()

		var lines []string
		var path string
		var entries []git.ConfigEntry

		switch sec {
		case configSectionGlobal:
			var err error
			entries, err = m.git.GlobalConfigList(ctx)
			if err != nil {
				entries = nil
			}
			path, _ = m.git.GlobalConfigRawPath()
			if data, err := os.ReadFile(path); err == nil {
				lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			}

		case configSectionLocal:
			var err error
			entries, err = m.git.LocalConfigList(ctx)
			if err != nil {
				entries = nil
			}
			path = m.git.LocalConfigRawPath()
			if data, err := os.ReadFile(path); err == nil {
				lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			}

		case configSectionGlobalIgnore:
			ignorePath, err := m.git.GlobalGitignorePath(ctx)
			if err != nil {
				ignorePath = ""
			}
			path = ignorePath
			if path != "" {
				if data, err := os.ReadFile(path); err == nil {
					lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
				}
			}

		case configSectionLocalIgnore:
			path = ".gitignore"
			if data, err := os.ReadFile(path); err == nil {
				lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
			}
		}

		return configFileMsg{
			section: sec,
			lines:   lines,
			path:    path,
			entries: entries,
		}
	}
}

func (m model) doOpenEditor(path string) tea.Cmd {
	editorStr := config.ResolveEditor(m.cfg)
	parts := strings.Fields(editorStr)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

func (m model) doLoadRecommendations() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()

		recs := []configRecommend{
			{
				key: "pull.rebase", value: "true",
				desc:      "pull.rebase = true",
				reasoning: "Rebases instead of merging on pull, keeping history linear.",
			},
			{
				key: "init.defaultBranch", value: "main",
				desc:      "init.defaultBranch = main",
				reasoning: "New repos start with 'main' instead of 'master'.",
			},
			{
				key: "fetch.prune", value: "true",
				desc:      "fetch.prune = true",
				reasoning: "Automatically removes remote-tracking refs for deleted branches.",
			},
			{
				key: "push.autoSetupRemote", value: "true",
				desc:      "push.autoSetupRemote = true",
				reasoning: "Sets upstream automatically on first push - no more --set-upstream.",
			},
			{
				key: "rerere.enabled", value: "true",
				desc:      "rerere.enabled = true",
				reasoning: "Remembers how you resolved conflicts and replays the resolution automatically.",
			},
			{
				key: "diff.colorMoved", value: "default",
				desc:      "diff.colorMoved = default",
				reasoning: "Highlights moved code blocks differently from added/removed lines.",
			},
			{
				key: "core.whitespace", value: "fix",
				desc:      "core.whitespace = fix",
				reasoning: "Automatically fixes trailing whitespace on commit.",
			},
			{
				key: "branch.sort", value: "-committerdate",
				desc:      "branch.sort = -committerdate",
				reasoning: "Lists branches by most recently used first.",
			},
		}

		for i, rec := range recs {
			current, err := m.git.GlobalConfigGet(ctx, rec.key)
			if err == nil && current == rec.value {
				recs[i].applied = true
			}
		}

		return configRecommendMsg(recs)
	}
}

func (m model) doApplyRecommendation(key, value string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.SetGlobalConfig(ctx, key, value)
		return actionDoneMsg{cmd: "git config --global " + key + " " + value, err: err}
	}
}

func (m model) doLoadProfiles() tea.Cmd {
	return func() tea.Msg {
		path, err := m.git.GlobalConfigRawPath()
		if err != nil {
			return configProfilesMsg(nil)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return configProfilesMsg(nil)
		}

		var profiles []configProfile
		lines := strings.Split(string(data), "\n")
		var currentGitdir string
		inIncludeIf := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[includeIf \"gitdir:") {
				rest := strings.TrimPrefix(trimmed, "[includeIf \"gitdir:")
				rest = strings.TrimSuffix(rest, "\"]")
				rest = strings.TrimSuffix(rest, "\"]")
				// strip trailing `"]`
				if idx := strings.Index(rest, "\""); idx >= 0 {
					rest = rest[:idx]
				}
				currentGitdir = rest
				inIncludeIf = true
				continue
			}
			if inIncludeIf {
				if strings.HasPrefix(trimmed, "path") {
					parts := strings.SplitN(trimmed, "=", 2)
					if len(parts) == 2 {
						includePath := strings.TrimSpace(parts[1])
						profiles = append(profiles, configProfile{gitdir: currentGitdir, path: includePath})
					}
					inIncludeIf = false
					currentGitdir = ""
				} else if strings.HasPrefix(trimmed, "[") {
					inIncludeIf = false
					currentGitdir = ""
				}
			}
		}

		return configProfilesMsg(profiles)
	}
}

func (m model) doAddProfile(gitdir, includePath string) tea.Cmd {
	return func() tea.Msg {
		path, err := m.git.GlobalConfigRawPath()
		if err != nil {
			return actionDoneMsg{cmd: "git config profile add", err: err}
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			return actionDoneMsg{cmd: "git config profile add", err: err}
		}
		defer f.Close()
		entry := fmt.Sprintf("\n[includeIf \"gitdir:%s\"]\n\tpath = %s\n", gitdir, includePath)
		_, err = f.WriteString(entry)
		return actionDoneMsg{cmd: "git config --global includeIf", err: err}
	}
}

func (m model) doFetch(all, prune bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.Fetch(ctx, all, prune)
		cmd := "git fetch"
		if all {
			cmd += " --all"
		}
		if prune {
			cmd += " --prune"
		}
		return actionDoneMsg{cmd: cmd, err: err}
	}
}

func (m model) doRestoreFile(path, source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RestoreFile(ctx, path, source, false)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doCleanPreview() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		files, err := m.git.CleanPreview(ctx)
		if err != nil {
			return cleanPreviewMsg(nil)
		}
		return cleanPreviewMsg(files)
	}
}

func (m model) doClean() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Clean(ctx)
		return actionDoneMsg{cmd: "git clean -fd", err: err}
	}
}

func (m model) doFetchReflog() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.Reflog(ctx)
		if err != nil {
			return reflogMsg(nil)
		}
		return reflogMsg(entries)
	}
}

func (m model) doFetchRemotes() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		remotes, err := m.git.Remotes(ctx)
		if err != nil {
			return remotesMsg(nil)
		}
		return remotesMsg(remotes)
	}
}

func (m model) doRemoteAdd(name, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteAdd(ctx, name, url)
		return actionDoneMsg{cmd: "git remote add " + name + " " + url, err: err}
	}
}

func (m model) doRemoteRemove(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteRemove(ctx, name)
		return actionDoneMsg{cmd: "git remote remove " + name, err: err}
	}
}

func (m model) doRemoteRename(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteRename(ctx, oldName, newName)
		return actionDoneMsg{cmd: "git remote rename " + oldName + " " + newName, err: err}
	}
}

func (m model) doFetchSubmodules() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		subs, err := m.git.Submodules(ctx)
		if err != nil {
			return submodulesMsg(nil)
		}
		return submodulesMsg(subs)
	}
}

func (m model) doSubmoduleAdd(url, path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.SubmoduleAdd(ctx, url, path)
		return actionDoneMsg{cmd: "git submodule add " + url, err: err}
	}
}

func (m model) doSubmoduleUpdate(init bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
		defer cancel()
		err := m.git.SubmoduleUpdate(ctx, init)
		cmd := "git submodule update"
		if init {
			cmd += " --init"
		}
		return actionDoneMsg{cmd: cmd, err: err}
	}
}

func (m model) doSubmoduleDeinit(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.SubmoduleDeinit(ctx, path)
		return actionDoneMsg{cmd: "git submodule deinit " + path, err: err}
	}
}

func (m model) doFetchNote(commit string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		content, err := m.git.NoteGet(ctx, commit)
		if err != nil {
			return noteMsg{commit: commit, content: ""}
		}
		return noteMsg{commit: commit, content: content}
	}
}

func (m model) doNoteAdd(commit, message string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.NoteAdd(ctx, commit, message)
		return actionDoneMsg{cmd: "git notes add -m " + commit, err: err}
	}
}

func (m model) doNoteRemove(commit string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.NoteRemove(ctx, commit)
		return actionDoneMsg{cmd: "git notes remove " + commit, err: err}
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

	case worktreeListMsg:
		m.worktrees = []git.WorktreeEntry(msg)
		m.worktreeCursor = 0
		m.panel = panelWorktreeList

	case blameMsg:
		m.blameLines = msg.lines
		m.blameTitle = msg.title
		m.blameScroll = 0
		m.panel = panelBlame

	case bisectStateMsg:
		m.bisectState = (*git.BisectState)(msg)

	case bisectLogMsg:
		m.bisectLog = string(msg)

	case bisectActionMsg:
		m.lastCmd = msg.cmd
		m.actionErr = msg.err
		return m, tea.Batch(m.fetchStatus(), m.doFetchBisectState(), m.doFetchBisectLog())

	case rebaseTodosMsg:
		m.rebaseBase = msg.base
		m.rebaseTodos = nil
		for _, line := range msg.lines {
			// each line: "abc1234 commit message here"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) == 2 {
				m.rebaseTodos = append(m.rebaseTodos, rebaseTodo{
					action: "pick",
					hash:   parts[0],
					msg:    parts[1],
				})
			}
		}
		if len(m.rebaseTodos) > 0 {
			m.rebaseCursor = 0
			m.rebaseStep = 1
		} else {
			m.actionErr = fmt.Errorf("no commits found for base %q", msg.base)
		}

	case amendDetailMsg:
		m.amendDetail = (*git.CommitDetail)(msg)

	case configFileMsg:
		m.configSection = msg.section
		m.configFileLines = msg.lines
		m.configFilePath = msg.path
		m.configEntries = msg.entries
		m.configFileScroll = 0
		m.panel = panelConfigFile

	case configRecommendMsg:
		m.configRecommendations = []configRecommend(msg)
		m.configRecommendCursor = 0
		m.panel = panelConfigRecommend

	case configProfilesMsg:
		m.configProfiles = []configProfile(msg)
		m.configProfileCursor = 0
		m.configProfileStep = 0
		m.panel = panelConfigProfiles

	case editorDoneMsg:
		if m.panel == panelConfigFile {
			return m, m.doLoadConfigSection(m.configSection)
		}

	case cleanPreviewMsg:
		m.cleanFiles = []string(msg)
		if len(m.cleanFiles) == 0 {
			m.actionErr = fmt.Errorf("nothing to clean - working tree already has no untracked files")
			m.panel = panelMain
		} else {
			lines := make([]string, len(m.cleanFiles))
			copy(lines, m.cleanFiles)
			m.confirmPrompt = fmt.Sprintf("remove %d untracked file(s)? this cannot be undone\n  %s",
				len(lines), strings.Join(lines, "\n  "))
			m.confirmCmd = m.doClean()
			m.panel = panelConfirm
		}

	case reflogMsg:
		m.reflogEntries = []git.ReflogEntry(msg)
		m.reflogCursor = 0
		m.panel = panelReflog

	case remotesMsg:
		m.remotes = []git.RemoteEntry(msg)
		m.remoteCursor = 0
		m.panel = panelRemoteList

	case submodulesMsg:
		m.submodules = []git.SubmoduleEntry(msg)
		m.submoduleCursor = 0
		m.panel = panelSubmoduleList

	case noteMsg:
		m.noteCommit = msg.commit
		m.noteContent = msg.content
		m.noteEditing = false
		m.panel = panelNoteView

	case hunkLoadMsg:
		if msg.err != nil {
			m.actionErr = msg.err
			m.panel = panelMain
			break
		}
		if len(msg.hunks) == 0 {
			m.actionErr = fmt.Errorf("no diff hunks found for %s", m.hunkFile)
			m.panel = panelMain
			break
		}
		m.hunkFileHdr = msg.fileHdr
		m.hunkList = msg.hunks
		m.hunkSel = make([]bool, len(msg.hunks))
		for i := range m.hunkSel {
			m.hunkSel[i] = true
		}
		m.hunkCursor = 0
		m.panel = panelHunkStage

	case fileHistoryMsg:
		m.fileHistoryEntries = []git.LogEntry(msg)
		m.fileHistoryCursor = 0
		m.panel = panelFileHistory

	case graphMsg:
		raw := string(msg)
		if raw == "" {
			m.graphLines = []string{}
		} else {
			m.graphLines = strings.Split(strings.TrimRight(raw, "\n"), "\n")
		}
		m.graphScroll = 0
		m.panel = panelGraph

	case actionDoneMsg:
		m.pushing = false
		m.pulling = false
		m.lastCmd = msg.cmd
		m.actionErr = msg.err

		var baseCmds []tea.Cmd
		baseCmds = append(baseCmds, m.fetchStatus())

		key := commandKey(msg.cmd)
		if key != "" && msg.err == nil {
			count := m.usage.Increment(key)
			baseCmds = append(baseCmds, m.doSaveUsage())

			// Ask once when the user first reaches the mastery threshold.
			if count >= masteryThreshold(key) && !m.usage.WasPrompted(key) {
				m.usage.SetPrompted(key)
				m.masteryKey = key
				m.masteryCursor = 0
				m.panel = panelMastery
				return m, tea.Batch(baseCmds...)
			}
		}

		suppressed := key != "" && m.usage.IsSuppressed(key)
		dur := m.cfg.Education.PanelDuration

		var showEdu bool
		if suppressed {
			showEdu = false
		} else if m.cfg.Modes.Default == "pro" {
			// Pro mode: only show education for complex operations the user has rarely run.
			showEdu = isProComplex(key) && msg.err == nil && m.usage.Count(key) <= 3
		} else {
			showEdu = dur > 0
		}

		if showEdu {
			m.edu = newEduPanel(msg.cmd, msg.err)
			if m.cfg.Modes.Default == "standard" {
				m.edu.explain = ""
			} else if hint := flowHint(msg.cmd, detectFlow(m.cfg)); hint != "" {
				if m.edu.explain != "" {
					m.edu.explain += "\n\n" + hint
				} else {
					m.edu.explain = hint
				}
			}
			eduDur := dur
			if eduDur <= 0 {
				eduDur = 5
			}
			m.eduTimer = eduDur
			m.panel = panelEducation
			baseCmds = append(baseCmds, startEduTimer())
		}

		return m, tea.Batch(baseCmds...)

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
		if m.panel == panelAmend && m.amendField > 0 {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through to updateAmendPanel
			default:
				var cmd tea.Cmd
				m.amendInput, cmd = m.amendInput.Update(msg)
				return m, cmd
			}
		}
		if m.panel == panelBisect && m.bisectInputActive {
			var cmd tea.Cmd
			m.bisectInput, cmd = m.bisectInput.Update(msg)
			return m, cmd
		}
		if m.panel == panelRebaseInteractive && m.rebaseStep == 0 {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through to updateRebaseInteractivePanel
			default:
				var cmd tea.Cmd
				m.rebaseBaseInput, cmd = m.rebaseBaseInput.Update(msg)
				return m, cmd
			}
		}
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
		if m.panel == panelWorktreeList {
			return m.updateWorktreeListPanel(msg)
		}
		if m.panel == panelWorktreeAdd {
			return m.updateWorktreeAddPanel(msg)
		}
		if m.panel == panelBlame {
			return m.updateBlamePanel(msg)
		}
		if m.panel == panelBisect {
			return m.updateBisectPanel(msg)
		}
		if m.panel == panelRebaseInteractive {
			return m.updateRebaseInteractivePanel(msg)
		}
		if m.panel == panelAmend {
			return m.updateAmendPanel(msg)
		}
		if m.panel == panelConfigMenu {
			return m.updateConfigMenuPanel(msg)
		}
		if m.panel == panelConfigFile {
			return m.updateConfigFilePanel(msg)
		}
		if m.panel == panelConfigRecommend {
			return m.updateConfigRecommendPanel(msg)
		}
		if m.panel == panelConfigProfiles {
			if m.configProfileStep > 0 {
				switch msg.String() {
				case "enter", "esc", "ctrl+c":
					// fall through to updateConfigProfilesPanel
				default:
					var cmd tea.Cmd
					m.configProfileInput, cmd = m.configProfileInput.Update(msg)
					return m, cmd
				}
			}
			return m.updateConfigProfilesPanel(msg)
		}
		if m.panel == panelFetch {
			return m.updateFetchPanel(msg)
		}
		if m.panel == panelRestore {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through
			default:
				var cmd tea.Cmd
				m.restoreInput, cmd = m.restoreInput.Update(msg)
				return m, cmd
			}
			return m.updateRestorePanel(msg)
		}
		if m.panel == panelReflog {
			return m.updateReflogPanel(msg)
		}
		if m.panel == panelRemoteList {
			return m.updateRemoteListPanel(msg)
		}
		if m.panel == panelRemoteAdd {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through
			default:
				var cmd tea.Cmd
				m.remoteAddInputs[m.remoteAddStep], cmd = m.remoteAddInputs[m.remoteAddStep].Update(msg)
				return m, cmd
			}
			return m.updateRemoteAddPanel(msg)
		}
		if m.panel == panelRemoteRename {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through
			default:
				var cmd tea.Cmd
				m.remoteRenameInput, cmd = m.remoteRenameInput.Update(msg)
				return m, cmd
			}
			return m.updateRemoteRenamePanel(msg)
		}
		if m.panel == panelSubmoduleList {
			return m.updateSubmoduleListPanel(msg)
		}
		if m.panel == panelSubmoduleAdd {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through
			default:
				var cmd tea.Cmd
				m.submoduleInputs[m.submoduleStep], cmd = m.submoduleInputs[m.submoduleStep].Update(msg)
				return m, cmd
			}
			return m.updateSubmoduleAddPanel(msg)
		}
		if m.panel == panelNoteView {
			if m.noteEditing {
				switch msg.String() {
				case "enter", "esc", "ctrl+c":
					// fall through
				default:
					var cmd tea.Cmd
					m.noteInput, cmd = m.noteInput.Update(msg)
					return m, cmd
				}
			}
			return m.updateNoteViewPanel(msg)
		}
		if m.panel == panelHunkStage {
			return m.updateHunkStagePanel(msg)
		}
		if m.panel == panelPushOpts {
			return m.updatePushOptsPanel(msg)
		}
		if m.panel == panelMastery {
			return m.updateMasteryPanel(msg)
		}
		if m.panel == panelStashMsg {
			switch msg.String() {
			case "enter", "esc", "ctrl+c":
				// fall through to updateStashMsgPanel
			default:
				var cmd tea.Cmd
				m.stashMsgInput, cmd = m.stashMsgInput.Update(msg)
				return m, cmd
			}
			return m.updateStashMsgPanel(msg)
		}
		if m.panel == panelFileHistory {
			return m.updateFileHistoryPanel(msg)
		}
		if m.panel == panelGraph {
			return m.updateGraphPanel(msg)
		}
		if m.panel == panelEduMgr {
			return m.updateEduMgrPanel(msg)
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
	if m.panel == panelWorktreeAdd {
		var cmd tea.Cmd
		m.branchInput, cmd = m.branchInput.Update(msg)
		return m, cmd
	}
	if m.panel == panelBisect && m.bisectInputActive {
		var cmd tea.Cmd
		m.bisectInput, cmd = m.bisectInput.Update(msg)
		return m, cmd
	}
	if m.panel == panelAmend && m.amendField > 0 {
		var cmd tea.Cmd
		m.amendInput, cmd = m.amendInput.Update(msg)
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
		m.pushOptCursor = 0
		m.panel = panelPushOpts
		m.actionErr = nil

	case "h":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category == catConflict {
			m.actionErr = fmt.Errorf("resolve conflict first - press [d] to view")
			break
		}
		if f.category == catUntracked {
			m.actionErr = fmt.Errorf("untracked file - press [space] to stage the whole file")
			break
		}
		staged := f.category == catStaged
		m.hunkFile = f.entry.Path
		m.hunkStaged = staged
		m.hunkList = nil
		m.hunkSel = nil
		m.panel = panelHunkStage
		return m, m.doLoadHunks(f.entry.Path, staged)

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
		ti := textinput.New()
		ti.Placeholder = "stash message (optional)"
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.stashMsgInput = ti
		m.panel = panelStashMsg
		m.actionErr = nil

	case "H":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		m.fileHistoryPath = f.entry.Path
		m.fileHistoryEntries = nil
		m.fileHistoryCursor = 0
		return m, m.doFileHistory(f.entry.Path)

	case kb.Graph, "g":
		m.graphLines = nil
		m.graphScroll = 0
		m.panel = panelGraph
		return m, m.doGraph()

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

	case "W":
		m.worktrees = nil
		m.worktreeCursor = 0
		m.panel = panelWorktreeList
		return m, m.doFetchWorktrees()

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

	case "e":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category == catUntracked {
			m.actionErr = fmt.Errorf("untracked file has no blame history - stage and commit it first")
			break
		}
		if f.category == catConflict {
			m.actionErr = fmt.Errorf("resolve this conflict before viewing blame")
			break
		}
		m.blameLines = nil
		m.blameScroll = 0
		m.panel = panelBlame
		return m, m.doBlame(f.entry.Path)

	case "i":
		m.panel = panelBisect
		m.bisectState = nil
		m.bisectLog = ""
		m.bisectInputActive = false
		m.actionErr = nil
		return m, tea.Batch(m.doFetchBisectState(), m.doFetchBisectLog())

	case "R":
		ti := textinput.New()
		ti.Placeholder = "HEAD~3  or  abc1234  (commits to include)"
		ti.Focus()
		ti.CharLimit = 64
		ti.Width = m.width - 6
		m.rebaseBaseInput = ti
		m.rebaseStep = 0
		m.rebaseTodos = nil
		m.rebaseCursor = 0
		m.panel = panelRebaseInteractive
		m.actionErr = nil

	case "A":
		m.amendField = 0
		m.amendDetail = nil
		m.panel = panelAmend
		m.actionErr = nil
		return m, m.doFetchAmendDetail()

	case "C":
		m.configMenuCursor = 0
		m.panel = panelConfigMenu
		m.actionErr = nil

	case "f":
		m.fetchCursor = 0
		m.panel = panelFetch
		m.actionErr = nil

	case "X":
		m.cleanFiles = nil
		m.actionErr = nil
		return m, m.doCleanPreview()

	case "o":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category == catUntracked {
			m.actionErr = fmt.Errorf("untracked files cannot be restored - they were never committed")
			break
		}
		ti := textinput.New()
		ti.Placeholder = "HEAD  (or a commit hash / ref)"
		ti.SetValue("HEAD")
		ti.Focus()
		ti.CharLimit = 64
		ti.Width = m.width - 6
		m.restoreFile = f.entry.Path
		m.restoreInput = ti
		m.panel = panelRestore
		m.actionErr = nil

	case "L":
		m.reflogEntries = nil
		m.reflogCursor = 0
		m.actionErr = nil
		return m, m.doFetchReflog()

	case "O":
		m.remotes = nil
		m.remoteCursor = 0
		m.actionErr = nil
		return m, m.doFetchRemotes()

	case "M":
		m.submodules = nil
		m.submoduleCursor = 0
		m.actionErr = nil
		return m, m.doFetchSubmodules()

	case "n":
		if m.status == nil {
			break
		}
		// Show note for HEAD commit via the log: open log and let user pick.
		// For now, load note for HEAD directly.
		m.noteCommit = "HEAD"
		m.noteContent = ""
		m.noteEditing = false
		m.actionErr = nil
		return m, m.doFetchNote("HEAD")
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
			if m.branchRenameTarget != "" {
				target := m.branchRenameTarget
				m.branchRenameTarget = ""
				return m, m.doRenameBranch(target, name)
			}
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
				m.commitDetailOrigin = panelLog
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
	case "d":
		if m.commitDetail != nil && m.commitDetail.Hash != "" {
			return m, m.doFetchDiffCommit(m.commitDetail.Hash)
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
		m.panel = m.commitDetailOrigin
		if m.commitDetailOrigin == 0 {
			m.panel = panelLog
		}
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
	case "d":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		if b.Current {
			m.actionErr = fmt.Errorf("cannot delete the current branch - switch to another branch first")
			break
		}
		m.confirmPrompt = fmt.Sprintf("delete branch %s? (unmerged work will be lost)", b.Name)
		m.confirmCmd = m.doDeleteBranch(b.Name)
		m.panel = panelConfirm
	case "n":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		ti := textinput.New()
		ti.Placeholder = "new branch name"
		ti.SetValue(b.Name)
		ti.Focus()
		ti.CharLimit = 128
		ti.Width = m.width - 6
		m.branchInput = ti
		m.branchMode = branchModeRename
		m.branchRenameTarget = b.Name
		m.panel = panelBranch
		m.actionErr = nil
	case "D":
		if len(m.branches) == 0 {
			break
		}
		b := m.branches[m.branchCursor]
		if b.Upstream == "" {
			m.actionErr = fmt.Errorf("branch %s has no remote tracking ref", b.Name)
			break
		}
		idx := strings.Index(b.Upstream, "/")
		if idx < 0 {
			m.actionErr = fmt.Errorf("could not parse upstream %q", b.Upstream)
			break
		}
		remote := b.Upstream[:idx]
		remoteBranch := b.Upstream[idx+1:]
		m.confirmPrompt = fmt.Sprintf("delete %s from remote %s?", remoteBranch, remote)
		m.confirmCmd = m.doDeleteRemoteBranch(remote, remoteBranch)
		m.panel = panelConfirm
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
	case "e":
		path := strings.TrimSuffix(m.diffTitle, "  (staged)")
		m.blameLines = nil
		m.blameScroll = 0
		m.panel = panelBlame
		return m, m.doBlame(path)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateBlamePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 5
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(m.blameLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	kb := m.cfg.Keybindings
	switch msg.String() {
	case "up", "k":
		if m.blameScroll > 0 {
			m.blameScroll--
		}
	case "down", "j":
		if m.blameScroll < maxScroll {
			m.blameScroll++
		}
	case "esc", kb.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateBisectPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.cfg.Keybindings
	active := m.bisectState != nil && m.bisectState.Active

	if m.bisectInputActive {
		switch msg.String() {
		case "enter":
			hash := strings.TrimSpace(m.bisectInput.Value())
			m.bisectInputActive = false
			m.bisectInput.Blur()
			m.actionErr = nil
			return m, m.doBisectGood(hash)
		case "esc":
			m.bisectInputActive = false
			m.bisectInput.Blur()
		}
		return m, nil
	}

	switch msg.String() {
	case "s":
		if !active {
			m.actionErr = nil
			return m, m.doBisectStart()
		}
	case "b":
		if active {
			m.actionErr = nil
			return m, m.doBisectBad()
		}
	case "g":
		if active {
			ti := textinput.New()
			ti.Placeholder = "commit hash (leave empty to mark HEAD as good)"
			ti.Focus()
			ti.CharLimit = 64
			ti.Width = m.width - 6
			m.bisectInput = ti
			m.bisectInputActive = true
			m.actionErr = nil
		}
	case "G":
		if active {
			m.actionErr = nil
			return m, m.doBisectGood("")
		}
	case "r":
		if active {
			m.confirmPrompt = "reset bisect session and return to original branch?"
			m.confirmCmd = m.doBisectReset()
			m.panel = panelConfirm
			m.actionErr = nil
		}
	case "l":
		return m, m.doFetchBisectLog()
	case "esc", kb.Quit:
		if active {
			m.actionErr = fmt.Errorf("bisect in progress - press [r] to reset first")
		} else {
			m.panel = panelMain
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateRebaseInteractivePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.rebaseStep == 0 {
		switch msg.String() {
		case "enter":
			base := strings.TrimSpace(m.rebaseBaseInput.Value())
			if base == "" {
				break
			}
			return m, m.doFetchRebaseTodos(base)
		case "esc":
			m.panel = panelMain
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// rebaseStep == 1: editing todo list
	switch msg.String() {
	case "up", "k":
		if m.rebaseCursor > 0 {
			m.rebaseCursor--
		}
	case "down", "j":
		if m.rebaseCursor < len(m.rebaseTodos)-1 {
			m.rebaseCursor++
		}
	case "K":
		if m.rebaseCursor > 0 {
			m.rebaseTodos[m.rebaseCursor], m.rebaseTodos[m.rebaseCursor-1] = m.rebaseTodos[m.rebaseCursor-1], m.rebaseTodos[m.rebaseCursor]
			m.rebaseCursor--
		}
	case "J":
		if m.rebaseCursor < len(m.rebaseTodos)-1 {
			m.rebaseTodos[m.rebaseCursor], m.rebaseTodos[m.rebaseCursor+1] = m.rebaseTodos[m.rebaseCursor+1], m.rebaseTodos[m.rebaseCursor]
			m.rebaseCursor++
		}
	case "p":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "pick"
		}
	case "r":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "reword"
		}
	case "e":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "edit"
		}
	case "s":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "squash"
		}
	case "f":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "fixup"
		}
	case "d":
		if len(m.rebaseTodos) > 0 {
			m.rebaseTodos[m.rebaseCursor].action = "drop"
		}
	case "enter":
		m.panel = panelMain
		return m, m.doRebaseInteractive()
	case "esc":
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateAmendPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.cfg.Keybindings
	if m.amendField == 0 {
		switch msg.String() {
		case "m":
			ti := textinput.New()
			ti.Placeholder = "commit message"
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.width - 6
			if m.amendDetail != nil {
				ti.SetValue(m.amendDetail.Subject)
			}
			m.amendInput = ti
			m.amendField = 1
		case "a":
			ti := textinput.New()
			ti.Placeholder = "Name <email>"
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.width - 6
			if m.amendDetail != nil {
				ti.SetValue(m.amendDetail.Author)
			}
			m.amendInput = ti
			m.amendField = 2
		case "d":
			ti := textinput.New()
			ti.Placeholder = "2026-01-15T10:30:00 or now"
			ti.Focus()
			ti.CharLimit = 64
			ti.Width = m.width - 6
			m.amendInput = ti
			m.amendField = 3
		case "n":
			m.panel = panelMain
			return m, m.doAmendNoEdit()
		case "esc", kb.Quit:
			m.panel = panelMain
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// amendField > 0: input is active
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.amendInput.Value())
		if value == "" {
			m.actionErr = fmt.Errorf("value cannot be empty")
			return m, nil
		}
		field := m.amendField
		m.amendField = 0
		m.panel = panelMain
		switch field {
		case 1:
			return m, m.doAmendMessage(value)
		case 2:
			return m, m.doAmendAuthor(value)
		case 3:
			return m, m.doAmendDate(value)
		}
		return m, nil
	case "esc":
		m.amendField = 0
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfigMenuPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const numItems = 7
	switch msg.String() {
	case "up", "k":
		if m.configMenuCursor > 0 {
			m.configMenuCursor--
		}
	case "down", "j":
		if m.configMenuCursor < numItems-1 {
			m.configMenuCursor++
		}
	case "enter":
		switch m.configMenuCursor {
		case 0:
			return m, m.doLoadConfigSection(configSectionGlobal)
		case 1:
			return m, m.doLoadConfigSection(configSectionLocal)
		case 2:
			return m, m.doLoadConfigSection(configSectionGlobalIgnore)
		case 3:
			return m, m.doLoadConfigSection(configSectionLocalIgnore)
		case 4:
			return m, m.doLoadRecommendations()
		case 5:
			return m, m.doLoadProfiles()
		case 6:
			m.eduMgrKeys = buildEduMgrKeys(m.usage)
			m.eduMgrCursor = 0
			m.panel = panelEduMgr
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfigFilePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines := m.height - 5
	if visibleLines < 1 {
		visibleLines = 1
	}
	contentLines := m.configFileLines
	if m.configEntries != nil {
		contentLines = formatConfigEntries(m.configEntries)
	}
	maxScroll := len(contentLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch msg.String() {
	case "up", "k":
		if m.configFileScroll > 0 {
			m.configFileScroll--
		}
	case "down", "j":
		if m.configFileScroll < maxScroll {
			m.configFileScroll++
		}
	case "e":
		if m.configFilePath != "" {
			return m, m.doOpenEditor(m.configFilePath)
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelConfigMenu
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfigRecommendPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.configRecommendCursor > 0 {
			m.configRecommendCursor--
		}
	case "down", "j":
		if m.configRecommendCursor < len(m.configRecommendations)-1 {
			m.configRecommendCursor++
		}
	case "enter", "a":
		if m.configRecommendCursor < len(m.configRecommendations) {
			rec := m.configRecommendations[m.configRecommendCursor]
			if !rec.applied {
				return m, tea.Batch(
					m.doApplyRecommendation(rec.key, rec.value),
					m.doLoadRecommendations(),
				)
			}
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelConfigMenu
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateConfigProfilesPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configProfileStep == 0 {
		switch msg.String() {
		case "up", "k":
			if m.configProfileCursor > 0 {
				m.configProfileCursor--
			}
		case "down", "j":
			if m.configProfileCursor < len(m.configProfiles)-1 {
				m.configProfileCursor++
			}
		case "n":
			ti := textinput.New()
			ti.Placeholder = "../work/"
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.width - 6
			m.configProfileInput = ti
			m.configProfileStep = 1
		case "esc", m.cfg.Keybindings.Quit:
			m.panel = panelConfigMenu
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	if m.configProfileStep == 1 {
		switch msg.String() {
		case "enter":
			m.configProfileNewPath = strings.TrimSpace(m.configProfileInput.Value())
			ti := textinput.New()
			ti.Placeholder = "~/.gitconfig-work"
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.width - 6
			m.configProfileInput = ti
			m.configProfileStep = 2
		case "esc":
			m.configProfileStep = 0
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// configProfileStep == 2
	switch msg.String() {
	case "enter":
		includePath := strings.TrimSpace(m.configProfileInput.Value())
		gitdir := m.configProfileNewPath
		m.configProfileStep = 0
		return m, m.doAddProfile(gitdir, includePath)
	case "esc":
		ti := textinput.New()
		ti.Placeholder = "../work/"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = m.width - 6
		ti.SetValue(m.configProfileNewPath)
		m.configProfileInput = ti
		m.configProfileStep = 1
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// fetch panel
// ---------------------------------------------------------------------------

var fetchOptions = []struct {
	label string
	all   bool
	prune bool
}{
	{"fetch origin", false, false},
	{"fetch origin --prune", false, true},
	{"fetch --all", true, false},
	{"fetch --all --prune", true, true},
}

func (m model) updateFetchPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.fetchCursor > 0 {
			m.fetchCursor--
		}
	case "down", "j":
		if m.fetchCursor < len(fetchOptions)-1 {
			m.fetchCursor++
		}
	case "enter":
		opt := fetchOptions[m.fetchCursor]
		m.panel = panelMain
		return m, m.doFetch(opt.all, opt.prune)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// restore panel
// ---------------------------------------------------------------------------

func (m model) updateRestorePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		source := strings.TrimSpace(m.restoreInput.Value())
		path := m.restoreFile
		m.panel = panelMain
		return m, m.doRestoreFile(path, source)
	case "esc", "ctrl+c":
		m.panel = panelMain
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// reflog panel
// ---------------------------------------------------------------------------

func (m model) updateReflogPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.reflogCursor > 0 {
			m.reflogCursor--
		}
	case "down", "j":
		if m.reflogCursor < len(m.reflogEntries)-1 {
			m.reflogCursor++
		}
	case "r":
		if len(m.reflogEntries) == 0 {
			break
		}
		e := m.reflogEntries[m.reflogCursor]
		hash := e.Hash
		m.confirmPrompt = fmt.Sprintf("reset HEAD to %s (%s)?", hash, e.Ref)
		m.confirmCmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "git", "reset", "--mixed", hash)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return actionDoneMsg{cmd: "git reset --mixed " + hash, err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			}
			return actionDoneMsg{cmd: "git reset --mixed " + hash, err: nil}
		}
		m.panel = panelConfirm
	case "y":
		if len(m.reflogEntries) > 0 {
			_ = writeClipboard(m.reflogEntries[m.reflogCursor].Hash)
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// remote list panel
// ---------------------------------------------------------------------------

func (m model) updateRemoteListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.remoteCursor > 0 {
			m.remoteCursor--
		}
	case "down", "j":
		if m.remoteCursor < len(m.remotes)-1 {
			m.remoteCursor++
		}
	case "a":
		ti0 := textinput.New()
		ti0.Placeholder = "name (e.g. origin)"
		ti0.Focus()
		ti0.CharLimit = 64
		ti0.Width = m.width - 6
		ti1 := textinput.New()
		ti1.Placeholder = "url (e.g. git@github.com:org/repo.git)"
		ti1.CharLimit = 256
		ti1.Width = m.width - 6
		m.remoteAddInputs = [2]textinput.Model{ti0, ti1}
		m.remoteAddStep = 0
		m.panel = panelRemoteAdd
	case "d":
		if len(m.remotes) == 0 {
			break
		}
		name := m.remotes[m.remoteCursor].Name
		m.confirmPrompt = fmt.Sprintf("remove remote %q?", name)
		m.confirmCmd = m.doRemoteRemove(name)
		m.panel = panelConfirm
	case "r":
		if len(m.remotes) == 0 {
			break
		}
		ti := textinput.New()
		ti.Placeholder = "new name"
		ti.Focus()
		ti.CharLimit = 64
		ti.Width = m.width - 6
		m.remoteRenameTarget = m.remotes[m.remoteCursor].Name
		m.remoteRenameInput = ti
		m.panel = panelRemoteRename
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// remote add panel
// ---------------------------------------------------------------------------

func (m model) updateRemoteAddPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.remoteAddStep == 0 {
			val := strings.TrimSpace(m.remoteAddInputs[0].Value())
			if val == "" {
				break
			}
			m.remoteAddInputs[0].Blur()
			m.remoteAddInputs[1].Focus()
			m.remoteAddStep = 1
		} else {
			name := strings.TrimSpace(m.remoteAddInputs[0].Value())
			url := strings.TrimSpace(m.remoteAddInputs[1].Value())
			if name == "" || url == "" {
				break
			}
			m.panel = panelRemoteList
			return m, tea.Batch(m.doRemoteAdd(name, url), m.doFetchRemotes())
		}
	case "esc":
		if m.remoteAddStep == 1 {
			m.remoteAddInputs[1].Blur()
			m.remoteAddInputs[0].Focus()
			m.remoteAddStep = 0
		} else {
			m.panel = panelRemoteList
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// remote rename panel
// ---------------------------------------------------------------------------

func (m model) updateRemoteRenamePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		newName := strings.TrimSpace(m.remoteRenameInput.Value())
		if newName == "" {
			break
		}
		old := m.remoteRenameTarget
		m.panel = panelRemoteList
		return m, tea.Batch(m.doRemoteRename(old, newName), m.doFetchRemotes())
	case "esc", "ctrl+c":
		m.panel = panelRemoteList
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// submodule list panel
// ---------------------------------------------------------------------------

func (m model) updateSubmoduleListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.submoduleCursor > 0 {
			m.submoduleCursor--
		}
	case "down", "j":
		if m.submoduleCursor < len(m.submodules)-1 {
			m.submoduleCursor++
		}
	case "a":
		ti0 := textinput.New()
		ti0.Placeholder = "repository url"
		ti0.Focus()
		ti0.CharLimit = 256
		ti0.Width = m.width - 6
		ti1 := textinput.New()
		ti1.Placeholder = "local path (leave empty to use repo name)"
		ti1.CharLimit = 256
		ti1.Width = m.width - 6
		m.submoduleInputs = [2]textinput.Model{ti0, ti1}
		m.submoduleStep = 0
		m.panel = panelSubmoduleAdd
	case "u":
		m.panel = panelMain
		return m, m.doSubmoduleUpdate(true)
	case "d":
		if len(m.submodules) == 0 {
			break
		}
		path := m.submodules[m.submoduleCursor].Path
		m.confirmPrompt = fmt.Sprintf("deinit submodule %q? (removes from .git/config, keeps files)", path)
		m.confirmCmd = m.doSubmoduleDeinit(path)
		m.panel = panelConfirm
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// submodule add panel
// ---------------------------------------------------------------------------

func (m model) updateSubmoduleAddPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.submoduleStep == 0 {
			val := strings.TrimSpace(m.submoduleInputs[0].Value())
			if val == "" {
				break
			}
			m.submoduleInputs[0].Blur()
			m.submoduleInputs[1].Focus()
			m.submoduleStep = 1
		} else {
			url := strings.TrimSpace(m.submoduleInputs[0].Value())
			path := strings.TrimSpace(m.submoduleInputs[1].Value())
			m.panel = panelSubmoduleList
			return m, m.doSubmoduleAdd(url, path)
		}
	case "esc":
		if m.submoduleStep == 1 {
			m.submoduleInputs[1].Blur()
			m.submoduleInputs[0].Focus()
			m.submoduleStep = 0
		} else {
			m.panel = panelSubmoduleList
		}
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// note view panel
// ---------------------------------------------------------------------------

func (m model) updateNoteViewPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.noteEditing {
		switch msg.String() {
		case "enter":
			note := strings.TrimSpace(m.noteInput.Value())
			m.noteEditing = false
			m.noteContent = note
			commit := m.noteCommit
			m.panel = panelMain
			return m, m.doNoteAdd(commit, note)
		case "esc":
			m.noteEditing = false
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}
	switch msg.String() {
	case "e":
		ti := textinput.New()
		ti.SetValue(m.noteContent)
		ti.Focus()
		ti.CharLimit = 512
		ti.Width = m.width - 6
		m.noteInput = ti
		m.noteEditing = true
	case "d":
		if m.noteContent == "" {
			break
		}
		m.confirmPrompt = fmt.Sprintf("remove note from commit %s?", m.noteCommit)
		m.confirmCmd = m.doNoteRemove(m.noteCommit)
		m.panel = panelConfirm
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateHunkStagePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.hunkList == nil {
		if msg.String() == "esc" || msg.String() == m.cfg.Keybindings.Quit || msg.String() == "ctrl+c" {
			m.panel = panelMain
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if m.hunkCursor > 0 {
			m.hunkCursor--
		}
	case "down", "j":
		if m.hunkCursor < len(m.hunkList)-1 {
			m.hunkCursor++
		}
	case " ":
		if m.hunkCursor < len(m.hunkSel) {
			m.hunkSel[m.hunkCursor] = !m.hunkSel[m.hunkCursor]
		}
	case "a":
		allOn := true
		for _, s := range m.hunkSel {
			if !s {
				allOn = false
				break
			}
		}
		for i := range m.hunkSel {
			m.hunkSel[i] = !allOn
		}
	case "enter":
		m.panel = panelMain
		return m, m.doApplyHunks()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updatePushOptsPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pushOptCursor > 0 {
			m.pushOptCursor--
		}
	case "down", "j":
		if m.pushOptCursor < len(pushMenuOptions)-1 {
			m.pushOptCursor++
		}
	case "enter":
		if m.pushing {
			break
		}
		m.pushing = true
		m.panel = panelMain
		return m, m.doPushWithOpts()
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
	case "a":
		if len(m.stashes) == 0 {
			break
		}
		ref := m.stashes[m.stashCursor].Ref
		m.panel = panelMain
		return m, m.doStashApply(ref)
	case "d":
		if len(m.stashes) == 0 {
			break
		}
		ref := m.stashes[m.stashCursor].Ref
		m.confirmPrompt = fmt.Sprintf("drop %s? this cannot be undone", ref)
		m.confirmCmd = m.doStashDrop(ref)
		m.panel = panelConfirm
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
	case "p":
		if len(m.tags) == 0 {
			break
		}
		tag := m.tags[m.tagCursor]
		m.confirmPrompt = fmt.Sprintf("push tag %s to origin?", tag.Name)
		m.confirmCmd = m.doPushTag("origin", tag.Name)
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

func (m model) updateWorktreeListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := m.cfg.Keybindings
	switch msg.String() {
	case "up", "k":
		if m.worktreeCursor > 0 {
			m.worktreeCursor--
		}
	case "down", "j":
		if m.worktreeCursor < len(m.worktrees)-1 {
			m.worktreeCursor++
		}
	case "n":
		ti := textinput.New()
		ti.Placeholder = "path/to/worktree  branch-name"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = m.width - 6
		m.branchInput = ti
		m.panel = panelWorktreeAdd
		m.actionErr = nil
	case "d":
		if len(m.worktrees) == 0 {
			break
		}
		wt := m.worktrees[m.worktreeCursor]
		if wt.Current {
			m.actionErr = fmt.Errorf("cannot remove the current (main) worktree")
			break
		}
		m.confirmPrompt = fmt.Sprintf("remove worktree at %s?", wt.Path)
		m.confirmCmd = m.doRemoveWorktree(wt.Path)
		m.panel = panelConfirm
		m.actionErr = nil
	case "esc", kb.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateWorktreeAddPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		raw := strings.TrimSpace(m.branchInput.Value())
		if raw == "" {
			m.actionErr = fmt.Errorf("path cannot be empty")
			return m, nil
		}
		var path, branch string
		if idx := strings.Index(raw, " "); idx >= 0 {
			path = strings.TrimSpace(raw[:idx])
			branch = strings.TrimSpace(raw[idx+1:])
		} else {
			path = raw
		}
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doAddWorktree(path, branch)
	case "esc":
		m.panel = panelWorktreeList
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
	if m.panel == panelWorktreeList {
		return m.worktreeListView()
	}
	if m.panel == panelWorktreeAdd {
		return m.worktreeAddView()
	}
	if m.panel == panelBlame {
		return m.blameView()
	}
	if m.panel == panelBisect {
		return m.bisectView()
	}
	if m.panel == panelRebaseInteractive {
		return m.rebaseInteractiveView()
	}
	if m.panel == panelAmend {
		return m.amendView()
	}
	if m.panel == panelConfigMenu {
		return m.configMenuView()
	}
	if m.panel == panelConfigFile {
		return m.configFileView()
	}
	if m.panel == panelConfigRecommend {
		return m.configRecommendView()
	}
	if m.panel == panelConfigProfiles {
		return m.configProfilesView()
	}
	if m.panel == panelFetch {
		return m.fetchView()
	}
	if m.panel == panelRestore {
		return m.restoreView()
	}
	if m.panel == panelReflog {
		return m.reflogView()
	}
	if m.panel == panelRemoteList {
		return m.remoteListView()
	}
	if m.panel == panelRemoteAdd {
		return m.remoteAddView()
	}
	if m.panel == panelRemoteRename {
		return m.remoteRenameView()
	}
	if m.panel == panelSubmoduleList {
		return m.submoduleListView()
	}
	if m.panel == panelSubmoduleAdd {
		return m.submoduleAddView()
	}
	if m.panel == panelNoteView {
		return m.noteView()
	}
	if m.panel == panelHunkStage {
		return m.hunkStageView()
	}
	if m.panel == panelPushOpts {
		return m.pushOptsView()
	}
	if m.panel == panelMastery {
		return m.masteryView()
	}
	if m.panel == panelStashMsg {
		return m.stashMsgView()
	}
	if m.panel == panelFileHistory {
		return m.fileHistoryView()
	}
	if m.panel == panelGraph {
		return m.graphView()
	}
	if m.panel == panelEduMgr {
		return m.eduMgrView()
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
				name = br.Name
			}
			if br.Upstream != "" {
				name += "  " + styleDim.Render("<- "+br.Upstream)
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
	return content + styleDim.Render("  [enter] switch  [m] merge  [r] rebase  [d] delete  [n] rename  [D] delete remote  [esc] back") + "\n"
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
	row("space", "stage / unstage selected file")
	row("h", "hunk staging - stage individual hunks within a file")
	row("d", "diff selected file (staged or unstaged)")
	row("H", "file history - every commit that touched this file")
	row("e", "blame - who last changed each line")
	row("x", "discard working tree changes (with confirmation)")
	row("o", "restore file to HEAD or a specific ref")
	b.WriteString("\n")

	section("Commit & sync")
	row(kb.Commit+" / c", "open commit panel")
	row("A", "amend HEAD (message, author, date, or add staged files)")
	row(kb.Push+" / p", "push menu (push / force-with-lease / set-upstream)")
	row("P", "pull from remote")
	row("f", "fetch menu (origin / all / prune)")
	b.WriteString("\n")

	section("Branches & history")
	row("b", "create new branch (flow picker in gitflow mode)")
	row("B", "branch list - switch, merge, rebase, delete, rename, delete remote")
	row("l", "commit log (search with ctrl+/ or ctrl+r)")
	row("L", "reflog - full HEAD history with reset-to")
	row(kb.Graph+" / g", "branch graph (git log --graph --all)")
	row("R", "interactive rebase (reorder, squash, fixup, drop)")
	b.WriteString("\n")

	section("Stash & tags")
	row(kb.Stash+" / s", "stash all changes (opens message input)")
	row("S", "stash list - pop, apply, drop")
	row("t", "tag list - create, delete, push to remote")
	b.WriteString("\n")

	section("Advanced")
	row("i", "bisect - binary search for a bug-introducing commit")
	row("z", "reset menu (soft / mixed / hard)")
	row("W", "worktree list - add, remove linked worktrees")
	row("O", "remote management - add, remove, rename")
	row("M", "submodule management - add, update, deinit")
	row("n", "git notes for HEAD commit")
	row("X", "clean untracked files (preview + confirm)")
	row("a", "abort in-progress merge / rebase / cherry-pick")
	b.WriteString("\n")

	section("App")
	row("C", "configuration manager (git config, gitignore, profiles, education)")
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
	return content + styleDim.Render("  [enter] pop  [a] apply  [d] drop  [esc] back") + "\n"
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
	return content + styleDim.Render("  [↑↓] scroll  [e] blame  [esc] back"+pos) + "\n"
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

func (m model) worktreeListView() string {
	var b strings.Builder
	b.WriteString("\n")

	title := "Worktrees"
	if len(m.worktrees) > 0 {
		title = fmt.Sprintf("Worktrees (%d)", len(m.worktrees))
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.worktrees == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.worktrees) == 0 {
		b.WriteString("  " + styleDim.Render("no worktrees found") + "\n")
	} else {
		for i, wt := range m.worktrees {
			cursor := "  "
			if m.worktreeCursor == i {
				cursor = styleSelected.Render("> ")
			}
			mark := "  "
			if wt.Current {
				mark = styleStaged.Render("* ")
			}
			path := styleCmd.Render(wt.Path)
			branch := styleDim.Render(wt.Branch)
			b.WriteString(cursor + mark + path + "  " + branch + "\n")
		}
	}
	b.WriteString("\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [n] add  [d] remove  [esc] back") + "\n"
}

func (m model) worktreeAddView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Add Worktree") + "\n\n")
	b.WriteString("  " + m.branchInput.View() + "\n\n")
	b.WriteString("  " + styleDim.Render("enter path and branch name separated by a space (e.g. ../project-feature feat/my-feature)") + "\n\n")

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

func (m model) blameView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Blame") + "  " + styleDim.Render(m.blameTitle) + "\n\n")

	if m.blameLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.blameLines) == 0 {
		b.WriteString("  " + styleDim.Render("no blame data (file may be untracked or empty)") + "\n")
	} else {
		visibleLines := m.height - 5
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := m.blameScroll
		end := start + visibleLines
		if end > len(m.blameLines) {
			end = len(m.blameLines)
		}
		for _, bl := range m.blameLines[start:end] {
			author := fmt.Sprintf("%-16.16s", bl.Author)
			lineNum := fmt.Sprintf("%4d", bl.LineNum)
			line := "  " +
				styleCmd.Render(bl.Hash) + "  " +
				styleDim.Render(bl.Date) + "  " +
				styleDim.Render(author) + "  " +
				styleDim.Render("│") + "  " +
				styleDim.Render(lineNum) + "  " +
				bl.Text
			b.WriteString(line + "\n")
		}
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	bar := "  [↑↓] scroll  [esc] back"
	if len(m.blameLines) > 0 {
		visibleLines := m.height - 5
		if visibleLines < 1 {
			visibleLines = 1
		}
		if len(m.blameLines) > visibleLines {
			bar += fmt.Sprintf("  (%d/%d)", m.blameScroll+1, len(m.blameLines))
		}
	}
	return content + styleDim.Render(bar) + "\n"
}

func (m model) bisectView() string {
	var b strings.Builder
	b.WriteString("\n")

	active := m.bisectState != nil && m.bisectState.Active

	if active {
		b.WriteString("  " + styleSection.Render("Bisect") + "  " + styleChanged.Render("(active)") + "\n\n")
	} else {
		b.WriteString("  " + styleSection.Render("Bisect") + "\n\n")
	}

	if m.bisectInputActive {
		b.WriteString("  " + styleDim.Render("Enter a known-good commit hash (or leave empty for HEAD):") + "\n")
		b.WriteString("  " + styleDim.Render("> ") + m.bisectInput.View() + "\n\n")
	} else if active {
		current := ""
		if m.bisectState != nil {
			current = m.bisectState.Current
		}
		b.WriteString("  " + styleDim.Render("Testing: ") + styleCmd.Render(current) + "\n")
		if m.bisectState != nil && m.bisectState.Status != "" {
			b.WriteString("  " + styleDim.Render(m.bisectState.Status) + "\n")
		}
		b.WriteString("\n")
		b.WriteString("  " + styleCmd.Render("[b]") + "  " + styleDim.Render("this commit is bad") + "\n")
		b.WriteString("  " + styleCmd.Render("[G]") + "  " + styleDim.Render("this commit is good (HEAD)") + "\n")
		b.WriteString("  " + styleCmd.Render("[g]") + "  " + styleDim.Render("good - enter a specific hash") + "\n")
		b.WriteString("  " + styleCmd.Render("[r]") + "  " + styleDim.Render("reset and end session") + "\n")
	} else {
		b.WriteString("  " + styleDim.Render("Binary search through commit history to find what introduced a bug.") + "\n\n")
		b.WriteString("  " + styleDim.Render("How it works:") + "\n")
		b.WriteString("    " + styleDim.Render("1. start a session       ") + styleCmd.Render("[s]") + "\n")
		b.WriteString("    " + styleDim.Render("2. mark current as bad   ") + styleCmd.Render("[b]") + "\n")
		b.WriteString("    " + styleDim.Render("3. mark a commit as good ") + styleCmd.Render("[g]") + styleDim.Render(" (enter hash) or ") + styleCmd.Render("[G]") + styleDim.Render(" (current is good)") + "\n")
		b.WriteString("    " + styleDim.Render("4. git checks out a midpoint - test your code") + "\n")
		b.WriteString("    " + styleDim.Render("5. repeat [b] / [G] until git finds the first bad commit") + "\n")
		b.WriteString("    " + styleDim.Render("6. reset when done       ") + styleCmd.Render("[r]") + "\n")
	}

	if m.bisectLog != "" {
		b.WriteString("\n")
		b.WriteString("  " + styleDim.Render("Bisect log:") + "\n")
		logLines := strings.Split(strings.TrimRight(m.bisectLog, "\n"), "\n")
		// Show last N lines that fit in viewport.
		overhead := strings.Count(b.String(), "\n") + 3
		visible := m.height - overhead
		if visible < 1 {
			visible = 1
		}
		start := 0
		if len(logLines) > visible {
			start = len(logLines) - visible
		}
		for _, line := range logLines[start:] {
			b.WriteString("  " + styleDim.Render(line) + "\n")
		}
	}

	if m.actionErr != nil {
		b.WriteString("\n  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	var bar string
	if active {
		bar = "  [b] bad  [G] good (HEAD)  [g] good (hash)  [r] reset  [l] log"
	} else {
		bar = "  [s] start  [esc] back"
	}
	return content + styleDim.Render(bar) + "\n"
}

func (m model) rebaseInteractiveView() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.rebaseStep == 0 {
		b.WriteString("  " + styleSection.Render("Interactive Rebase") + "\n\n")
		b.WriteString("  " + styleDim.Render("Enter the base ref - commits from BASE to HEAD will be included.") + "\n")
		b.WriteString("  " + styleDim.Render("Examples: HEAD~3  |  HEAD~10  |  abc1234") + "\n\n")
		b.WriteString("  " + m.rebaseBaseInput.View() + "\n\n")

		if m.actionErr != nil {
			b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
		}

		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [enter] load commits  [esc] cancel") + "\n"
	}

	// rebaseStep == 1: editing todo list
	title := fmt.Sprintf("Interactive Rebase  (%d commits, base: %s)", len(m.rebaseTodos), m.rebaseBase)
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	for i, todo := range m.rebaseTodos {
		cursor := "  "
		if m.rebaseCursor == i {
			cursor = styleSelected.Render("> ")
		}
		var actionStyled string
		switch todo.action {
		case "pick":
			actionStyled = styleStaged.Render(fmt.Sprintf("%-6s", todo.action))
		case "drop":
			actionStyled = styleChanged.Render(fmt.Sprintf("%-6s", todo.action))
		case "squash", "fixup":
			actionStyled = styleUntracked.Render(fmt.Sprintf("%-6s", todo.action))
		default: // reword, edit
			actionStyled = styleCmd.Render(fmt.Sprintf("%-6s", todo.action))
		}
		b.WriteString(cursor + "  " + actionStyled + "  " + styleCmd.Render(todo.hash) + "  " + styleDim.Render(todo.msg) + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + styleDim.Render("actions: [p]ick  [r]eword  [e]dit  [s]quash  [f]ixup  [d]rop") + "\n")
	b.WriteString("           " + styleDim.Render("[K] move up  [J] move down") + "\n\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] execute rebase  [esc] cancel") + "\n"
}

func (m model) amendView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Amend Last Commit") + "\n\n")

	if m.amendDetail == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [esc] cancel") + "\n"
	}

	d := m.amendDetail
	hash := d.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	b.WriteString("  " + styleCmd.Render(hash) + "  " + d.Subject + "\n")
	b.WriteString("  " + styleDim.Render("Author  ") + d.Author + "\n")
	b.WriteString("  " + styleDim.Render("Date    ") + d.Date + "\n")
	b.WriteString("\n")

	if m.amendField == 0 {
		b.WriteString("  " + styleCmd.Render("[m]") + "  " + styleDim.Render("edit message") + "\n")
		b.WriteString("  " + styleCmd.Render("[a]") + "  " + styleDim.Render("edit author") + "\n")
		b.WriteString("  " + styleCmd.Render("[d]") + "  " + styleDim.Render("edit date") + "\n")

		nStaged := 0
		if m.status != nil {
			nStaged = len(m.status.Staged)
		}
		if nStaged > 0 {
			b.WriteString("  " + styleCmd.Render("[n]") + "  " + styleStaged.Render(fmt.Sprintf("add staged files without changing message  (%d file(s) staged)", nStaged)) + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("[n]  add staged files without changing message  (nothing staged)") + "\n")
		}
		b.WriteString("\n")

		if m.actionErr != nil {
			b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
		}

		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [m] message  [a] author  [d] date  [n] add staged  [esc] cancel") + "\n"
	}

	// amendField > 0: input active
	var fieldName string
	switch m.amendField {
	case 1:
		fieldName = "message"
	case 2:
		fieldName = "author (Name <email>)"
	case 3:
		fieldName = "date (ISO 8601 or 'now')"
	}
	b.WriteString("  " + styleDim.Render("Editing: "+fieldName) + "\n\n")
	b.WriteString("  " + styleDim.Render("> ") + m.amendInput.View() + "\n\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] apply  [esc] back") + "\n"
}

// formatConfigEntries converts parsed config entries into display lines grouped
// by the prefix before the first dot.
func formatConfigEntries(entries []git.ConfigEntry) []string {
	var lines []string
	lastGroup := ""
	for _, e := range entries {
		group := e.Key
		if idx := strings.IndexByte(e.Key, '.'); idx >= 0 {
			group = e.Key[:idx]
		}
		if group != lastGroup {
			if lastGroup != "" {
				lines = append(lines, "")
			}
			lastGroup = group
		}
		lines = append(lines, e.Key+"  =  "+e.Value)
	}
	return lines
}

func (m model) configMenuView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Configuration Manager") + "\n\n")

	items := []struct {
		label string
		hint  string
	}{
		{"Global config", "(~/.gitconfig)"},
		{"Local config", "(.git/config)"},
		{"Global gitignore", "(core.excludesfile)"},
		{"Local gitignore", "(.gitignore)"},
		{"Recommendations", "(best practices)"},
		{"Profiles", "(includeIf conditionals)"},
		{"Education & Usage", "(command usage and tip settings)"},
	}

	for i, item := range items {
		cursor := "  "
		if m.configMenuCursor == i {
			cursor = styleSelected.Render("> ")
		}
		label := item.label
		if m.configMenuCursor == i {
			label = styleSelected.Render(label)
		}
		hint := styleDim.Render(item.hint)
		b.WriteString(cursor + "  " + label + "  " + hint + "\n")
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] open  [esc] back") + "\n"
}

func (m model) configFileView() string {
	var b strings.Builder
	b.WriteString("\n")

	var title string
	switch m.configSection {
	case configSectionGlobal:
		title = "Global Config"
	case configSectionLocal:
		title = "Local Config"
	case configSectionGlobalIgnore:
		title = "Global Gitignore"
	case configSectionLocalIgnore:
		title = "Local Gitignore"
	}

	pathHint := ""
	if m.configFilePath != "" {
		pathHint = "  " + styleDim.Render(m.configFilePath)
	}
	b.WriteString("  " + styleSection.Render(title) + pathHint + "\n\n")

	visibleLines := m.height - 5
	if visibleLines < 1 {
		visibleLines = 1
	}

	isGitignore := m.configSection == configSectionGlobalIgnore || m.configSection == configSectionLocalIgnore
	isConfig := m.configSection == configSectionGlobal || m.configSection == configSectionLocal

	var displayLines []string
	if isConfig && len(m.configEntries) > 0 {
		displayLines = formatConfigEntries(m.configEntries)
	} else {
		displayLines = m.configFileLines
	}

	if len(displayLines) == 0 {
		b.WriteString("  " + styleDim.Render("(empty or file not found)") + "\n")
	} else {
		end := m.configFileScroll + visibleLines
		if end > len(displayLines) {
			end = len(displayLines)
		}
		for _, line := range displayLines[m.configFileScroll:end] {
			if isConfig {
				// color key = value
				if idx := strings.Index(line, "  =  "); idx >= 0 {
					key := line[:idx]
					val := line[idx+5:]
					b.WriteString("  " + styleCmd.Render(key) + "  " + styleDim.Render("=") + "  " + val + "\n")
				} else if line == "" {
					b.WriteString("\n")
				} else {
					b.WriteString("  " + styleDim.Render(line) + "\n")
				}
			} else if isGitignore {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					b.WriteString("  " + styleDim.Render(line) + "\n")
				} else if line == "" {
					b.WriteString("\n")
				} else {
					b.WriteString("  " + line + "\n")
				}
			} else {
				b.WriteString("  " + line + "\n")
			}
		}
	}

	if isGitignore {
		b.WriteString("\n")
		b.WriteString("  " + styleDim.Render("Common patterns to consider:") + "\n")
		b.WriteString("  " + styleDim.Render(".DS_Store  Thumbs.db  .env  *.log  .idea/  .vscode/  node_modules/  *.swp") + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	bar := "  [e] open in editor  [↑↓] scroll  [esc] back"
	total := len(displayLines)
	if total > visibleLines {
		bar += fmt.Sprintf("  (%d/%d)", m.configFileScroll+1, total)
	}
	return content + styleDim.Render(bar) + "\n"
}

func (m model) configRecommendView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Recommendations") + "\n\n")

	for i, rec := range m.configRecommendations {
		cursor := "  "
		if m.configRecommendCursor == i {
			cursor = styleSelected.Render("> ")
		}
		var check string
		if rec.applied {
			check = styleStaged.Render("[+]")
		} else {
			check = styleDim.Render("[ ]")
		}
		desc := rec.desc
		if m.configRecommendCursor == i {
			desc = styleSelected.Render(desc)
		}
		b.WriteString(cursor + "  " + check + "  " + desc + "\n")
	}
	b.WriteString("\n")

	if m.configRecommendCursor < len(m.configRecommendations) {
		rec := m.configRecommendations[m.configRecommendCursor]
		b.WriteString("  " + styleDim.Render(rec.reasoning) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter/a] apply selected  [esc] back") + "\n"
}

func (m model) configProfilesView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Git Profiles (includeIf)") + "\n\n")

	if m.configProfileStep == 0 {
		b.WriteString("  " + styleDim.Render("Conditionally load different configs based on working directory.") + "\n")
		b.WriteString("  " + styleDim.Render("Example: different email for work vs personal projects.") + "\n\n")

		if len(m.configProfiles) == 0 {
			b.WriteString("  " + styleDim.Render("(none configured)") + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("Configured profiles:") + "\n")
			for _, p := range m.configProfiles {
				b.WriteString("    " + styleDim.Render("gitdir: "+p.gitdir) + "  " + styleDim.Render("->  "+p.path) + "\n")
			}
		}
		b.WriteString("\n")

		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [n] add profile  [esc] back") + "\n"
	}

	if m.configProfileStep == 1 {
		b.WriteString("  " + styleDim.Render("Step 1/2 - Enter the gitdir pattern (e.g. ~/work/):") + "\n\n")
		b.WriteString("  " + m.configProfileInput.View() + "\n\n")

		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [enter] next  [esc] cancel") + "\n"
	}

	// step == 2
	b.WriteString("  " + styleDim.Render("Step 2/2 - Enter the path to include (e.g. ~/.gitconfig-work):") + "\n")
	b.WriteString("  " + styleDim.Render("gitdir: "+m.configProfileNewPath) + "\n\n")
	b.WriteString("  " + m.configProfileInput.View() + "\n\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] save  [esc] back") + "\n"
}

// ---------------------------------------------------------------------------
// new panel views
// ---------------------------------------------------------------------------

func (m model) fetchView() string {
	title := styleTitle.Render("Fetch")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	for i, opt := range fetchOptions {
		cursor := "  "
		if i == m.fetchCursor {
			cursor = styleSelected.Render("▶ ")
		}
		b.WriteString(cursor + opt.label + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] select  [enter] run  [esc] back") + "\n")
	return b.String()
}

func (m model) restoreView() string {
	title := styleTitle.Render("Restore File")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  file: " + styleSelected.Render(m.restoreFile) + "\n\n")
	b.WriteString("  restore to ref (commit hash, tag, or HEAD):\n")
	b.WriteString("  " + m.restoreInput.View() + "\n\n")
	b.WriteString(styleDim.Render("  [enter] restore  [esc] cancel") + "\n")
	return b.String()
}

func (m model) reflogView() string {
	title := styleTitle.Render("Reflog")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	if len(m.reflogEntries) == 0 {
		b.WriteString(styleDim.Render("  no reflog entries") + "\n")
	}
	visibleLines := m.height - 6
	start := 0
	if m.reflogCursor >= visibleLines {
		start = m.reflogCursor - visibleLines + 1
	}
	end := start + visibleLines
	if end > len(m.reflogEntries) {
		end = len(m.reflogEntries)
	}
	for i := start; i < end; i++ {
		e := m.reflogEntries[i]
		cursor := "  "
		if i == m.reflogCursor {
			cursor = styleSelected.Render("▶ ")
		}
		ref := styleDim.Render(e.Ref)
		action := styleChanged.Render(e.Action)
		b.WriteString(fmt.Sprintf("%s%s  %s  %s  %s\n", cursor, styleHash.Render(e.Hash), ref, action, e.Subject))
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] scroll  [r] reset to  [y] copy hash  [esc] back") + "\n")
	return b.String()
}

func (m model) remoteListView() string {
	title := styleTitle.Render("Remotes")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	if len(m.remotes) == 0 {
		b.WriteString(styleDim.Render("  no remotes configured") + "\n")
	}
	for i, r := range m.remotes {
		cursor := "  "
		if i == m.remoteCursor {
			cursor = styleSelected.Render("▶ ")
		}
		b.WriteString(fmt.Sprintf("%s%-14s %s\n", cursor, styleSelected.Render(r.Name), r.FetchURL))
		if r.PushURL != r.FetchURL && r.PushURL != "" {
			b.WriteString(fmt.Sprintf("    %-14s %s (push)\n", "", styleDim.Render(r.PushURL)))
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [a] add  [d] remove  [r] rename  [esc] back") + "\n")
	return b.String()
}

func (m model) remoteAddView() string {
	title := styleTitle.Render("Add Remote")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  name:\n  " + m.remoteAddInputs[0].View() + "\n\n")
	b.WriteString("  url:\n  " + m.remoteAddInputs[1].View() + "\n\n")
	b.WriteString(styleDim.Render("  [enter] next/confirm  [esc] back") + "\n")
	return b.String()
}

func (m model) remoteRenameView() string {
	title := styleTitle.Render("Rename Remote")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString(fmt.Sprintf("  renaming: %s\n\n", styleSelected.Render(m.remoteRenameTarget)))
	b.WriteString("  new name:\n  " + m.remoteRenameInput.View() + "\n\n")
	b.WriteString(styleDim.Render("  [enter] rename  [esc] cancel") + "\n")
	return b.String()
}

func (m model) submoduleListView() string {
	title := styleTitle.Render("Submodules")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	if len(m.submodules) == 0 {
		b.WriteString(styleDim.Render("  no submodules in this repository") + "\n")
	}
	for i, s := range m.submodules {
		cursor := "  "
		if i == m.submoduleCursor {
			cursor = styleSelected.Render("▶ ")
		}
		statusIcon := " "
		switch s.Status {
		case "+":
			statusIcon = styleChanged.Render("M")
		case "-":
			statusIcon = styleAdded.Render("?")
		case "U":
			statusIcon = styleConflict.Render("!")
		}
		b.WriteString(fmt.Sprintf("%s%s %-30s %s\n", cursor, statusIcon, s.Path, styleDim.Render(s.Hash[:min(7, len(s.Hash))])))
		if s.URL != "" {
			b.WriteString(fmt.Sprintf("     %s\n", styleDim.Render(s.URL)))
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [a] add  [u] update --init  [d] deinit  [esc] back") + "\n")
	return b.String()
}

func (m model) submoduleAddView() string {
	title := styleTitle.Render("Add Submodule")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  repository url:\n  " + m.submoduleInputs[0].View() + "\n\n")
	b.WriteString("  local path (leave empty to use repo name):\n  " + m.submoduleInputs[1].View() + "\n\n")
	b.WriteString(styleDim.Render("  [enter] next/confirm  [esc] back") + "\n")
	return b.String()
}

func (m model) noteView() string {
	title := styleTitle.Render("Note")
	var b strings.Builder
	b.WriteString("\n  " + title + "  " + styleDim.Render(m.noteCommit) + "\n\n")
	if m.noteEditing {
		b.WriteString("  edit note:\n  " + m.noteInput.View() + "\n\n")
		b.WriteString(styleDim.Render("  [enter] save  [esc] cancel") + "\n")
		return b.String()
	}
	if m.noteContent == "" {
		b.WriteString(styleDim.Render("  no note for this commit") + "\n\n")
	} else {
		for _, line := range strings.Split(m.noteContent, "\n") {
			b.WriteString("  " + line + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render("  [e] edit  [d] delete  [esc] back") + "\n")
	return b.String()
}

func (m model) updateMasteryPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.masteryCursor > 0 {
			m.masteryCursor--
		}
	case "down", "j":
		if m.masteryCursor < 1 {
			m.masteryCursor++
		}
	case "enter":
		if m.masteryCursor == 0 {
			m.usage.Suppress(m.masteryKey)
		}
		m.panel = panelMain
		return m, m.doSaveUsage()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) masteryView() string {
	threshold := masteryThreshold(m.masteryKey)
	title := styleTitle.Render("Command mastered")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString(fmt.Sprintf("  You've run `git %s` %d times.\n\n", m.masteryKey, threshold))
	b.WriteString("  Stop showing tips for this command?\n\n")

	options := []string{"Yes, I've got it", "No, keep showing tips"}
	for i, opt := range options {
		cursor := "  "
		if m.masteryCursor == i {
			cursor = styleSelected.Render("▶ ")
		}
		b.WriteString(cursor + opt + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] select  [enter] confirm  [esc] keep showing") + "\n")
	return b.String()
}

func (m model) updateStashMsgPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		message := strings.TrimSpace(m.stashMsgInput.Value())
		m.panel = panelMain
		return m, m.doStashWithMsg(message)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) stashMsgView() string {
	title := styleTitle.Render("Stash changes")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  Enter a message for this stash (optional):\n")
	b.WriteString("  " + m.stashMsgInput.View() + "\n\n")
	b.WriteString(styleDim.Render("  [enter] stash  [esc] cancel") + "\n")
	return b.String()
}

func (m model) updateFileHistoryPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.fileHistoryEntries == nil {
		if msg.String() == "esc" || msg.String() == m.cfg.Keybindings.Quit || msg.String() == "ctrl+c" {
			m.panel = panelMain
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if m.fileHistoryCursor > 0 {
			m.fileHistoryCursor--
		}
	case "down", "j":
		if m.fileHistoryCursor < len(m.fileHistoryEntries)-1 {
			m.fileHistoryCursor++
		}
	case "enter":
		if m.fileHistoryCursor < len(m.fileHistoryEntries) {
			entry := m.fileHistoryEntries[m.fileHistoryCursor]
			if entry.Hash != "" {
				m.commitDetail = nil
				m.commitDetailScroll = 0
				m.commitDetailOrigin = panelFileHistory
				return m, m.doFetchCommitDetail(entry.Hash)
			}
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) fileHistoryView() string {
	title := styleTitle.Render("File history: " + m.fileHistoryPath)
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	if m.fileHistoryEntries == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		return b.String()
	}
	if len(m.fileHistoryEntries) == 0 {
		b.WriteString("  " + styleDim.Render("no commits found for this file") + "\n")
	} else {
		for i, e := range m.fileHistoryEntries {
			cursor := "  "
			if m.fileHistoryCursor == i {
				cursor = styleSelected.Render("> ")
			}
			b.WriteString(cursor + e.Line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] navigate  [enter] view commit  [esc] back") + "\n")
	return b.String()
}

func (m model) updateGraphPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.graphLines == nil {
		if msg.String() == "esc" || msg.String() == m.cfg.Keybindings.Quit || msg.String() == "ctrl+c" {
			m.panel = panelMain
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
		return m, nil
	}
	visible := m.height - 5
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(m.graphLines) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	switch msg.String() {
	case "up", "k":
		if m.graphScroll > 0 {
			m.graphScroll--
		}
	case "down", "j":
		if m.graphScroll < maxScroll {
			m.graphScroll++
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) graphView() string {
	title := styleTitle.Render("Branch graph")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	if m.graphLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		return b.String()
	}
	if len(m.graphLines) == 0 {
		b.WriteString("  " + styleDim.Render("no commits found") + "\n")
	} else {
		visible := m.height - 6
		if visible < 1 {
			visible = 1
		}
		end := m.graphScroll + visible
		if end > len(m.graphLines) {
			end = len(m.graphLines)
		}
		for _, line := range m.graphLines[m.graphScroll:end] {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] scroll  [esc] back") + "\n")
	return b.String()
}

// buildEduMgrKeys returns command keys ordered by usage count descending,
// followed by any suppressed commands not yet run (count = 0).
func buildEduMgrKeys(u *usage.Data) []string {
	type kc struct {
		key   string
		count int
	}
	var items []kc
	seen := map[string]bool{}
	for k, c := range u.Counts {
		items = append(items, kc{k, c})
		seen[k] = true
	}
	for k := range u.Suppressed {
		if !seen[k] {
			items = append(items, kc{k, 0})
			seen[k] = true
		}
	}
	// sort by count descending, then alphabetically
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && (items[j].count > items[j-1].count ||
			(items[j].count == items[j-1].count && items[j].key < items[j-1].key)); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.key
	}
	return keys
}

func (m model) updateEduMgrPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.eduMgrCursor > 0 {
			m.eduMgrCursor--
		}
	case "down", "j":
		if m.eduMgrCursor < len(m.eduMgrKeys)-1 {
			m.eduMgrCursor++
		}
	case " ", "enter":
		if m.eduMgrCursor < len(m.eduMgrKeys) {
			key := m.eduMgrKeys[m.eduMgrCursor]
			if m.usage.IsSuppressed(key) {
				delete(m.usage.Suppressed, key)
				// allow mastery prompt to appear again so user can make a fresh choice
				delete(m.usage.Prompted, key)
			} else {
				m.usage.Suppress(key)
			}
			return m, m.doSaveUsage()
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelConfigMenu
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) eduMgrView() string {
	title := styleTitle.Render("Education & Usage")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	if len(m.eduMgrKeys) == 0 {
		b.WriteString("  " + styleDim.Render("no commands tracked yet - run some git operations first") + "\n\n")
		b.WriteString(styleDim.Render("  [esc] back") + "\n")
		return b.String()
	}

	for i, key := range m.eduMgrKeys {
		cursor := "  "
		if m.eduMgrCursor == i {
			cursor = styleSelected.Render("> ")
		}
		tips := styleAdded.Render("[✓]")
		if m.usage.IsSuppressed(key) {
			tips = styleDim.Render("[ ]")
		}
		count := m.usage.Count(key)
		threshold := masteryThreshold(key)
		var countStr string
		if count >= threshold {
			countStr = styleDim.Render(fmt.Sprintf("  mastered (%d uses)", count))
		} else {
			countStr = styleDim.Render(fmt.Sprintf("  %d / %d uses", count, threshold))
		}
		b.WriteString(cursor + tips + "  " + fmt.Sprintf("%-18s", "git "+key) + countStr + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [✓] tips on  [ ] tips off  [space/enter] toggle  [esc] back") + "\n")
	return b.String()
}

func (m model) hunkStageView() string {
	action := "Stage hunks"
	if m.hunkStaged {
		action = "Unstage hunks"
	}
	title := styleTitle.Render(action + ": " + m.hunkFile)
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	if m.hunkList == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		return b.String()
	}
	if len(m.hunkList) == 0 {
		b.WriteString("  " + styleDim.Render("no hunks found") + "\n\n")
		b.WriteString(styleDim.Render("  [esc] back") + "\n")
		return b.String()
	}

	for i, h := range m.hunkList {
		cursor := "  "
		if m.hunkCursor == i {
			cursor = styleSelected.Render("▶ ")
		}
		checkbox := "[ ]"
		if i < len(m.hunkSel) && m.hunkSel[i] {
			checkbox = styleAdded.Render("[✓]")
		}
		b.WriteString(cursor + checkbox + " " + styleDim.Render(h.Header) + "\n")
		maxLines := 5
		for j, line := range h.Body {
			if j >= maxLines {
				b.WriteString("        " + styleDim.Render("...") + "\n")
				break
			}
			var styled string
			switch {
			case strings.HasPrefix(line, "+"):
				styled = styleAdded.Render(line)
			case strings.HasPrefix(line, "-"):
				styled = styleConflict.Render(line)
			default:
				styled = styleDim.Render(line)
			}
			b.WriteString("        " + styled + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(styleDim.Render("  [↑↓] navigate  [space] toggle  [a] all/none  [enter] apply  [esc] back") + "\n")
	return b.String()
}

func (m model) pushOptsView() string {
	title := styleTitle.Render("Push")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	branch := ""
	if m.status != nil {
		branch = m.status.Branch
	}
	labels := []string{
		"Push",
		"Push --force-with-lease",
		"Push --set-upstream origin " + branch,
	}
	for i, label := range labels {
		cursor := "  "
		if m.pushOptCursor == i {
			cursor = styleSelected.Render("▶ ")
		}
		b.WriteString(cursor + label + "\n")
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] select  [enter] push  [esc] back") + "\n")
	return b.String()
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
		"[h] hunks",
		"[d] diff",
		"[e] blame",
		"[H] file history",
		fmt.Sprintf("[%s] commit", kb.Commit),
		"[A] amend",
		fmt.Sprintf("[%s] push", kb.Push),
		"[P] pull",
		"[f] fetch",
		fmt.Sprintf("[%s/%s] stash", kb.Stash, strings.ToUpper(kb.Stash)),
		"[b/B] branch",
		"[l] log",
		fmt.Sprintf("[%s] graph", kb.Graph),
		"[z] reset",
		"[o] restore",
		"[L] reflog",
		"[t] tags",
		"[i] bisect",
		"[R] rebase",
		"[W] worktrees",
		"[O] remotes",
		"[M] submodules",
		"[n] notes",
		"[X] clean",
		"[a] abort",
		"[C] config",
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

	usagePath, _ := config.UsageFilePath()
	usageData, _ := usage.Load(usagePath)
	if usageData == nil {
		usageData = &usage.Data{
			Counts:     map[string]int{},
			Suppressed: map[string]bool{},
			Prompted:   map[string]bool{},
		}
	}

	m := model{
		cfg:            cfg,
		git:            g,
		logFilterInput: fi,
		usage:          usageData,
		usagePath:      usagePath,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
