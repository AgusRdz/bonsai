package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// gitConfig reads a single git config key using git's full resolution order
// (system → global → local → includeIf), so conditional includes are honoured.
// Returns "" when the key is not set (exit code 1).
func gitConfig(key string) string {
	out, err := exec.Command("git", "config", "--get", key).Output()
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

	// 13. SSH key
	checks = append(checks, checkSSHKey())

	// 14. SSH agent
	checks = append(checks, checkSSHAgent())

	// 15. SSH connectivity (GitHub)
	checks = append(checks, checkSSHConnectivity())

	return checks
}

const explainGitVersion = "git 2.28+ introduced init.defaultBranch and other settings bonsai depends on; older versions may silently ignore them."

func isGitVersionSupported(ver string) bool {
	parts := strings.SplitN(ver, ".", 3)
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major > 2 || (major == 2 && minor >= 28)
}

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
	ver := strings.TrimPrefix(out, "git version ")
	if isGitVersionSupported(ver) {
		return Check{Level: OK, Label: "git version", Message: ver, Explain: explainGitVersion}
	}
	return Check{
		Level:   Warn,
		Label:   "git version",
		Message: fmt.Sprintf("%s (upgrade recommended)", ver),
		Explain: explainGitVersion,
	}
}

// authorIdent returns the (name, email) git will actually use for a commit,
// resolving env vars → local config → includeIf → global in the correct order.
func authorIdent() (name, email string) {
	// "git var GIT_AUTHOR_IDENT" returns "Name <email> timestamp tz"
	out, err := gitOutput("var", "GIT_AUTHOR_IDENT")
	if err != nil || out == "" {
		return gitConfig("user.name"), gitConfig("user.email")
	}
	// parse: everything before the last '<' is the name
	lt := strings.LastIndex(out, "<")
	gt := strings.LastIndex(out, ">")
	if lt < 0 || gt < 0 || gt <= lt {
		return gitConfig("user.name"), gitConfig("user.email")
	}
	return strings.TrimSpace(out[:lt]), out[lt+1 : gt]
}

const explainUserName = "Every commit records an author name. Without it git falls back to your OS username, which often looks unprofessional in shared repos."

func checkUserName() Check {
	name, _ := authorIdent()
	if name == "" {
		return Check{
			Level:   Fail,
			Label:   "user.name",
			Message: "not configured",
			Fix:     `run: git config --global user.name "Your Name"`,
			Explain: explainUserName,
		}
	}
	if env := os.Getenv("GIT_AUTHOR_NAME"); env != "" && env != gitConfig("user.name") {
		return Check{
			Level:   Warn,
			Label:   "user.name",
			Message: fmt.Sprintf("%s (overridden by GIT_AUTHOR_NAME env var)", name),
			Fix:     "unset GIT_AUTHOR_NAME or remove it from your shell profile",
			Explain: explainUserName,
		}
	}
	return Check{Level: OK, Label: "user.name", Message: name, Explain: explainUserName}
}

const explainUserEmail = "Commits are linked to you via email on GitHub/GitLab. Use the same address as your forge account so contributions are credited correctly."

