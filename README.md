# bonsai

> A TUI Git client that teaches while you work.

bonsai wraps the official Git binary and adds an interactive terminal interface. After every action it shows what happened in plain language, the exact Git command that ran, and an option to copy it. The goal is that a user who starts as a novice eventually understands Git well enough to not need bonsai - or keeps using it simply because it is a pleasure to use.

## Install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.ps1 | iex
```

## Commands

| Command | Description |
|---------|-------------|
| `bonsai` | Open the interactive TUI |
| `bonsai help` | Show help |
| `bonsai version` | Print version |
| `bonsai update` | Update to the latest release |
| `bonsai uninstall` | Remove bonsai from this system |
| `bonsai changelog` | Show the changelog |
| `bonsai init` | Create a `.bonsai.toml` template in the current directory |
| `bonsai config` | Open global config in your editor |
| `bonsai config local` | Open (or create) per-project `.bonsai.toml` in your editor |
| `bonsai config global` | Same as `bonsai config` |
| `bonsai config path` | Print the path to the global config file |

## TUI keybindings

Default keybindings. All are remappable via `[keybindings]` in your config.

| Key | Action |
|-----|--------|
| `space` | Stage / unstage the selected file |
| `d` | View diff for the selected file |
| `D` | Discard working tree changes for the selected file |
| `c` | Open commit panel |
| `p` | Push to remote |
| `P` | Pull from remote |
| `s` | Stash all changes |
| `S` | View stash list and pop an entry |
| `b` | Create a new branch (flow picker shown in gitflow mode) |
| `B` | Switch to another branch |
| `g` | View commit log (recent 20) |
| `z` | Undo: unstage or restore last staged file |
| `?` | Help panel (full keybinding reference) |
| `q` / `ctrl+c` | Quit |

The current branch, ahead/behind counts, active flow, and mode are always visible in the header on the main screen.

## Modes

| Mode | Behavior |
|------|----------|
| `novice` | Full explanations after every action, contextual tips, Git command always visible |
| `learning` | Shows what will happen before executing, asks for confirmation |
| `pro` | Education panel hidden; clean interface with no explanations |

**How to change mode:** run `bonsai config` to open the global config and set `modes.default`. To change it for one project only, run `bonsai config local`.

```toml
[modes]
default = "novice"   # novice | pro | learning
```

## Workflow flows

bonsai adapts its hints and branch picker to your team's workflow.

| Flow | Description |
|------|-------------|
| `auto` | Detected from your branch conventions (default) |
| `gitflow` | feature/bugfix/release/hotfix branches; PRs target `develop` |
| `trunk` | Short-lived branches merged back to `main` frequently |
| `githubflow` | Feature branches with PRs directly into `main` |
| `forking` | Fork-based contribution model |

When `auto` is set and all four gitflow branch types (feature, bugfix, release, hotfix) are configured, bonsai detects gitflow automatically.

```toml
[flow]
type = "auto"   # auto | gitflow | trunk | githubflow | forking
```

## Editor

bonsai uses your editor for `bonsai config` and `bonsai config local`. It resolves in this order:

1. `editor.command` in your config
2. `$VISUAL` environment variable
3. `$EDITOR` environment variable
4. `vi` as a last resort

```toml
[editor]
command = "vim"   # or "nano", "code --wait", "hx", etc.
```

To set the editor used by `git commit` and `git merge`, configure Git directly:

```sh
git config --global core.editor "vim"
```

## Configuration

Global config at `~/.config/bonsai/config.toml`. Per-project config at `.bonsai.toml` in the repo root. Project config inherits from global and overrides selectively.

Run `bonsai init` to create a commented `.bonsai.toml` template in the current directory.

### Full config reference

```toml
[modes]
default = "novice"   # novice | pro | learning

[flow]
type = "auto"        # auto | gitflow | trunk | githubflow | forking

[conventions.branches.feature]
prefix  = "feat/"
pattern = "feat/{ticket-id}-{description}"
example = "feat/PROJ-123-login-oauth"

[conventions.branches.bugfix]
prefix  = "fix/"
pattern = "fix/{ticket-id}-{description}"
example = "fix/PROJ-456-crash-on-login"

[conventions.branches.release]
prefix = "release/"

[conventions.branches.hotfix]
prefix = "hotfix/"

[conventions.validation]
mode = "strict"   # strict | warn | off
# strict: blocks the action and shows the convention panel
# warn:   shows a warning in the status bar but does not block
# off:    no validation

[education]
panel_duration = 4   # seconds the education panel stays visible; 0 disables it

[editor]
command = ""   # e.g. "vim", "nano", "code --wait"

[keybindings]
graph  = "g"
commit = "c"
branch = "b"
push   = "p"
pull   = "l"
stash  = "s"
undo   = "z"
quit   = "q"

[metrics]
enabled = false

  [metrics.track]
  errors      = true
  conventions = true
  commits     = false
  habits      = false
```

## Requirements

- Git installed on the system

## Philosophy

- bonsai never takes autonomous decisions
- bonsai never hides the underlying Git command
- bonsai never locks you into a workflow
- bonsai's success is measured by your independence, not your dependency

## License

MIT
