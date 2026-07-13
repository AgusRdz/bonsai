package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/conventions"
	"github.com/AgusRdz/bonsai/git"
	"github.com/AgusRdz/bonsai/metrics"
	"github.com/AgusRdz/bonsai/plugins"
	"github.com/AgusRdz/bonsai/pr"
	"github.com/AgusRdz/bonsai/usage"
	chroma "github.com/alecthomas/chroma/v2"
	chromaLexers "github.com/alecthomas/chroma/v2/lexers"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const gitTimeout = 5 * time.Second
const autoRefreshInterval = 2 * time.Second

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
	panelWorktreeBaseChoice
	panelWorktreePostCreate
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
	panelStashFiles
	panelFileHistory
	panelGraph
	panelEduMgr
	panelCommandBar
	panelDiverged
	panelPR
	panelPRDetail
	panelPRReview
	panelPRCreate
	panelPRMerge
	panelIssues
	panelSSH
	panelLFS
	panelDashboard
	panelInit
	panelAbout
)

type branchMode int

const (
	branchModeCreate branchMode = iota
	branchModeRename
)

type fileItem struct {
	entry    git.FileEntry
	category int // catStaged | catChanged | catUntracked
	selected bool
}

type rebaseTodo struct {
	action string // "pick", "reword", "edit", "squash", "fixup", "drop"
	hash   string // abbreviated commit hash
	msg    string // commit subject
}

// conflictPart is one segment of a conflict file: either a block of context
// lines or a conflict hunk (ours vs theirs, plus base when available).
type conflictPart struct {
	context []string // non-nil for context segments
	ours    []string // non-nil for conflict segments
	theirs  []string
	base    []string // common ancestor lines for this hunk; nil when unavailable
}

func (p conflictPart) isConflict() bool { return p.ours != nil }

const (
	hunkUnresolved = 0
	hunkOurs       = 1
	hunkTheirs     = 2
	hunkBoth       = 3
)

// parseConflictFile splits lines into context and conflict segments.
func parseConflictFile(lines []string) []conflictPart {
	var parts []conflictPart
	var ctx []string
	state := 0 // 0=context, 1=in-ours, 2=in-theirs
	var ours, theirs []string
	for _, line := range lines {
		switch {
		case state == 0 && strings.HasPrefix(line, "<<<<<<<"):
			if len(ctx) > 0 {
				parts = append(parts, conflictPart{context: ctx})
				ctx = nil
			}
			ours = nil
			theirs = nil
			state = 1
		case state == 1 && strings.HasPrefix(line, "======="):
			state = 2
		case state == 2 && strings.HasPrefix(line, ">>>>>>>"):
			parts = append(parts, conflictPart{ours: ours, theirs: theirs})
			ours = nil
			theirs = nil
			state = 0
		case state == 1:
			ours = append(ours, line)
		case state == 2:
			theirs = append(theirs, line)
		default:
			ctx = append(ctx, line)
		}
	}
	if len(ctx) > 0 {
		parts = append(parts, conflictPart{context: ctx})
	}
	return parts
}

// resolveConflictFile rebuilds file content from parts, per-hunk resolutions,
// and optional custom content (manually edited hunks override res[i]).
func resolveConflictFile(parts []conflictPart, res []int, custom []string) []string {
	var out []string
	hi := 0
	for _, p := range parts {
		if !p.isConflict() {
			out = append(out, p.context...)
			continue
		}
		// Custom edit takes priority over ours/theirs/both.
		if hi < len(custom) && custom[hi] != "" {
			out = append(out, strings.Split(custom[hi], "\n")...)
			hi++
			continue
		}
		r := hunkUnresolved
		if hi < len(res) {
			r = res[hi]
		}
		hi++
		switch r {
		case hunkOurs:
			out = append(out, p.ours...)
		case hunkTheirs:
			out = append(out, p.theirs...)
		case hunkBoth:
			out = append(out, p.ours...)
			out = append(out, p.theirs...)
		default:
			// Keep raw markers for unresolved hunks.
			out = append(out, "<<<<<<< (yours)")
			out = append(out, p.ours...)
			out = append(out, "=======")
			out = append(out, p.theirs...)
			out = append(out, ">>>>>>> (incoming)")
		}
	}
	return out
}

