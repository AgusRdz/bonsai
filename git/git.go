package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
	Upstream   string // remote tracking ref, e.g. "origin/main"; empty if unset
	Staged     []FileEntry
	Changed    []FileEntry
	Untracked  []FileEntry
	Conflicts  []FileEntry // files with unresolved merge conflicts
	Ahead      int         // local commits not on remote tracking branch
	Behind     int         // remote commits not yet pulled
	MergeState string      // "merge", "cherry-pick", "rebase", or ""
}

// StructuredLogEntry is one commit returned by LogStructured and CommitsInRange.
type StructuredLogEntry struct {
	Hash    string
	Subject string
	Author  string
	Date    string // YYYY-MM-DD
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

// LogEntry is one line of `git log` output.
type LogEntry struct {
	Line string
	Hash string // abbreviated commit hash; empty for pure graph lines (|, \, /)
	// Sig is the GPG/SSH signature status from %G?: G=good, B=bad, U=untrusted,
	// N=no sig, X=expired, E=missing key. Empty for pure graph connector lines.
	Sig string
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
	Name     string
	Current  bool
	Upstream string // remote tracking ref, e.g. "origin/feat/login"; empty if not set
	Date     string // last commit date, e.g. "2026-05-18"
	Ahead    int    // commits ahead of upstream
	Behind   int    // commits behind upstream
	Gone     bool   // upstream ref was deleted
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

	var upstream string
	if strings.HasPrefix(raw, "## ") {
		nl := strings.IndexByte(raw, '\n')
		var header string
		if nl < 0 {
			header, body = raw[3:], ""
		} else {
			header, body = raw[3:nl], raw[nl+1:]
		}
		branch, upstream, ahead, behind = parseBranchLine(header)
	}

	s := parseStatus(branch, body)
	s.Upstream = upstream
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
func parseBranchLine(line string) (branch, upstream string, ahead, behind int) {
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
			upstream = line[idx+3:]
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

// StageAll stages all changes in the working tree (git add .).
func (r *Runner) StageAll(ctx context.Context) error {
	_, err := r.run(ctx, "add", ".")
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

// RmCached removes a path from the git index (stops tracking it) without
// touching the file on disk. Equivalent to git rm --cached <path>.
func (r *Runner) RmCached(ctx context.Context, path string) error {
	_, err := r.run(ctx, "rm", "--cached", "--", path)
	return err
}

// Commit creates a commit with the given message.
func (r *Runner) Commit(ctx context.Context, message string) error {
	_, err := r.run(ctx, "commit", "-m", message)
	return err
}

// CommitSigned commits with GPG/SSH signing. key may be empty to use the git
// default signing key (commit.gpgsign / user.signingkey).
func (r *Runner) CommitSigned(ctx context.Context, message, key string) error {
	args := []string{"commit", "-m", message}
	if key != "" {
		args = append(args, "--gpg-sign="+key)
	} else {
		args = append(args, "-S")
	}
	_, err := r.run(ctx, args...)
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
	// %G?%x1f prefixes each commit line with the signature status and a unit
	// separator so we can strip it without affecting graph connector lines.
	args = append(args, "--format=%G?%x1f%h%d %s")

	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	entries := make([]LogEntry, 0, n)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if idx := strings.IndexByte(line, '\x1f'); idx >= 0 {
			sig := ""
			if idx >= 1 {
				sig = string(line[idx-1])
			}
			display := line[idx+1:]
			entries = append(entries, LogEntry{Line: display, Hash: extractCommitHash(display), Sig: sig})
		} else {
			entries = append(entries, LogEntry{Line: line})
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
	out, err := r.run(ctx, "branch", "--format=%(refname:short)\t%(HEAD)\t%(upstream:short)\t%(authordate:short)\t%(upstream:track)")
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

// PushWithOptions pushes with optional --force-with-lease or --set-upstream.
func (r *Runner) PushWithOptions(ctx context.Context, force, setUpstream bool, remote, branch string) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	if setUpstream && remote != "" && branch != "" {
		args = append(args, "--set-upstream", remote, branch)
	}
	_, err := r.run(ctx, args...)
	return err
}

// Hunk is a single contiguous block of diff output.
type Hunk struct {
	Header string   // the "@@ -x,y +a,b @@" line
	Body   []string // context and changed lines
}

func (h Hunk) raw() string {
	raw := h.Header + "\n"
	if len(h.Body) > 0 {
		raw += strings.Join(h.Body, "\n") + "\n"
	}
	return raw
}

// DiffHunks returns the file-level patch header and individual hunks for one
// file. staged=true diffs the index against HEAD; staged=false diffs the
// working tree against the index.
func (r *Runner) DiffHunks(ctx context.Context, path string, staged bool) (fileHeader string, hunks []Hunk, err error) {
	args := []string{"diff", "--unified=3"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)

	out, runErr := r.run(ctx, args...)
	if runErr != nil {
		return "", nil, runErr
	}
	if len(out) == 0 {
		return "", nil, nil
	}
	return parseDiffOutput(string(out))
}

// parseDiffOutput splits raw unified diff output into a file header and
// individual hunks. Each hunk starts at a "@@" line.
func parseDiffOutput(raw string) (fileHeader string, hunks []Hunk, err error) {
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var headerLines []string
	var cur *Hunk
	inHeader := true

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			inHeader = false
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			cur = &Hunk{Header: line}
			continue
		}
		if inHeader {
			headerLines = append(headerLines, line)
		} else if cur != nil {
			cur.Body = append(cur.Body, line)
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}

	fileHeader = strings.Join(headerLines, "\n")
	return fileHeader, hunks, nil
}

// ApplyHunks stages (reverse=false) or unstages (reverse=true) the given
// hunks by piping a constructed patch to git apply --cached.
func (r *Runner) ApplyHunks(ctx context.Context, fileHeader string, hunks []Hunk, reverse bool) error {
	if len(hunks) == 0 {
		return nil
	}

	var patch strings.Builder
	patch.WriteString(fileHeader)
	if !strings.HasSuffix(fileHeader, "\n") {
		patch.WriteByte('\n')
	}
	for _, h := range hunks {
		patch.WriteString(h.raw())
	}

	args := []string{"apply", "--cached"}
	if reverse {
		args = append(args, "--reverse")
	}
	r.lastCmd = "git " + strings.Join(args, " ")
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = strings.NewReader(patch.String())
	outB, applyErr := cmd.CombinedOutput()
	if applyErr != nil {
		return fmt.Errorf("%w: %s", applyErr, strings.TrimSpace(string(outB)))
	}
	return nil
}

// ApplyLines stages a subset of lines from a single hunk.
// lineSel is parallel to hunk.Body: context lines are always included;
// '-' lines are kept as removals if selected, converted to context if not;
// '+' lines are included if selected, omitted entirely if not.
func (r *Runner) ApplyLines(ctx context.Context, fileHeader string, hunk Hunk, lineSel []bool, reverse bool) error {
	partial := buildPartialHunk(hunk, lineSel)
	if partial == nil {
		return nil
	}
	var patch strings.Builder
	patch.WriteString(fileHeader)
	if !strings.HasSuffix(fileHeader, "\n") {
		patch.WriteByte('\n')
	}
	patch.WriteString(partial.raw())

	args := []string{"apply", "--cached"}
	if reverse {
		args = append(args, "--reverse")
	}
	r.lastCmd = "git " + strings.Join(args, " ")
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = strings.NewReader(patch.String())
	outB, applyErr := cmd.CombinedOutput()
	if applyErr != nil {
		return fmt.Errorf("%w: %s", applyErr, strings.TrimSpace(string(outB)))
	}
	return nil
}

// buildPartialHunk returns a new Hunk containing only the selected change lines.
// Unselected '-' lines become context; unselected '+' lines are dropped.
// Returns nil if no change lines are selected.
func buildPartialHunk(h Hunk, lineSel []bool) *Hunk {
	var newBody []string
	var C, D, d, a int

	for i, line := range h.Body {
		sel := i >= len(lineSel) || lineSel[i]
		switch {
		case strings.HasPrefix(line, "-"):
			D++
			if sel {
				d++
				newBody = append(newBody, line)
			} else {
				newBody = append(newBody, " "+line[1:])
			}
		case strings.HasPrefix(line, "+"):
			if sel {
				a++
				newBody = append(newBody, line)
			}
		default:
			C++
			newBody = append(newBody, line)
		}
	}

	if d == 0 && a == 0 {
		return nil
	}

	oldStart, newStart := parseHunkStarts(h.Header)
	oldCount := C + D
	newCount := C + D - d + a

	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)
	if idx := strings.Index(h.Header, " @@ "); idx >= 0 {
		header += h.Header[idx+3:]
	}

	return &Hunk{Header: header, Body: newBody}
}

func parseHunkStarts(header string) (oldStart, newStart int) {
	var os, oc, ns, nc int
	_, _ = fmt.Sscanf(header, "@@ -%d,%d +%d,%d", &os, &oc, &ns, &nc)
	return os, ns
}

// Pull fetches and fast-forwards the current branch.
func (r *Runner) Pull(ctx context.Context) error {
	_, err := r.run(ctx, "pull")
	return err
}

// PullRebase fetches and rebases local commits on top of the upstream.
func (r *Runner) PullRebase(ctx context.Context) error {
	_, err := r.run(ctx, "pull", "--rebase")
	return err
}

// PullMerge fetches and creates a merge commit when branches have diverged.
func (r *Runner) PullMerge(ctx context.Context) error {
	_, err := r.run(ctx, "pull", "--no-ff")
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

// StashWithMessage stashes with an optional descriptive message.
// If msg is empty it falls back to a plain stash.
func (r *Runner) StashWithMessage(ctx context.Context, msg string) error {
	if msg == "" {
		return r.Stash(ctx)
	}
	_, err := r.run(ctx, "stash", "push", "-m", msg)
	return err
}

// FileLog returns the commit log entries that touched the given file.
func (r *Runner) FileLog(ctx context.Context, path string, limit int) ([]LogEntry, error) {
	args := []string{"log", fmt.Sprintf("--max-count=%d", limit), "--format=%G?%x1f%h%d %s", "--", path}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		if idx := strings.IndexByte(line, '\x1f'); idx >= 0 {
			sig := ""
			if idx >= 1 {
				sig = string(line[idx-1])
			}
			display := line[idx+1:]
			entries = append(entries, LogEntry{Line: display, Hash: extractCommitHash(display), Sig: sig})
		} else {
			entries = append(entries, LogEntry{Line: line})
		}
	}
	return entries, nil
}

// Graph returns a text representation of the full commit graph across all branches.
func (r *Runner) Graph(ctx context.Context, limit int) (string, error) {
	out, err := r.run(ctx, "log", "--graph", "--oneline", "--decorate", "--all",
		fmt.Sprintf("--max-count=%d", limit), "--color=never")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DiffCommit returns the unified diff introduced by a single commit.
// Works for the initial commit (no parent) by using git show.
func (r *Runner) DiffCommit(ctx context.Context, hash string) (string, error) {
	out, err := r.run(ctx, "show", "--format=", "--", hash)
	if err != nil {
		return "", err
	}
	// git show prefixes output with a blank line from the empty format; strip it.
	result := strings.TrimPrefix(string(out), "\n")
	return result, nil
}

// Clone clones a remote repository to an optional local directory.
func (r *Runner) Clone(ctx context.Context, url, dir string) error {
	args := []string{"clone", url}
	if dir != "" {
		args = append(args, dir)
	}
	_, err := r.run(ctx, args...)
	return err
}

// StashPop applies the given stash ref and removes it from the stash list.
func (r *Runner) StashPop(ctx context.Context, ref string) error {
	_, err := r.run(ctx, "stash", "pop", ref)
	return err
}

// StashApply applies the given stash ref without removing it from the list.
func (r *Runner) StashApply(ctx context.Context, ref string) error {
	_, err := r.run(ctx, "stash", "apply", ref)
	return err
}

// StashDrop removes the given stash ref without applying it.
func (r *Runner) StashDrop(ctx context.Context, ref string) error {
	_, err := r.run(ctx, "stash", "drop", ref)
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

// ConflictVersions returns the base (stage 1), ours (stage 2), and theirs
// (stage 3) versions of a conflicted file from git's index. Any stage that
// does not exist (e.g. file-add conflicts) returns nil.
func (r *Runner) ConflictVersions(ctx context.Context, path string) (base, ours, theirs []string) {
	getStage := func(n int) []string {
		out, err := r.run(ctx, "show", fmt.Sprintf(":%d:%s", n, path))
		if err != nil || len(out) == 0 {
			return nil
		}
		return strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	}
	return getStage(1), getStage(2), getStage(3)
}

// WorktreeEntry is one entry from `git worktree list`.
type WorktreeEntry struct {
	Path    string // absolute path to the worktree
	Branch  string // checked-out branch name, or "(detached)" for detached HEAD
	Current bool   // true if this is the main worktree (the one we're in)
}

// Worktrees returns all worktrees including the main one.
func (r *Runner) Worktrees(ctx context.Context) ([]WorktreeEntry, error) {
	out, err := r.run(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(string(out)), nil
}

// parseWorktrees parses the porcelain output of `git worktree list --porcelain`.
// Blocks are separated by blank lines; the first block is the main worktree.
func parseWorktrees(output string) []WorktreeEntry {
	var entries []WorktreeEntry
	blocks := strings.Split(strings.TrimRight(output, "\n"), "\n\n")
	for idx, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var entry WorktreeEntry
		entry.Current = idx == 0
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				entry.Path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "branch "):
				entry.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			case line == "detached":
				entry.Branch = "(detached)"
			}
		}
		if entry.Path != "" {
			entries = append(entries, entry)
		}
	}
	return entries
}

// AddWorktree creates a new worktree at path checked out to branch.
// If branch is empty, creates a new branch named after the last path component.
func (r *Runner) AddWorktree(ctx context.Context, path, branch string) error {
	var err error
	if branch == "" {
		base := filepath.Base(path)
		_, err = r.run(ctx, "worktree", "add", "-b", base, path)
	} else {
		_, err = r.run(ctx, "worktree", "add", path, branch)
	}
	return err
}

// RemoveWorktree removes a worktree by path. Uses --force to handle unclean state.
func (r *Runner) RemoveWorktree(ctx context.Context, path string) error {
	_, err := r.run(ctx, "worktree", "remove", "--force", path)
	return err
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

// PushTag pushes a local tag to the given remote.
func (r *Runner) PushTag(ctx context.Context, remote, tag string) error {
	_, err := r.run(ctx, "push", remote, tag)
	return err
}

// DeleteBranch deletes a local branch. Use force=true for unmerged branches.
func (r *Runner) DeleteBranch(ctx context.Context, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run(ctx, "branch", flag, name)
	return err
}

// RenameBranch renames a branch (any branch, not just the current one).
func (r *Runner) RenameBranch(ctx context.Context, oldName, newName string) error {
	_, err := r.run(ctx, "branch", "-m", oldName, newName)
	return err
}

// DeleteRemoteBranch deletes a branch on the given remote.
func (r *Runner) DeleteRemoteBranch(ctx context.Context, remote, branch string) error {
	_, err := r.run(ctx, "push", remote, "--delete", branch)
	return err
}

// BlameLine is one line of git blame output.
type BlameLine struct {
	Hash    string // abbreviated 8-char hash
	Author  string // author name (not email)
	Date    string // short date YYYY-MM-DD
	LineNum int    // 1-based line number
	Text    string // the actual line content
}

// Blame returns the blame output for a file.
func (r *Runner) Blame(ctx context.Context, path string) ([]BlameLine, error) {
	out, err := r.run(ctx, "blame", "--porcelain", path)
	if err != nil {
		// untracked or empty file
		return nil, nil
	}
	raw := strings.TrimRight(string(out), "\n")
	if raw == "" {
		return nil, nil
	}
	return parseBlamePorcelain(raw), nil
}

func parseBlamePorcelain(raw string) []BlameLine {
	var lines []BlameLine
	var cur BlameLine
	inGroup := false

	for _, line := range strings.Split(raw, "\n") {
		if !inGroup {
			// Header: <40-char-hash> <orig-line> <final-line> [<num-lines>]
			parts := strings.Fields(line)
			if len(parts) >= 3 && len(parts[0]) == 40 && isAllHex(parts[0]) {
				cur = BlameLine{}
				cur.Hash = parts[0][:8]
				if n, e := strconv.Atoi(parts[2]); e == nil {
					cur.LineNum = n
				}
				inGroup = true
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "author "):
			cur.Author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-time "):
			ts, e := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64)
			if e == nil {
				cur.Date = time.Unix(ts, 0).UTC().Format("2006-01-02")
			}
		case strings.HasPrefix(line, "\t"):
			cur.Text = line[1:]
			lines = append(lines, cur)
			inGroup = false
		}
	}
	return lines
}

// BisectState holds the current state of a bisect session.
type BisectState struct {
	Active  bool   // true when .git/BISECT_LOG exists
	Status  string // last output line from git bisect log
	Current string // current commit hash being tested (abbreviated)
}

// BisectStatus returns the current bisect state by reading sentinel files.
func (r *Runner) BisectStatus(ctx context.Context) (*BisectState, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return &BisectState{}, nil
	}
	gitDir := strings.TrimSpace(string(out))
	logPath := filepath.Join(gitDir, "BISECT_LOG")

	state := &BisectState{}
	if _, err := os.Stat(logPath); err != nil {
		// No bisect in progress.
		return state, nil
	}
	state.Active = true

	// Get the last non-empty line from bisect log for status.
	logOut, err := exec.CommandContext(ctx, "git", "bisect", "log").Output()
	if err == nil {
		lines := strings.Split(strings.TrimRight(string(logOut), "\n"), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) != "" {
				state.Status = strings.TrimSpace(lines[i])
				break
			}
		}
	}

	// Get the current short hash.
	hashOut, err := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		state.Current = strings.TrimSpace(string(hashOut))
	}

	return state, nil
}

