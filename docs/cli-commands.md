# CLI Commands

All commands that do not open the TUI accept no positional arguments other than subcommands. Options use `--flag` syntax.

## bonsai (no args)

Opens the interactive TUI. Runs the setup wizard on first launch.

```sh
bonsai
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

### patch create

```sh
bonsai patch create --base=<ref> [--output=<dir>]
```

| Flag | Description |
|------|-------------|
| `--base=<ref>` | Required. Create patches for commits since this ref (e.g. `main`, `HEAD~3`, a hash) |
| `--output=<dir>` | Directory to write `.patch` files into. Defaults to the current directory |

```sh
# Create patches for the last 3 commits
bonsai patch create --base=HEAD~3

# Write to a patches/ directory
bonsai patch create --base=main --output=patches/
```

### patch apply

```sh
bonsai patch apply <file> [<file>...]
```

Applies one or more `.patch` files in order using `git am`.

```sh
bonsai patch apply patches/0001-fix-auth.patch
bonsai patch apply patches/*.patch
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

### bundle create

```sh
bonsai bundle create <file> [<ref>...]
```

If no refs are given, bundles all branches (`--all`).

```sh
# Bundle everything
bonsai bundle create repo.bundle

# Bundle a specific branch
bonsai bundle create feature.bundle refs/heads/feat/login
```

### bundle verify

```sh
bonsai bundle verify <file>
```

Verifies the bundle is valid and prints a summary of what it contains.

```sh
bonsai bundle verify repo.bundle
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

## bonsai version

Prints the current version.

```sh
bonsai version
# bonsai v0.29.0
```
