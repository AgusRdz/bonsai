package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AgusRdz/bonsai/agent"
	"github.com/AgusRdz/bonsai/config"
	"github.com/AgusRdz/bonsai/doctor"
	"github.com/AgusRdz/bonsai/git"
	"github.com/AgusRdz/bonsai/gitcheck"
	"github.com/AgusRdz/bonsai/hooks"
	"github.com/AgusRdz/bonsai/ignore"
	"github.com/AgusRdz/bonsai/mcp"
	"github.com/AgusRdz/bonsai/metrics"
	"github.com/AgusRdz/bonsai/plugins"
	"github.com/AgusRdz/bonsai/pr"
	"github.com/AgusRdz/bonsai/setup"
	"github.com/AgusRdz/bonsai/tui"
	"github.com/AgusRdz/bonsai/updater"
)

//go:embed CHANGELOG.md
var changelog string

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	pr.RegisterBuiltins()
	plugins.Discover()
	updater.CleanupStaleUpdate()
	gitcheck.EnsureInstalled()

	if len(os.Args) < 2 {
		runTUI()
		gitcheck.SuggestUpdate()
		return
	}

	switch os.Args[1] {
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version", "-v":
		fmt.Printf("bonsai %s · AgusRdz\n", version)
	case "about", "--about":
		printAbout(version)
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
	case "clone":
		runClone(os.Args[2:])
	case "ssh":
		runSSH(os.Args[2:])
	case "metrics":
		runMetrics()
	case "lfs":
		runLFS(os.Args[2:])
	case "standup":
		runStandup(os.Args[2:])
	case "repo":
		runRepo(os.Args[2:])
	case "ignore":
		runIgnore(os.Args[2:])
	case "hooks":
		runHooks(os.Args[2:])
	case "mcp":
		runMCP(os.Args[2:])
	case "context":
		runAgentContext(os.Args[2:])
	case "status":
		runAgentStatus(os.Args[2:])
	case "log":
		runAgentLog(os.Args[2:])
	case "diff":
		runAgentDiff(os.Args[2:])
	case "show":
		runAgentShow(os.Args[2:])
	case "blame":
		runAgentBlame(os.Args[2:])
	case "branches":
		runAgentBranches(os.Args[2:])
	case "stash-list":
		runAgentStashList(os.Args[2:])
	case "review":
		runAgentReview(os.Args[2:])
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

	// Open metrics DB if enabled (best-effort - never block the TUI on failure).
	var mdb *metrics.DB
	if cfg.Metrics.Enabled {
		if p, err := metrics.DefaultPath(); err == nil {
			if db, err := metrics.Open(p); err == nil {
				mdb = db
				defer mdb.Close()
			}
		}
	}

	if err := tui.Run(cfg, mdb, version); err != nil {
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

func printAbout(version string) {
	if version == "" {
		version = "dev"
	}
	fmt.Printf("bonsai %s\n", version)
	fmt.Println("A terminal UI for git.")
	fmt.Println()
	fmt.Println("Author  AgusRdz")
	fmt.Println("Repo    https://github.com/AgusRdz/bonsai")
}

func printHelp() {
	fmt.Printf(`bonsai %s - a TUI Git client that teaches while you work

Usage:
  bonsai              open the interactive TUI
  bonsai [command]

Commands:
  help              show this help
  version           print version
  about             show author and repository info
  update            update to the latest release
  uninstall         remove bonsai from this system
  changelog         show the changelog

  setup             interactive first-run setup (global config)
  setup --local     per-project .bonsai.toml setup wizard
  init              create a .bonsai.toml template in the current directory
  config            open global config in your editor
  config --local    open (or create) per-project .bonsai.toml in your editor
  config --path     print the path to the global config file

  ignore               list all patterns (local, global, .git/info/exclude)
  ignore --add <pat>   add a pattern (--global or --exclude to target other scopes)
  ignore --remove <pat> remove a pattern
  ignore --check <pat> dry-run: show which files would be matched
  ignore --seed        write base patterns (OS, editor, secrets) to .gitignore
                       --global             seed into global ignore instead
                       --lang go|node|python|dotnet|java  also add language patterns

  hooks               list all hooks (global, shared, local) and dispatcher status
  hooks --install     wire bonsai dispatcher into current repo (.git/bonsai-hooks/)
  hooks --global-install  create ~/.config/git/hooks and set global core.hooksPath
  hooks --add <name>  add a hook (requires --global, --shared, or --local)
                      --template=<name>  use a built-in template
                      --force            overwrite if it already exists
  hooks --remove <name>   remove a hook (--global|--shared|--local)
  hooks --enable <name>   make a hook executable (--global|--shared|--local)
  hooks --disable <name>  remove executable bit, keep the file (--global|--shared|--local)
  hooks --show <name>     print hook content (--global|--shared|--local)
  hooks --edit <name>     open hook in editor (--global|--shared|--local)
  hooks --templates       list built-in hook templates

  doctor            check global and local git configuration health
  doctor --verbose  same, with a one-line explanation per check
  stats             show repository statistics
  standup           show your recent commits (today by default)
  standup --days N  show commits from the last N days (default: 1)
  standup -w        shorthand for --days 7 (the whole week)
  standup -a name   filter by author name (default: you)
  metrics           show locally tracked habit and error metrics

  patch --create    create .patch files from commits (git format-patch)
  patch --apply     apply a .patch file (git am)
  archive           export repo snapshot as tar.gz or zip
  bundle --create   pack refs into a portable bundle file
  bundle --verify   verify a bundle file

  ssh --status      show SSH key and agent status
  ssh --keygen      generate a new SSH key pair
  ssh --show        print your SSH public key

  lfs --status      show pending git lfs objects
  lfs --track <pat> track a file pattern via git lfs (.gitattributes)
  lfs --untrack <pat> stop tracking a pattern
  lfs --pull        download all lfs objects for the current checkout
  lfs --install     install lfs hooks into this repository

MCP server:
  mcp --install          detect AI tools and configure bonsai as an MCP server
  mcp --uninstall        remove bonsai MCP server configuration from AI tools
  mcp --doctor           check MCP config and explain where tools are accessible
  mcp --doctor --scan    also scan local skill files for missing bonsai tools
  mcp --doctor --test    also run a live end-to-end test of the MCP server
  mcp                    start the MCP stdio server (used by AI tools internally)

Agent / structured output:
  context           full repo snapshot: status + diff + recent commits
                    --limit=N    number of commits (default: 10)
                    --detailed   include patch hunks in diff
  status            repository status (json/markdown/xml)
  log               recent commits (--limit=N, --yesterday, --weeks=N,
                    --from=YYYY-MM-DD, --to=YYYY-MM-DD)
  diff              changed files; default: all scopes, counts only
                    --staged / --unstaged / --untracked  show only that scope
                    --detailed                           include patch hunks
                    --file=<path>                        filter to one file
  show [<ref>]      single commit (default: HEAD); --detailed for hunks
  blame --file=path line-by-line blame
  branches          branch list
  stash-list        stash entries
  review            diff and commit context (--base=<ref>)
                    --detailed                           include patch hunks
  --format=<fmt>    override output format: json (default), markdown, xml

Options:
  -h, --help        show help
  -v, --version     print version
      --about       show author and repository info
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
		fmt.Fprintln(os.Stderr, "usage: bonsai patch --create --base=<ref> [--output=<dir>]")
		fmt.Fprintln(os.Stderr, "       bonsai patch --apply <file> [<file>...]")
		os.Exit(1)
	}
	ctx := context.Background()
	r := git.New()
	switch args[0] {
	case "--create":
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
			fmt.Fprintln(os.Stderr, "bonsai: patch --create requires --base=<ref>")
			os.Exit(1)
		}
		files, err := r.FormatPatch(ctx, base, outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: patch --create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("created %d patch file(s):\n", len(files))
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
	case "--apply":
		files := args[1:]
		if len(files) == 0 {
			fmt.Fprintln(os.Stderr, "bonsai: patch --apply requires at least one file")
			os.Exit(1)
		}
		if err := r.ApplyPatch(ctx, files...); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: patch --apply: %v\n", err)
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
		fmt.Fprintln(os.Stderr, "usage: bonsai bundle --create <file> [<ref>...]")
		fmt.Fprintln(os.Stderr, "       bonsai bundle --verify <file>")
		os.Exit(1)
	}
	ctx := context.Background()
	r := git.New()
	switch args[0] {
	case "--create":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "bonsai: bundle --create requires <file>")
			os.Exit(1)
		}
		output := args[1]
		refs := args[2:]
		if err := r.BundleCreate(ctx, output, refs...); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: bundle --create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("bundle written to %s\n", output)
	case "--verify":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "bonsai: bundle --verify requires <file>")
			os.Exit(1)
		}
		msg, err := r.BundleVerify(ctx, args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "bonsai: bundle --verify: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(msg)
	default:
		fmt.Fprintf(os.Stderr, "bonsai: bundle: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func runClone(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai clone <url> [<directory>]")
		os.Exit(1)
	}
	url := args[0]
	dir := ""
	if len(args) > 1 {
		dir = args[1]
	}
	ctx := context.Background()
	r := git.New()
	fmt.Printf("cloning %s...\n", url)
	if err := r.Clone(ctx, url, dir); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: clone: %v\n", err)
		os.Exit(1)
	}
	target := dir
	if target == "" {
		// derive from URL: last path segment without .git
		base := filepath.Base(url)
		target = strings.TrimSuffix(base, ".git")
	}
	fmt.Printf("cloned into %s\n", target)
}

// ---------------------------------------------------------------------------
// bonsai ssh
// ---------------------------------------------------------------------------

func runSSH(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai ssh --status")
		fmt.Fprintln(os.Stderr, "       bonsai ssh --keygen")
		fmt.Fprintln(os.Stderr, "       bonsai ssh --show")
		os.Exit(1)
	}
	switch args[0] {
	case "--status":
		runSSHStatus()
	case "--keygen":
		runSSHKeygen()
	case "--show":
		runSSHShow()
	default:
		fmt.Fprintf(os.Stderr, "bonsai: ssh: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func runSSHStatus() {
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
	bold := func(s string) string {
		if color {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}

	fmt.Println(bold("SSH Status"))
	fmt.Println()

	// Key files
	key := doctor.FindSSHKey()
	fmt.Print("  key file       ")
	if key != nil {
		fmt.Println(green(key.PrivateKey))
		fmt.Print("  public key     ")
		if _, err := os.Stat(key.PublicKey); err == nil {
			fmt.Println(green(key.PublicKey))
		} else {
			fmt.Println(yellow("not found"))
		}
	} else {
		fmt.Println(yellow("no key found in ~/.ssh"))
		fmt.Println("  run: bonsai ssh keygen")
	}
	fmt.Println()

	// ssh-agent
	fmt.Print("  ssh-agent      ")
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		fmt.Println(yellow("not running (SSH_AUTH_SOCK not set)"))
	} else {
		out, err := exec.Command("ssh-add", "-l").CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if strings.Contains(msg, "no identities") {
				fmt.Println(yellow("running but no keys loaded"))
				fmt.Println("  run: ssh-add " + func() string {
					if key != nil {
						return key.PrivateKey
					}
					return "~/.ssh/id_ed25519"
				}())
			} else {
				fmt.Println(yellow("socket found but agent not responding"))
			}
		} else {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			fmt.Printf("%s (%d key(s) loaded)\n", green("running"), len(lines))
		}
	}
	fmt.Println()

	// Connectivity - test all SSH hosts found in the repo's remotes,
	// falling back to github.com when not in a repo or no SSH remotes.
	hosts := doctor.DetectSSHHosts()
	if len(hosts) == 0 {
		hosts = []string{"github.com"}
	}
	for _, host := range hosts {
		label := fmt.Sprintf("  %-16s", host)
		fmt.Print(label)
		ok, msg := doctor.TestSSHHost(host)
		if ok {
			fmt.Println(green(msg))
		} else {
			fmt.Println(yellow(msg))
			fmt.Println("  add your key at: " + doctor.SSHKeyURL(host))
		}
	}
}

func runSSHKeygen() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: ssh keygen: %v\n", err)
		os.Exit(1)
	}
	sshDir := filepath.Join(home, ".ssh")
	keyPath := filepath.Join(sshDir, "id_ed25519")

	if _, err := os.Stat(keyPath); err == nil {
		fmt.Printf("SSH key already exists at %s\n", keyPath)
		fmt.Println("use 'bonsai ssh show' to print your public key")
		return
	}

	// Use git user.email as the key comment if available.
	email := strings.TrimSpace(func() string {
		out, err := exec.Command("git", "config", "--global", "--get", "user.email").Output()
		if err != nil {
			return ""
		}
		return string(out)
	}())
	if email == "" {
		fmt.Print("enter your email address for the key comment: ")
		if _, err := fmt.Scanln(&email); err != nil || strings.TrimSpace(email) == "" {
			email = "bonsai"
		}
	}

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: ssh keygen: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", keyPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: ssh keygen: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("key generated: %s\n", keyPath)
	fmt.Println()
	fmt.Println("your public key (copy and paste it into your forge's SSH key settings):")
	fmt.Println()
	pubKey, err := os.ReadFile(keyPath + ".pub")
	if err == nil {
		fmt.Println(strings.TrimSpace(string(pubKey)))
	}
	fmt.Println()
	fmt.Println("where to add it:")
	fmt.Println("  GitHub    https://github.com/settings/keys")
	fmt.Println("  GitLab    https://gitlab.com/-/profile/keys")
	fmt.Println("  Bitbucket https://bitbucket.org/account/settings/ssh-keys/")
	fmt.Println("  Azure     https://dev.azure.com/<org>/_usersSettings/keys")
	fmt.Println("  Gitea/Forgejo/self-hosted: Settings -> SSH Keys in your profile")
	fmt.Println()
	fmt.Println("then load it into the agent: ssh-add " + keyPath)
}

func runMetrics() {
	p, err := metrics.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai metrics:", err)
		os.Exit(1)
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		fmt.Println("no metrics data found")
		fmt.Println("enable metrics in your config: run 'bonsai config' and set [metrics] enabled = true")
		return
	}
	db, err := metrics.Open(p)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai metrics:", err)
		os.Exit(1)
	}
	defer db.Close()
	s, err := db.Summarize()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai metrics:", err)
		os.Exit(1)
	}

	color := isTTY()
	bold := func(v string) string {
		if color {
			return "\033[1m" + v + "\033[0m"
		}
		return v
	}

	fmt.Println(bold("bonsai metrics"))
	fmt.Println()
	fmt.Printf("  commits tracked:  %d\n", s.TotalCommits)
	fmt.Printf("  sessions:         %d\n", s.Sessions)
	fmt.Printf("  errors recorded:  %d\n", s.TotalErrors)
	fmt.Printf("  violations:       %d\n", s.TotalViolations)

	if len(s.TopErrorCmds) > 0 {
		fmt.Print("\n  " + bold("top error sources:") + "\n")
		for _, c := range s.TopErrorCmds {
			fmt.Printf("    %-28s %d\n", c.Name, c.Count)
		}
	}
	if len(s.TopBranches) > 0 {
		fmt.Print("\n  " + bold("top violating branches:") + "\n")
		for _, c := range s.TopBranches {
			fmt.Printf("    %-28s %d\n", c.Name, c.Count)
		}
	}
	if s.TotalCommits > 0 {
		fmt.Print("\n  " + bold("commit activity by hour (local time):") + "\n  ")
		bars := []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
		maxN := 1
		for _, n := range s.HourDist {
			if n > maxN {
				maxN = n
			}
		}
		for h, n := range s.HourDist {
			idx := 0
			if n > 0 {
				idx = 1 + (n*(len(bars)-2))/maxN
				if idx >= len(bars) {
					idx = len(bars) - 1
				}
			}
			fmt.Print(bars[idx])
			if h < 23 {
				fmt.Print(" ")
			}
		}
		fmt.Println()
		fmt.Println("  00 02 04 06 08 10 12 14 16 18 20 22")
	}
	fmt.Println()
}

func runLFS(args []string) {
	g := git.New()
	ctx := context.Background()

	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "", "--status":
		out, err := g.LFSStatus(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs: git lfs may not be installed:", err)
			os.Exit(1)
		}
		if strings.TrimSpace(out) == "" {
			fmt.Println("git lfs is active - nothing pending")
		} else {
			fmt.Print(out)
		}
	case "--track":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: bonsai lfs --track <pattern>")
			os.Exit(1)
		}
		if err := g.LFSTrack(ctx, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs --track:", err)
			os.Exit(1)
		}
		fmt.Printf("tracking %s via git lfs\n", args[1])
	case "--untrack":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: bonsai lfs --untrack <pattern>")
			os.Exit(1)
		}
		if err := g.LFSUntrack(ctx, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs --untrack:", err)
			os.Exit(1)
		}
		fmt.Printf("untracked %s from git lfs\n", args[1])
	case "--pull":
		if err := g.LFSPull(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs --pull:", err)
			os.Exit(1)
		}
		fmt.Println("lfs objects downloaded")
	case "--install":
		if err := g.LFSInstall(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs --install:", err)
			os.Exit(1)
		}
		fmt.Println("git lfs hooks installed")
	default:
		fmt.Fprintf(os.Stderr, "bonsai lfs: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: bonsai lfs [--status|--track <pat>|--untrack <pat>|--pull|--install]")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// bonsai agent / structured output commands
// ---------------------------------------------------------------------------

func printJSON(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai: json:", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// agentResolveFormat returns the output format for an agent command.
// Priority: --format= flag > config Agent.DefaultFormat > "json".
func agentResolveFormat(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "--format=") {
			f := strings.TrimPrefix(a, "--format=")
			switch f {
			case "json":
				return "json"
			case "markdown", "md":
				return "markdown"
			case "xml":
				return "xml"
			default:
				fmt.Fprintf(os.Stderr, "bonsai: unsupported format %q (json, markdown, xml)\n", f)
				os.Exit(1)
			}
		}
	}
	if cfg, err := config.Load(); err == nil && cfg.Agent.DefaultFormat != "" {
		switch cfg.Agent.DefaultFormat {
		case "json":
			return "json"
		case "markdown", "md":
			return "markdown"
		case "xml":
			return "xml"
		}
	}
	return "json"
}

func printOutput(format string, v any) {
	switch format {
	case "markdown":
		fmt.Print(agent.FormatMarkdown(v))
	case "xml":
		fmt.Print(agent.FormatXML(v))
	default:
		printJSON(v)
	}
}

func runMCP(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "--install":
			if err := mcp.Install(); err != nil {
				fmt.Fprintln(os.Stderr, "bonsai mcp --install:", err)
				os.Exit(1)
			}
			return
		case "--uninstall":
			if err := mcp.Uninstall(); err != nil {
				fmt.Fprintln(os.Stderr, "bonsai mcp --uninstall:", err)
				os.Exit(1)
			}
			return
		case "--doctor":
			scan, test := false, false
			for _, a := range args[1:] {
				switch a {
				case "--scan":
					scan = true
				case "--test":
					test = true
				}
			}
			mcp.Doctor(scan, test)
			return
		}
	}
	mcp.Run(version)
}

func runAgentContext(args []string) {
	limit := 10
	detailed := false
	var contextLines int
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--limit="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--limit=")); err == nil && n > 0 {
				limit = n
			}
		case a == "--detailed":
			detailed = true
		case strings.HasPrefix(a, "--context="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--context=")); err == nil {
				contextLines = n
			}
		}
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildContext(ctx, g, limit, detailed, contextLines)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai context:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentStatus(args []string) {
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildStatus(ctx, g)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai status:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentLog(args []string) {
	params := agent.LogParams{Limit: 20}
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--limit="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--limit=")); err == nil && n > 0 {
				params.Limit = n
			}
		case a == "--yesterday":
			params.Since = "yesterday"
			params.Until = "today"
		case strings.HasPrefix(a, "--weeks="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--weeks=")); err == nil && n > 0 {
				params.Since = fmt.Sprintf("%d weeks ago", n)
			}
		case strings.HasPrefix(a, "--from="):
			params.Since = strings.TrimPrefix(a, "--from=")
		case strings.HasPrefix(a, "--to="):
			params.Until = strings.TrimPrefix(a, "--to=")
		}
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildLog(ctx, g, params)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai log:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentDiff(args []string) {
	var file string
	var showStaged, showUnstaged, showUntracked, detailed bool
	var contextLines int
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		case a == "--staged":
			showStaged = true
		case a == "--unstaged":
			showUnstaged = true
		case a == "--untracked":
			showUntracked = true
		case a == "--detailed":
			detailed = true
		case strings.HasPrefix(a, "--context="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--context=")); err == nil {
				contextLines = n
			}
		}
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildDiff(ctx, g, file, nil, showStaged, showUnstaged, showUntracked, detailed, contextLines)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai diff:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentBlame(args []string) {
	var file string
	var startLine, endLine int
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		case strings.HasPrefix(a, "--start-line="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--start-line=")); err == nil {
				startLine = n
			}
		case strings.HasPrefix(a, "--end-line="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--end-line=")); err == nil {
				endLine = n
			}
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "usage: bonsai blame --file=<path>")
		os.Exit(1)
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildBlame(ctx, g, file, startLine, endLine)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai blame:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentBranches(args []string) {
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildBranches(ctx, g)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai branches:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentStashList(args []string) {
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildStashList(ctx, g)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai stash-list:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentShow(args []string) {
	ref := "HEAD"
	detailed := false
	var contextLines int
	for _, a := range args {
		switch {
		case a == "--detailed":
			detailed = true
		case strings.HasPrefix(a, "--context="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--context=")); err == nil {
				contextLines = n
			}
		case !strings.HasPrefix(a, "--"):
			ref = a
		}
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildShow(ctx, g, ref, detailed, contextLines)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai show:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runAgentReview(args []string) {
	var base, target string
	var paths []string
	detailed := false
	var contextLines int
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--base="):
			base = strings.TrimPrefix(a, "--base=")
		case strings.HasPrefix(a, "--source="):
			base = strings.TrimPrefix(a, "--source=")
		case strings.HasPrefix(a, "--target="):
			target = strings.TrimPrefix(a, "--target=")
		case strings.HasPrefix(a, "--paths="):
			for _, p := range strings.Split(strings.TrimPrefix(a, "--paths="), ",") {
				if p = strings.TrimSpace(p); p != "" {
					paths = append(paths, p)
				}
			}
		case a == "--detailed":
			detailed = true
		case strings.HasPrefix(a, "--context="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--context=")); err == nil {
				contextLines = n
			}
		}
	}
	ctx := context.Background()
	g := git.New()
	out, err := agent.BuildReview(ctx, g, base, target, true, detailed, contextLines, paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai review:", err)
		os.Exit(1)
	}
	printOutput(agentResolveFormat(args), out)
}

func runSSHShow() {
	key := doctor.FindSSHKey()
	if key == nil {
		fmt.Fprintln(os.Stderr, "no SSH key found in ~/.ssh")
		fmt.Fprintln(os.Stderr, "run: bonsai ssh keygen")
		os.Exit(1)
	}
	pub, err := os.ReadFile(key.PublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bonsai: ssh show: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(pub))
}

func runStandup(args []string) {
	days := 1
	author := ""

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-w" || args[i] == "--week":
			days = 7
		case args[i] == "--days" && i+1 < len(args):
			i++
			if n, err := fmt.Sscanf(args[i], "%d", &days); n != 1 || err != nil {
				days = 1
			}
		case strings.HasPrefix(args[i], "--days="):
			if n, err := fmt.Sscanf(strings.TrimPrefix(args[i], "--days="), "%d", &days); n != 1 || err != nil {
				days = 1
			}
		case args[i] == "-a" || args[i] == "--author":
			if i+1 < len(args) {
				i++
				author = args[i]
			}
		case strings.HasPrefix(args[i], "--author="):
			author = strings.TrimPrefix(args[i], "--author=")
		}
	}

	g := git.New()
	ctx := context.Background()

	if author == "" {
		if name, err := g.ConfigGet(ctx, "user.name"); err == nil && name != "" {
			author = name
		}
	}

	entries, err := g.StandupLog(ctx, author, days)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai standup:", err)
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
	green := func(v string) string {
		if color {
			return "\033[32m" + v + "\033[0m"
		}
		return v
	}

	label := "today"
	if days == 7 {
		label = "last 7 days"
	} else if days > 1 {
		label = fmt.Sprintf("last %d days", days)
	}
	who := author
	if who == "" {
		who = "all authors"
	}
	fmt.Println(bold("bonsai standup") + dim("  "+label+" / "+who))
	fmt.Println()

	if len(entries) == 0 {
		fmt.Println(dim("  no commits found"))
		if days == 1 {
			fmt.Println(dim("  try: bonsai standup --days 7"))
		}
		fmt.Println()
		return
	}

	// Group by date so multi-day output is easy to scan.
	type group struct {
		date    string
		entries []git.StandupEntry
	}
	var groups []group
	for _, e := range entries {
		if len(groups) == 0 || groups[len(groups)-1].date != e.Date {
			groups = append(groups, group{date: e.Date})
		}
		groups[len(groups)-1].entries = append(groups[len(groups)-1].entries, e)
	}

	for _, g := range groups {
		if days > 1 {
			fmt.Println("  " + bold(g.date))
		}
		for _, e := range g.entries {
			fmt.Printf("  %s  %s\n", green(e.Hash), e.Subject)
		}
		if days > 1 {
			fmt.Println()
		}
	}

	if days == 1 {
		fmt.Println()
	}
	total := len(entries)
	suffix := "commit"
	if total != 1 {
		suffix = "commits"
	}
	repo, _ := os.Getwd()
	fmt.Printf("  %s  %s\n", bold(fmt.Sprintf("%d %s", total, suffix)), dim(filepath.Base(repo)))
	if days == 1 {
		fmt.Println(dim("  run 'bonsai standup --days 7' to see the week"))
	}
	fmt.Println()
}

// ---------------------------------------------------------------------------
// bonsai ignore
// ---------------------------------------------------------------------------

func runIgnore(args []string) {
	if len(args) == 0 {
		runIgnoreList()
		return
	}

	sub := args[0]
	rest := args[1:]

	scope := ignore.ScopeLocal
	var langs []string
	var pattern string

	for _, a := range rest {
		switch {
		case a == "--global":
			scope = ignore.ScopeGlobal
		case a == "--exclude":
			scope = ignore.ScopeExclude
		case strings.HasPrefix(a, "--lang="):
			for _, l := range strings.Split(strings.TrimPrefix(a, "--lang="), ",") {
				if l = strings.TrimSpace(l); l != "" {
					langs = append(langs, l)
				}
			}
		case strings.HasPrefix(a, "--lang"):
			// --lang go  (space-separated value handled below)
		case !strings.HasPrefix(a, "--") && pattern == "":
			pattern = a
		}
	}
	// Handle: --lang go  (value as next positional after the flag)
	for i, a := range rest {
		if a == "--lang" && i+1 < len(rest) {
			langs = append(langs, rest[i+1])
		}
	}

	switch sub {
	case "--add":
		if pattern == "" {
			fmt.Fprintln(os.Stderr, "usage: bonsai ignore --add <pattern> [--global|--exclude]")
			os.Exit(1)
		}
		if err := ignore.Add(scope, pattern); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai ignore --add:", err)
			os.Exit(1)
		}
		path, _ := ignore.FilePath(scope)
		fmt.Printf("added %q → %s\n", pattern, path)

	case "--remove":
		if pattern == "" {
			fmt.Fprintln(os.Stderr, "usage: bonsai ignore --remove <pattern> [--global|--exclude]")
			os.Exit(1)
		}
		if err := ignore.Remove(scope, pattern); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai ignore --remove:", err)
			os.Exit(1)
		}
		path, _ := ignore.FilePath(scope)
		fmt.Printf("removed %q from %s\n", pattern, path)

	case "--check":
		if pattern == "" {
			fmt.Fprintln(os.Stderr, "usage: bonsai ignore --check <pattern>")
			os.Exit(1)
		}
		matches, err := ignore.Check(pattern)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai ignore --check:", err)
			os.Exit(1)
		}
		color := isTTY()
		dim := func(s string) string {
			if color {
				return "\033[2m" + s + "\033[0m"
			}
			return s
		}
		if len(matches) == 0 {
			fmt.Printf("no files matched by %q\n", pattern)
			return
		}
		fmt.Printf("%d file(s) matched by %q:\n", len(matches), pattern)
		for _, m := range matches {
			fmt.Printf("  %s\n", dim(m))
		}

	case "--seed":
		n, err := ignore.Seed(scope, langs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai ignore --seed:", err)
			os.Exit(1)
		}
		path, _ := ignore.FilePath(scope)
		if n == 0 {
			fmt.Printf("nothing new to add — %s already has all seed patterns\n", path)
			return
		}
		fmt.Printf("added %d pattern(s) to %s\n", n, path)
		if len(langs) == 0 {
			fmt.Println()
			fmt.Printf("tip: add language patterns with --lang  (supported: %s)\n", strings.Join(ignore.SupportedLangs, ", "))
		}

	default:
		fmt.Fprintf(os.Stderr, "bonsai ignore: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "run 'bonsai ignore' to list patterns or 'bonsai help' for all commands")
		os.Exit(1)
	}
}

func runIgnoreList() {
	global, local, exclude, err := ignore.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai ignore:", err)
		os.Exit(1)
	}

	color := isTTY()
	bold := func(s string) string {
		if color {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}
	dim := func(s string) string {
		if color {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}

	printScope := func(label, path string, entries []ignore.PatternEntry) {
		fmt.Printf("  %s  %s\n", bold(label), dim(path))
		if len(entries) == 0 {
			fmt.Println(dim("    none"))
		} else {
			for _, e := range entries {
				fmt.Printf("    %s\n", e.Pattern)
			}
		}
		fmt.Println()
	}

	globalPath, _ := ignore.GlobalPath()

	fmt.Println(bold("bonsai ignore"))
	fmt.Println()
	printScope("Local", "(.gitignore)", local)
	printScope("Global", fmt.Sprintf("(%s)", globalPath), global)
	printScope("Exclude", "(.git/info/exclude)", exclude)

	fmt.Println(dim("  add:   bonsai ignore --add <pattern> [--global|--exclude]"))
	fmt.Println(dim("  seed:  bonsai ignore --seed [--lang go|node|python|dotnet|java]"))
	fmt.Println(dim("  check: bonsai ignore --check <pattern>"))
}

// ---------------------------------------------------------------------------
// bonsai hooks
// ---------------------------------------------------------------------------

func runHooks(args []string) {
	if len(args) == 0 {
		runHooksList()
		return
	}

	sub := args[0]
	rest := args[1:]

	var scope hooks.Scope
	scopeSet := false
	var name, templateName string
	force := false

	for _, a := range rest {
		switch {
		case a == "--global":
			scope = hooks.ScopeGlobal
			scopeSet = true
		case a == "--shared":
			scope = hooks.ScopeShared
			scopeSet = true
		case a == "--local":
			scope = hooks.ScopeLocal
			scopeSet = true
		case strings.HasPrefix(a, "--template="):
			templateName = strings.TrimPrefix(a, "--template=")
		case a == "--force":
			force = true
		case !strings.HasPrefix(a, "--") && name == "":
			name = a
		}
	}

	switch sub {
	case "--install":
		if err := hooks.Install(); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --install:", err)
			os.Exit(1)
		}
		fmt.Println("bonsai hooks installed")
		fmt.Println()
		fmt.Println("  dispatcher  .git/bonsai-hooks/  (core.hooksPath set locally)")
		fmt.Println("  shared      .githooks/           (commit this directory)")
		fmt.Println("  local       .git/hooks/          (personal, not committed)")
		fmt.Println()
		fmt.Println("add a hook:  bonsai hooks --add commit-msg --shared --template=conventional-commits")
		fmt.Println("templates:   bonsai hooks --templates")

	case "--global-install":
		if err := hooks.InstallGlobal(); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --global-install:", err)
			os.Exit(1)
		}
		globalDir, _ := hooks.GlobalDir()
		fmt.Printf("global hooks: %s\n", globalDir)
		fmt.Println("core.hooksPath set globally — personal hooks run in every repo")
		fmt.Println()
		fmt.Println("add a hook:  bonsai hooks --add pre-commit --global --template=no-debug")

	case "--templates":
		runHooksTemplates()

	case "--add":
		if name == "" {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --add <hook-name> --global|--shared|--local [--template=<name>] [--force]")
			os.Exit(1)
		}
		if !scopeSet {
			fmt.Fprintln(os.Stderr, "bonsai hooks --add: scope required (--global, --shared, or --local)")
			os.Exit(1)
		}
		content := hooks.DefaultScript(name)
		if templateName != "" {
			tpl := hooks.TemplateByName(templateName)
			if tpl == nil {
				fmt.Fprintf(os.Stderr, "bonsai hooks --add: unknown template %q\n", templateName)
				fmt.Fprintln(os.Stderr, "run 'bonsai hooks --templates' to see available templates")
				os.Exit(1)
			}
			content = tpl.Script
		}
		if err := hooks.Add(scope, name, content, force); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --add:", err)
			os.Exit(1)
		}
		dir, _ := hooks.Dir(scope)
		fmt.Printf("added %s → %s\n", name, filepath.Join(dir, name))
		if scope != hooks.ScopeGlobal && !hooks.IsInstalled() {
			fmt.Println()
			fmt.Println("tip: run 'bonsai hooks --install' to activate the dispatcher in this repo")
		}

	case "--remove":
		if name == "" || !scopeSet {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --remove <hook-name> --global|--shared|--local")
			os.Exit(1)
		}
		if err := hooks.Remove(scope, name); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --remove:", err)
			os.Exit(1)
		}
		fmt.Printf("removed %s (%s)\n", name, hooks.ScopeLabel(scope))

	case "--enable":
		if name == "" || !scopeSet {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --enable <hook-name> --global|--shared|--local")
			os.Exit(1)
		}
		if err := hooks.Enable(scope, name); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --enable:", err)
			os.Exit(1)
		}
		fmt.Printf("enabled %s (%s)\n", name, hooks.ScopeLabel(scope))

	case "--disable":
		if name == "" || !scopeSet {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --disable <hook-name> --global|--shared|--local")
			os.Exit(1)
		}
		if err := hooks.Disable(scope, name); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --disable:", err)
			os.Exit(1)
		}
		fmt.Printf("disabled %s (%s)\n", name, hooks.ScopeLabel(scope))

	case "--show":
		if name == "" || !scopeSet {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --show <hook-name> --global|--shared|--local")
			os.Exit(1)
		}
		content, err := hooks.Show(scope, name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --show:", err)
			os.Exit(1)
		}
		fmt.Print(content)

	case "--edit":
		if name == "" || !scopeSet {
			fmt.Fprintln(os.Stderr, "usage: bonsai hooks --edit <hook-name> --global|--shared|--local")
			os.Exit(1)
		}
		dir, err := hooks.Dir(scope)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bonsai hooks --edit:", err)
			os.Exit(1)
		}
		cfg, _ := config.Load()
		openInEditor(config.ResolveEditor(cfg), filepath.Join(dir, name))

	default:
		fmt.Fprintf(os.Stderr, "bonsai hooks: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "run 'bonsai hooks' to list hooks or 'bonsai help' for all commands")
		os.Exit(1)
	}
}

func runHooksList() {
	global, shared, local, installed, err := hooks.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "bonsai hooks:", err)
		os.Exit(1)
	}

	color := isTTY()
	bold := func(s string) string {
		if color {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}
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
	dim := func(s string) string {
		if color {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}

	printScope := func(label, path string, entries []hooks.HookEntry) {
		fmt.Printf("  %s  %s\n", bold(label), dim(path))
		if len(entries) == 0 {
			fmt.Println(dim("    none"))
		} else {
			for _, e := range entries {
				if e.Active {
					fmt.Printf("    %s  %s\n", green("✓"), e.Name)
				} else {
					fmt.Printf("    %s  %s\n", yellow("✗"), e.Name+dim(" (not executable)"))
				}
			}
		}
		fmt.Println()
	}

	fmt.Println(bold("bonsai hooks"))
	fmt.Println()

	fmt.Print("  dispatcher  ")
	if installed {
		fmt.Println(green("installed") + dim("  (.git/bonsai-hooks/ → core.hooksPath)"))
	} else {
		fmt.Println(yellow("not installed") + dim("  run: bonsai hooks --install"))
	}
	fmt.Println()

	globalDir, _ := hooks.GlobalDir()
	printScope("Global", fmt.Sprintf("(%s)", globalDir), global)
	printScope("Shared", "(.githooks/)", shared)
	printScope("Local", "(.git/hooks/)", local)

	fmt.Println(dim("  add:        bonsai hooks --add <name> --shared --template=<template>"))
	fmt.Println(dim("  templates:  bonsai hooks --templates"))
}

func runHooksTemplates() {
	color := isTTY()
	bold := func(s string) string {
		if color {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}
	dim := func(s string) string {
		if color {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}

	fmt.Println(bold("bonsai hooks templates"))
	fmt.Println()
	for _, tpl := range hooks.Templates {
		fmt.Printf("  %-30s %s  %s\n", tpl.Name, dim(tpl.HookName), tpl.Description)
	}
	fmt.Println()
	fmt.Println("use with: bonsai hooks --add <hook-name> --shared --template=<name>")
	fmt.Println(dim("example:  bonsai hooks --add commit-msg --shared --template=conventional-commits"))
}

func runRepo(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai repo --create [--private] <name>")
		fmt.Fprintln(os.Stderr, "       bonsai repo --open")
		os.Exit(1)
	}
	switch args[0] {
	case "--create":
		name := ""
		visibility := "public"
		for _, a := range args[1:] {
			if a == "--private" {
				visibility = "private"
			} else if a == "--internal" {
				visibility = "internal"
			} else if name == "" {
				name = a
			}
		}
		if name == "" {
			fmt.Fprintln(os.Stderr, "bonsai repo --create: a repository name is required")
			os.Exit(1)
		}
		g := git.New()
		ctx := context.Background()
		remoteURL := g.OriginURL(ctx)
		prov := pr.Detect(remoteURL)
		if prov == nil {
			prov = pr.DetectByCLI()
		}
		if prov == nil {
			fmt.Fprintln(os.Stderr, "bonsai repo --create: no supported CLI found (gh or glab)")
			os.Exit(1)
		}
		creator, ok := prov.(pr.RepoCreator)
		if !ok {
			fmt.Fprintln(os.Stderr, "bonsai repo --create: provider does not support repo creation")
			os.Exit(1)
		}
		if err := creator.CreateRepo(ctx, name, visibility); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai repo --create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("repository %s created (%s)\n", name, visibility)
	case "--open":
		g := git.New()
		ctx := context.Background()
		remoteURL := g.OriginURL(ctx)
		prov := pr.Detect(remoteURL)
		if prov == nil {
			fmt.Fprintln(os.Stderr, "bonsai repo --open: no supported provider detected for this remote")
			os.Exit(1)
		}
		if err := prov.Open(ctx, ""); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai repo --open: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "bonsai repo: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}
