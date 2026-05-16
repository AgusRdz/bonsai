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

	if err := tui.Run(cfg, mdb); err != nil {
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

  setup             interactive first-run setup (global config)
  setup --local     per-project .bonsai.toml setup wizard
  init              create a .bonsai.toml template in the current directory
  config            open global config in your editor
  config --local    open (or create) per-project .bonsai.toml in your editor
  config --path     print the path to the global config file

  doctor            check global and local git configuration health
  doctor --verbose  same, with a one-line explanation per check
  stats             show repository statistics
  standup           show your recent commits (today by default)
  standup --days N  show commits from the last N days (default: 1)
  standup -w        shorthand for --days 7 (the whole week)
  standup -a name   filter by author name (default: you)
  metrics           show locally tracked habit and error metrics

  patch create      create .patch files from commits (git format-patch)
  patch apply       apply a .patch file (git am)
  archive           export repo snapshot as tar.gz or zip
  bundle create     pack refs into a portable bundle file
  bundle verify     verify a bundle file

  ssh status        show SSH key and agent status
  ssh keygen        generate a new SSH key pair
  ssh show          print your SSH public key

  lfs status        show pending git lfs objects
  lfs track <pat>   track a file pattern via git lfs (.gitattributes)
  lfs untrack <pat> stop tracking a pattern
  lfs pull          download all lfs objects for the current checkout
  lfs install       install lfs hooks into this repository

Options:
  -h, --help        show help
  -v, --version     print version
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
		fmt.Fprintln(os.Stderr, "usage: bonsai ssh status")
		fmt.Fprintln(os.Stderr, "       bonsai ssh keygen")
		fmt.Fprintln(os.Stderr, "       bonsai ssh show")
		os.Exit(1)
	}
	switch args[0] {
	case "status":
		runSSHStatus()
	case "keygen":
		runSSHKeygen()
	case "show":
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
	case "", "status":
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
	case "track":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: bonsai lfs track <pattern>")
			os.Exit(1)
		}
		if err := g.LFSTrack(ctx, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs track:", err)
			os.Exit(1)
		}
		fmt.Printf("tracking %s via git lfs\n", args[1])
	case "untrack":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: bonsai lfs untrack <pattern>")
			os.Exit(1)
		}
		if err := g.LFSUntrack(ctx, args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs untrack:", err)
			os.Exit(1)
		}
		fmt.Printf("untracked %s from git lfs\n", args[1])
	case "pull":
		if err := g.LFSPull(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs pull:", err)
			os.Exit(1)
		}
		fmt.Println("lfs objects downloaded")
	case "install":
		if err := g.LFSInstall(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "bonsai lfs install:", err)
			os.Exit(1)
		}
		fmt.Println("git lfs hooks installed")
	default:
		fmt.Fprintf(os.Stderr, "bonsai lfs: unknown subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "usage: bonsai lfs [status|track <pat>|untrack <pat>|pull|install]")
		os.Exit(1)
	}
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

func runRepo(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bonsai repo create [--private] <name>")
		fmt.Fprintln(os.Stderr, "       bonsai repo open")
		os.Exit(1)
	}
	switch args[0] {
	case "create":
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
			fmt.Fprintln(os.Stderr, "bonsai repo create: a repository name is required")
			os.Exit(1)
		}
		// Detect provider from origin URL (best-effort).
		g := git.New()
		ctx := context.Background()
		remoteURL := g.OriginURL(ctx)
		prov := pr.Detect(remoteURL)
		if prov == nil {
			prov = pr.DetectByCLI()
		}
		if prov == nil {
			fmt.Fprintln(os.Stderr, "bonsai repo create: no supported CLI found (gh or glab)")
			os.Exit(1)
		}
		creator, ok := prov.(pr.RepoCreator)
		if !ok {
			fmt.Fprintln(os.Stderr, "bonsai repo create: provider does not support repo creation")
			os.Exit(1)
		}
		if err := creator.CreateRepo(ctx, name, visibility); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai repo create: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("repository %s created (%s)\n", name, visibility)
	case "open":
		g := git.New()
		ctx := context.Background()
		remoteURL := g.OriginURL(ctx)
		prov := pr.Detect(remoteURL)
		if prov == nil {
			fmt.Fprintln(os.Stderr, "bonsai repo open: no supported provider detected for this remote")
			os.Exit(1)
		}
		// Open the repo URL - use pr.Open with empty string to open the repo root.
		if err := prov.Open(ctx, ""); err != nil {
			fmt.Fprintf(os.Stderr, "bonsai repo open: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "bonsai repo: unknown subcommand %q\n", args[0])
		os.Exit(1)
	}
}
