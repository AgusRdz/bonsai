package pr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemoteHost(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"git@github.com:owner/repo", "github.com"},
		{"git@gitlab.com:owner/repo.git", "gitlab.com"},
		{"git@bitbucket.org:owner/repo.git", "bitbucket.org"},
		{"https://github.com/owner/repo", "github.com"},
		{"https://gitlab.com/owner/repo.git", "gitlab.com"},
		{"http://gitlab.internal/owner/repo", "gitlab.internal"},
		{"git@self-hosted.gitlab.company.com:owner/repo", "self-hosted.gitlab.company.com"},
	}
	for _, c := range cases {
		got := ParseRemoteHost(c.url)
		if got != c.want {
			t.Errorf("ParseRemoteHost(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestDetectRemoteGH(t *testing.T) {
	p := &ghProvider{}
	cases := []struct {
		url  string
		want bool
	}{
		{"git@github.com:owner/repo", true},
		{"https://github.com/owner/repo", true},
		{"git@gitlab.com:owner/repo", false},
		{"https://bitbucket.org/owner/repo", false},
	}
	for _, c := range cases {
		if got := p.DetectRemote(c.url); got != c.want {
			t.Errorf("ghProvider.DetectRemote(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestDetectRemoteGlab(t *testing.T) {
	p := &glabProvider{}
	cases := []struct {
		url  string
		want bool
	}{
		{"git@gitlab.com:owner/repo", true},
		{"https://gitlab.com/owner/repo", true},
		{"git@self.gitlab.example.com:owner/repo", true},
		{"https://github.com/owner/repo", false},
		{"https://bitbucket.org/owner/repo", false},
	}
	for _, c := range cases {
		if got := p.DetectRemote(c.url); got != c.want {
			t.Errorf("glabProvider.DetectRemote(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestDetectRemoteBB(t *testing.T) {
	p := &bbProvider{}
	cases := []struct {
		url  string
		want bool
	}{
		{"git@bitbucket.org:owner/repo", true},
		{"https://bitbucket.org/owner/repo", true},
		{"https://github.com/owner/repo", false},
	}
	for _, c := range cases {
		if got := p.DetectRemote(c.url); got != c.want {
			t.Errorf("bbProvider.DetectRemote(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestDetectRegistry(t *testing.T) {
	old := registry
	registry = nil
	defer func() { registry = old }()

	Register(&ghProvider{})
	Register(&glabProvider{})
	Register(&bbProvider{})

	cases := []struct {
		url      string
		wantName string
	}{
		{"git@github.com:owner/repo", "gh"},
		{"https://gitlab.com/owner/repo", "glab"},
		{"git@bitbucket.org:owner/repo", "bb"},
	}
	for _, c := range cases {
		p := Detect(c.url)
		if p == nil {
			t.Errorf("Detect(%q) = nil, want %q", c.url, c.wantName)
			continue
		}
		if p.Name() != c.wantName {
			t.Errorf("Detect(%q).Name() = %q, want %q", c.url, p.Name(), c.wantName)
		}
	}

	if p := Detect("https://unknown.example.com/repo"); p != nil {
		t.Errorf("Detect(unknown) = %q, want nil", p.Name())
	}
}

func TestDiscoverPluginsEmpty(t *testing.T) {
	old := registry
	registry = nil
	defer func() { registry = old }()

	// Set PATH to an empty dir - no plugins should be found
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	discoverPlugins()
	if len(registry) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(registry))
	}
}

func TestDiscoverPluginsFindsExecutable(t *testing.T) {
	old := registry
	registry = nil
	defer func() { registry = old }()

	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-pr-myprovider")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho yes\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("PATH", dir)
	discoverPlugins()

	if len(registry) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(registry))
	}
	if registry[0].Name() != "myprovider" {
		t.Errorf("plugin name = %q, want myprovider", registry[0].Name())
	}
}

func TestDiscoverPluginsSkipsNonExecutable(t *testing.T) {
	old := registry
	registry = nil
	defer func() { registry = old }()

	dir := t.TempDir()
	bin := filepath.Join(dir, "bonsai-pr-nope")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Setenv("PATH", dir)
	discoverPlugins()

	if len(registry) != 0 {
		t.Errorf("expected 0 plugins (non-executable), got %d", len(registry))
	}
}

func TestNormaliseGlabState(t *testing.T) {
	cases := []struct{ in, want string }{
		{"opened", "open"},
		{"OPENED", "open"},
		{"merged", "merged"},
		{"closed", "closed"},
		{"locked", "locked"},
	}
	for _, c := range cases {
		if got := normaliseGlabState(c.in); got != c.want {
			t.Errorf("normaliseGlabState(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormaliseBBState(t *testing.T) {
	cases := []struct{ in, want string }{
		{"OPEN", "open"},
		{"MERGED", "merged"},
		{"DECLINED", "closed"},
		{"SUPERSEDED", "closed"},
		{"unknown", "unknown"},
	}
	for _, c := range cases {
		if got := normaliseBBState(c.in); got != c.want {
			t.Errorf("normaliseBBState(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRollupCI(t *testing.T) {
	cases := []struct {
		checks []ciCheck
		want   string
	}{
		{nil, "none"},
		{[]ciCheck{}, "none"},
		{[]ciCheck{{Conclusion: "SUCCESS", Status: "COMPLETED"}}, "success"},
		{[]ciCheck{{Conclusion: "FAILURE", Status: "COMPLETED"}}, "failure"},
		{[]ciCheck{{Conclusion: "TIMED_OUT", Status: "COMPLETED"}}, "failure"},
		{[]ciCheck{{Status: "IN_PROGRESS", Conclusion: ""}}, "pending"},
		{[]ciCheck{{Conclusion: "SUCCESS"}, {Status: "IN_PROGRESS"}}, "pending"},
		{[]ciCheck{{Conclusion: "FAILURE"}, {Status: "IN_PROGRESS"}}, "failure"},
	}
	for _, c := range cases {
		got := rollupCI(c.checks)
		if got != c.want {
			t.Errorf("rollupCI(%v) = %q, want %q", c.checks, got, c.want)
		}
	}
}
