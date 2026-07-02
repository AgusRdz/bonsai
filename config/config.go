package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// GlobalConfigPath returns the path to the global config file.
// It is exported so CLI commands can open it directly.
func GlobalConfigPath() (string, error) {
	return globalConfigPath()
}

type Config struct {
	Flow        FlowConfig        `toml:"flow"`
	Conventions ConventionsConfig `toml:"conventions"`
	Modes       ModesConfig       `toml:"modes"`
	Education   EducationConfig   `toml:"education"`
	Keybindings KeybindingsConfig `toml:"keybindings"`
	Metrics     MetricsConfig     `toml:"metrics"`
	Editor      EditorConfig      `toml:"editor"`
	CommandBar  CommandBarConfig  `toml:"command_bar"`
	Signing     SigningConfig     `toml:"signing"`
	Dashboard   DashboardConfig   `toml:"dashboard"`
	Agent       AgentConfig       `toml:"agent"`
	Overview    OverviewConfig    `toml:"overview"`
	PR          PRConfig          `toml:"pr"`
	Worktree    WorktreeConfig    `toml:"worktree"`
}

// WorktreeConfig controls post-create behaviour for git worktrees.
type WorktreeConfig struct {
	// PostCreate lists shell commands run after every worktree is created.
	// nil = not yet configured (bonsai will prompt on first use).
	// empty slice = explicitly disabled.
	// $BONSAI_MAIN_WORKTREE is replaced with the main worktree path at runtime.
	PostCreate *[]string `toml:"post_create"`
}

// SaveProjectWorktree writes the post_create command list to the [worktree]
// section of .bonsai.toml in dir, preserving every OTHER section already in the
// file. It decodes into a generic map rather than a worktree-only struct on
// purpose: a struct decode would silently drop unrelated sections like [flow]
// or [conventions] on re-encode.
func SaveProjectWorktree(dir string, cmds []string) error {
	path := filepath.Join(dir, ".bonsai.toml")
	existing := map[string]any{}
	// Don't clobber a file we can't read: a corrupt or hand-broken .bonsai.toml
	// should surface an error, not be silently overwritten.
	if _, err := toml.DecodeFile(path, &existing); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing %s: %w", path, err)
	}
	existing["worktree"] = WorktreeConfig{PostCreate: &cmds}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(existing); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// OverviewConfig controls the clean-tree overview panel.
type OverviewConfig struct {
	// Enabled shows open PRs and recent commits when the working tree is clean.
	// Defaults to true when a PR provider is configured.
	Enabled *bool `toml:"enabled"`
}

// PRConfig holds pull-request workflow preferences.
type PRConfig struct {
	// MergeMethod sets the default merge strategy: "merge", "squash", or "rebase".
	// When set, the merge picker is skipped and this method is used directly.
	// Leave empty to always show the picker.
	MergeMethod string `toml:"merge_method"`
}

// AgentConfig controls structured JSON output for AI agent consumption.
type AgentConfig struct {
	DefaultFormat string `toml:"default_format"` // "json" or "" (plain text)
}

// DashboardConfig lists repositories shown in the multi-repo dashboard ([D] key).
type DashboardConfig struct {
	Repos []string `toml:"repos"` // absolute or ~-prefixed paths
}

// CommandBarConfig controls which shortcuts appear in the main command bar.
// When Items is empty the default set is used.
type CommandBarConfig struct {
	Items []string `toml:"items"`
}

type EditorConfig struct {
	// Command is the editor binary (e.g. "vim", "nano", "code --wait").
	// When empty, bonsai falls back to $VISUAL, then $EDITOR, then "vi".
	Command string `toml:"command"`
}

// SigningConfig controls GPG/SSH commit signing.
type SigningConfig struct {
	Enabled bool   `toml:"enabled"`
	Key     string `toml:"key"` // GPG key ID or SSH key path; empty = git default
}

type FlowConfig struct {
	Type string `toml:"type"`
}

type ConventionsConfig struct {
	Branches   map[string]BranchRule `toml:"branches"`
	Validation ValidationConfig      `toml:"validation"`
}

type BranchRule struct {
	Prefix  string `toml:"prefix"`
	Pattern string `toml:"pattern"`
	Example string `toml:"example"`
}

type ValidationConfig struct {
	Mode string `toml:"mode"`
}

type ModesConfig struct {
	Default string `toml:"default"`
}

type EducationConfig struct {
	PanelDuration int `toml:"panel_duration"`
}

type KeybindingsConfig struct {
	Graph  string `toml:"graph"`
	Commit string `toml:"commit"`
	Branch string `toml:"branch"`
	Push   string `toml:"push"`
	Pull   string `toml:"pull"`
	Stash  string `toml:"stash"`
	Undo   string `toml:"undo"`
	Quit   string `toml:"quit"`
}

