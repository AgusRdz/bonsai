package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FileEntry represents one file in the working tree.
type FileEntry struct {
	Code string // two-char XY porcelain status code, e.g. "M ", " M", "??"
	Path string
}

// StagedCode returns the index (staged) status character.
func (f FileEntry) StagedCode() byte { return f.Code[0] }

// UnstagedCode returns the working-tree (unstaged) status character.
func (f FileEntry) UnstagedCode() byte { return f.Code[1] }

// Status holds the result of parsing `git status`.
type Status struct {
	Branch     string
	Staged     []FileEntry
	Changed    []FileEntry
	Untracked  []FileEntry
	Conflicts  []FileEntry // files with unresolved merge conflicts
	Ahead      int         // local commits not on remote tracking branch
	Behind     int         // remote commits not yet pulled
	MergeState string      // "merge", "cherry-pick", "rebase", or ""
}

// ConflictDesc returns a short human description for a conflict status code.
func ConflictDesc(code string) string {
	switch code {
	case "UU":
		return "both modified"
	case "AA":
		return "both added"
	case "DD":
		return "both deleted"
	case "AU":
		return "added by us"
	case "UA":
		return "added by them"
	case "DU":
		return "deleted by us"
	case "UD":
		return "deleted by them"
	}
	return "conflict"
}

// LogEntry is one line of `git log --oneline --graph` output.
type LogEntry struct {
	Line string
	Hash string // abbreviated commit hash; empty for pure graph lines (|, \, /)
}

// CommitDetail holds the parsed output of `git show --stat` for one commit.
type CommitDetail struct {
	Hash    string
	Author  string
	Date    string
	Subject string
	Body    string
	Stat    []string // file-stat lines including summary
}

// Branch represents a local git branch.
type Branch struct {
	Name    string
	Current bool
}

// StashEntry is one entry from `git stash list`.
type StashEntry struct {
	Ref         string // e.g. stash@{0}
	Description string // e.g. "On main: WIP on login flow"
}

// Runner wraps the git binary. All commands run in the current working directory.
// LastCmd is updated after every operation so the education panel can show the
// exact command that was executed.
type Runner struct {
	lastCmd string
}

// New returns a ready-to-use Runner.
func New() *Runner { return &Runner{} }

// LastCmd returns the most recently executed git invocation.
func (r *Runner) LastCmd() string { return r.lastCmd }

func (r *Runner) run(ctx context.Context, args ...string) ([]byte, error) {
	r.lastCmd = "git " + strings.Join(args, " ")
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

// Status returns the current branch and file status.
// A single `git status --porcelain --branch` call provides branch name,
// ahead/behind counts, and file status - replacing the previous 4-call approach.
func (r *Runner) Status(ctx context.Context) (*Status, error) {
	out, err := r.run(ctx, "status", "--porcelain", "--branch")
	if err != nil {
		return nil, err
	}
	r.lastCmd = "git status"

	raw := string(out)
	branch := "HEAD"
	var ahead, behind int
	body := raw

	if strings.HasPrefix(raw, "## ") {
		nl := strings.IndexByte(raw, '\n')
		var header string
		if nl < 0 {
			header, body = raw[3:], ""
		} else {
			header, body = raw[3:nl], raw[nl+1:]
		}
		branch, ahead, behind = parseBranchLine(header)
	}

	s := parseStatus(branch, body)
	s.Ahead = ahead
	s.Behind = behind
	s.MergeState = detectMergeState(ctx)
	return s, nil
}

// detectMergeState checks sentinel files that git leaves in .git/ to indicate
// an in-progress operation. Returns "merge", "cherry-pick", "rebase", or "".
func detectMergeState(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return ""
	}
	gitDir := strings.TrimSpace(string(out))
	sentinels := []struct {
		path  string
		label string
	}{
		{filepath.Join(gitDir, "MERGE_HEAD"), "merge"},
		{filepath.Join(gitDir, "CHERRY_PICK_HEAD"), "cherry-pick"},
		{filepath.Join(gitDir, "rebase-merge"), "rebase"},
		{filepath.Join(gitDir, "rebase-apply"), "rebase"},
	}
	for _, s := range sentinels {
		if _, err := os.Stat(s.path); err == nil {
			return s.label
		}
	}
	return ""
}

