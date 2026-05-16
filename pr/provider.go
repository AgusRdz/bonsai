package pr

import (
	"context"
	"os/exec"
	"strings"
)

// PRStatus holds the current state of a pull request.
type PRStatus struct {
	Number int
	Title  string
	State  string // "open" | "closed" | "merged"
	URL    string
	CI     string // "success" | "failure" | "pending" | "none"
}

// Provider abstracts a PR hosting platform (GitHub, GitLab, Bitbucket, or plugin).
type Provider interface {
	Name() string
	CLIAvailable() bool
	DetectRemote(remoteURL string) bool
	CurrentPR(ctx context.Context, branch string) (*PRStatus, error)
	CreatePR(ctx context.Context, branch string) error
	ListPRs(ctx context.Context) ([]PRStatus, error)
	Open(ctx context.Context, branch string) error
}

var registry []Provider

// Register adds a provider to the global registry.
func Register(p Provider) {
	registry = append(registry, p)
}

// RegisterBuiltins registers gh, glab, and bb providers and discovers plugins.
func RegisterBuiltins() {
	Register(&ghProvider{})
	Register(&glabProvider{})
	Register(&bbProvider{})
	discoverPlugins()
}

// Detect returns the first provider that recognises the remote URL, or nil.
func Detect(remoteURL string) Provider {
	for _, p := range registry {
		if p.DetectRemote(remoteURL) {
			return p
		}
	}
	return nil
}

// ParseRemoteHost extracts the hostname from an SSH or HTTPS remote URL.
// Examples: "git@github.com:owner/repo" -> "github.com"
//
//	"https://gitlab.com/owner/repo" -> "gitlab.com"
func ParseRemoteHost(url string) string {
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		if idx := strings.Index(url, ":"); idx >= 0 {
			return url[:idx]
		}
	}
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	if idx := strings.Index(url, "/"); idx >= 0 {
		return url[:idx]
	}
	return url
}

func cliExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// PRDiffer is an optional Provider extension for fetching a PR/MR unified diff.
// Type-assert the Provider before calling: d, ok := p.(PRDiffer).
type PRDiffer interface {
	Diff(ctx context.Context, number int) (string, error)
}

// PRForker is an optional Provider extension for forking the current repo.
// Type-assert the Provider before calling: f, ok := p.(PRForker).
type PRForker interface {
	Fork(ctx context.Context) error
}
