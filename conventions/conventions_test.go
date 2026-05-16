package conventions

import (
	"testing"

	"github.com/AgusRdz/bonsai/config"
)

var testCfg = config.ConventionsConfig{
	Branches: map[string]config.BranchRule{
		"feature": {Prefix: "feat/", Pattern: "feat/{ticket-id}-{description}", Example: "feat/PROJ-123-login"},
		"bugfix":  {Prefix: "fix/", Pattern: "fix/{ticket-id}-{description}", Example: "fix/PROJ-456-crash"},
		"release": {Prefix: "release/"},
		"hotfix":  {Prefix: "hotfix/"},
	},
	Validation: config.ValidationConfig{Mode: "strict"},
}

func TestValidateSpecialBranches(t *testing.T) {
	specials := []string{"main", "master", "HEAD", "develop", "development"}
	for _, b := range specials {
		r := Validate(b, testCfg)
		if !r.Valid {
			t.Errorf("Validate(%q) should be valid (special branch)", b)
		}
		if !r.Special {
			t.Errorf("Validate(%q) should set Special=true", b)
		}
	}
}

func TestValidateMatchingBranches(t *testing.T) {
	cases := []struct {
		branch    string
		matchName string
	}{
		{"feat/PROJ-123-login-oauth", "feature"},
		{"feat/my-feature", "feature"},
		{"fix/PROJ-456-crash", "bugfix"},
		{"release/1.2.0", "release"},
		{"hotfix/urgent", "hotfix"},
	}
	for _, c := range cases {
		r := Validate(c.branch, testCfg)
		if !r.Valid {
			t.Errorf("Validate(%q) should be valid", c.branch)
		}
		if r.Match == nil || r.Match.Name != c.matchName {
			t.Errorf("Validate(%q).Match = %v, want %q", c.branch, r.Match, c.matchName)
		}
	}
}

func TestValidateNonMatchingBranches(t *testing.T) {
	cases := []string{
		"johns-branch",
		"my-feature",
		"PROJ-123",
		"random",
		"feature/no-slash-prefix", // "feat/" prefix, not "feature/"
	}
	for _, b := range cases {
		r := Validate(b, testCfg)
		if r.Valid {
			t.Errorf("Validate(%q) should be invalid", b)
		}
		if r.Match != nil {
			t.Errorf("Validate(%q).Match should be nil for invalid branch", b)
		}
		if len(r.Rules) == 0 {
			t.Errorf("Validate(%q).Rules should not be empty", b)
		}
	}
}

func TestValidateEmptyBranch(t *testing.T) {
	r := Validate("", testCfg)
	if !r.Valid {
		t.Error("empty branch should be considered valid (special case)")
	}
}

func TestOrderedRules(t *testing.T) {
	rules := orderedRules(testCfg.Branches)
	if len(rules) == 0 {
		t.Fatal("expected rules, got none")
	}
	// feature should come before hotfix in the standard order
	featureIdx, hotfixIdx := -1, -1
	for i, r := range rules {
		if r.Name == "feature" {
			featureIdx = i
		}
		if r.Name == "hotfix" {
			hotfixIdx = i
		}
	}
	if featureIdx == -1 || hotfixIdx == -1 {
		t.Fatal("feature and hotfix should both be present")
	}
	if featureIdx >= hotfixIdx {
		t.Errorf("feature (idx %d) should come before hotfix (idx %d)", featureIdx, hotfixIdx)
	}
}

func TestValidateEmptyConventions(t *testing.T) {
	emptyCfg := config.ConventionsConfig{}
	r := Validate("any-branch", emptyCfg)
	// With no rules configured, every non-special branch is invalid
	if r.Valid {
		t.Error("with no rules configured, non-special branch should be invalid")
	}
}