// BisectStart starts a bisect session.
func (r *Runner) BisectStart(ctx context.Context) error {
	_, err := r.run(ctx, "bisect", "start")
	return err
}

// BisectBad marks the current commit as bad.
func (r *Runner) BisectBad(ctx context.Context) error {
	_, err := r.run(ctx, "bisect", "bad")
	return err
}

// BisectGood marks a specific commit as good. hash may be empty to use HEAD.
func (r *Runner) BisectGood(ctx context.Context, hash string) error {
	if hash == "" {
		_, err := r.run(ctx, "bisect", "good")
		return err
	}
	_, err := r.run(ctx, "bisect", "good", hash)
	return err
}

// BisectReset ends the bisect session and returns to the original branch.
func (r *Runner) BisectReset(ctx context.Context) error {
	_, err := r.run(ctx, "bisect", "reset")
	return err
}

// BisectLog returns the full bisect log output for display.
func (r *Runner) BisectLog(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "bisect", "log")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// RebaseInteractiveCommits returns the commits that would appear in the
// interactive rebase todo for the given base ref (e.g. "HEAD~3").
// Returns commits in rebase order: oldest first (bottom to top in log).
func (r *Runner) RebaseInteractiveCommits(ctx context.Context, base string) ([]string, error) {
	out, err := r.run(ctx, "log", "--oneline", "--no-decorate", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	lines := strings.Split(raw, "\n")
	// git log gives newest-first; rebase todo is oldest-first, so reverse.
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	return lines, nil
}

// RebaseInteractive executes an interactive rebase using the provided todo lines.
// Each todo line is like "pick abc1234 commit message" or "drop abc1234 commit message".
// base is the ref passed to git rebase -i (e.g. "HEAD~3").
func (r *Runner) RebaseInteractive(ctx context.Context, base string, todoLines []string) error {
	tmpFile, err := os.CreateTemp("", "bonsai-rebase-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(strings.Join(todoLines, "\n") + "\n"); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	cmd := exec.CommandContext(ctx, "git", "rebase", "-i", base)
	cmd.Env = append(os.Environ(), "GIT_SEQUENCE_EDITOR=cp "+tmpFile.Name())
	out, err := cmd.CombinedOutput()
	r.lastCmd = "git rebase -i " + base
	if err != nil {
		if len(out) > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}

// AmendMessage rewrites the last commit with a new message.
func (r *Runner) AmendMessage(ctx context.Context, msg string) error {
	_, err := r.run(ctx, "commit", "--amend", "-m", msg)
	return err
}

// AmendAuthor rewrites the last commit with a new author string.
// author must be in "Name <email>" format.
func (r *Runner) AmendAuthor(ctx context.Context, author string) error {
	_, err := r.run(ctx, "commit", "--amend", "--author="+author, "--no-edit")
	return err
}

// AmendDate rewrites the last commit with a new date.
// date is passed directly to git (accepts ISO 8601 or "now").
func (r *Runner) AmendDate(ctx context.Context, date string) error {
	_, err := r.run(ctx, "commit", "--amend", "--date="+date, "--no-edit")
	return err
}

// AmendNoEdit adds currently staged files to the last commit without
// changing its message.
func (r *Runner) AmendNoEdit(ctx context.Context) error {
	_, err := r.run(ctx, "commit", "--amend", "--no-edit")
	return err
}

// Merge merges branch into the current branch.
func (r *Runner) Merge(ctx context.Context, branch string) error {
	_, err := r.run(ctx, "merge", branch)
	return err
}

// MergeNoFF merges branch into HEAD with --no-ff (always creates a merge commit).
func (r *Runner) MergeNoFF(ctx context.Context, branch string) error {
	_, err := r.run(ctx, "merge", "--no-ff", branch)
	return err
}

// FinishBranch executes a multi-step gitflow finish for the given feature/bugfix
// branch: switches to targetBranch, does a no-ff merge, then deletes branch.
func (r *Runner) FinishBranch(ctx context.Context, branch, targetBranch string) error {
	if err := r.Switch(ctx, targetBranch); err != nil {
		return fmt.Errorf("switch to %s: %w", targetBranch, err)
	}
	if err := r.MergeNoFF(ctx, branch); err != nil {
		return fmt.Errorf("merge %s: %w", branch, err)
	}
	if err := r.DeleteBranch(ctx, branch, false); err != nil {
		return fmt.Errorf("delete %s: %w", branch, err)
	}
	return nil
}

// FinishRelease executes a multi-step gitflow release finish:
// merge to mainBranch, create tag, merge to devBranch, delete branch.
func (r *Runner) FinishRelease(ctx context.Context, branch, mainBranch, devBranch, tagName string) error {
	if err := r.Switch(ctx, mainBranch); err != nil {
		return fmt.Errorf("switch to %s: %w", mainBranch, err)
	}
	if err := r.MergeNoFF(ctx, branch); err != nil {
		return fmt.Errorf("merge %s to %s: %w", branch, mainBranch, err)
	}
	if tagName != "" {
		if err := r.CreateTag(ctx, tagName); err != nil {
			return fmt.Errorf("create tag %s: %w", tagName, err)
		}
	}
	if err := r.Switch(ctx, devBranch); err != nil {
		return fmt.Errorf("switch to %s: %w", devBranch, err)
	}
	if err := r.MergeNoFF(ctx, branch); err != nil {
		return fmt.Errorf("merge %s to %s: %w", branch, devBranch, err)
	}
	if err := r.DeleteBranch(ctx, branch, false); err != nil {
		return fmt.Errorf("delete %s: %w", branch, err)
	}
	return nil
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

// ResetOrig resets the branch to ORIG_HEAD - the state before the last merge,
// rebase, or amend. Returns an error if ORIG_HEAD does not exist.
func (r *Runner) ResetOrig(ctx context.Context) error {
	_, err := r.run(ctx, "reset", "--hard", "ORIG_HEAD")
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

// ConfigEntry is one key=value pair from git config --list.
type ConfigEntry struct {
	Key   string
	Value string
}

// GlobalConfigList returns all entries from the global git config.
func (r *Runner) GlobalConfigList(ctx context.Context) ([]ConfigEntry, error) {
	out, err := r.run(ctx, "config", "--global", "--list")
	if err != nil {
		return nil, err
	}
	return parseConfigList(string(out)), nil
}

// LocalConfigList returns all entries from the local repo git config.
// On error (not in a repo) returns nil, nil.
func (r *Runner) LocalConfigList(ctx context.Context) ([]ConfigEntry, error) {
	out, err := r.run(ctx, "config", "--local", "--list")
	if err != nil {
		return nil, nil
	}
	return parseConfigList(string(out)), nil
}

func parseConfigList(output string) []ConfigEntry {
	var entries []ConfigEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			entries = append(entries, ConfigEntry{Key: line, Value: ""})
			continue
		}
		entries = append(entries, ConfigEntry{Key: line[:idx], Value: line[idx+1:]})
	}
	return entries
}

// GlobalConfigGet returns the value of a single global config key, or "" if unset.
func (r *Runner) GlobalConfigGet(ctx context.Context, key string) (string, error) {
	out, err := r.run(ctx, "config", "--global", "--get", key)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetGlobalConfig sets or updates a global git config key.
func (r *Runner) SetGlobalConfig(ctx context.Context, key, value string) error {
	_, err := r.run(ctx, "config", "--global", key, value)
	return err
}

// GlobalGitignorePath returns the path to the global gitignore file.
// Checks core.excludesfile; falls back to ~/.config/git/ignore.
func (r *Runner) GlobalGitignorePath(ctx context.Context) (string, error) {
	p, err := r.GlobalConfigGet(ctx, "core.excludesfile")
	if err != nil {
		return "", err
	}
	if p == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "git", "ignore"), nil
	}
	// Expand leading ~
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	return p, nil
}

// GlobalConfigRawPath returns the absolute path to ~/.gitconfig.
func (r *Runner) GlobalConfigRawPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gitconfig"), nil
}

// LocalConfigRawPath returns the path to .git/config in the current repo.
func (r *Runner) LocalConfigRawPath() string {
	return ".git/config"
}

// parseBranches converts `git branch` output into a slice of Branch.
func parseBranches(output string) []Branch {
	var branches []Branch
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		current := len(parts) > 1 && strings.TrimSpace(parts[1]) == "*"
		upstream := ""
		if len(parts) > 2 {
			upstream = strings.TrimSpace(parts[2])
		}
		date := ""
		if len(parts) > 3 {
			date = strings.TrimSpace(parts[3])
		}
		ahead, behind, gone := 0, 0, false
		if len(parts) > 4 {
			ahead, behind, gone = parseTrack(parts[4])
		}
		branches = append(branches, Branch{
			Name: name, Current: current, Upstream: upstream,
			Date: date, Ahead: ahead, Behind: behind, Gone: gone,
		})
	}
	return branches
}

