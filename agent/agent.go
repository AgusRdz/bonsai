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

type HunkOut struct {
	Header string   `json:"header"`
	Lines  []string `json:"lines"`
}

type FileDiff struct {
	Path      string    `json:"path"`
	Status    string    `json:"status,omitempty"`
	Additions int       `json:"additions"`
	Deletions int       `json:"deletions"`
	Hunks     []HunkOut `json:"hunks"`
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
}

type StashEntry struct {
	Ref         string `json:"ref"`
	Description string `json:"description"`
}

type ContextOut struct {
	Repo      string        `json:"repo"`
	Branch    string        `json:"branch"`
	Upstream  string        `json:"upstream,omitempty"`
	Status    *StatusOut    `json:"status"`
	RecentLog []LogEntry    `json:"recent_log"`
	Branches  []BranchEntry `json:"branches"`
}

type ReviewLines struct {
	Added        int `json:"added"`
	Removed      int `json:"removed"`
	TotalChanged int `json:"total_changed"`
}

type ReviewContextOut struct {
	Base         string      `json:"base,omitempty"`
	Head         string      `json:"head"`
	Lines        ReviewLines `json:"lines"`
	FilesChanged int         `json:"files_changed"`
	CommitsCount int         `json:"commits_count,omitempty"`
	Commits      []LogEntry  `json:"commits,omitempty"`
	Diff         []FileDiff  `json:"diff"`
	Status       *StatusOut  `json:"status"`
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

func BuildLog(ctx context.Context, g *git.Runner, limit int) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	entries, err := g.LogStructured(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, LogEntry{Hash: e.Hash, Subject: e.Subject, Author: e.Author, Date: e.Date})
	}
	return out, nil
}

func BuildDiff(ctx context.Context, g *git.Runner, path string, staged bool) ([]FileDiff, error) {
	if path != "" {
		raw, err := g.Diff(ctx, path, staged)
		if err != nil {
			return nil, err
		}
		fd := parseSingleFileDiff(raw)
		if fd.Path == "" {
			fd.Path = path
		}
		return []FileDiff{fd}, nil
	}
	raw, err := g.DiffAll(ctx, staged)
	if err != nil {
		return nil, err
	}
	files := parseMultiFileDiff(raw)
	if files == nil {
		files = []FileDiff{}
	}
	return files, nil
}

func BuildBlame(ctx context.Context, g *git.Runner, path string) ([]BlameEntry, error) {
	lines, err := g.Blame(ctx, path)
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
		out = append(out, BranchEntry{Name: b.Name, Current: b.Current, Upstream: b.Upstream})
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

func BuildContext(ctx context.Context, g *git.Runner) (*ContextOut, error) {
	repo, _ := os.Getwd()

	status, err := BuildStatus(ctx, g)
	if err != nil {
		return nil, err
	}

	log, _ := BuildLog(ctx, g, 20)
	if log == nil {
		log = []LogEntry{}
	}

	branches, _ := BuildBranches(ctx, g)
	if branches == nil {
		branches = []BranchEntry{}
	}

	return &ContextOut{
		Repo:      repo,
		Branch:    status.Branch,
		Upstream:  status.Upstream,
		Status:    status,
		RecentLog: log,
		Branches:  branches,
	}, nil
}

func BuildReviewContext(ctx context.Context, g *git.Runner, base string) (*ReviewContextOut, error) {
	status, err := BuildStatus(ctx, g)
	if err != nil {
		return nil, err
	}

	out := &ReviewContextOut{
		Base:    base,
		Head:    status.Branch,
		Status:  status,
		Diff:    []FileDiff{},
		Commits: []LogEntry{},
	}

	if base != "" {
		raw, err := g.DiffRange(ctx, base)
		if err != nil {
			return nil, err
		}
		out.Diff = parseMultiFileDiff(raw)
		if out.Diff == nil {
			out.Diff = []FileDiff{}
		}

		nameStatus, _ := g.DiffRangeNameStatus(ctx, base)
		applyStatuses(out.Diff, nameStatus)

		commits, _ := g.CommitsInRange(ctx, base)
		for _, c := range commits {
			out.Commits = append(out.Commits, LogEntry{Hash: c.Hash, Subject: c.Subject, Author: c.Author, Date: c.Date})
		}
		out.CommitsCount = len(out.Commits)
	} else {
		raw, err := g.DiffAll(ctx, true)
		if err != nil {
			return nil, err
		}
		out.Diff = parseMultiFileDiff(raw)
		if out.Diff == nil {
			out.Diff = []FileDiff{}
		}

		nameStatus, _ := g.DiffNameStatus(ctx, true)
		applyStatuses(out.Diff, nameStatus)
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
