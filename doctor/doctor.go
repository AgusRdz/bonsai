package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/conventions"
)

// Level represents the severity of a check result.
type Level int

const (
	OK Level = iota
	Warn
	Fail
)

// Check is the result of a single health check.
type Check struct {
	Level   Level
	Label   string // short label, e.g. "user.name"
	Message string // full description of result
	Fix     string // optional: what to do if not OK
	Explain string // one-line explanation shown with --verbose
}

// Report groups the global and local check results.
type Report struct {
	Global []Check
	Local  []Check
	InRepo bool // false if not in a git repo
}

// Run executes all health checks and returns a Report.
func Run() (*Report, error) {
	r := &Report{}

	r.Global = runGlobalChecks()

	// Detect whether we are inside a git repo.
	_, err := gitOutput("rev-parse", "--git-dir")
	r.InRepo = err == nil

	if r.InRepo {
		r.Local = runLocalChecks()
	}

	return r, nil
}

// gitOutput runs git with the given args and returns trimmed stdout.
func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

// gitConfig reads a single git config key (global scope) and returns its value.
// Returns "" when the key is not set (exit code 1).
func gitConfig(key string) string {
	out, err := exec.Command("git", "config", "--global", "--get", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runGlobalChecks runs all checks that do not require a git repo.
func runGlobalChecks() []Check {
	var checks []Check

	// 1. git version
	checks = append(checks, checkGitVersion())

	// 2. user.name
	checks = append(checks, checkUserName())

	// 3. user.email
	checks = append(checks, checkUserEmail())

	// 4. credential.helper
	checks = append(checks, checkCredentialHelper())

	// 5. init.defaultBranch
	checks = append(checks, checkDefaultBranch())

	// 6. pull.rebase
	checks = append(checks, checkPullRebase())

	// 7. fetch.prune
	checks = append(checks, checkFetchPrune())

	// 8. push.autoSetupRemote
	checks = append(checks, checkPushAutoSetupRemote())

	// 9. rerere.enabled
	checks = append(checks, checkRerereEnabled())

	// 10. core.editor
	checks = append(checks, checkEditor())

	// 11. global gitignore
	checks = append(checks, checkGlobalGitignore())

	// 12. gpg signing
	checks = append(checks, checkGPGSigning())

	return checks
}

const explainGitVersion = "git 2.28+ introduced init.defaultBranch and other settings bonsai depends on; older versions may silently ignore them."

func checkGitVersion() Check {
	out, err := gitOutput("--version")
	if err != nil {
		return Check{
			Level:   Fail,
			Label:   "git version",
			Message: "git not found",
			Fix:     "install git: https://git-scm.com/downloads",
			Explain: explainGitVersion,
		}
	}
	// "git version 2.39.0" - parse major.minor
	ver := strings.TrimPrefix(out, "git version ")
	parts := strings.SplitN(ver, ".", 3)
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if major > 2 || (major == 2 && minor >= 28) {
		return Check{Level: OK, Label: "git version", Message: ver, Explain: explainGitVersion}
	}
	return Check{
		Level:   Warn,
		Label:   "git version",
		Message: fmt.Sprintf("%s (upgrade recommended)", ver),
		Explain: explainGitVersion,
	}
}

const explainUserName = "Every commit records an author name. Without it git falls back to your OS username, which often looks unprofessional in shared repos."

func checkUserName() Check {
	val := gitConfig("user.name")
	if val == "" {
		return Check{
			Level:   Fail,
			Label:   "user.name",
			Message: "not configured",
			Fix:     `run: git config --global user.name "Your Name"`,
			Explain: explainUserName,
		}
	}
	return Check{Level: OK, Label: "user.name", Message: val, Explain: explainUserName}
}

const explainUserEmail = "Commits are linked to you via email on GitHub/GitLab. Use the same address as your forge account so contributions are credited correctly."

func checkUserEmail() Check {
	val := gitConfig("user.email")
	if val == "" {
		return Check{
			Level:   Fail,
			Label:   "user.email",
			Message: "not configured",
			Fix:     `run: git config --global user.email "you@example.com"`,
			Explain: explainUserEmail,
		}
	}
	if !strings.Contains(val, "@") {
		return Check{
			Level:   Warn,
			Label:   "user.email",
			Message: fmt.Sprintf("%s (does not look like a valid email)", val),
			Fix:     `run: git config --global user.email "you@example.com"`,
			Explain: explainUserEmail,
		}
	}
	return Check{Level: OK, Label: "user.email", Message: val, Explain: explainUserEmail}
}

const explainCredentialHelper = "Without a credential helper git asks for your password on every push/pull. A keychain helper stores it securely so you only authenticate once."

func checkCredentialHelper() Check {
	val := gitConfig("credential.helper")
	if val == "" {
		return Check{
			Level:   Warn,
			Label:   "credential.helper",
			Message: "not set",
			Fix:     "run: git config --global credential.helper osxkeychain  (macOS)",
			Explain: explainCredentialHelper,
		}
	}
	return Check{Level: OK, Label: "credential.helper", Message: val, Explain: explainCredentialHelper}
}

const explainDefaultBranch = "Sets the name of the first branch when you run 'git init'. GitHub and GitLab default to 'main'; mismatching this causes confusion when pushing a new repo for the first time."

func checkDefaultBranch() Check {
	val := gitConfig("init.defaultBranch")
	if val == "" || val == "master" {
		msg := "not set"
		if val == "master" {
			msg = `set to "master"`
		}
		return Check{
			Level:   Warn,
			Label:   "init.defaultBranch",
			Message: msg + " (recommended: main)",
			Fix:     "run: git config --global init.defaultBranch main",
			Explain: explainDefaultBranch,
		}
	}
	return Check{Level: OK, Label: "init.defaultBranch", Message: val, Explain: explainDefaultBranch}
}

const explainPullRebase = "'git pull' merges by default, which creates noisy merge commits on every sync. With pull.rebase=true it rebases instead, keeping history linear and easier to read."

func checkPullRebase() Check {
	val := gitConfig("pull.rebase")
	if val != "true" {
		msg := "not set"
		if val != "" {
			msg = fmt.Sprintf("%q (recommended: true)", val)
		}
		return Check{
			Level:   Warn,
			Label:   "pull.rebase",
			Message: msg,
			Fix:     "run: git config --global pull.rebase true",
			Explain: explainPullRebase,
		}
	}
	return Check{Level: OK, Label: "pull.rebase", Message: val, Explain: explainPullRebase}
}

const explainFetchPrune = "Automatically deletes local references to remote branches that were deleted on the server. Without this, 'git branch -r' fills up with ghosts of merged branches."

func checkFetchPrune() Check {
	val := gitConfig("fetch.prune")
	if val != "true" {
		msg := "not set"
		if val != "" {
			msg = fmt.Sprintf("%q (recommended: true)", val)
		}
		return Check{
			Level:   Warn,
			Label:   "fetch.prune",
			Message: msg,
			Fix:     "run: git config --global fetch.prune true",
			Explain: explainFetchPrune,
		}
	}
	return Check{Level: OK, Label: "fetch.prune", Message: val, Explain: explainFetchPrune}
}

const explainPushAutoSetupRemote = "Lets you run 'git push' on a new branch without having to pass '-u origin <branch>' every time. Introduced in git 2.37."

func checkPushAutoSetupRemote() Check {
	val := gitConfig("push.autoSetupRemote")
	if val != "true" {
		msg := "not set"
		if val != "" {
			msg = fmt.Sprintf("%q (recommended: true)", val)
		}
		return Check{
			Level:   Warn,
			Label:   "push.autoSetupRemote",
			Message: msg,
			Fix:     "run: git config --global push.autoSetupRemote true",
			Explain: explainPushAutoSetupRemote,
		}
	}
	return Check{Level: OK, Label: "push.autoSetupRemote", Message: val, Explain: explainPushAutoSetupRemote}
}

const explainRerereEnabled = "rerere (reuse recorded resolution) memorises how you resolved a merge conflict so git can replay the same fix automatically next time the same conflict appears."

func checkRerereEnabled() Check {
	val := gitConfig("rerere.enabled")
	if val != "true" {
		msg := "not set"
		if val != "" {
			msg = fmt.Sprintf("%q (recommended: true)", val)
		}
		return Check{
			Level:   Warn,
			Label:   "rerere.enabled",
			Message: msg,
			Fix:     "run: git config --global rerere.enabled true",
			Explain: explainRerereEnabled,
		}
	}
	return Check{Level: OK, Label: "rerere.enabled", Message: "true", Explain: explainRerereEnabled}
}

const explainEditor = "The editor git opens for commit messages, rebase todos, and tag annotations. Falling back to vi can surprise people who are not familiar with it."

func checkEditor() Check {
	if v := os.Getenv("VISUAL"); v != "" {
		return Check{Level: OK, Label: "core.editor", Message: "using " + v, Explain: explainEditor}
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return Check{Level: OK, Label: "core.editor", Message: "using " + v, Explain: explainEditor}
	}
	if v := gitConfig("core.editor"); v != "" {
		return Check{Level: OK, Label: "core.editor", Message: "using " + v, Explain: explainEditor}
	}
	return Check{
		Level:   Warn,
		Label:   "core.editor",
		Message: "using vi (set VISUAL, EDITOR, or core.editor)",
		Explain: explainEditor,
	}
}

const explainGlobalGitignore = "A global gitignore lets you ignore OS and editor noise (e.g. .DS_Store, .idea/, *.swp) across every repo without cluttering each project's .gitignore."

func checkGlobalGitignore() Check {
	excludesFile := gitConfig("core.excludesfile")
	if excludesFile != "" {
		// Expand leading ~
		if strings.HasPrefix(excludesFile, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				excludesFile = filepath.Join(home, excludesFile[2:])
			}
		}
		if _, err := os.Stat(excludesFile); err == nil {
			return Check{Level: OK, Label: "global gitignore", Message: excludesFile, Explain: explainGlobalGitignore}
		}
		return Check{
			Level:   Warn,
			Label:   "global gitignore",
			Message: fmt.Sprintf("configured (%s) but file does not exist", excludesFile),
			Fix:     "create ~/.config/git/ignore with common patterns like .DS_Store, .env, *.log",
			Explain: explainGlobalGitignore,
		}
	}

	// Not configured via git config - check default locations.
	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".config", "git", "ignore"),
			filepath.Join(home, ".gitignore_global"),
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return Check{Level: OK, Label: "global gitignore", Message: p, Explain: explainGlobalGitignore}
			}
		}
	}

	return Check{
		Level:   Warn,
		Label:   "global gitignore",
		Message: "not configured",
		Fix:     "create ~/.config/git/ignore with common patterns like .DS_Store, .env, *.log",
		Explain: explainGlobalGitignore,
	}
}