// populateBaseParts fills the base field of each conflict part by scanning the
// common-ancestor (stage 1) content. It finds the base lines that correspond to
// each conflict hunk by using the surrounding context lines as anchors.
func populateBaseParts(parts []conflictPart, baseLines []string) {
	bi := 0
	for i := range parts {
		if !parts[i].isConflict() {
			// Advance bi past each context line.
			for _, cl := range parts[i].context {
				for bi < len(baseLines) && baseLines[bi] != cl {
					bi++
				}
				if bi < len(baseLines) {
					bi++
				}
			}
			continue
		}
		start := bi
		// Find end: first line of the next context block.
		end := len(baseLines)
		if i+1 < len(parts) && !parts[i+1].isConflict() && len(parts[i+1].context) > 0 {
			target := parts[i+1].context[0]
			for end = start; end < len(baseLines); end++ {
				if baseLines[end] == target {
					break
				}
			}
		}
		if start <= end && end <= len(baseLines) {
			parts[i].base = baseLines[start:end]
		}
		bi = end
	}
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

// sshKeyEntry represents one SSH key found in ~/.ssh/.
type sshKeyEntry struct {
	Name        string // filename without extension (e.g. "id_ed25519")
	PubKeyFile  string // absolute path to .pub file
	Fingerprint string // output of ssh-keygen -lf
	Comment     string // comment field from the public key
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
	cfg                   *config.Config
	version               string
	git                   *git.Runner
	status                *git.Status
	files                 []fileItem // flat list of all selectable files
	cursor                int
	panel                 panel
	commitMsg             textinput.Model
	branchInput           textinput.Model
	branchMode            branchMode
	convViolation         *conventions.Result
	convPanelShown        bool // panel already shown for current branch violation
	logEntries            []git.LogEntry
	logCursor             int
	logOffset             int             // pagination: how many commits already loaded
	logHasMore            bool            // more commits available to load
	logFilter             string          // active filter query; empty = no filter
	logFilterInput        textinput.Model // search input field
	logFiltering          bool            // search input is focused
	branches              []git.Branch
	branchCursor          int
	branchFilter          string
	branchFilterInput     textinput.Model
	branchFiltering       bool
	branchSelected        map[string]bool   // multi-select set for batch delete, keyed by branch name
	branchDeleting        bool              // a branch delete/sweep is running; shows an in-progress hint
	branchWorktrees       map[string]string // branch name -> worktree path for branches checked out elsewhere
	diffLines             []string
	diffPositions         []diffLinePos
	diffCursor            int
	diffScroll            int
	diffTitle             string
	prDiffNumber          int
	prLineCommentActive   bool
	prLineCommentInput    textinput.Model
	stashes               []git.StashEntry
	stashCursor           int
	stashFilter           string
	stashFilterInput      textinput.Model
	stashFiltering        bool
	stashFilesRef         string
	stashFilesList        []string
	stashFilesSel         []bool
	stashFilesCursor      int
	stashPendingAction    string // "apply" or "pop" — set when previewing stash diff before confirming
	confirmPrompt         string
	confirmCmd            tea.Cmd
	confirmOrigin         panel // panel to return to on cancel; zero = panelMain
	flowOptions           []flowOption
	flowPickCursor        int
	commitDetail          *git.CommitDetail
	commitDetailScroll    int
	cpRangeTarget         string // "to" hash for range cherry-pick
	cpRangeInput          textinput.Model
	cpRangeInputActive    bool
	conflictPath          string
	conflictLines         []string
	conflictScroll        int
	conflictParts         []conflictPart
	conflictHunkCursor    int
	conflictHunkRes       []int    // parallel to conflict-only parts; hunkUnresolved/Ours/Theirs/Both
	conflictCustom        []string // per-hunk custom content when edited manually (nil = not edited)
	conflictEditMode      bool
	conflictEditInput     textinput.Model
	tags                  []git.TagEntry
	tagCursor             int
	tagFilter             string
	tagFilterInput        textinput.Model
	tagFiltering          bool
	tagAnnotated          bool // true when creating an annotated tag
	tagAnnotateStep       int  // 0=name, 1=message
	tagMsgInput           textinput.Model
	worktrees             []git.WorktreeEntry
	worktreeCursor        int
	repoRoot              string
	worktreeAddStep       int             // 0=label, 1=branch name
	worktreeBranchInput   textinput.Model // step-1 branch name input
	pendingWorktreePath   string
	pendingWorktreeBranch string
	postCreatePath        string
	postCreateSetup       bool // true=first-time textarea, false=confirm existing
	postCreateTA          textarea.Model
	edu                   *educationPanel
	eduTimer              int
	width                 int
	height                int
	ready                 bool
	err                   error  // startup/refresh error
	actionErr             error  // error from last action
	lastCmd               string // last git command run
	lastInfo              string // human-readable result of last action
	pushing               bool
	pulling               bool
	committing            bool
	amending              bool
	amendProgressCh       <-chan string
	amendLog              []string
	noVerify              bool
	blameLines            []git.BlameLine
	blameScroll           int
	blameTitle            string
	bisectState           *git.BisectState
	bisectLog             string
	bisectInput           textinput.Model
	bisectInputActive     bool
	rebaseTodos           []rebaseTodo
	rebaseCursor          int
	rebaseBase            string          // e.g. "HEAD~3"
	rebaseBaseInput       textinput.Model // input for entering the base ref
	rebaseStep            int             // 0 = enter base ref, 1 = edit todo list
	amendInput            textinput.Model
	amendField            int               // 0=menu, 1=message, 2=author, 3=date
	amendDetail           *git.CommitDetail // HEAD commit shown in the panel

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
	reflogEntries     []git.ReflogEntry
	reflogCursor      int
	reflogFilter      string
	reflogFilterInput textinput.Model
	reflogFiltering   bool

	// remotes
	remotes            []git.RemoteEntry
	remoteCursor       int
	remoteAddInputs    [2]textinput.Model // [0]=name [1]=url
	remoteAddStep      int
	remoteRenameInput  textinput.Model
	remoteRenameTarget string // name of remote being renamed

	// init panel
	initFromNoRepo bool // true when panelInit was opened from a non-repo state

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
	// line-level staging (within a hunk)
	hunkLineMode   bool
	hunkLineCursor int
	hunkLineSel    []bool // parallel to hunkList[hunkCursor].Body

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
	graphLines   []string
	graphColored []string // colorized version of graphLines, cached
	graphScroll  int

	// branch rename from branch list
	branchRenameTarget string

	// smart auto-refresh: track .git/index and .git/HEAD mtimes so we only
	// call git status when the working tree or HEAD actually changed.
	gitIndexMtime time.Time
	gitHeadMtime  time.Time

	// education manager
	eduMgrKeys   []string // ordered list of command keys shown
	eduMgrCursor int

	cmdBarCursor  int
	cmdBarEnabled []bool // parallel to cmdBarCatalog; initialized on panel open

	// metrics (nil when disabled)
	mdb *metrics.DB

	// pr integration
	prProvider        pr.Provider
	prListLoading     bool
	prListErr         error
	prStatus          *pr.PRStatus
	prListItems       []pr.PRStatus
	prListCursor      int
	protectedBranches map[string]bool // branch names known to be protected on remote
	mergedBranches    map[string]bool // branch names merged into the default branch
	squashedBranches  map[string]bool // branch names whose net diff is already in the default branch (squash-merged)
	prReviewInput     textinput.Model
	prReviewMode      string // "approve" | "changes" | "comment"
	prReviewNumber    int

	// pr create form
	prCreateTitleInput textinput.Model
	prCreateBodyTA     textarea.Model
	prCreateBaseInput  textinput.Model
	prCreateField      int  // 0=title 1=body 2=base
	prCreateDraft      bool // create as draft PR

	// overview (shown when working tree is clean)
	overviewCursor     int
	overviewLogEntries []git.LogEntry

	// pr merge picker
	prMergeNumber int
	prMergeCursor int // 0=merge 1=squash 2=rebase

	// issues
	issues           []pr.Issue
	issueCursor      int
	issueFilter      string
	issueFilterInput textinput.Model
	issueFiltering   bool

	// SSH key manager
	sshKeys        []sshKeyEntry
	sshCursor      int
	sshTestResults map[string]string // host -> "ok"/"fail"/"..."

	// LFS panel
	lfsTracked    []string // files tracked by LFS (git lfs ls-files)
	lfsPatterns   []string // patterns in .gitattributes
	lfsStatus     string   // raw output of git lfs status
	lfsInstalled  bool     // whether git-lfs is available
	lfsCursor     int      // cursor in pattern list
	lfsTracking   bool     // whether track-pattern input is active
	lfsTrackInput textinput.Model

	// Multi-repo dashboard
	dashEntries []git.DashboardEntry
	dashCursor  int
	dashLoading bool

	// undo: up to 5 reversible operations (most recent last)
	undoStack []undoEntry

	// which panel to return to when escaping the diff viewer
	diffOrigin panel

	// context for the file currently shown in the diff viewer
	diffFilePath   string
	diffFileStaged bool

	// diff search
	diffSearch        string
	diffSearchInput   textinput.Model
	diffSearching     bool
	diffSearchMatches []int
	diffSearchCursor  int
	// diff context lines
	diffContext int
	// diff word diff mode
	diffWordDiff bool
	// commit body (multi-line)
	commitBodyTA     textarea.Model
	commitBodyActive bool
}

// --- messages ---

type statusMsg *git.Status
type errMsg struct{ err error }
type actionDoneMsg struct {
	cmd  string
	err  error
	info string // optional human-readable summary shown in the footer
}

// diffActionDoneMsg is returned when a stage/unstage/discard is triggered from
// within the diff viewer. It carries the actionDoneMsg payload plus enough
// context to re-fetch the diff with the new staged state.
type diffActionDoneMsg struct {
	actionDoneMsg
	path   string
	staged bool // new staged state to display after the action
	close  bool // true when the file is gone and the panel should close
}
type branchListMsg []git.Branch
type protectedBranchesMsg map[string]bool
type mergedBranchesMsg map[string]bool
type squashedBranchesMsg map[string]bool
type issueListMsg struct {
	items []pr.Issue
	err   error
}

type sshKeyListMsg []sshKeyEntry

type sshTestMsg struct {
	host   string
	result string
}

type lfsDataMsg struct {
	tracked   []string // files tracked (git lfs ls-files)
	patterns  []string // patterns configured in .gitattributes
	status    string   // raw git lfs status output
	installed bool     // whether git-lfs binary is available
}

type dashboardMsg []git.DashboardEntry

type diffMsg struct {
	title    string
	lines    []string
	wordDiff bool // true when loaded with --word-diff=plain; skips syntax highlighting
}

type diffLinePos struct {
	file     string // empty = not commentable (header lines)
	position int    // 0 = not commentable
	newLine  int    // 1-based line number in the new file; 0 = unknown
}

// undoEntry records a reversible operation for multi-level undo.
type undoEntry struct {
	cmd  tea.Cmd
	desc string
}
type stashListMsg []git.StashEntry

type stashFilesMsg struct {
	ref   string
	files []string
}
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

type conflictVersionsMsg struct {
	base   []string
	ours   []string
	theirs []string
}

type tagListMsg []git.TagEntry

type worktreeListMsg []git.WorktreeEntry

// branchWorktreesMsg carries a branch-name -> worktree-path map for the
// branches panel; unlike worktreeListMsg it never switches panels.
type branchWorktreesMsg map[string]string

type worktreeCreatedMsg struct {
	path   string
	branch string
	err    error
	cmd    string
}

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
type amendProgressMsg string
type amendProgressDoneMsg struct{}

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

type prStatusMsg struct {
	status *pr.PRStatus
	err    error
}
type prListMsg struct {
	items []pr.PRStatus
	err   error
}

type prCreatePrefillMsg struct {
	subject string
	body    string
}

type prMergeResultMsg struct {
	cmd  string
	info string
	err  error
}

type overviewLogMsg []git.LogEntry

type tickMsg time.Time

func autoRefreshCmd() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// gitRepoChanged returns the latest mtimes of .git/index and .git/HEAD.
// Returns zero times on error (e.g. not in a git repo yet).
func gitRepoChanged() (indexMtime, headMtime time.Time) {
	if info, err := os.Stat(".git/index"); err == nil {
		indexMtime = info.ModTime()
	}
	if info, err := os.Stat(".git/HEAD"); err == nil {
		headMtime = info.ModTime()
	}
	return
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
	branch := ""
	if m.status != nil {
		branch = m.status.Branch
	}
	sign := m.cfg.Signing.Enabled
	key := m.cfg.Signing.Key
	noVerify := m.noVerify
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		if sign {
			err = m.git.CommitSigned(ctx, msg, key, noVerify)
		} else {
			err = m.git.Commit(ctx, msg, noVerify)
		}
		var info string
		if err == nil {
			info = "committed to " + branch
			if sign {
				info += " (signed)"
			}
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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
	ahead := 0
	if m.status != nil {
		ahead = m.status.Ahead
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.PushWithOptions(ctx, opt.force, opt.setUpstream, remote, branch)
		var info string
		if err == nil {
			switch {
			case opt.force:
				info = fmt.Sprintf("force-pushed %s to %s", branch, remote)
			case opt.setUpstream:
				info = fmt.Sprintf("pushed and set upstream to %s/%s", remote, branch)
			case ahead > 0:
				info = fmt.Sprintf("pushed %d commit(s) to %s/%s", ahead, remote, branch)
			default:
				info = fmt.Sprintf("pushed to %s/%s", remote, branch)
			}
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doSaveUsage() tea.Cmd {
	data := m.usage
	path := m.usagePath
	return func() tea.Msg {
		// Best-effort by design: usage.json is command-frequency telemetry that
		// drives the education/mastery prompts. A failed write must never nag or
		// interrupt the user, so the error is intentionally dropped.
		_ = data.Save(path)
		return nil
	}
}

func (m model) doStashWithMsg(msg string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashWithMessage(ctx, msg)
		var info string
		if err == nil {
			info = "stashed changes"
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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
		out, err := m.git.Graph(ctx, 150)
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
		var info string
		if err == nil {
			info = "applied " + ref + " (stash kept)"
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doStashDrop(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashDrop(ctx, ref)
		var info string
		if err == nil {
			info = "dropped " + ref
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doDeleteBranch(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		// Try safe delete first; fall back to force if branch has unmerged work.
		err := m.git.DeleteBranch(ctx, name, false)
		if err != nil {
			err = m.git.DeleteBranch(ctx, name, true)
		}
		if err != nil {
			return actionDoneMsg{cmd: "git branch -D " + name, err: err}
		}
		return actionDoneMsg{cmd: "git branch -D " + name, err: nil, info: "deleted branch " + name}
	}
}

func (m model) doDeleteRemoteBranch(remote, branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DeleteRemoteBranch(ctx, remote, branch)
		var info string
		if err == nil {
			info = "deleted " + remote + "/" + branch
		}
		return actionDoneMsg{cmd: "git push " + remote + " --delete " + branch, err: err, info: info}
	}
}

func (m model) doDeleteBranchesMany(branches []git.Branch, worktrees map[string]string) tea.Cmd {
	return func() tea.Msg {
		deleted := 0
		var lastErr error
		for _, b := range branches {
			// Per-deletion timeout so a large batch can't expire a shared
			// context partway through and silently drop the tail.
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			// If the branch is checked out in a worktree, remove that worktree
			// first (pruning a stale ref if the dir is gone) — git won't delete
			// a branch still used by a worktree.
			if path, ok := worktrees[b.Name]; ok {
				if err := m.git.RemoveWorktree(ctx, path); err != nil && isStaleWorktreeErr(err) {
					_ = m.git.PruneWorktrees(ctx)
				}
			}
			// Safe delete first; fall back to force for unmerged work.
			err := m.git.DeleteBranch(ctx, b.Name, false)
			if err != nil {
				err = m.git.DeleteBranch(ctx, b.Name, true)
			}
			cancel()
			if err != nil {
				lastErr = err
			} else {
				deleted++
			}
		}
		info := ""
		if deleted > 0 {
			info = fmt.Sprintf("deleted %d branch(es)", deleted)
		}
		return actionDoneMsg{cmd: "git branch -D (batch)", err: lastErr, info: info}
	}
}

func (m model) doDeleteGoneBranches(branches []git.Branch) tea.Cmd {
	return func() tea.Msg {
		deleted := 0
		var lastErr error
		for _, b := range branches {
			// Each deletion gets its own timeout — a single shared context
			// sized to gitTimeout would expire partway through a large sweep
			// and silently drop the remaining branches.
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			err := m.git.DeleteBranch(ctx, b.Name, true)
			cancel()
			if err != nil {
				lastErr = err
			} else {
				deleted++
			}
		}
		info := ""
		if deleted > 0 {
			info = fmt.Sprintf("deleted %d gone branch(es)", deleted)
		}
		return actionDoneMsg{cmd: "git branch -D (gone)", err: lastErr, info: info}
	}
}

func (m model) doRenameBranch(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RenameBranch(ctx, oldName, newName)
		var info string
		if err == nil {
			info = "renamed " + oldName + " to " + newName
		}
		return actionDoneMsg{cmd: "git branch -m " + oldName + " " + newName, err: err, info: info}
	}
}

func (m model) doPushTag(remote, tag string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.PushTag(ctx, remote, tag)
		var info string
		if err == nil {
			info = "pushed tag " + tag + " to " + remote
		}
		return actionDoneMsg{cmd: "git push " + remote + " " + tag, err: err, info: info}
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

func (m model) doApplyLines() tea.Cmd {
	hdr := m.hunkFileHdr
	hunk := m.hunkList[m.hunkCursor]
	sel := make([]bool, len(m.hunkLineSel))
	copy(sel, m.hunkLineSel)
	reverse := m.hunkStaged
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.ApplyLines(ctx, hdr, hunk, sel, reverse)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doCreateBranch(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CreateBranch(ctx, name)
		var info string
		if err == nil {
			info = "created and switched to " + name
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doRename(name string) tea.Cmd {
	old := ""
	if m.status != nil {
		old = m.status.Branch
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Rename(ctx, name)
		var info string
		if err == nil {
			info = "renamed " + old + " to " + name
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doPull() tea.Cmd {
	behind := 0
	if m.status != nil {
		behind = m.status.Behind
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.Pull(ctx)
		var info string
		if err == nil {
			if behind > 0 {
				info = fmt.Sprintf("pulled %d new commit(s) from remote", behind)
			} else {
				info = "already up to date"
			}
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doPullRebase() tea.Cmd {
	ahead, behind := 0, 0
	if m.status != nil {
		ahead = m.status.Ahead
		behind = m.status.Behind
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.PullRebase(ctx)
		var info string
		if err == nil {
			info = fmt.Sprintf("rebased %d local commit(s) on top of %d remote commit(s)", ahead, behind)
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doPullMerge() tea.Cmd {
	ahead, behind := 0, 0
	if m.status != nil {
		ahead = m.status.Ahead
		behind = m.status.Behind
	}
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.PullMerge(ctx)
		var info string
		if err == nil {
			info = fmt.Sprintf("merged %d remote commit(s) into %d local commit(s)", behind, ahead)
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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
		var info string
		if err == nil {
			info = "switched to " + name
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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
		content, err := m.git.Diff(ctx, path, staged, m.diffContext)
		if err != nil || content == "" {
			return diffMsg{title: title, lines: []string{}}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: title, lines: lines}
	}
}

// doStageFromDiff stages or unstages path, then closes the diff viewer.
// stage=true means git add, false means git restore.
func (m model) doStageFromDiff(path string, stage bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		var err error
		if stage {
			err = m.git.Add(ctx, path)
		} else {
			err = m.git.Restore(ctx, path)
		}
		return diffActionDoneMsg{
			actionDoneMsg: actionDoneMsg{cmd: m.git.LastCmd(), err: err},
			path:          path,
			staged:        stage,
			close:         true,
		}
	}
}

// doDiscardFromDiff discards working-tree changes for path and signals the diff
// viewer to close (the file will have no diff after discard).
func (m model) doDiscardFromDiff(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Discard(ctx, path)
		return diffActionDoneMsg{
			actionDoneMsg: actionDoneMsg{cmd: m.git.LastCmd(), err: err},
			path:          path,
			close:         true,
		}
	}
}

func (m model) doStashPop(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashPop(ctx, ref)
		var info string
		if err == nil {
			info = "popped " + ref
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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

func (m model) doDiscardAll(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DiscardAll(ctx, path)
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
	}
}

func (m model) doDeleteFromDisk(path string) tea.Cmd {
	return func() tea.Msg {
		err := os.RemoveAll(strings.TrimSuffix(path, "/"))
		if err != nil {
			return actionDoneMsg{cmd: "rm " + path, err: err}
		}
		return actionDoneMsg{cmd: "rm " + path, err: nil, info: "deleted " + path + " from disk"}
	}
}

func (m model) doGitRmCached(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RmCached(ctx, path)
		var info string
		if err == nil {
			info = path + " untracked (file kept on disk)"
		}
		return actionDoneMsg{cmd: "git rm --cached " + path, err: err, info: info}
	}
}

func (m model) doAddMany(paths []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Add(ctx, paths...)
		info := ""
		if err == nil {
			info = fmt.Sprintf("staged %d files", len(paths))
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doRestoreMany(paths []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Restore(ctx, paths...)
		info := ""
		if err == nil {
			info = fmt.Sprintf("unstaged %d files", len(paths))
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doGitRmCachedMany(paths []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		for _, p := range paths {
			if err := m.git.RmCached(ctx, p); err != nil {
				return actionDoneMsg{cmd: "git rm --cached", err: err}
			}
		}
		return actionDoneMsg{cmd: "git rm --cached", info: fmt.Sprintf("untracked %d files (files kept on disk)", len(paths))}
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
		// StashList returns a nil slice when no stashes remain (e.g. after
		// dropping the last one). The panel treats a nil m.stashes as the
		// "loading..." sentinel, so normalize to a non-nil empty slice —
		// otherwise the list hangs on "loading..." instead of "no stashes".
		if entries == nil {
			entries = []git.StashEntry{}
		}
		return stashListMsg(entries)
	}
}

func (m model) doFetchStashDiff(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		content, err := m.git.StashShow(ctx, ref)
		if err != nil || content == "" {
			return diffMsg{title: ref, lines: []string{}}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: ref, lines: lines}
	}
}

func (m model) doFetchStashFiles(ref string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		files, err := m.git.StashShowFiles(ctx, ref)
		if err != nil {
			return stashFilesMsg{ref: ref, files: nil}
		}
		return stashFilesMsg{ref: ref, files: files}
	}
}

func (m model) doStashCheckoutFiles(ref string, files []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.StashCheckoutFiles(ctx, ref, files)
		return actionDoneMsg{cmd: "git checkout " + ref + " -- <files>", err: err}
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

func (m model) doLoadConflictVersions(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		base, ours, theirs := m.git.ConflictVersions(ctx, path)
		return conflictVersionsMsg{base: base, ours: ours, theirs: theirs}
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
		var info string
		if err == nil {
			info = "created tag " + name
		}
		return actionDoneMsg{cmd: "git tag " + name, err: err, info: info}
	}
}

func (m model) doCreateAnnotatedTag(name, message string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CreateAnnotatedTag(ctx, name, message)
		var info string
		if err == nil {
			info = "created annotated tag " + name
		}
		return actionDoneMsg{cmd: "git tag -a " + name + " -m " + message, err: err, info: info}
	}
}

func (m model) doDeleteTag(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.DeleteTag(ctx, name)
		var info string
		if err == nil {
			info = "deleted tag " + name
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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

// doLoadBranchWorktrees builds a branch-name -> worktree-path map so the
// branches panel can flag branches checked out in another worktree and offer to
// remove that worktree before deleting. Best-effort and panel-neutral: a failure
// yields an empty map, which simply disables the enhancement.
func (m model) doLoadBranchWorktrees() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		entries, err := m.git.Worktrees(ctx)
		if err != nil {
			return branchWorktreesMsg(map[string]string{})
		}
		out := make(map[string]string)
		for _, wt := range entries {
			// The main worktree's branch is the current one (undeletable), and
			// detached HEADs have no branch to map.
			if wt.Current || wt.Branch == "" || wt.Branch == "(detached)" {
				continue
			}
			out[wt.Branch] = wt.Path
		}
		return branchWorktreesMsg(out)
	}
}

// isStaleWorktreeErr reports whether a `git worktree remove` failure is caused
// by the working directory already being gone (a stale admin ref), which
// `git worktree prune` can clean up. LC_ALL=C makes these messages deterministic.
func isStaleWorktreeErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "is not a working tree") ||
		strings.Contains(s, "not a working tree") ||
		strings.Contains(s, "no such file or directory")
}

// doRemoveWorktreeThenDeleteBranch removes the worktree holding a branch (pruning
// a stale admin ref if the directory is already gone) and then deletes the
// branch. git refuses `branch -d/-D` while the branch is used by a worktree, so
// the two steps must happen together.
func (m model) doRemoveWorktreeThenDeleteBranch(path, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		if err := m.git.RemoveWorktree(ctx, path); err != nil && isStaleWorktreeErr(err) {
			_ = m.git.PruneWorktrees(ctx)
		}
		err := m.git.DeleteBranch(ctx, name, false)
		if err != nil {
			err = m.git.DeleteBranch(ctx, name, true)
		}
		info := ""
		if err == nil {
			info = "removed worktree and deleted branch " + name
		}
		return actionDoneMsg{cmd: "git branch -D " + name, err: err, info: info}
	}
}

func (m model) doAddWorktree(path, branch, base string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.AddWorktree(ctx, path, branch, base)
		return worktreeCreatedMsg{path: path, branch: branch, err: err, cmd: m.git.LastCmd()}
	}
}

// lockPIDRe extracts the owning process id from a worktree lock reason such as
// "claude agent agent-a65aadfc4b09df79a (pid 9500)".
var lockPIDRe = regexp.MustCompile(`(?i)pid[:= ]*(\d+)`)

// lockOwnerAlive reports whether a locked worktree's owning process is still
// running. The bool is false when the reason carries no parseable pid — the
// caller must then decide how to treat an unverifiable lock.
func lockOwnerAlive(reason string) (alive bool, pidKnown bool) {
	m := lockPIDRe.FindStringSubmatch(reason)
	if m == nil {
		return false, false
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		return false, false
	}
	return processAlive(pid), true
}

func (m model) doRemoveWorktree(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoveWorktree(ctx, path)
		// The working dir was already gone (deleted or moved outside bonsai):
		// `remove` refuses, but `prune` clears the stale admin ref. Fall back so
		// the user doesn't have to know the difference between the two.
		if err != nil && isStaleWorktreeErr(err) {
			if perr := m.git.PruneWorktrees(ctx); perr == nil {
				return actionDoneMsg{cmd: "git worktree prune", err: nil, info: "pruned stale worktree ref (" + path + ")"}
			}
		}
		var info string
		if err == nil {
			info = "removed worktree at " + path
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

// doForceRemoveWorktree removes a locked worktree with a double --force. Only
// reached after the panel has confirmed the lock is stale (owner dead) or
// unverifiable and the user has explicitly accepted the override.
func (m model) doForceRemoveWorktree(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoveWorktreeLocked(ctx, path)
		var info string
		if err == nil {
			info = "force-removed locked worktree at " + path
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doRemoveMergedWorktrees(entries []git.WorktreeEntry) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		removed, skipped := 0, 0
		var lastErr error
		for _, wt := range entries {
			var err error
			if wt.Locked {
				// Never force-remove a worktree whose owning process is still
				// alive — an agent may be working in it. Skip it and let the
				// user handle it deliberately from the single-remove path.
				if alive, known := lockOwnerAlive(wt.LockReason); known && alive {
					skipped++
					continue
				}
				err = m.git.RemoveWorktreeLocked(ctx, wt.Path)
			} else {
				err = m.git.RemoveWorktree(ctx, wt.Path)
			}
			if err != nil {
				lastErr = err
			} else {
				removed++
			}
		}
		info := ""
		if lastErr == nil {
			info = fmt.Sprintf("removed %d merged worktree(s)", removed)
			if skipped > 0 {
				info += fmt.Sprintf(" (%d skipped — locked by a live process)", skipped)
			}
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: lastErr, info: info}
	}
}

func (m model) doRunPostCreate(worktreePath string, cmds []string) tea.Cmd {
	// Resolve $BONSAI_MAIN_WORKTREE before handing off to goroutine.
	mainPath := ""
	for _, wt := range m.worktrees {
		if wt.Current {
			mainPath = wt.Path
			break
		}
	}
	return func() tea.Msg {
		for _, raw := range cmds {
			expanded := strings.ReplaceAll(raw, "$BONSAI_MAIN_WORKTREE", mainPath)
			c := exec.Command("sh", "-c", expanded)
			c.Dir = worktreePath
			c.Env = os.Environ()
			if out, err := c.CombinedOutput(); err != nil {
				return actionDoneMsg{
					cmd: expanded,
					err: fmt.Errorf("%s\n%s", err.Error(), strings.TrimSpace(string(out))),
				}
			}
		}
		return actionDoneMsg{
			cmd:  strings.Join(cmds, " && "),
			info: fmt.Sprintf("post-create: %d command(s) completed", len(cmds)),
		}
	}
}

func (m model) doPruneWorktrees() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.PruneWorktrees(ctx)
		var info string
		if err == nil {
			info = "stale worktree refs pruned"
		}
		return actionDoneMsg{cmd: "git worktree prune", err: err, info: info}
	}
}

func (m model) doBlame(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		lines, err := m.git.Blame(ctx, path, 0, 0)
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
		var info string
		if err == nil {
			info = "merged " + branch
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doCherryPick(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CherryPick(ctx, hash)
		var info string
		if err == nil {
			info = "cherry-picked " + hash
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doCherryPickRange(from, to string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.CherryPickRange(ctx, from, to)
		var info string
		rangeSpec := from + ".." + to
		if err == nil {
			info = "cherry-picked range " + rangeSpec
		}
		return actionDoneMsg{cmd: "git cherry-pick " + rangeSpec, err: err, info: info}
	}
}

func (m model) doReset(mode string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Reset(ctx, mode)
		var info string
		if err == nil {
			info = "reset last commit (--" + mode + ")"
		}
		return actionDoneMsg{cmd: "git reset --" + mode + " HEAD~1", err: err, info: info}
	}
}

func (m model) doResetOrig() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.ResetOrig(ctx)
		var info string
		if err == nil {
			info = "reset to ORIG_HEAD"
		}
		return actionDoneMsg{cmd: "git reset --hard ORIG_HEAD", err: err, info: info}
	}
}

func (m model) doRebase(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Rebase(ctx, branch)
		var info string
		if err == nil {
			info = "rebased onto " + branch
		}
		return actionDoneMsg{cmd: "git rebase " + branch, err: err, info: info}
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

func (m model) doRevert(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Revert(ctx, hash)
		var info string
		if err == nil {
			info = "reverted " + hash
		}
		return actionDoneMsg{cmd: "git revert --no-edit " + hash, err: err, info: info}
	}
}

func (m model) doRevertContinue() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RevertContinue(ctx)
		return actionDoneMsg{cmd: "git revert --continue", err: err}
	}
}

func (m model) doRevertAbort() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RevertAbort(ctx)
		return actionDoneMsg{cmd: "git revert --abort", err: err}
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

func (m model) doBisectSkip(hash string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.BisectSkip(ctx, hash)
		cmd := "git bisect skip"
		if hash != "" {
			cmd += " " + hash
		}
		return bisectActionMsg{cmd: cmd, err: err}
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
		var info string
		if err == nil {
			info = fmt.Sprintf("interactive rebase of %d commit(s) onto %s complete", len(todos), base)
		}
		return actionDoneMsg{cmd: "git rebase -i " + base, err: err, info: info}
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
	noVerify := m.noVerify
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendMessage(ctx, msg, noVerify)
		var info string
		if err == nil {
			info = "amended commit message"
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doAmendAuthor(author string) tea.Cmd {
	noVerify := m.noVerify
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendAuthor(ctx, author, noVerify)
		var info string
		if err == nil {
			info = "amended commit author to " + author
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doAmendDate(date string) tea.Cmd {
	noVerify := m.noVerify
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendDate(ctx, date, noVerify)
		var info string
		if err == nil {
			info = "amended commit date to " + date
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func listenAmendProgress(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return amendProgressDoneMsg{}
		}
		return amendProgressMsg(line)
	}
}

func (m model) doAmendNoEditStream(ch chan<- string) tea.Cmd {
	noVerify := m.noVerify
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.AmendNoEditStream(ctx, ch, noVerify)
		var info string
		if err == nil {
			info = "amended last commit (staged changes added)"
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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

// doOpenEditorAtLine opens path at the given 1-based line number using the
// configured editor, using editor-specific line flags.
func (m model) doOpenEditorAtLine(path string, line int) tea.Cmd {
	editorStr := config.ResolveEditor(m.cfg)
	parts := strings.Fields(editorStr)
	if len(parts) == 0 {
		parts = []string{"vi"}
	}
	bin := parts[0]
	binBase := filepath.Base(bin)
	// Strip .exe suffix on Windows for matching
	binBase = strings.TrimSuffix(binBase, ".exe")
	var args []string
	switch binBase {
	case "hx", "helix":
		// hx file:line
		args = append(parts[1:], fmt.Sprintf("%s:%d", path, line))
	case "code", "codium", "vscodium":
		// code --goto file:line
		args = append(parts[1:], "--goto", fmt.Sprintf("%s:%d", path, line))
	case "subl", "sublime_text":
		// subl file:line
		args = append(parts[1:], fmt.Sprintf("%s:%d", path, line))
	case "zed":
		// zed file:line
		args = append(parts[1:], fmt.Sprintf("%s:%d", path, line))
	case "emacs", "emacsclient":
		// emacs +line file
		if line > 0 {
			args = append(parts[1:], fmt.Sprintf("+%d", line), path)
		} else {
			args = append(parts[1:], path)
		}
	case "idea", "goland", "pycharm", "webstorm", "clion", "rider":
		// JetBrains: idea --line LINE file
		if line > 0 {
			args = append(parts[1:], "--line", fmt.Sprintf("%d", line), path)
		} else {
			args = append(parts[1:], path)
		}
	default:
		// vim, nvim, vi, nano, and others that accept +LINE file
		if line > 0 {
			args = append(parts[1:], fmt.Sprintf("+%d", line), path)
		} else {
			args = append(parts[1:], path)
		}
	}
	cmd := exec.Command(bin, args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

// doCompareDiff loads the diff between HEAD and the given branch name.
func (m model) doCompareDiff(branch string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		title := "HEAD → " + branch
		content, err := m.git.DiffRange(ctx, "HEAD", branch, false, m.diffContext, nil)
		if err != nil || content == "" {
			return diffMsg{title: title, lines: []string{}}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: title, lines: lines}
	}
}

// doFetchWordDiff loads a word-level diff for path using --word-diff=plain.
func (m model) doFetchWordDiff(path string, staged bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		title := path
		if staged {
			title += "  (staged)"
		}
		content, err := m.git.DiffWordDiff(ctx, path, staged, m.diffContext)
		if err != nil || content == "" {
			return diffMsg{title: title, lines: []string{}, wordDiff: true}
		}
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		return diffMsg{title: title, lines: lines, wordDiff: true}
	}
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
		var info string
		if err == nil {
			info = "applied " + key + " = " + value
		}
		return actionDoneMsg{cmd: "git config --global " + key + " " + value, err: err, info: info}
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
		ctx := context.Background()
		err := m.git.Fetch(ctx, all, prune)
		cmd := "git fetch"
		if all {
			cmd += " --all"
		}
		if prune {
			cmd += " --prune"
		}
		var info string
		if err == nil {
			switch {
			case all && prune:
				info = "fetched all remotes (pruned stale branches)"
			case all:
				info = "fetched all remotes"
			case prune:
				info = "fetched from remote (pruned stale branches)"
			default:
				info = "fetched from remote"
			}
		}
		return actionDoneMsg{cmd: cmd, err: err, info: info}
	}
}

func (m model) doRestoreFile(path, source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RestoreFile(ctx, path, source, false)
		var info string
		if err == nil {
			info = "restored " + path + " from " + source
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
	}
}

func (m model) doRestoreAndStage(path, source string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		if err := m.git.RestoreFile(ctx, path, source, false); err != nil {
			return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
		}
		err := m.git.Add(ctx, path)
		var info string
		if err == nil {
			info = "restored " + path + " from " + source + " and staged"
		}
		return actionDoneMsg{cmd: m.git.LastCmd(), err: err, info: info}
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
		var info string
		if err == nil {
			info = "removed untracked files and directories"
		}
		return actionDoneMsg{cmd: "git clean -fd", err: err, info: info}
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

// sanitizeRemoteURL strips a leading "git remote add <name>" prefix so users
// can safely paste the full command into the URL field.
// Returns the cleaned URL and, if extracted from the command, the remote name.
func sanitizeRemoteURL(raw string) (url, extractedName string) {
	s := strings.TrimSpace(raw)
	// Match: git remote add <name> <url>
	if after, ok := strings.CutPrefix(s, "git remote add "); ok {
		parts := strings.SplitN(strings.TrimSpace(after), " ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
		}
	}
	return s, ""
}

func (m model) doRemoteAdd(name, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteAdd(ctx, name, url)
		var info string
		if err == nil {
			info = "added remote " + name
		}
		return actionDoneMsg{cmd: "git remote add " + name + " " + url, err: err, info: info}
	}
}

func (m model) doRemoteRemove(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteRemove(ctx, name)
		var info string
		if err == nil {
			info = "removed remote " + name
		}
		return actionDoneMsg{cmd: "git remote remove " + name, err: err, info: info}
	}
}

func (m model) doRemoteRename(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemoteRename(ctx, oldName, newName)
		var info string
		if err == nil {
			info = "renamed remote " + oldName + " to " + newName
		}
		return actionDoneMsg{cmd: "git remote rename " + oldName + " " + newName, err: err, info: info}
	}
}

func (m model) doRemotePrune(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.RemotePrune(ctx, name)
		var info string
		if err == nil {
			info = "pruned stale tracking refs for " + name
		}
		return actionDoneMsg{cmd: "git remote prune " + name, err: err, info: info}
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
		ctx := context.Background()
		err := m.git.SubmoduleAdd(ctx, url, path)
		var info string
		if err == nil {
			info = "added submodule at " + path
		}
		return actionDoneMsg{cmd: "git submodule add " + url, err: err, info: info}
	}
}

func (m model) doSubmoduleUpdate(init bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.git.SubmoduleUpdate(ctx, init)
		cmd := "git submodule update"
		if init {
			cmd += " --init"
		}
		var info string
		if err == nil {
			if init {
				info = "submodules initialized and updated"
			} else {
				info = "submodules updated"
			}
		}
		return actionDoneMsg{cmd: cmd, err: err, info: info}
	}
}

func (m model) doSubmoduleDeinit(path string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.SubmoduleDeinit(ctx, path)
		var info string
		if err == nil {
			info = "removed submodule " + path
		}
		return actionDoneMsg{cmd: "git submodule deinit " + path, err: err, info: info}
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
		var info string
		if err == nil {
			info = "note saved on " + commit
		}
		return actionDoneMsg{cmd: "git notes add -m " + commit, err: err, info: info}
	}
}

func (m model) doNoteRemove(commit string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.NoteRemove(ctx, commit)
		var info string
		if err == nil {
			info = "note removed from " + commit
		}
		return actionDoneMsg{cmd: "git notes remove " + commit, err: err, info: info}
	}
}

// --- init ---

func (m model) Init() tea.Cmd {
	if m.mdb != nil && m.cfg.Metrics.Track.Habits {
		repo, _ := os.Getwd()
		_ = m.mdb.RecordHabit(repo)
	}
	return tea.Batch(m.fetchStatus(), textinput.Blink, autoRefreshCmd())
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m = m.scrollUp()
			case tea.MouseButtonWheelDown:
				m = m.scrollDown()
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		// Silently refresh status on the main panel only when the working tree
		// or HEAD actually changed, avoiding spurious git status calls.
		if m.panel == panelMain && !m.pushing && !m.pulling && !m.committing {
			idx, head := gitRepoChanged()
			if idx != m.gitIndexMtime || head != m.gitHeadMtime {
				m.gitIndexMtime = idx
				m.gitHeadMtime = head
				return m, tea.Batch(m.fetchStatus(), autoRefreshCmd())
			}
		}
		return m, autoRefreshCmd()

	case statusMsg:
		prevBranch := ""
		if m.status != nil {
			prevBranch = m.status.Branch
		}
		initialLoad := !m.ready
		m.status = msg

		// Save anchor before rebuilding so cursor survives a stage/unstage that
		// moves the current file into a different section of the list.
		var anchorPath string
		var anchorCat int
		var anchorCatIdx int
		if !initialLoad && m.cursor < len(m.files) {
			anchorPath = m.files[m.cursor].entry.Path
			anchorCat = m.files[m.cursor].category
			for i := 0; i < m.cursor; i++ {
				if m.files[i].category == anchorCat {
					anchorCatIdx++
				}
			}
		}

		m.files = buildFileList(msg)

		// Restore cursor: exact path match first; if the file moved categories
		// (staged/unstaged), land on the same index within the original category.
		if anchorPath != "" {
			found := false
			for i, f := range m.files {
				if f.entry.Path == anchorPath && f.category == anchorCat {
					m.cursor = i
					found = true
					break
				}
			}
			if !found {
				var catIdxs []int
				for i, f := range m.files {
					if f.category == anchorCat {
						catIdxs = append(catIdxs, i)
					}
				}
				if len(catIdxs) > 0 {
					idx := anchorCatIdx
					if idx >= len(catIdxs) {
						idx = len(catIdxs) - 1
					}
					m.cursor = catIdxs[idx]
				} else if m.cursor >= len(m.files) {
					m.cursor = max(0, len(m.files)-1)
				}
			}
		} else if m.cursor >= len(m.files) {
			m.cursor = max(0, len(m.files)-1)
		}

		m.ready = true
		m.err = nil
		branchChanged := msg.Branch != prevBranch
		if branchChanged {
			m.convPanelShown = false
			m.overviewCursor = 0
			m.overviewLogEntries = nil
			m.prStatus = nil
		}
		if !m.convPanelShown && m.cfg.Conventions.Validation.Mode != "off" && len(m.cfg.Conventions.Branches) > 0 {
			result := conventions.Validate(msg.Branch, m.cfg.Conventions)
			if !result.Valid {
				m.convViolation = &result
				m.convPanelShown = true
				m.panel = panelConvention
				if m.mdb != nil && m.cfg.Metrics.Track.Conventions {
					repo, _ := os.Getwd()
					rule := ""
					if len(result.Rules) > 0 {
						rule = result.Rules[0].Name
					}
					_ = m.mdb.RecordViolation(repo, msg.Branch, rule)
				}
			} else {
				m.convViolation = nil
			}
		}
		// Fetch PR status only on initial load or branch change, not every tick.
		var cmds []tea.Cmd
		if initialLoad || branchChanged {
			if prCmd := m.fetchPRStatus(); prCmd != nil {
				cmds = append(cmds, prCmd)
			}
		}
		// When working tree is clean, pre-fetch PRs and log for the overview.
		if len(m.files) == 0 && config.OverviewEnabled(m.cfg) {
			if m.prProvider != nil && m.prListItems == nil {
				m.prListLoading = true
				cmds = append(cmds, m.fetchPRList())
			}
			if m.overviewLogEntries == nil {
				cmds = append(cmds, m.fetchOverviewLog())
			}
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}

	case prStatusMsg:
		if msg.err == nil {
			m.prStatus = msg.status
		}

	case prListMsg:
		if msg.err != nil {
			m.prListErr = msg.err
			m.prListItems = []pr.PRStatus{} // prevent nil-guard retrying on every tick
		} else if msg.items == nil {
			m.prListItems = []pr.PRStatus{}
		} else {
			m.prListItems = msg.items
		}
		m.prListLoading = false

	case prCreatePrefillMsg:
		if msg.subject != "" {
			m.prCreateTitleInput.SetValue(msg.subject)
		}
		if msg.body != "" {
			m.prCreateBodyTA.SetValue(msg.body)
		}
		// Re-focus the title field after SetValue so the cursor is active.
		m.prCreateTitleInput.Focus()
		m.prCreateBodyTA.Blur()
		m.prCreateBaseInput.Blur()
		m.prCreateField = 0

	case prMergeResultMsg:
		m.lastCmd = msg.cmd
		m.lastInfo = msg.info
		m.actionErr = msg.err
		if msg.err == nil {
			if m.prListCursor > 0 {
				m.prListCursor--
			}
			m.panel = panelPR
		}
		return m, m.fetchPRList()

	case errMsg:
		m.err = msg.err
		m.ready = true
		if strings.Contains(msg.err.Error(), "not a git repository") {
			m.initFromNoRepo = true
			m.panel = panelInit
		}

	case overviewLogMsg:
		m.overviewLogEntries = []git.LogEntry(msg)

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
		return m, tea.Batch(m.doLoadProtectedBranches(), m.doLoadMergedBranches(), m.doLoadSquashedBranches(), m.doLoadBranchWorktrees())

	case protectedBranchesMsg:
		m.protectedBranches = map[string]bool(msg)

	case mergedBranchesMsg:
		m.mergedBranches = map[string]bool(msg)

	case squashedBranchesMsg:
		m.squashedBranches = map[string]bool(msg)

	case issueListMsg:
		if msg.err != nil {
			m.actionErr = msg.err
		} else {
			m.issues = msg.items
			m.issueCursor = 0
		}
		m.panel = panelIssues

	case sshKeyListMsg:
		m.sshKeys = []sshKeyEntry(msg)
		m.sshCursor = 0
		m.sshTestResults = map[string]string{}
		m.panel = panelSSH

	case sshTestMsg:
		if m.sshTestResults == nil {
			m.sshTestResults = map[string]string{}
		}
		m.sshTestResults[msg.host] = msg.result

	case lfsDataMsg:
		m.lfsTracked = msg.tracked
		m.lfsPatterns = msg.patterns
		m.lfsStatus = msg.status
		m.lfsInstalled = msg.installed
		m.lfsCursor = 0
		m.lfsTracking = false
		m.panel = panelLFS

	case dashboardMsg:
		m.dashEntries = []git.DashboardEntry(msg)
		m.dashLoading = false
		m.dashCursor = 0
		m.panel = panelDashboard

	case diffMsg:
		m.diffPositions = parseDiffLinePositions(msg.lines)
		// Word diff output uses [-...-]/{+...+} markers — skip syntax highlighting
		// so chroma doesn't corrupt them.
		if msg.wordDiff {
			m.diffLines = msg.lines
		} else {
			m.diffLines = syntaxHighlightDiff(msg.lines)
		}
		m.diffWordDiff = msg.wordDiff
		m.diffTitle = msg.title
		m.diffCursor = 0
		m.diffSearch = ""
		m.diffSearchMatches = nil
		m.diffSearchCursor = 0
		m.diffSearching = false

	case stashListMsg:
		m.stashes = []git.StashEntry(msg)
		m.stashCursor = 0
		m.panel = panelStashList

	case stashFilesMsg:
		m.stashFilesRef = msg.ref
		m.stashFilesList = msg.files
		m.stashFilesSel = make([]bool, len(msg.files))
		for i := range m.stashFilesSel {
			m.stashFilesSel[i] = true
		}
		m.stashFilesCursor = 0
		m.panel = panelStashFiles

	case commitDetailMsg:
		m.commitDetail = (*git.CommitDetail)(msg)
		m.commitDetailScroll = 0
		m.panel = panelCommitDetail

	case conflictLinesMsg:
		m.conflictPath = msg.path
		m.conflictLines = msg.lines
		m.conflictScroll = 0
		parts := parseConflictFile(msg.lines)
		m.conflictParts = parts
		m.conflictHunkCursor = 0
		// Count conflict-only parts for the resolution slice.
		nHunks := 0
		for _, p := range parts {
			if p.isConflict() {
				nHunks++
			}
		}
		m.conflictHunkRes = make([]int, nHunks)
		m.conflictCustom = make([]string, nHunks)
		m.conflictEditMode = false
		m.panel = panelConflict
		return m, m.doLoadConflictVersions(msg.path)

	case conflictVersionsMsg:
		if len(msg.base) > 0 && len(m.conflictParts) > 0 {
			populateBaseParts(m.conflictParts, msg.base)
		}

	case tagListMsg:
		m.tags = []git.TagEntry(msg)
		m.tagCursor = 0
		m.panel = panelTagList

	case worktreeCreatedMsg:
		m.lastCmd = msg.cmd
		if msg.err != nil {
			m.actionErr = msg.err
			return m, m.doFetchWorktrees()
		}
		m.lastInfo = "created worktree " + msg.branch + " at " + msg.path
		m.actionErr = nil
		switch {
		case m.cfg.Worktree.PostCreate == nil:
			// First time — detect and show setup panel.
			suggested := detectPostCreateCmds(msg.path)
			m.postCreatePath = msg.path
			m.postCreateSetup = true
			ta := textarea.New()
			ta.Placeholder = "one command per line..."
			ta.SetWidth(m.width - 6)
			ta.SetHeight(8)
			ta.ShowLineNumbers = false
			ta.Focus()
			if len(suggested) > 0 {
				ta.SetValue(strings.Join(suggested, "\n"))
			}
			m.postCreateTA = ta
			m.panel = panelWorktreePostCreate
		case len(*m.cfg.Worktree.PostCreate) == 0:
			// Explicitly disabled — skip.
			m.panel = panelMain
		default:
			// Already configured — show confirm.
			m.postCreatePath = msg.path
			m.postCreateSetup = false
			m.panel = panelWorktreePostCreate
		}
		return m, m.doFetchWorktrees()

	case worktreeListMsg:
		m.worktrees = []git.WorktreeEntry(msg)
		m.worktreeCursor = 0
		if m.panel != panelWorktreePostCreate {
			m.panel = panelWorktreeList
		}

	case branchWorktreesMsg:
		m.branchWorktrees = map[string]string(msg)

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

	case amendProgressMsg:
		m.amendLog = append(m.amendLog, string(msg))
		return m, listenAmendProgress(m.amendProgressCh)

	case amendProgressDoneMsg:
		// channel closed; actionDoneMsg will arrive separately with the result

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
		m.graphColored = colorizeGraphLines(m.graphLines)
		m.graphScroll = 0
		m.panel = panelGraph

	case actionDoneMsg:
		m.pushing = false
		m.pulling = false
		m.committing = false
		m.amending = false
		m.lastCmd = msg.cmd
		m.lastInfo = msg.info
		// Enhance protected-branch push errors with an actionable message.
		if msg.err != nil && strings.Contains(msg.cmd, "push") {
			errLower := strings.ToLower(msg.err.Error())
			if strings.Contains(errLower, "protected branch") ||
				strings.Contains(errLower, "push to protected") ||
				strings.Contains(errLower, "denied") && strings.Contains(errLower, "push") {
				msg.err = fmt.Errorf("push rejected: branch is protected - open a PR instead ([K] to open PR panel)")
			}
		}
		// Enhance the "branch used by a worktree" delete error. This is the
		// fallback path (e.g. the worktree map wasn't loaded yet); the [d]
		// handler normally offers to remove the worktree up front.
		if msg.err != nil && strings.Contains(msg.cmd, "git branch -D") &&
			strings.Contains(msg.err.Error(), "used by worktree") {
			msg.err = fmt.Errorf("branch is checked out in a worktree - remove it first ([W] worktrees), or press [d] on the branch to remove both")
		}
		m.actionErr = msg.err

		// A stash drop should return to the (refreshed) stash list rather than
		// bounce out to the main panel via the education screen — so several
		// stashes can be triaged in a row. On success only; errors fall through
		// so they're displayed. stashListMsg resets the panel and cursor.
		if msg.err == nil && strings.Contains(msg.cmd, "stash drop") {
			return m, tea.Batch(m.fetchStatus(), m.doFetchStashList())
		}

		// Branch deletes (single, multi-select batch, or sweep) refresh the
		// branch list in place instead of bouncing to the main/education
		// panel — so the user keeps their spot and can triage several in a
		// row. branchListMsg re-selects panelBranchList. Errors fall through
		// here too so they're shown on the refreshed list.
		if strings.Contains(msg.cmd, "git branch -D") {
			m.branchDeleting = false
			if msg.err == nil {
				m.branchSelected = nil
			}
			return m, tea.Batch(m.fetchStatus(), m.doFetchBranches())
		}

		// Metrics tracking (best-effort, never block the TUI).
		if m.mdb != nil {
			repo, _ := os.Getwd()
			if msg.err != nil && m.cfg.Metrics.Track.Errors {
				_ = m.mdb.RecordError(msg.cmd, msg.err.Error(), "")
			}
			if msg.err == nil && m.cfg.Metrics.Track.Commits {
				if commandKey(msg.cmd) == "commit" || commandKey(msg.cmd) == "amend" {
					branch := ""
					if m.status != nil {
						branch = m.status.Branch
					}
					_ = m.mdb.RecordCommit(repo, branch, m.cfg.Modes.Default)
				}
			}
		}

		// Fire plugin events and track undoable operations.
		if msg.err == nil {
			branch := ""
			if m.status != nil {
				branch = m.status.Branch
			}
			switch commandKey(msg.cmd) {
			case "commit":
				plugins.Fire(plugins.Request{Event: plugins.EventCommitCreated, Branch: branch})
				m.undoStack = appendUndo(m.undoStack, undoEntry{cmd: m.doReset("soft"), desc: "uncommit (soft reset)"})
				m.noVerify = false
			case "amend":
				plugins.Fire(plugins.Request{Event: plugins.EventCommitCreated, Branch: branch})
				m.noVerify = false
				if m.panel == panelAmend {
					m.panel = panelMain
				}
			case "push":
				plugins.Fire(plugins.Request{Event: plugins.EventPushAfter, Branch: branch})
			case "branch":
				plugins.Fire(plugins.Request{Event: plugins.EventBranchCreated, Branch: branch})
			case "merge":
				m.undoStack = appendUndo(m.undoStack, undoEntry{cmd: m.doResetOrig(), desc: "undo merge"})
			case "rebase":
				m.undoStack = appendUndo(m.undoStack, undoEntry{cmd: m.doResetOrig(), desc: "undo rebase"})
			case "cherry-pick":
				m.undoStack = appendUndo(m.undoStack, undoEntry{cmd: m.doReset("soft"), desc: "undo cherry-pick"})
			case "revert":
				m.undoStack = appendUndo(m.undoStack, undoEntry{cmd: m.doReset("soft"), desc: "undo revert"})
			}
		}

		// After a successful git init, clear the error and offer to add a remote.
		if msg.cmd == "git init" && msg.err == nil {
			m.err = nil
			ti0 := textinput.New()
			ti0.Placeholder = "name (e.g. origin)"
			ti0.CharLimit = 64
			ti0.Width = m.width - 6
			ti0.SetValue("origin")
			ti0.Blur()
			ti1 := textinput.New()
			ti1.Placeholder = "url (e.g. git@github.com:org/repo.git)"
			ti1.CharLimit = 256
			ti1.Width = m.width - 6
			ti1.Focus()
			m.remoteAddInputs = [2]textinput.Model{ti0, ti1}
			m.remoteAddStep = 1
			m.panel = panelRemoteAdd
			return m, m.fetchStatus()
		}

		var baseCmds []tea.Cmd
		baseCmds = append(baseCmds, m.fetchStatus())

		// After a PR create, refresh the list so prListLoading clears.
		if strings.Contains(msg.cmd, "pr create") && msg.err == nil {
			baseCmds = append(baseCmds, m.fetchPRList())
		}

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

	case diffActionDoneMsg:
		m.actionErr = msg.err
		m.lastCmd = msg.actionDoneMsg.cmd
		cmds := []tea.Cmd{m.fetchStatus()}
		if msg.err == nil && m.panel == panelDiff {
			if msg.close {
				m.panel = m.diffOrigin
				m.diffOrigin = panelMain
			} else {
				m.diffFileStaged = msg.staged
				m.diffLines = nil
				m.diffScroll = 0
				cmds = append(cmds, m.doFetchDiff(msg.path, msg.staged))
			}
		}
		return m, tea.Batch(cmds...)

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
		if m.panel == panelStashFiles {
			return m.updateStashFilesPanel(msg)
		}
		if m.panel == panelHelp {
			return m.updateHelpPanel(msg)
		}
		if m.panel == panelAbout {
			return m.updateAboutPanel(msg)
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
		if m.panel == panelWorktreeBaseChoice {
			return m.updateWorktreeBaseChoicePanel(msg)
		}
		if m.panel == panelWorktreePostCreate {
			return m.updateWorktreePostCreatePanel(msg)
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
		if m.panel == panelCommandBar {
			return m.updateCommandBarPanel(msg)
		}
		if m.panel == panelDiverged {
			return m.updateDivergedPanel(msg)
		}
		if m.panel == panelPR {
			return m.updatePRPanel(msg)
		}
		if m.panel == panelPRDetail {
			return m.updatePRDetailPanel(msg)
		}
		if m.panel == panelPRReview {
			return m.updatePRReviewPanel(msg)
		}
		if m.panel == panelPRCreate {
			return m.updatePRCreatePanel(msg)
		}
		if m.panel == panelPRMerge {
			return m.updatePRMergePanel(msg)
		}
		if m.panel == panelIssues {
			return m.updateIssuesPanel(msg)
		}
		if m.panel == panelSSH {
			return m.updateSSHPanel(msg)
		}
		if m.panel == panelLFS {
			return m.updateLFSPanel(msg)
		}
		if m.panel == panelDashboard {
			return m.updateDashboardPanel(msg)
		}
		if m.panel == panelInit {
			return m.updateInitPanel(msg)
		}
		return m.updateMainPanel(msg)
	}

	if m.panel == panelCommit {
		if m.commitBodyActive {
			var cmd tea.Cmd
			m.commitBodyTA, cmd = m.commitBodyTA.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.commitMsg, cmd = m.commitMsg.Update(msg)
		return m, cmd
	}
	if m.panel == panelDiff && m.diffSearching {
		var cmd tea.Cmd
		m.diffSearchInput, cmd = m.diffSearchInput.Update(msg)
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
		if m.worktreeAddStep == 1 {
			m.worktreeBranchInput, cmd = m.worktreeBranchInput.Update(msg)
		} else {
			m.branchInput, cmd = m.branchInput.Update(msg)
		}
		return m, cmd
	}
	if m.panel == panelWorktreePostCreate && m.postCreateSetup {
		var cmd tea.Cmd
		m.postCreateTA, cmd = m.postCreateTA.Update(msg)
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
		if len(m.files) == 0 && config.OverviewEnabled(m.cfg) {
			if m.overviewCursor > 0 {
				m.overviewCursor--
			}
		} else if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if len(m.files) == 0 && config.OverviewEnabled(m.cfg) {
			total := len(m.prListItems) + len(m.overviewLogEntries)
			if m.overviewCursor < total-1 {
				m.overviewCursor++
			}
		} else if m.cursor < len(m.files)-1 {
			m.cursor++
		}

	case "+":
		// Stage all unstaged/untracked files (git add .)
		if m.status == nil || (len(m.status.Changed) == 0 && len(m.status.Untracked) == 0) {
			m.actionErr = fmt.Errorf("nothing to stage")
			break
		}
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			defer cancel()
			err := m.git.StageAll(ctx)
			return actionDoneMsg{cmd: "git add .", err: err, info: "staged all changes"}
		}

	case "tab":
		if len(m.files) == 0 {
			break
		}
		f := m.files[m.cursor]
		if f.category != catConflict {
			m.files[m.cursor].selected = !f.selected
		}
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}

	case "esc":
		// clear multi-selection if active, otherwise no-op on main panel
		hasSelection := false
		for _, f := range m.files {
			if f.selected {
				hasSelection = true
				break
			}
		}
		if hasSelection {
			for i := range m.files {
				m.files[i].selected = false
			}
		}

	case " ", "enter":
		if len(m.files) == 0 {
			if config.OverviewEnabled(m.cfg) {
				if m.overviewCursor < len(m.prListItems) && len(m.prListItems) > 0 {
					m.prListCursor = m.overviewCursor
					m.panel = panelPRDetail
				} else {
					idx := m.overviewCursor - len(m.prListItems)
					if idx >= 0 && idx < len(m.overviewLogEntries) {
						return m, m.doFetchCommitDetail(m.overviewLogEntries[idx].Hash)
					}
				}
			}
			break
		}
		// batch mode: if any files are selected, act on all of them
		var selected []fileItem
		for _, f := range m.files {
			if f.selected {
				selected = append(selected, f)
			}
		}
		if len(selected) > 0 {
			var toAdd, toRestore []string
			for _, f := range selected {
				if f.category == catConflict {
					continue
				}
				if f.category == catStaged {
					toRestore = append(toRestore, f.entry.Path)
				} else {
					toAdd = append(toAdd, f.entry.Path)
				}
			}
			for i := range m.files {
				m.files[i].selected = false
			}
			var cmds []tea.Cmd
			if len(toAdd) > 0 {
				cmds = append(cmds, m.doAddMany(toAdd))
			}
			if len(toRestore) > 0 {
				cmds = append(cmds, m.doRestoreMany(toRestore))
			}
			return m, tea.Batch(cmds...)
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
		if m.status != nil && m.status.MergeState == "revert" {
			if len(m.status.Conflicts) > 0 {
				m.actionErr = fmt.Errorf("resolve all conflicts first, then press [c] to continue the revert")
				break
			}
			return m, m.doRevertContinue()
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
		ta := textarea.New()
		ta.Placeholder = "body (optional) — tab to focus, ctrl+d to submit..."
		ta.SetWidth(m.width - 6)
		ta.SetHeight(4)
		ta.ShowLineNumbers = false
		ta.Blur()
		m.commitBodyTA = ta
		m.commitBodyActive = false
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
		m.actionErr = nil
		m.lastInfo = ""
		if m.status != nil && m.status.Ahead > 0 && m.status.Behind > 0 {
			m.panel = panelDiverged
			break
		}
		m.pulling = true
		return m, m.doPull()

	case "K":
		if m.prProvider != nil {
			m.prListItems = nil
			m.prListErr = nil
			m.prListLoading = true
			m.prListCursor = 0
			m.panel = panelPR
			return m, m.fetchPRList()
		}
	case "I":
		if m.prProvider != nil {
			if _, ok := m.prProvider.(pr.IssueProvider); ok {
				m.issues = nil
				m.issueCursor = 0
				m.issueFilter = ""
				m.issueFilterInput.SetValue("")
				return m, m.fetchIssues()
			}
		}

	case "`":
		// SSH key manager
		m.sshKeys = nil
		m.sshCursor = 0
		return m, doLoadSSHKeys()

	case "V":
		// LFS panel
		m.lfsTracked = nil
		m.lfsStatus = ""
		return m, m.doLoadLFSData()

	case "D":
		// Multi-repo dashboard
		m.dashEntries = nil
		m.dashLoading = true
		m.dashCursor = 0
		m.panel = panelDashboard
		return m, m.doLoadDashboard()

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
		m.branchSelected = nil
		m.actionErr = nil
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
		m.diffOrigin = panelMain
		m.diffFilePath = f.entry.Path
		m.diffFileStaged = f.category == catStaged
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
		// batch mode
		var xSelected []fileItem
		for _, f := range m.files {
			if f.selected {
				xSelected = append(xSelected, f)
			}
		}
		if len(xSelected) > 0 {
			var toDiscard, toDiscardStaged, toDelete []string
			for _, f := range xSelected {
				switch f.category {
				case catStaged:
					toDiscardStaged = append(toDiscardStaged, f.entry.Path)
				case catChanged:
					toDiscard = append(toDiscard, f.entry.Path)
				case catUntracked:
					toDelete = append(toDelete, f.entry.Path)
				}
			}
			allDiscard := len(toDiscard) + len(toDiscardStaged)
			if allDiscard+len(toDelete) == 0 {
				m.actionErr = fmt.Errorf("select staged, changed, or untracked files to discard/delete")
				break
			}
			desc := ""
			if allDiscard > 0 && len(toDelete) > 0 {
				desc = fmt.Sprintf("discard %d changed and delete %d untracked files? this cannot be undone", allDiscard, len(toDelete))
			} else if allDiscard > 0 {
				desc = fmt.Sprintf("discard changes to %d files? this cannot be undone", allDiscard)
			} else {
				desc = fmt.Sprintf("delete %d files from disk? this cannot be undone", len(toDelete))
			}
			for i := range m.files {
				m.files[i].selected = false
			}
			toDiscardCopy := toDiscard
			toDiscardStagedCopy := toDiscardStaged
			toDeleteCopy := toDelete
			m.confirmPrompt = desc
			m.confirmCmd = func() tea.Msg {
				if len(toDiscardCopy) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
					err := m.git.Discard(ctx, toDiscardCopy...)
					cancel()
					if err != nil {
						return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
					}
				}
				if len(toDiscardStagedCopy) > 0 {
					ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
					err := m.git.DiscardAll(ctx, toDiscardStagedCopy...)
					cancel()
					if err != nil {
						return actionDoneMsg{cmd: m.git.LastCmd(), err: err}
					}
				}
				for _, p := range toDeleteCopy {
					if err := os.RemoveAll(strings.TrimSuffix(p, "/")); err != nil {
						return actionDoneMsg{cmd: "rm " + p, err: err}
					}
				}
				n := len(toDiscardCopy) + len(toDiscardStagedCopy) + len(toDeleteCopy)
				return actionDoneMsg{cmd: "discard/delete", info: fmt.Sprintf("discarded/deleted %d files", n)}
			}
			m.panel = panelConfirm
			m.actionErr = nil
			break
		}
		f := m.files[m.cursor]
		switch f.category {
		case catStaged:
			m.confirmPrompt = fmt.Sprintf("discard all changes to %s? this cannot be undone", f.entry.Path)
			m.confirmCmd = m.doDiscardAll(f.entry.Path)
			m.panel = panelConfirm
			m.actionErr = nil
		case catChanged:
			m.confirmPrompt = fmt.Sprintf("discard all changes to %s? this cannot be undone", f.entry.Path)
			m.confirmCmd = m.doDiscard(f.entry.Path)
			m.panel = panelConfirm
			m.actionErr = nil
		case catUntracked:
			isDir := strings.HasSuffix(f.entry.Path, "/")
			if isDir {
				m.confirmPrompt = fmt.Sprintf("delete directory %s and all its contents from disk? this cannot be undone", f.entry.Path)
			} else {
				m.confirmPrompt = fmt.Sprintf("delete %s from disk? this cannot be undone", f.entry.Path)
			}
			m.confirmCmd = m.doDeleteFromDisk(f.entry.Path)
			m.panel = panelConfirm
			m.actionErr = nil
		default:
			m.actionErr = fmt.Errorf("select an unstaged changed file to discard, or an untracked file to delete from disk")
		}

	case "u":
		if len(m.files) == 0 {
			break
		}
		// batch mode
		var uSelected []fileItem
		for _, f := range m.files {
			if f.selected {
				uSelected = append(uSelected, f)
			}
		}
		if len(uSelected) > 0 {
			var toUntrack []string
			for _, f := range uSelected {
				if f.category == catStaged {
					toUntrack = append(toUntrack, f.entry.Path)
				}
			}
			if len(toUntrack) == 0 {
				m.actionErr = fmt.Errorf("select staged files to untrack (git rm --cached)")
				break
			}
			for i := range m.files {
				m.files[i].selected = false
			}
			m.confirmPrompt = fmt.Sprintf("untrack %d files? removes from git index, files stay on disk", len(toUntrack))
			m.confirmCmd = m.doGitRmCachedMany(toUntrack)
			m.panel = panelConfirm
			m.actionErr = nil
			break
		}
		f := m.files[m.cursor]
		if f.category != catStaged {
			m.actionErr = fmt.Errorf("select a staged file to untrack (git rm --cached)")
			break
		}
		m.confirmPrompt = fmt.Sprintf("untrack %s? removes from git index, file stays on disk", f.entry.Path)
		m.confirmCmd = m.doGitRmCached(f.entry.Path)
		m.panel = panelConfirm
		m.actionErr = nil

	case kb.Undo, "z":
		m.panel = panelResetPick
		m.actionErr = nil

	case "U":
		if len(m.undoStack) == 0 {
			m.actionErr = fmt.Errorf("nothing to undo - [z] for reset options")
			break
		}
		top := m.undoStack[len(m.undoStack)-1]
		m.undoStack = m.undoStack[:len(m.undoStack)-1]
		m.actionErr = nil
		return m, top.cmd

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
		case "revert":
			m.confirmPrompt = "abort revert? the operation will be cancelled"
			m.confirmCmd = m.doRevertAbort()
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
		m.amendLog = nil
		m.amending = false
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
		if len(m.files) > 0 && m.files[m.cursor].category == catConflict {
			path := m.files[m.cursor].entry.Path
			m.actionErr = nil
			return m, m.doAcceptOurs(path)
		}
		m.remotes = nil
		m.remoteCursor = 0
		m.actionErr = nil
		return m, m.doFetchRemotes()

	case "T":
		if len(m.files) > 0 && m.files[m.cursor].category == catConflict {
			path := m.files[m.cursor].entry.Path
			m.actionErr = nil
			return m, m.doAcceptTheirs(path)
		}

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

	case "F":
		if detectFlow(m.cfg) != "gitflow" || m.status == nil {
			break
		}
		branch := m.status.Branch
		bType := gitflowBranchType(branch, m.cfg)
		if bType == "" {
			m.actionErr = fmt.Errorf("current branch %q does not match any gitflow prefix", branch)
			break
		}
		mainBranch := gitflowMainBranch(m.branches)
		devBranch := gitflowDevBranch(m.branches)
		var steps string
		switch bType {
		case "feature", "bugfix":
			steps = fmt.Sprintf("merge %s to %s (--no-ff), delete %s", branch, devBranch, branch)
		case "release":
			tag := strings.TrimPrefix(branch, "release/")
			steps = fmt.Sprintf("merge %s to %s, tag %s, merge to %s, delete %s", branch, mainBranch, tag, devBranch, branch)
		case "hotfix":
			steps = fmt.Sprintf("merge %s to %s, merge to %s, delete %s", branch, mainBranch, devBranch, branch)
		}
		m.confirmPrompt = fmt.Sprintf("finish %s (%s)?\n  steps: %s", branch, bType, steps)
		m.confirmCmd = m.doFinishGitflowBranch(branch, bType, mainBranch, devBranch)
		m.panel = panelConfirm
		m.actionErr = nil
	}

	return m, nil
}

func (m model) updateCommitPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.commitBodyActive {
		// Body textarea is focused — ctrl+d submits, esc returns to subject.
		switch msg.String() {
		case "ctrl+d":
			subject := strings.TrimSpace(m.commitMsg.Value())
			if subject == "" {
				m.actionErr = fmt.Errorf("commit message cannot be empty")
				m.panel = panelMain
				return m, nil
			}
			body := strings.TrimSpace(m.commitBodyTA.Value())
			message := subject
			if body != "" {
				message = subject + "\n\n" + body
			}
			m.commitBodyActive = false
			m.panel = panelMain
			m.committing = true
			return m, m.doCommit(message)
		case "ctrl+n":
			m.noVerify = !m.noVerify
			return m, nil
		case "esc":
			m.commitBodyActive = false
			m.commitBodyTA.Blur()
			m.commitMsg.Focus()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.commitBodyTA, cmd = m.commitBodyTA.Update(msg)
			return m, cmd
		}
	}

	// Subject line is focused.
	switch msg.String() {
	case "enter":
		subject := strings.TrimSpace(m.commitMsg.Value())
		if subject == "" {
			m.actionErr = fmt.Errorf("commit message cannot be empty")
			m.panel = panelMain
			return m, nil
		}
		body := strings.TrimSpace(m.commitBodyTA.Value())
		message := subject
		if body != "" {
			message = subject + "\n\n" + body
		}
		m.panel = panelMain
		m.committing = true
		return m, m.doCommit(message)
	case "ctrl+n":
		m.noVerify = !m.noVerify
		return m, nil
	case "tab":
		m.commitBodyActive = true
		m.commitMsg.Blur()
		m.commitBodyTA.Focus()
		return m, nil
	case "esc":
		m.panel = panelMain
		m.noVerify = false
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
	// Range cherry-pick input intercepts all keys while active.
	if m.cpRangeInputActive {
		switch msg.String() {
		case "enter":
			from := strings.TrimSpace(m.cpRangeInput.Value())
			if from == "" {
				m.actionErr = fmt.Errorf("enter the exclusive start commit hash")
				return m, nil
			}
			to := m.cpRangeTarget
			m.cpRangeInputActive = false
			m.cpRangeTarget = ""
			m.actionErr = nil
			m.panel = panelMain
			return m, m.doCherryPickRange(from, to)
		case "esc":
			m.cpRangeInputActive = false
			m.cpRangeTarget = ""
			m.actionErr = nil
			return m, nil
		default:
			var cmd tea.Cmd
			m.cpRangeInput, cmd = m.cpRangeInput.Update(msg)
			return m, cmd
		}
	}
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
	case "r":
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
			m.confirmPrompt = fmt.Sprintf("revert %s on %s? (creates a new undo-commit)", short, current)
			m.confirmCmd = m.doRevert(hash)
			m.panel = panelConfirm
		}
	case "R":
		if m.commitDetail != nil && m.commitDetail.Hash != "" {
			ti := textinput.New()
			ti.Placeholder = "exclusive start hash (from..THIS — e.g. abc123)"
			ti.Focus()
			ti.CharLimit = 64
			ti.Width = m.width - 6
			m.cpRangeInput = ti
			m.cpRangeTarget = m.commitDetail.Hash
			m.cpRangeInputActive = true
			m.actionErr = nil
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

// filterBranches returns the subset of branches whose name contains q (case-insensitive).
func filterBranches(branches []git.Branch, q string) []git.Branch {
	if q == "" {
		return branches
	}
	q = strings.ToLower(q)
	var out []git.Branch
	for _, b := range branches {
		if strings.Contains(strings.ToLower(b.Name), q) {
			out = append(out, b)
		}
	}
	return out
}

func filterStashes(stashes []git.StashEntry, q string) []git.StashEntry {
	if q == "" {
		return stashes
	}
	q = strings.ToLower(q)
	var out []git.StashEntry
	for _, s := range stashes {
		if strings.Contains(strings.ToLower(s.Ref), q) || strings.Contains(strings.ToLower(s.Description), q) {
			out = append(out, s)
		}
	}
	return out
}

func filterTags(tags []git.TagEntry, q string) []git.TagEntry {
	if q == "" {
		return tags
	}
	q = strings.ToLower(q)
	var out []git.TagEntry
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	return out
}

func filterReflog(entries []git.ReflogEntry, q string) []git.ReflogEntry {
	if q == "" {
		return entries
	}
	q = strings.ToLower(q)
	var out []git.ReflogEntry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Hash), q) ||
			strings.Contains(strings.ToLower(e.Ref), q) ||
			strings.Contains(strings.ToLower(e.Action), q) ||
			strings.Contains(strings.ToLower(e.Subject), q) {
			out = append(out, e)
		}
	}
	return out
}

// sigBadge returns a short styled badge for a GPG/SSH signature status char.
// Returns "" for N (no sig) and "" for pure graph connector lines.
func sigBadge(sig string) string {
	switch sig {
	case "G":
		return styleStaged.Render("✓ ")
	case "B":
		return styleChanged.Render("✗ ")
	case "U":
		return styleChanged.Render("? ")
	case "X", "E":
		return styleChanged.Render("! ")
	default:
		return ""
	}
}

func (m model) updateBranchListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := filterBranches(m.branches, m.branchFilter)

	if m.branchFiltering {
		switch msg.String() {
		case "enter", "esc":
			m.branchFiltering = false
			m.branchFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.branchFilterInput, cmd = m.branchFilterInput.Update(msg)
			m.branchFilter = m.branchFilterInput.Value()
			m.branchCursor = 0
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
		}
	case "down", "j":
		if m.branchCursor < len(visible)-1 {
			m.branchCursor++
		}
	case " ":
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
		if b.Current {
			m.actionErr = fmt.Errorf("cannot select the current branch for deletion")
			break
		}
		if m.protectedBranches[b.Name] {
			m.actionErr = fmt.Errorf("branch %s is protected", b.Name)
			break
		}
		if m.branchSelected == nil {
			m.branchSelected = make(map[string]bool)
		}
		if m.branchSelected[b.Name] {
			delete(m.branchSelected, b.Name)
		} else {
			m.branchSelected[b.Name] = true
		}
		if m.branchCursor < len(visible)-1 {
			m.branchCursor++
		}
		m.actionErr = nil
	case "enter":
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
		if b.Current {
			m.panel = panelMain
			return m, nil
		}
		m.panel = panelMain
		return m, m.doSwitch(b.Name)
	case "m":
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
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
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
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
		// Batch delete when there is an active multi-selection.
		if len(m.branchSelected) > 0 {
			var sel []git.Branch
			for _, br := range m.branches {
				if m.branchSelected[br.Name] && !br.Current {
					sel = append(sel, br)
				}
			}
			if len(sel) == 0 {
				m.branchSelected = nil
				break
			}
			names := make([]string, len(sel))
			for i, br := range sel {
				names[i] = "  · " + br.Name
			}
			wtNote := ""
			for _, br := range sel {
				if _, ok := m.branchWorktrees[br.Name]; ok {
					wtNote = "\n\nbranches in a worktree will have that worktree removed first."
					break
				}
			}
			m.confirmPrompt = fmt.Sprintf("delete %d selected branch(es)? (unmerged work will be lost)\n\n%s%s", len(sel), strings.Join(names, "\n"), wtNote)
			m.confirmCmd = m.doDeleteBranchesMany(sel, m.branchWorktrees)
			m.confirmOrigin = panelBranchList
			m.panel = panelConfirm
			m.actionErr = nil
			break
		}
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
		if b.Current {
			m.actionErr = fmt.Errorf("cannot delete the current branch - switch to another branch first")
			break
		}
		// A branch checked out in a worktree can't be deleted until that
		// worktree is gone — offer to do both in one step.
		if path, ok := m.branchWorktrees[b.Name]; ok {
			m.confirmPrompt = fmt.Sprintf("branch %s is checked out in a worktree at:\n  %s\n\nremove that worktree AND delete the branch? (unmerged work will be lost)", b.Name, path)
			m.confirmCmd = m.doRemoveWorktreeThenDeleteBranch(path, b.Name)
			m.confirmOrigin = panelBranchList
			m.panel = panelConfirm
			m.actionErr = nil
			break
		}
		m.confirmPrompt = fmt.Sprintf("delete branch %s? (unmerged work will be lost)", b.Name)
		m.confirmCmd = m.doDeleteBranch(b.Name)
		m.confirmOrigin = panelBranchList
		m.panel = panelConfirm
	case "n":
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
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
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
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
	case "v":
		if len(visible) == 0 {
			break
		}
		b := visible[m.branchCursor]
		if b.Current {
			m.actionErr = fmt.Errorf("cannot compare branch with itself")
			break
		}
		m.diffOrigin = panelBranchList
		m.diffFilePath = ""
		m.diffFileStaged = false
		m.diffLines = nil
		m.diffScroll = 0
		m.panel = panelDiff
		return m, m.doCompareDiff(b.Name)
	case "X":
		var sweep []git.Branch
		for _, b := range m.branches {
			if b.Current || m.protectedBranches[b.Name] {
				continue
			}
			// gone = remote tracking ref deleted; squashed = net diff already in
			// the default branch. Both mean the local branch is safe to remove.
			if b.Gone || m.squashedBranches[b.Name] {
				sweep = append(sweep, b)
			}
		}
		if len(sweep) == 0 {
			m.actionErr = fmt.Errorf("no gone or squashed branches to clean up")
			break
		}
		names := make([]string, len(sweep))
		for i, b := range sweep {
			tag := "gone"
			if !b.Gone {
				tag = "squashed"
			}
			names[i] = fmt.Sprintf("  · %s  (%s)", b.Name, tag)
		}
		m.confirmPrompt = fmt.Sprintf("delete %d gone/squashed branch(es)?\n\n%s", len(sweep), strings.Join(names, "\n"))
		m.confirmCmd = m.doDeleteGoneBranches(sweep)
		m.confirmOrigin = panelBranchList
		m.panel = panelConfirm
		m.actionErr = nil
	case "/":
		m.branchFiltering = true
		m.branchFilterInput.Focus()
	case "esc", m.cfg.Keybindings.Quit:
		if m.branchFilter != "" {
			m.branchFilter = ""
			m.branchFilterInput.SetValue("")
			m.branchCursor = 0
			return m, nil
		}
		if len(m.branchSelected) > 0 {
			m.branchSelected = nil
			return m, nil
		}
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

// scrollUp scrolls the currently active panel up by one line.
func (m model) scrollUp() model {
	switch m.panel {
	case panelDiff:
		if m.diffScroll > 0 {
			m.diffScroll--
		}
	case panelLog:
		if m.logCursor > 0 {
			m.logCursor--
		}
	case panelBranchList:
		if m.branchCursor > 0 {
			m.branchCursor--
		}
	case panelCommitDetail:
		if m.commitDetailScroll > 0 {
			m.commitDetailScroll--
		}
	case panelBlame:
		if m.blameScroll > 0 {
			m.blameScroll--
		}
	case panelReflog:
		if m.reflogCursor > 0 {
			m.reflogCursor--
		}
	case panelStashList:
		if m.stashCursor > 0 {
			m.stashCursor--
		}
	case panelGraph:
		if m.graphScroll > 0 {
			m.graphScroll--
		}
	case panelMain:
		if m.cursor > 0 {
			m.cursor--
		}
	}
	return m
}

// scrollDown scrolls the currently active panel down by one line.
func (m model) scrollDown() model {
	switch m.panel {
	case panelDiff:
		_, ms := m.diffViewport()
		if m.diffScroll < ms {
			m.diffScroll++
		}
	case panelLog:
		if m.logCursor < len(m.logEntries)-1 {
			m.logCursor++
		}
	case panelBranchList:
		visible := filterBranches(m.branches, m.branchFilter)
		if m.branchCursor < len(visible)-1 {
			m.branchCursor++
		}
	case panelCommitDetail:
		if m.commitDetailScroll < 200 {
			m.commitDetailScroll++
		}
	case panelBlame:
		if m.blameScroll < len(m.blameLines)-1 {
			m.blameScroll++
		}
	case panelReflog:
		if m.reflogCursor < len(m.reflogEntries)-1 {
			m.reflogCursor++
		}
	case panelStashList:
		if m.stashCursor < len(m.stashes)-1 {
			m.stashCursor++
		}
	case panelGraph:
		if m.graphScroll < len(m.graphLines)-1 {
			m.graphScroll++
		}
	case panelMain:
		if m.cursor < len(m.files)-1 {
			m.cursor++
		}
	}
	return m
}

func (m model) updateDiffPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visibleLines, maxScroll := m.diffViewport()

	// PR diff mode: cursor-based navigation with inline comment support.
	if m.diffOrigin == panelPR {
		if m.prLineCommentActive {
			switch msg.String() {
			case "enter":
				body := strings.TrimSpace(m.prLineCommentInput.Value())
				if body != "" && m.diffCursor < len(m.diffPositions) {
					pos := m.diffPositions[m.diffCursor]
					if pos.position > 0 {
						m.prLineCommentActive = false
						num := m.prDiffNumber
						return m, func() tea.Msg {
							ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
							defer cancel()
							commenter, ok := m.prProvider.(pr.PRLineCommenter)
							if !ok {
								return actionDoneMsg{cmd: "pr comment line", err: fmt.Errorf("provider does not support inline comments")}
							}
							err := commenter.CommentPRLine(ctx, num, pos.file, pos.position, body)
							info := ""
							if err == nil {
								info = fmt.Sprintf("commented on %s (position %d)", pos.file, pos.position)
							}
							return actionDoneMsg{cmd: "pr comment line", err: err, info: info}
						}
					}
				}
				m.prLineCommentActive = false
			case "esc":
				m.prLineCommentActive = false
			case "ctrl+c":
				return m, tea.Quit
			default:
				var cmd tea.Cmd
				m.prLineCommentInput, cmd = m.prLineCommentInput.Update(msg)
				return m, cmd
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.diffCursor > 0 {
				m.diffCursor--
				if m.diffCursor < m.diffScroll {
					m.diffScroll = m.diffCursor
				}
			}
		case "down", "j":
			if m.diffCursor < len(m.diffLines)-1 {
				m.diffCursor++
				if m.diffCursor >= m.diffScroll+visibleLines {
					m.diffScroll = m.diffCursor - visibleLines + 1
					if m.diffScroll > maxScroll {
						m.diffScroll = maxScroll
					}
				}
			}
		case "]":
			next := jumpHunk(m.diffLines, m.diffCursor, +1)
			m.diffCursor = next
			if m.diffCursor >= m.diffScroll+visibleLines {
				m.diffScroll = m.diffCursor - visibleLines + 1
				if m.diffScroll > maxScroll {
					m.diffScroll = maxScroll
				}
			}
		case "[":
			prev := jumpHunk(m.diffLines, m.diffCursor, -1)
			m.diffCursor = prev
			if m.diffCursor < m.diffScroll {
				m.diffScroll = m.diffCursor
			}
		case "c":
			if m.diffCursor < len(m.diffPositions) && m.diffPositions[m.diffCursor].position > 0 {
				ti := textinput.New()
				ti.Placeholder = "inline comment..."
				ti.Focus()
				ti.CharLimit = 512
				ti.Width = m.width - 6
				m.prLineCommentInput = ti
				m.prLineCommentActive = true
			}
		case "esc", m.cfg.Keybindings.Quit:
			m.panel = m.diffOrigin
			m.diffOrigin = panelMain
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// Regular scroll mode for non-PR diffs.
	_ = maxScroll

	// Search mode: intercept all keys when search input is active.
	if m.diffSearching {
		switch msg.String() {
		case "enter":
			q := strings.TrimSpace(m.diffSearchInput.Value())
			m.diffSearching = false
			m.diffSearchInput.Blur()
			if q != "" {
				m.diffSearch = q
				m.diffSearchMatches = buildDiffSearchMatches(m.diffLines, q)
				m.diffSearchCursor = 0
				if len(m.diffSearchMatches) > 0 {
					m.diffScroll = m.diffSearchMatches[0]
				}
			}
		case "esc":
			m.diffSearching = false
			m.diffSearchInput.Blur()
			m.diffSearch = ""
			m.diffSearchMatches = nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.diffSearchInput, cmd = m.diffSearchInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.diffScroll > 0 {
			m.diffScroll--
		}
	case "down", "j":
		_, ms := m.diffViewport()
		if m.diffScroll < ms {
			m.diffScroll++
		}
	case "]":
		m.diffScroll = jumpHunk(m.diffLines, m.diffScroll, +1)
	case "[":
		m.diffScroll = jumpHunk(m.diffLines, m.diffScroll, -1)
	case "/":
		ti := textinput.New()
		ti.Placeholder = "search diff..."
		ti.Focus()
		ti.CharLimit = 100
		ti.Width = m.width - 6
		m.diffSearchInput = ti
		m.diffSearching = true
	case "n":
		if len(m.diffSearchMatches) > 0 {
			m.diffSearchCursor = (m.diffSearchCursor + 1) % len(m.diffSearchMatches)
			target := m.diffSearchMatches[m.diffSearchCursor]
			_, ms := m.diffViewport()
			if target > ms {
				target = ms
			}
			m.diffScroll = target
		}
	case "N":
		if len(m.diffSearchMatches) > 0 {
			m.diffSearchCursor = (m.diffSearchCursor - 1 + len(m.diffSearchMatches)) % len(m.diffSearchMatches)
			target := m.diffSearchMatches[m.diffSearchCursor]
			_, ms := m.diffViewport()
			if target > ms {
				target = ms
			}
			m.diffScroll = target
		}
	case "+", "=":
		if m.diffContext < 15 && m.diffOrigin == panelMain && m.diffFilePath != "" {
			m.diffContext++
			return m, m.doFetchDiff(m.diffFilePath, m.diffFileStaged)
		}
	case "-", "_":
		if m.diffContext > 0 && m.diffOrigin == panelMain && m.diffFilePath != "" {
			m.diffContext--
			return m, m.doFetchDiff(m.diffFilePath, m.diffFileStaged)
		}
	case " ":
		if m.diffOrigin == panelMain && m.diffFilePath != "" {
			return m, m.doStageFromDiff(m.diffFilePath, !m.diffFileStaged)
		}
	case "x":
		if m.diffOrigin == panelMain && m.diffFilePath != "" && !m.diffFileStaged {
			m.confirmPrompt = fmt.Sprintf("discard all changes to %s? this cannot be undone", m.diffFilePath)
			m.confirmCmd = m.doDiscardFromDiff(m.diffFilePath)
			m.confirmOrigin = panelDiff
			m.panel = panelConfirm
		}
	case "o":
		// Open the current file at the visible line in the editor.
		filePath := m.diffFilePath
		if filePath == "" {
			// Try to get file from diff positions
			if m.diffScroll < len(m.diffPositions) {
				filePath = m.diffPositions[m.diffScroll].file
			}
		}
		if filePath == "" {
			break
		}
		line := 0
		if m.diffScroll < len(m.diffPositions) {
			line = m.diffPositions[m.diffScroll].newLine
		}
		return m, m.doOpenEditorAtLine(filePath, line)
	case "w":
		if m.diffOrigin == panelMain && m.diffFilePath != "" {
			m.diffWordDiff = !m.diffWordDiff
			if m.diffWordDiff {
				return m, m.doFetchWordDiff(m.diffFilePath, m.diffFileStaged)
			}
			return m, m.doFetchDiff(m.diffFilePath, m.diffFileStaged)
		}
	case "a":
		if m.diffOrigin == panelStashList {
			ref := m.diffTitle
			m.panel = panelMain
			m.diffOrigin = panelMain
			m.stashPendingAction = ""
			return m, m.doStashApply(ref)
		}
	case "enter":
		if m.diffOrigin == panelStashList {
			ref := m.diffTitle
			m.panel = panelMain
			m.diffOrigin = panelMain
			m.stashPendingAction = ""
			return m, m.doStashPop(ref)
		}
	case "e":
		path := strings.TrimSuffix(m.diffTitle, "  (staged)")
		m.blameLines = nil
		m.blameScroll = 0
		m.panel = panelBlame
		return m, m.doBlame(path)
	case "esc", m.cfg.Keybindings.Quit:
		if m.diffOrigin == panelStashList {
			m.stashPendingAction = ""
			m.panel = panelStashList
			m.diffOrigin = panelMain
			return m, nil
		}
		m.panel = m.diffOrigin
		m.diffOrigin = panelMain
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
		m.actionErr = nil
		if active {
			return m, m.doBisectSkip("")
		}
		return m, m.doBisectStart()
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
			m.actionErr = nil
			m.amending = true
			m.amendLog = nil
			ch := make(chan string, 1000)
			m.amendProgressCh = ch
			return m, tea.Batch(m.doAmendNoEditStream(ch), listenAmendProgress(ch))
		case "ctrl+n":
			m.noVerify = !m.noVerify
			return m, nil
		case "esc", kb.Quit:
			m.panel = panelMain
			m.noVerify = false
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
	const numItems = 8
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
		case 7:
			m.cmdBarEnabled = cmdBarEnabledFromConfig(m.cfg.CommandBar.Items)
			m.cmdBarCursor = 0
			m.panel = panelCommandBar
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
		for _, f := range m.files {
			if f.entry.Path == path && f.category == catConflict {
				return m, m.doRestoreAndStage(path, source)
			}
		}
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
	visible := filterReflog(m.reflogEntries, m.reflogFilter)

	if m.reflogFiltering {
		switch msg.String() {
		case "enter", "esc":
			m.reflogFiltering = false
			m.reflogFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.reflogFilterInput, cmd = m.reflogFilterInput.Update(msg)
			m.reflogFilter = m.reflogFilterInput.Value()
			m.reflogCursor = 0
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.reflogCursor > 0 {
			m.reflogCursor--
		}
	case "down", "j":
		if m.reflogCursor < len(visible)-1 {
			m.reflogCursor++
		}
	case "r":
		if len(visible) == 0 {
			break
		}
		e := visible[m.reflogCursor]
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
			return actionDoneMsg{cmd: "git reset --mixed " + hash, err: nil, info: "reset HEAD to " + hash}
		}
		m.panel = panelConfirm
	case "y":
		if len(visible) > 0 {
			_ = writeClipboard(visible[m.reflogCursor].Hash)
		}
	case "/":
		m.reflogFiltering = true
		m.reflogFilterInput.Focus()
	case "esc", m.cfg.Keybindings.Quit:
		if m.reflogFilter != "" {
			m.reflogFilter = ""
			m.reflogFilterInput.SetValue("")
			m.reflogCursor = 0
			return m, nil
		}
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
	case "f":
		forker, ok := m.prProvider.(pr.PRForker)
		if !ok {
			m.actionErr = fmt.Errorf("no PR provider available - detect a remote first")
			break
		}
		prov := forker
		m.confirmPrompt = "fork this repository? a fork will be created on your account"
		m.confirmCmd = func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := prov.Fork(ctx)
			var info string
			if err == nil {
				info = "repository forked successfully"
			}
			return actionDoneMsg{cmd: "repo fork", err: err, info: info}
		}
		m.panel = panelConfirm
	case "p":
		if len(m.remotes) == 0 {
			break
		}
		name := m.remotes[m.remoteCursor].Name
		m.confirmPrompt = fmt.Sprintf("prune stale tracking refs for %q? (removes refs to deleted remote branches)", name)
		m.confirmCmd = m.doRemotePrune(name)
		m.panel = panelConfirm
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
	backPanel := panelRemoteList
	if m.initFromNoRepo {
		backPanel = panelMain
	}
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
			url, extractedName := sanitizeRemoteURL(m.remoteAddInputs[1].Value())
			if name == "" && extractedName != "" {
				name = extractedName
			}
			if name == "" || url == "" {
				break
			}
			m.initFromNoRepo = false
			m.panel = backPanel
			return m, tea.Batch(m.doRemoteAdd(name, url), m.doFetchRemotes())
		}
	case "esc":
		if m.remoteAddStep == 1 {
			m.remoteAddInputs[1].Blur()
			m.remoteAddInputs[0].Focus()
			m.remoteAddStep = 0
		} else {
			m.initFromNoRepo = false
			m.panel = backPanel
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

	// Line mode: navigate and toggle individual lines within the focused hunk.
	if m.hunkLineMode {
		hunk := m.hunkList[m.hunkCursor]
		switch msg.String() {
		case "up", "k":
			if m.hunkLineCursor > 0 {
				m.hunkLineCursor--
			}
		case "down", "j":
			if m.hunkLineCursor < len(hunk.Body)-1 {
				m.hunkLineCursor++
			}
		case " ":
			i := m.hunkLineCursor
			if i < len(hunk.Body) {
				line := hunk.Body[i]
				if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
					m.hunkLineSel[i] = !m.hunkLineSel[i]
				}
			}
		case "enter":
			m.hunkLineMode = false
			m.panel = panelMain
			return m, m.doApplyLines()
		case "esc":
			m.hunkLineMode = false
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// Hunk mode: navigate and toggle whole hunks.
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
	case "l":
		// Enter line mode for the focused hunk.
		if m.hunkCursor < len(m.hunkList) {
			hunk := m.hunkList[m.hunkCursor]
			m.hunkLineSel = make([]bool, len(hunk.Body))
			for i := range m.hunkLineSel {
				m.hunkLineSel[i] = true
			}
			m.hunkLineCursor = 0
			m.hunkLineMode = true
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
	visible := filterStashes(m.stashes, m.stashFilter)

	if m.stashFiltering {
		switch msg.String() {
		case "enter", "esc":
			m.stashFiltering = false
			m.stashFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.stashFilterInput, cmd = m.stashFilterInput.Update(msg)
			m.stashFilter = m.stashFilterInput.Value()
			m.stashCursor = 0
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.stashCursor > 0 {
			m.stashCursor--
		}
	case "down", "j":
		if m.stashCursor < len(visible)-1 {
			m.stashCursor++
		}
	case "enter":
		if len(visible) == 0 {
			break
		}
		ref := visible[m.stashCursor].Ref
		m.diffLines = nil
		m.diffScroll = 0
		m.diffOrigin = panelStashList
		m.stashPendingAction = "pop"
		m.panel = panelDiff
		return m, m.doFetchStashDiff(ref)
	case "a":
		if len(visible) == 0 {
			break
		}
		ref := visible[m.stashCursor].Ref
		m.diffLines = nil
		m.diffScroll = 0
		m.diffOrigin = panelStashList
		m.stashPendingAction = "apply"
		m.panel = panelDiff
		return m, m.doFetchStashDiff(ref)
	case "p":
		if len(visible) == 0 {
			break
		}
		ref := visible[m.stashCursor].Ref
		return m, m.doFetchStashFiles(ref)
	case " ":
		if len(visible) == 0 {
			break
		}
		ref := visible[m.stashCursor].Ref
		m.diffLines = nil
		m.diffScroll = 0
		m.diffOrigin = panelStashList
		m.panel = panelDiff
		return m, m.doFetchStashDiff(ref)
	case "d":
		if len(visible) == 0 {
			break
		}
		ref := visible[m.stashCursor].Ref
		m.confirmPrompt = fmt.Sprintf("drop %s? this cannot be undone", ref)
		m.confirmCmd = m.doStashDrop(ref)
		m.panel = panelConfirm
	case "/":
		m.stashFiltering = true
		m.stashFilterInput.Focus()
	case "esc", m.cfg.Keybindings.Quit:
		if m.stashFilter != "" {
			m.stashFilter = ""
			m.stashFilterInput.SetValue("")
			m.stashCursor = 0
			return m, nil
		}
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateStashFilesPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.stashFilesCursor > 0 {
			m.stashFilesCursor--
		}
	case "down", "j":
		if m.stashFilesCursor < len(m.stashFilesList)-1 {
			m.stashFilesCursor++
		}
	case " ":
		if m.stashFilesCursor < len(m.stashFilesSel) {
			m.stashFilesSel[m.stashFilesCursor] = !m.stashFilesSel[m.stashFilesCursor]
		}
	case "a":
		allOn := true
		for _, s := range m.stashFilesSel {
			if !s {
				allOn = false
				break
			}
		}
		for i := range m.stashFilesSel {
			m.stashFilesSel[i] = !allOn
		}
	case "enter":
		var selected []string
		for i, f := range m.stashFilesList {
			if i < len(m.stashFilesSel) && m.stashFilesSel[i] {
				selected = append(selected, f)
			}
		}
		if len(selected) == 0 {
			break
		}
		ref := m.stashFilesRef
		m.panel = panelMain
		return m, m.doStashCheckoutFiles(ref, selected)
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelStashList
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) stashFilesView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Partial Apply") + "  " + styleDim.Render(m.stashFilesRef) + "\n\n")

	if m.stashFilesList == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.stashFilesList) == 0 {
		b.WriteString("  " + styleDim.Render("no files in stash") + "\n")
	} else {
		selectedCount := 0
		for _, s := range m.stashFilesSel {
			if s {
				selectedCount++
			}
		}
		b.WriteString("  " + styleDim.Render(fmt.Sprintf("%d/%d files selected — space to toggle, a to toggle all", selectedCount, len(m.stashFilesList))) + "\n\n")
		for i, f := range m.stashFilesList {
			cursor := "  "
			if m.stashFilesCursor == i {
				cursor = styleSelected.Render("> ")
			}
			check := "[ ]"
			if i < len(m.stashFilesSel) && m.stashFilesSel[i] {
				check = styleSelected.Render("[x]")
			}
			b.WriteString(cursor + "  " + check + "  " + f + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [space] toggle  [a] toggle all  [enter] apply selected  [esc] back") + "\n"
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
		// Return to the panel the confirm was opened from (panelMain by
		// default). Branch delete/sweep set confirmOrigin = panelBranchList so
		// they stay on the list and can show the in-progress hint.
		m.panel = m.confirmOrigin
		if m.confirmOrigin == panelBranchList {
			m.branchDeleting = true
		}
		m.confirmOrigin = panelMain
		return m, cmd
	case "n", "N", "esc", m.cfg.Keybindings.Quit:
		m.confirmCmd = nil
		origin := m.confirmOrigin
		m.confirmOrigin = panelMain
		m.panel = origin
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateHelpPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "a":
		m.panel = panelAbout
	default:
		m.panel = panelMain
	}
	return m, nil
}

func (m model) updateAboutPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "q", "enter", "?":
		m.panel = panelMain
	}
	return m, nil
}

func (m model) doWriteResolved(path string, parts []conflictPart, res []int, custom []string) tea.Cmd {
	return func() tea.Msg {
		lines := resolveConflictFile(parts, res, custom)
		content := strings.Join(lines, "\n")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return actionDoneMsg{cmd: "write " + path, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
		defer cancel()
		err := m.git.Add(ctx, path)
		info := ""
		if err == nil {
			info = path + " resolved and staged"
		}
		return actionDoneMsg{cmd: "git add " + path, err: err, info: info}
	}
}

func (m model) updateConflictPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle edit input mode first.
	if m.conflictEditMode {
		switch msg.String() {
		case "enter":
			value := m.conflictEditInput.Value()
			if m.conflictHunkCursor < len(m.conflictCustom) {
				m.conflictCustom[m.conflictHunkCursor] = value
				m.conflictHunkRes[m.conflictHunkCursor] = hunkOurs // mark resolved
				m.conflictHunkCursor = nextUnresolved(m.conflictHunkRes, m.conflictHunkCursor)
			}
			m.conflictEditMode = false
			if allResolved(m.conflictHunkRes) {
				m.panel = panelMain
				return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
			}
		case "esc":
			m.conflictEditMode = false
		default:
			var cmd tea.Cmd
			m.conflictEditInput, cmd = m.conflictEditInput.Update(msg)
			return m, cmd
		}
		return m, nil
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

	// Count conflict hunks for bounds checking.
	nHunks := len(m.conflictHunkRes)

	// hunkIndex returns the index in conflictHunkRes for the N-th conflict part.
	hunkAt := func(n int) int { return n } // direct mapping

	switch msg.String() {
	case "up", "k":
		if m.conflictScroll > 0 {
			m.conflictScroll--
		}
	case "down", "j":
		if m.conflictScroll < nHunks-1 {
			m.conflictScroll++
		}
	case "n":
		if m.conflictHunkCursor < nHunks-1 {
			m.conflictHunkCursor++
			m.conflictScroll = m.conflictHunkCursor
		}
	case "N":
		if m.conflictHunkCursor > 0 {
			m.conflictHunkCursor--
			m.conflictScroll = m.conflictHunkCursor
		}
	case "o":
		if code == "DD" {
			break
		}
		if nHunks == 0 {
			// No parsed hunks - fall back to whole-file accept ours.
			m.panel = panelMain
			return m, m.doAcceptOurs(m.conflictPath)
		}
		m.conflictHunkRes[hunkAt(m.conflictHunkCursor)] = hunkOurs
		// Advance to next unresolved hunk.
		m.conflictHunkCursor = nextUnresolved(m.conflictHunkRes, m.conflictHunkCursor)
		if allResolved(m.conflictHunkRes) {
			m.panel = panelMain
			return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		}
	case "t":
		if code == "DD" {
			break
		}
		if nHunks == 0 {
			m.panel = panelMain
			return m, m.doAcceptTheirs(m.conflictPath)
		}
		m.conflictHunkRes[hunkAt(m.conflictHunkCursor)] = hunkTheirs
		m.conflictHunkCursor = nextUnresolved(m.conflictHunkRes, m.conflictHunkCursor)
		if allResolved(m.conflictHunkRes) {
			m.panel = panelMain
			return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		}
	case "b":
		if code != "DD" && nHunks > 0 {
			m.conflictHunkRes[hunkAt(m.conflictHunkCursor)] = hunkBoth
			m.conflictHunkCursor = nextUnresolved(m.conflictHunkRes, m.conflictHunkCursor)
			if allResolved(m.conflictHunkRes) {
				m.panel = panelMain
				return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
			}
		}
	case "enter":
		if allResolved(m.conflictHunkRes) && nHunks > 0 {
			m.panel = panelMain
			return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		}
	case "r":
		if code == "DD" {
			m.panel = panelMain
			return m, m.doRemoveConflict(m.conflictPath)
		}
	case "O":
		// Accept ours for ALL hunks at once.
		if code != "DD" && nHunks > 0 {
			for i := range m.conflictHunkRes {
				m.conflictHunkRes[i] = hunkOurs
			}
			m.panel = panelMain
			return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		}
		if nHunks == 0 {
			m.panel = panelMain
			return m, m.doAcceptOurs(m.conflictPath)
		}
	case "T":
		// Accept theirs for ALL hunks at once.
		if code != "DD" && nHunks > 0 {
			for i := range m.conflictHunkRes {
				m.conflictHunkRes[i] = hunkTheirs
			}
			m.panel = panelMain
			return m, m.doWriteResolved(m.conflictPath, m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		}
		if nHunks == 0 {
			m.panel = panelMain
			return m, m.doAcceptTheirs(m.conflictPath)
		}
	case "e":
		// Open current hunk in manual edit mode (single-line input with ours as default).
		if code != "DD" && nHunks > 0 && m.conflictHunkCursor < len(m.conflictParts) {
			// Find the current hunk's ours content as the default edit value.
			hi := -1
			defaultVal := ""
			for _, p := range m.conflictParts {
				if !p.isConflict() {
					continue
				}
				hi++
				if hi == m.conflictHunkCursor {
					defaultVal = strings.Join(p.ours, "\n")
					break
				}
			}
			ti := textinput.New()
			ti.SetValue(defaultVal)
			ti.Focus()
			ti.CharLimit = 4000
			ti.Width = m.width - 6
			m.conflictEditInput = ti
			m.conflictEditMode = true
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func nextUnresolved(res []int, current int) int {
	// Search forward from current+1, then wrap.
	for i := current + 1; i < len(res); i++ {
		if res[i] == hunkUnresolved {
			return i
		}
	}
	for i := 0; i < current; i++ {
		if res[i] == hunkUnresolved {
			return i
		}
	}
	return current
}

func allResolved(res []int) bool {
	for _, r := range res {
		if r == hunkUnresolved {
			return false
		}
	}
	return len(res) > 0
}

func (m model) updateResetPickPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		m.confirmPrompt = "soft reset HEAD~1? the commit will be removed but all changes remain staged"
		m.confirmCmd = m.doReset("soft")
		m.confirmOrigin = panelResetPick
		m.panel = panelConfirm
	case "m":
		m.confirmPrompt = "mixed reset HEAD~1? the commit will be removed and changes moved back to working tree"
		m.confirmCmd = m.doReset("mixed")
		m.confirmOrigin = panelResetPick
		m.panel = panelConfirm
	case "h":
		m.confirmPrompt = "hard reset HEAD~1? uncommitted changes will be permanently discarded"
		m.confirmCmd = m.doReset("hard")
		m.confirmOrigin = panelResetPick
		m.panel = panelConfirm
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTagListPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := filterTags(m.tags, m.tagFilter)

	if m.tagFiltering {
		switch msg.String() {
		case "enter", "esc":
			m.tagFiltering = false
			m.tagFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.tagFilterInput, cmd = m.tagFilterInput.Update(msg)
			m.tagFilter = m.tagFilterInput.Value()
			m.tagCursor = 0
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.tagCursor > 0 {
			m.tagCursor--
		}
	case "down", "j":
		if m.tagCursor < len(visible)-1 {
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
		if len(visible) == 0 {
			break
		}
		tag := visible[m.tagCursor]
		m.confirmPrompt = fmt.Sprintf("delete tag %s?", tag.Name)
		m.confirmCmd = m.doDeleteTag(tag.Name)
		m.panel = panelConfirm
	case "p":
		if len(visible) == 0 {
			break
		}
		tag := visible[m.tagCursor]
		m.confirmPrompt = fmt.Sprintf("push tag %s to origin?", tag.Name)
		m.confirmCmd = m.doPushTag("origin", tag.Name)
		m.panel = panelConfirm
	case "/":
		m.tagFiltering = true
		m.tagFilterInput.Focus()
	case "esc", m.cfg.Keybindings.Quit:
		if m.tagFilter != "" {
			m.tagFilter = ""
			m.tagFilterInput.SetValue("")
			m.tagCursor = 0
			return m, nil
		}
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateTagCreatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		// Toggle annotated mode while on the name step.
		if m.tagAnnotateStep == 0 {
			m.tagAnnotated = !m.tagAnnotated
		}
		return m, nil
	case "enter":
		if m.tagAnnotateStep == 0 {
			name := strings.TrimSpace(m.branchInput.Value())
			if name == "" {
				m.actionErr = fmt.Errorf("tag name cannot be empty")
				return m, nil
			}
			if !m.tagAnnotated {
				m.panel = panelMain
				m.actionErr = nil
				return m, m.doCreateTag(name)
			}
			// Annotated: move to message step.
			ti := textinput.New()
			ti.Placeholder = "tag message (e.g. Release v1.2.0)"
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.branchInput.Width
			m.tagMsgInput = ti
			m.tagAnnotateStep = 1
			m.actionErr = nil
			return m, nil
		}
		// Step 1: create annotated tag.
		name := strings.TrimSpace(m.branchInput.Value())
		message := strings.TrimSpace(m.tagMsgInput.Value())
		if message == "" {
			m.actionErr = fmt.Errorf("message cannot be empty for an annotated tag")
			return m, nil
		}
		m.tagAnnotateStep = 0
		m.tagAnnotated = false
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doCreateAnnotatedTag(name, message)
	case "esc":
		if m.tagAnnotateStep == 1 {
			m.tagAnnotateStep = 0
			return m, nil
		}
		m.tagAnnotated = false
		m.panel = panelTagList
		m.actionErr = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	if m.tagAnnotateStep == 1 {
		m.tagMsgInput, cmd = m.tagMsgInput.Update(msg)
	} else {
		m.branchInput, cmd = m.branchInput.Update(msg)
	}
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
		ti.Placeholder = "worktree label  (e.g. res-123)"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = m.width - 6
		m.branchInput = ti
		m.worktreeAddStep = 0
		m.panel = panelWorktreeAdd
		m.actionErr = nil
	case "d":
		if len(m.worktrees) == 0 {
			break
		}
		wt := m.worktrees[m.worktreeCursor]
		if wt.Current {
			m.actionErr = fmt.Errorf("main worktree is protected")
			break
		}
		if wt.Locked {
			alive, known := lockOwnerAlive(wt.LockReason)
			if known && alive {
				// A live process holds the lock — refuse. Forcing here could
				// yank the tree out from under a running agent.
				m.actionErr = fmt.Errorf("worktree is locked by a live process (%s) — stop it or 'git worktree unlock' first", wt.LockReason)
				break
			}
			reason := wt.LockReason
			if reason == "" {
				reason = "no reason given"
			}
			var detail string
			if known {
				detail = "the locking process is not running"
			} else {
				detail = "could not verify the lock owner"
			}
			m.confirmPrompt = fmt.Sprintf("worktree at %s is LOCKED (%s)\n%s — force-remove it?", wt.Path, reason, detail)
			m.confirmCmd = m.doForceRemoveWorktree(wt.Path)
			m.panel = panelConfirm
			m.actionErr = nil
			break
		}
		m.confirmPrompt = fmt.Sprintf("remove worktree at %s?", wt.Path)
		m.confirmCmd = m.doRemoveWorktree(wt.Path)
		m.panel = panelConfirm
		m.actionErr = nil
	case "D":
		var merged []git.WorktreeEntry
		for _, wt := range m.worktrees {
			if wt.Merged || wt.Gone {
				merged = append(merged, wt)
			}
		}
		if len(merged) == 0 {
			m.actionErr = fmt.Errorf("no merged or gone worktrees to remove")
			break
		}
		m.confirmPrompt = fmt.Sprintf("remove %d merged/gone worktree(s)?", len(merged))
		m.confirmCmd = m.doRemoveMergedWorktrees(merged)
		m.panel = panelConfirm
		m.actionErr = nil
	case "p":
		m.confirmPrompt = "prune stale worktree refs? (removes admin files for gone worktrees)"
		m.confirmCmd = m.doPruneWorktrees()
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
		if m.worktreeAddStep == 0 {
			label := strings.TrimSpace(m.branchInput.Value())
			if label == "" {
				m.actionErr = fmt.Errorf("label cannot be empty")
				return m, nil
			}
			ti := textinput.New()
			ti.Placeholder = "feat/" + label + "  or  fix/" + label
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = m.branchInput.Width
			m.worktreeBranchInput = ti
			m.worktreeAddStep = 1
			m.actionErr = nil
			return m, nil
		}
		// Step 1: collect branch name (empty = detached HEAD), validate, then create.
		label := strings.TrimSpace(m.branchInput.Value())
		branch := strings.TrimSpace(m.worktreeBranchInput.Value())
		if m.repoRoot == "" {
			m.actionErr = fmt.Errorf("cannot determine repo root for default path")
			return m, nil
		}
		path := worktreeDefaultPath(m.repoRoot, label)
		result := conventions.Validate(branch, m.cfg.Conventions)
		if !result.Valid && m.cfg.Conventions.Validation.Mode == "strict" {
			m.actionErr = fmt.Errorf("branch name does not follow conventions (strict mode)")
			return m, nil
		}
		if m.status != nil && m.status.Behind > 0 && m.status.Upstream != "" {
			m.pendingWorktreePath = path
			m.pendingWorktreeBranch = branch
			m.panel = panelWorktreeBaseChoice
			m.actionErr = nil
			return m, nil
		}
		m.worktreeAddStep = 0
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doAddWorktree(path, branch, "")
	case "esc":
		if m.worktreeAddStep == 1 {
			m.worktreeAddStep = 0
			m.actionErr = nil
			return m, nil
		}
		m.panel = panelWorktreeList
		m.actionErr = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	if m.worktreeAddStep == 1 {
		m.worktreeBranchInput, cmd = m.worktreeBranchInput.Update(msg)
	} else {
		m.branchInput, cmd = m.branchInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateWorktreeBaseChoicePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path, branch, base := m.pendingWorktreePath, m.pendingWorktreeBranch, m.status.Upstream
		m.pendingWorktreePath, m.pendingWorktreeBranch = "", ""
		m.worktreeAddStep = 0
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doAddWorktree(path, branch, base)
	case "m", "M":
		path, branch := m.pendingWorktreePath, m.pendingWorktreeBranch
		m.pendingWorktreePath, m.pendingWorktreeBranch = "", ""
		m.worktreeAddStep = 0
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doAddWorktree(path, branch, "")
	case "esc":
		m.panel = panelWorktreeAdd
		m.actionErr = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) updateWorktreePostCreatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.postCreateSetup {
		switch msg.String() {
		case "ctrl+d", "ctrl+s":
			raw := strings.TrimSpace(m.postCreateTA.Value())
			var cmds []string
			for _, line := range strings.Split(raw, "\n") {
				if s := strings.TrimSpace(line); s != "" {
					cmds = append(cmds, s)
				}
			}
			if err := config.SaveProjectWorktree(m.repoRoot, cmds); err != nil {
				m.actionErr = err
				return m, nil
			}
			m.cfg.Worktree.PostCreate = &cmds
			m.panel = panelMain
			m.actionErr = nil
			if len(cmds) == 0 {
				return m, nil
			}
			return m, m.doRunPostCreate(m.postCreatePath, cmds)
		case "esc":
			m.panel = panelMain
			m.actionErr = nil
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.postCreateTA, cmd = m.postCreateTA.Update(msg)
		return m, cmd
	}
	// Confirm mode — already configured.
	switch msg.String() {
	case "enter", "y", "Y":
		cmds := *m.cfg.Worktree.PostCreate
		m.panel = panelMain
		m.actionErr = nil
		return m, m.doRunPostCreate(m.postCreatePath, cmds)
	case "s", "S", "n", "N", "esc":
		m.panel = panelMain
		m.actionErr = nil
		return m, nil
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
	if m.panel == panelStashFiles {
		return m.stashFilesView()
	}
	if m.panel == panelHelp {
		return m.helpView()
	}
	if m.panel == panelAbout {
		return m.aboutView()
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
	if m.panel == panelWorktreeBaseChoice {
		return m.worktreeBaseChoiceView()
	}
	if m.panel == panelWorktreePostCreate {
		return m.worktreePostCreateView()
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
	if m.panel == panelCommandBar {
		return m.commandBarConfigView()
	}
	if m.panel == panelDiverged {
		return m.divergedView()
	}
	if m.panel == panelPR {
		return m.prView()
	}
	if m.panel == panelPRDetail {
		return m.prDetailView()
	}
	if m.panel == panelPRReview {
		return m.prReviewView()
	}
	if m.panel == panelPRCreate {
		return m.prCreateView()
	}
	if m.panel == panelPRMerge {
		return m.prMergeView()
	}
	if m.panel == panelIssues {
		return m.issuesView()
	}
	if m.panel == panelSSH {
		return m.sshView()
	}
	if m.panel == panelLFS {
		return m.lfsView()
	}
	if m.panel == panelDashboard {
		return m.dashboardView()
	}
	if m.panel == panelInit {
		return m.initView()
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
		if m.prStatus != nil {
			prBadge := fmt.Sprintf("#%d %s", m.prStatus.Number, m.prStatus.State)
			switch m.prStatus.CI {
			case "success":
				header += "  " + styleStaged.Render(prBadge+" ✓")
			case "failure":
				header += "  " + styleConflict.Render(prBadge+" ✗")
			case "pending":
				header += "  " + styleChanged.Render(prBadge+" ●")
			default:
				header += "  " + styleDim.Render(prBadge)
			}
		} else if m.prProvider != nil {
			header += "  " + styleDim.Render("["+m.prProvider.Name()+"]")
		}
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
			case "revert":
				if len(m.status.Conflicts) > 0 {
					banner = fmt.Sprintf("revert in progress - resolve %d conflict(s), then [c] to continue  [a] to abort", len(m.status.Conflicts))
				} else {
					banner = "revert in progress - conflicts resolved, press [c] to continue  [a] to abort"
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
			b.WriteString("  " + styleDim.Render("nothing to commit, working tree clean") + "\n\n")
			if config.OverviewEnabled(m.cfg) {
				m.renderOverview(&b)
			}
		}
		b.WriteString("\n")
	}

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n")
	} else if m.lastInfo != "" {
		b.WriteString("  " + styleStaged.Render("✓ "+m.lastInfo) + "\n")
	} else if m.lastCmd != "" {
		b.WriteString("  " + styleDim.Render("$ "+m.lastCmd) + "\n")
	} else if m.convViolation != nil && m.cfg.Conventions.Validation.Mode == "warn" {
		b.WriteString("  " + styleChanged.Render("! "+m.convViolation.Branch+" does not follow conventions") + "\n")
	} else if m.cfg.Modes.Default != "pro" {
		b.WriteString("  " + styleDim.Render(contextTip(m)) + "\n")
	} else {
		b.WriteString("  " + styleDim.Render("run 'bonsai config' to change mode or flow") + "\n")
	}
	if hint := fileActionHint(m); hint != "" {
		b.WriteString("  " + styleDim.Render(hint) + "\n")
	}
	if hint := prHint(m); hint != "" {
		b.WriteString("  " + styleStaged.Render(hint) + "\n")
	}
	if len(m.undoStack) > 0 {
		b.WriteString("  " + styleDim.Render("[U] undo: "+m.undoStack[len(m.undoStack)-1].desc) + "\n")
	}
	if detectFlow(m.cfg) == "gitflow" && m.status != nil {
		if bType := gitflowBranchType(m.status.Branch, m.cfg); bType != "" {
			b.WriteString("  " + styleDim.Render("[F] finish "+bType+": "+m.status.Branch) + "\n")
		}
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

	inSelMode := false
	for _, f := range m.files {
		if f.selected {
			inSelMode = true
			break
		}
	}

	b.WriteString("  " + styleSection.Render(fmt.Sprintf("%s (%d)", title, len(entries))) + "\n")
	for i, f := range entries {
		cursor := "  "
		if m.cursor == offset+i {
			cursor = styleSelected.Render("> ")
		}
		check := ""
		checkW := 0
		if inSelMode {
			checkW = 4
			if offset+i < len(m.files) && m.files[offset+i].selected {
				check = styleSelected.Render("[x] ")
			} else {
				check = styleDim.Render("[ ] ")
			}
		}
		// prefix visual width: cursor(2) + sep(2) + check(0|4) + code+"  "(3)
		prefixW := 2 + 2 + checkW + 3
		path := f.Path
		if m.width > prefixW+1 {
			availW := m.width - prefixW
			if len(path) > availW {
				// first chunk on this line
				b.WriteString(cursor + "  " + check + style.Render(fileCode(f, cat)+"  "+path[:availW]) + "\n")
				rest := path[availW:]
				indent := strings.Repeat(" ", prefixW)
				for len(rest) > 0 {
					chunk := rest
					if len(chunk) > availW {
						chunk = rest[:availW]
					}
					b.WriteString(indent + style.Render(chunk) + "\n")
					rest = rest[len(chunk):]
				}
				continue
			}
		}
		b.WriteString(cursor + "  " + check + style.Render(fileCode(f, cat)+"  "+path) + "\n")
	}
	b.WriteString("\n")
}

func (m model) renderOverview(b *strings.Builder) {
	if m.prProvider == nil {
		return
	}
	if m.prListLoading {
		b.WriteString("  " + styleDim.Render("Open PRs") + "\n")
		b.WriteString("  " + styleDim.Render("  loading...") + "\n\n")
	} else if len(m.prListItems) > 0 {
		b.WriteString("  " + styleSection.Render(fmt.Sprintf("Open PRs (%d)", len(m.prListItems))) + "\n")
		for i, item := range m.prListItems {
			cursor := "  "
			if i == m.overviewCursor {
				cursor = styleSelected.Render(">>")
			}
			ci := ""
			switch item.CI {
			case "success":
				ci = styleStaged.Render(" ✓")
			case "failure":
				ci = styleConflict.Render(" ✗")
			case "pending":
				ci = styleChanged.Render(" ●")
			}
			state := ""
			if item.Draft {
				state = styleDim.Render(" [draft]")
			}
			line := fmt.Sprintf("#%d  %s", item.Number, item.Title)
			b.WriteString(cursor + "  " + styleDim.Render(line) + ci + state + "\n")
		}
		b.WriteString("\n")
	}
	if len(m.overviewLogEntries) > 0 {
		b.WriteString("  " + styleSection.Render("Recent commits") + "\n")
		for i, e := range m.overviewLogEntries {
			hash := e.Hash
			if len(hash) > 7 {
				hash = hash[:7]
			}
			cursor := "  "
			if len(m.prListItems)+i == m.overviewCursor {
				cursor = styleSelected.Render(">>")
			}
			b.WriteString(cursor + "  " + styleDim.Render(hash+"  "+e.Line) + "\n")
		}
		b.WriteString("\n")
	}
}

func (m model) commitView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Commit") + "\n\n")
	subjectLabel := "  subject: "
	if !m.commitBodyActive {
		subjectLabel = "  " + styleSelected.Render("subject") + ": "
	}
	b.WriteString(subjectLabel + m.commitMsg.View() + "\n\n")
	bodyLabel := "  body:    "
	if m.commitBodyActive {
		bodyLabel = "  " + styleSelected.Render("body") + ":    "
	}
	b.WriteString(bodyLabel + "\n")
	b.WriteString(m.commitBodyTA.View() + "\n\n")
	b.WriteString("  " + styleDim.Render("staged files that will be committed:") + "\n")
	for _, f := range m.status.Staged {
		b.WriteString("    " + styleStaged.Render(string(f.StagedCode())+"  "+f.Path) + "\n")
	}
	b.WriteString("\n")

	if m.noVerify {
		b.WriteString("  " + styleChanged.Render("--no-verify  hooks will be skipped") + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	var hint string
	if m.commitBodyActive {
		hint = "  [ctrl+d] commit  [ctrl+n] toggle --no-verify  [esc] back to subject"
	} else {
		hint = "  [enter] commit  [tab] add body  [ctrl+n] toggle --no-verify  [esc] cancel"
	}
	return content + styleDim.Render(hint) + "\n"
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
	visible := filterBranches(m.branches, m.branchFilter)
	var b strings.Builder
	b.WriteString("\n")

	// Pre-calculate scroll position so we can show it in the title.
	overhead := 6
	if m.branchFiltering {
		overhead += 2
	}
	visibleLines := m.height - overhead
	if visibleLines < 1 {
		visibleLines = 1
	}
	start := 0
	if m.branchCursor >= visibleLines {
		start = m.branchCursor - visibleLines + 1
	}
	end := start + visibleLines
	if end > len(visible) {
		end = len(visible)
	}

	title := "Branches"
	if len(m.branches) > 0 {
		if m.branchFilter != "" {
			title = fmt.Sprintf("Branches (%d/%d)", len(visible), len(m.branches))
		} else if len(m.branches) >= git.MaxBranches {
			title = fmt.Sprintf("Branches (%d+ — use filter to search)", len(m.branches))
		} else {
			title = fmt.Sprintf("Branches (%d)", len(m.branches))
		}
	}
	if m.branchFilter != "" {
		title += "  " + styleCmd.Render("["+m.branchFilter+"]")
	}
	titleLine := "  " + styleSection.Render(title)
	if len(visible) > visibleLines {
		scrollPos := fmt.Sprintf("%d/%d", m.branchCursor+1, len(visible))
		if start > 0 {
			scrollPos = "↑ " + scrollPos
		}
		if end < len(visible) {
			scrollPos += " ↓"
		}
		titleLine += "  " + styleDim.Render(scrollPos)
	}
	b.WriteString(titleLine + "\n\n")

	if m.branchFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.branchFilterInput.View() + "\n\n")
	}

	if m.branches == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(visible) == 0 {
		if m.branchFilter != "" {
			b.WriteString("  " + styleDim.Render("no branches matched - press esc to clear") + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("no branches found") + "\n")
		}
	} else {
		for i := start; i < end; i++ {
			br := visible[i]
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
			if br.Date != "" {
				name += "  " + styleDim.Render(br.Date)
			}
			if br.Gone {
				name += "  " + styleChanged.Render("gone")
			} else if br.Ahead > 0 || br.Behind > 0 {
				track := ""
				if br.Ahead > 0 {
					track += styleAdded.Render(fmt.Sprintf("↑%d", br.Ahead))
				}
				if br.Behind > 0 {
					if track != "" {
						track += " "
					}
					track += styleChanged.Render(fmt.Sprintf("↓%d", br.Behind))
				}
				name += "  " + track
			} else if br.Upstream != "" {
				name += "  " + styleSynced.Render("↑↓ synced")
			}
			if m.mergedBranches[br.Name] && !br.Current {
				name += "  " + styleMerged.Render("merged")
			} else if m.squashedBranches[br.Name] && !br.Current {
				name += "  " + styleMerged.Render("squashed")
			}
			if m.protectedBranches[br.Name] {
				name += "  " + styleChanged.Render("(protected)")
			}
			if _, ok := m.branchWorktrees[br.Name]; ok {
				name += "  " + styleDim.Render("[worktree]")
			}
			mark := "  "
			if m.branchSelected[br.Name] {
				mark = styleAdded.Render("✓ ")
			}
			b.WriteString(cursor + mark + name + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	// Context-sensitive hint line.
	var hint string
	if m.branchDeleting {
		return content + styleDim.Render("  deleting branch(es)...") + "\n"
	}
	if m.actionErr != nil {
		return content + styleChanged.Render("  error: "+m.actionErr.Error()) + "\n"
	}
	curBranch := ""
	if len(visible) > 0 && m.branchCursor < len(visible) {
		curBranch = visible[m.branchCursor].Name
	}
	switch {
	case len(m.branchSelected) > 0:
		hint = styleDim.Render(fmt.Sprintf("  %d selected  [space] toggle  [d] delete selected  [esc] clear", len(m.branchSelected)))
	case len(visible) > 0 && m.branchCursor < len(visible) && visible[m.branchCursor].Gone:
		hint = styleDim.Render("  gone - remote tracking ref deleted, safe to remove  [d] delete  [X] sweep gone/squashed  [esc] back")
	case curBranch != "" && m.squashedBranches[curBranch] && !m.protectedBranches[curBranch]:
		hint = styleDim.Render("  squashed - net diff already in the default branch, safe to remove  [d] delete  [X] sweep gone/squashed  [esc] back")
	default:
		sweepCount := 0
		for _, br := range m.branches {
			if br.Current || m.protectedBranches[br.Name] {
				continue
			}
			if br.Gone || m.squashedBranches[br.Name] {
				sweepCount++
			}
		}
		h := "  [enter] switch  [space] select  [m] merge  [r] rebase  [d] delete  [n] rename  [D] delete remote  [v] compare diff  [/] search"
		if sweepCount > 0 {
			h += fmt.Sprintf("  [X] sweep gone/squashed (%d)", sweepCount)
		}
		h += "  [esc] back"
		hint = styleDim.Render(h)
	}
	return content + hint + "\n"
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
	row("tab", "toggle file selection (multi-select); esc to clear selection")
	row("space", "stage / unstage — acts on all selected files, or cursor file if none selected")
	row("+", "stage all changes (git add .)")
	row("h", "stage by section - pick which parts of a file to stage (useful for splitting a commit)")
	row("d", "diff selected file (staged or unstaged)")
	row("H", "file history - every commit that touched this file")
	row("e", "blame - who last changed each line")
	row("x", "discard/delete — acts on all selected files, or cursor file if none selected")
	row("u", "untrack (git rm --cached) — acts on all selected staged files, or cursor file if none selected")
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
	row("B", "branch list - switch, merge, rebase, delete, rename, delete remote, [v] compare diff vs HEAD; [space] multi-select then [d] to batch-delete, [X] sweep gone/squashed; deleting a [worktree] branch offers to remove its worktree too")
	row("l", "commit log (search with ctrl+/ or ctrl+r); [p] cherry-pick, [R] range cherry-pick, [r] revert")
	row("L", "reflog - full HEAD history with reset-to")
	row(kb.Graph+" / g", "branch graph (git log --graph --all)")
	row("R", "interactive rebase (reorder, squash, fixup, drop)")
	b.WriteString("\n")

	section("Stash & tags")
	row(kb.Stash+" / s", "stash all changes (opens message input)")
	row("S", "stash list - pop, apply (both preview diff before acting), drop")
	row("t", "tag list - create (lightweight or annotated), delete, push to remote")
	b.WriteString("\n")

	section("Advanced")
	row("i", "bisect - binary search for a bug-introducing commit ([b] bad, [G/g] good, [s] skip)")
	row("z", "reset menu (soft / mixed / hard)")
	row("U", "undo — pops the most recent reversible action (up to 5 levels: commit, merge, rebase, cherry-pick, revert)")
	row("W", "worktree list - add, remove linked worktrees; [p] prune stale entries")
	row("O", "remote management - add, remove, rename; [p] prune stale tracking refs")
	row("M", "submodule management - add, update, deinit")
	row("n", "git notes for HEAD commit")
	row("X", "clean untracked files (preview + confirm)")
	row("a", "abort in-progress merge / rebase / cherry-pick / revert")
	row("`", "SSH key manager - list keys, test connections")
	row("V", "LFS panel - tracked files and status")
	row("D", "multi-repo dashboard (configure repos in [dashboard] config)")
	b.WriteString("\n")

	section("App")
	row("C", "configuration manager (git config, gitignore, profiles, education)")
	row("?", "all shortcuts (this panel)")
	row("a", "about bonsai (from this panel)")
	row(kb.Quit+" / ctrl+c", "quit")
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  press any key to close") + "\n"
}

func (m model) aboutView() string {
	v := m.version
	if v == "" {
		v = "dev"
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleTitle.Render("bonsai") + "  " + styleDim.Render(v) + "\n\n")
	b.WriteString("  " + styleDim.Render("A terminal UI for git.") + "\n\n")
	b.WriteString("  " + styleCmd.Render("Author ") + "  AgusRdz\n")
	b.WriteString("  " + styleCmd.Render("Repo   ") + "  https://github.com/AgusRdz/bonsai\n")
	b.WriteString("  " + styleCmd.Render("Issues ") + "  https://github.com/AgusRdz/bonsai/issues\n\n")
	b.WriteString(styleDim.Render("  [esc / q] close") + "\n")
	return b.String()
}

func (m model) stashListView() string {
	visible := filterStashes(m.stashes, m.stashFilter)
	var b strings.Builder
	b.WriteString("\n")

	title := "Stashes"
	if len(m.stashes) > 0 {
		if m.stashFilter != "" {
			title = fmt.Sprintf("Stashes (%d/%d)", len(visible), len(m.stashes))
		} else {
			title = fmt.Sprintf("Stashes (%d)", len(m.stashes))
		}
	}
	if m.stashFilter != "" {
		title += "  " + styleCmd.Render("["+m.stashFilter+"]")
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.stashFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.stashFilterInput.View() + "\n\n")
	}

	if m.stashes == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(visible) == 0 {
		if m.stashFilter != "" {
			b.WriteString("  " + styleDim.Render("no stashes matched - press esc to clear") + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("no stashes") + "\n")
		}
	} else {
		for i, st := range visible {
			cursor := "  "
			if m.stashCursor == i {
				cursor = styleSelected.Render("> ")
			}
			ref := styleCmd.Render(st.Ref)
			age := ""
			if a := timeAgo(st.Date); a != "" {
				age = styleDim.Render(a) + "  "
			}
			stale := ""
			if st.Stale {
				stale = styleChanged.Render("⚠ stale") + "  "
			}
			old := ""
			if isOldStash(st.Date) {
				old = styleWarn.Render("⚠ old") + "  "
			}
			behind := ""
			if st.Behind > 0 {
				behind = styleDim.Render(fmt.Sprintf("↓%d behind", st.Behind)) + "  "
			}
			desc := st.Description
			b.WriteString(cursor + "  " + ref + "  " + age + stale + old + behind + desc + "\n")
		}
	}
	b.WriteString("\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] pop  [a] apply  [space] preview only  [p] partial  [d] drop  [/] search  [esc] back") + "\n"
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
		for i, line := range m.diffLines[m.diffScroll:end] {
			lineIdx := m.diffScroll + i
			cursor := m.diffOrigin == panelPR && lineIdx == m.diffCursor
			rendered := renderDiffLine(line, cursor)
			// Highlight the current search match line
			if m.diffSearch != "" && len(m.diffSearchMatches) > 0 &&
				lineIdx == m.diffSearchMatches[m.diffSearchCursor] {
				rendered = styleSelected.Render("▶ ") + rendered
			} else if m.diffSearch != "" {
				rendered = "  " + rendered
			}
			b.WriteString(rendered + "\n")
		}
	}

	if m.diffOrigin == panelPR && m.prLineCommentActive {
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 3; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		var fileHint string
		if m.diffCursor < len(m.diffPositions) && m.diffPositions[m.diffCursor].position > 0 {
			p := m.diffPositions[m.diffCursor]
			fileHint = styleDim.Render(fmt.Sprintf("  %s  position %d", p.file, p.position)) + "\n"
		}
		return content + fileHint + "  " + m.prLineCommentInput.View() + "\n" +
			styleDim.Render("  [enter] post  [esc] cancel") + "\n"
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	visibleLines, _ := m.diffViewport()
	scrollable := len(m.diffLines) > visibleLines
	pos := ""
	if scrollable {
		pos = fmt.Sprintf("  (%d/%d)", m.diffScroll+1, len(m.diffLines))
	}
	hasHunks := func() bool {
		for _, l := range m.diffLines {
			if strings.HasPrefix(l, "@@") {
				return true
			}
		}
		return false
	}()
	var hint string
	if m.diffOrigin == panelPR {
		parts := []string{"[↑↓] move cursor"}
		if hasHunks {
			parts = append(parts, "[]/[] hunk")
		}
		parts = append(parts, "[c] comment line  [esc] back")
		hint = "  " + strings.Join(parts, "  ")
	} else if m.diffOrigin == panelStashList {
		var parts []string
		if scrollable {
			parts = append(parts, "[↑↓] scroll")
		}
		if hasHunks {
			parts = append(parts, "[[/]] hunk")
		}
		if m.stashPendingAction == "pop" {
			parts = append(parts, "[enter] pop  [a] apply  [esc] back")
		} else {
			parts = append(parts, "[a] apply  [enter] pop  [esc] back")
		}
		hint = "  " + strings.Join(parts, "  ")
	} else {
		var parts []string
		if scrollable {
			parts = append(parts, "[↑↓] scroll")
		}
		if hasHunks {
			parts = append(parts, "[[/]] hunk")
		}
		if m.diffSearch != "" {
			parts = append(parts, fmt.Sprintf("[n/N] match %d/%d", m.diffSearchCursor+1, len(m.diffSearchMatches)))
		}
		if m.diffOrigin == panelMain && m.diffFilePath != "" {
			if m.diffFileStaged {
				parts = append(parts, "[space] unstage")
			} else {
				parts = append(parts, "[space] stage  [x] discard")
			}
			wordHint := "[w] word diff"
			if m.diffWordDiff {
				wordHint = "[w] line diff"
			}
			parts = append(parts, wordHint+"  [+/-] context")
		}
		parts = append(parts, "[/] search  [o] open  [e] blame  [esc] back")
		hint = "  " + strings.Join(parts, "  ")
	}

	if m.diffSearching {
		searchBar := "  " + m.diffSearchInput.View() + "\n"
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 2; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + searchBar + styleDim.Render("  [enter] confirm  [esc] cancel") + "\n"
	}

	return content + styleDim.Render(hint+pos) + "\n"
}

// jumpHunk returns the scroll position of the next (dir=+1) or previous
// (dir=-1) @@ hunk header relative to currentScroll. Returns currentScroll
// unchanged when no hunk is found in that direction.
func jumpHunk(lines []string, currentScroll, dir int) int {
	if dir > 0 {
		for i := currentScroll + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "@@") {
				return i
			}
		}
	} else {
		for i := currentScroll - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "@@") {
				return i
			}
		}
	}
	return currentScroll
}

func renderDiffLine(line string, cursor bool) string {
	pfx := "  "
	if cursor {
		pfx = styleSelected.Render(">") + " "
	}
	switch {
	case strings.HasPrefix(line, "@@"):
		return pfx + styleCmd.Render(line)
	case strings.HasPrefix(line, "+"):
		content := line[1:]
		if strings.HasPrefix(content, intraLineDiffSentinel) {
			// Intra-line highlights embedded — colorize prefix only.
			return pfx + styleStaged.Render("+") + content[len(intraLineDiffSentinel):]
		}
		if isLFSPointerLine(content) {
			return pfx + styleStaged.Render("+") + lfsPointerStyle(content)
		}
		return pfx + styleStaged.Render(line)
	case strings.HasPrefix(line, "-"):
		content := line[1:]
		if strings.HasPrefix(content, intraLineDiffSentinel) {
			// Intra-line highlights embedded — colorize prefix only.
			return pfx + styleChanged.Render("-") + content[len(intraLineDiffSentinel):]
		}
		if isLFSPointerLine(content) {
			return pfx + styleChanged.Render("-") + lfsPointerStyle(content)
		}
		return pfx + styleChanged.Render(line)
	case strings.HasPrefix(line, "Binary files"), strings.HasPrefix(line, "GIT binary patch"):
		return pfx + styleChanged.Render(line)
	default:
		return pfx + styleDim.Render(line)
	}
}

// chromaTokenStyles maps chroma token types (prefix match) to lipgloss colors.
// Checked in order; first match wins.
var chromaTokenStyles = []struct {
	prefix string
	style  lipgloss.Style
}{
	{"Keyword", lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)},
	{"Name.Builtin", lipgloss.NewStyle().Foreground(lipgloss.Color("81"))},
	{"Name.Function", lipgloss.NewStyle().Foreground(lipgloss.Color("148")).Bold(true)},
	{"Name.Class", lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)},
	{"Name.Type", lipgloss.NewStyle().Foreground(lipgloss.Color("214"))},
	{"Name.Decorator", lipgloss.NewStyle().Foreground(lipgloss.Color("208"))},
	{"Literal.String", lipgloss.NewStyle().Foreground(lipgloss.Color("214"))},
	{"Literal.Number", lipgloss.NewStyle().Foreground(lipgloss.Color("135"))},
	{"Literal", lipgloss.NewStyle().Foreground(lipgloss.Color("214"))},
	{"Comment", lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)},
	{"Operator", lipgloss.NewStyle().Foreground(lipgloss.Color("252"))},
	{"Punctuation", lipgloss.NewStyle().Foreground(lipgloss.Color("244"))},
}

// syntaxHighlightDiff applies per-language syntax highlighting to diff lines.
// It detects the filename from "diff --git" headers and uses chroma to
// colorize added (+) and context ( ) lines. Removed (-) lines are left plain
// so the red removal color stays dominant.
// parseDiffLinePositions returns a diffLinePos for each raw diff line, tracking
// which file and 1-based diff position (from the first @@ of each file) it maps to.
// Lines that cannot be commented on (headers, blank separators) have position=0.
// newLine tracks the 1-based line number in the new file (+++ side).
func parseDiffLinePositions(lines []string) []diffLinePos {
	result := make([]diffLinePos, len(lines))
	var currentFile string
	var position int
	var newLine int
	hunkHeaderRe := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			currentFile = ""
			position = 0
			newLine = 0
		case strings.HasPrefix(line, "+++ b/"):
			currentFile = strings.TrimPrefix(line, "+++ b/")
			position = 0
			newLine = 0
		case strings.HasPrefix(line, "@@"):
			position++
			if m := hunkHeaderRe.FindStringSubmatch(line); m != nil {
				n, _ := strconv.Atoi(m[1])
				newLine = n
			}
			result[i] = diffLinePos{file: currentFile, position: position}
		default:
			if currentFile != "" && position > 0 {
				position++
				result[i] = diffLinePos{file: currentFile, position: position, newLine: newLine}
				if !strings.HasPrefix(line, "-") {
					newLine++ // context and added lines advance the new-file line counter
				}
			}
		}
	}
	return result
}

// buildDiffSearchMatches returns the indices of diffLines that contain query (case-insensitive).
func buildDiffSearchMatches(lines []string, query string) []int {
	q := strings.ToLower(query)
	var matches []int
	for i, l := range lines {
		plain := strings.ReplaceAll(l, intraLineDiffSentinel, "")
		if strings.Contains(strings.ToLower(plain), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

// appendUndo pushes e onto the stack, keeping at most 5 entries.
func appendUndo(stack []undoEntry, e undoEntry) []undoEntry {
	stack = append(stack, e)
	if len(stack) > 5 {
		stack = stack[len(stack)-5:]
	}
	return stack
}

// intraLineDiffSentinel is prepended to diff lines that have had intra-line
// character-level highlights embedded, so renderDiffLine knows to only
// colorize the +/- prefix rather than wrapping the whole line.
const intraLineDiffSentinel = "\x01"

// intraLineDiff computes character-level diff between a removed and an added
// line (content only, without the leading -/+). Returns versions with
// background-highlighted changed segments, or the originals unchanged when
// the lines are too dissimilar to produce useful highlights.
func intraLineDiff(removed, added string) (hlRemoved, hlAdded string) {
	if removed == added || removed == "" || added == "" {
		return removed, added
	}

	// Find common prefix length.
	prefixLen := 0
	minLen := len(removed)
	if len(added) < minLen {
		minLen = len(added)
	}
	for prefixLen < minLen && removed[prefixLen] == added[prefixLen] {
		prefixLen++
	}

	// Find common suffix length (must not overlap with prefix).
	suffixLen := 0
	for suffixLen < minLen-prefixLen &&
		removed[len(removed)-1-suffixLen] == added[len(added)-1-suffixLen] {
		suffixLen++
	}

	// Skip highlighting when lines share less than 20% of their content —
	// they are probably unrelated lines that happen to be adjacent, and
	// a full-line highlight would be more confusing than helpful.
	shared := prefixLen + suffixLen
	longer := len(removed)
	if len(added) > longer {
		longer = len(added)
	}
	if longer == 0 || shared*100/longer < 20 {
		return removed, added
	}

	prefix := removed[:prefixLen]
	removedMid := removed[prefixLen : len(removed)-suffixLen]
	addedMid := added[prefixLen : len(added)-suffixLen]
	suffix := ""
	if suffixLen > 0 {
		suffix = removed[len(removed)-suffixLen:]
	}

	// Apply line colors to unchanged segments so they stay red/green, and use
	// background highlights only on the characters that actually changed.
	rPrefix := styleChanged.Render(prefix)
	rSuffix := styleChanged.Render(suffix)
	aPrefix := styleStaged.Render(prefix)
	aSuffix := styleStaged.Render(suffix)

	if removedMid == "" {
		hlRemoved = removed
	} else {
		hlRemoved = rPrefix + styleRemovedHL.Render(removedMid) + rSuffix
	}
	if addedMid == "" {
		hlAdded = added
	} else {
		hlAdded = aPrefix + styleAddedHL.Render(addedMid) + aSuffix
	}
	return hlRemoved, hlAdded
}

func syntaxHighlightDiff(lines []string) []string {
	out := make([]string, len(lines))
	var lexer chroma.Lexer = chromaLexers.Fallback

	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[3], "b/")
				if l := chromaLexers.Match(path); l != nil {
					lexer = l
				} else {
					lexer = chromaLexers.Fallback
				}
			}
			out[i] = line
			continue
		}
		// Only highlight added and context lines.
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, " ") {
			out[i] = line
			continue
		}
		prefix := string(line[0])
		content := line[1:]
		if isLFSPointerLine(content) {
			out[i] = line
			continue
		}
		out[i] = prefix + chromaHighlightLine(content, lexer)
	}

	// Second pass: apply intra-line character-level diff highlighting to
	// consecutive - / + line pairs (real changes, not file header lines).
	for i := 0; i < len(lines)-1; i++ {
		curr := lines[i]
		next := lines[i+1]
		if strings.HasPrefix(curr, "-") && !strings.HasPrefix(curr, "---") &&
			strings.HasPrefix(next, "+") && !strings.HasPrefix(next, "+++") {
			hlR, hlA := intraLineDiff(curr[1:], next[1:])
			// Only replace when at least one side was actually highlighted.
			if hlR != curr[1:] || hlA != next[1:] {
				out[i] = "-" + intraLineDiffSentinel + hlR
				out[i+1] = "+" + intraLineDiffSentinel + hlA
				i++ // skip the paired + line — already handled
			}
		}
	}

	return out
}

// chromaHighlightLine tokenizes a single line and returns it with lipgloss colors.
func chromaHighlightLine(line string, lexer chroma.Lexer) string {
	iter, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}
	var b strings.Builder
	for tok := iter(); tok != chroma.EOF; tok = iter() {
		tokenType := tok.Type.String()
		styled := false
		for _, ts := range chromaTokenStyles {
			if strings.HasPrefix(tokenType, ts.prefix) {
				b.WriteString(ts.style.Render(tok.Value))
				styled = true
				break
			}
		}
		if !styled {
			b.WriteString(tok.Value)
		}
	}
	return b.String()
}

// isLFSPointerLine reports whether a diff content line is part of an LFS pointer.
func isLFSPointerLine(s string) bool {
	return strings.HasPrefix(s, "version https://git-lfs.github.com") ||
		strings.HasPrefix(s, "oid sha256:") ||
		strings.HasPrefix(s, "size ")
}

// lfsPointerStyle renders an LFS pointer line with a distinct label.
func lfsPointerStyle(s string) string {
	badge := lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render("[LFS]")
	return " " + badge + " " + styleDim.Render(s)
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
	bar := "  [↑↓] scroll  [esc] back  [y] copy hash  [p] cherry-pick  [R] range  [r] revert"
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

var reCommitType = regexp.MustCompile(`^(feat|fix|docs|refactor|test|chore|ci|build|perf|style|revert)(\([^)]*\))?(!)?\s*:`)

func colorizeGraph(s string) string {
	var b strings.Builder
	for _, ch := range s {
		switch ch {
		case '*':
			b.WriteString(styleCmd.Render("*"))
		case '|', '/', '\\', '_', '-':
			b.WriteString(styleDim.Render(string(ch)))
		default:
			b.WriteString(string(ch))
		}
	}
	return b.String()
}

func colorizeCommitSubject(s string) string {
	loc := reCommitType.FindStringIndex(s)
	if loc == nil {
		return s
	}
	prefix := s[:loc[1]]
	rest := s[loc[1]:]
	var sty lipgloss.Style
	switch {
	case strings.Contains(prefix, "!"):
		sty = styleChanged
	case strings.HasPrefix(prefix, "feat"):
		sty = styleStaged
	case strings.HasPrefix(prefix, "fix"):
		sty = styleUntracked
	default:
		sty = styleDim
	}
	return sty.Render(prefix) + rest
}

func colorizeLogLine(line, hash string) string {
	if hash == "" {
		return styleDim.Render(line)
	}
	idx := strings.Index(line, hash)
	if idx < 0 {
		return styleDim.Render(line)
	}
	graphOut := colorizeGraph(line[:idx])
	hashOut := styleHash.Render(hash)
	rest := line[idx+len(hash):]
	decoOut := ""
	if strings.HasPrefix(rest, " (") {
		end := strings.Index(rest, ")")
		if end >= 0 {
			decoOut = " " + styleBranch.Render(rest[1:end+1])
			rest = rest[end+1:]
		}
	}
	subject := strings.TrimPrefix(rest, " ")
	return graphOut + hashOut + decoOut + " " + colorizeCommitSubject(subject)
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
			badge := sigBadge(e.Sig)
			if m.logCursor == i {
				b.WriteString("  " + styleSelected.Render(">") + " " + badge + colorizeLogLine(e.Line, e.Hash) + "\n")
			} else {
				b.WriteString("    " + badge + colorizeLogLine(e.Line, e.Hash) + "\n")
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
		b.WriteString("\n")
		content := b.String()
		if pad := m.height - strings.Count(content, "\n") - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [r] remove file  [esc] back") + "\n"
	}

	if m.conflictLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		content := b.String()
		if pad := m.height - strings.Count(content, "\n") - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [esc] back") + "\n"
	}

	nHunks := len(m.conflictHunkRes)
	if nHunks == 0 {
		b.WriteString("  " + styleDim.Render("(no conflict markers found - file may already be resolved)") + "\n")
		content := b.String()
		if pad := m.height - strings.Count(content, "\n") - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [esc] back") + "\n"
	}

	// Hunk progress dots: ● resolved (ours=green, theirs=red, both=yellow), ○ unresolved
	dots := "  "
	for i, r := range m.conflictHunkRes {
		cursor := i == m.conflictHunkCursor
		switch r {
		case hunkOurs:
			if cursor {
				dots += styleStaged.Render("[✓]")
			} else {
				dots += styleStaged.Render("✓")
			}
		case hunkTheirs:
			if cursor {
				dots += styleChanged.Render("[✓]")
			} else {
				dots += styleChanged.Render("✓")
			}
		case hunkBoth:
			if cursor {
				dots += styleUntracked.Render("[✓]")
			} else {
				dots += styleUntracked.Render("✓")
			}
		default:
			if cursor {
				dots += styleSelected.Render("[○]")
			} else {
				dots += styleDim.Render("○")
			}
		}
		dots += " "
	}
	b.WriteString(dots + styleDim.Render(fmt.Sprintf("  hunk %d of %d", m.conflictHunkCursor+1, nHunks)) + "\n\n")

	// Find the current conflict hunk in parts.
	hi := -1
	var curPart conflictPart
	for _, p := range m.conflictParts {
		if !p.isConflict() {
			continue
		}
		hi++
		if hi == m.conflictHunkCursor {
			curPart = p
			break
		}
	}

	// Show context lines before (last 2 lines of preceding context block).
	// Find the preceding context block.
	ctxLines := []string{}
	ci := 0
	for _, p := range m.conflictParts {
		if !p.isConflict() {
			ctxLines = p.context
			continue
		}
		if ci == m.conflictHunkCursor {
			break
		}
		ci++
		ctxLines = nil
	}
	if len(ctxLines) > 2 {
		ctxLines = ctxLines[len(ctxLines)-2:]
	}
	for _, l := range ctxLines {
		b.WriteString("  " + styleDim.Render(l) + "\n")
	}

	// Calculate available lines per section; with base that's 3 sections.
	hasBase := len(curPart.base) > 0
	sections := 2
	if hasBase {
		sections = 3
	}
	overhead := 10 + sections*2 // header + section labels
	visLines := (m.height - overhead) / sections
	if visLines < 2 {
		visLines = 2
	}

	// OURS section.
	b.WriteString("  " + styleStaged.Render("--- yours (HEAD)") + "\n")
	oursLines := curPart.ours
	if len(oursLines) > visLines {
		oursLines = oursLines[:visLines]
	}
	if len(oursLines) == 0 {
		b.WriteString("  " + styleDim.Render("(empty)") + "\n")
	}
	for _, l := range oursLines {
		b.WriteString("  " + styleStaged.Render(l) + "\n")
	}

	// BASE section - only shown when common ancestor is available.
	if hasBase {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("--- base (common ancestor)") + "\n")
		baseLines := curPart.base
		if len(baseLines) > visLines {
			baseLines = baseLines[:visLines]
		}
		if len(baseLines) == 0 {
			b.WriteString("  " + styleDim.Render("(empty)") + "\n")
		}
		for _, l := range baseLines {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render(l) + "\n")
		}
	}

	b.WriteString("  " + styleChanged.Render("--- incoming") + "\n")

	// THEIRS section.
	theirLines := curPart.theirs
	if len(theirLines) > visLines {
		theirLines = theirLines[:visLines]
	}
	if len(theirLines) == 0 {
		b.WriteString("  " + styleDim.Render("(empty)") + "\n")
	}
	for _, l := range theirLines {
		b.WriteString("  " + styleChanged.Render(l) + "\n")
	}

	// Manual edit input - shown when e is pressed.
	if m.conflictEditMode {
		b.WriteString("\n  " + lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("--- edit result (one line; \\n for newlines)") + "\n")
		b.WriteString("  " + m.conflictEditInput.View() + "\n")
		content := b.String()
		if pad := m.height - strings.Count(content, "\n") - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [enter] confirm  [esc] cancel") + "\n"
	}

	// Result preview - shows up to 3 resolved lines from the merged output.
	if len(m.conflictHunkRes) > 0 {
		preview := resolveConflictFile(m.conflictParts, m.conflictHunkRes, m.conflictCustom)
		resolved := 0
		for _, r := range m.conflictHunkRes {
			if r != hunkUnresolved {
				resolved++
			}
		}
		total := len(m.conflictHunkRes)
		b.WriteString("\n  " + styleDim.Render(fmt.Sprintf("--- result preview  (%d/%d hunks resolved, %d lines total)",
			resolved, total, len(preview))) + "\n")
		previewLines := preview
		if len(previewLines) > 4 {
			previewLines = previewLines[:4]
		}
		for _, l := range previewLines {
			if strings.HasPrefix(l, "<<<<<<<") || strings.HasPrefix(l, ">>>>>>>") {
				b.WriteString("  " + styleChanged.Render(l) + "\n")
			} else {
				b.WriteString("  " + styleDim.Render(l) + "\n")
			}
		}
		if len(preview) > 4 {
			b.WriteString("  " + styleDim.Render(fmt.Sprintf("... (%d more lines)", len(preview)-4)) + "\n")
		}
	}

	content := b.String()
	if pad := m.height - strings.Count(content, "\n") - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	var bar string
	if allResolved(m.conflictHunkRes) {
		bar = "  [enter] write and stage  [n/N] prev/next hunk  [esc] back"
	} else {
		bar = "  [o] yours  [t] incoming  [b] both  [e] edit  [n/N] next/prev  [O/T] all  [esc] back"
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
	visible := filterTags(m.tags, m.tagFilter)
	var b strings.Builder
	b.WriteString("\n")

	title := "Tags"
	if len(m.tags) > 0 {
		if m.tagFilter != "" {
			title = fmt.Sprintf("Tags (%d/%d)", len(visible), len(m.tags))
		} else {
			title = fmt.Sprintf("Tags (%d)", len(m.tags))
		}
	}
	if m.tagFilter != "" {
		title += "  " + styleCmd.Render("["+m.tagFilter+"]")
	}
	b.WriteString("  " + styleSection.Render(title) + "\n\n")

	if m.tagFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.tagFilterInput.View() + "\n\n")
	}

	if m.tags == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(visible) == 0 {
		if m.tagFilter != "" {
			b.WriteString("  " + styleDim.Render("no tags matched - press esc to clear") + "\n")
		} else {
			b.WriteString("  " + styleDim.Render("no tags found") + "\n")
		}
	} else {
		for i, tag := range visible {
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
	return content + styleDim.Render("  [n] new tag  [d] delete  [p] push  [/] search  [esc] back") + "\n"
}

func (m model) tagCreateView() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.tagAnnotateStep == 1 {
		b.WriteString("  " + styleSection.Render("Create Annotated Tag") + "\n\n")
		b.WriteString("  " + styleDim.Render("Tag name: ") + styleCmd.Render(m.branchInput.Value()) + "\n\n")
		b.WriteString("  " + styleDim.Render("Annotation message:") + "\n")
		b.WriteString("  " + m.tagMsgInput.View() + "\n\n")
	} else {
		title := "Create Tag"
		if m.tagAnnotated {
			title = "Create Annotated Tag"
		}
		b.WriteString("  " + styleSection.Render(title) + "\n\n")
		b.WriteString("  " + m.branchInput.View() + "\n\n")
		if m.tagAnnotated {
			b.WriteString("  " + styleDim.Render("Annotated tag — next step will ask for a message.") + "\n\n")
		}
	}

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	var bar string
	if m.tagAnnotateStep == 1 {
		bar = "  [enter] create annotated tag  [esc] back to name"
	} else if m.tagAnnotated {
		bar = "  [enter] next (add message)  [tab] lightweight  [esc] cancel"
	} else {
		bar = "  [enter] create  [tab] annotated  [esc] cancel"
	}
	return content + styleDim.Render(bar) + "\n"
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
			tag := ""
			if wt.Merged {
				tag = "  " + styleMerged.Render("[merged]")
			} else if wt.Gone {
				tag = "  " + styleMerged.Render("[gone]")
			}
			if wt.Locked {
				tag += "  " + styleChanged.Render("🔒 locked")
			}
			b.WriteString(cursor + mark + path + "  " + branch + tag + "\n")
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
	hasMerged := false
	for _, wt := range m.worktrees {
		if wt.Merged || wt.Gone {
			hasMerged = true
			break
		}
	}
	footer := "  [n] add  [d] remove  [p] prune stale  [esc] back"
	if hasMerged {
		footer = "  [n] add  [d] remove  [D] remove merged  [p] prune stale  [esc] back"
	}
	if len(m.worktrees) > 0 && m.worktrees[m.worktreeCursor].Current {
		footer = "  [n] add  [p] prune stale  [esc] back  ·  main is protected"
		if hasMerged {
			footer = "  [n] add  [D] remove merged  [p] prune stale  [esc] back"
		}
	}
	return content + styleDim.Render(footer) + "\n"
}

// worktreeDefaultPath builds the auto-generated worktree path: <repo-parent>/<repo-name>-<sanitized-branch>.
func worktreeDefaultPath(repoRoot, branch string) string {
	sanitized := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-"+sanitized)
}

func (m model) worktreeAddView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Add Worktree") + "\n\n")

	if m.worktreeAddStep == 0 {
		b.WriteString("  " + m.branchInput.View() + "\n\n")
		label := strings.TrimSpace(m.branchInput.Value())
		if label != "" && m.repoRoot != "" {
			b.WriteString("  " + styleDim.Render("path: "+worktreeDefaultPath(m.repoRoot, label)) + "\n\n")
		} else {
			b.WriteString("  " + styleDim.Render("worktree label — sets the directory name (e.g. res-123)") + "\n\n")
		}
	} else {
		label := strings.TrimSpace(m.branchInput.Value())
		if m.repoRoot != "" {
			b.WriteString("  " + styleDim.Render("path: "+worktreeDefaultPath(m.repoRoot, label)) + "\n\n")
		}
		b.WriteString("  " + m.worktreeBranchInput.View() + "\n\n")
		b.WriteString("  " + styleDim.Render("leave empty to start detached from HEAD (create branch later)") + "\n\n")
		branch := strings.TrimSpace(m.worktreeBranchInput.Value())
		if branch != "" && m.cfg.Conventions.Validation.Mode != "off" && len(m.cfg.Conventions.Branches) > 0 {
			result := conventions.Validate(branch, m.cfg.Conventions)
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
	}

	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	footer := "  [enter] next  [esc] cancel"
	if m.worktreeAddStep == 1 {
		footer = "  [enter] create  [esc] back"
	}
	return content + styleDim.Render(footer) + "\n"
}

func (m model) worktreeBaseChoiceView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Divergence Warning") + "\n\n")
	if m.status != nil {
		b.WriteString(fmt.Sprintf("  %s is %s behind %s\n\n",
			styleBranch.Render(m.status.Branch),
			styleChanged.Render(fmt.Sprintf("%d commits", m.status.Behind)),
			styleDim.Render(m.status.Upstream),
		))
	}
	b.WriteString("  Base new branch on:\n\n")
	upstream := ""
	if m.status != nil {
		upstream = m.status.Upstream
	}
	b.WriteString("  " + styleCmd.Render("[enter]") + "  " + styleStaged.Render(upstream) + "  " + styleDim.Render("(recommended — up to date)") + "\n")
	localLabel := "local"
	if m.status != nil {
		localLabel = m.status.Branch
	}
	behind := 0
	if m.status != nil {
		behind = m.status.Behind
	}
	b.WriteString("  " + styleCmd.Render("[m]") + "      " + styleChanged.Render(localLabel) + "  " + styleDim.Render(fmt.Sprintf("(behind by %d)", behind)) + "\n")

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [esc] cancel") + "\n"
}

func (m model) worktreePostCreateView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Post-create Setup") + "\n\n")

	if m.postCreateSetup {
		b.WriteString("  " + styleDim.Render("Commands to run after creating a worktree (one per line).") + "\n")
		b.WriteString("  " + styleDim.Render("$BONSAI_MAIN_WORKTREE = path to the main worktree.") + "\n\n")
		b.WriteString("  " + m.postCreateTA.View() + "\n\n")
		if m.actionErr != nil {
			b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
		}
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [ctrl+d] save & run  [esc] skip once") + "\n"
	}

	cmds := *m.cfg.Worktree.PostCreate
	b.WriteString("  " + styleDim.Render("Run post-create commands?") + "\n\n")
	for _, cmd := range cmds {
		b.WriteString("  " + styleCmd.Render(cmd) + "\n")
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
	return content + styleDim.Render("  [enter] run  [s] skip") + "\n"
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
		b.WriteString("  " + styleCmd.Render("[s]") + "  " + styleDim.Render("skip this commit (can't test it)") + "\n")
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
		bar = "  [b] bad  [G] good (HEAD)  [g] good (hash)  [s] skip  [r] reset  [l] log"
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
		// Show streaming log while running, or after failure if output was captured.
		if m.amending || (m.actionErr != nil && len(m.amendLog) > 0) {
			if len(m.amendLog) == 0 {
				b.WriteString("  " + styleDim.Render("running pre-commit hooks...") + "\n")
			} else {
				for _, line := range m.amendLog {
					b.WriteString("  " + line + "\n")
				}
			}
			content := b.String()
			lines := strings.Count(content, "\n")
			footer := ""
			if !m.amending && m.actionErr != nil {
				footer = styleChanged.Render("  hook failed — [n] retry  [esc] cancel") + "\n"
			}
			if pad := m.height - lines - 1; pad > 0 {
				content += strings.Repeat("\n", pad)
			}
			return content + footer
		}

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

		if m.noVerify {
			b.WriteString("  " + styleChanged.Render("--no-verify  hooks will be skipped") + "\n")
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
		return content + styleDim.Render("  [m] message  [a] author  [d] date  [n] add staged  [ctrl+n] --no-verify  [esc] cancel") + "\n"
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
		{"Command bar", "(choose which shortcuts appear)"},
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
	visible := filterReflog(m.reflogEntries, m.reflogFilter)
	titleStr := "Reflog"
	if m.reflogFilter != "" {
		titleStr = fmt.Sprintf("Reflog (%d/%d)  %s", len(visible), len(m.reflogEntries), styleCmd.Render("["+m.reflogFilter+"]"))
	}
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render(titleStr) + "\n\n")

	if m.reflogFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.reflogFilterInput.View() + "\n\n")
	}

	if len(visible) == 0 {
		if m.reflogFilter != "" {
			b.WriteString(styleDim.Render("  no entries matched - press esc to clear") + "\n")
		} else {
			b.WriteString(styleDim.Render("  no reflog entries") + "\n")
		}
	} else {
		overhead := 6
		if m.reflogFiltering {
			overhead += 2
		}
		visibleLines := m.height - overhead
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := 0
		if m.reflogCursor >= visibleLines {
			start = m.reflogCursor - visibleLines + 1
		}
		end := start + visibleLines
		if end > len(visible) {
			end = len(visible)
		}
		for i := start; i < end; i++ {
			e := visible[i]
			cursor := "  "
			if i == m.reflogCursor {
				cursor = styleSelected.Render("▶ ")
			}
			ref := styleDim.Render(e.Ref)
			action := styleChanged.Render(e.Action)
			b.WriteString(fmt.Sprintf("%s%s  %s  %s  %s\n", cursor, styleHash.Render(e.Hash), ref, action, e.Subject))
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓] scroll  [r] reset to  [y] copy hash  [/] search  [esc] back") + "\n")
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
	b.WriteString(styleDim.Render("  [a] add  [d] remove  [r] rename  [p] prune stale refs  [f] fork  [esc] back") + "\n")
	return b.String()
}

func (m model) remoteAddView() string {
	title := styleTitle.Render("Add Remote")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  name:\n  " + m.remoteAddInputs[0].View() + "\n\n")
	b.WriteString("  url:\n  " + m.remoteAddInputs[1].View() + "\n\n")
	hint := "  [enter] next/confirm  [esc] back"
	if m.initFromNoRepo {
		hint = "  [enter] confirm  [esc] skip"
	}
	b.WriteString(styleDim.Render(hint) + "\n")
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
			b.WriteString(cursor + colorizeLogLine(e.Line, e.Hash) + "\n")
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
	case "g":
		m.graphScroll = 0
	case "G":
		m.graphScroll = maxScroll
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
	b.WriteString("\n  " + title)
	if len(m.graphLines) > 0 {
		b.WriteString("  " + styleDim.Render(fmt.Sprintf("(%d commits)", len(m.graphLines))))
	}
	b.WriteString("\n\n")
	if m.graphLines == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		return b.String()
	}
	if len(m.graphLines) == 0 {
		b.WriteString("  " + styleDim.Render("no commits found") + "\n")
	} else {
		colored := m.graphColored
		if colored == nil {
			colored = colorizeGraphLines(m.graphLines)
		}
		visible := m.height - 6
		if visible < 1 {
			visible = 1
		}
		end := m.graphScroll + visible
		if end > len(colored) {
			end = len(colored)
		}
		for _, line := range colored[m.graphScroll:end] {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑↓/jk] scroll  [g/G] top/bottom  [esc] back") + "\n")
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

// graphPalette is the set of ANSI colors assigned to branches in the graph view.
var graphPalette = []lipgloss.Color{"9", "10", "12", "13", "14", "208", "11", "201"}

// colorizeGraphLines applies per-branch terminal colors to git --graph output.
// Branch colors are derived from the column of each '*' commit marker so
// parallel branches stay consistently colored across lines.
func colorizeGraphLines(lines []string) []string {
	colColor := map[int]lipgloss.Color{}
	nextIdx := 0
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = colorizeGraphLine(line, colColor, &nextIdx)
	}
	return out
}

func colorizeGraphLine(line string, colColor map[int]lipgloss.Color, nextIdx *int) string {
	runes := []rune(line)
	// First pass: assign colors to new '*' columns.
	for col, ch := range runes {
		if ch == '*' {
			if _, ok := colColor[col]; !ok {
				colColor[col] = graphPalette[*nextIdx%len(graphPalette)]
				*nextIdx++
			}
		}
	}
	// Find where the graph prefix ends (first non-graph character run).
	graphEnd := graphPrefixEnd(runes)

	var b strings.Builder
	for col, ch := range runes {
		if col >= graphEnd {
			// Colorize refs: (HEAD -> branch, origin/branch).
			b.WriteRune(ch)
			continue
		}
		switch ch {
		case '*':
			b.WriteString(lipgloss.NewStyle().Foreground(colColor[col]).Bold(true).Render("*"))
		case '|':
			b.WriteString(lipgloss.NewStyle().Foreground(graphNearestColor(colColor, col)).Render("|"))
		case '/', '\\':
			b.WriteString(lipgloss.NewStyle().Foreground(graphNearestColor(colColor, col)).Render(string(ch)))
		case '-', '_':
			b.WriteString(styleDim.Render(string(ch)))
		default:
			b.WriteRune(ch)
		}
	}
	// Color decorations in the commit info portion.
	info := string(runes[min(graphEnd, len(runes)):])
	b.Reset()
	prefix := string(runes[:min(graphEnd, len(runes))])
	_ = prefix
	// Re-build: graph prefix already written above — but we reset b.
	// Rebuild cleanly.
	var out strings.Builder
	for col, ch := range runes {
		if col >= graphEnd {
			break
		}
		switch ch {
		case '*':
			out.WriteString(lipgloss.NewStyle().Foreground(colColor[col]).Bold(true).Render("*"))
		case '|':
			out.WriteString(lipgloss.NewStyle().Foreground(graphNearestColor(colColor, col)).Render("|"))
		case '/', '\\':
			out.WriteString(lipgloss.NewStyle().Foreground(graphNearestColor(colColor, col)).Render(string(ch)))
		case '-', '_':
			out.WriteString(styleDim.Render(string(ch)))
		default:
			out.WriteRune(ch)
		}
	}
	out.WriteString(colorizeGraphInfo(info))
	return out.String()
}

// graphPrefixEnd returns the index in runes where the commit info starts.
// The graph area contains only: * | / \ - _ and spaces.
func graphPrefixEnd(runes []rune) int {
	graphChars := "*|/\\-_ "
	for i, ch := range runes {
		if !strings.ContainsRune(graphChars, ch) {
			return i
		}
	}
	return len(runes)
}

// graphNearestColor returns the color of the nearest column that has one,
// falling back to dim gray when no columns are colored yet.
func graphNearestColor(colColor map[int]lipgloss.Color, col int) lipgloss.Color {
	if c, ok := colColor[col]; ok {
		return c
	}
	best := lipgloss.Color("8")
	bestDist := 1000
	for k, v := range colColor {
		d := k - col
		if d < 0 {
			d = -d
		}
		if d < bestDist {
			bestDist = d
			best = v
		}
	}
	return best
}

// colorizeGraphInfo colors the commit info part of a --graph line.
// It dims the hash, styles HEAD and branch refs distinctively.
func colorizeGraphInfo(info string) string {
	if info == "" {
		return ""
	}
	// Format: <space>hash<space>(refs) message
	// Just pass through - keep simple, hash already styled by log panel.
	// Color branch decorations: text between ( ) after hash.
	open := strings.Index(info, "(")
	close := strings.Index(info, ")")
	if open < 0 || close < 0 || close < open {
		return styleDim.Render(info)
	}
	before := info[:open]
	refs := info[open+1 : close]
	after := info[close+1:]

	var b strings.Builder
	b.WriteString(styleDim.Render(before))
	b.WriteString(styleDim.Render("("))
	// Color each ref.
	parts := strings.Split(refs, ", ")
	for i, ref := range parts {
		if i > 0 {
			b.WriteString(styleDim.Render(", "))
		}
		ref = strings.TrimSpace(ref)
		switch {
		case strings.HasPrefix(ref, "HEAD"):
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(ref))
		case strings.HasPrefix(ref, "tag:"):
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Render(ref))
		case strings.HasPrefix(ref, "origin/") || strings.Contains(ref, "/"):
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(ref))
		default:
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(ref))
		}
	}
	b.WriteString(styleDim.Render(")"))
	b.WriteString(after)
	return b.String()
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

func (m model) doSaveCommandBar() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		p, err := config.GlobalConfigPath()
		if err != nil {
			return errMsg{fmt.Errorf("save command bar: %w", err)}
		}
		if err := config.Write(p, cfg); err != nil {
			return errMsg{fmt.Errorf("save command bar: %w", err)}
		}
		return nil
	}
}

func (m model) updateCommandBarPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cmdBarCursor > 0 {
			m.cmdBarCursor--
		}
	case "down", "j":
		if m.cmdBarCursor < len(cmdBarCatalog)-1 {
			m.cmdBarCursor++
		}
	case " ", "enter":
		if m.cmdBarCursor < len(m.cmdBarEnabled) {
			m.cmdBarEnabled[m.cmdBarCursor] = !m.cmdBarEnabled[m.cmdBarCursor]
			m.cfg.CommandBar.Items = buildCommandBarItems(m.cmdBarEnabled)
			return m, m.doSaveCommandBar()
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelConfigMenu
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) commandBarConfigView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleSection.Render("Command Bar") + "\n\n")

	for i, entry := range cmdBarCatalog {
		cursor := "  "
		if m.cmdBarCursor == i {
			cursor = styleSelected.Render("> ")
		}
		check := styleDim.Render("[ ]")
		if i < len(m.cmdBarEnabled) && m.cmdBarEnabled[i] {
			check = styleAdded.Render("[✓]")
		}
		key := styleCmd.Render(fmt.Sprintf("%-9s", entry.displayKey))
		desc := styleDim.Render(entry.description)
		b.WriteString(cursor + "  " + check + "  " + key + "  " + desc + "\n")
	}

	b.WriteString("\n")
	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [✓] shown  [ ] hidden  [space/enter] toggle  [esc] back") + "\n"
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

	if m.hunkLineMode && m.hunkCursor < len(m.hunkList) {
		return m.hunkLineModeView(title)
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
			b.WriteString("        " + m.styledDiffLine(line) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(styleDim.Render("  [↑↓] navigate  [space] toggle  [a] all/none  [l] line mode  [enter] apply  [esc] back") + "\n")
	return b.String()
}

func (m model) hunkLineModeView(title string) string {
	var lb strings.Builder
	lb.WriteString("\n  " + title + "  " + styleDim.Render("(line mode)") + "\n\n")
	hunk := m.hunkList[m.hunkCursor]
	lb.WriteString("  " + styleDim.Render(hunk.Header) + "\n")
	for i, line := range hunk.Body {
		isChange := strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")
		cursor := "  "
		if m.hunkLineCursor == i {
			cursor = styleSelected.Render("▶ ")
		}
		var checkbox string
		if isChange {
			if i < len(m.hunkLineSel) && m.hunkLineSel[i] {
				checkbox = styleAdded.Render("[✓]") + " "
			} else {
				checkbox = "[ ] "
			}
		} else {
			checkbox = "    "
		}
		lb.WriteString("  " + cursor + checkbox + m.styledDiffLine(line) + "\n")
	}
	lb.WriteString("\n")
	lb.WriteString(styleDim.Render("  [↑↓] navigate  [space] toggle line  [enter] apply  [esc] back to hunks") + "\n")
	return lb.String()
}

func (m model) styledDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+"):
		return styleAdded.Render(line)
	case strings.HasPrefix(line, "-"):
		return styleConflict.Render(line)
	default:
		return styleDim.Render(line)
	}
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

func fileActionHint(m model) string {
	if len(m.files) == 0 {
		return ""
	}
	switch m.files[m.cursor].category {
	case catConflict:
		return "[O] take ours  [T] take theirs"
	case catStaged:
		return "[u] untrack (keep on disk)"
	case catChanged:
		return "[x] discard changes"
	case catUntracked:
		return "[x] delete from disk"
	}
	return ""
}

// prHint returns a one-line hint to create a PR when the branch is pushed
// but has no open PR yet. Empty string when not applicable.
func prHint(m model) string {
	if m.prProvider == nil || m.status == nil {
		return ""
	}
	// Don't hint on default integration branches.
	branch := m.status.Branch
	switch branch {
	case "main", "master", "develop", "trunk", "HEAD", "":
		return ""
	}
	// Only hint when the branch is in sync with its remote (was pushed) and
	// no PR is known for it yet.
	if m.status.Ahead > 0 || m.prStatus != nil {
		return ""
	}
	return fmt.Sprintf("[K] create PR on %s", m.prProvider.Name())
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
	case s.Ahead > 0 && s.Behind > 0:
		return fmt.Sprintf("tip: branches diverged (↑%d local ↓%d remote) - press [P] to choose rebase or merge", s.Ahead, s.Behind)
	case s.Behind > 0:
		return fmt.Sprintf("tip: %d commit(s) available on remote - press [P] to pull", s.Behind)
	case nChanged > 0 && nStaged == 0:
		return fmt.Sprintf("tip: %d file(s) changed - navigate and press [space] to stage", nChanged)
	case nChanged > 0 && nStaged > 0:
		return fmt.Sprintf("tip: %d staged, %d unstaged - press [c] to commit or keep staging", nStaged, nChanged)
	case nStaged > 0:
		return fmt.Sprintf("tip: %d file(s) staged - press [c] to commit", nStaged)
	case s.Ahead > 0:
		if m.prProvider != nil {
			return fmt.Sprintf("tip: %d commit(s) ready - push [p] then [K] to create a PR on %s", s.Ahead, m.prProvider.Name())
		}
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
		if m.prProvider != nil && len(m.prListItems) > 0 {
			return "tip: [↑↓] select PR  [enter] open detail  [K] full list  [l] log"
		}
		return "tip: working tree is clean - edit a file to get started"
	}
}

type cmdBarEntry struct {
	key         string
	displayKey  string
	description string
}

var cmdBarCatalog = []cmdBarEntry{
	{"space", "[space]", "stage/unstage"},
	{"stage-all", "[+]", "stage all"},
	{"hunks", "[h]", "stage by section"},
	{"diff", "[d]", "diff"},
	{"commit", "[c]", "commit"},
	{"push", "[p]", "push"},
	{"pull", "[P]", "pull"},
	{"branch", "[b/B]", "branch"},
	{"log", "[l]", "log"},
	{"amend", "[A]", "amend"},
	{"fetch", "[f]", "fetch"},
	{"stash", "[s/S]", "stash"},
	{"graph", "[g]", "graph"},
	{"reset", "[z]", "reset"},
	{"restore", "[o]", "restore"},
	{"reflog", "[L]", "reflog"},
	{"tags", "[t]", "tags"},
	{"bisect", "[i]", "bisect"},
	{"rebase", "[R]", "rebase"},
	{"worktrees", "[W]", "worktrees"},
	{"remotes", "[O]", "remotes"},
	{"submodules", "[M]", "submodules"},
	{"notes", "[n]", "notes"},
	{"clean", "[X]", "clean"},
	{"abort", "[a]", "abort"},
	{"config", "[C]", "config"},
}

var defaultCmdBarItems = []string{"space", "stage-all", "commit", "push", "pull", "branch", "log"}

func cmdBarEnabledFromConfig(items []string) []bool {
	active := items
	if len(active) == 0 {
		active = defaultCmdBarItems
	}
	set := map[string]bool{}
	for _, k := range active {
		set[k] = true
	}
	enabled := make([]bool, len(cmdBarCatalog))
	for i, e := range cmdBarCatalog {
		enabled[i] = set[e.key]
	}
	return enabled
}

func buildCommandBarItems(enabled []bool) []string {
	var items []string
	for i, on := range enabled {
		if on && i < len(cmdBarCatalog) {
			items = append(items, cmdBarCatalog[i].key)
		}
	}
	return items
}

func (m model) commandBar() string {
	if m.committing {
		return styleDim.Render("  committing...") + "\n"
	}
	if m.pushing {
		return styleDim.Render("  pushing...") + "\n"
	}
	if m.pulling {
		return styleDim.Render("  pulling...") + "\n"
	}
	kb := m.cfg.Keybindings
	labelFor := map[string]string{
		"space":      "[space] stage/unstage",
		"stage-all":  "[+] stage all",
		"hunks":      "[h] stage by section",
		"diff":       "[d] diff",
		"commit":     fmt.Sprintf("[%s] commit", kb.Commit),
		"push":       fmt.Sprintf("[%s] push", kb.Push),
		"pull":       "[P] pull",
		"branch":     "[b/B] branch",
		"log":        "[l] log",
		"amend":      "[A] amend",
		"fetch":      "[f] fetch",
		"stash":      fmt.Sprintf("[%s/%s] stash", kb.Stash, strings.ToUpper(kb.Stash)),
		"graph":      fmt.Sprintf("[%s] graph", kb.Graph),
		"reset":      "[z] reset",
		"restore":    "[o] restore",
		"reflog":     "[L] reflog",
		"tags":       "[t] tags",
		"bisect":     "[i] bisect",
		"rebase":     "[R] rebase",
		"worktrees":  "[W] worktrees",
		"remotes":    "[O] remotes",
		"submodules": "[M] submodules",
		"notes":      "[n] notes",
		"clean":      "[X] clean",
		"abort":      "[a] abort",
		"config":     "[C] config",
	}
	keys := m.cfg.CommandBar.Items
	if len(keys) == 0 {
		keys = defaultCmdBarItems
	}
	var parts []string
	for _, k := range keys {
		if label, ok := labelFor[k]; ok {
			parts = append(parts, label)
		}
	}
	parts = append(parts, "[?] all shortcuts")
	parts = append(parts, fmt.Sprintf("[%s] quit", kb.Quit))

	const sep = "  "
	const indent = "  "
	if m.width <= 0 {
		return styleDim.Render(indent+strings.Join(parts, sep)) + "\n"
	}
	var lines []string
	var row []string
	rowLen := len(indent)
	for _, p := range parts {
		need := len(p)
		if len(row) > 0 {
			need += len(sep)
		}
		if len(row) > 0 && rowLen+need > m.width {
			lines = append(lines, styleDim.Render(indent+strings.Join(row, sep)))
			row = nil
			rowLen = len(indent)
		}
		row = append(row, p)
		rowLen += need
	}
	if len(row) > 0 {
		lines = append(lines, styleDim.Render(indent+strings.Join(row, sep)))
	}
	return strings.Join(lines, "\n") + "\n"
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

// timeAgo returns a human-readable relative time string ("2 hours ago", "3 days ago", etc.).
// Returns an empty string when t is the zero value.
// stashOldThreshold is how long a stash can sit before the stash list flags it
// "⚠ old". This is a pure age check, independent of the stale (base-branch-moved)
// check — a stash can be old, stale, both, or neither.
const stashOldThreshold = 7 * 24 * time.Hour

// isOldStash reports whether a stash created at t has aged past stashOldThreshold.
func isOldStash(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	return time.Since(t) >= stashOldThreshold
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 30*24*time.Hour:
		weeks := int(d.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// writeClipboard is isolated to make it easy to stub in tests and to contain
// the platform-specific clipboard dependency.
func writeClipboard(s string) error {
	return clipboard.WriteAll(s)
}

func (m model) updateDivergedPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r", "R":
		m.panel = panelMain
		m.pulling = true
		m.actionErr = nil
		m.lastInfo = ""
		return m, m.doPullRebase()
	case "m", "M":
		m.panel = panelMain
		m.pulling = true
		m.actionErr = nil
		m.lastInfo = ""
		return m, m.doPullMerge()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) divergedView() string {
	ahead, behind := 0, 0
	if m.status != nil {
		ahead = m.status.Ahead
		behind = m.status.Behind
	}
	title := styleTitle.Render("Pull - branches have diverged")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		styleDim.Render(fmt.Sprintf("↑%d local", ahead)),
		styleChanged.Render(fmt.Sprintf("↓%d remote", behind)),
	))
	b.WriteString("  How do you want to integrate the remote changes?\n\n")
	b.WriteString("  " + styleSelected.Render("[R]") + "  Rebase  - replay your commits on top of the remote (linear history)\n")
	b.WriteString("  " + styleSelected.Render("[M]") + "  Merge   - create a merge commit preserving both histories\n")
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [esc] cancel") + "\n")
	return b.String()
}

func (m model) updatePRPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.prListCursor > 0 {
			m.prListCursor--
		}
	case "down", "j":
		if m.prListCursor < len(m.prListItems)-1 {
			m.prListCursor++
		}
	case "enter":
		if len(m.prListItems) > 0 {
			m.panel = panelPRDetail
		}
	case "o":
		if len(m.prListItems) > 0 {
			item := m.prListItems[m.prListCursor]
			if item.URL != "" {
				ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
				defer cancel()
				_ = m.prProvider.Open(ctx, item.URL)
			}
		}
	case "n":
		if m.prProvider != nil && m.status != nil {
			m, cmd := m.openPRCreatePanel()
			return m, cmd
		}
	case "r":
		return m, m.fetchPRList()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) prView() string {
	title := styleTitle.Render("Pull Requests - " + func() string {
		if m.prProvider != nil {
			return m.prProvider.Name()
		}
		return "no provider"
	}())
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	if m.prListErr != nil {
		b.WriteString(styleConflict.Render("  error: "+m.prListErr.Error()) + "\n\n")
	} else if m.prListLoading {
		b.WriteString(styleDim.Render("  loading...") + "\n\n")
	} else if len(m.prListItems) == 0 {
		b.WriteString(styleDim.Render("  no pull requests") + "\n\n")
	} else {
		for i, item := range m.prListItems {
			cursor := "  "
			if i == m.prListCursor {
				cursor = styleSelected.Render(">>")
			}
			ci := ""
			switch item.CI {
			case "success":
				ci = styleStaged.Render(" ✓")
			case "failure":
				ci = styleConflict.Render(" ✗")
			case "pending":
				ci = styleChanged.Render(" ●")
			}
			state := styleDim.Render("[" + item.State + "]")
			draftBadge := ""
			if item.Draft {
				draftBadge = styleDim.Render(" [draft]")
			}
			b.WriteString(fmt.Sprintf("  %s #%-4d %s %s%s%s\n", cursor, item.Number, state, item.Title, draftBadge, ci))
			if i == m.prListCursor {
				// Show metadata for selected PR.
				if len(item.Labels) > 0 {
					b.WriteString("        " + styleDim.Render("labels: "+strings.Join(item.Labels, ", ")) + "\n")
				}
				if len(item.Reviewers) > 0 {
					b.WriteString("        " + styleDim.Render("reviewers: "+strings.Join(item.Reviewers, ", ")) + "\n")
				}
				if len(item.Assignees) > 0 {
					b.WriteString("        " + styleDim.Render("assignees: "+strings.Join(item.Assignees, ", ")) + "\n")
				}
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [enter] details  [o] open browser  [n] new PR  [r] refresh  [esc] back") + "\n")
	return b.String()
}

func filterIssues(issues []pr.Issue, q string) []pr.Issue {
	if q == "" {
		return issues
	}
	q = strings.ToLower(q)
	var out []pr.Issue
	for _, i := range issues {
		if strings.Contains(strings.ToLower(i.Title), q) ||
			strings.Contains(fmt.Sprintf("%d", i.Number), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m model) updateIssuesPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := filterIssues(m.issues, m.issueFilter)

	if m.issueFiltering {
		switch msg.String() {
		case "enter", "esc":
			m.issueFiltering = false
			m.issueFilterInput.Blur()
		default:
			var cmd tea.Cmd
			m.issueFilterInput, cmd = m.issueFilterInput.Update(msg)
			m.issueFilter = m.issueFilterInput.Value()
			m.issueCursor = 0
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.issueCursor > 0 {
			m.issueCursor--
		}
	case "down", "j":
		if m.issueCursor < len(visible)-1 {
			m.issueCursor++
		}
	case "enter", "o":
		if len(visible) > 0 && visible[m.issueCursor].URL != "" {
			url := visible[m.issueCursor].URL
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			defer cancel()
			// Re-use the PR provider's Open to avoid duplicating browser-open logic.
			_ = m.prProvider.Open(ctx, url)
		}
	case "b":
		if len(visible) == 0 {
			break
		}
		if _, ok := m.prProvider.(pr.IssueProvider); !ok {
			break
		}
		issue := visible[m.issueCursor]
		branchName := fmt.Sprintf("issue/%d", issue.Number)
		m.panel = panelMain
		return m, m.doCreateIssueBranch(issue.Number, branchName)
	case "/":
		m.issueFiltering = true
		m.issueFilterInput.Focus()
	case "r":
		m.issues = nil
		m.issueCursor = 0
		return m, m.fetchIssues()
	case "esc", m.cfg.Keybindings.Quit:
		if m.issueFilter != "" {
			m.issueFilter = ""
			m.issueFilterInput.SetValue("")
			m.issueCursor = 0
			return m, nil
		}
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) issuesView() string {
	visible := filterIssues(m.issues, m.issueFilter)
	title := "Issues"
	if len(m.issues) > 0 {
		if m.issueFilter != "" {
			title = fmt.Sprintf("Issues (%d/%d)  %s", len(visible), len(m.issues), styleCmd.Render("["+m.issueFilter+"]"))
		} else {
			title = fmt.Sprintf("Issues (%d)", len(m.issues))
		}
	}
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render(title) + "\n\n")

	if m.issueFiltering {
		b.WriteString("  " + styleDim.Render("/") + " " + m.issueFilterInput.View() + "\n\n")
	}

	if m.issues == nil {
		b.WriteString(styleDim.Render("  loading...") + "\n")
	} else if len(visible) == 0 {
		if m.issueFilter != "" {
			b.WriteString(styleDim.Render("  no issues matched - press esc to clear") + "\n")
		} else {
			b.WriteString(styleDim.Render("  no open issues") + "\n")
		}
	} else {
		overhead := 6
		if m.issueFiltering {
			overhead += 2
		}
		visibleLines := m.height - overhead
		if visibleLines < 1 {
			visibleLines = 1
		}
		start := 0
		if m.issueCursor >= visibleLines {
			start = m.issueCursor - visibleLines + 1
		}
		end := start + visibleLines
		if end > len(visible) {
			end = len(visible)
		}
		for i := start; i < end; i++ {
			issue := visible[i]
			cursor := "  "
			if i == m.issueCursor {
				cursor = styleSelected.Render(">>")
			}
			b.WriteString(fmt.Sprintf("  %s #%-4d %s\n", cursor, issue.Number, issue.Title))
			if i == m.issueCursor {
				if len(issue.Labels) > 0 {
					b.WriteString("        " + styleDim.Render("labels: "+strings.Join(issue.Labels, ", ")) + "\n")
				}
				if len(issue.Assignees) > 0 {
					b.WriteString("        " + styleDim.Render("assignees: "+strings.Join(issue.Assignees, ", ")) + "\n")
				}
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [enter/o] open  [b] create branch  [/] search  [r] refresh  [esc] back") + "\n")
	return b.String()
}

func (m model) updateSSHPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.sshCursor > 0 {
			m.sshCursor--
		}
	case "down", "j":
		if m.sshCursor < len(m.sshKeys)-1 {
			m.sshCursor++
		}
	case "t":
		// Test connections to known git hosts.
		hosts := []string{"github.com", "gitlab.com", "bitbucket.org"}
		cmds := make([]tea.Cmd, len(hosts))
		for i, h := range hosts {
			cmds[i] = doTestSSHHost(h)
		}
		return m, tea.Batch(cmds...)
	case "r":
		return m, doLoadSSHKeys()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) sshView() string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render("SSH Keys") + "\n\n")

	if m.sshKeys == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else if len(m.sshKeys) == 0 {
		b.WriteString("  " + styleDim.Render("no SSH keys found in ~/.ssh/") + "\n")
	} else {
		for i, key := range m.sshKeys {
			cursor := "  "
			if i == m.sshCursor {
				cursor = styleSelected.Render(">>")
			}
			name := styleCmd.Render(fmt.Sprintf("%-20s", key.Name))
			fp := styleDim.Render(key.Fingerprint)
			b.WriteString(fmt.Sprintf("  %s %s  %s\n", cursor, name, fp))
			if i == m.sshCursor && key.Comment != "" {
				b.WriteString("        " + styleDim.Render("comment: "+key.Comment) + "\n")
				b.WriteString("        " + styleDim.Render("pub: "+key.PubKeyFile) + "\n")
			}
		}
	}

	b.WriteString("\n")
	if len(m.sshTestResults) > 0 {
		b.WriteString("  " + styleSection.Render("Connection tests") + "\n")
		hosts := []string{"github.com", "gitlab.com", "bitbucket.org"}
		for _, h := range hosts {
			result, tested := m.sshTestResults[h]
			if !tested {
				continue
			}
			var badge string
			switch result {
			case "ok":
				badge = styleStaged.Render("ok ")
			case "...":
				badge = styleDim.Render("...")
			default:
				badge = styleChanged.Render("fail")
			}
			b.WriteString(fmt.Sprintf("  %s  %s\n", badge, styleDim.Render(h)))
		}
		b.WriteString("\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [t] test connections  [r] refresh  [esc] back") + "\n"
}

func (m model) updateLFSPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.lfsTracking {
		switch msg.String() {
		case "enter":
			pattern := strings.TrimSpace(m.lfsTrackInput.Value())
			m.lfsTracking = false
			if pattern != "" {
				return m, tea.Batch(m.doLFSTrack(pattern), m.doLoadLFSData())
			}
		case "esc":
			m.lfsTracking = false
		default:
			var cmd tea.Cmd
			m.lfsTrackInput, cmd = m.lfsTrackInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.lfsCursor > 0 {
			m.lfsCursor--
		}
	case "down", "j":
		if m.lfsCursor < len(m.lfsPatterns)-1 {
			m.lfsCursor++
		}
	case "p":
		if m.lfsInstalled {
			m.panel = panelMain
			return m, m.doLFSPull()
		}
	case "P":
		if m.lfsInstalled {
			m.panel = panelMain
			return m, m.doLFSPush()
		}
	case "t":
		if m.lfsInstalled {
			ti := textinput.New()
			ti.Placeholder = "pattern, e.g. *.psd or assets/**"
			ti.Focus()
			ti.CharLimit = 200
			ti.Width = m.width - 6
			m.lfsTrackInput = ti
			m.lfsTracking = true
		}
	case "u":
		if m.lfsInstalled && m.lfsCursor < len(m.lfsPatterns) {
			pattern := m.lfsPatterns[m.lfsCursor]
			return m, tea.Batch(m.doLFSUntrack(pattern), m.doLoadLFSData())
		}
	case "i":
		if !m.lfsInstalled {
			m.panel = panelMain
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				err := m.git.LFSInstall(ctx)
				return actionDoneMsg{cmd: "git lfs install", err: err, info: "LFS hooks installed"}
			}
		}
	case "r":
		return m, m.doLoadLFSData()
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) lfsView() string {
	var b strings.Builder

	installBadge := styleStaged.Render("installed")
	if !m.lfsInstalled {
		installBadge = styleChanged.Render("not installed")
	}
	b.WriteString("\n  " + styleTitle.Render("LFS") + "  " + styleDim.Render("Git Large File Storage") + "  " + installBadge + "\n\n")

	if !m.lfsInstalled {
		b.WriteString("  " + styleDim.Render("git-lfs is not installed. Press [i] to run `git lfs install`,") + "\n")
		b.WriteString("  " + styleDim.Render("or install it from https://git-lfs.com") + "\n")
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [i] git lfs install  [r] refresh  [esc] back") + "\n"
	}

	if m.lfsTracked == nil {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [esc] back") + "\n"
	}

	if m.lfsTracking {
		b.WriteString("  " + styleSection.Render("Track pattern") + "\n\n")
		b.WriteString("  " + styleDim.Render("pattern: ") + m.lfsTrackInput.View() + "\n\n")
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [enter] confirm  [esc] cancel") + "\n"
	}

	// Tracked patterns section.
	if len(m.lfsPatterns) == 0 {
		b.WriteString("  " + styleDim.Render("no patterns tracked yet  - press [t] to add one") + "\n")
	} else {
		b.WriteString("  " + styleSection.Render(fmt.Sprintf("Tracked patterns (%d)", len(m.lfsPatterns))) + "\n\n")
		for i, pat := range m.lfsPatterns {
			cursor := "  "
			if i == m.lfsCursor {
				cursor = styleSelected.Render(">>")
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", cursor, styleCmd.Render(pat)))
		}
	}

	// Tracked files count.
	if len(m.lfsTracked) > 0 {
		b.WriteString("\n  " + styleDim.Render(fmt.Sprintf("%d file(s) currently stored in LFS", len(m.lfsTracked))) + "\n")
	}

	// Status section.
	if m.lfsStatus != "" {
		b.WriteString("\n  " + styleSection.Render("Status") + "\n")
		for _, line := range strings.Split(strings.TrimRight(m.lfsStatus, "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				b.WriteString("  " + styleDim.Render(line) + "\n")
			}
		}
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	bar := "  [p] pull  [P] push  [t] track pattern  [u] untrack selected  [r] refresh  [esc] back"
	return content + styleDim.Render(bar) + "\n"
}

func (m model) updateDashboardPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.dashCursor > 0 {
			m.dashCursor--
		}
	case "down", "j":
		if m.dashCursor < len(m.dashEntries)-1 {
			m.dashCursor++
		}
	case "r":
		m.dashLoading = true
		m.dashEntries = nil
		return m, m.doLoadDashboard()
	case "y":
		if m.dashCursor < len(m.dashEntries) {
			_ = clipboard.WriteAll(m.dashEntries[m.dashCursor].Path)
		}
	case "esc", m.cfg.Keybindings.Quit:
		m.panel = panelMain
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// init panel - shown when bonsai is opened outside a git repository
// ---------------------------------------------------------------------------

func (m model) updateInitPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i":
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			defer cancel()
			err := m.git.InitRepo(ctx)
			return actionDoneMsg{cmd: "git init", err: err, info: "initialized empty git repository"}
		}
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) initView() string {
	cwd, _ := os.Getwd()
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + styleTitle.Render("No git repository found") + "\n\n")
	b.WriteString("  " + styleDim.Render(cwd) + "\n\n")
	b.WriteString("  " + styleChanged.Render("this directory is not a git repository") + "\n\n")
	b.WriteString("  " + styleDim.Render("[i] initialize here   [q] quit") + "\n")
	return b.String()
}

func (m model) dashboardView() string {
	var b strings.Builder
	b.WriteString("\n  " + styleTitle.Render("Multi-repo Dashboard"))
	if len(m.dashEntries) > 0 {
		b.WriteString("  " + styleDim.Render(fmt.Sprintf("(%d repos)", len(m.dashEntries))))
	}
	b.WriteString("\n\n")

	if m.dashLoading {
		b.WriteString("  " + styleDim.Render("scanning repos...") + "\n")
		content := b.String()
		lines := strings.Count(content, "\n")
		if pad := m.height - lines - 1; pad > 0 {
			content += strings.Repeat("\n", pad)
		}
		return content + styleDim.Render("  [esc] back") + "\n"
	}

	repos := m.cfg.Dashboard.Repos
	if len(repos) == 0 {
		b.WriteString("  " + styleDim.Render("no repos configured") + "\n\n")
		b.WriteString("  " + styleDim.Render("add paths to [dashboard] repos in ~/.bonsai.toml or .bonsai.toml:") + "\n")
		b.WriteString("  " + styleCmd.Render(`[dashboard]`) + "\n")
		b.WriteString("  " + styleCmd.Render(`repos = ["~/projects/api", "~/projects/frontend"]`) + "\n")
	} else if len(m.dashEntries) == 0 {
		b.WriteString("  " + styleDim.Render("loading...") + "\n")
	} else {
		for i, e := range m.dashEntries {
			cursor := "  "
			if i == m.dashCursor {
				cursor = styleSelected.Render(">>")
			}
			nameStyle := styleCmd
			if e.Error != "" {
				nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
			}
			name := nameStyle.Render(fmt.Sprintf("%-20s", e.Name))

			var status strings.Builder
			if e.Error != "" {
				status.WriteString(styleChanged.Render(e.Error))
			} else {
				status.WriteString(styleDim.Render(fmt.Sprintf("%-18s", e.Branch)))
				if e.Dirty {
					status.WriteString(styleChanged.Render("*"))
				} else {
					status.WriteString(" ")
				}
				if e.Ahead > 0 {
					status.WriteString(styleStaged.Render(fmt.Sprintf(" ↑%d", e.Ahead)))
				}
				if e.Behind > 0 {
					status.WriteString(styleChanged.Render(fmt.Sprintf(" ↓%d", e.Behind)))
				}
			}

			b.WriteString(fmt.Sprintf("  %s %s  %s\n", cursor, name, status.String()))
			if i == m.dashCursor && e.LastCommit != "" {
				msg := e.LastCommit
				if len(msg) > 60 {
					msg = msg[:57] + "..."
				}
				b.WriteString("        " + styleDim.Render(msg) + "\n")
				b.WriteString("        " + styleDim.Render(e.Path) + "\n")
			}
		}
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [↑↓/jk] navigate  [y] copy path  [r] refresh  [esc] back") + "\n"
}

func (m model) updatePRDetailPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.prListItems) == 0 || m.prListCursor >= len(m.prListItems) {
		m.panel = panelPR
		return m, nil
	}
	item := m.prListItems[m.prListCursor]
	switch msg.String() {
	case "esc":
		m.panel = panelPR
	case "ctrl+c":
		return m, tea.Quit
	case "o":
		if item.URL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
			defer cancel()
			_ = m.prProvider.Open(ctx, item.URL)
		}
	case "d":
		differ, ok := m.prProvider.(pr.PRDiffer)
		if !ok {
			m.actionErr = fmt.Errorf("this provider does not support PR diffs")
			break
		}
		num := item.Number
		m.diffLines = nil
		m.diffScroll = 0
		m.diffCursor = 0
		m.prDiffNumber = num
		m.prLineCommentActive = false
		m.diffOrigin = panelPR
		m.panel = panelDiff
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			raw, err := differ.Diff(ctx, num)
			if err != nil {
				return actionDoneMsg{cmd: "pr diff", err: fmt.Errorf("pr diff: %w", err)}
			}
			var lines []string
			if raw != "" {
				lines = strings.Split(strings.TrimRight(raw, "\n"), "\n")
			}
			return diffMsg{title: fmt.Sprintf("PR #%d diff", num), lines: lines}
		}
	case "a":
		reviewer, ok := m.prProvider.(pr.PRReviewer)
		if !ok {
			m.actionErr = fmt.Errorf("this provider does not support PR reviews")
			break
		}
		num := item.Number
		prov := reviewer
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := prov.Approve(ctx, num)
			return actionDoneMsg{cmd: fmt.Sprintf("pr review --approve #%d", num), err: err,
				info: fmt.Sprintf("approved PR #%d", num)}
		}
	case "A":
		if _, ok := m.prProvider.(pr.PRReviewer); !ok {
			m.actionErr = fmt.Errorf("this provider does not support PR reviews")
			break
		}
		m.prReviewNumber = item.Number
		m.prReviewMode = "changes"
		ti := textinput.New()
		ti.Placeholder = "reason for requesting changes (required)"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = m.width - 6
		m.prReviewInput = ti
		m.panel = panelPRReview
	case "c":
		if _, ok := m.prProvider.(pr.PRReviewer); !ok {
			m.actionErr = fmt.Errorf("this provider does not support PR reviews")
			break
		}
		m.prReviewNumber = item.Number
		m.prReviewMode = "comment"
		ti := textinput.New()
		ti.Placeholder = "comment text"
		ti.Focus()
		ti.CharLimit = 512
		ti.Width = m.width - 6
		m.prReviewInput = ti
		m.panel = panelPRReview
	case "m":
		merger, ok := m.prProvider.(pr.PRMerger)
		if !ok {
			m.actionErr = fmt.Errorf("this provider does not support merging PRs")
			break
		}
		if method := m.cfg.PR.MergeMethod; method != "" {
			num := item.Number
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				err := merger.MergePR(ctx, num, method)
				cmd := fmt.Sprintf("pr merge #%d --%s", num, method)
				info := ""
				if err == nil {
					info = fmt.Sprintf("merged PR #%d (%s)", num, method)
				}
				return prMergeResultMsg{cmd: cmd, info: info, err: err}
			}
		}
		m.prMergeNumber = item.Number
		m.prMergeCursor = 0
		m.panel = panelPRMerge
	case "y":
		_ = clipboard.WriteAll(item.URL)
	}
	return m, nil
}

func (m model) prDetailView() string {
	if len(m.prListItems) == 0 || m.prListCursor >= len(m.prListItems) {
		return ""
	}
	item := m.prListItems[m.prListCursor]

	stateStyle := styleStaged
	if item.State == "closed" || item.State == "merged" {
		stateStyle = styleDim
	}

	ciIcon := ""
	switch item.CI {
	case "success":
		ciIcon = styleStaged.Render(" ✓")
	case "failure":
		ciIcon = styleConflict.Render(" ✗")
	case "pending":
		ciIcon = styleChanged.Render(" ●")
	}

	title := styleTitle.Render(fmt.Sprintf("PR #%d", item.Number))
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	b.WriteString("  " + item.Title + "\n\n")

	b.WriteString("  State:   " + stateStyle.Render(item.State))
	if item.Draft {
		b.WriteString("  " + styleDim.Render("[draft]"))
	}
	b.WriteString(ciIcon + "\n")

	if len(item.Labels) > 0 {
		b.WriteString("  Labels:  " + strings.Join(item.Labels, ", ") + "\n")
	}
	if len(item.Reviewers) > 0 {
		b.WriteString("  Reviews: " + strings.Join(item.Reviewers, ", ") + "\n")
	}
	if len(item.Assignees) > 0 {
		b.WriteString("  Assigned:" + strings.Join(item.Assignees, ", ") + "\n")
	}
	if item.URL != "" {
		b.WriteString("  URL:     " + styleDim.Render(item.URL) + "\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}

	mergeHint := "[m] merge"
	if m.cfg.PR.MergeMethod != "" {
		mergeHint = fmt.Sprintf("[m] merge (%s)", m.cfg.PR.MergeMethod)
	}
	hints := fmt.Sprintf("  [o] open browser  [d] diff  [a] approve  [A] req changes  [c] comment  %s  [y] copy URL  [esc] back", mergeHint)
	return content + styleDim.Render(hints) + "\n"
}

func (m model) updatePRReviewPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		body := strings.TrimSpace(m.prReviewInput.Value())
		num := m.prReviewNumber
		mode := m.prReviewMode
		reviewer := m.prProvider.(pr.PRReviewer)
		m.panel = panelPR
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			var err error
			var info string
			switch mode {
			case "changes":
				err = reviewer.RequestChanges(ctx, num, body)
				info = fmt.Sprintf("requested changes on PR #%d", num)
			case "comment":
				err = reviewer.ReviewComment(ctx, num, body)
				info = fmt.Sprintf("commented on PR #%d", num)
			}
			return actionDoneMsg{cmd: "pr review #" + fmt.Sprintf("%d", num), err: err, info: info}
		}
	case "esc", "ctrl+c":
		m.panel = panelPR
	default:
		var cmd tea.Cmd
		m.prReviewInput, cmd = m.prReviewInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) prReviewView() string {
	action := map[string]string{
		"changes": "Request Changes",
		"comment": "Add Comment",
	}[m.prReviewMode]
	title := styleTitle.Render(action + fmt.Sprintf(" - PR #%d", m.prReviewNumber))
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")
	b.WriteString("  " + m.prReviewInput.View() + "\n\n")
	if m.actionErr != nil {
		b.WriteString("  " + styleChanged.Render("error: "+m.actionErr.Error()) + "\n\n")
	}
	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [enter] submit  [esc] cancel") + "\n"
}

// openPRCreatePanel initialises the PR create form and switches to panelPRCreate.
// It returns a command that fetches the HEAD commit to pre-fill title/body.
func (m model) openPRCreatePanel() (model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "PR title"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = m.width - 6
	m.prCreateTitleInput = ti

	ta := textarea.New()
	ta.Placeholder = "Description (optional)"
	ta.CharLimit = 8000
	ta.SetWidth(m.width - 6)
	ta.SetHeight(6)
	m.prCreateBodyTA = ta

	base := textinput.New()
	base.Placeholder = "Base branch (e.g. main)"
	base.CharLimit = 100
	base.Width = m.width - 6
	m.prCreateBaseInput = base

	m.prCreateField = 0
	m.prCreateDraft = false
	m.panel = panelPRCreate

	runner := m.git
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		detail, err := runner.ShowStat(ctx, "HEAD")
		if err != nil || detail == nil {
			return prCreatePrefillMsg{}
		}
		return prCreatePrefillMsg{subject: detail.Subject, body: detail.Body}
	}
}

func (m model) submitPRCreate() (tea.Model, tea.Cmd) {
	title := strings.TrimSpace(m.prCreateTitleInput.Value())
	if title == "" {
		m.actionErr = fmt.Errorf("title is required")
		return m, nil
	}
	opts := pr.PRCreateOpts{
		Branch: m.status.Branch,
		Title:  title,
		Body:   m.prCreateBodyTA.Value(),
		Base:   strings.TrimSpace(m.prCreateBaseInput.Value()),
		Draft:  m.prCreateDraft,
	}
	prov := m.prProvider
	runner := m.git
	branch := m.status.Branch
	upstream := m.status.Upstream
	m.panel = panelPR
	m.prListLoading = true
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Push the branch first if it has no upstream tracking ref.
		if upstream == "" {
			if pushErr := runner.PushWithOptions(ctx, false, true, "origin", branch); pushErr != nil {
				return actionDoneMsg{cmd: "gh pr create", err: fmt.Errorf("push failed: %w", pushErr)}
			}
		}
		err := prov.CreatePR(ctx, opts)
		return actionDoneMsg{cmd: "gh pr create", err: err, info: fmt.Sprintf("created PR: %s", opts.Title)}
	}
}

func (m model) updatePRCreatePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.panel = panelPR
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "d":
		// toggle draft — only when not editing a text field (title or body)
		if m.prCreateField != 0 && m.prCreateField != 1 {
			m.prCreateDraft = !m.prCreateDraft
			return m, nil
		}
	case "tab", "shift+tab":
		// cycle through fields
		if msg.String() == "tab" {
			m.prCreateField = (m.prCreateField + 1) % 3
		} else {
			m.prCreateField = (m.prCreateField + 2) % 3
		}
		switch m.prCreateField {
		case 0:
			m.prCreateTitleInput.Focus()
			m.prCreateBodyTA.Blur()
			m.prCreateBaseInput.Blur()
		case 1:
			m.prCreateTitleInput.Blur()
			m.prCreateBodyTA.Focus()
			m.prCreateBaseInput.Blur()
		case 2:
			m.prCreateTitleInput.Blur()
			m.prCreateBodyTA.Blur()
			m.prCreateBaseInput.Focus()
		}
		return m, nil
	case "ctrl+s":
		return m.submitPRCreate()
	case "enter":
		// in title or base field: submit; in body textarea: let textarea handle (newline)
		if m.prCreateField != 1 {
			return m.submitPRCreate()
		}
	}

	var cmd tea.Cmd
	switch m.prCreateField {
	case 0:
		m.prCreateTitleInput, cmd = m.prCreateTitleInput.Update(msg)
	case 1:
		m.prCreateBodyTA, cmd = m.prCreateBodyTA.Update(msg)
	case 2:
		m.prCreateBaseInput, cmd = m.prCreateBaseInput.Update(msg)
	}
	return m, cmd
}

func (m model) prCreateView() string {
	title := styleTitle.Render("Create Pull Request")
	var b strings.Builder
	b.WriteString("\n  " + title + "\n\n")

	labelStyle := styleDim
	activeLabel := styleStaged // bright green highlight for focused field

	titleLabel := labelStyle.Render("  Title")
	if m.prCreateField == 0 {
		titleLabel = activeLabel.Render("  Title")
	}
	b.WriteString(titleLabel + "\n")
	b.WriteString("  " + m.prCreateTitleInput.View() + "\n\n")

	bodyLabel := labelStyle.Render("  Description")
	if m.prCreateField == 1 {
		bodyLabel = activeLabel.Render("  Description")
	}
	b.WriteString(bodyLabel + "\n")
	b.WriteString("  " + m.prCreateBodyTA.View() + "\n\n")

	baseLabel := labelStyle.Render("  Base branch")
	if m.prCreateField == 2 {
		baseLabel = activeLabel.Render("  Base branch")
	}
	b.WriteString(baseLabel + "\n")
	b.WriteString("  " + m.prCreateBaseInput.View() + "\n\n")

	draftIndicator := "[ ]"
	if m.prCreateDraft {
		draftIndicator = "[x]"
	}
	b.WriteString("  " + labelStyle.Render(draftIndicator+" Draft PR") + "\n\n")

	if m.actionErr != nil {
		b.WriteString("  " + styleConflict.Render("error: "+m.actionErr.Error()) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [tab] next field  [d] toggle draft  [ctrl+s] submit  [esc] cancel") + "\n"
}

func (m model) updatePRMergePanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.panel = panelPR
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.prMergeCursor > 0 {
			m.prMergeCursor--
		}
	case "down", "j":
		if m.prMergeCursor < 2 {
			m.prMergeCursor++
		}
	case "enter":
		methods := []string{"merge", "squash", "rebase"}
		method := methods[m.prMergeCursor]
		merger := m.prProvider.(pr.PRMerger)
		num := m.prMergeNumber
		m.panel = panelPR
		m.prListLoading = true
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := merger.MergePR(ctx, num, method)
			return prMergeResultMsg{
				cmd:  fmt.Sprintf("gh pr merge #%d --%s", num, method),
				info: fmt.Sprintf("merged PR #%d (%s)", num, method),
				err:  err,
			}
		}
	}
	return m, nil
}

func (m model) prMergeView() string {
	item := pr.PRStatus{}
	for _, it := range m.prListItems {
		if it.Number == m.prMergeNumber {
			item = it
			break
		}
	}
	title := styleTitle.Render(fmt.Sprintf("Merge PR #%d", m.prMergeNumber))
	var b strings.Builder
	b.WriteString("\n  " + title + "\n")
	if item.Title != "" {
		b.WriteString("  " + styleDim.Render(item.Title) + "\n")
	}
	b.WriteString("\n")

	opts := []struct {
		label string
		desc  string
	}{
		{"Merge commit", "keep all commits, add a merge commit"},
		{"Squash and merge", "squash into one commit on base branch"},
		{"Rebase and merge", "rebase commits onto base branch, no merge commit"},
	}
	for i, opt := range opts {
		cursor := "  "
		if i == m.prMergeCursor {
			cursor = styleSelected.Render(">>")
		}
		b.WriteString(cursor + " " + opt.label + "\n")
		b.WriteString("     " + styleDim.Render(opt.desc) + "\n\n")
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	if pad := m.height - lines - 1; pad > 0 {
		content += strings.Repeat("\n", pad)
	}
	return content + styleDim.Render("  [↑/↓] select  [enter] merge  [esc] cancel") + "\n"
}

// fetchPRStatus fetches the current PR status in the background.
func (m model) fetchPRStatus() tea.Cmd {
	if m.prProvider == nil || m.status == nil {
		return nil
	}
	branch := m.status.Branch
	prov := m.prProvider
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s, err := prov.CurrentPR(ctx, branch)
		return prStatusMsg{status: s, err: err}
	}
}

// fetchOverviewLog fetches the 5 most recent commits for the clean-tree overview.
func (m model) fetchOverviewLog() tea.Cmd {
	g := m.git
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		entries, err := g.Log(ctx, 5)
		if err != nil {
			return overviewLogMsg(nil)
		}
		return overviewLogMsg(entries)
	}
}