type MetricsConfig struct {
	Enabled bool        `toml:"enabled"`
	Track   TrackConfig `toml:"track"`
}

type TrackConfig struct {
	Errors      bool `toml:"errors"`
	Conventions bool `toml:"conventions"`
	Commits     bool `toml:"commits"`
	Habits      bool `toml:"habits"`
}

// UsageFilePath returns the path to the command usage tracking file.
func UsageFilePath() (string, error) {
	p, err := globalConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "usage.json"), nil
}

// GlobalExists reports whether the global config file already exists.
// On Windows it also runs the one-time migration from AppData\Roaming so
// that an existing config is found before the first-run setup wizard fires.
func GlobalExists() (bool, error) {
	p, err := globalConfigPath()
	if err != nil {
		return false, err
	}
	migrateRoamingConfig(p)
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// Write serialises cfg and writes it to path, creating parent directories as
// needed. It overwrites any existing file.
func Write(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

// Load reads the global config (creating it with defaults on first run) and
// merges any per-project .bonsai.toml found in the current directory.
func Load() (*Config, error) {
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, err
	}

	migrateRoamingConfig(globalPath)

	cfg := defaults()

	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		if err := writeDefaults(globalPath, &cfg); err != nil {
			return nil, fmt.Errorf("config: create %s: %w", globalPath, err)
		}
	} else if err == nil {
		if _, err := toml.DecodeFile(globalPath, &cfg); err != nil {
			return nil, fmt.Errorf("config: %s: %w", globalPath, err)
		}
	}

	if _, err := toml.DecodeFile(".bonsai.toml", &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("config: .bonsai.toml: %w", err)
	}

	// Migrate mode names from older releases.
	switch cfg.Modes.Default {
	case "novice":
		cfg.Modes.Default = "guided"
	case "learning":
		cfg.Modes.Default = "standard"
	}

	// Keybindings written by older setup runs may be empty strings; fill in
	// defaults for any that are missing so the TUI always has valid bindings.
	applyKeybindingDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DefaultKeybindings returns the default keybinding set.
func DefaultKeybindings() KeybindingsConfig {
	return defaults().Keybindings
}

// LoadFile decodes a single config file at the given path without merging
// any other files. Returns nil if the file does not exist.
func LoadFile(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	cfg := defaults()
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: %s: %w", path, err)
	}
	return &cfg, nil
}

func applyKeybindingDefaults(cfg *Config) {
	d := defaults().Keybindings
	kb := &cfg.Keybindings
	if kb.Graph == "" {
		kb.Graph = d.Graph
	}
	if kb.Commit == "" {
		kb.Commit = d.Commit
	}
	if kb.Branch == "" {
		kb.Branch = d.Branch
	}
	if kb.Push == "" {
		kb.Push = d.Push
	}
	if kb.Pull == "" {
		kb.Pull = d.Pull
	}
	if kb.Stash == "" {
		kb.Stash = d.Stash
	}
	if kb.Undo == "" {
		kb.Undo = d.Undo
	}
	if kb.Quit == "" {
		kb.Quit = d.Quit
	}
}

func validate(cfg *Config) error {
	validModes := map[string]bool{"guided": true, "standard": true, "pro": true}
	if !validModes[cfg.Modes.Default] {
		return fmt.Errorf("config: modes.default must be guided, standard, or pro (got %q)", cfg.Modes.Default)
	}
	validFlow := map[string]bool{"auto": true, "gitflow": true, "trunk": true, "githubflow": true, "forking": true}
	if !validFlow[cfg.Flow.Type] {
		return fmt.Errorf("config: flow.type must be auto, gitflow, trunk, githubflow, or forking (got %q)", cfg.Flow.Type)
	}
	validValidation := map[string]bool{"strict": true, "warn": true, "off": true}
	if !validValidation[cfg.Conventions.Validation.Mode] {
		return fmt.Errorf("config: conventions.validation.mode must be strict, warn, or off (got %q)", cfg.Conventions.Validation.Mode)
	}
	validMerge := map[string]bool{"": true, "merge": true, "squash": true, "rebase": true}
	if !validMerge[cfg.PR.MergeMethod] {
		return fmt.Errorf("config: pr.merge_method must be merge, squash, or rebase (got %q)", cfg.PR.MergeMethod)
	}
	return nil
}

// OverviewEnabled returns true when the clean-tree overview should be shown.
// Defaults to true (opt-out).
func OverviewEnabled(cfg *Config) bool {
	if cfg == nil || cfg.Overview.Enabled == nil {
		return true
	}
	return *cfg.Overview.Enabled
}

