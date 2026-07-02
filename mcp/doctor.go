package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var allBonsaiMCPTools = []string{
	"mcp__bonsai__git_context",
	"mcp__bonsai__git_status",
	"mcp__bonsai__git_log",
	"mcp__bonsai__git_diff",
	"mcp__bonsai__git_show",
	"mcp__bonsai__git_blame",
	"mcp__bonsai__git_branches",
	"mcp__bonsai__git_stash_list",
	"mcp__bonsai__git_review",
}

// Doctor checks MCP configuration status and explains where tools are accessible.
// scan=true walks local plugin directories for skill files that need updating.
// test=true spawns the MCP server and verifies it responds correctly end-to-end.
func Doctor(scan, test bool) {
	stat, err := os.Stdout.Stat()
	tty := err == nil && (stat.Mode()&os.ModeCharDevice) != 0

	green := func(s string) string {
		if tty {
			return "\033[32m" + s + "\033[0m"
		}
		return s
	}
	yellow := func(s string) string {
		if tty {
			return "\033[33m" + s + "\033[0m"
		}
		return s
	}
	dim := func(s string) string {
		if tty {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}
	bold := func(s string) string {
		if tty {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}

	pad := func(s string, n int) string {
		r := []rune(s)
		for len(r) < n {
			r = append(r, ' ')
		}
		return string(r)
	}

	fmt.Println(bold("bonsai mcp doctor"))
	fmt.Println()

	// --- Binary ---
	bonsaiExe, binErr := exec.LookPath("bonsai")
	fmt.Println(bold("Binary"))
	if binErr == nil {
		fmt.Printf("  %s  %s  %s\n", green("✓"), pad("bonsai", 28), bonsaiExe)
	} else {
		fmt.Printf("  %s  %s  %s\n", yellow("⚠"), pad("bonsai", 28), "not found in PATH — install first")
	}
	fmt.Println()

	// --- Configuration targets ---
	home, _ := os.UserHomeDir()

	type configTarget struct {
		label string
		path  string
		scope string
		skip  bool
		why   string
	}

	targets := []configTarget{
		{
			label: "Claude Code  user scope",
			path:  filepath.Join(home, ".claude.json"),
			scope: "all sessions and projects",
		},
		{
			label: "Claude Code  project",
			path:  ".mcp.json",
			scope: "this project only",
		},
	}

	if p := claudeDesktopConfigPath(); p != "" {
		_, statErr := os.Stat(filepath.Dir(p))
		targets = append(targets, configTarget{
			label: "Claude Desktop",
			path:  p,
			scope: "Desktop app",
			skip:  statErr != nil,
			why:   "not installed",
		})
	}

	targets = append(targets, configTarget{
		label: "Cursor  project",
		path:  filepath.Join(".cursor", "mcp.json"),
		scope: "Cursor in this project",
		skip:  !cursorDetected(),
		why:   "not detected",
	})

	fmt.Println(bold("Configuration"))
	configured, applicable := 0, 0
	for _, t := range targets {
		label := pad(t.label, 28)
		if t.skip {
			fmt.Printf("  %s  %s  %s\n", dim("-"), label, dim(t.why))
			continue
		}
		applicable++
		if isBonsaiInConfig(t.path) {
			configured++
			fmt.Printf("  %s  %s  %s\n", green("✓"), label, dim(t.scope))
		} else {
			fmt.Printf("  %s  %s  %s\n", yellow("⚠"), label, "not configured — run: bonsai mcp --install")
		}
	}
	fmt.Println()

	// --- MCP scope model ---
	fmt.Println(bold("MCP scope"))
	fmt.Printf("  %s  %s  %s\n", green("✓"), pad("main session", 34), dim("bonsai tools available unconditionally"))
	fmt.Printf("  %s  %s  %s\n", green("✓"), pad("skills: no tools: declaration", 34), dim("bonsai tools available"))
	fmt.Printf("  %s  %s  %s\n", yellow("⚠"), pad("sub-agents: explicit tools: list", 34), "NOT accessible unless listed")
	fmt.Println()
	fmt.Println(dim("  Sub-agents declare a fixed tools: list in their frontmatter. Only listed"))
	fmt.Println(dim("  tools are accessible — bonsai tools are excluded unless explicitly added."))
	fmt.Println(dim("  You cannot fix this for skills you do not own."))
	if !scan {
		fmt.Println()
		fmt.Println(dim("  Run 'bonsai mcp --doctor --scan' to find local skill files that need updating."))
	}
	fmt.Println()

	// --- Skill scan ---
	if scan {
		fmt.Println(bold("Skill scan"))
		dirs := doctorScanDirs(home)
		if len(dirs) == 0 {
			fmt.Printf("  %s  no plugin directories found\n", dim("-"))
			fmt.Printf("     %s\n", dim("checked: "+filepath.Join(home, ".claude", "plugins")+
				", "+filepath.Join(home, ".claude", "local-marketplace", "plugins")))
			fmt.Println()
		} else {
			type result struct {
				path      string
				hasBonsai bool
			}
			var results []result

			for _, dir := range dirs {
				_ = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
					if walkErr != nil || info.IsDir() || !strings.HasSuffix(p, ".md") {
						return nil
					}
					hasTools, hasBonsai := parseFrontmatterTools(p)
					if hasTools {
						results = append(results, result{path: p, hasBonsai: hasBonsai})
					}
					return nil
				})
			}

			if len(results) == 0 {
				fmt.Printf("  %s  no agent files with tools: restrictions found\n", green("✓"))
				fmt.Println(dim("  (all installed skills use unrestricted tool access — bonsai tools already available)"))
			} else {
				needsUpdate := 0
				for _, r := range results {
					rel, relErr := filepath.Rel(home, r.path)
					if relErr != nil || rel == "" {
						rel = r.path
					} else {
						rel = "~/" + rel
					}
					if r.hasBonsai {
						fmt.Printf("  %s  %s\n", green("✓"), rel)
					} else {
						needsUpdate++
						fmt.Printf("  %s  %s\n", yellow("⚠"), rel)
						fmt.Printf("        %s\n", dim("add to tools: line — "+strings.Join(allBonsaiMCPTools, ", ")))
					}
				}
				if needsUpdate > 0 {
					fmt.Println()
					fmt.Printf("  %d file(s) need updating to access bonsai MCP tools\n", needsUpdate)
				}
			}
			fmt.Println()
		}
	}

	// --- Live test ---
	if test {
		fmt.Println(bold("Live test"))
		if binErr != nil {
			fmt.Printf("  %s  skipped — bonsai binary not found\n", yellow("⚠"))
		} else {
			runMCPSelfTest(bonsaiExe, green, yellow, dim, pad)
		}
		fmt.Println()
	}

	// --- Summary ---
	fmt.Printf("Summary: bonsai MCP configured in %d/%d applicable target(s)\n", configured, applicable)
	if configured == 0 && applicable > 0 {
		fmt.Println()
		fmt.Println("  Run 'bonsai mcp --install' to configure.")
	}
}

