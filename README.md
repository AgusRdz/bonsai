# bonsai

> A TUI Git client that teaches while you work.

bonsai wraps the official Git binary and adds an interactive terminal interface. After every action it shows what happened in plain language and the exact Git command that ran. The goal is that a user who starts as a beginner eventually understands Git well enough to not need bonsai - or keeps using it simply because it is a pleasure to use.

```
 main  ↑1  [gitflow]  [mode:standard]

  Conflicts (1)
  !  src/auth.go           both modified

  Staged (2)
  M  src/api.go
  A  src/middleware.go

  Changed (1)
  M  README.md

  [space] stage/unstage  [c] commit  [p] push  [P] pull  [f] fetch  [l] log  [?] help  [q] quit
```

## Install

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/AgusRdz/bonsai/main/install.ps1 | iex
```

**From source**

```sh
git clone https://github.com/AgusRdz/bonsai.git
cd bonsai
go build -o bonsai .
```

Requires Git 2.28 or later.

## Quick start

```sh
bonsai setup          # interactive wizard - configure flow, conventions, and mode
cd your-repo
bonsai                # open the TUI
```

On first run bonsai opens the setup wizard automatically.

## CLI commands

| Command | Description |
|---------|-------------|
| `bonsai` | Open the interactive TUI |
| `bonsai help` | Show help |
| `bonsai version` | Print version |
| `bonsai update` | Update to the latest release |
| `bonsai uninstall` | Remove bonsai from this system |
| `bonsai changelog` | Show the changelog |
| `bonsai setup` | Interactive wizard - global config |
| `bonsai setup --local` | Interactive wizard - per-project `.bonsai.toml` |
| `bonsai init` | Create a commented `.bonsai.toml` template (no wizard) |
| `bonsai config` | Open global config in your editor |
| `bonsai config --local` | Open (or create) per-project `.bonsai.toml` |
| `bonsai config --path` | Print the path to the global config file |
| `bonsai doctor` | Check global and local git configuration health |
| `bonsai doctor --verbose` | Same, with a one-line explanation per check |
| `bonsai stats` | Repository statistics (commits, contributors, file types) |
| `bonsai patch create --base=<ref>` | Create `.patch` files from commits (git format-patch) |
| `bonsai patch apply <file>` | Apply a `.patch` file (git am) |
| `bonsai archive` | Export the repo as `tar.gz` (default) or `zip` |
| `bonsai bundle create <file>` | Pack refs into a portable bundle |
| `bonsai bundle verify <file>` | Verify a bundle file |

### Examples

```sh
# health check before a release
bonsai doctor --verbose

# repo statistics
bonsai stats

# export a patch set for email review
bonsai patch create --base=main --output=patches/

# apply patches from a contributor
bonsai patch apply patches/0001-*.patch

# archive the current HEAD as a zip
bonsai archive --format=zip --output=release.zip

