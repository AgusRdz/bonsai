package tui

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 5 * time.Second

type gitState struct {
	branch    string
	staged    []string
	changed   []string
	untracked []string
}

func loadGitState() (gitState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	branch, err := gitBranch(ctx)
	if err != nil {
		return gitState{}, err
	}

	out, err := exec.CommandContext(ctx, "git", "status", "--porcelain").Output()
	if err != nil {
		return gitState{}, err
	}

	staged, changed, untracked := parseStatus(string(out))
	return gitState{
		branch:    branch,
		staged:    staged,
		changed:   changed,
		untracked: untracked,
	}, nil
}

func gitBranch(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseStatus parses `git status --porcelain` output into three categories.
// A file with both staged and unstaged changes appears in both slices.
func parseStatus(porcelain string) (staged, changed, untracked []string) {
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		path := strings.TrimSpace(line[3:])
		x, y := rune(xy[0]), rune(xy[1])

		if xy == "??" {
			untracked = append(untracked, path)
			continue
		}
		if x != ' ' && x != '?' {
			staged = append(staged, string(x)+"  "+path)
		}
		if y != ' ' && y != '?' {
			changed = append(changed, string(y)+"  "+path)
		}
	}
	return
}