// parseTrack parses the %(upstream:track) token, e.g. "[ahead 2, behind 1]".
func parseTrack(s string) (ahead, behind int, gone bool) {
	s = strings.Trim(strings.TrimSpace(s), "[]")
	if s == "" {
		return
	}
	if s == "gone" {
		gone = true
		return
	}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "ahead "):
			ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
		case strings.HasPrefix(part, "behind "):
			behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
		}
	}
	return
}

// MergedBranches returns names of local branches fully merged into target.
func (r *Runner) MergedBranches(ctx context.Context, target string) ([]string, error) {
	out, err := r.run(ctx, "branch", "--merged", target, "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// ---------------------------------------------------------------------------
// Fetch
// ---------------------------------------------------------------------------

// Fetch runs git fetch. When all is true fetches all remotes; when prune is
// true adds --prune to remove deleted remote refs.
func (r *Runner) Fetch(ctx context.Context, all, prune bool) error {
	args := []string{"fetch"}
	if all {
		args = append(args, "--all")
	}
	if prune {
		args = append(args, "--prune")
	}
	_, err := r.run(ctx, args...)
	return err
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

// RestoreFile restores a file to a specific ref. When source is "" it defaults
// to HEAD. When staged is true it also unstages the result.
func (r *Runner) RestoreFile(ctx context.Context, path, source string, staged bool) error {
	args := []string{"restore"}
	if staged {
		args = append(args, "--staged")
	}
	src := source
	if src == "" {
		src = "HEAD"
	}
	args = append(args, "--source="+src, path)
	_, err := r.run(ctx, args...)
	return err
}

// ---------------------------------------------------------------------------
// Clean
// ---------------------------------------------------------------------------

// CleanPreview returns the list of files that would be removed by git clean
// without actually removing them.
func (r *Runner) CleanPreview(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "clean", "-fd", "--dry-run")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// lines look like "Would remove path/to/file"
		f := strings.TrimPrefix(line, "Would remove ")
		files = append(files, f)
	}
	return files, nil
}

// Clean removes untracked files and directories (git clean -fd).
func (r *Runner) Clean(ctx context.Context) error {
	_, err := r.run(ctx, "clean", "-fd")
	return err
}

// ---------------------------------------------------------------------------
// Reflog
// ---------------------------------------------------------------------------

// ReflogEntry is one line from git reflog.
type ReflogEntry struct {
	Hash    string // short hash
	Ref     string // e.g. HEAD@{0}
	Action  string // e.g. "commit", "checkout", "reset"
	Subject string // short description
}

// Reflog returns the recent reflog entries for HEAD.
func (r *Runner) Reflog(ctx context.Context) ([]ReflogEntry, error) {
	out, err := r.run(ctx, "reflog", "--format=%h\x1e%gd\x1e%gs", "-100")
	if err != nil {
		return nil, err
	}
	return parseReflogOutput(string(out)), nil
}

// parseReflogOutput converts the \x1e-delimited reflog output into entries.
func parseReflogOutput(output string) []ReflogEntry {
	var entries []ReflogEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1e", 3)
		if len(parts) != 3 {
			continue
		}
		action := parts[2]
		fields := strings.Fields(action)
		if len(fields) == 0 {
			continue
		}
		actionType := strings.TrimSuffix(fields[0], ":")
		entries = append(entries, ReflogEntry{
			Hash:    parts[0],
			Ref:     parts[1],
			Action:  actionType,
			Subject: action,
		})
	}
	return entries
}