// runMCPSelfTest spawns bonsai mcp, sends real JSON-RPC requests, and reports results.
func runMCPSelfTest(
	bonsaiExe string,
	green, yellow, dim func(string) string,
	pad func(string, int) string,
) {
	cmd := exec.Command(bonsaiExe, "mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("  %s  could not create stdin pipe: %v\n", yellow("⚠"), err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("  %s  could not create stdout pipe: %v\n", yellow("⚠"), err)
		return
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		fmt.Printf("  %s  could not start bonsai mcp: %v\n", yellow("⚠"), err)
		return
	}
	defer func() {
		stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	enc := json.NewEncoder(stdin)
	dec := json.NewDecoder(stdout)

	send := func(id int, method string, params any) (map[string]any, error) {
		req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
		if params != nil {
			req["params"] = params
		}
		if err := enc.Encode(req); err != nil {
			return nil, err
		}

		done := make(chan struct{})
		var resp map[string]any
		var decErr error
		go func() {
			decErr = dec.Decode(&resp)
			close(done)
		}()
		select {
		case <-done:
			return resp, decErr
		case <-time.After(5 * time.Second):
			return nil, fmt.Errorf("timeout")
		}
	}

	check := func(label string, resp map[string]any, err error, validate func(map[string]any) string) {
		l := pad(label, 28)
		if err != nil {
			fmt.Printf("  %s  %s  %v\n", yellow("⚠"), l, err)
			return
		}
		if errField, ok := resp["error"]; ok {
			fmt.Printf("  %s  %s  server error: %v\n", yellow("⚠"), l, errField)
			return
		}
		result, _ := resp["result"].(map[string]any)
		if result == nil {
			fmt.Printf("  %s  %s  empty result\n", yellow("⚠"), l)
			return
		}
		if note := validate(result); note != "" {
			fmt.Printf("  %s  %s  %s\n", green("✓"), l, dim(note))
		} else {
			fmt.Printf("  %s  %s\n", green("✓"), l)
		}
	}

	// initialize
	resp, err := send(1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo":      map[string]any{"name": "bonsai-doctor", "version": "0"},
	})
	check("initialize", resp, err, func(r map[string]any) string {
		if info, ok := r["serverInfo"].(map[string]any); ok {
			if v, ok := info["version"].(string); ok {
				return "server version " + v
			}
		}
		return ""
	})

	// tools/list
	resp, err = send(2, "tools/list", nil)
	check("tools/list", resp, err, func(r map[string]any) string {
		tools, _ := r["tools"].([]any)
		return fmt.Sprintf("%d tools registered", len(tools))
	})

	// git_status (smoke test — works inside or outside a git repo)
	resp, err = send(3, "tools/call", map[string]any{
		"name":      "git_status",
		"arguments": map[string]any{},
	})
	check("tools/call git_status", resp, err, func(r map[string]any) string {
		content, _ := r["content"].([]any)
		if len(content) > 0 {
			if item, ok := content[0].(map[string]any); ok {
				if text, ok := item["text"].(string); ok && len(text) > 0 {
					lines := strings.Count(text, "\n") + 1
					return fmt.Sprintf("returned %d lines", lines)
				}
			}
		}
		return "returned content"
	})
}

// isBonsaiInConfig reports whether a JSON config file has a bonsai mcpServers entry.
func isBonsaiInConfig(path string) bool {
	cfg := readJSONMap(path)
	if cfg == nil {
		return false
	}
	servers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	_, exists := servers["bonsai"]
	return exists
}

// doctorScanDirs returns the default plugin directories to walk for skill files.
func doctorScanDirs(home string) []string {
	var dirs []string
	candidates := []string{
		filepath.Join(home, ".claude", "plugins"),
		filepath.Join(home, ".claude", "local-marketplace", "plugins"),
	}
	for _, d := range candidates {
		if _, err := os.Stat(d); err == nil {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// parseFrontmatterTools reads the YAML frontmatter of a markdown file.
// Returns (hasTools, hasBonsai): whether a tools: field exists, and whether
// it already includes at least one mcp__bonsai__* tool.
func parseFrontmatterTools(path string) (hasTools, hasBonsai bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return
	}

	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "tools:") {
			continue
		}
		hasTools = true
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "tools:"))
		for _, t := range strings.Split(rest, ",") {
			if strings.HasPrefix(strings.TrimSpace(t), "mcp__bonsai__") {
				hasBonsai = true
				return
			}
		}
	}
	return
}
