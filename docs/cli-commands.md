# CLI Commands

All commands that do not open the TUI accept no positional arguments other than subcommands. Options use `--flag` syntax.

## bonsai (no args)

Opens the interactive TUI. Runs the setup wizard on first launch.

```sh
bonsai
```

## bonsai clone

Clones a remote repository and opens bonsai in the resulting directory.

```sh
bonsai clone <url> [<directory>]
```

```sh
# Clone into a directory named after the repo
bonsai clone https://github.com/example/repo.git

# Clone into a custom directory
bonsai clone https://github.com/example/repo.git my-project
```

## bonsai setup

Interactive wizard that walks you through flow, branch prefixes, mode, and validation. Writes to the global config.

```sh
bonsai setup
```

**Options**

| Flag | Description |
|------|-------------|
| `--local` | Write to `.bonsai.toml` in the current directory instead of the global config |

```sh
bonsai setup --local   # per-project overrides
```

## bonsai init

Creates a commented `.bonsai.toml` template in the current directory without running the wizard. Does not overwrite an existing file.

```sh
bonsai init
```

## bonsai config

Opens the global config in your editor.

```sh
bonsai config
```

**Options**

| Flag | Description |
|------|-------------|
| `--local` | Open (or create) `.bonsai.toml` in the current directory |
| `--path` | Print the path to the global config file and exit |

```sh
bonsai config --local
bonsai config --path
# /Users/jane/.config/bonsai/config.toml
```

## bonsai doctor

Checks global and local Git configuration health. See [doctor.md](doctor.md) for details.

```sh
bonsai doctor
bonsai doctor --verbose
```

## bonsai stats

Prints repository statistics. Must be run inside a Git repo.

```sh
bonsai stats
```

Output includes:

- Commit count and date range
- Commits in the last 30 days
- Branch and tag count
- Tracked file count
- Top contributors with a bar chart
- File type breakdown
- Most-changed files (top 10)

## bonsai patch

Wraps `git format-patch` and `git am`.

### patch --create

```sh
bonsai patch --create --base=<ref> [--output=<dir>]
```

| Flag | Description |
|------|-------------|
| `--base=<ref>` | Required. Create patches for commits since this ref (e.g. `main`, `HEAD~3`, a hash) |
| `--output=<dir>` | Directory to write `.patch` files into. Defaults to the current directory |

```sh
# Create patches for the last 3 commits
bonsai patch --create --base=HEAD~3

# Write to a patches/ directory
bonsai patch --create --base=main --output=patches/
```

### patch --apply

```sh
bonsai patch --apply <file> [<file>...]
```

Applies one or more `.patch` files in order using `git am`.

```sh
bonsai patch --apply patches/0001-fix-auth.patch
bonsai patch --apply patches/*.patch
```

If conflicts occur, resolve them manually and run `git am --continue`, or run `git am --abort` to cancel.

## bonsai archive

Exports the repository at a given ref as a compressed archive.

```sh
bonsai archive [--format=<fmt>] [--output=<file>] [--ref=<ref>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `tar.gz` | Archive format: `tar.gz` or `zip` |
| `--output` | `archive.<format>` | Destination file path |
| `--ref` | `HEAD` | Git ref to archive |

```sh
# Archive HEAD as tar.gz
bonsai archive

# Archive a specific tag as zip
bonsai archive --format=zip --ref=v1.2.0 --output=release-v1.2.0.zip
```

## bonsai bundle

Wraps `git bundle` for offline transfer of a repository.

### bundle --create

```sh
bonsai bundle --create <file> [<ref>...]
```

If no refs are given, bundles all branches (`--all`).

```sh
# Bundle everything
bonsai bundle --create repo.bundle

