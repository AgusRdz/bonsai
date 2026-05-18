package mcp

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// Pass scan=true to also walk local plugin directories for skill files that need updating.
func Doctor(scan bool) {
	stat, err := os.Stdout.Stat()
	tty := err == nil && (stat.Mode()&os.ModeCharDevice) != 0

	green  := func(s string) string { if tty { return "\033[32m" + s + "\033[0m" }; return s }
	yellow := func(s string) string { if tty { return "\033[33m" + s + "\033[0m" }; return s }
	dim    := func(s string) string { if tty { return "\033[2m"  + s + "\033[0m" }; return s }
	bold   := func(s string) string { if tty { return "\033[1m"  + s + "\033[0m" }; return s }

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

	// --- Summary ---
	fmt.Printf("Summary: bonsai MCP configured in %d/%d applicable target(s)\n", configured, applicable)
	if configured == 0 && applicable > 0 {
		fmt.Println()
		fmt.Println("  Run 'bonsai mcp --install' to configure.")
	}
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