// ResolveEditor returns the editor command to use, checking cfg.Editor.Command,
// then $VISUAL, then $EDITOR, then falling back to "vi".
func ResolveEditor(cfg *Config) string {
	if cfg != nil && cfg.Editor.Command != "" {
		return cfg.Editor.Command
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	return "vi"
}

// WriteLocalTemplate writes a commented .bonsai.toml template to path.
// It does not overwrite an existing file.
func WriteLocalTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}
	const tmpl = `# bonsai per-project configuration
# All fields are optional. Values here override the global config.
# Global config: ~/.config/bonsai/config.toml

# [modes]
# default = "standard"  # standard | guided | pro

# [flow]
# type = "auto"        # auto | gitflow | trunk | githubflow | forking

# [conventions.branches.feature]
# prefix  = "feat/"
# pattern = "feat/{ticket-id}-{description}"
# example = "feat/PROJ-123-login-oauth"

# [conventions.branches.bugfix]
# prefix  = "fix/"

# [conventions.validation]
# mode = "strict"      # strict | warn | off

# [education]
# panel_duration = 4   # seconds; 0 disables the panel

# [editor]
# command = ""         # e.g. "vim", "nano", "code --wait"

# [command_bar]
# items = ["space", "hunks", "diff", "commit", "push", "pull", "branch", "log"]
# Available: space hunks diff commit push pull branch log amend fetch stash graph
#            reset restore reflog tags bisect rebase worktrees remotes submodules
#            notes clean abort config

# [agent]
# default_format = "json"  # emit JSON from status/log/diff/show/review commands
`
	return os.WriteFile(path, []byte(tmpl), 0o644)
}

func globalConfigPath() (string, error) {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", fmt.Errorf("%%LOCALAPPDATA%% is not set")
		}
	} else {
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, "bonsai", "config.toml"), nil
}

// roamingConfigPath returns the legacy Windows config path (AppData\Roaming)
// used before v0.80.1. Only meaningful on Windows.
func roamingConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "bonsai", "config.toml")
}

// migrateRoamingConfig moves bonsai data files (config.toml and usage.json)
// from the legacy AppData\Roaming location to AppData\Local when upgrading
// from v0.80.1 and earlier. It is a no-op on non-Windows platforms and when
// the destination config already exists (user has an intentional config there).
func migrateRoamingConfig(localPath string) {
	if runtime.GOOS != "windows" {
		return
	}
	oldConfigPath := roamingConfigPath()
	if oldConfigPath == "" {
		return
	}
	// Nothing to migrate if the old config doesn't exist.
	if _, err := os.Stat(oldConfigPath); err != nil {
		return
	}
	// Don't overwrite an already-present Local config.
	if _, err := os.Stat(localPath); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: migration: could not create config dir: %v\n", err)
		return
	}
	if err := moveFile(oldConfigPath, localPath); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: migration: could not move config.toml: %v\n", err)
		return
	}
	// Also migrate usage.json so command-frequency history is preserved.
	oldUsagePath := filepath.Join(filepath.Dir(oldConfigPath), "usage.json")
	newUsagePath := filepath.Join(filepath.Dir(localPath), "usage.json")
	if _, err := os.Stat(oldUsagePath); err == nil {
		if _, err := os.Stat(newUsagePath); err != nil {
			if err := moveFile(oldUsagePath, newUsagePath); err != nil {
				fmt.Fprintf(os.Stderr, "bonsai: migration: could not move usage.json: %v\n", err)
			}
		}
	}
	// Best-effort cleanup of the old directory (only removes if empty).
	_ = os.Remove(filepath.Dir(oldConfigPath))
}

// moveFile moves src to dst, falling back to copy+delete if rename fails
// (e.g. src and dst are on different volumes).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

func writeDefaults(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func defaults() Config {
	return Config{
		Flow: FlowConfig{Type: "auto"},
		Conventions: ConventionsConfig{
			Branches: map[string]BranchRule{
				"feature": {Prefix: "feat/", Pattern: "feat/{ticket-id}-{description}", Example: "feat/PROJ-123-login-oauth"},
				"bugfix":  {Prefix: "fix/", Pattern: "fix/{ticket-id}-{description}", Example: "fix/PROJ-456-crash-on-login"},
				"release": {Prefix: "release/"},
				"hotfix":  {Prefix: "hotfix/"},
			},
			Validation: ValidationConfig{Mode: "strict"},
		},
		Modes:     ModesConfig{Default: "standard"},
		Education: EducationConfig{PanelDuration: 4},
		Keybindings: KeybindingsConfig{
			Graph:  "g",
			Commit: "c",
			Branch: "b",
			Push:   "p",
			Pull:   "l",
			Stash:  "s",
			Undo:   "z",
			Quit:   "q",
		},
		Metrics: MetricsConfig{
			Enabled: false,
			Track: TrackConfig{
				Errors:      true,
				Conventions: true,
				Commits:     false,
				Habits:      false,
			},
		},
	}
}
