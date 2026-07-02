package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AgusRdz/bonsai/config"
)

// RunGlobal runs the interactive setup wizard and writes the result to the
// global config file. Safe to call on first run or explicitly via bonsai setup.
func RunGlobal() error {
	p, err := config.GlobalConfigPath()
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("bonsai setup - configure your global defaults")
	fmt.Println(strings.Repeat("-", 46))

	existing, _ := config.LoadFile(p) // nil on first run

	cfg, err := wizard(false, existing)
	if err != nil {
		return err
	}
	if err := config.Write(p, cfg); err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("config written to %s\n", p)
	fmt.Println("run 'bonsai setup --local' inside a repo to add per-project overrides")
	return nil
}

// RunLocal runs the interactive setup wizard and writes the result to
// .bonsai.toml in the current directory.
func RunLocal() error {
	const path = ".bonsai.toml"
	fmt.Println()
	fmt.Println("bonsai setup local - per-project overrides")
	fmt.Println(strings.Repeat("-", 42))
	fmt.Println("press enter on any question to inherit from your global config")
	fmt.Println()

	existing, _ := config.LoadFile(".bonsai.toml")
	if existing == nil {
		// No local config yet - use global as the baseline so the wizard shows
		// effective values instead of "inherit" for everything.
		if p, err := config.GlobalConfigPath(); err == nil {
			existing, _ = config.LoadFile(p)
		}
	}

	cfg, err := wizard(true, existing)
	if err != nil {
		return err
	}
	if err := config.Write(path, cfg); err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("config written to %s\n", path)
	return nil
}

