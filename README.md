# bonsai

> A TUI Git client that teaches while you work.

bonsai wraps the official Git binary and adds an interactive terminal interface. After every action it shows what happened in plain language, the exact Git command that ran, and an option to copy it. The goal is that a user who starts as a novice eventually understands Git well enough to not need bonsai — or keeps using it simply because it's a pleasure to use.

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

## Modes

| Mode | Behavior |
|------|----------|
| `novice` | Full explanations, Git command always visible |
| `pro` | Education panel hidden, no explanations |
| `learning` | Pauses before executing, shows what will happen, asks for confirmation |

Set the mode in your config or pass it per session.

## Configuration

Global config at `~/.config/bonsai/config.toml`. Per-project config at `.bonsai.toml` in the repo root. Project config inherits from global and overrides selectively.

```toml
[modes]
default = "novice"

[conventions.branches]
feature = { prefix = "feat/", pattern = "feat/{ticket-id}-{description}", example = "feat/PROJ-123-login-oauth" }

[education]
panel_duration = 4  # seconds, 0 to disable
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