const explainGPGSigning = "Signing commits with GPG or SSH lets GitHub show a 'Verified' badge and proves the commit was made by you, not someone who pushed with your name."

func checkGPGSigning() Check {
	gpgsign := gitConfig("commit.gpgsign")
	if gpgsign != "true" {
		return Check{Level: OK, Label: "gpg signing", Message: "not enabled (optional)", Explain: explainGPGSigning}
	}
	signingKey := gitConfig("user.signingkey")
	if signingKey == "" {
		return Check{
			Level:   Warn,
			Label:   "gpg signing",
			Message: "commit.gpgsign=true but user.signingkey is not set",
			Fix:     "run: git config --global user.signingkey <your-key-id>",
			Explain: explainGPGSigning,
		}
	}
	return Check{Level: OK, Label: "gpg signing", Message: "enabled, key: " + signingKey, Explain: explainGPGSigning}
}

// runLocalChecks runs checks that require an active git repository.
func runLocalChecks() []Check {
	var checks []Check

	// 1. remote origin
	checks = append(checks, checkRemoteOrigin())

	// 2. upstream tracking
	checks = append(checks, checkUpstreamTracking())

	// 3. local .gitignore
	checks = append(checks, checkLocalGitignore())

	// 4. merge/rebase in progress
	checks = append(checks, checkMergeInProgress())

	// 5. uncommitted changes
	checks = append(checks, checkUncommittedChanges())

	// 6. stale remote branches
	checks = append(checks, checkStaleBranches())

	// 7. branch conventions
	checks = append(checks, checkBranchConventions())

	// 8. large repo
	checks = append(checks, checkLargeRepo())

	return checks
}