// parseBranchLine parses the header line from `git status --porcelain --branch`
// (the part after the leading "## ").
// Examples:
//
//	"main...origin/main [ahead 2, behind 1]"
//	"main...origin/main"
//	"main"
//	"No commits yet on main"
//	"HEAD (no branch)"
func parseBranchLine(line string) (branch string, ahead, behind int) {
	// Strip [ahead N, behind M] suffix first.
	if idx := strings.Index(line, " ["); idx >= 0 {
		bracket := line[idx+2:]
		if end := strings.Index(bracket, "]"); end >= 0 {
			bracket = bracket[:end]
		}
		for _, part := range strings.Split(bracket, ", ") {
			part = strings.TrimSpace(part)
			switch {
			case strings.HasPrefix(part, "ahead "):
				ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
			case strings.HasPrefix(part, "behind "):
				behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
			}
		}
		line = line[:idx]
	}

	switch {
	case strings.HasPrefix(line, "No commits yet on "):
		branch = strings.TrimPrefix(line, "No commits yet on ")
	case line == "HEAD (no branch)":
		branch = "HEAD"
	default:
		// "main...origin/main" or just "main"
		if idx := strings.Index(line, "..."); idx >= 0 {
			branch = line[:idx]
		} else {
			branch = line
		}
	}
	return
}

// Add stages the given paths.
func (r *Runner) Add(ctx context.Context, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	_, err := r.run(ctx, args...)
	return err
}

// Restore removes the given paths from the index (unstages), keeping working tree.
func (r *Runner) Restore(ctx context.Context, paths ...string) error {
	args := append([]string{"restore", "--staged", "--"}, paths...)
	_, err := r.run(ctx, args...)
	return err
}

// Discard removes working tree changes for the given paths.
// Unlike Restore, this does NOT touch the index - it discards uncommitted edits permanently.
func (r *Runner) Discard(ctx context.Context, paths ...string) error {
	args := append([]string{"restore", "--"}, paths...)
	_, err := r.run(ctx, args...)
	return err
}

// Commit creates a commit with the given message.
func (r *Runner) Commit(ctx context.Context, message string) error {
	_, err := r.run(ctx, "commit", "-m", message)
	return err
}

// LogOptions controls what git log returns.
type LogOptions struct {
	MaxCount int    // number of commits to fetch; 0 → 100
	Skip     int    // pagination: skip this many commits
	Grep     string // filter by commit message (case-insensitive)
	Author   string // filter by author name/email
	Since    string // show commits more recent than this date
	Until    string // show commits older than this date
}

// Log returns the n most recent commits as graph/oneline entries.
func (r *Runner) Log(ctx context.Context, n int) ([]LogEntry, error) {
	return r.LogOpts(ctx, LogOptions{MaxCount: n})
}

// LogOpts returns commits matching opts. When any filter (Grep/Author/Since/Until)
// is set the graph is omitted since it does not compose well with filtered output.
func (r *Runner) LogOpts(ctx context.Context, opts LogOptions) ([]LogEntry, error) {
	n := opts.MaxCount
	if n <= 0 {
		n = 100
	}
	args := []string{"log", fmt.Sprintf("--max-count=%d", n)}
	if opts.Skip > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", opts.Skip))
	}

	filtered := opts.Grep != "" || opts.Author != "" || opts.Since != "" || opts.Until != ""
	if opts.Grep != "" {
		args = append(args, "--grep="+opts.Grep, "--regexp-ignore-case")
	}
	if opts.Author != "" {
		args = append(args, "--author="+opts.Author, "--regexp-ignore-case")
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	if !filtered {
		args = append(args, "--graph")
	}
	args = append(args, "--oneline", "--decorate")

	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	entries := make([]LogEntry, 0, n)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			entries = append(entries, LogEntry{Line: line, Hash: extractCommitHash(line)})
		}
	}
	return entries, nil
}

// extractCommitHash returns the 7-char abbreviated hash from a graph/oneline
// log line. Graph prefix characters (| \ / * space _ -) are skipped; the
// first non-prefix token is checked for a valid hex string. Returns "" for
// pure graph lines that carry no commit (e.g. "|\", "| |").
func extractCommitHash(line string) string {
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '|' || c == '\\' || c == '/' || c == '*' || c == ' ' || c == '_' || c == '-' {
			continue
		}
		// First non-graph byte - next 7 bytes must be lowercase hex.
		if i+7 > len(line) {
			return ""
		}
		candidate := line[i : i+7]
		if isAllHex(candidate) {
			return candidate
		}
		return ""
	}
	return ""
}

func isAllHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// ShowStat returns structured commit detail for a single commit hash.
func (r *Runner) ShowStat(ctx context.Context, hash string) (*CommitDetail, error) {
	// \x1e (ASCII Record Separator) is safe to pass via exec args, unlike \x00.
	const sep = "\x1e"
	format := "%H" + sep + "%an <%ae>" + sep + "%ad" + sep + "%s" + sep + "%b"
	out, err := r.run(ctx, "show", "--no-color",
		"--format=format:"+format, "--stat", hash)
	if err != nil {
		return nil, err
	}

	raw := string(out)
	parts := strings.SplitN(raw, sep, 5)
	if len(parts) < 5 {
		return &CommitDetail{Hash: hash}, nil
	}

	// The 5th part contains body + stat block separated by the diff-stat header.
	bodyAndStat := parts[4]

	// git show puts a blank line between the body and the stat block.
	// The stat block starts after the last blank line that precedes file paths.
	detail := &CommitDetail{
		Hash:    strings.TrimSpace(parts[0]),
		Author:  strings.TrimSpace(parts[1]),
		Date:    strings.TrimSpace(parts[2]),
		Subject: strings.TrimSpace(parts[3]),
	}

	// Split body from stat: stat lines contain " | " or end with "changed".
	lines := strings.Split(strings.TrimRight(bodyAndStat, "\n"), "\n")
	var bodyLines, statLines []string
	inStat := false
	for _, l := range lines {
		if !inStat && (strings.Contains(l, " | ") || strings.Contains(l, "changed,") || strings.Contains(l, "changed\n") || (strings.Contains(l, "file") && strings.Contains(l, "changed"))) {
			inStat = true
		}
		if inStat {
			statLines = append(statLines, l)
		} else {
			bodyLines = append(bodyLines, l)
		}
	}
	detail.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	detail.Stat = statLines
	return detail, nil
}

// Branches returns all local branches.
func (r *Runner) Branches(ctx context.Context) ([]Branch, error) {
	out, err := r.run(ctx, "branch")
	if err != nil {
		return nil, err
	}
	return parseBranches(string(out)), nil
}

// Switch changes to an existing branch.
func (r *Runner) Switch(ctx context.Context, name string) error {
	_, err := r.run(ctx, "switch", name)
	return err
}

// CreateBranch creates and switches to a new branch.
func (r *Runner) CreateBranch(ctx context.Context, name string) error {
	_, err := r.run(ctx, "switch", "-c", name)
	return err
}

// Push pushes the current branch to its upstream.
func (r *Runner) Push(ctx context.Context) error {
	_, err := r.run(ctx, "push")
	return err
}

// Pull fetches and merges the upstream into the current branch.
func (r *Runner) Pull(ctx context.Context) error {
	_, err := r.run(ctx, "pull")
	return err
}

// Diff returns the unified diff for a single file.
// When staged is true it diffs the index against HEAD (what will be committed).
// When staged is false it diffs the working tree against the index (unstaged changes).
func (r *Runner) Diff(ctx context.Context, path string, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	args = append(args, "--", path)
	out, err := r.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Rename renames the current branch.
func (r *Runner) Rename(ctx context.Context, newName string) error {
	_, err := r.run(ctx, "branch", "-m", newName)
	return err
}

// Stash saves the current working tree changes to the stash.
func (r *Runner) Stash(ctx context.Context) error {
	_, err := r.run(ctx, "stash", "push")
	return err
}

// StashPop applies the given stash ref and removes it from the stash list.
func (r *Runner) StashPop(ctx context.Context, ref string) error {
	_, err := r.run(ctx, "stash", "pop", ref)
	return err
}

// StashList returns all stash entries in order.
func (r *Runner) StashList(ctx context.Context) ([]StashEntry, error) {
	out, err := r.run(ctx, "stash", "list")
	if err != nil {
		return nil, err
	}
	return parseStashList(string(out)), nil
}

func parseStashList(output string) []StashEntry {
	var entries []StashEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ": ")
		if idx < 0 {
			continue
		}
		entries = append(entries, StashEntry{
			Ref:         line[:idx],
			Description: line[idx+2:],
		})
	}
	return entries
}

// AcceptOurs resolves a conflict by keeping our version (current branch HEAD)
// and staging the result. Works for UU, AU, UD conflicts.
func (r *Runner) AcceptOurs(ctx context.Context, path string) error {
	if _, err := r.run(ctx, "checkout", "--ours", "--", path); err != nil {
		return err
	}
	_, err := r.run(ctx, "add", "--", path)
	return err
}

// AcceptTheirs resolves a conflict by keeping the incoming version
// and staging the result. Works for UU, UA, DU conflicts.
func (r *Runner) AcceptTheirs(ctx context.Context, path string) error {
	if _, err := r.run(ctx, "checkout", "--theirs", "--", path); err != nil {
		return err
	}
	_, err := r.run(ctx, "add", "--", path)
	return err
}