// fetchPRList fetches all open PRs in the background.
func (m model) fetchPRList() tea.Cmd {
	if m.prProvider == nil {
		return nil
	}
	prov := m.prProvider
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		items, err := prov.ListPRs(ctx)
		return prListMsg{items: items, err: err}
	}
}

func (m model) fetchIssues() tea.Cmd {
	if m.prProvider == nil {
		return nil
	}
	ip, ok := m.prProvider.(pr.IssueProvider)
	if !ok {
		return func() tea.Msg { return issueListMsg{err: fmt.Errorf("this provider does not support issues")} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		items, err := ip.ListIssues(ctx)
		return issueListMsg{items: items, err: err}
	}
}

func (m model) doCreateIssueBranch(number int, branchName string) tea.Cmd {
	ip := m.prProvider.(pr.IssueProvider)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := ip.CreateIssueBranch(ctx, number, branchName)
		info := ""
		if err == nil {
			info = fmt.Sprintf("created branch %s for issue #%d", branchName, number)
		}
		return actionDoneMsg{cmd: fmt.Sprintf("issue branch #%d", number), err: err, info: info}
	}
}

func (m model) doLoadProtectedBranches() tea.Cmd {
	if m.prProvider == nil {
		return nil
	}
	checker, ok := m.prProvider.(pr.ProtectionChecker)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		names, err := checker.ProtectedBranches(ctx)
		if err != nil {
			return nil // best-effort, not surfaced as error
		}
		result := make(map[string]bool, len(names))
		for _, n := range names {
			result[n] = true
		}
		return protectedBranchesMsg(result)
	}
}