const explainRemoteOrigin = "The 'origin' remote is the conventional name for the canonical upstream repo (GitHub, GitLab, etc.). Without it, push/pull have nowhere to go."

func checkRemoteOrigin() Check {
	url, err := gitOutput("remote", "get-url", "origin")
	if err != nil || url == "" {
		return Check{
			Level:   Fail,
			Label:   "remote origin",
			Message: "not configured",
			Fix:     "run: git remote add origin <url>",
			Explain: explainRemoteOrigin,
		}
	}
	return Check{Level: OK, Label: "remote origin", Message: url, Explain: explainRemoteOrigin}
}

const explainUpstreamTracking = "When your local branch tracks a remote branch, 'git status' shows how many commits ahead/behind you are and 'git pull' knows where to fetch from."

func checkUpstreamTracking() Check {
	branch, _ := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	_, err := gitOutput("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		msg := "no upstream for current branch"
		if branch != "" {
			msg = fmt.Sprintf("no upstream for branch '%s'", branch)
		}
		fix := "run: git push -u origin <branch>"
		if branch != "" {
			fix = fmt.Sprintf("run: git push -u origin %s", branch)
		}
		return Check{
			Level:   Warn,
			Label:   "upstream tracking",
			Message: msg,
			Fix:     fix,
			Explain: explainUpstreamTracking,
		}
	}
	upstream, _ := gitOutput("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return Check{Level: OK, Label: "upstream tracking", Message: upstream, Explain: explainUpstreamTracking}
}

const explainLocalGitignore = "A project-level .gitignore keeps build artifacts, secrets, and editor files out of version control. Without it, accidental commits of sensitive files are easy to make."

func checkLocalGitignore() Check {
	if _, err := os.Stat(".gitignore"); os.IsNotExist(err) {
		return Check{
			Level:   Warn,
			Label:   ".gitignore",
			Message: "missing",
			Fix:     "create a .gitignore file for this project",
			Explain: explainLocalGitignore,
		}
	}
	return Check{Level: OK, Label: ".gitignore", Message: "present", Explain: explainLocalGitignore}
}

const explainMergeInProgress = "An interrupted merge, cherry-pick, or rebase leaves the repo in a partial state. You need to either resolve and continue, or abort, before starting other operations."

func checkMergeInProgress() Check {
	gitDir, err := gitOutput("rev-parse", "--git-dir")
	if err != nil || gitDir == "" {
		return Check{Level: OK, Label: "merge/rebase state", Message: "clean", Explain: explainMergeInProgress}
	}

	sentinels := []string{
		filepath.Join(gitDir, "MERGE_HEAD"),
		filepath.Join(gitDir, "CHERRY_PICK_HEAD"),
		filepath.Join(gitDir, "rebase-merge"),
	}
	for _, s := range sentinels {
		if _, err := os.Stat(s); err == nil {
			return Check{
				Level:   Warn,
				Label:   "merge/rebase state",
				Message: "merge/rebase in progress - complete or abort before continuing",
				Explain: explainMergeInProgress,
			}
		}
	}
	return Check{Level: OK, Label: "merge/rebase state", Message: "clean", Explain: explainMergeInProgress}
}

const explainUncommittedChanges = "Files modified or staged but not yet committed. Not a problem on its own, but worth knowing if you are about to switch branches or run an operation that rewrites history."

func checkUncommittedChanges() Check {
	out, err := gitOutput("status", "--porcelain")
	if err != nil {
		return Check{Level: OK, Label: "uncommitted changes", Message: "clean", Explain: explainUncommittedChanges}
	}
	if out == "" {
		return Check{Level: OK, Label: "uncommitted changes", Message: "clean", Explain: explainUncommittedChanges}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return Check{
		Level:   Warn,
		Label:   "uncommitted changes",
		Message: fmt.Sprintf("%d file(s) with uncommitted changes", count),
		Explain: explainUncommittedChanges,
	}
}

const explainStaleBranches = "Remote-tracking refs (origin/my-feature) that no longer exist on the server. They pile up over time and clutter branch listings; pruning removes them safely."

func checkStaleBranches() Check {
	out, err := gitOutput("remote", "prune", "origin", "--dry-run")
	if err != nil {
		// remote prune failing likely means no origin - already caught above
		return Check{Level: OK, Label: "stale remote branches", Message: "none", Explain: explainStaleBranches}
	}
	if strings.Contains(out, "would prune") || strings.Contains(out, "[would prune]") {
		lines := strings.Split(out, "\n")
		count := 0
		for _, l := range lines {
			if strings.Contains(l, "would prune") || strings.Contains(l, "[would prune]") {
				count++
			}
		}
		return Check{
			Level:   Warn,
			Label:   "stale remote branches",
			Message: fmt.Sprintf("%d stale ref(s) found", count),
			Fix:     "run: git remote prune origin",
			Explain: explainStaleBranches,
		}
	}
	return Check{Level: OK, Label: "stale remote branches", Message: "none", Explain: explainStaleBranches}
}

const explainBranchConventions = "Checks whether the current branch name matches the prefixes defined in .bonsai.toml (e.g. feat/, fix/). Consistent naming makes it easier to automate PRs, CI rules, and changelog generation."

func checkBranchConventions() Check {
	// Load .bonsai.toml if present; skip check if not found.
	var cfg config.Config
	if _, err := os.Stat(".bonsai.toml"); os.IsNotExist(err) {
		return Check{Level: OK, Label: "branch conventions", Message: "no .bonsai.toml (skipped)", Explain: explainBranchConventions}
	}

	cfg2, err := config.Load()
	if err != nil {
		return Check{Level: OK, Label: "branch conventions", Message: "could not load config (skipped)", Explain: explainBranchConventions}
	}
	cfg = *cfg2

	if len(cfg.Conventions.Branches) == 0 {
		return Check{Level: OK, Label: "branch conventions", Message: "none configured", Explain: explainBranchConventions}
	}
	if cfg.Conventions.Validation.Mode == "off" {
		return Check{Level: OK, Label: "branch conventions", Message: "validation off", Explain: explainBranchConventions}
	}

	branch, err := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || branch == "" {
		return Check{Level: OK, Label: "branch conventions", Message: "could not determine branch", Explain: explainBranchConventions}
	}

	result := conventions.Validate(branch, cfg.Conventions)
	if result.Special {
		return Check{Level: OK, Label: "branch conventions", Message: fmt.Sprintf("'%s' is a special branch", branch), Explain: explainBranchConventions}
	}
	if !result.Valid {
		prefixes := make([]string, 0, len(result.Rules))
		for _, r := range result.Rules {
			if r.Rule.Prefix != "" {
				prefixes = append(prefixes, r.Rule.Prefix)
			}
		}
		return Check{
			Level:   Warn,
			Label:   "branch conventions",
			Message: fmt.Sprintf("'%s' does not match any configured prefix (%s)", branch, strings.Join(prefixes, ", ")),
			Fix:     "rename branch to follow your project conventions",
			Explain: explainBranchConventions,
		}
	}
	return Check{Level: OK, Label: "branch conventions", Message: fmt.Sprintf("'%s' matches '%s'", branch, result.Match.Name), Explain: explainBranchConventions}
}

const explainLargeRepo = "Large pack sizes (> 100 MB) slow down clone, fetch, and CI. Common causes are accidentally committed binaries or build artifacts; git lfs or a cleanup rewrite can help."

func checkLargeRepo() Check {
	out, err := gitOutput("count-objects", "-v")
	if err != nil {
		return Check{Level: OK, Label: "repo size", Message: "could not determine", Explain: explainLargeRepo}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "size-pack:") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			kb, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				continue
			}
			if kb > 100000 {
				mb := kb / 1024
				return Check{
					Level:   Warn,
					Label:   "repo size",
					Message: fmt.Sprintf("pack size is %d MB (> 100 MB)", mb),
					Fix:     "consider git gc or git lfs for large files",
					Explain: explainLargeRepo,
				}
			}
			return Check{Level: OK, Label: "repo size", Message: fmt.Sprintf("%d KB packed", kb), Explain: explainLargeRepo}
		}
	}
	return Check{Level: OK, Label: "repo size", Message: "OK", Explain: explainLargeRepo}
}
