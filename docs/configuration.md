# Configuration

## File locations

| File | Purpose |
|------|---------|
| `~/.config/bonsai/config.toml` | Global defaults (all repos) |
| `.bonsai.toml` | Per-project overrides (committed to the repo) |

The per-project file inherits every value from global and overrides only what it defines. Fields left out in `.bonsai.toml` fall back to the global value.

On Windows the global config is at `%APPDATA%\bonsai\config.toml`.

## Quick setup

```sh
bonsai setup           # global wizard
bonsai setup --local   # per-project wizard (inside a repo)
```

To create a commented template without the wizard:

```sh
bonsai init   # creates .bonsai.toml in the current directory
```

To open the config in your editor:

```sh
bonsai config          # global
bonsai config --local  # per-project
bonsai config --path   # print the path to the global file
```

## Full reference

```toml
# ── Modes ──────────────────────────────────────────────────────────────────
[modes]
# standard  shows the command that ran after each action (default)
# guided    full explanation after every action - good for beginners
# pro       no feedback panel at all
default = "standard"

# ── Workflow flow ───────────────────────────────────────────────────────────
[flow]
# auto        detected from your branch conventions (default)
# trunk       short-lived branches merged frequently into main
# gitflow     feature/bugfix/release/hotfix; PRs target develop
# githubflow  feature branches with PRs into main
# forking     fork-based contribution model
type = "auto"

# ── Branch conventions ──────────────────────────────────────────────────────
# Define as many branch types as you need.
# prefix   required - bonsai checks that branch names start with this
# pattern  optional - human-readable template shown in the convention panel
# example  optional - shown as a concrete example

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

# ── Branch validation ────────────────────────────────────────────────────────
[conventions.validation]
# strict  blocks the action and shows the convention panel
# warn    shows a warning in the header but does not block
# off     no validation
mode = "strict"

# ── Education panel ──────────────────────────────────────────────────────────
[education]
# How long the feedback panel stays visible (seconds).
# Set to 0 to disable it entirely.
panel_duration = 4

# ── Editor ──────────────────────────────────────────────────────────────────
[editor]
# Editor used by `bonsai config` and `bonsai config --local`.
# Falls back to $VISUAL, then $EDITOR, then vi.
command = ""   # e.g. "vim", "nano", "code --wait", "hx"

# ── Keybindings ─────────────────────────────────────────────────────────────
# Remappable keys. Single-character strings only.
[keybindings]
graph  = "g"   # unused in TUI navigation but reserved
commit = "c"
branch = "b"
push   = "p"
pull   = "l"
stash  = "s"
undo   = "z"
quit   = "q"

# ── Metrics ─────────────────────────────────────────────────────────────────
[metrics]
# Local-only metrics written to ~/.config/bonsai/metrics.json.
# Nothing is ever sent to a server.
enabled = false

  [metrics.track]
  errors      = true   # failed git operations
  conventions = true   # branch convention violations
  commits     = false  # commit frequency
  habits      = false  # which features you use
```

## Per-project example

A minimal `.bonsai.toml` that switches to guided mode and enforces stricter branch names:

```toml
[modes]
default = "guided"

[conventions.branches.feature]
prefix  = "feature/"
example = "feature/auth-oauth"

[conventions.branches.bugfix]
prefix = "bugfix/"

[conventions.validation]
mode = "strict"
```

## Mode migration

If you have an older config with the deprecated mode names, bonsai migrates them automatically on load:

| Old name | New name |
|----------|----------|
| `novice` | `guided` |
| `learning` | `standard` |

## Keybinding remapping

Only the eight keys in `[keybindings]` are remappable. All other keys (`l`, `B`, `d`, `x`, `o`, `f`, `t`, `W`, `O`, `M`, `n`, `L`, `X`, `C`, `R`, `A`, `i`, `e`, `?`, `a`) are fixed.

Example - swap push and pull:

```toml
[keybindings]
push = "l"
pull = "p"
```