// wizard collects answers and returns a Config. When local is true, empty
// answers are left as zero-values so the global config takes precedence.
// existing is the previously saved config (nil on first run).
func wizard(local bool, existing *config.Config) (*config.Config, error) {
	sc := bufio.NewScanner(os.Stdin)
	cfg := &config.Config{}

	// --- flow ---
	fmt.Println("workflow flow:")
	fmt.Println("  1) trunk       short-lived branches off main, merge back frequently")
	fmt.Println("  2) gitflow     feature/bugfix/release/hotfix, PRs target develop")
	fmt.Println("  3) githubflow  feature branches, PRs into main")
	fmt.Println("  4) forking     fork-based contributions")
	if local {
		fmt.Println("  5) inherit     use global setting")
	}
	flowDefault := "1"
	if local {
		flowDefault = "5"
	}
	if existing != nil {
		flowDefault = flowTypeToNumber(existing.Flow.Type, local)
	}
	flowChoice := ask(sc, "choice", flowDefault)
	flowMap := map[string]string{
		"1": "trunk",
		"2": "gitflow",
		"3": "githubflow",
		"4": "forking",
		"5": "auto",
	}
	if v, ok := flowMap[flowChoice]; ok {
		cfg.Flow.Type = v
	} else {
		cfg.Flow.Type = flowMap[flowDefault]
	}

	// --- ticket IDs ---
	fmt.Println()
	fmt.Println("ticket IDs in branch names:")
	fmt.Println("  if your team includes a ticket reference, e.g.")
	fmt.Println("    feat/RES-123-login-oauth")
	fmt.Println("    fix/PROJ-456-crash-on-login")
	fmt.Println("  enter your project key (e.g. RES, PROJ, APP).")
	fmt.Println("  leave empty to skip.")
	ticketKey := ""
	if existing != nil {
		ticketKey = inferTicketKey(existing.Conventions.Branches)
	}
	ticketKey = ask(sc, "project key (e.g. RES, PROJ)", ticketKey)

	// --- branch prefixes ---
	fmt.Println()
	if ticketKey != "" {
		fmt.Printf("branch types — your format: {prefix}%s-{number}-{description}\n", ticketKey)
	} else {
		fmt.Println("branch types — your format: {prefix}{description}")
	}
	fmt.Println("press enter to keep the shown default:")
	cfg.Conventions.Branches = make(map[string]config.BranchRule)

	var types []branchDef
	switch cfg.Flow.Type {
	case "gitflow":
		types = gitflowDefaults()
	case "trunk":
		types = trunkDefaults()
	case "githubflow":
		types = githubflowDefaults()
	case "forking":
		types = githubflowDefaults() // same structure as githubflow
	default:
		// local "inherit" - still ask so the user can override
		types = trunkDefaults()
	}

	if !local || cfg.Flow.Type != "auto" {
		for _, t := range types {
			prefix := t.prefix
			if existing != nil {
				if rule, ok := existing.Conventions.Branches[t.name]; ok && rule.Prefix != "" {
					prefix = rule.Prefix
				}
			}
			fmt.Printf("  %-10s → e.g. %s\n", t.name, buildExampleWithTicket(prefix, ticketKey, t.example))
			val := ask(sc, fmt.Sprintf("  %-10s prefix", t.name), prefix)
			if val != "" {
				cfg.Conventions.Branches[t.name] = config.BranchRule{
					Prefix:  val,
					Pattern: buildPatternWithTicket(val, ticketKey),
					Example: buildExampleWithTicket(val, ticketKey, t.example),
				}
			}
		}

		// --- common extra types (conventional commits style) ---
		fmt.Println()
		fmt.Println("extra branch types (conventional commits):")
		fmt.Println("  enable the ones your team uses [y/n]:")
		commonExtras := []branchDef{
			{name: "chore", prefix: "chore/", example: "update-deps"},
			{name: "refactor", prefix: "refactor/", example: "extract-auth-service"},
			{name: "test", prefix: "test/", example: "add-login-tests"},
			{name: "docs", prefix: "docs/", example: "update-readme"},
			{name: "ci", prefix: "ci/", example: "add-ci-pipeline"},
		}
		knownTypes := make(map[string]bool)
		for _, t := range types {
			knownTypes[t.name] = true
		}
		for _, t := range commonExtras {
			if knownTypes[t.name] {
				continue
			}
			defaultAnswer := "n"
			defaultPrefix := t.prefix
			if existing != nil {
				if rule, ok := existing.Conventions.Branches[t.name]; ok && rule.Prefix != "" {
					defaultAnswer = "y"
					defaultPrefix = rule.Prefix
				}
			}
			fmt.Printf("  %-10s → e.g. %s\n", t.name, buildExampleWithTicket(defaultPrefix, ticketKey, t.example))
			answer := ask(sc, fmt.Sprintf("  enable %-10s [y/n]", t.name), defaultAnswer)
			if answer == "y" || answer == "yes" {
				prefix := ask(sc, "    prefix", defaultPrefix)
				if prefix != "" {
					cfg.Conventions.Branches[t.name] = config.BranchRule{
						Prefix:  prefix,
						Pattern: buildPatternWithTicket(prefix, ticketKey),
						Example: buildExampleWithTicket(prefix, ticketKey, t.example),
					}
				}
			}
		}

		// existing custom types not in the standard or common set
		if existing != nil {
			allKnown := make(map[string]bool)
			for _, t := range types {
				allKnown[t.name] = true
			}
			for _, t := range commonExtras {
				allKnown[t.name] = true
			}
			for name, rule := range existing.Conventions.Branches {
				if allKnown[name] {
					continue
				}
				fmt.Printf("  %-10s → e.g. %s\n", name, buildExampleWithTicket(rule.Prefix, ticketKey, "description"))
				prefix := ask(sc, fmt.Sprintf("  %-10s prefix", name), rule.Prefix)
				if prefix != "" {
					cfg.Conventions.Branches[name] = config.BranchRule{
						Prefix:  prefix,
						Pattern: buildPatternWithTicket(prefix, ticketKey),
						Example: buildExampleWithTicket(prefix, ticketKey, "description"),
					}
				}
			}
		}

		// free-form custom types
		fmt.Println()
		fmt.Println("  any other custom types? (leave name empty to finish)")
		for {
			name := ask(sc, "  type name", "")
			if name == "" {
				break
			}
			defaultPrefix := name + "/"
			fmt.Printf("    → e.g. %s\n", buildExampleWithTicket(defaultPrefix, ticketKey, "description"))
			prefix := ask(sc, fmt.Sprintf("  %s prefix", name), defaultPrefix)
			cfg.Conventions.Branches[name] = config.BranchRule{
				Prefix:  prefix,
				Pattern: buildPatternWithTicket(prefix, ticketKey),
				Example: buildExampleWithTicket(prefix, ticketKey, "description"),
			}
		}
	}

	// --- mode ---
	fmt.Println()
	fmt.Println("mode:")
	fmt.Println("  1) standard  shows the command that ran after each action")
	fmt.Println("  2) guided    full explanations after every action (new to git)")
	fmt.Println("  3) pro       no feedback panel")
	if local {
		fmt.Println("  4) inherit   use global setting")
	}
	modeDefault := "1"
	if local {
		modeDefault = "4"
	}
	if existing != nil {
		modeDefault = modeToNumber(existing.Modes.Default, local)
	}
	modeChoice := ask(sc, "choice", modeDefault)
	modeMap := map[string]string{
		"1": "standard",
		"2": "guided",
		"3": "pro",
		"4": "",
	}
	if v, ok := modeMap[modeChoice]; ok {
		cfg.Modes.Default = v
	} else {
		cfg.Modes.Default = modeMap[modeDefault]
	}

	// --- validation ---
	fmt.Println()
	fmt.Println("branch name validation:")
	fmt.Println("  1) strict  blocks the action if the name doesn't match configured prefixes")
	fmt.Println("  2) warn    shows a warning but doesn't block")
	fmt.Println("  3) off     no validation")
	if local {
		fmt.Println("  4) inherit use global setting")
	}
	valDefault := "2"
	if local {
		valDefault = "4"
	}
	if existing != nil {
		valDefault = validationToNumber(existing.Conventions.Validation.Mode, local)
	}
	valChoice := ask(sc, "choice", valDefault)
	valMap := map[string]string{
		"1": "strict",
		"2": "warn",
		"3": "off",
		"4": "",
	}
	if v, ok := valMap[valChoice]; ok {
		cfg.Conventions.Validation.Mode = v
	} else {
		cfg.Conventions.Validation.Mode = valMap[valDefault]
	}

	// --- editor (global only) ---
	if !local {
		fmt.Println()
		fmt.Println("preferred editor command (used by 'bonsai config')")
		fmt.Println("leave empty to use $VISUAL / $EDITOR / vi")
		editorDefault := ""
		if existing != nil {
			editorDefault = existing.Editor.Command
		}
		cfg.Editor.Command = ask(sc, "editor", editorDefault)
	}

	// --- agent / AI output (global only) ---
	if !local {
		fmt.Println()
		fmt.Println("AI coding assistants (Claude, Copilot, Cursor, etc.)")
		usesAgentDefault := "n"
		if existing != nil && existing.Agent.DefaultFormat != "" {
			usesAgentDefault = "y"
		}
		usesAgent := ask(sc, "do you use AI agents? [y/n]", usesAgentDefault)
		if usesAgent == "y" || usesAgent == "yes" {
			fmt.Println()
			fmt.Println("output format for agent commands:")
			fmt.Println("  1) json      universal, most AI tools parse it natively")
			fmt.Println("  2) markdown  readable in chat interfaces (Claude, ChatGPT)")
			fmt.Println("  3) xml       for tools that prefer structured XML")
			fmtDefault := "1"
			if existing != nil {
				fmtDefault = agentFormatToNumber(existing.Agent.DefaultFormat)
			}
			fmtChoice := ask(sc, "choice", fmtDefault)
			fmtMap := map[string]string{"1": "json", "2": "markdown", "3": "xml"}
			if v, ok := fmtMap[fmtChoice]; ok {
				cfg.Agent.DefaultFormat = v
			} else {
				cfg.Agent.DefaultFormat = fmtMap[fmtDefault]
			}
		}
	}

	// --- overview ---
	fmt.Println()
	fmt.Println("overview panel (shown when working tree is clean):")
	fmt.Println("  shows open PRs and recent commits - requires a PR provider (gh/glab/bb)")
	overviewDefault := "y"
	if local {
		overviewDefault = "inherit"
	}
	if existing != nil && existing.Overview.Enabled != nil {
		if *existing.Overview.Enabled {
			overviewDefault = "y"
		} else {
			overviewDefault = "n"
		}
	}
	overviewChoices := "[y/n]"
	if local {
		overviewChoices = "[y/n/inherit]"
	}
	overviewAnswer := ask(sc, "enable overview "+overviewChoices, overviewDefault)
	switch overviewAnswer {
	case "y", "yes":
		t := true
		cfg.Overview.Enabled = &t
	case "n", "no":
		f := false
		cfg.Overview.Enabled = &f
		// "inherit" or empty: leave nil so global config takes precedence
	}

	// --- PR merge method ---
	fmt.Println()
	fmt.Println("default PR merge method:")
	fmt.Println("  1) always ask  show the picker every time")
	fmt.Println("  2) merge       keep all commits, add a merge commit")
	fmt.Println("  3) squash      squash into one commit on base branch")
	fmt.Println("  4) rebase      rebase commits onto base branch")
	if local {
		fmt.Println("  5) inherit     use global setting")
	}
	mergeDefault := "1"
	if local {
		mergeDefault = "5"
	}
	if existing != nil {
		mergeDefault = mergeMethodToNumber(existing.PR.MergeMethod, local)
	}
	mergeChoice := ask(sc, "choice", mergeDefault)
	mergeMap := map[string]string{
		"1": "",
		"2": "merge",
		"3": "squash",
		"4": "rebase",
		"5": "",
	}
	if v, ok := mergeMap[mergeChoice]; ok {
		cfg.PR.MergeMethod = v
	} else {
		cfg.PR.MergeMethod = mergeMap[mergeDefault]
	}

	// Fill required fields that weren't set so config.Write produces valid TOML.
	if !local {
		if cfg.Modes.Default == "" {
			cfg.Modes.Default = "standard"
		}
		if cfg.Conventions.Validation.Mode == "" {
			cfg.Conventions.Validation.Mode = "warn"
		}
		if cfg.Flow.Type == "" {
			cfg.Flow.Type = "trunk"
		}
		// Wizard doesn't ask about keybindings; always write the defaults so
		// the config file doesn't end up with empty strings that break the TUI.
		cfg.Keybindings = config.DefaultKeybindings()
	}

	return cfg, nil
}

