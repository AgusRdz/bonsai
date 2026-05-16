package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type ghProvider struct{}

func (g *ghProvider) Name() string { return "gh" }

func (g *ghProvider) CLIAvailable() bool { return cliExists("gh") }

func (g *ghProvider) DetectRemote(remoteURL string) bool {
	host := ParseRemoteHost(remoteURL)
	return strings.EqualFold(host, "github.com")
}

func (g *ghProvider) CurrentPR(ctx context.Context, branch string) (*PRStatus, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", branch,
		"--json", "number,title,state,url,statusCheckRollup").Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w", err)
	}

	var raw struct {
		Number int       `json:"number"`
		Title  string    `json:"title"`
		State  string    `json:"state"`
		URL    string    `json:"url"`
		Checks []ciCheck `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh pr view parse: %w", err)
	}

	ci := rollupCI(raw.Checks)
	return &PRStatus{
		Number: raw.Number,
		Title:  raw.Title,
		State:  strings.ToLower(raw.State),
		URL:    raw.URL,
		CI:     ci,
	}, nil
}

func (g *ghProvider) CreatePR(ctx context.Context, branch string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	cmd := exec.CommandContext(ctx, "gh", "pr", "create", "--head", branch, "--fill")
	cmd.Stdin = nil
	return cmd.Run()
}

func (g *ghProvider) ListPRs(ctx context.Context) ([]PRStatus, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "pr", "list",
		"--json", "number,title,state,url").Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}

	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh pr list parse: %w", err)
	}

	out2 := make([]PRStatus, len(raw))
	for i, r := range raw {
		out2[i] = PRStatus{Number: r.Number, Title: r.Title, State: strings.ToLower(r.State), URL: r.URL, CI: "none"}
	}
	return out2, nil
}

func (g *ghProvider) Open(ctx context.Context, branch string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	return exec.CommandContext(ctx, "gh", "pr", "view", branch, "--web").Run()
}

func (g *ghProvider) Diff(ctx context.Context, number int) (string, error) {
	if !g.CLIAvailable() {
		return "", fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "pr", "diff", fmt.Sprintf("%d", number)).Output()
	if err != nil {
		return "", fmt.Errorf("gh pr diff: %w", err)
	}
	return string(out), nil
}

func (g *ghProvider) Fork(ctx context.Context) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	return exec.CommandContext(ctx, "gh", "repo", "fork", "--clone=false", "--remote=true").Run()
}

// ciCheck holds one entry from gh's statusCheckRollup JSON field.
type ciCheck struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

// rollupCI collapses a list of check statuses into a single summary.
func rollupCI(checks []ciCheck) string {
	if len(checks) == 0 {
		return "none"
	}
	for _, c := range checks {
		if c.Conclusion == "FAILURE" || c.Conclusion == "TIMED_OUT" || c.Conclusion == "CANCELLED" {
			return "failure"
		}
	}
	for _, c := range checks {
		if c.Status == "IN_PROGRESS" || c.Status == "QUEUED" || c.Status == "PENDING" {
			return "pending"
		}
	}
	return "success"
}
