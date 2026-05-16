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
func GlobalExists() (bool, error) {
	p, err := globalConfigPath()
	if err != nil {
		return false, err
	}
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
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// Load reads the global config (creating it with defaults on first run) and
// merges any per-project .bonsai.toml found in the current directory.
func Load() (*Config, error) {
	globalPath, err := globalConfigPath()
	if err != nil {
		return nil, err
	}

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
	return nil
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
`
	return os.WriteFile(path, []byte(tmpl), 0o644)
}

func globalConfigPath() (string, error) {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", fmt.Errorf("%%APPDATA%% is not set")
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

func writeDefaults(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
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
