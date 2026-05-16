package conventions

import (
	"strings"

	"github.com/AgusRdz/bonsai/config"
)

// specialBranches are never subject to convention validation.
var specialBranches = map[string]bool{
	"main": true, "master": true, "HEAD": true,
	"develop": true, "development": true,
}

// RuleMatch pairs a rule name with its config.
type RuleMatch struct {
	Name string
	Rule config.BranchRule
}

// Result is the outcome of validating a branch name.
type Result struct {
	Branch  string
	Valid   bool
	Special bool        // true for main, master, develop, etc.
	Match   *RuleMatch  // set when Valid && !Special
	Rules   []RuleMatch // all configured rules, for display when invalid
}

// Validate checks whether branch satisfies the configured conventions.
// Special branches (main, master, HEAD, develop) always pass.
// Validation is prefix-only: a branch is valid if it starts with any configured prefix.
func Validate(branch string, cfg config.ConventionsConfig) Result {
	if specialBranches[branch] || branch == "" {
		return Result{Branch: branch, Valid: true, Special: true}
	}

	rules := orderedRules(cfg.Branches)

	for i, rm := range rules {
		if rm.Rule.Prefix != "" && strings.HasPrefix(branch, rm.Rule.Prefix) {
			match := rules[i]
			return Result{Branch: branch, Valid: true, Match: &match, Rules: rules}
		}
	}

	return Result{Branch: branch, Valid: false, Rules: rules}
}

// Rules returns all configured rules in stable display order.
func Rules(cfg config.ConventionsConfig) []RuleMatch {
	return orderedRules(cfg.Branches)
}

// orderedRules returns rules in a stable display order.
// Well-known types come first; any extra user-defined types append at the end.
func orderedRules(branches map[string]config.BranchRule) []RuleMatch {
	known := []string{"feature", "bugfix", "release", "hotfix"}
	seen := map[string]bool{}
	var rules []RuleMatch

	for _, name := range known {
		if rule, ok := branches[name]; ok {
			rules = append(rules, RuleMatch{Name: name, Rule: rule})
			seen[name] = true
		}
	}
	for name, rule := range branches {
		if !seen[name] {
			rules = append(rules, RuleMatch{Name: name, Rule: rule})
		}
	}
	return rules
}