# bundle the whole repo for offline transfer
bonsai bundle create repo.bundle
```

## TUI key bindings

Open bonsai and press `?` to see the full in-app reference.

### Main panel

| Key | Action |
|-----|--------|
| `space` | Stage / unstage the selected file |
| `d` | View diff for the selected file |
| `x` | Discard working tree changes for the selected file |
| `o` | Restore a file to HEAD or any ref |
| `c` | Open commit panel |
| `p` | Push to remote |
| `P` | Pull from remote |
| `f` | Fetch menu (origin, --prune, --all) |
| `l` | Commit log |
| `b` | Create / switch branch (flow picker in gitflow mode) |
| `B` | List all branches |
| `s` | Stash all changes |
| `S` | Stash list - pop, apply, drop |
| `z` | Reset menu (soft / mixed / hard) |
| `t` | Tag list - create, delete |
| `e` | Blame for the selected file |
| `i` | Bisect panel |
| `R` | Interactive rebase |
| `A` | Amend HEAD (message, author, date, --no-edit) |
| `W` | Worktree list - add, remove |
| `O` | Remote management - list, add, remove, rename |
| `M` | Submodule management - list, add, update, deinit |
| `n` | View / edit / delete git note for HEAD |
| `L` | Reflog - scroll history, reset to entry |
| `X` | Clean untracked files (preview + confirm) |
| `C` | Configuration manager |
| `a` | Abort in-progress merge / rebase / cherry-pick |
| `?` | Help panel |
| `q` / `ctrl+c` | Quit |

### Log panel

| Key | Action |
|-----|--------|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `enter` | Show commit details |
| `m` | Merge selected commit |
| `ctrl+/` or `ctrl+r` | Search / filter commits |
| `esc` | Back |

### Conflict panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `o` | Accept ours |
| `t` | Accept theirs |
| `r` | Remove conflict markers (keep both) |
| `esc` | Back |

All keybindings for `commit`, `push`, `pull`, `stash`, `undo`, and `quit` are remappable via `[keybindings]` in your config.

## Modes

| Mode | Behavior |
|------|----------|
| `standard` | Shows the command that ran after each action - default |
| `guided` | Full explanation after every action, contextual tips (new to Git) |
| `pro` | No feedback panel - clean interface |

Change mode globally:

```sh
bonsai config
```

```toml
[modes]
default = "standard"   # standard | guided | pro
```

Override for one project:

```sh
bonsai config --local
```

## Workflow flows

bonsai adapts its branch picker and hints to your team's workflow.

| Flow | Description |
|------|-------------|
| `auto` | Detected from your branch conventions (default) |
| `trunk` | Short-lived branches merged frequently into `main` |
| `gitflow` | feature / bugfix / release / hotfix; PRs target `develop` |
| `githubflow` | Feature branches with PRs directly into `main` |
| `forking` | Fork-based contribution model |

```toml
[flow]
type = "auto"   # auto | trunk | gitflow | githubflow | forking
```

## Branch conventions

Bonsai can validate branch names against prefixes you define and warn or block when a name does not match.

```toml
[conventions.branches.feature]
prefix  = "feat/"
pattern = "feat/{ticket-id}-{description}"
example = "feat/PROJ-123-login-oauth"

[conventions.branches.bugfix]
prefix = "fix/"
example = "fix/PROJ-456-crash-on-login"

[conventions.validation]
mode = "strict"   # strict | warn | off
```

Run `bonsai setup` to configure conventions interactively.

## Doctor

`bonsai doctor` checks both your global `~/.gitconfig` and the local repo config and reports findings as `✓ / ⚠ / ✗`.

```
bonsai doctor

Global
  ✓  git version             2.50.1
  ✓  user.name               Jane Doe
  ✓  user.email              jane@example.com
  ⚠  pull.rebase             not set
     fix: run: git config --global pull.rebase true
  ✓  rerere.enabled          true
  ...

Local  (my-project)
  ✓  remote origin           git@github.com:org/my-project.git
  ✓  upstream tracking       origin/main
  ✓  .gitignore              present
  ...

Summary: 0 errors, 2 warnings, 13 passed
```

Pass `--verbose` to see a one-line explanation for every check:

```sh
bonsai doctor --verbose
```

## Stats

```
bonsai stats

bonsai stats

Overview
  commits      312  (2024-01-15 - 2025-05-16)
  last 30 days 47
  branches     8
  tags         12
  files        143 tracked

Contributors
  Jane Doe                  ████████████████████ 187
  John Smith                ████████████░░░░░░░░ 98
  Alice Lopes               ████░░░░░░░░░░░░░░░░ 27

File types
  .go           ████████████████████ 89
  .md           ████████░░░░░░░░░░░░ 31
  .toml         ██░░░░░░░░░░░░░░░░░░ 9

Most changed files
   1. main.go                                    47 changes
   2. tui/tui.go                                 38 changes
   ...
```

## Configuration reference

Full config with all defaults:

```toml
[modes]
default = "standard"   # standard | guided | pro

[flow]
type = "auto"          # auto | trunk | gitflow | githubflow | forking

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

[education]
panel_duration = 4   # seconds; 0 disables the feedback panel

[editor]
command = ""   # e.g. "vim", "nano", "code --wait", "hx"

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

Global config location: `~/.config/bonsai/config.toml`  
Per-project config: `.bonsai.toml` in the repo root  
Project config inherits from global and overrides selectively.

## Requirements

- Git 2.28 or later

## Philosophy

- bonsai never takes autonomous decisions
- bonsai never hides the underlying Git command
- bonsai never locks you into a workflow
- bonsai's success is measured by your independence, not your dependency

## License

MIT
