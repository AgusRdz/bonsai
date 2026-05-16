package tui

import (
	"strings"

	"github.com/AgusRdz/bonsai/config"
)

// flowOption is one branch type in the gitflow picker.
type flowOption struct {
	name    string // "feature"
	prefix  string // "feat/"
	example string // "feat/PROJ-123-description"
}

// detectFlow resolves "auto" to an actual flow name by inspecting the
// conventions config. All other values are returned as-is.
func detectFlow(cfg *config.Config) string {
	if cfg.Flow.Type != "auto" {
		return cfg.Flow.Type
	}
	b := cfg.Conventions.Branches
	_, hasFeature := b["feature"]
	_, hasBugfix := b["bugfix"]
	_, hasRelease := b["release"]
	_, hasHotfix := b["hotfix"]
	if hasFeature && hasBugfix && hasRelease && hasHotfix {
		return "gitflow"
	}
	return "githubflow"
}

// gitflowOptions returns the four gitflow branch types, pulling prefixes and
// examples from the conventions config when available.
func gitflowOptions(cfg *config.Config) []flowOption {
	order := []string{"feature", "bugfix", "release", "hotfix"}
	fallbacks := map[string]flowOption{
		"feature": {name: "feature", prefix: "feat/", example: "feat/PROJ-123-description"},
		"bugfix":  {name: "bugfix", prefix: "fix/", example: "fix/PROJ-456-crash"},
		"release": {name: "release", prefix: "release/", example: "release/1.2.0"},
		"hotfix":  {name: "hotfix", prefix: "hotfix/", example: "hotfix/critical-fix"},
	}
	var opts []flowOption
	for _, name := range order {
		opt := fallbacks[name]
		if rule, ok := cfg.Conventions.Branches[name]; ok && rule.Prefix != "" {
			opt.prefix = rule.Prefix
			if rule.Example != "" {
				opt.example = rule.Example
			}
		}
		opts = append(opts, opt)
	}
	return opts
}

// flowHint returns a short flow-specific sentence to append to the education
// panel after certain git actions. Returns "" when no hint applies.
func flowHint(cmd, flow string) string {
	switch flow {
	case "gitflow":
		switch {
		case strings.HasPrefix(cmd, "git switch -c"):
			return "gitflow: when the work is complete, merge this branch into develop - not directly into main."
		case strings.HasPrefix(cmd, "git push"):
			return "gitflow: open a PR targeting develop so your changes can be reviewed before merging."
		}
	case "trunk":
		switch {
		case strings.HasPrefix(cmd, "git switch -c"):
			return "trunk-based: keep this branch short-lived and merge back to main as soon as possible."
		case strings.HasPrefix(cmd, "git commit"):
			return "trunk-based: push and merge frequently to avoid diverging from main."
		}
	case "githubflow", "forking":
		if strings.HasPrefix(cmd, "git push") {
			return "github flow: open a pull request on GitHub to review and merge these changes into main."
		}
	}
	return ""
}