# Bundle a specific branch
bonsai bundle --create feature.bundle refs/heads/feat/login
```

### bundle --verify

```sh
bonsai bundle --verify <file>
```

Verifies the bundle is valid and prints a summary of what it contains.

```sh
bonsai bundle --verify repo.bundle
```

## bonsai update

Updates bonsai to the latest release by downloading the new binary and replacing the current one in-place.

```sh
bonsai update
```

## bonsai uninstall

Removes the bonsai binary from the system. Prompts for confirmation.

```sh
bonsai uninstall
```

## bonsai changelog

Prints the full changelog to stdout.

```sh
bonsai changelog
bonsai changelog | less
```

## bonsai ssh

Manage SSH keys and check connectivity.

### ssh --status

```sh
bonsai ssh --status
```

Prints:
- Path of the active SSH key in `~/.ssh`
- ssh-agent status and number of loaded keys
- Authentication result against the detected remote host

### ssh --keygen

```sh
bonsai ssh --keygen
```

Generates a new `ed25519` key at `~/.ssh/id_ed25519`. Uses `user.email` from your global git config as the key comment. Prints the public key and the GitHub/GitLab URLs to add it. Does nothing if a key already exists.

### ssh --show

```sh
bonsai ssh --show
```

Prints the public key from the first SSH key found in `~/.ssh`.

## bonsai lfs

Manage Git LFS tracked files and objects.

### lfs --status

```sh
bonsai lfs --status
```

Shows pending LFS objects. Equivalent to `git lfs status`.

### lfs --track

```sh
bonsai lfs --track <pattern>
```

Adds a file pattern to `.gitattributes` to track via git lfs (e.g. `*.psd`, `*.zip`).

### lfs --untrack

```sh
bonsai lfs --untrack <pattern>
```

Removes a pattern from `.gitattributes`.

### lfs --pull

```sh
bonsai lfs --pull
```

Downloads all LFS objects for the current checkout.

### lfs --install

```sh
bonsai lfs --install
```

Installs git lfs hooks into the current repository.

## bonsai standup

Shows your recent commits, defaulting to today.

```sh
bonsai standup
bonsai standup --days 3
bonsai standup -w               # last 7 days
bonsai standup -a "Jane Doe"    # filter by author name
```

| Flag | Description |
|------|-------------|
| `--days N` | Show commits from the last N days (default: 1) |
| `-w` | Shorthand for `--days 7` |
| `-a <name>` | Filter by author name (defaults to `user.name` from git config) |

## bonsai repo

Create and open remote repositories.

### repo --create

```sh
bonsai repo --create <name> [--private] [--internal]
```

Creates a new repository on the detected provider (GitHub via `gh`, GitLab via `glab`).

```sh
bonsai repo --create my-new-repo
bonsai repo --create my-new-repo --private
```

### repo --open

```sh
bonsai repo --open
```

Opens the current repository in the browser.

## bonsai version

Prints the current version.

```sh
bonsai version
# bonsai v0.29.0
```

## bonsai mcp

Manages the bonsai MCP server, which exposes git analysis tools to AI coding assistants.

### mcp --install

```sh
bonsai mcp --install
```

Interactive wizard that detects installed AI tools (Claude Code, Claude Desktop, Cursor) and writes the bonsai MCP server configuration automatically. Supports project scope (`.mcp.json`) and user scope (`~/.claude.json`).

### mcp --uninstall

```sh
bonsai mcp --uninstall
```

Removes the bonsai MCP server entry from all known configuration locations.

### mcp (no flags)

```sh
bonsai mcp
```

Starts the MCP stdio server. This is called by AI tools internally - you do not need to run it manually.

---

## AI / structured output commands

The following commands output structured data (JSON by default, Markdown or XML with `--format`) intended for AI agents and scripting. They are also the tools exposed via the MCP server.

All accept `--format=json` (default), `--format=markdown`, or `--format=md` / `--format=xml`.

### bonsai context

Full repository snapshot combining status, diff, and recent commits in a single call.

```sh
bonsai context
bonsai context --limit=20
bonsai context --detailed          # include full patch hunks
bonsai context --format=markdown
```

| Flag | Description |
|------|-------------|
| `--limit=N` | Number of recent commits (default 10) |
| `--detailed` | Include patch hunks in the diff section |
| `--format=<fmt>` | Output format: `json` (default), `markdown` / `md`, `xml` |

### bonsai status

Structured working-tree status: branch, upstream, staged/unstaged/untracked files, conflicts, stash count.

```sh
bonsai status
bonsai status --format=markdown
```

### bonsai diff

Structured diff showing staged, unstaged, and untracked changes.

```sh
bonsai diff
bonsai diff --staged
bonsai diff --staged --detailed    # include patch hunks
bonsai diff --file=src/main.go
```

| Flag | Description |
|------|-------------|
| `--staged` | Include staged changes only |
| `--unstaged` | Include unstaged changes only |
| `--untracked` | Include untracked files only |
| `--detailed` | Include patch hunks |
| `--file=<path>` | Filter to a single file |

### bonsai log

Structured commit history with hash, subject, author, and date.

```sh
bonsai log
bonsai log --limit=50
bonsai log --since="1 week ago"
```

| Flag | Description |
|------|-------------|
| `--limit=N` | Maximum commits to return (default 20) |
| `--since=<expr>` | Start date or expression, e.g. `yesterday`, `2026-05-01` |
| `--until=<expr>` | End date |

### bonsai show

Structured detail for a single commit: metadata, changed files, and optionally patch hunks.

```sh
bonsai show
bonsai show --ref=abc1234
bonsai show --detailed
```

| Flag | Description |
|------|-------------|
| `--ref=<ref>` | Commit ref (default `HEAD`) |
| `--detailed` | Include patch hunks |

### bonsai blame

Line-by-line blame for a file: each line annotated with commit hash, author, and date.

```sh
bonsai blame --file=src/main.go
```

| Flag | Description |
|------|-------------|
| `--file=<path>` | Required. File path to blame |

### bonsai branches

Structured list of all local branches with current marker and upstream tracking info.

```sh
bonsai branches
```

### bonsai stash-list

Structured list of all stash entries.

```sh
bonsai stash-list
```

### bonsai review

Diff and commit context comparing HEAD against a base branch, optimized for code review.

```sh
bonsai review
bonsai review --base=main
bonsai review --base=main --detailed
```

| Flag | Description |
|------|-------------|
| `--base=<ref>` | Base branch or ref to compare against (default: detected main branch) |
| `--detailed` | Include patch hunks |
