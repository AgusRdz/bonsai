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

type Config struct {
	Flow        FlowConfig        `toml:"flow"`
	Conventions ConventionsConfig `toml:"conventions"`
	Modes       ModesConfig       `toml:"modes"`
	Education   EducationConfig   `toml:"education"`
	Keybindings KeybindingsConfig `toml:"keybindings"`
	Metrics     MetricsConfig     `toml:"metrics"`
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

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	validModes := map[string]bool{"novice": true, "pro": true, "learning": true}
	if !validModes[cfg.Modes.Default] {
		return fmt.Errorf("config: modes.default must be novice, pro, or learning (got %q)", cfg.Modes.Default)
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
		Modes:     ModesConfig{Default: "pro"},
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
