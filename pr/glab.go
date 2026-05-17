package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type glabProvider struct{}

func (g *glabProvider) Name() string { return "glab" }

func (g *glabProvider) CLIAvailable() bool { return cliExists("glab") }

func (g *glabProvider) DetectRemote(remoteURL string) bool {
	host := ParseRemoteHost(remoteURL)
	// Match gitlab.com and self-hosted GitLab instances (gitlab.*)
	return strings.EqualFold(host, "gitlab.com") || strings.Contains(strings.ToLower(host), "gitlab")
}

func (g *glabProvider) CurrentPR(ctx context.Context, branch string) (*PRStatus, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("glab CLI not found")
	}
	// glab calls them MRs; "mr view" shows the MR for the current or given branch
	out, err := exec.CommandContext(ctx, "glab", "mr", "view", branch, "--output", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("glab mr view: %w", err)
	}

	var raw struct {
		IID   int    `json:"iid"`
		Title string `json:"title"`
		State string `json:"state"`
		URL   string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("glab mr view parse: %w", err)
	}

	return &PRStatus{
		Number: raw.IID,
		Title:  raw.Title,
		State:  normaliseGlabState(raw.State),
		URL:    raw.URL,
		CI:     "none", // pipeline status requires an extra call; omit for now
	}, nil
}

func (g *glabProvider) CreatePR(ctx context.Context, branch string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	out, err := exec.CommandContext(ctx, "glab", "mr", "create", "--source-branch", branch, "--web").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

func (g *glabProvider) ListPRs(ctx context.Context) ([]PRStatus, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("glab CLI not found")
	}
	out, err := exec.CommandContext(ctx, "glab", "mr", "list", "--output", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("glab mr list: %w", err)
	}

	var raw []struct {
		IID   int    `json:"iid"`
		Title string `json:"title"`
		State string `json:"state"`
		URL   string `json:"web_url"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("glab mr list parse: %w", err)
	}

	out2 := make([]PRStatus, len(raw))
	for i, r := range raw {
		out2[i] = PRStatus{Number: r.IID, Title: r.Title, State: normaliseGlabState(r.State), URL: r.URL, CI: "none"}
	}
	return out2, nil
}

func (g *glabProvider) Open(ctx context.Context, branch string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	return exec.CommandContext(ctx, "glab", "mr", "view", branch, "--web").Run()
}

func (g *glabProvider) Diff(ctx context.Context, number int) (string, error) {
	if !g.CLIAvailable() {
		return "", fmt.Errorf("glab CLI not found")
	}
	out, err := exec.CommandContext(ctx, "glab", "mr", "diff", fmt.Sprintf("%d", number)).Output()
	if err != nil {
		return "", fmt.Errorf("glab mr diff: %w", err)
	}
	return string(out), nil
}

func (g *glabProvider) Fork(ctx context.Context) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	return exec.CommandContext(ctx, "glab", "repo", "fork").Run()
}

func (g *glabProvider) ListIssues(ctx context.Context) ([]Issue, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("glab CLI not found")
	}
	out, err := exec.CommandContext(ctx, "glab", "issue", "list", "--output", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("glab issue list: %w", err)
	}
	var raw []struct {
		IID       int      `json:"iid"`
		Title     string   `json:"title"`
		State     string   `json:"state"`
		URL       string   `json:"web_url"`
		Labels    []string `json:"labels"`
		Assignees []struct {
			Username string `json:"username"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("glab issue list parse: %w", err)
	}
	issues := make([]Issue, len(raw))
	for i, r := range raw {
		assignees := make([]string, len(r.Assignees))
		for j, a := range r.Assignees {
			assignees[j] = a.Username
		}
		issues[i] = Issue{Number: r.IID, Title: r.Title, State: strings.ToLower(r.State), URL: r.URL, Labels: r.Labels, Assignees: assignees}
	}
	return issues, nil
}

func (g *glabProvider) CreateIssueBranch(ctx context.Context, number int, branchName string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	return exec.CommandContext(ctx, "glab", "issue", "create-branch",
		fmt.Sprintf("%d", number), "--name", branchName).Run()
}

func (g *glabProvider) CreateRepo(ctx context.Context, name, visibility string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	args := []string{"repo", "create", name}
	if visibility == "private" {
		args = append(args, "--visibility", "private")
	}
	return exec.CommandContext(ctx, "glab", args...).Run()
}

func (g *glabProvider) Approve(ctx context.Context, number int) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	return exec.CommandContext(ctx, "glab", "mr", "approve", fmt.Sprintf("%d", number)).Run()
}

func (g *glabProvider) RequestChanges(_ context.Context, _ int, _ string) error {
	return fmt.Errorf("glab CLI does not support request-changes - use the web interface")
}

func (g *glabProvider) ReviewComment(ctx context.Context, number int, body string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("glab CLI not found")
	}
	return exec.CommandContext(ctx, "glab", "mr", "note", fmt.Sprintf("%d", number), "--message", body).Run()
}

func (g *glabProvider) ProtectedBranches(ctx context.Context) ([]string, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("glab CLI not found")
	}
	out, err := exec.CommandContext(ctx, "glab", "api", "projects/:fullpath/protected_branches",
		"--field", "per_page=100").Output()
	if err != nil {
		return nil, fmt.Errorf("glab api protected branches: %w", err)
	}
	var raw []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("glab api protected branches parse: %w", err)
	}
	names := make([]string, len(raw))
	for i, r := range raw {
		names[i] = r.Name
	}
	return names, nil
}

func normaliseGlabState(s string) string {
	switch strings.ToLower(s) {
	case "opened":
		return "open"
	case "merged":
		return "merged"
	case "closed":
		return "closed"
	default:
		return strings.ToLower(s)
	}
}
