package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/doctor"
	"github.com/AgusRdz/bonsai/gitcheck"
	"github.com/AgusRdz/bonsai/setup"
	"github.com/AgusRdz/bonsai/tui"
	"github.com/AgusRdz/bonsai/updater"
)

//go:embed CHANGELOG.md
var changelog string

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	updater.CleanupStaleUpdate()
	gitcheck.EnsureInstalled()

	if len(os.Args) < 2 {
		gitcheck.SuggestUpdate()
		runTUI()
		return
	}

	switch os.Args[1] {
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version", "-v":
		fmt.Printf("bonsai %s\n", version)
	case "update":
		updater.Run(version)
	case "uninstall":
		runUninstall()
	case "changelog", "--changelog":
		fmt.Print(changelog)
	case "config":
		runConfig(os.Args[2:])
	case "doctor":
		runDoctor()
	case "init":
		runInit()
	case "setup":
		runSetup(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "bonsai: unknown command %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run 'bonsai help' for available commands.")
		os.Exit(1)
	}
}

func runTUI() {
	// First-run: no global config yet - walk the user through setup before
	// opening the TUI so they start with a config that matches their workflow.
	exists, err := config.GlobalExists()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: config: %v\n", err)
		os.Exit(1)
	}
	if !exists {
		if err := setup.RunGlobal(); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: setup: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		fmt.Println("opening bonsai...")
		fmt.Println()
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: config: %v\n", err)
		os.Exit(1)
	}
	if err := tui.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: %v\n", err)
		os.Exit(1)
	}
}

func runSetup(args []string) {
	sub := "global"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "local":
		if err := setup.RunLocal(); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: setup:", err)
			os.Exit(1)
		}
	default:
		if err := setup.RunGlobal(); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: setup:", err)
			os.Exit(1)
		}
	}
}

func printHelp() {
	fmt.Printf(`bonsai %s - a TUI Git client that teaches while you work

Usage:
  bonsai              open the interactive TUI
  bonsai [command]

Commands:
  help              show this help
  version           print version
  update            update to the latest release
  uninstall         remove bonsai from this system
  changelog         show the changelog
  setup             interactive setup wizard (global config)
  setup local       interactive setup wizard (per-project .bonsai.toml)
  init              create a .bonsai.toml template without a wizard
  config            open global config in your editor
  config local      open (or create) per-project .bonsai.toml in your editor
  config global     open global config in your editor (same as 'config')
  config path       print the path to the global config file
  doctor            check global and local git configuration health

Options:
  -h, --help     show help
  -v, --version  print version
`, version)
}

func runConfig(args []string) {
	sub := "global"
	if len(args) > 0 {
		sub = args[0]
	}

	cfg, _ := config.Load()
	editor := config.ResolveEditor(cfg)

	switch sub {
	case "path":
		p, err := config.GlobalConfigPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: config:", err)
			os.Exit(1)
		}
		fmt.Println(p)
		return
	case "local":
		openInEditor(editor, ".bonsai.toml")
	default:
		p, err := config.GlobalConfigPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: config:", err)
			os.Exit(1)
		}
		openInEditor(editor, p)
	}
}

func openInEditor(editor, path string) {
	// Split editor string so flags like "code --wait" work correctly.
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	cmd := exec.Command(parts[0], args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: editor %q: %v\n", editor, err)
		os.Exit(1)
	}
}

func runInit() {
	const local = ".bonsai.toml"
	if err := config.WriteLocalTemplate(local); err != nil {
		fmt.Fprintln(os.Stderr, "bonsai: init:", err)
		os.Exit(1)
	}
	fmt.Printf("created %s\n", local)
	fmt.Println("edit it to customise conventions, mode, and flow for this project")
	fmt.Println("run 'bonsai config local' to open it in your editor")
}

func runDoctor() {
	report, err := doctor.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: doctor: %v\n", err)
		os.Exit(1)
	}
	printReport(report)
}

// isTTY reports whether stdout is an interactive terminal.
func isTTY() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func printReport(report *doctor.Report) {
	color := isTTY()

	green := func(s string) string {
		if color {
			return "\033[32m" + s + "\033[0m"
		}
		return s
	}
	yellow := func(s string) string {
		if color {
			return "\033[33m" + s + "\033[0m"
		}
		return s
	}
	red := func(s string) string {
		if color {
			return "\033[31m" + s + "\033[0m"
		}
		return s
	}

	icon := func(lvl doctor.Level) string {
		switch lvl {
		case doctor.OK:
			return green("✓")
		case doctor.Warn:
			return yellow("⚠")
		default:
			return red("✗")
		}
	}

	printChecks := func(checks []doctor.Check) (errors, warnings, passed int) {
		for _, c := range checks {
			label := c.Label
			// Pad label to 22 chars.
			for len([]rune(label)) < 22 {
				label += " "
			}
			fmt.Printf("  %s  %s  %s\n", icon(c.Level), label, c.Message)
			if c.Fix != "" && c.Level != doctor.OK {
				fmt.Printf("     fix: %s\n", c.Fix)
			}
			switch c.Level {
			case doctor.OK:
				passed++
			case doctor.Warn:
				warnings++
			case doctor.Fail:
				errors++
			}
		}
		return
	}

	fmt.Println("bonsai doctor")
	fmt.Println()

	fmt.Println("Global")
	ge, gw, gp := printChecks(report.Global)
	fmt.Println()

	cwd, _ := os.Getwd()
	repoName := filepath.Base(cwd)
	if report.InRepo {
		fmt.Printf("Local  (%s)\n", repoName)
	} else {
		fmt.Println("Local  (not in a git repository)")
	}
	le, lw, lp := printChecks(report.Local)
	fmt.Println()

	totalErrors := ge + le
	totalWarnings := gw + lw
	totalPassed := gp + lp
	fmt.Printf("Summary: %d errors, %d warnings, %d passed\n", totalErrors, totalWarnings, totalPassed)

	if totalErrors > 0 {
		os.Exit(1)
	}
}

func runUninstall() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: could not locate binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("remove bonsai from %s? [y/N] ", exe)
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		fmt.Println("cancelled")
		return
	}
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Println("cancelled")
		return
	}

	if err := os.Remove(exe); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("bonsai removed.")
	fmt.Println()
	fmt.Println("you may also want to remove:")
	fmt.Println("  ~/.config/bonsai/    global config and metrics")
	fmt.Println("  .bonsai.toml         per-project config files")
	fmt.Println("  the PATH entry in your shell config")
}
