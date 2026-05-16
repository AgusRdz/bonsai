package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/doctor"
	"github.com/AgusRdz/bonsai/git"
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
		runDoctor(os.Args[2:])
	case "init":
		runInit()
	case "setup":
		runSetup(os.Args[2:])
	case "stats":
		runStats()
	case "patch":
		runPatch(os.Args[2:])
	case "archive":
		runArchive(os.Args[2:])
	case "bundle":
		runBundle(os.Args[2:])
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
	local := false
	for _, a := range args {
		if a == "--local" {
			local = true
		}
	}
	if local {
		if err := setup.RunLocal(); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: setup:", err)
			os.Exit(1)
		}
	} else {
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
  setup --local     interactive setup wizard (per-project .bonsai.toml)
  init              create a .bonsai.toml template without a wizard
  config            open global config in your editor
  config --local    open (or create) per-project .bonsai.toml in your editor
  config --path     print the path to the global config file
  doctor            check global and local git configuration health
  doctor --verbose  same, with a one-line explanation per check
  stats             show repository statistics
  patch create      create .patch files from commits (git format-patch)
  patch apply       apply a .patch file (git am)
  archive           export repo as tar.gz or zip
  bundle create     pack refs into a portable bundle file
  bundle verify     verify a bundle file

Options:
  -h, --help     show help
  -v, --version  print version
`, version)
}

func runConfig(args []string) {
	local := false
	path := false
	for _, a := range args {
		switch a {
		case "--local":
			local = true
		case "--path":
			path = true
		}
	}

	if path {
		p, err := config.GlobalConfigPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai: config:", err)
			os.Exit(1)
		}
		fmt.Println(p)
		return
	}

	cfg, _ := config.Load()
	editor := config.ResolveEditor(cfg)

	if local {
		openInEditor(editor, ".bonsai.toml")
	} else {
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
	fmt.Println("run 'bonsai config --local' to open it in your editor")
}

func runDoctor(args []string) {
	verbose := false
	for _, a := range args {
		if a == "--verbose" || a == "-v" {
			verbose = true
		}
	}
	report, err := doctor.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: doctor: %v\n", err)
		os.Exit(1)
	}
	printReport(report, verbose)
}

// isTTY reports whether stdout is an interactive terminal.
func isTTY() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func printReport(report *doctor.Report, verbose bool) {
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
	dim := func(s string) string {
		if color {
			return "\033[2m" + s + "\033[0m"
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
			if verbose && c.Explain != "" {
				fmt.Printf("     %s\n", dim(c.Explain))
			}
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

// ---------------------------------------------------------------------------
// bonsai stats
// ---------------------------------------------------------------------------

func runStats() {
	ctx := context.Background()
	r := git.New()
	s, err := r.Stats(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: stats: %v\n", err)
		os.Exit(1)
	}

	color := isTTY()
	bold := func(v string) string {
		if color {
			return "\033[1m" + v + "\033[0m"
		}
		return v
	}
	dim := func(v string) string {
		if color {
			return "\033[2m" + v + "\033[0m"
		}
		return v
	}

	fmt.Println(bold("bonsai stats"))
	fmt.Println()

	// Overview
	fmt.Println(bold("Overview"))
	fmt.Printf("  commits      %d", s.TotalCommits)
	if s.FirstCommitDate != "" && s.LastCommitDate != "" {
		fmt.Printf("  %s(%s - %s)%s", dim(""), s.FirstCommitDate, s.LastCommitDate, dim(""))
	}
	fmt.Println()
	fmt.Printf("  last 30 days %d\n", s.CommitsLast30d)
	fmt.Printf("  branches     %d\n", s.TotalBranches)
	fmt.Printf("  tags         %d\n", s.TotalTags)
	fmt.Printf("  files        %d tracked\n", s.TrackedFiles)
	fmt.Println()

	// Contributors
	if len(s.Contributors) > 0 {
		fmt.Println(bold("Contributors"))
		maxCount := s.Contributors[0].Count
		barWidth := 20
		for _, c := range s.Contributors {
			filled := 0
			if maxCount > 0 {
				filled = c.Count * barWidth / maxCount
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			fmt.Printf("  %-25s %s %d\n", c.Name, dim(bar), c.Count)
		}
		fmt.Println()
	}

	// File types
	if len(s.ExtBreakdown) > 0 {
		fmt.Println(bold("File types"))
		maxCount := s.ExtBreakdown[0].Count
		barWidth := 20
		for _, e := range s.ExtBreakdown {
			filled := 0
			if maxCount > 0 {
				filled = e.Count * barWidth / maxCount
			}
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
			fmt.Printf("  %-12s %s %d\n", e.Ext, dim(bar), e.Count)
		}
		fmt.Println()
	}

	// Most changed files
	if len(s.TopFiles) > 0 {
		fmt.Println(bold("Most changed files"))
		for i, f := range s.TopFiles {
			fmt.Printf("  %2d. %-40s %d changes\n", i+1, f.Path, f.Count)
		}
	}
}

// ---------------------------------------------------------------------------
// bonsai patch
// ---------------------------------------------------------------------------

func runPatch(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai patch create --base=<ref> [--output=<dir>]")
		fmt.Fprintln(os.Stderr, "       bonsai patch apply <file> [<file>...]")
		os.Exit(1)
	}
	ctx := context.Background()
	r := git.New()
	switch args[0] {
	case "create":
		base := ""
		outputDir := ""
		for _, a := range args[1:] {
			if strings.HasPrefix(a, "--base=") {
				base = strings.TrimPrefix(a, "--base=")
			} else if strings.HasPrefix(a, "--output=") {
				outputDir = strings.TrimPrefix(a, "--output=")
			}
		}
		if base == "" {
			fmt.Fprintln(os.Stderr, "bonsai: patch create requires --base=<ref>")
			os.Exit(1)
		}
		files, err := r.FormatPatch(ctx, base, outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: patch create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("created %d patch file(s):\n", len(files))
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
	case "apply":
		files := args[1:]
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "bonsai: patch apply requires at least one file")
			os.Exit(1)
		}
		if err := r.ApplyPatch(ctx, files...); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: patch apply: %v\n", err)
			fmt.Fprintln(os.Stderr, "resolve conflicts then run: git am --continue")
			fmt.Fprintln(os.Stderr, "to abort: git am --abort")
			os.Exit(1)
		}
		fmt.Printf("applied %d patch file(s)\n", len(files))
	default:
		fmt.Fprintf(os.Stderr, "bonsai: patch: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// bonsai archive
// ---------------------------------------------------------------------------

func runArchive(args []string) {
	format := "tar.gz"
	output := ""
	ref := "HEAD"
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case strings.HasPrefix(a, "--output="):
			output = strings.TrimPrefix(a, "--output=")
		case strings.HasPrefix(a, "--ref="):
			ref = strings.TrimPrefix(a, "--ref=")
		}
	}
	if output == "" {
		output = "archive." + format
	}
	ctx := context.Background()
	r := git.New()
	if err := r.Archive(ctx, format, output, ref); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: archive: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("created %s from %s\n", output, ref)
}

// ---------------------------------------------------------------------------
// bonsai bundle
// ---------------------------------------------------------------------------

func runBundle(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai bundle create <file> [<ref>...]")
		fmt.Fprintln(os.Stderr, "       bonsai bundle verify <file>")
		os.Exit(1)
	}
	ctx := context.Background()
	r := git.New()
	switch args[0] {
	case "create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "bonsai: bundle create requires <file>")
			os.Exit(1)
		}
		output := args[1]
		refs := args[2:]
		if err := r.BundleCreate(ctx, output, refs...); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: bundle create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("bundle written to %s\n", output)
	case "verify":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "bonsai: bundle verify requires <file>")
			os.Exit(1)
		}
		msg, err := r.BundleVerify(ctx, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: bundle verify: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(msg)
	default:
		fmt.Fprintf(os.Stderr, "bonsai: bundle: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}
