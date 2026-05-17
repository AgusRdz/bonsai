package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// mcpConfig is the standard MCP server config block shared by all tools.
var mcpConfig = map[string]any{
	"mcpServers": map[string]any{
		"bonsai": map[string]any{
			"command": "bonsai",
			"args":    []string{"mcp"},
		},
	},
}

// Install runs the interactive MCP install wizard.
func Install() error {
	// Verify bonsai is in PATH first.
	if _, err := exec.LookPath("bonsai"); err != nil {
		return fmt.Errorf("bonsai is not in PATH - install it first")
	}

	fmt.Println()
	fmt.Println("bonsai mcp install - configure bonsai as an MCP server")
	fmt.Println(strings.Repeat("-", 54))
	fmt.Println()

	targets := detectTargets()

	if len(targets) == 0 {
		fmt.Println("No supported AI tools detected on this system.")
		fmt.Println()
		printManualConfig()
		return nil
	}

	fmt.Println("Detected AI tools:")
	for i, t := range targets {
		fmt.Printf("  %d) %s\n", i+1, t.label)
	}
	fmt.Printf("  %d) Print config to copy manually\n", len(targets)+1)
	fmt.Println()

	choice := ask("Install for (enter number, or 'all')", "all")

	if choice == "all" {
		for _, t := range targets {
			installOne(t)
		}
		return nil
	}

	// Single choice.
	n := 0
	_, _ = fmt.Sscanf(choice, "%d", &n)
	if n == len(targets)+1 {
		printManualConfig()
		return nil
	}
	if n >= 1 && n <= len(targets) {
		installOne(targets[n-1])
		return nil
	}

	fmt.Println("invalid choice")
	return nil
}

func installOne(t installTarget) {
	fmt.Printf("Installing for %s... ", t.label)
	if err := t.install(); err != nil {
		fmt.Printf("failed: %v\n", err)
		return
	}
	fmt.Println("done")
	if t.note != "" {
		fmt.Printf("  %s\n", t.note)
	}
}

// ---------------------------------------------------------------------------
// Target detection
// ---------------------------------------------------------------------------

type installTarget struct {
	label   string
	install func() error
	note    string
}

func detectTargets() []installTarget {
	var targets []installTarget

	// Claude Code (project scope) - available if `claude` CLI is in PATH.
	if _, err := exec.LookPath("claude"); err == nil {
		targets = append(targets, installTarget{
			label:   "Claude Code  (project scope - writes .mcp.json in current directory)",
			install: installClaudeCodeProject,
			note:    "Restart Claude Code in this directory to pick it up.",
		})
		targets = append(targets, installTarget{
			label:   "Claude Code  (user scope - available in all projects)",
			install: installClaudeCodeUser,
			note:    "Restart Claude Code to pick it up.",
		})
	}

	// Claude Desktop.
	if p := claudeDesktopConfigPath(); p != "" {
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			targets = append(targets, installTarget{
				label:   "Claude Desktop",
				install: func() error { return installClaudeDesktop(p) },
				note:    "Restart Claude Desktop to pick it up.",
			})
		}
	}

	// Cursor (project scope) - check common install locations.
	if cursorDetected() {
		targets = append(targets, installTarget{
			label:   "Cursor  (project scope - writes .cursor/mcp.json in current directory)",
			install: installCursor,
			note:    "Restart Cursor in this directory to pick it up.",
		})
	}

	return targets
}

func claudeDesktopConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "Claude", "claude_desktop_config.json")
		}
	case "linux":
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
	return ""
}

func cursorDetected() bool {
	if _, err := exec.LookPath("cursor"); err == nil {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	// Check common Cursor install locations.
	candidates := []string{
		filepath.Join(home, ".cursor"),
		filepath.Join(home, "Library", "Application Support", "Cursor"),
		filepath.Join(home, ".config", "Cursor"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Installers
// ---------------------------------------------------------------------------

// installClaudeCodeProject writes .mcp.json in the current directory.
func installClaudeCodeProject() error {
	return writeMCPJSON(".mcp.json")
}

// installClaudeCodeUser merges bonsai into ~/.claude/settings.json.
func installClaudeCodeUser() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return mergeIntoConfig(filepath.Join(home, ".claude", "settings.json"))
}

// installClaudeDesktop merges bonsai into claude_desktop_config.json.
func installClaudeDesktop(path string) error {
	return mergeIntoConfig(path)
}

// installCursor writes .cursor/mcp.json in the current directory.
func installCursor() error {
	if err := os.MkdirAll(".cursor", 0755); err != nil {
		return err
	}
	return writeMCPJSON(filepath.Join(".cursor", "mcp.json"))
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// writeMCPJSON writes a fresh MCP config file.
// If the file already exists and contains a bonsai entry, it is left unchanged.
func writeMCPJSON(path string) error {
	existing := readJSONMap(path)
	if existing != nil {
		if servers, ok := existing["mcpServers"].(map[string]any); ok {
			if _, alreadySet := servers["bonsai"]; alreadySet {
				fmt.Printf("(already configured in %s) ", path)
				return nil
			}
		}
	}

	var cfg map[string]any
	if existing != nil {
		cfg = existing
	} else {
		cfg = map[string]any{}
	}

	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["bonsai"] = map[string]any{
		"command": "bonsai",
		"args":    []string{"mcp"},
	}
	cfg["mcpServers"] = servers

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// mergeIntoConfig merges the bonsai entry into an existing JSON config file.
func mergeIntoConfig(path string) error {
	cfg := readJSONMap(path)
	if cfg == nil {
		cfg = map[string]any{}
	}

	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, alreadySet := servers["bonsai"]; alreadySet {
		fmt.Printf("(already configured in %s) ", path)
		return nil
	}
	servers["bonsai"] = map[string]any{
		"command": "bonsai",
		"args":    []string{"mcp"},
	}
	cfg["mcpServers"] = servers

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func readJSONMap(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func printManualConfig() {
	data, _ := json.MarshalIndent(mcpConfig, "", "  ")
	fmt.Println("Add this to your AI tool's MCP config file:")
	fmt.Println()
	fmt.Println(string(data))
	fmt.Println()
	fmt.Println("Common config file locations:")
	fmt.Println("  Claude Code (project)  .mcp.json  in your project root")
	fmt.Println("  Claude Code (user)     run: claude mcp add --scope user bonsai bonsai mcp")
	fmt.Println("  Claude Desktop (mac)   ~/Library/Application Support/Claude/claude_desktop_config.json")
	fmt.Println("  Cursor (project)       .cursor/mcp.json  in your project root")
	fmt.Println("  Windsurf               ~/.windsurf/mcp.json")
}

func ask(label, def string) string {
	sc := bufio.NewScanner(os.Stdin)
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	if !sc.Scan() {
		return def
	}
	v := strings.TrimSpace(sc.Text())
	if v == "" {
		return def
	}
	return v
}
