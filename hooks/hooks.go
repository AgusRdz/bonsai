package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	dispatcherDir = ".git/bonsai-hooks"
	sharedDir     = ".githooks"

	// disabledSuffix marks a disabled hook. Git only runs hooks whose file name
	// matches exactly, so renaming works on every OS — unlike clearing the
	// executable bit, which Windows and Git Bash ignore.
	disabledSuffix = ".disabled"
)

// Scope identifies which hooks directory an operation targets.
type Scope int

const (
	ScopeGlobal Scope = iota // ~/.config/git/hooks — personal, all repos
	ScopeShared              // .githooks/ — committed, team-wide
	ScopeLocal               // .git/hooks/ — personal, this repo only
)

// HookEntry describes a single hook file found during List.
type HookEntry struct {
	Name   string
	Scope  Scope
	Path   string
	Active bool // not renamed to <name>.disabled and executable bit is set (always true on Windows)
}

// AllHookNames is the standard set of git hook names.
var AllHookNames = []string{
	"applypatch-msg",
	"commit-msg",
	"fsmonitor-watchman",
	"post-commit",
	"post-merge",
	"post-update",
	"pre-applypatch",
	"pre-commit",
	"pre-merge-commit",
	"pre-push",
	"pre-rebase",
	"pre-receive",
	"prepare-commit-msg",
	"push-to-checkout",
	"update",
}

// ScopeLabel returns a human-readable name for the scope.
func ScopeLabel(s Scope) string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeShared:
		return "shared"
	case ScopeLocal:
		return "local"
	}
	return "unknown"
}

// GlobalDir returns the global personal hooks directory (~/.config/git/hooks).
func GlobalDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "git", "hooks"), nil
}

// Dir returns the hooks directory for the given scope.
func Dir(scope Scope) (string, error) {
	switch scope {
	case ScopeGlobal:
		return GlobalDir()
	case ScopeShared:
		return sharedDir, nil
	case ScopeLocal:
		return ".git/hooks", nil
	}
	return "", fmt.Errorf("unknown scope")
}

// IsInstalled reports whether the bonsai dispatcher is active in the current repo.
func IsInstalled() bool {
	if _, err := os.Stat(dispatcherDir); err != nil {
		return false
	}
	out, err := exec.Command("git", "config", "--local", "--get", "core.hooksPath").Output()
	if err != nil {
		return false
	}
	p := filepath.ToSlash(strings.TrimSpace(string(out)))
	return p == filepath.ToSlash(dispatcherDir)
}

// Install wires up the bonsai dispatcher in the current repo.
// Creates .git/bonsai-hooks/, generates all dispatcher scripts, sets
// core.hooksPath locally, and creates .githooks/ if missing.
func Install() error {
	if _, err := os.Stat(".git"); err != nil {
		return fmt.Errorf("not a git repository (no .git/ found)")
	}
	if err := os.MkdirAll(dispatcherDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", dispatcherDir, err)
	}
	for _, name := range AllHookNames {
		if err := writeDispatcher(name); err != nil {
			return fmt.Errorf("write dispatcher %s: %w", name, err)
		}
	}
	if err := exec.Command("git", "config", "--local", "core.hooksPath", dispatcherDir).Run(); err != nil {
		return fmt.Errorf("set core.hooksPath: %w", err)
	}
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", sharedDir, err)
	}
	return nil
}

// InstallGlobal creates ~/.config/git/hooks and sets core.hooksPath globally.
func InstallGlobal() error {
	dir, err := GlobalDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := exec.Command("git", "config", "--global", "core.hooksPath", dir).Run(); err != nil {
		return fmt.Errorf("set global core.hooksPath: %w", err)
	}
	return nil
}

// Rebuild regenerates all dispatcher scripts. Call after adding or removing hooks
// to keep the dispatcher in sync with installed hook names.
func Rebuild() error {
	if !IsInstalled() {
		return fmt.Errorf("bonsai hooks not installed — run: bonsai hooks --install")
	}
	for _, name := range AllHookNames {
		if err := writeDispatcher(name); err != nil {
			return fmt.Errorf("write dispatcher %s: %w", name, err)
		}
	}
	return nil
}

// List returns all hook entries found across all three scopes plus whether the
// dispatcher is installed in the current repo.
func List() (global, shared, local []HookEntry, installed bool, err error) {
	installed = IsInstalled()

	globalDir, gerr := GlobalDir()
	if gerr != nil {
		return nil, nil, nil, installed, gerr
	}

	global = scanDir(globalDir, ScopeGlobal)
	shared = scanDir(sharedDir, ScopeShared)
	local = scanDir(".git/hooks", ScopeLocal)
	return global, shared, local, installed, nil
}

