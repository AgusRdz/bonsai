package pr

import (
	"context"
	"os/exec"
	"strings"
)

// PRStatus holds the current state of a pull request.
type PRStatus struct {
	Number    int
	Title     string
	State     string // "open" | "closed" | "merged"
	URL       string
	CI        string // "success" | "failure" | "pending" | "none"
	Draft     bool
	Labels    []string
	Reviewers []string // requested reviewer login names
	Assignees []string
}

// PRCreateOpts holds the parameters for creating a new pull request.
type PRCreateOpts struct {
	Branch string
	Title  string
	Body   string
	Base   string // target branch; empty means provider default
}

// Provider abstracts a PR hosting platform (GitHub, GitLab, Bitbucket, or plugin).
type Provider interface {
	Name() string
	CLIAvailable() bool
	DetectRemote(remoteURL string) bool
	CurrentPR(ctx context.Context, branch string) (*PRStatus, error)
	CreatePR(ctx context.Context, opts PRCreateOpts) error
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

// DetectByCLI returns the first registered provider whose CLI is available on
// the system. Useful as a fallback when no remote URL is configured.
func DetectByCLI() Provider {
	for _, p := range registry {
		if p.CLIAvailable() {
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

// ProtectionChecker is an optional Provider extension for querying which branches
// are protected on the remote. Type-assert before calling: c, ok := p.(ProtectionChecker).
type ProtectionChecker interface {
	ProtectedBranches(ctx context.Context) ([]string, error)
}

// Issue holds the data for one issue from the hosting platform.
type Issue struct {
	Number    int
	Title     string
	State     string
	URL       string
	Labels    []string
	Assignees []string
}

// IssueProvider is an optional Provider extension for fetching issues.
// Type-assert before calling: ip, ok := p.(IssueProvider).
type IssueProvider interface {
	ListIssues(ctx context.Context) ([]Issue, error)
	CreateIssueBranch(ctx context.Context, number int, branchName string) error
}

// RepoCreator is an optional Provider extension for creating a new remote repo.
// Type-assert before calling: rc, ok := p.(RepoCreator).
type RepoCreator interface {
	CreateRepo(ctx context.Context, name, visibility string) error
}

// PRReviewer is an optional Provider extension for submitting PR reviews.
// Type-assert before calling: r, ok := p.(PRReviewer).
type PRReviewer interface {
	Approve(ctx context.Context, number int) error
	RequestChanges(ctx context.Context, number int, body string) error
	ReviewComment(ctx context.Context, number int, body string) error
}

// PRMerger is an optional Provider extension for merging a pull request.
// method must be "merge", "squash", or "rebase". Type-assert before calling.
type PRMerger interface {
	MergePR(ctx context.Context, number int, method string) error
}

// PRLineCommenter is an optional Provider extension for posting inline diff comments
// on a specific line of a pull request. Type-assert before calling.
// position is 1-based from the first @@ line within the file's diff section.
type PRLineCommenter interface {
	CommentPRLine(ctx context.Context, number int, path string, position int, body string) error
}