func (m model) doLoadMergedBranches() tea.Cmd {
	g := m.git
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		// find the default branch to check merged-into
		target := "main"
		branches, err := g.Branches(ctx)
		if err == nil {
			for _, b := range branches {
				if b.Name == "main" || b.Name == "master" {
					target = b.Name
					break
				}
			}
		}
		names, err := g.MergedBranches(ctx, target)
		if err != nil {
			return nil
		}
		result := make(map[string]bool, len(names))
		for _, n := range names {
			result[n] = true
		}
		return mergedBranchesMsg(result)
	}
}

func (m model) doLoadSquashedBranches() tea.Cmd {
	g := m.git
	return func() tea.Msg {
		// commit-tree + cherry per branch is heavier than --merged, so allow
		// more headroom than the default gitTimeout.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		target := "main"
		branches, err := g.Branches(ctx)
		if err == nil {
			for _, b := range branches {
				if b.Name == "main" || b.Name == "master" {
					target = b.Name
					break
				}
			}
		}
		names, err := g.SquashMergedBranches(ctx, target)
		if err != nil {
			return nil
		}
		result := make(map[string]bool, len(names))
		for _, n := range names {
			result[n] = true
		}
		return squashedBranchesMsg(result)
	}
}

func doLoadSSHKeys() tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return sshKeyListMsg(nil)
		}
		sshDir := filepath.Join(home, ".ssh")
		entries, err := os.ReadDir(sshDir)
		if err != nil {
			return sshKeyListMsg(nil)
		}
		var keys []sshKeyEntry
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".pub") {
				continue
			}
			pubPath := filepath.Join(sshDir, name)
			data, err := os.ReadFile(pubPath)
			if err != nil {
				continue
			}
			fields := strings.Fields(string(data))
			comment := ""
			if len(fields) >= 3 {
				comment = fields[2]
			}
			baseName := strings.TrimSuffix(name, ".pub")
			// Run ssh-keygen -lf to get fingerprint.
			fp := ""
			if out, err := exec.Command("ssh-keygen", "-lf", pubPath).Output(); err == nil {
				parts := strings.Fields(string(out))
				if len(parts) >= 2 {
					fp = parts[1]
				}
			}
			keys = append(keys, sshKeyEntry{
				Name:        baseName,
				PubKeyFile:  pubPath,
				Fingerprint: fp,
				Comment:     comment,
			})
		}
		return sshKeyListMsg(keys)
	}
}

