package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := defaults()

	if cfg.Flow.Type != "auto" {
		t.Errorf("flow.type = %q, want auto", cfg.Flow.Type)
	}
	if cfg.Modes.Default != "standard" {
		t.Errorf("modes.default = %q, want standard", cfg.Modes.Default)
	}
	if cfg.Education.PanelDuration != 4 {
		t.Errorf("education.panel_duration = %d, want 4", cfg.Education.PanelDuration)
	}
	if cfg.Keybindings.Quit != "q" {
		t.Errorf("keybindings.quit = %q, want q", cfg.Keybindings.Quit)
	}
	if cfg.Metrics.Enabled {
		t.Error("metrics.enabled should default to false")
	}
	if !cfg.Metrics.Track.Errors {
		t.Error("metrics.track.errors should default to true")
	}
	if len(cfg.Conventions.Branches) == 0 {
		t.Error("conventions.branches should not be empty")
	}
}

func TestGlobalConfigPath(t *testing.T) {
	path, err := globalConfigPath()
	if err != nil {
		t.Fatalf("globalConfigPath: %v", err)
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("file name = %q, want config.toml", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != "bonsai" {
		t.Errorf("parent dir = %q, want bonsai", filepath.Base(filepath.Dir(path)))
	}
	if runtime.GOOS != "windows" {
		home, _ := os.UserHomeDir()
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		want := filepath.Join(xdg, "bonsai", "config.toml")
		if path != want {
			t.Errorf("path = %q, want %q", path, want)
		}
	}
}

func TestLoadCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Modes.Default != "standard" {
		t.Errorf("modes.default = %q, want standard", cfg.Modes.Default)
	}

	configPath := filepath.Join(dir, "bonsai", "config.toml")
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestLoadMergesProjectConfig(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", globalDir)
	}

	projectDir := t.TempDir()
	// Write a project config using the old mode name to verify migration.
	if err := os.WriteFile(filepath.Join(projectDir, ".bonsai.toml"), []byte(`
[modes]
default = "novice"
`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig) //nolint:errcheck

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// "novice" is migrated to "guided" automatically.
	if cfg.Modes.Default != "guided" {
		t.Errorf("modes.default = %q, want guided (migrated from novice)", cfg.Modes.Default)
	}
}

func TestValidateAcceptsDefaults(t *testing.T) {
	cfg := defaults()
	if err := validate(&cfg); err != nil {
		t.Errorf("defaults should pass validation: %v", err)
	}
}

func TestValidateRejectsInvalidMode(t *testing.T) {
	cfg := defaults()
	cfg.Modes.Default = "superuser"
	if err := validate(&cfg); err == nil {
		t.Error("expected error for invalid mode, got nil")
	}
}

func TestValidateRejectsInvalidFlow(t *testing.T) {
	cfg := defaults()
	cfg.Flow.Type = "svn"
	if err := validate(&cfg); err == nil {
		t.Error("expected error for invalid flow type, got nil")
	}
}

func TestValidateRejectsInvalidValidationMode(t *testing.T) {
	cfg := defaults()
	cfg.Conventions.Validation.Mode = "loud"
	if err := validate(&cfg); err == nil {
		t.Error("expected error for invalid validation mode, got nil")
	}
}

func TestResolveEditor(t *testing.T) {
	t.Run("from config", func(t *testing.T) {
		cfg := &Config{Editor: EditorConfig{Command: "code"}}
		if got := ResolveEditor(cfg); got != "code" {
			t.Errorf("ResolveEditor(cfg.code) = %q, want code", got)
		}
	})
	t.Run("from VISUAL", func(t *testing.T) {
		t.Setenv("VISUAL", "nano")
		t.Setenv("EDITOR", "vim")
		if got := ResolveEditor(&Config{}); got != "nano" {
			t.Errorf("ResolveEditor(VISUAL=nano) = %q, want nano", got)
		}
	})
	t.Run("from EDITOR", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "emacs")
		if got := ResolveEditor(&Config{}); got != "emacs" {
			t.Errorf("ResolveEditor(EDITOR=emacs) = %q, want emacs", got)
		}
	})
	t.Run("fallback vi", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		if got := ResolveEditor(&Config{}); got != "vi" {
			t.Errorf("ResolveEditor(fallback) = %q, want vi", got)
		}
	})
	t.Run("nil config", func(t *testing.T) {
		t.Setenv("VISUAL", "")
		t.Setenv("EDITOR", "")
		if got := ResolveEditor(nil); got != "vi" {
			t.Errorf("ResolveEditor(nil) = %q, want vi", got)
		}
	})
}