func scanDir(dir string, scope Scope) []HookEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []HookEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sample") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		hookName, disabled := strings.CutSuffix(name, disabledSuffix)
		result = append(result, HookEntry{
			Name:   hookName,
			Scope:  scope,
			Path:   filepath.Join(dir, name),
			Active: !disabled && isExecutable(info),
		})
	}
	return result
}

func isExecutable(info os.FileInfo) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0111 != 0
}

// Add writes a hook script for the given scope.
// Returns an error if the hook already exists unless overwrite is true.
func Add(scope Scope, name, content string, overwrite bool) error {
	dir, err := Dir(scope)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if !overwrite {
		if existing, err := resolve(dir, name); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", existing)
		}
	}
	// Drop any disabled copy so the overwrite doesn't leave two versions behind.
	if err := os.Remove(filepath.Join(dir, name+disabledSuffix)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0755)
}

// Remove deletes a hook for the given scope.
func Remove(scope Scope, name string) error {
	dir, err := Dir(scope)
	if err != nil {
		return err
	}
	path, err := resolve(dir, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// Enable restores a disabled hook and makes it executable. The chmod also
// re-enables hooks disabled by older bonsai versions, which cleared the
// executable bit instead of renaming.
func Enable(scope Scope, name string) error {
	dir, err := Dir(scope)
	if err != nil {
		return err
	}
	path, err := resolve(dir, name)
	if err != nil {
		return err
	}
	active := filepath.Join(dir, name)
	if path != active {
		if err := os.Rename(path, active); err != nil {
			return fmt.Errorf("enable %s: %w", name, err)
		}
	}
	return os.Chmod(active, 0755)
}

// Disable renames a hook to <name>.disabled so git skips it without deleting it.
func Disable(scope Scope, name string) error {
	dir, err := Dir(scope)
	if err != nil {
		return err
	}
	path, err := resolve(dir, name)
	if err != nil {
		return err
	}
	disabled := filepath.Join(dir, name+disabledSuffix)
	if path == disabled {
		return nil
	}
	if err := os.Rename(path, disabled); err != nil {
		return fmt.Errorf("disable %s: %w", name, err)
	}
	return nil
}

// Path returns the file backing a hook, enabled or disabled. When the hook
// doesn't exist it returns the path a new hook would be created at.
func Path(scope Scope, name string) (string, error) {
	dir, err := Dir(scope)
	if err != nil {
		return "", err
	}
	if path, err := resolve(dir, name); err == nil {
		return path, nil
	}
	return filepath.Join(dir, name), nil
}

// resolve finds the file for a hook, preferring the enabled name over the
// disabled one.
func resolve(dir, name string) (string, error) {
	for _, candidate := range []string{name, name + disabledSuffix} {
		path := filepath.Join(dir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("hook %s not found in %s", name, dir)
}

// Show returns the content of a hook.
func Show(scope Scope, name string) (string, error) {
	dir, err := Dir(scope)
	if err != nil {
		return "", err
	}
	path, err := resolve(dir, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DefaultScript returns a minimal placeholder script for a hook.
func DefaultScript(name string) string {
	return fmt.Sprintf("#!/bin/sh\n# %s hook — add your commands here\n", name)
}

func writeDispatcher(name string) error {
	path := filepath.Join(dispatcherDir, name)
	return os.WriteFile(path, []byte(dispatcherScript(name)), 0755)
}

// dispatcherScript returns the shell script that chains all three hook scopes.
// Order: global personal → shared team → local personal.
// A non-zero exit from any scope aborts the chain and propagates the exit code.
func dispatcherScript(name string) string {
	return fmt.Sprintf(`#!/bin/sh
# bonsai dispatcher — do not edit manually
# regenerate with: bonsai hooks --install
HOOK=%q

_bonsai_run() {
  _bonsai_f="$1"
  shift
  [ -f "$_bonsai_f" ] && [ -x "$_bonsai_f" ] || return 0
  "$_bonsai_f" "$@"
}

_bonsai_run "${HOME}/.config/git/hooks/${HOOK}" "$@" || exit $?
_bonsai_run ".githooks/${HOOK}" "$@" || exit $?
_bonsai_run ".git/hooks/${HOOK}" "$@" || exit $?
`, name)
}
