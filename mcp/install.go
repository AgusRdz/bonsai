package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var errAlreadyConfigured = errors.New("already configured")

// Install runs the interactive MCP install wizard.
func Install() error {
	bonsaiCmd, err := exec.LookPath("bonsai")
	if err != nil {
		return fmt.Errorf("bonsai is not in PATH - install it first")
	}

	fmt.Println()
	fmt.Println("bonsai mcp install - configure bonsai as an MCP server")
	fmt.Println(strings.Repeat("-", 54))
	fmt.Println()

	targets := detectTargets(bonsaiCmd)

	if len(targets) == 0 {
		fmt.Println("No supported AI tools detected on this system.")
		fmt.Println()
		printManualConfig(bonsaiCmd)
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
		printManualConfig(bonsaiCmd)
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
	err := t.install()
	if errors.Is(err, errAlreadyConfigured) {
		fmt.Println("already configured, skipping")
		return
	}
	if err != nil {
		fmt.Printf("failed: %v\n", err)
		return
	}
	fmt.Println("done")
	if t.note != "" {
		fmt.Printf("  note: %s\n", t.note)
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

func detectTargets(bonsaiCmd string) []installTarget {
	var targets []installTarget

	// Claude Code (project scope) - available if `claude` CLI is in PATH.
	if _, err := exec.LookPath("claude"); err == nil {
		targets = append(targets, installTarget{
			label:   "Claude Code  (project scope - writes .mcp.json in current directory)",
			install: func() error { return writeMCPJSON(".mcp.json", bonsaiCmd) },
			note:    "Restart Claude Code in this directory to pick it up.",
		})
		targets = append(targets, installTarget{
			label:   "Claude Code  (user scope - available in all projects)",
			install: func() error { return installClaudeCodeUser(bonsaiCmd) },
			note:    "Restart Claude Code to pick it up.",
		})
	}

	// Claude Desktop.
	if p := claudeDesktopConfigPath(); p != "" {
		if _, err := os.Stat(filepath.Dir(p)); err == nil {
			targets = append(targets, installTarget{
				label:   "Claude Desktop",
				install: func() error { return mergeIntoConfig(p, bonsaiCmd) },
				note:    "Restart Claude Desktop to pick it up.",
			})
		}
	}

	// Cursor (project scope) - check common install locations.
	if cursorDetected() {
		targets = append(targets, installTarget{
			label:   "Cursor  (project scope - writes .cursor/mcp.json in current directory)",
			install: func() error { return installCursor(bonsaiCmd) },
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

func installClaudeCodeUser(bonsaiCmd string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return mergeIntoConfig(filepath.Join(home, ".claude", "settings.json"), bonsaiCmd)
}

func installCursor(bonsaiCmd string) error {
	if err := os.MkdirAll(".cursor", 0755); err != nil {
		return err
	}
	return writeMCPJSON(filepath.Join(".cursor", "mcp.json"), bonsaiCmd)
}

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

// writeMCPJSON writes a fresh MCP config file.
// If the file already exists and contains a bonsai entry, it is left unchanged.
func writeMCPJSON(path, bonsaiCmd string) error {
	existing := readJSONMap(path)
	if existing != nil {
		if servers, ok := existing["mcpServers"].(map[string]any); ok {
			if entry, alreadySet := servers["bonsai"]; alreadySet {
				if m, ok := entry.(map[string]any); ok && m["command"] == bonsaiCmd {
					return errAlreadyConfigured
				}
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
		"command": bonsaiCmd,
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
func mergeIntoConfig(path, bonsaiCmd string) error {
	cfg := readJSONMap(path)
	if cfg == nil {
		cfg = map[string]any{}
	}

	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if entry, alreadySet := servers["bonsai"]; alreadySet {
		if m, ok := entry.(map[string]any); ok && m["command"] == bonsaiCmd {
			return errAlreadyConfigured
		}
	}
	servers["bonsai"] = map[string]any{
		"command": bonsaiCmd,
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

// Uninstall removes the bonsai MCP server entry from all known config locations.
func Uninstall() error {
	fmt.Println()
	fmt.Println("bonsai mcp uninstall - remove bonsai MCP server configuration")
	fmt.Println(strings.Repeat("-", 56))
	fmt.Println()

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	type location struct {
		path  string
		label string
	}
	locations := []location{
		{".mcp.json", "Claude Code project scope"},
		{filepath.Join(".cursor", "mcp.json"), "Cursor project scope"},
		{filepath.Join(home, ".claude", "settings.json"), "Claude Code user scope"},
	}
	if p := claudeDesktopConfigPath(); p != "" {
		locations = append(locations, location{p, "Claude Desktop"})
	}

	removed := 0
	for _, loc := range locations {
		if removeFromFile(loc.path) {
			fmt.Printf("  removed from %s (%s)\n", loc.path, loc.label)
			removed++
		}
	}

	fmt.Println()
	if removed == 0 {
		fmt.Println("bonsai was not found in any known configuration location.")
	} else {
		fmt.Printf("Removed from %d location(s). Restart affected tools to apply.\n", removed)
	}
	return nil
}

// removeFromFile deletes the bonsai key from mcpServers in a JSON config file.
// Returns true if the entry was present and successfully removed.
func removeFromFile(path string) bool {
	cfg := readJSONMap(path)
	if cfg == nil {
		return false
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	if _, exists := servers["bonsai"]; !exists {
		return false
	}
	delete(servers, "bonsai")
	cfg["mcpServers"] = servers

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return false
	}
	return os.WriteFile(path, append(data, '\n'), 0644) == nil
}

func printManualConfig(bonsaiCmd string) {
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"bonsai": map[string]any{
				"command": bonsaiCmd,
				"args":    []string{"mcp"},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	fmt.Println("Add this to your AI tool's MCP config file:")
	fmt.Println()
	fmt.Println(string(data))
	fmt.Println()
	fmt.Println("Common config file locations:")
	fmt.Println("  Claude Code (project)  .mcp.json  in your project root")
	fmt.Println("  Claude Code (user)     ~/.claude/settings.json")
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
