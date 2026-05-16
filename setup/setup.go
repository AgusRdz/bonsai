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

	existing, _ := config.LoadFile(".bonsai.toml") // nil if no local config yet

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

	// --- branch prefixes ---
	fmt.Println()
	fmt.Println("branch prefixes (press enter to keep the shown default):")
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
			val := ask(sc, fmt.Sprintf("  %-8s prefix", t.name), prefix)
			if val != "" {
				cfg.Conventions.Branches[t.name] = config.BranchRule{
					Prefix:  val,
					Example: buildExample(val, t.example),
				}
			}
		}

		// custom types - include any existing branches not in the default set
		fmt.Println()
		fmt.Println("additional branch types? (e.g. chore, docs - leave empty to skip)")
		if existing != nil {
			knownTypes := make(map[string]bool)
			for _, t := range types {
				knownTypes[t.name] = true
			}
			for name, rule := range existing.Conventions.Branches {
				if knownTypes[name] {
					continue
				}
				prefix := ask(sc, fmt.Sprintf("  %-8s prefix", name), rule.Prefix)
				if prefix != "" {
					cfg.Conventions.Branches[name] = config.BranchRule{
						Prefix:  prefix,
						Example: buildExample(prefix, "description"),
					}
				}
			}
		}
		for {
			name := ask(sc, "  type name", "")
			if name == "" {
				break
			}
			prefix := ask(sc, fmt.Sprintf("  %s prefix", name), name+"/")
			cfg.Conventions.Branches[name] = config.BranchRule{
				Prefix:  prefix,
				Example: buildExample(prefix, "description"),
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

func githubflowDefaults() []branchDef {
	return []branchDef{
		{name: "feature", prefix: "feat/", example: "login-oauth"},
		{name: "bugfix", prefix: "fix/", example: "crash-on-login"},
	}
}