// RemoveConflict resolves a DD (both deleted) conflict by removing the file
// from the index.
func (r *Runner) RemoveConflict(ctx context.Context, path string) error {
	_, err := r.run(ctx, "rm", "--", path)
	return err
}

// ConflictLines reads a conflicted file from the working tree and returns its
// lines. The caller is responsible for rendering the conflict markers.
func (r *Runner) ConflictLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	return lines, nil
}

// TagEntry is one tag returned by git tag.
type TagEntry struct {
	Name string
}

// Tags returns all local tags sorted newest first.
func (r *Runner) Tags(ctx context.Context) ([]TagEntry, error) {
	out, err := r.run(ctx, "tag", "--sort=-creatordate")
	if err != nil {
		return nil, err
	}
	var tags []TagEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tags = append(tags, TagEntry{Name: line})
		}
	}
	return tags, nil
}

// CreateTag creates a lightweight tag at HEAD.
func (r *Runner) CreateTag(ctx context.Context, name string) error {
	_, err := r.run(ctx, "tag", name)
	return err
}

// DeleteTag deletes a local tag by name.
func (r *Runner) DeleteTag(ctx context.Context, name string) error {
	_, err := r.run(ctx, "tag", "-d", name)
	return err
}

// Merge merges branch into the current branch.
func (r *Runner) Merge(ctx context.Context, branch string) error {
	_, err := r.run(ctx, "merge", branch)
	return err
}

// CherryPick applies the commit hash onto the current branch.
func (r *Runner) CherryPick(ctx context.Context, hash string) error {
	_, err := r.run(ctx, "cherry-pick", hash)
	return err
}

// Reset undoes the last commit. mode must be "soft", "mixed", or "hard".
func (r *Runner) Reset(ctx context.Context, mode string) error {
	_, err := r.run(ctx, "reset", "--"+mode, "HEAD~1")
	return err
}

// Rebase rebases the current branch onto the named branch.
func (r *Runner) Rebase(ctx context.Context, branch string) error {
	_, err := r.run(ctx, "rebase", branch)
	return err
}

// RebaseContinue continues an in-progress rebase after conflicts are resolved.
func (r *Runner) RebaseContinue(ctx context.Context) error {
	_, err := r.run(ctx, "-c", "core.editor=true", "rebase", "--continue")
	return err
}

// RebaseAbort aborts the current rebase and restores the original branch state.
func (r *Runner) RebaseAbort(ctx context.Context) error {
	_, err := r.run(ctx, "rebase", "--abort")
	return err
}

// MergeAbort aborts an in-progress merge.
func (r *Runner) MergeAbort(ctx context.Context) error {
	_, err := r.run(ctx, "merge", "--abort")
	return err
}

// CherryPickAbort aborts an in-progress cherry-pick.
func (r *Runner) CherryPickAbort(ctx context.Context) error {
	_, err := r.run(ctx, "cherry-pick", "--abort")
	return err
}

// parseStatus converts `git status --porcelain` output into a Status.
// Files with both staged and unstaged changes appear in both slices.
// conflictCodes are the two-character porcelain codes that indicate an
// unresolved merge conflict. These must be routed to Status.Conflicts and
// never treated as ordinary staged/changed entries.
var conflictCodes = map[string]bool{
	"UU": true, "AA": true, "DD": true,
	"AU": true, "UA": true,
	"DU": true, "UD": true,
}

func parseStatus(branch, porcelain string) *Status {
	s := &Status{Branch: branch}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		x, y := code[0], code[1]

		if code == "??" {
			s.Untracked = append(s.Untracked, FileEntry{Code: code, Path: path})
			continue
		}
		if conflictCodes[code] {
			s.Conflicts = append(s.Conflicts, FileEntry{Code: code, Path: path})
			continue
		}
		if x != ' ' && x != '?' {
			s.Staged = append(s.Staged, FileEntry{Code: code, Path: path})
		}
		if y != ' ' && y != '?' {
			s.Changed = append(s.Changed, FileEntry{Code: code, Path: path})
		}
	}
	return s
}

// parseBranches converts `git branch` output into a slice of Branch.
func parseBranches(output string) []Branch {
	var branches []Branch
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		current := strings.HasPrefix(line, "* ")
		name := strings.TrimPrefix(strings.TrimPrefix(line, "* "), "  ")
		branches = append(branches, Branch{Name: name, Current: current})
	}
	return branches
}
