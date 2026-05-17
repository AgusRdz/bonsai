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
		"--json", "number,title,state,url,isDraft,labels,reviewRequests,assignees,statusCheckRollup").Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w", err)
	}

	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"url"`
		Draft  bool   `json:"isDraft"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		ReviewRequests []struct {
			Login string `json:"login"`
		} `json:"reviewRequests"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		Checks []ciCheck `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh pr list parse: %w", err)
	}

	out2 := make([]PRStatus, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		reviewers := make([]string, len(r.ReviewRequests))
		for j, rr := range r.ReviewRequests {
			reviewers[j] = rr.Login
		}
		assignees := make([]string, len(r.Assignees))
		for j, a := range r.Assignees {
			assignees[j] = a.Login
		}
		out2[i] = PRStatus{
			Number:    r.Number,
			Title:     r.Title,
			State:     strings.ToLower(r.State),
			URL:       r.URL,
			CI:        rollupCI(r.Checks),
			Draft:     r.Draft,
			Labels:    labels,
			Reviewers: reviewers,
			Assignees: assignees,
		}
	}
	return out2, nil
}

func (g *ghProvider) Approve(ctx context.Context, number int) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	return exec.CommandContext(ctx, "gh", "pr", "review", fmt.Sprintf("%d", number), "--approve").Run()
}

func (g *ghProvider) RequestChanges(ctx context.Context, number int, body string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	args := []string{"pr", "review", fmt.Sprintf("%d", number), "--request-changes"}
	if body != "" {
		args = append(args, "--body", body)
	} else {
		args = append(args, "--body", "Changes requested.")
	}
	return exec.CommandContext(ctx, "gh", args...).Run()
}

func (g *ghProvider) ReviewComment(ctx context.Context, number int, body string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	return exec.CommandContext(ctx, "gh", "pr", "review", fmt.Sprintf("%d", number),
		"--comment", "--body", body).Run()
}

func (g *ghProvider) CommentPRLine(ctx context.Context, number int, path string, position int, body string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", fmt.Sprintf("%d", number),
		"--json", "headRefOid", "-q", ".headRefOid").Output()
	if err != nil {
		return fmt.Errorf("pr head SHA: %w", err)
	}
	commitID := strings.TrimSpace(string(out))
	cmd := exec.CommandContext(ctx, "gh", "api",
		fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments", number),
		"--method", "POST",
		"-f", "body="+body,
		"-f", "commit_id="+commitID,
		"-f", "path="+path,
		"-F", fmt.Sprintf("position=%d", position),
	)
	if out2, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh api: %s", strings.TrimSpace(string(out2)))
	}
	return nil
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

func (g *ghProvider) ListIssues(ctx context.Context) ([]Issue, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "issue", "list",
		"--json", "number,title,state,url,labels,assignees").Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w", err)
	}
	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"url"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh issue list parse: %w", err)
	}
	issues := make([]Issue, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		assignees := make([]string, len(r.Assignees))
		for j, a := range r.Assignees {
			assignees[j] = a.Login
		}
		issues[i] = Issue{
			Number:    r.Number,
			Title:     r.Title,
			State:     strings.ToLower(r.State),
			URL:       r.URL,
			Labels:    labels,
			Assignees: assignees,
		}
	}
	return issues, nil
}

func (g *ghProvider) CreateIssueBranch(ctx context.Context, number int, branchName string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	// gh issue develop creates a branch linked to the issue.
	return exec.CommandContext(ctx, "gh", "issue", "develop",
		fmt.Sprintf("%d", number), "--name", branchName).Run()
}

func (g *ghProvider) CreateRepo(ctx context.Context, name, visibility string) error {
	if !g.CLIAvailable() {
		return fmt.Errorf("gh CLI not found")
	}
	args := []string{"repo", "create", name}
	switch visibility {
	case "private":
		args = append(args, "--private")
	case "internal":
		args = append(args, "--internal")
	default:
		args = append(args, "--public")
	}
	return exec.CommandContext(ctx, "gh", args...).Run()
}

func (g *ghProvider) ProtectedBranches(ctx context.Context) ([]string, error) {
	if !g.CLIAvailable() {
		return nil, fmt.Errorf("gh CLI not found")
	}
	out, err := exec.CommandContext(ctx, "gh", "api", "repos/{owner}/{repo}/branches",
		"--jq", "[.[] | select(.protected) | .name]").Output()
	if err != nil {
		return nil, fmt.Errorf("gh api branches: %w", err)
	}
	var names []string
	if err := json.Unmarshal(out, &names); err != nil {
		return nil, fmt.Errorf("gh api branches parse: %w", err)
	}
	return names, nil
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