func flowTypeToNumber(flowType string, local bool) string {
	switch flowType {
	case "trunk":
		return "1"
	case "gitflow":
		return "2"
	case "githubflow":
		return "3"
	case "forking":
		return "4"
	case "auto":
		if local {
			return "5"
		}
		return "1"
	default:
		if local {
			return "5"
		}
		return "1"
	}
}

func modeToNumber(mode string, local bool) string {
	switch mode {
	case "standard":
		return "1"
	case "guided":
		return "2"
	case "pro":
		return "3"
	case "":
		if local {
			return "4"
		}
		return "1"
	default:
		return "1"
	}
}

func validationToNumber(mode string, local bool) string {
	switch mode {
	case "strict":
		return "1"
	case "warn":
		return "2"
	case "off":
		return "3"
	case "":
		if local {
			return "4"
		}
		return "2"
	default:
		return "2"
	}
}

// ask prints the prompt with the default value and reads a line from sc.
// Returns the trimmed input, or def if the line is empty.
func ask(sc *bufio.Scanner, label, def string) string {
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

type branchDef struct {
	name    string
	prefix  string
	example string
}

func buildExample(prefix, suffix string) string {
	return prefix + suffix
}

func trunkDefaults() []branchDef {
	return []branchDef{
		{name: "feature", prefix: "feat/", example: "login-oauth"},
		{name: "bugfix", prefix: "bug/", example: "crash-on-login"},
		{name: "hotfix", prefix: "hotfix/", example: "critical-fix"},
	}
}

func gitflowDefaults() []branchDef {
	return []branchDef{
		{name: "feature", prefix: "feat/", example: "PROJ-123-login-oauth"},
		{name: "bugfix", prefix: "fix/", example: "PROJ-456-crash-on-login"},
		{name: "release", prefix: "release/", example: "1.2.0"},
		{name: "hotfix", prefix: "hotfix/", example: "critical-fix"},
	}
}

func agentFormatToNumber(format string) string {
	switch format {
	case "json":
		return "1"
	case "markdown":
		return "2"
	case "xml":
		return "3"
	default:
		return "1"
	}
}

func mergeMethodToNumber(method string, local bool) string {
	switch method {
	case "merge":
		return "2"
	case "squash":
		return "3"
	case "rebase":
		return "4"
	default: // "" = always ask
		if local {
			return "5"
		}
		return "1"
	}
}

func githubflowDefaults() []branchDef {
	return []branchDef{
		{name: "feature", prefix: "feat/", example: "login-oauth"},
		{name: "bugfix", prefix: "fix/", example: "crash-on-login"},
	}
}

// buildExampleWithTicket returns a full branch name example incorporating a
// ticket key if one is configured (e.g. "feat/RES-123-login-oauth").
func buildExampleWithTicket(prefix, ticketKey, suffix string) string {
	if ticketKey != "" {
		return prefix + ticketKey + "-123-" + suffix
	}
	return prefix + suffix
}

// buildPatternWithTicket returns the pattern string for a branch rule.
func buildPatternWithTicket(prefix, ticketKey string) string {
	if ticketKey != "" {
		return prefix + ticketKey + "-{number}-{description}"
	}
	return prefix + "{description}"
}

// inferTicketKey guesses the project key from existing branch examples.
// It looks for a pattern like "PREFIX/KEY-NNN-" in stored examples.
func inferTicketKey(branches map[string]config.BranchRule) string {
	for _, rule := range branches {
		if rule.Example == "" {
			continue
		}
		// strip prefix
		rest := rule.Example
		if i := strings.Index(rest, "/"); i >= 0 {
			rest = rest[i+1:]
		}
		// rest should be KEY-NNN-description; find first "-"
		dash := strings.Index(rest, "-")
		if dash <= 0 {
			continue
		}
		key := rest[:dash]
		// sanity: all uppercase letters
		if key == strings.ToUpper(key) && len(key) >= 2 {
			return key
		}
	}
	return ""
}