func doTestSSHHost(host string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "ssh", "-T", "-o", "StrictHostKeyChecking=accept-new",
			"-o", "BatchMode=yes", "git@"+host)
		err := cmd.Run()
		// ssh -T to git hosts returns exit code 1 even on success ("Hi username!").
		stderr, _ := cmd.CombinedOutput()
		result := "fail"
		out := strings.ToLower(string(stderr))
		if strings.Contains(out, "successfully authenticated") ||
			strings.Contains(out, "hi ") ||
			strings.Contains(out, "welcome to") ||
			err == nil {
			result = "ok"
		}
		return sshTestMsg{host: host, result: result}
	}
}

func (m model) doLoadLFSData() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		installed := git.IsLFSInstalled()
		tracked, _ := m.git.LFSTrackedFiles(ctx)
		patterns, _ := m.git.LFSTrackedPatterns(ctx)
		status, _ := m.git.LFSStatus(ctx)
		return lfsDataMsg{tracked: tracked, patterns: patterns, status: status, installed: installed}
	}
}

func (m model) doLFSPull() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		err := m.git.LFSPull(ctx)
		return actionDoneMsg{cmd: "git lfs pull", err: err, info: "LFS objects downloaded"}
	}
}

func (m model) doLFSPush() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		err := m.git.LFSPush(ctx)
		return actionDoneMsg{cmd: "git lfs push --all origin", err: err, info: "LFS objects pushed to origin"}
	}
}