func checkUserEmail() Check {
	_, email := authorIdent()
	if email == "" {
		return Check{
			Level:   Fail,
			Label:   "user.email",
			Message: "not configured",
			Fix:     `run: git config --global user.email "you@example.com"`,
			Explain: explainUserEmail,
		}
	}
	if !strings.Contains(email, "@") {
		return Check{
			Level:   Warn,
			Label:   "user.email",
			Message: fmt.Sprintf("%s (does not look like a valid email)", email),
			Fix:     `run: git config --global user.email "you@example.com"`,
			Explain: explainUserEmail,
		}
	}
	if env := os.Getenv("GIT_AUTHOR_EMAIL"); env != "" && env != gitConfig("user.email") {
		return Check{
			Level:   Warn,
			Label:   "user.email",
			Message: fmt.Sprintf("%s (overridden by GIT_AUTHOR_EMAIL env var)", email),
			Fix:     "unset GIT_AUTHOR_EMAIL or remove it from your shell profile",
			Explain: explainUserEmail,
		}
	}
	return Check{Level: OK, Label: "user.email", Message: email, Explain: explainUserEmail}
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
	fix := `run: git config --global core.editor "nano"`
	if runtime.GOOS == "windows" {
		fix = `run: git config --global core.editor "code --wait"`
	} else if runtime.GOOS == "darwin" {
		fix = `run: git config --global core.editor "nano"  # or: code --wait, vim, etc.`
	}
	return Check{
		Level:   Warn,
		Label:   "core.editor",
		Message: "using vi (set VISUAL, EDITOR, or core.editor)",
		Fix:     fix,
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

// ---------------------------------------------------------------------------
// SSH checks
// ---------------------------------------------------------------------------

const explainSSHKey = "An SSH key lets you authenticate with GitHub/GitLab without a password. Without one you must use HTTPS with a credential helper or a personal access token."

// SSHKeyInfo describes a found SSH key pair.
type SSHKeyInfo struct {
	PrivateKey string
	PublicKey  string
}

// FindSSHKey returns the first SSH key pair found in ~/.ssh, or nil.
func FindSSHKey() *SSHKeyInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	names := []string{"id_ed25519", "id_ecdsa", "id_rsa", "id_dsa"}
	for _, name := range names {
		priv := filepath.Join(home, ".ssh", name)
		pub := priv + ".pub"
		if _, err := os.Stat(priv); err == nil {
			return &SSHKeyInfo{PrivateKey: priv, PublicKey: pub}
		}
	}
	return nil
}

func checkSSHKey() Check {
	key := FindSSHKey()
	if key == nil {
		return Check{
			Level:   Warn,
			Label:   "ssh key",
			Message: "no SSH key found in ~/.ssh",
			Fix:     "run: bonsai ssh keygen",
			Explain: explainSSHKey,
		}
	}
	return Check{Level: OK, Label: "ssh key", Message: key.PrivateKey, Explain: explainSSHKey}
}

const explainSSHAgent = "ssh-agent holds your decrypted key in memory so you do not have to type your passphrase on every push. Without it, git prompts for your passphrase each time."

func checkSSHAgent() Check {
	if runtime.GOOS == "windows" {
		return checkSSHAgentWindows()
	}
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return Check{
			Level:   Warn,
			Label:   "ssh-agent",
			Message: "SSH_AUTH_SOCK not set - ssh-agent may not be running",
			Fix:     "run: eval $(ssh-agent -s) && ssh-add",
			Explain: explainSSHAgent,
		}
	}
	out, err := exec.Command("ssh-add", "-l").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "no identities") {
			return Check{
				Level:   Warn,
				Label:   "ssh-agent",
				Message: "agent running but no keys loaded",
				Fix:     "run: ssh-add",
				Explain: explainSSHAgent,
			}
		}
		return Check{
			Level:   Warn,
			Label:   "ssh-agent",
			Message: "agent socket found but not responding",
			Fix:     "run: eval $(ssh-agent -s) && ssh-add",
			Explain: explainSSHAgent,
		}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return Check{Level: OK, Label: "ssh-agent", Message: fmt.Sprintf("%d key(s) loaded", len(lines)), Explain: explainSSHAgent}
}

// checkSSHAgentWindows checks the OpenSSH Authentication Agent Windows service
// and gives targeted advice based on its actual state.
func checkSSHAgentWindows() Check {
	out, err := exec.Command("sc", "query", "ssh-agent").CombinedOutput()
	if err != nil {
		// Service not found — OpenSSH client may not be installed.
		return Check{
			Level:   Warn,
			Label:   "ssh-agent",
			Message: "OpenSSH Authentication Agent service not found",
			Fix:     "install via: Settings > Apps > Optional Features > Add a feature > OpenSSH Client",
			Explain: explainSSHAgent,
		}
	}
	status := strings.ToUpper(string(out))
	if strings.Contains(status, "RUNNING") {
		// Service is up — check whether ssh-add can reach it.
		addOut, addErr := exec.Command("ssh-add", "-l").CombinedOutput()
		if addErr != nil {
			msg := strings.TrimSpace(string(addOut))
			if strings.Contains(msg, "no identities") {
				return Check{
					Level:   Warn,
					Label:   "ssh-agent",
					Message: "agent running but no keys loaded",
					Fix:     "run: ssh-add",
					Explain: explainSSHAgent,
				}
			}
		}
		lines := strings.Split(strings.TrimSpace(string(addOut)), "\n")
		return Check{Level: OK, Label: "ssh-agent", Message: fmt.Sprintf("%d key(s) loaded", len(lines)), Explain: explainSSHAgent}
	}
	// Service exists but is not running — needs an admin PowerShell once.
	return Check{
		Level:   Warn,
		Label:   "ssh-agent",
		Message: "OpenSSH Authentication Agent service is not running",
		Fix:     "in an elevated PowerShell: Set-Service ssh-agent -StartupType Automatic; Start-Service ssh-agent\nthen run: ssh-add",
		Explain: explainSSHAgent,
	}
}