// ---------------------------------------------------------------------------
// Remote management
// ---------------------------------------------------------------------------

// RemoteEntry is one configured remote.
type RemoteEntry struct {
	Name     string
	FetchURL string
	PushURL  string
}

// Remotes returns all configured remotes with their fetch/push URLs.
func (r *Runner) Remotes(ctx context.Context) ([]RemoteEntry, error) {
	out, err := r.run(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemoteList(string(out)), nil
}

// parseRemoteList converts `git remote -v` output into RemoteEntry values.
// Each remote appears twice (fetch and push); the function deduplicates them.
func parseRemoteList(output string) []RemoteEntry {
	seen := map[string]*RemoteEntry{}
	var order []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Format: "name\turl (fetch)" or "name\turl (push)"
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		url := parts[1]
		kind := strings.Trim(parts[2], "()")
		e, ok := seen[name]
		if !ok {
			e = &RemoteEntry{Name: name}
			seen[name] = e
			order = append(order, name)
		}
		switch kind {
		case "fetch":
			e.FetchURL = url
		case "push":
			e.PushURL = url
		}
	}
	var result []RemoteEntry
	for _, name := range order {
		result = append(result, *seen[name])
	}
	return result
}

// InitRepo runs git init in the current directory.
func (r *Runner) InitRepo(ctx context.Context) error {
	_, err := r.run(ctx, "init")
	return err
}