func (m model) doLFSTrack(pattern string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := m.git.LFSTrack(ctx, pattern)
		return actionDoneMsg{cmd: "git lfs track " + pattern, err: err, info: "tracking " + pattern}
	}
}

func (m model) doLFSUntrack(pattern string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := m.git.LFSUntrack(ctx, pattern)
		return actionDoneMsg{cmd: "git lfs untrack " + pattern, err: err, info: "untracked " + pattern}
	}
}

func (m model) doLoadDashboard() tea.Cmd {
	repos := m.cfg.Dashboard.Repos
	if len(repos) == 0 {
		return func() tea.Msg { return dashboardMsg(nil) }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		entries := make([]git.DashboardEntry, 0, len(repos))
		for _, raw := range repos {
			// Expand ~ prefix.
			path := raw
			if strings.HasPrefix(path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					path = filepath.Join(home, path[2:])
				}
			}
			entries = append(entries, git.QuickStatus(ctx, path))
		}
		return dashboardMsg(entries)
	}
}

// gitflowBranchType returns the gitflow type ("feature", "bugfix", "release",
// "hotfix") for branch, or "" if it does not match any configured prefix.
func gitflowBranchType(branch string, cfg *config.Config) string {
	for bType, rule := range cfg.Conventions.Branches {
		if rule.Prefix != "" && strings.HasPrefix(branch, rule.Prefix) {
			return bType
		}
	}
	return ""
}