// ParseSSHHost extracts the hostname from a git remote URL when it uses the
// SSH protocol. Returns "" for HTTPS or local paths.
//
// Handles both common formats:
//
//	git@github.com:user/repo.git        -> github.com
//	ssh://git@github.com/user/repo.git  -> github.com
func ParseSSHHost(rawURL string) string {
	if strings.HasPrefix(rawURL, "ssh://") {
		rest := strings.TrimPrefix(rawURL, "ssh://")
		if at := strings.Index(rest, "@"); at != -1 {
			rest = rest[at+1:]
		}
		if slash := strings.Index(rest, "/"); slash != -1 {
			rest = rest[:slash]
		}
		if colon := strings.Index(rest, ":"); colon != -1 {
			rest = rest[:colon]
		}
		return rest
	}
	// SCP-like: user@host:path  (must not contain "/" before the colon)
	if at := strings.Index(rawURL, "@"); at != -1 {
		rest := rawURL[at+1:]
		if colon := strings.Index(rest, ":"); colon != -1 {
			host := rest[:colon]
			if !strings.Contains(host, "/") {
				return host
			}
		}
	}
	return ""
}

// DetectSSHHosts returns the unique SSH hostnames found in the current repo's
// remotes. Returns nil when not in a git repo or when no remote uses SSH.
func DetectSSHHosts() []string {
	out, err := exec.Command("git", "remote", "-v").Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var hosts []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if h := ParseSSHHost(fields[1]); h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// TestSSHHost attempts SSH authentication against host and reports whether it
// succeeded. msg is the first line of output from the server.
func TestSSHHost(host string) (ok bool, msg string) {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-T", "git@"+host,
	)
	out, _ := cmd.CombinedOutput()
	raw := strings.TrimSpace(string(out))
	first := raw
	if idx := strings.Index(raw, "\n"); idx != -1 {
		first = raw[:idx]
	}
	lower := strings.ToLower(raw)
	ok = strings.Contains(lower, "successfully authenticated") || // GitHub, Gitea, Forgejo
		strings.Contains(lower, "hi ") || // GitHub
		strings.Contains(lower, "welcome to gitlab") || // GitLab
		strings.Contains(lower, "logged in as") || // Bitbucket
		strings.Contains(lower, "authenticated as") // Bitbucket
	if first == "" {
		first = "connection failed or timed out"
	}
	return ok, first
}

// SSHKeyURL returns the settings URL for adding SSH keys on known providers.
// For unknown hosts it returns a generic hint.
func SSHKeyURL(host string) string {
	switch host {
	case "github.com":
		return "github.com/settings/keys"
	case "gitlab.com":
		return "gitlab.com/-/profile/keys"
	case "bitbucket.org":
		return "bitbucket.org/account/settings/ssh-keys/"
	case "ssh.dev.azure.com":
		return "dev.azure.com/<org>/_usersSettings/keys"
	default:
		return host + " (check your forge's SSH key settings)"
	}
}

const explainSSHConnectivity = "Verifies the git remote host is reachable via SSH. A failure usually means a firewall blocks port 22, the key is not added to the server, or the agent has no loaded keys."

func checkSSHConnectivity() Check {
	hosts := DetectSSHHosts()
	if len(hosts) == 0 {
		hosts = []string{"github.com"}
	}
	host := hosts[0]
	ok, msg := TestSSHHost(host)
	label := "ssh " + host
	if ok {
		return Check{Level: OK, Label: label, Message: msg, Explain: explainSSHConnectivity}
	}
	return Check{
		Level:   Warn,
		Label:   label,
		Message: msg,
		Fix:     "ensure your SSH public key is added at " + SSHKeyURL(host),
		Explain: explainSSHConnectivity,
	}
}

const explainLargeRepo = "Large pack sizes (> 100 MB) slow down clone, fetch, and CI. Common causes are accidentally committed binaries or build artifacts; git lfs or a cleanup rewrite can help."

func parseSizePack(output string) (int64, bool) {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "size-pack:") {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			kb, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				continue
			}
			return kb, true
		}
	}
	return 0, false
}

func checkLargeRepo() Check {
	out, err := gitOutput("count-objects", "-v")
	if err != nil {
		return Check{Level: OK, Label: "repo size", Message: "could not determine", Explain: explainLargeRepo}
	}
	kb, ok := parseSizePack(out)
	if !ok {
		return Check{Level: OK, Label: "repo size", Message: "OK", Explain: explainLargeRepo}
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
