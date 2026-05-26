<div align="center">
  <img src="docs/logo.svg" alt="bonsai" width="240">
</div>

# bonsai

> trim. stage. grow.

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

If you run bonsai in a directory that is not a git repository, it will offer to initialize one for you:

```
  No git repository found

  /Users/you/my-project

  this directory is not a git repository

  [i] initialize here   [q] quit
```

Press `i` to run `git init`, then optionally add a remote - or press `esc` to skip and go straight to the TUI.

## CLI commands

| Command | Description |
|---------|-------------|
| `bonsai` | Open the interactive TUI |
| `bonsai help` | Show help |
| `bonsai version` | Print version |
| `bonsai update` | Update to the latest release |
| `bonsai uninstall` | Remove bonsai from this system |
| `bonsai changelog` | Show the changelog |
| `bonsai clone <url> [dir]` | Clone a repo and open bonsai in it |
| `bonsai setup` | Interactive wizard - global config |
| `bonsai setup --local` | Interactive wizard - per-project `.bonsai.toml` |
| `bonsai init` | Create a commented `.bonsai.toml` template (no wizard) |
| `bonsai config` | Open global config in your editor |
| `bonsai config --local` | Open (or create) per-project `.bonsai.toml` |
| `bonsai config --path` | Print the path to the global config file |
| `bonsai doctor` | Check global and local git configuration health |
| `bonsai doctor --verbose` | Same, with a one-line explanation per check |
| `bonsai stats` | Repository statistics (commits, contributors, file types) |
| `bonsai patch --create --base=<ref>` | Create `.patch` files from commits (git format-patch) |
| `bonsai patch --apply <file>` | Apply a `.patch` file (git am) |
| `bonsai archive` | Export the repo as `tar.gz` (default) or `zip` |
| `bonsai bundle --create <file>` | Pack refs into a portable bundle |
| `bonsai bundle --verify <file>` | Verify a bundle file |
| `bonsai ssh --status` | Show SSH key, agent status, and remote connectivity |
| `bonsai ssh --keygen` | Generate a new `ed25519` SSH key |
| `bonsai ssh --show` | Print your SSH public key |
| `bonsai lfs --status` | Show pending LFS objects |
| `bonsai lfs --track <pattern>` | Track a file pattern via git lfs |
| `bonsai lfs --untrack <pattern>` | Stop tracking a pattern |
| `bonsai lfs --pull` | Download all LFS objects for the current checkout |
| `bonsai lfs --install` | Install LFS hooks into this repository |
| `bonsai standup` | Show your commits today (add `--days N` or `-w` for a week) |
| `bonsai repo --create <name>` | Create a new remote repository (GitHub / GitLab) |
| `bonsai repo --open` | Open the current repo in the browser |
| `bonsai mcp --install` | Detect AI tools and configure bonsai as an MCP server |
| `bonsai mcp --uninstall` | Remove bonsai MCP server configuration from AI tools |
| `bonsai mcp` | Start the MCP stdio server (used by AI tools internally) |
| `bonsai context` | Full repo snapshot: status + diff + recent commits (AI output) |
| `bonsai status` | Structured working-tree status (AI output) |
| `bonsai diff` | Structured diff: staged, unstaged, untracked (AI output) |
| `bonsai log` | Structured commit history (AI output) |
| `bonsai show [--ref=<ref>]` | Structured commit detail (AI output) |
| `bonsai blame --file=<path> [--start-line=N --end-line=N]` | Structured line-by-line blame; optional line range (AI output) |
| `bonsai branches` | Structured branch list (AI output) |
| `bonsai stash-list` | Structured stash entries (AI output) |
| `bonsai review [--base=<ref>]` | Structured diff against a base branch for code review (AI output) |

### AI / MCP integration