// gitflowMainBranch returns the main/master branch name by checking which
// common names exist in the branch list. Falls back to "main".
func gitflowMainBranch(branches []git.Branch) string {
	for _, name := range []string{"main", "master"} {
		for _, b := range branches {
			if b.Name == name {
				return name
			}
		}
	}
	return "main"
}

// gitflowDevBranch returns the develop branch name. Falls back to "develop".
func gitflowDevBranch(branches []git.Branch) string {
	for _, name := range []string{"develop", "development", "dev"} {
		for _, b := range branches {
			if b.Name == name {
				return name
			}
		}
	}
	return "develop"
}

func (m model) doFinishGitflowBranch(branch, branchType, mainBranch, devBranch string) tea.Cmd {
	g := m.git
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		var err error
		var info string
		switch branchType {
		case "feature", "bugfix":
			err = g.FinishBranch(ctx, branch, devBranch)
			info = fmt.Sprintf("finished %s: merged to %s, deleted branch", branch, devBranch)
		case "release":
			tagName := strings.TrimPrefix(branch, "release/")
			err = g.FinishRelease(ctx, branch, mainBranch, devBranch, tagName)
			info = fmt.Sprintf("finished %s: merged to %s and %s, tagged %s, deleted branch", branch, mainBranch, devBranch, tagName)
		case "hotfix":
			tagName := ""
			err = g.FinishRelease(ctx, branch, mainBranch, devBranch, tagName)
			info = fmt.Sprintf("finished %s: merged to %s and %s, deleted branch", branch, mainBranch, devBranch)
		default:
			err = fmt.Errorf("unknown branch type %q for gitflow finish", branchType)
		}
		return actionDoneMsg{cmd: "gitflow finish " + branch, err: err, info: info}
	}
}

