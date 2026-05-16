package git

import (
	"context"
	"fmt"
	"os/exec"
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
	Branch    string
	Staged    []FileEntry
	Changed   []FileEntry
	Untracked []FileEntry
	Ahead     int // local commits not on remote tracking branch
	Behind    int // remote commits not yet pulled
}

// LogEntry is one line of `git log --oneline --graph` output.
type LogEntry struct {
	Line string
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
	return s, nil
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

// Log returns the n most recent commits as graph/oneline entries.
func (r *Runner) Log(ctx context.Context, n int) ([]LogEntry, error) {
	out, err := r.run(ctx, "log", fmt.Sprintf("--max-count=%d", n),
		"--oneline", "--graph", "--decorate")
	if err != nil {
		return nil, err
	}
	entries := make([]LogEntry, 0, n)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			entries = append(entries, LogEntry{Line: line})
		}
	}
	return entries, nil
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

// parseStatus converts `git status --porcelain` output into a Status.
// Files with both staged and unstaged changes appear in both slices.
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