bonsai exposes its git analysis tools as an [MCP](https://modelcontextprotocol.io) server, making them available to any AI coding assistant that supports the protocol (Claude Code, Cursor, Windsurf, and others).

```sh
bonsai mcp --install    # detect installed AI tools and configure automatically
bonsai mcp --uninstall  # remove configuration from all detected tools
```

Once installed, AI agents prefer bonsai tools over raw git commands for read-only analysis (diff, log, blame, review, etc.) because bonsai returns structured, AI-optimized output. Write operations (commit, push, pull, merge) still go through git directly.

The `bonsai context`, `bonsai diff`, `bonsai review`, and related commands are the same tools exposed via MCP - you can also call them from the terminal to get structured JSON, Markdown, or XML output:

```sh
bonsai context --format=markdown                    # full repo snapshot
bonsai review --base=main --detailed               # code review diff
bonsai review --base=main --detailed --context=1   # same, 1 context line per hunk
bonsai diff --staged --detailed                    # staged changes with patch hunks
bonsai blame --file=git/git.go --start-line=10 --end-line=50  # blame a line range
```

### Examples

```sh
# clone and open immediately
bonsai clone https://github.com/example/repo.git

# health check before a release
bonsai doctor --verbose

# check SSH setup
bonsai ssh --status

# generate an SSH key if you do not have one
bonsai ssh --keygen

# repo statistics
bonsai stats

# export a patch set for email review
bonsai patch --create --base=main --output=patches/

# apply patches from a contributor
bonsai patch --apply patches/0001-*.patch

# archive the current HEAD as a zip
bonsai archive --format=zip --output=release.zip

# bundle the whole repo for offline transfer
bonsai bundle --create repo.bundle

# show your commits this week
bonsai standup -w
```

## TUI key bindings

Open bonsai and press `?` to see the full in-app reference.

Open bonsai and press `?` for the full in-app reference. Key highlights:

### Main panel

| Key | Action |
|-----|--------|
| `space` | Stage / unstage the selected file |
| `+` | Stage all changes at once (`git add .`) |
| `h` | Hunk staging - choose which hunks to stage; press `l` inside to select individual lines |
| `d` | Diff for the selected file |
| `H` | File history - every commit that touched this file (uses `--follow` to track renames automatically) |
| `e` | Blame - who last changed each line |
| `x` | Discard working tree changes (confirm required) |
| `o` | Restore file to HEAD or a specific ref |
| `c` | Commit panel |
| `p` | Push menu (push / force-with-lease / set-upstream) |
| `P` | Pull |
| `f` | Fetch menu |
| `s` | Stash with message input |
| `S` | Stash list - pop, apply, drop |
| `g` | Branch graph (`git log --graph --all`) |
| `l` | Commit log |
| `L` | Reflog |
| `b` | Create / switch branch (flow picker in gitflow mode) |
| `B` | Branch list - switch, merge, rebase, delete, rename, delete remote |
| `z` | Reset menu (soft / mixed / hard) |
| `t` | Tag list - create, delete, push to remote (press `tab` in create form to toggle lightweight / annotated) |
| `i` | Bisect panel (press `s` to skip the current commit when a session is active) |
| `R` | Interactive rebase - reorder/squash/fixup/drop commits |
| `A` | Amend HEAD |
| `U` | Undo last commit / merge / rebase |
| `u` | Untrack selected staged file (`git rm --cached`) |
| `W` | Worktree list (press `p` to prune stale worktree refs) |
| `O` | Remote management (press `p` on a remote to prune stale remote-tracking refs) |
| `M` | Submodule management |
| `n` | Git notes for HEAD |
| `X` | Clean untracked files |
| `K` | PR / MR panel (GitHub, GitLab, Bitbucket) |
| `I` | Issues panel |
| `` ` `` | SSH key manager - list keys, test connections |
| `V` | LFS panel - tracked files, pull/push/track/untrack |
| `D` | Multi-repo dashboard |
| `F` | Finish gitflow branch (gitflow mode only) |
| `C` | Configuration manager (config, gitignore, profiles, education) |
| `a` | Abort in-progress merge / rebase / cherry-pick |
| `?` | Help panel |
| `q` / `ctrl+c` | Quit |

### Log panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `enter` | Open commit detail |
| `r` | Revert selected commit (safe undo via new commit) |
| `/` | Open search / filter input |
| `ctrl+/` or `ctrl+r` | Clear active filter |
| `m` | Load more commits |
| `esc` | Back |

### Commit detail panel

| Key | Action |
|-----|--------|
| `↑` / `↓` | Scroll |
| `d` | Full diff for this commit |
| `p` | Cherry-pick onto current branch |
| `R` | Cherry-pick a range of commits (enter starting hash) |
| `r` | Revert this commit — safe undo via a new commit (safe on shared branches) |
| `y` | Copy commit hash to clipboard |
| `esc` | Back |

### Branch list panel

| Key | Action |
|-----|--------|
| `enter` | Switch to selected branch |
| `m` | Merge into current |
| `r` | Rebase current onto selected |
| `d` | Delete local branch |
| `n` | Rename branch |
| `D` | Delete remote tracking branch |
| `X` | Sweep all `gone` branches (bulk delete with confirmation) |
| `esc` | Back |

Each branch shows its status: `↑↓ synced` (green), `↑N`/`↓N` (ahead/behind), `gone` (red — remote was deleted), `merged` (purple), `(protected)` (red). When the list is longer than the terminal, the title shows your scroll position (`5/36 ↓`).

### PR / MR panel

| Key | Action |
|-----|--------|
| `enter` | Open PR detail view (state, CI status for GitHub / GitLab / Bitbucket, labels, reviewers, URL) |
| `o` | Open PR in browser |
| `d` | View full PR diff (cursor-based, press `c` to comment on a line) |
| `a` | Approve PR |
| `A` | Request changes (with reason) |
| `c` | Post a general comment |
| `m` | Merge picker - choose merge / squash / rebase |
| `n` | Create a new PR (title, description, base branch form; press `d` to toggle draft mode — GitHub and GitLab only) |
| `r` | Refresh list |
| `esc` | Back |

### PR detail panel

| Key | Action |
|-----|--------|
| `o` | Open in browser |
| `d` | View diff |
| `a` | Approve |
| `m` | Merge picker |
| `y` | Copy URL to clipboard |
| `esc` | Back to PR list |

### Conflict panel

| Key | Action |
|-----|--------|
| `o` | Accept ours |
| `t` | Accept theirs |
| `b` | Accept base (common ancestor) |
| `r` | Remove markers (keep both) |
| `e` | Manual edit mode - type a custom resolution |
| `esc` | Back |

All keybindings for `commit`, `push`, `pull`, `stash`, `graph`, `undo`, and `quit` are remappable via `[keybindings]` in your config.

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

`bonsai doctor` checks your global `~/.gitconfig`, local repo config, and SSH setup. Reports `✓ / ⚠ / ✗`.

Global checks include: git version, user.name/email, credential.helper, init.defaultBranch, pull.rebase, fetch.prune, push.autoSetupRemote, rerere.enabled, core.editor, global gitignore, GPG signing, SSH key, ssh-agent, and SSH connectivity to the detected remote host.

```
bonsai doctor --verbose

Global
  ✓  git version             2.50.1
  ✓  user.name               Jane Doe
  ✓  user.email              jane@example.com
  ⚠  pull.rebase             not set
     fix: run: git config --global pull.rebase true
  ✓  ssh key                 /Users/jane/.ssh/id_ed25519
  ✓  ssh-agent               running (1 key(s) loaded)
  ✓  ssh github.com          Hi jane! You've successfully authenticated...

Local  (my-project)
  ✓  remote origin           git@github.com:org/my-project.git
  ✓  upstream tracking       origin/main
  ✓  .gitignore              present
  ...

Summary: 0 errors, 1 warning, 16 passed
```

## SSH

```sh
bonsai ssh --status    # SSH key, agent status, connectivity to the repo's remote
bonsai ssh --keygen    # generate ~/.ssh/id_ed25519 (uses your git user.email)
bonsai ssh --show      # print your public key (ready to paste into GitHub/GitLab)
```

`bonsai ssh --status` detects the remote host from the current repo's remotes and tests SSH auth against it. Works with GitHub, GitLab, Bitbucket, Gitea/Forgejo, Azure DevOps, and any other SSH-based forge.

Press `` ` `` inside the TUI to open the SSH key manager panel.

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
