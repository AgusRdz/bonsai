package setup

import (
	"testing"

	"github.com/AgusRdz/bonsai/config"
)

func TestBuildExample(t *testing.T) {
	if got := buildExample("feat/", "login-oauth"); got != "feat/login-oauth" {
		t.Errorf("buildExample = %q, want feat/login-oauth", got)
	}
	if got := buildExample("", "desc"); got != "desc" {
		t.Errorf("buildExample(empty prefix) = %q, want desc", got)
	}
	if got := buildExample("fix/", ""); got != "fix/" {
		t.Errorf("buildExample(empty suffix) = %q, want fix/", got)
	}
}

func TestTrunkDefaults(t *testing.T) {
	defs := trunkDefaults()
	if len(defs) != 3 {
		t.Fatalf("trunkDefaults len = %d, want 3", len(defs))
	}
	names := map[string]bool{"feature": true, "bugfix": true, "hotfix": true}
	for _, d := range defs {
		if !names[d.name] {
			t.Errorf("unexpected branch type: %q", d.name)
		}
		if d.prefix == "" {
			t.Errorf("branch type %q has empty prefix", d.name)
		}
		if d.example == "" {
			t.Errorf("branch type %q has empty example", d.name)
		}
	}
}

func TestGitflowDefaults(t *testing.T) {
	defs := gitflowDefaults()
	if len(defs) != 4 {
		t.Fatalf("gitflowDefaults len = %d, want 4", len(defs))
	}
	names := map[string]bool{"feature": true, "bugfix": true, "release": true, "hotfix": true}
	for _, d := range defs {
		if !names[d.name] {
			t.Errorf("unexpected branch type: %q", d.name)
		}
		if d.prefix == "" {
			t.Errorf("branch type %q has empty prefix", d.name)
		}
	}
}

func TestGithubflowDefaults(t *testing.T) {
	defs := githubflowDefaults()
	if len(defs) != 2 {
		t.Fatalf("githubflowDefaults len = %d, want 2", len(defs))
	}
	if defs[0].name != "feature" {
		t.Errorf("defs[0].name = %q, want feature", defs[0].name)
	}
	if defs[1].name != "bugfix" {
		t.Errorf("defs[1].name = %q, want bugfix", defs[1].name)
	}
	for _, d := range defs {
		if d.prefix == "" {
			t.Errorf("branch type %q has empty prefix", d.name)
		}
	}
}

func TestBuildExampleWithTicket(t *testing.T) {
	cases := []struct {
		prefix, key, suffix, want string
	}{
		{"feat/", "RES", "login-oauth", "feat/RES-123-login-oauth"},
		{"fix/", "PROJ", "crash", "fix/PROJ-123-crash"},
		{"feat/", "", "login-oauth", "feat/login-oauth"},
		{"chore/", "", "update-deps", "chore/update-deps"},
	}
	for _, c := range cases {
		if got := buildExampleWithTicket(c.prefix, c.key, c.suffix); got != c.want {
			t.Errorf("buildExampleWithTicket(%q,%q,%q) = %q, want %q", c.prefix, c.key, c.suffix, got, c.want)
		}
	}
}

func TestBuildPatternWithTicket(t *testing.T) {
	if got := buildPatternWithTicket("feat/", "RES"); got != "feat/RES-{number}-{description}" {
		t.Errorf("got %q", got)
	}
	if got := buildPatternWithTicket("feat/", ""); got != "feat/{description}" {
		t.Errorf("got %q", got)
	}
}

func TestInferTicketKey(t *testing.T) {
	branches := map[string]config.BranchRule{
		"feature": {Prefix: "feat/", Example: "feat/RES-123-login-oauth"},
	}
	if got := inferTicketKey(branches); got != "RES" {
		t.Errorf("inferTicketKey = %q, want RES", got)
	}
	if got := inferTicketKey(map[string]config.BranchRule{}); got != "" {
		t.Errorf("inferTicketKey(empty) = %q, want empty", got)
	}
}

func TestTrunkDefaultsPrefixes(t *testing.T) {
	defs := trunkDefaults()
	want := map[string]string{
		"feature": "feat/",
		"bugfix":  "bug/",
		"hotfix":  "hotfix/",
	}
	for _, d := range defs {
		if p, ok := want[d.name]; ok && d.prefix != p {
			t.Errorf("trunkDefaults %q prefix = %q, want %q", d.name, d.prefix, p)
		}
	}
}
