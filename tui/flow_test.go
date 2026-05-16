package tui

import (
	"testing"

	"github.com/AgusRdz/bonsai/config"
)

func TestDetectFlowPassthrough(t *testing.T) {
	for _, flow := range []string{"trunk", "gitflow", "githubflow", "forking"} {
		cfg := &config.Config{Flow: config.FlowConfig{Type: flow}}
		if got := detectFlow(cfg); got != flow {
			t.Errorf("detectFlow(%q) = %q, want %q", flow, got, flow)
		}
	}
}

func TestDetectFlowAutoGitflow(t *testing.T) {
	cfg := &config.Config{
		Flow: config.FlowConfig{Type: "auto"},
		Conventions: config.ConventionsConfig{
			Branches: map[string]config.BranchRule{
				"feature": {Prefix: "feat/"},
				"bugfix":  {Prefix: "fix/"},
				"release": {Prefix: "release/"},
				"hotfix":  {Prefix: "hotfix/"},
			},
		},
	}
	if got := detectFlow(cfg); got != "gitflow" {
		t.Errorf("detectFlow(auto, all four types) = %q, want gitflow", got)
	}
}

func TestDetectFlowAutoFallback(t *testing.T) {
	cases := []struct {
		branches map[string]config.BranchRule
		desc     string
	}{
		{nil, "empty"},
		{map[string]config.BranchRule{"feature": {Prefix: "feat/"}}, "feature only"},
		{map[string]config.BranchRule{
			"feature": {Prefix: "feat/"},
			"bugfix":  {Prefix: "fix/"},
			"release": {Prefix: "release/"},
		}, "missing hotfix"},
	}
	for _, c := range cases {
		cfg := &config.Config{
			Flow:        config.FlowConfig{Type: "auto"},
			Conventions: config.ConventionsConfig{Branches: c.branches},
		}
		if got := detectFlow(cfg); got != "githubflow" {
			t.Errorf("detectFlow(auto, %s) = %q, want githubflow", c.desc, got)
		}
	}
}

func TestGitflowOptionsFallbackPrefixes(t *testing.T) {
	cfg := &config.Config{}
	opts := gitflowOptions(cfg)
	if len(opts) != 4 {
		t.Fatalf("len = %d, want 4", len(opts))
	}
	want := []struct {
		name   string
		prefix string
	}{
		{"feature", "feat/"},
		{"bugfix", "fix/"},
		{"release", "release/"},
		{"hotfix", "hotfix/"},
	}
	for i, w := range want {
		if opts[i].name != w.name {
			t.Errorf("opts[%d].name = %q, want %q", i, opts[i].name, w.name)
		}
		if opts[i].prefix != w.prefix {
			t.Errorf("opts[%d].prefix = %q, want %q", i, opts[i].prefix, w.prefix)
		}
		if opts[i].example == "" {
			t.Errorf("opts[%d].example is empty", i)
		}
	}
}

func TestGitflowOptionsFromConfig(t *testing.T) {
	cfg := &config.Config{
		Conventions: config.ConventionsConfig{
			Branches: map[string]config.BranchRule{
				"feature": {Prefix: "feature/", Example: "feature/JIRA-123-desc"},
				"bugfix":  {Prefix: "bug/"},
			},
		},
	}
	opts := gitflowOptions(cfg)
	if len(opts) != 4 {
		t.Fatalf("len = %d, want 4", len(opts))
	}
	if opts[0].prefix != "feature/" {
		t.Errorf("feature prefix = %q, want feature/", opts[0].prefix)
	}
	if opts[0].example != "feature/JIRA-123-desc" {
		t.Errorf("feature example = %q, want feature/JIRA-123-desc", opts[0].example)
	}
	if opts[1].prefix != "bug/" {
		t.Errorf("bugfix prefix = %q, want bug/", opts[1].prefix)
	}
	// release and hotfix should still use fallbacks
	if opts[2].prefix != "release/" {
		t.Errorf("release prefix = %q, want release/", opts[2].prefix)
	}
}

func TestFlowHintGitflow(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"git switch -c feat/x", "gitflow"},
		{"git push origin feat/x", "gitflow"},
		{"git commit -m msg", ""},
	}
	for _, c := range cases {
		got := flowHint(c.cmd, "gitflow")
		if c.want == "" && got != "" {
			t.Errorf("flowHint(%q, gitflow) = %q, want empty", c.cmd, got)
		}
		if c.want != "" && got == "" {
			t.Errorf("flowHint(%q, gitflow) returned empty, want non-empty", c.cmd)
		}
	}
}

func TestFlowHintTrunk(t *testing.T) {
	cases := []struct {
		cmd     string
		wantHit bool
	}{
		{"git switch -c feat/x", true},
		{"git commit -m msg", true},
		{"git push origin feat/x", false},
	}
	for _, c := range cases {
		got := flowHint(c.cmd, "trunk")
		if c.wantHit && got == "" {
			t.Errorf("flowHint(%q, trunk) returned empty, want non-empty", c.cmd)
		}
		if !c.wantHit && got != "" {
			t.Errorf("flowHint(%q, trunk) = %q, want empty", c.cmd, got)
		}
	}
}

func TestFlowHintGithubflow(t *testing.T) {
	got := flowHint("git push origin main", "githubflow")
	if got == "" {
		t.Error("flowHint(push, githubflow) returned empty, want non-empty")
	}
	got2 := flowHint("git push origin main", "forking")
	if got2 == "" {
		t.Error("flowHint(push, forking) returned empty, want non-empty")
	}
}

func TestFlowHintUnknownFlow(t *testing.T) {
	if got := flowHint("git push", "unknown"); got != "" {
		t.Errorf("flowHint(push, unknown) = %q, want empty", got)
	}
}

func TestFlowHintNoMatchReturnsEmpty(t *testing.T) {
	if got := flowHint("git fetch", "gitflow"); got != "" {
		t.Errorf("flowHint(fetch, gitflow) = %q, want empty", got)
	}
}
