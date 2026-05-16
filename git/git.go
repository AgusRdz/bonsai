package git

import (
	"context"
	"fmt"
	"os/exec"
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
func (r *Runner) Status(ctx context.Context) (*Status, error) {
	branchOut, err := r.run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	statusOut, err := r.run(ctx, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	r.lastCmd = "git status"
	return parseStatus(strings.TrimSpace(string(branchOut)), string(statusOut)), nil
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
	var entries []LogEntry
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

// Rename renames the current branch.
func (r *Runner) Rename(ctx context.Context, newName string) error {
	_, err := r.run(ctx, "branch", "-m", newName)
	return err
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
