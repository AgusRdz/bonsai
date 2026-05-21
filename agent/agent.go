package agent

import (
	"context"
	"os"
	"strings"

	"github.com/AgusRdz/bonsai/git"
)

// ---------------------------------------------------------------------------
// JSON output types
// ---------------------------------------------------------------------------

type FileEntry struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type StatusOut struct {
	Repo       string      `json:"repo"`
	Branch     string      `json:"branch"`
	Upstream   string      `json:"upstream,omitempty"`
	Ahead      int         `json:"ahead"`
	Behind     int         `json:"behind"`
	Staged     []FileEntry `json:"staged"`
	Unstaged   []FileEntry `json:"unstaged"`
	Conflicts  []FileEntry `json:"conflicts"`
	Untracked  []FileEntry `json:"untracked"`
	StashCount int         `json:"stash_count"`
	MergeState string      `json:"merge_state,omitempty"`
}

type LogEntry struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// LogParams controls what BuildLog returns.
type LogParams struct {
	Limit int    // 0 = 20
	Since string // e.g. "yesterday", "1 week ago", "2026-05-01"
	Until string // e.g. "2026-05-17"
}

type HunkOut struct {
	Header string   `json:"header"`
	Lines  []string `json:"lines"`
}

type FileDiff struct {
	Path      string    `json:"path"`
	Status    string    `json:"status,omitempty"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Hunks     []HunkOut `json:"hunks,omitempty"`
}

// UntrackedEntry is a path-only entry for a file not tracked by git.
type UntrackedEntry struct {
	Path string `json:"path"`
}

// DiffOut is the structured output for `bonsai diff`.
type DiffOut struct {
	Staged    []FileDiff       `json:"staged"`
	Unstaged  []FileDiff       `json:"unstaged"`
	Untracked []UntrackedEntry `json:"untracked"`
}

// ShowOut is the structured output for `bonsai show`.
type ShowOut struct {
	Hash         string     `json:"hash"`
	Subject      string     `json:"subject"`
	Author       string     `json:"author"`
	Date         string     `json:"date"`
	Additions    int        `json:"additions"`
	Deletions    int        `json:"deletions"`
	FilesChanged int        `json:"files_changed"`
	Diff         []FileDiff `json:"diff"`
}

type BlameEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type BranchEntry struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	Upstream string `json:"upstream,omitempty"`
	Date     string `json:"date,omitempty"`
	Ahead    int    `json:"ahead,omitempty"`
	Behind   int    `json:"behind,omitempty"`
	Gone     bool   `json:"gone,omitempty"`
}

type StashEntry struct {
	Ref         string `json:"ref"`
	Description string `json:"description"`
}

type ReviewLines struct {
	Added        int `json:"added"`
	Removed      int `json:"removed"`
	TotalChanged int `json:"total_changed"`
}

type ReviewOut struct {
	Base         string      `json:"base,omitempty"`
	Head         string      `json:"head"`
	Lines        ReviewLines `json:"lines"`
	FilesChanged int         `json:"files_changed"`
	CommitsCount int         `json:"commits_count,omitempty"`
	Commits      []LogEntry  `json:"commits,omitempty"`
	Diff         []FileDiff  `json:"diff"`
	Status       *StatusOut  `json:"status"`
}

// ContextOut is the structured output for `bonsai context`.
// It bundles status, working-tree diff, and recent commits into one snapshot.
type ContextOut struct {
	Status *StatusOut `json:"status"`
	Diff   *DiffOut   `json:"diff"`
	Log    []LogEntry `json:"log"`
}

// ---------------------------------------------------------------------------
// Builders
// ---------------------------------------------------------------------------

func BuildStatus(ctx context.Context, g *git.Runner) (*StatusOut, error) {
	s, err := g.Status(ctx)
	if err != nil {
		return nil, err
	}

	repo, _ := os.Getwd()

	out := &StatusOut{
		Repo:       repo,
		Branch:     s.Branch,
		Upstream:   s.Upstream,
		Ahead:      s.Ahead,
		Behind:     s.Behind,
		MergeState: s.MergeState,
		Staged:     []FileEntry{},
		Unstaged:   []FileEntry{},
		Conflicts:  []FileEntry{},
		Untracked:  []FileEntry{},
	}

	for _, f := range s.Staged {
		out.Staged = append(out.Staged, FileEntry{Status: string(f.StagedCode()), Path: f.Path})
	}
	for _, f := range s.Changed {
		out.Unstaged = append(out.Unstaged, FileEntry{Status: string(f.UnstagedCode()), Path: f.Path})
	}
	for _, f := range s.Conflicts {
		out.Conflicts = append(out.Conflicts, FileEntry{Status: f.Code, Path: f.Path})
	}
	for _, f := range s.Untracked {
		out.Untracked = append(out.Untracked, FileEntry{Status: "??", Path: f.Path})
	}

	stashes, err := g.StashList(ctx)
	if err == nil {
		out.StashCount = len(stashes)
	}

	return out, nil
}

func BuildLog(ctx context.Context, g *git.Runner, p LogParams) ([]LogEntry, error) {
	opts := git.LogStructuredOpts{Limit: p.Limit, Since: p.Since, Until: p.Until}
	entries, err := g.LogStructuredWithOpts(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, LogEntry{Hash: e.Hash, Subject: e.Subject, Author: e.Author, Date: e.Date})
	}
	return out, nil
}

// BuildDiff returns diff output grouped by scope (staged/unstaged/untracked).
// When no scope flag is true, all three scopes are included.
// When detailed is false, only file counts are returned (no patch hunks).
// path filters results to a single file; untracked is skipped when path is set.
// contextLines controls the -U<n> flag (0 = git default).
func BuildDiff(ctx context.Context, g *git.Runner, path string, showStaged, showUnstaged, showUntracked, detailed bool, contextLines int) (*DiffOut, error) {
	if !showStaged && !showUnstaged && !showUntracked {
		showStaged, showUnstaged, showUntracked = true, true, true
	}

	out := &DiffOut{
		Staged:    []FileDiff{},
		Unstaged:  []FileDiff{},
		Untracked: []UntrackedEntry{},
	}

	if showStaged {
		files, err := buildScopeDiff(ctx, g, path, true, detailed, contextLines)
		if err != nil {
			return nil, err
		}
		out.Staged = files
	}

	if showUnstaged {
		files, err := buildScopeDiff(ctx, g, path, false, detailed, contextLines)
		if err != nil {
			return nil, err
		}
		out.Unstaged = files
	}

	if showUntracked && path == "" {
		paths, err := g.ListUntracked(ctx)
		if err == nil {
			for _, p := range paths {
				out.Untracked = append(out.Untracked, UntrackedEntry{Path: p})
			}
		}
	}

	return out, nil
}

// buildScopeDiff returns FileDiff entries for staged or unstaged changes.
// contextLines controls the -U<n> flag (0 = git default).
func buildScopeDiff(ctx context.Context, g *git.Runner, path string, staged, detailed bool, contextLines int) ([]FileDiff, error) {
	if detailed {
		var (
			raw string
			err error
		)
		if path != "" {
			raw, err = g.Diff(ctx, path, staged, contextLines)
		} else {
			raw, err = g.DiffAll(ctx, staged, contextLines)
		}
		if err != nil {
			return []FileDiff{}, err
		}
		files := parseMultiFileDiff(raw)
		if files == nil {
			return []FileDiff{}, nil
		}
		ns, _ := g.DiffNameStatus(ctx, staged)
		applyStatuses(files, ns)
		return files, nil
	}

	numstats, err := g.DiffNumstat(ctx, staged)
	if err != nil {
		return []FileDiff{}, err
	}
	ns, _ := g.DiffNameStatus(ctx, staged)
	var files []FileDiff
	for _, n := range numstats {
		if path != "" && n.Path != path {
			continue
		}
		fd := FileDiff{Path: n.Path, Additions: n.Additions, Deletions: n.Deletions}
		if s, ok := ns[n.Path]; ok {
			fd.Status = s
		}
		files = append(files, fd)
	}
	if files == nil {
		return []FileDiff{}, nil
	}
	return files, nil
}

// BuildShow returns metadata and optionally diff hunks for a single commit.
// ref may be a hash, HEAD, or HEAD~N.
// contextLines controls the -U<n> flag (0 = git default).
func BuildShow(ctx context.Context, g *git.Runner, ref string, detailed bool, contextLines int) (*ShowOut, error) {
	entry, err := g.ShowCommit(ctx, ref)
	if err != nil {
		return nil, err
	}

	out := &ShowOut{
		Hash:    entry.Hash,
		Subject: entry.Subject,
		Author:  entry.Author,
		Date:    entry.Date,
		Diff:    []FileDiff{},
	}

	if detailed {
		raw, err := g.ShowDiff(ctx, ref, contextLines)
		if err != nil {
			return nil, err
		}
		files := parseMultiFileDiff(raw)
		if files != nil {
			ns, _ := g.ShowNameStatus(ctx, ref)
			applyStatuses(files, ns)
			out.Diff = files
		}
	} else {
		numstats, err := g.ShowNumstat(ctx, ref)
		if err != nil {
			return nil, err
		}
		ns, _ := g.ShowNameStatus(ctx, ref)
		for _, n := range numstats {
			fd := FileDiff{Path: n.Path, Additions: n.Additions, Deletions: n.Deletions}
			if s, ok := ns[n.Path]; ok {
				fd.Status = s
			}
			out.Diff = append(out.Diff, fd)
		}
		if out.Diff == nil {
			out.Diff = []FileDiff{}
		}
	}

	out.FilesChanged = len(out.Diff)
	for _, fd := range out.Diff {
		out.Additions += fd.Additions
		out.Deletions += fd.Deletions
	}

	return out, nil
}

func BuildBlame(ctx context.Context, g *git.Runner, path string, startLine, endLine int) ([]BlameEntry, error) {
	lines, err := g.Blame(ctx, path, startLine, endLine)
	if err != nil {
		return nil, err
	}
	out := make([]BlameEntry, 0, len(lines))
	for _, l := range lines {
		out = append(out, BlameEntry{Hash: l.Hash, Author: l.Author, Date: l.Date, Line: l.LineNum, Content: l.Text})
	}
	return out, nil
}

func BuildBranches(ctx context.Context, g *git.Runner) ([]BranchEntry, error) {
	branches, err := g.Branches(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BranchEntry, 0, len(branches))
	for _, b := range branches {
		out = append(out, BranchEntry{Name: b.Name, Current: b.Current, Upstream: b.Upstream, Date: b.Date, Ahead: b.Ahead, Behind: b.Behind, Gone: b.Gone})
	}
	return out, nil
}

func BuildStashList(ctx context.Context, g *git.Runner) ([]StashEntry, error) {
	entries, err := g.StashList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StashEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, StashEntry{Ref: e.Ref, Description: e.Description})
	}
	return out, nil
}

// BuildContext returns a single snapshot of repo status, working-tree diff,
// and recent commits. logLimit controls how many commits to include (0 = 10).
// When detailed is false, diff contains only file counts (no patch hunks).
// contextLines controls the -U<n> flag (0 = git default).
func BuildContext(ctx context.Context, g *git.Runner, logLimit int, detailed bool, contextLines int) (*ContextOut, error) {
	if logLimit <= 0 {
		logLimit = 10
	}

	status, err := BuildStatus(ctx, g)
	if err != nil {
		return nil, err
	}

	diff, err := BuildDiff(ctx, g, "", true, true, true, detailed, contextLines)
	if err != nil {
		return nil, err
	}

	log, err := BuildLog(ctx, g, LogParams{Limit: logLimit})
	if err != nil {
		return nil, err
	}

	return &ContextOut{Status: status, Diff: diff, Log: log}, nil
}

// BuildReview returns diff and commit context for code review.
// When detailed is false, only file counts are returned (no patch hunks).
// contextLines controls the -U<n> flag (0 = git default).
// BuildReview compares base..target (two-dot diff). target defaults to HEAD when empty.
// paths restricts the diff to a subset of files; always passed after a literal -- separator.
func BuildReview(ctx context.Context, g *git.Runner, base, target string, detailed bool, contextLines int, paths []string) (*ReviewOut, error) {
	if target == "" {
		target = "HEAD"
	}
	status, err := BuildStatus(ctx, g)
	if err != nil {
		return nil, err
	}

	out := &ReviewOut{
		Base:    base,
		Head:    status.Branch,
		Status:  status,
		Diff:    []FileDiff{},
		Commits: []LogEntry{},
	}

	if base != "" {
		if detailed {
			raw, err := g.DiffRange(ctx, base, target, contextLines, paths)
			if err != nil {
				return nil, err
			}
			out.Diff = parseMultiFileDiff(raw)
			if out.Diff == nil {
				out.Diff = []FileDiff{}
			}
			nameStatus, _ := g.DiffRangeNameStatus(ctx, base, target, paths)
			applyStatuses(out.Diff, nameStatus)
		} else {
			numstats, err := g.DiffRangeNumstat(ctx, base, target, paths)
			if err != nil {
				return nil, err
			}
			nameStatus, _ := g.DiffRangeNameStatus(ctx, base, target, paths)
			for _, n := range numstats {
				fd := FileDiff{Path: n.Path, Additions: n.Additions, Deletions: n.Deletions}
				if s, ok := nameStatus[n.Path]; ok {
					fd.Status = s
				}
				out.Diff = append(out.Diff, fd)
			}
			if out.Diff == nil {
				out.Diff = []FileDiff{}
			}
		}

		commits, _ := g.CommitsInRange(ctx, base, target)
		for _, c := range commits {
			out.Commits = append(out.Commits, LogEntry{Hash: c.Hash, Subject: c.Subject, Author: c.Author, Date: c.Date})
		}
		out.CommitsCount = len(out.Commits)
	} else {
		if detailed {
			raw, err := g.DiffAll(ctx, true, contextLines)
			if err != nil {
				return nil, err
			}
			out.Diff = parseMultiFileDiff(raw)
			if out.Diff == nil {
				out.Diff = []FileDiff{}
			}
			nameStatus, _ := g.DiffNameStatus(ctx, true)
			applyStatuses(out.Diff, nameStatus)
		} else {
			numstats, err := g.DiffNumstat(ctx, true)
			if err != nil {
				return nil, err
			}
			nameStatus, _ := g.DiffNameStatus(ctx, true)
			for _, n := range numstats {
				fd := FileDiff{Path: n.Path, Additions: n.Additions, Deletions: n.Deletions}
				if s, ok := nameStatus[n.Path]; ok {
					fd.Status = s
				}
				out.Diff = append(out.Diff, fd)
			}
			if out.Diff == nil {
				out.Diff = []FileDiff{}
			}
		}
	}

	out.FilesChanged = len(out.Diff)
	for _, fd := range out.Diff {
		out.Lines.Added += fd.Additions
		out.Lines.Removed += fd.Deletions
	}
	out.Lines.TotalChanged = out.Lines.Added + out.Lines.Removed

	return out, nil
}

// ---------------------------------------------------------------------------
// Diff parsing
// ---------------------------------------------------------------------------

func parseMultiFileDiff(raw string) []FileDiff {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var sections []string
	var current []string

	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "diff --git ") && len(current) > 0 {
			sections = append(sections, strings.Join(current, "\n"))
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	var files []FileDiff
	for _, section := range sections {
		fd := parseSingleFileDiff(section)
		if fd.Path != "" {
			files = append(files, fd)
		}
	}
	return files
}

func parseSingleFileDiff(raw string) FileDiff {
	var fd FileDiff
	if raw == "" {
		return fd
	}

	lines := strings.Split(raw, "\n")

	// Extract path: prefer "+++ b/<path>" for modified/new files.
	// For deletions "+++ /dev/null", fall back to "--- a/<path>".
	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			fd.Path = strings.TrimPrefix(line, "+++ b/")
			break
		}
		if line == "+++ /dev/null" {
			for _, l := range lines {
				if strings.HasPrefix(l, "--- a/") {
					fd.Path = strings.TrimPrefix(l, "--- a/")
				}
			}
			break
		}
	}
	// New file where --- is /dev/null.
	if fd.Path == "" {
		for _, line := range lines {
			if line == "--- /dev/null" {
				for _, l := range lines {
					if strings.HasPrefix(l, "+++ b/") {
						fd.Path = strings.TrimPrefix(l, "+++ b/")
					}
				}
				break
			}
		}
	}

	var curHunk *HunkOut
	inHunks := false

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			inHunks = true
			if curHunk != nil {
				fd.Hunks = append(fd.Hunks, *curHunk)
			}
			curHunk = &HunkOut{Header: line, Lines: []string{}}
			continue
		}
		if !inHunks || curHunk == nil {
			continue
		}
		curHunk.Lines = append(curHunk.Lines, line)
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			fd.Additions++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			fd.Deletions++
		}
	}
	if curHunk != nil {
		fd.Hunks = append(fd.Hunks, *curHunk)
	}
	if fd.Hunks == nil {
		fd.Hunks = []HunkOut{}
	}

	return fd
}

func applyStatuses(files []FileDiff, nameStatus map[string]string) {
	for i, fd := range files {
		if s, ok := nameStatus[fd.Path]; ok {
			files[i].Status = s
		}
	}
}