func TestApplyKeybindingDefaults(t *testing.T) {
	// all empty -> gets all defaults
	cfg := &Config{}
	applyKeybindingDefaults(cfg)
	if cfg.Keybindings.Graph == "" {
		t.Error("Graph should not be empty after defaults")
	}
	if cfg.Keybindings.Quit == "" {
		t.Error("Quit should not be empty after defaults")
	}
	if cfg.Keybindings.Commit == "" {
		t.Error("Commit should not be empty after defaults")
	}
	// existing value must not be overwritten
	cfg2 := &Config{Keybindings: KeybindingsConfig{Graph: "G"}}
	applyKeybindingDefaults(cfg2)
	if cfg2.Keybindings.Graph != "G" {
		t.Errorf("Graph overwritten: got %q, want G", cfg2.Keybindings.Graph)
	}
	if cfg2.Keybindings.Quit == "" {
		t.Error("Quit should be filled by defaults even when Graph is set")
	}
}

func TestLoadReadsExistingGlobalConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	}

	configDir := filepath.Join(dir, "bonsai")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a config using the old mode name to verify migration.
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(`
[modes]
default = "learning"
`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// "learning" is migrated to "standard" automatically.
	if cfg.Modes.Default != "standard" {
		t.Errorf("modes.default = %q, want standard (migrated from learning)", cfg.Modes.Default)
	}
}

// TestMigrateRoamingToLocal verifies that config.toml and usage.json are moved
// from AppData\Roaming to AppData\Local on Windows (or the XDG equivalent in
// the test harness) when no Local config exists yet.
func TestMigrateRoamingToLocal(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Roaming migration is Windows-only")
	}

	roamingDir := t.TempDir()
	localDir := t.TempDir()
	t.Setenv("APPDATA", roamingDir)
	t.Setenv("LOCALAPPDATA", localDir)

	// Seed legacy files in Roaming.
	roamingBonsai := filepath.Join(roamingDir, "bonsai")
	if err := os.MkdirAll(roamingBonsai, 0o755); err != nil {
		t.Fatal(err)
	}
	configContent := []byte("[flow]\ntype = \"trunk\"\n")
	usageContent := []byte(`{"bonsai":1}`)
	if err := os.WriteFile(filepath.Join(roamingBonsai, "config.toml"), configContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(roamingBonsai, "usage.json"), usageContent, 0o644); err != nil {
		t.Fatal(err)
	}

	localConfigPath := filepath.Join(localDir, "bonsai", "config.toml")
	localUsagePath := filepath.Join(localDir, "bonsai", "usage.json")

	// Migration fires inside GlobalExists.
	exists, err := GlobalExists()
	if err != nil {
		t.Fatalf("GlobalExists: %v", err)
	}
	if !exists {
		t.Fatal("GlobalExists = false after migration, want true")
	}

	// config.toml must be in Local.
	got, err := os.ReadFile(localConfigPath)
	if err != nil {
		t.Fatalf("config.toml not found in Local: %v", err)
	}
	if string(got) != string(configContent) {
		t.Errorf("config.toml content mismatch: got %q, want %q", got, configContent)
	}

	// usage.json must be in Local.
	gotUsage, err := os.ReadFile(localUsagePath)
	if err != nil {
		t.Fatalf("usage.json not found in Local: %v", err)
	}
	if string(gotUsage) != string(usageContent) {
		t.Errorf("usage.json content mismatch: got %q, want %q", gotUsage, usageContent)
	}

	// Old Roaming files must be gone.
	if _, err := os.Stat(filepath.Join(roamingBonsai, "config.toml")); err == nil {
		t.Error("config.toml still exists in Roaming after migration")
	}
	if _, err := os.Stat(filepath.Join(roamingBonsai, "usage.json")); err == nil {
		t.Error("usage.json still exists in Roaming after migration")
	}
}

// TestMigrateRoamingNoOpWhenLocalExists verifies that an existing Local config
// is never overwritten by the migration.
func TestMigrateRoamingNoOpWhenLocalExists(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Roaming migration is Windows-only")
	}

	roamingDir := t.TempDir()
	localDir := t.TempDir()
	t.Setenv("APPDATA", roamingDir)
	t.Setenv("LOCALAPPDATA", localDir)

	roamingBonsai := filepath.Join(roamingDir, "bonsai")
	localBonsai := filepath.Join(localDir, "bonsai")
	if err := os.MkdirAll(roamingBonsai, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localBonsai, 0o755); err != nil {
		t.Fatal(err)
	}

	roamingConfig := []byte("[flow]\ntype = \"trunk\"\n")
	localConfig := []byte("[flow]\ntype = \"gitflow\"\n")
	if err := os.WriteFile(filepath.Join(roamingBonsai, "config.toml"), roamingConfig, 0o644); err != nil {
		t.Fatal(err)
	}
	localConfigPath := filepath.Join(localBonsai, "config.toml")
	if err := os.WriteFile(localConfigPath, localConfig, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := GlobalExists(); err != nil {
		t.Fatalf("GlobalExists: %v", err)
	}

	// Local config must be unchanged.
	got, err := os.ReadFile(localConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(localConfig) {
		t.Errorf("local config was overwritten: got %q, want %q", got, localConfig)
	}
}