// Run starts the bonsai TUI. mdb may be nil when metrics are disabled.
func Run(cfg *config.Config, mdb *metrics.DB, version string) error {
	g := git.New()
	fi := textinput.New()
	fi.Placeholder = "message text  |  author:name  |  since:2026-01-01  |  until:2026-03-01"
	fi.CharLimit = 120

	newFilter := func(placeholder string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 80
		return ti
	}
	branchFI := newFilter("branch name")
	stashFI := newFilter("ref or description")
	tagFI := newFilter("tag name")
	reflogFI := newFilter("hash, ref, action or message")
	issueFI := newFilter("issue number or title")

	usagePath, _ := config.UsageFilePath()
	usageData, _ := usage.Load(usagePath)
	if usageData == nil {
		usageData = &usage.Data{
			Counts:     map[string]int{},
			Suppressed: map[string]bool{},
			Prompted:   map[string]bool{},
		}
	}

	// Detect PR provider from origin remote URL (best-effort; nil if not found).
	ctx0, cancel0 := context.WithTimeout(context.Background(), gitTimeout)
	remoteURL := g.OriginURL(ctx0)
	repoRoot, _ := g.RepoRoot(ctx0)
	cancel0()
	prov := pr.Detect(remoteURL)

	m := model{
		cfg:               cfg,
		version:           version,
		git:               g,
		repoRoot:          repoRoot,
		logFilterInput:    fi,
		branchFilterInput: branchFI,
		stashFilterInput:  stashFI,
		tagFilterInput:    tagFI,
		reflogFilterInput: reflogFI,
		issueFilterInput:  issueFI,
		usage:             usageData,
		usagePath:         usagePath,
		prProvider:        prov,
		mdb:               mdb,
		diffContext:       3,
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}