// RemoteAdd adds a new remote.
func (r *Runner) RemoteAdd(ctx context.Context, name, url string) error {
	_, err := r.run(ctx, "remote", "add", name, url)
	return err
}

// OriginURL returns the fetch URL for the "origin" remote, or empty string if
// there is no origin or the command fails (e.g. not in a repo).
func (r *Runner) OriginURL(ctx context.Context) string {
	out, err := r.run(ctx, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RemoteRemove removes a remote.
func (r *Runner) RemoteRemove(ctx context.Context, name string) error {
	_, err := r.run(ctx, "remote", "remove", name)
	return err
}

// RemoteRename renames a remote.
func (r *Runner) RemoteRename(ctx context.Context, oldName, newName string) error {
	_, err := r.run(ctx, "remote", "rename", oldName, newName)
	return err
}

// ---------------------------------------------------------------------------
// Submodules
// ---------------------------------------------------------------------------

// SubmoduleEntry is one submodule in the repo.
type SubmoduleEntry struct {
	Path   string
	URL    string
	Status string // "+", "-", "U", " " (modified/uninit/conflict/clean)
	Hash   string // current checked-out hash
}

// Submodules returns all submodules with their status.
func (r *Runner) Submodules(ctx context.Context) ([]SubmoduleEntry, error) {
	out, err := r.run(ctx, "submodule", "status")
	if err != nil {
		return nil, err
	}
	var entries []SubmoduleEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Format: "[+- U]<hash> <path> (<describe>)"
		status := string(line[0])
		rest := strings.TrimSpace(line[1:])
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		hash := fields[0]
		path := fields[1]
		// Get URL from gitmodules config.
		urlOut, _ := exec.CommandContext(ctx, "git", "config", "--file=.gitmodules",
			"submodule."+path+".url").Output()
		entries = append(entries, SubmoduleEntry{
			Path:   path,
			URL:    strings.TrimSpace(string(urlOut)),
			Status: status,
			Hash:   hash,
		})
	}
	return entries, nil
}

// SubmoduleAdd registers and clones a new submodule.
func (r *Runner) SubmoduleAdd(ctx context.Context, url, path string) error {
	args := []string{"submodule", "add", url}
	if path != "" {
		args = append(args, path)
	}
	_, err := r.run(ctx, args...)
	return err
}

// SubmoduleInit initialises all submodules (registers them in .git/config).
func (r *Runner) SubmoduleInit(ctx context.Context) error {
	_, err := r.run(ctx, "submodule", "init")
	return err
}

// SubmoduleUpdate updates all submodules to the commit recorded by the parent.
func (r *Runner) SubmoduleUpdate(ctx context.Context, init bool) error {
	args := []string{"submodule", "update"}
	if init {
		args = append(args, "--init")
	}
	_, err := r.run(ctx, args...)
	return err
}

// SubmoduleDeinit deregisters a submodule (removes its config from .git/config).
func (r *Runner) SubmoduleDeinit(ctx context.Context, path string) error {
	_, err := r.run(ctx, "submodule", "deinit", path)
	return err
}

// SubmoduleSync updates the remote URL for all submodules from .gitmodules.
func (r *Runner) SubmoduleSync(ctx context.Context) error {
	_, err := r.run(ctx, "submodule", "sync")
	return err
}

// ---------------------------------------------------------------------------
// Notes
// ---------------------------------------------------------------------------

// NoteGet returns the note attached to a commit, or "" if none exists.
func (r *Runner) NoteGet(ctx context.Context, commit string) (string, error) {
	out, err := r.run(ctx, "notes", "show", commit)
	if err != nil {
		// exit 1 means no note - not an error.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// NoteAdd adds or replaces the note for a commit.
func (r *Runner) NoteAdd(ctx context.Context, commit, message string) error {
	_, err := r.run(ctx, "notes", "add", "-f", "-m", message, commit)
	return err
}

// NoteRemove removes the note for a commit. Returns nil if no note existed.
func (r *Runner) NoteRemove(ctx context.Context, commit string) error {
	_, err := r.run(ctx, "notes", "remove", commit)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// Format-patch / am
// ---------------------------------------------------------------------------

// FormatPatch generates .patch files for commits since base into outputDir.
// Returns the list of generated file paths.
func (r *Runner) FormatPatch(ctx context.Context, base, outputDir string) ([]string, error) {
	args := []string{"format-patch", base}
	if outputDir != "" {
		args = append(args, "-o", outputDir)
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// ApplyPatch applies one or more patch files with git am.
func (r *Runner) ApplyPatch(ctx context.Context, files ...string) error {
	args := append([]string{"am"}, files...)
	_, err := r.run(ctx, args...)
	return err
}

// ApplyPatchAbort aborts an in-progress git am.
func (r *Runner) ApplyPatchAbort(ctx context.Context) error {
	_, err := r.run(ctx, "am", "--abort")
	return err
}

// ---------------------------------------------------------------------------
// Archive
// ---------------------------------------------------------------------------

// Archive creates a tar or zip archive of the repo at the given ref.
// format should be "tar.gz" or "zip". output is the destination file path.
func (r *Runner) Archive(ctx context.Context, format, output, ref string) error {
	if ref == "" {
		ref = "HEAD"
	}
	r.lastCmd = fmt.Sprintf("git archive --format=%s --output=%s %s", format, output, ref)
	cmd := exec.CommandContext(ctx, "git", "archive", "--format="+format, "--output="+output, ref)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Bundle
// ---------------------------------------------------------------------------

// BundleCreate creates a git bundle file containing the given refs.
// If refs is empty, bundles all branches.
func (r *Runner) BundleCreate(ctx context.Context, output string, refs ...string) error {
	args := append([]string{"bundle", "create", output}, refs...)
	if len(refs) == 0 {
		args = append(args, "--all")
	}
	_, err := r.run(ctx, args...)
	return err
}

// BundleVerify verifies a bundle file and returns a human-readable summary.
func (r *Runner) BundleVerify(ctx context.Context, file string) (string, error) {
	out, err := r.run(ctx, "bundle", "verify", file)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// RepoStats holds aggregated repository statistics.
type RepoStats struct {
	TotalCommits    int
	Contributors    []ContributorStat
	FirstCommitDate string
	LastCommitDate  string
	TotalBranches   int
	TotalTags       int
	TrackedFiles    int
	ExtBreakdown    []ExtStat  // top file extensions by count
	TopFiles        []FileStat // most-changed files
	CommitsLast30d  int
}

// ContributorStat holds commit count per author, grouped by email.
type ContributorStat struct {
	Name  string
	Email string
	Count int
}

// ExtStat holds file count per extension.
type ExtStat struct {
	Ext   string
	Count int
}

// FileStat holds change frequency for a file.
type FileStat struct {
	Path  string
	Count int
}

// collectContributors groups commits by author email, resolving the display
// name by picking the most frequent name for each email. When the email
// matches the global user.email the configured user.name is preferred.
func collectContributors(ctx context.Context, r *Runner) []ContributorStat {
	out, err := exec.CommandContext(ctx, "git", "log", "--no-merges",
		"--format=%ae\x1e%an").Output()
	if err != nil {
		return nil
	}

	emailCounts := map[string]int{}
	emailNames := map[string]map[string]int{}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1e", 2)
		if len(parts) != 2 {
			continue
		}
		email := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		if email == "" {
			continue
		}
		emailCounts[email]++
		if emailNames[email] == nil {
			emailNames[email] = map[string]int{}
		}
		emailNames[email][name]++
	}

	// Resolve the canonical name for the global user.
	configEmail, _ := exec.CommandContext(ctx, "git", "config", "--global", "--get", "user.email").Output()
	configName, _ := exec.CommandContext(ctx, "git", "config", "--global", "--get", "user.name").Output()
	myEmail := strings.TrimSpace(string(configEmail))
	myName := strings.TrimSpace(string(configName))

	// Build sorted contributor list.
	type entry struct {
		email string
		count int
	}
	var entries []entry
	for email, count := range emailCounts {
		entries = append(entries, entry{email, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	var result []ContributorStat
	for _, e := range entries {
		name := mostFrequentName(emailNames[e.email])
		// Prefer the configured user.name when this is the global user's email.
		if myEmail != "" && e.email == myEmail && myName != "" {
			name = myName
		}
		result = append(result, ContributorStat{Name: name, Email: e.email, Count: e.count})
		if len(result) >= 10 {
			break
		}
	}
	return result
}

func mostFrequentName(names map[string]int) string {
	best := ""
	bestCount := 0
	for name, count := range names {
		if count > bestCount || (count == bestCount && name < best) {
			best = name
			bestCount = count
		}
	}
	return best
}

// ConfigGet returns the value of a git config key for the current repo/user.
func (r *Runner) ConfigGet(ctx context.Context, key string) (string, error) {
	out, err := r.run(ctx, "config", "--get", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StandupEntry is one commit returned by StandupLog.
type StandupEntry struct {
	Hash    string
	Subject string
	Date    string // YYYY-MM-DD
	Author  string
}

// StandupLog returns commits by author since N days ago, excluding merges.
// author may be a name substring (case-insensitive); pass "" to skip filter.
func (r *Runner) StandupLog(ctx context.Context, author string, days int) ([]StandupEntry, error) {
	since := fmt.Sprintf("%d.days.ago", days)
	// Use git's %x1f (unit separator) escape so the argument contains no
	// special bytes - git expands %xNN in the output, not in the argument.
	args := []string{
		"log", "--no-merges",
		"--pretty=tformat:%h%x1f%s%x1f%as%x1f%an",
		"--since=" + since,
	}
	if author != "" {
		args = append(args, "--author="+author, "--regexp-ignore-case")
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []StandupEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, StandupEntry{
			Hash:    parts[0],
			Subject: parts[1],
			Date:    parts[2],
			Author:  parts[3],
		})
	}
	return entries, nil
}

// LFSStatus returns the output of git lfs status. Returns an error when git-lfs
// is not installed.
func (r *Runner) LFSStatus(ctx context.Context) (string, error) {
	out, err := r.run(ctx, "lfs", "status")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// LFSTrack adds pattern to .gitattributes so matching files are stored via LFS.
func (r *Runner) LFSTrack(ctx context.Context, pattern string) error {
	_, err := r.run(ctx, "lfs", "track", pattern)
	return err
}

// LFSUntrack removes a pattern from .gitattributes LFS tracking.
func (r *Runner) LFSUntrack(ctx context.Context, pattern string) error {
	_, err := r.run(ctx, "lfs", "untrack", pattern)
	return err
}

// LFSPull downloads all LFS objects for the current checkout.
func (r *Runner) LFSPull(ctx context.Context) error {
	_, err := r.run(ctx, "lfs", "pull")
	return err
}

// LFSInstall installs git-lfs hooks into the current repository.
func (r *Runner) LFSInstall(ctx context.Context) error {
	_, err := r.run(ctx, "lfs", "install")
	return err
}

// LFSTrackedFiles returns the list of files currently tracked by LFS in the
// working tree. Returns an empty slice when git-lfs is not installed.
func (r *Runner) LFSTrackedFiles(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "lfs", "ls-files", "--name-only")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// LFSPush uploads all LFS objects to the remote.
func (r *Runner) LFSPush(ctx context.Context) error {
	_, err := r.run(ctx, "lfs", "push", "--all", "origin")
	return err
}

// LFSTrackedPatterns returns the patterns configured for LFS in .gitattributes.
// Unlike LFSTrackedFiles, this returns patterns (e.g. "*.psd") not file paths.
func (r *Runner) LFSTrackedPatterns(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "lfs", "track")
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		// Output format: "    *.psd (git)"
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		// Strip trailing " (git)" or " (.gitattributes)"
		if idx := strings.Index(line, " ("); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line != "" {
			patterns = append(patterns, line)
		}
	}
	return patterns, nil
}

// IsLFSInstalled reports whether git-lfs is available in the current environment.
func IsLFSInstalled() bool {
	_, err := exec.LookPath("git-lfs")
	return err == nil
}

// Stats computes repository statistics. May be slow on large repos.
func (r *Runner) Stats(ctx context.Context) (*RepoStats, error) {
	s := &RepoStats{}

	// Total commits.
	if out, err := r.run(ctx, "rev-list", "--count", "HEAD"); err == nil {
		s.TotalCommits, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	// First and last commit dates.
	if out, err := r.run(ctx, "log", "--format=%ad", "--date=short", "--reverse", "--max-count=1"); err == nil {
		s.FirstCommitDate = strings.TrimSpace(string(out))
	}
	if out, err := r.run(ctx, "log", "--format=%ad", "--date=short", "--max-count=1"); err == nil {
		s.LastCommitDate = strings.TrimSpace(string(out))
	}

	// Top contributors - grouped by email to collapse duplicate identities.
	s.Contributors = collectContributors(ctx, r)

	// Branch count.
	if out, err := r.run(ctx, "branch", "--list"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				s.TotalBranches++
			}
		}
	}

	// Tag count.
	if out, err := r.run(ctx, "tag"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if strings.TrimSpace(line) != "" {
				s.TotalTags++
			}
		}
	}

	// Tracked files and extension breakdown.
	if out, err := r.run(ctx, "ls-files"); err == nil {
		extCount := map[string]int{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			s.TrackedFiles++
			ext := filepath.Ext(line)
			if ext == "" {
				ext = "(no ext)"
			}
			extCount[ext]++
		}
		// Sort by count descending.
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range extCount {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].v > sorted[j].v
		})
		for _, kv := range sorted {
			s.ExtBreakdown = append(s.ExtBreakdown, ExtStat{Ext: kv.k, Count: kv.v})
			if len(s.ExtBreakdown) >= 10 {
				break
			}
		}
	}

	// Most-changed files (top 10). Capped at 2000 commits to stay within timeouts.
	if out, err := r.run(ctx, "log", "--format=", "--name-only", "--no-merges", "--max-count=2000"); err == nil {
		fileCount := map[string]int{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fileCount[line]++
			}
		}
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range fileCount {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].v > sorted[j].v
		})
		for _, kv := range sorted {
			s.TopFiles = append(s.TopFiles, FileStat{Path: kv.k, Count: kv.v})
			if len(s.TopFiles) >= 10 {
				break
			}
		}
	}

	// Commits in last 30 days.
	since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	if out, err := r.run(ctx, "rev-list", "--count", "--after="+since, "HEAD"); err == nil {
		s.CommitsLast30d, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	return s, nil
}

// DashboardEntry holds the quick status of one repository for the multi-repo
// dashboard. It is produced without changing the process working directory.
type DashboardEntry struct {
	Path       string
	Name       string // base name of the path
	Branch     string
	Ahead      int
	Behind     int
	Dirty      bool   // true when there are uncommitted changes
	LastCommit string // short subject of the HEAD commit
	Error      string // non-empty when the repo could not be read
}

// QuickStatus runs git -C path commands to populate a DashboardEntry.
func QuickStatus(ctx context.Context, repoPath string) DashboardEntry {
	run := func(args ...string) (string, error) {
		all := append([]string{"-C", repoPath}, args...)
		out, err := exec.CommandContext(ctx, "git", all...).Output()
		return strings.TrimSpace(string(out)), err
	}

	entry := DashboardEntry{
		Path: repoPath,
		Name: filepath.Base(repoPath),
	}

	branch, err := run("symbolic-ref", "--short", "HEAD")
	if err != nil {
		entry.Error = "not a git repo"
		return entry
	}
	entry.Branch = branch

	if ahead, err := run("rev-list", "--count", "@{u}..HEAD"); err == nil {
		entry.Ahead, _ = strconv.Atoi(ahead)
	}
	if behind, err := run("rev-list", "--count", "HEAD..@{u}"); err == nil {
		entry.Behind, _ = strconv.Atoi(behind)
	}
	if out, err := run("status", "--porcelain"); err == nil {
		entry.Dirty = strings.TrimSpace(out) != ""
	}
	if subject, err := run("log", "-1", "--format=%s"); err == nil {
		entry.LastCommit = subject
	}

	return entry
}

// ---------------------------------------------------------------------------
// Structured output — agent / machine-readable methods
// ---------------------------------------------------------------------------

// LogStructured returns the n most recent commits as structured entries
// with hash, subject, author, and date.
func (r *Runner) LogStructured(ctx context.Context, n int) ([]StructuredLogEntry, error) {
	if n <= 0 {
		n = 20
	}
	args := []string{
		"log",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=tformat:%h\x1f%s\x1f%an\x1f%as",
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []StructuredLogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, StructuredLogEntry{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return entries, nil
}

// CommitsInRange returns structured commits reachable from HEAD but not from base.
func (r *Runner) CommitsInRange(ctx context.Context, base string) ([]StructuredLogEntry, error) {
	args := []string{
		"log",
		"--pretty=tformat:%h\x1f%s\x1f%an\x1f%as",
		base + "..HEAD",
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []StructuredLogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, StructuredLogEntry{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return entries, nil
}

// DiffAll returns the full unified diff for all staged (staged=true) or
// unstaged (staged=false) changes across the working tree.
func (r *Runner) DiffAll(ctx context.Context, staged bool) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DiffRange returns the unified diff of all changes introduced between base
// and HEAD (equivalent to git diff base..HEAD).
func (r *Runner) DiffRange(ctx context.Context, base string) (string, error) {
	out, err := r.run(ctx, "diff", base+"..HEAD")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DiffNameStatus returns a map of path -> single-character status code for
// staged (staged=true) or unstaged (staged=false) changed files.
func (r *Runner) DiffNameStatus(ctx context.Context, staged bool) (map[string]string, error) {
	args := []string{"diff", "--name-status"}
	if staged {
		args = append(args, "--staged")
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseDiffNameStatus(string(out)), nil
}

// DiffRangeNameStatus returns a map of path -> single-character status code
// for all files changed between base and HEAD.
func (r *Runner) DiffRangeNameStatus(ctx context.Context, base string) (map[string]string, error) {
	out, err := r.run(ctx, "diff", "--name-status", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	return parseDiffNameStatus(string(out)), nil
}

// parseDiffNameStatus converts `git diff --name-status` output into a
// path -> status map. For renames and copies the destination path is used.
func parseDiffNameStatus(output string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		status := string(parts[0][0])
		path := parts[1]
		if len(parts) == 3 && (parts[0][0] == 'R' || parts[0][0] == 'C') {
			path = parts[2]
		}
		result[path] = status
	}
	return result
}

// FileNumstat is one entry from git diff --numstat or git show --numstat.
type FileNumstat struct {
	Additions int
	Deletions int
	Path      string
}

// DiffNumstat returns per-file line counts without patch content.
// staged=true diffs the index against HEAD; staged=false diffs the working tree against the index.
func (r *Runner) DiffNumstat(ctx context.Context, staged bool) ([]FileNumstat, error) {
	args := []string{"diff", "--numstat"}
	if staged {
		args = append(args, "--staged")
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseDiffNumstat(string(out)), nil
}

// DiffRangeNumstat returns per-file line counts for base..HEAD.
func (r *Runner) DiffRangeNumstat(ctx context.Context, base string) ([]FileNumstat, error) {
	out, err := r.run(ctx, "diff", "--numstat", base+"..HEAD")
	if err != nil {
		return nil, err
	}
	return parseDiffNumstat(string(out)), nil
}

// ListUntracked returns paths of untracked, non-ignored files.
func (r *Runner) ListUntracked(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// ShowCommit returns structured metadata for a single commit ref (hash, HEAD, HEAD~N, etc.).
func (r *Runner) ShowCommit(ctx context.Context, ref string) (*StructuredLogEntry, error) {
	out, err := r.run(ctx, "show", "--no-patch", "--pretty=tformat:%h\x1f%s\x1f%an\x1f%as", ref)
	if err != nil {
		return nil, err
	}
	line := strings.TrimSpace(string(out))
	parts := strings.SplitN(line, "\x1f", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected show output for %s", ref)
	}
	return &StructuredLogEntry{Hash: parts[0], Subject: parts[1], Author: parts[2], Date: parts[3]}, nil
}

// ShowNumstat returns per-file line counts for a single commit.
func (r *Runner) ShowNumstat(ctx context.Context, ref string) ([]FileNumstat, error) {
	out, err := r.run(ctx, "show", "--numstat", "--format=", ref)
	if err != nil {
		return nil, err
	}
	return parseDiffNumstat(strings.TrimPrefix(string(out), "\n")), nil
}

// ShowDiff returns the full unified diff for a single commit.
func (r *Runner) ShowDiff(ctx context.Context, ref string) (string, error) {
	out, err := r.run(ctx, "show", "--format=", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(out), "\n"), nil
}

// ShowNameStatus returns a path -> status map for files changed in a single commit.
func (r *Runner) ShowNameStatus(ctx context.Context, ref string) (map[string]string, error) {
	out, err := r.run(ctx, "show", "--name-status", "--format=", ref)
	if err != nil {
		return nil, err
	}
	return parseDiffNameStatus(strings.TrimPrefix(string(out), "\n")), nil
}

// LogStructuredOpts filters for LogStructuredWithOpts.
type LogStructuredOpts struct {
	Limit int    // 0 = 20
	Since string // e.g. "yesterday", "1 week ago", "2026-05-01"
	Until string // e.g. "2026-05-17"
}

// LogStructuredWithOpts returns structured log entries filtered by opts.
func (r *Runner) LogStructuredWithOpts(ctx context.Context, opts LogStructuredOpts) ([]StructuredLogEntry, error) {
	n := opts.Limit
	if n <= 0 {
		n = 20
	}
	args := []string{
		"log",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=tformat:%h\x1f%s\x1f%an\x1f%as",
	}
	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var entries []StructuredLogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, StructuredLogEntry{
			Hash:    parts[0],
			Subject: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return entries, nil
}

func parseDiffNumstat(output string) []FileNumstat {
	var files []FileNumstat
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		files = append(files, FileNumstat{Additions: add, Deletions: del, Path: parts[2]})
	}
	return files
}
