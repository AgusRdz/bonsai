package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type bbProvider struct{}

func (b *bbProvider) Name() string { return "bb" }

func (b *bbProvider) CLIAvailable() bool { return cliExists("bb") }

func (b *bbProvider) DetectRemote(remoteURL string) bool {
	host := ParseRemoteHost(remoteURL)
	return strings.EqualFold(host, "bitbucket.org")
}

func (b *bbProvider) CurrentPR(ctx context.Context, branch string) (*PRStatus, error) {
	if !b.CLIAvailable() {
		return nil, fmt.Errorf("bb CLI not found")
	}
	// bb pr get --current outputs JSON with the PR for the current branch
	out, err := exec.CommandContext(ctx, "bb", "pr", "get", "--current", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("bb pr get: %w", err)
	}

	var raw struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		State string `json:"state"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("bb pr get parse: %w", err)
	}

	return &PRStatus{
		Number: raw.ID,
		Title:  raw.Title,
		State:  normaliseBBState(raw.State),
		URL:    raw.Links.HTML.Href,
		CI:     "none",
	}, nil
}

func (b *bbProvider) CreatePR(ctx context.Context, branch string) error {
	if !b.CLIAvailable() {
		// Fall back to browser if CLI is unavailable
		return fmt.Errorf("bb CLI not found - open Bitbucket in your browser to create a PR")
	}
	return exec.CommandContext(ctx, "bb", "pr", "create", "--source", branch).Run()
}

func (b *bbProvider) ListPRs(ctx context.Context) ([]PRStatus, error) {
	if !b.CLIAvailable() {
		return nil, fmt.Errorf("bb CLI not found")
	}
	out, err := exec.CommandContext(ctx, "bb", "pr", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("bb pr list: %w", err)
	}

	var raw []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		State string `json:"state"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("bb pr list parse: %w", err)
	}

	out2 := make([]PRStatus, len(raw))
	for i, r := range raw {
		out2[i] = PRStatus{
			Number: r.ID,
			Title:  r.Title,
			State:  normaliseBBState(r.State),
			URL:    r.Links.HTML.Href,
			CI:     "none",
		}
	}
	return out2, nil
}

func (b *bbProvider) Open(ctx context.Context, branch string) error {
	if !b.CLIAvailable() {
		// branch is the PR URL when called from the TUI list panel
		if branch != "" {
			return openBrowser(ctx, branch)
		}
		return fmt.Errorf("bb CLI not found")
	}
	return exec.CommandContext(ctx, "bb", "pr", "view", "--current", "--browser").Run()
}

func normaliseBBState(s string) string {
	switch strings.ToUpper(s) {
	case "OPEN":
		return "open"
	case "MERGED":
		return "merged"
	case "DECLINED", "SUPERSEDED":
		return "closed"
	default:
		return strings.ToLower(s)
	}
}
