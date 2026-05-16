# bonsai doctor

`bonsai doctor` audits your Git configuration and reports findings as a structured health check. It covers both the global `~/.gitconfig` and the local repo config.

## Usage

```sh
bonsai doctor            # standard output
bonsai doctor --verbose  # adds a one-line explanation per check
```

## Output format

```
bonsai doctor

Global
  ✓  git version             2.50.1 (Apple Git-155)
  ✓  user.name               Jane Doe
  ✓  user.email              jane@example.com
  ⚠  credential.helper       not set
     fix: run: git config --global credential.helper osxkeychain  (macOS)
  ⚠  init.defaultBranch      set to "master" (recommended: main)
     fix: run: git config --global init.defaultBranch main
  ✓  pull.rebase             true
  ✓  fetch.prune             true
  ✓  push.autoSetupRemote    true
  ✓  rerere.enabled          true
  ✓  core.editor             using vim
  ✓  global gitignore        /Users/jane/.gitignore
  ✓  gpg signing             enabled, key: /Users/jane/.ssh/id_ed25519.pub
  ✓  ssh key                 /Users/jane/.ssh/id_ed25519
  ✓  ssh-agent               running (2 key(s) loaded)
  ✓  ssh github.com          Hi jane! You've successfully authenticated...

Local  (my-project)
  ✓  remote origin           git@github.com:org/my-project.git
  ✓  upstream tracking       origin/main
  ✓  .gitignore              present
  ✓  merge/rebase state      clean
  ⚠  uncommitted changes     3 file(s) with uncommitted changes
  ✓  stale remote branches   none
  ✓  branch conventions      'feat/login' matches 'feature'
  ✓  repo size               142 KB packed

Summary: 0 errors, 2 warnings, 17 passed
```

Exit code is `1` if any check has level **error** (✗), `0` otherwise.

## Global checks

| Check | What it looks for |
|-------|-------------------|
| `git version` | Git 2.28 or later |
| `user.name` | Must be set; used on every commit |
| `user.email` | Must be set and look like an email address |
| `credential.helper` | Prevents re-entering passwords on every push |
| `init.defaultBranch` | Should be `main` to match GitHub/GitLab defaults |
| `pull.rebase` | `true` keeps history linear by rebasing instead of merging on pull |
| `fetch.prune` | `true` auto-removes stale remote-tracking refs |
| `push.autoSetupRemote` | `true` eliminates the need for `-u origin <branch>` on first push |
| `rerere.enabled` | `true` memorises conflict resolutions so git can replay them |
| `core.editor` | Checks `$VISUAL`, `$EDITOR`, and `core.editor`; warns if falling back to `vi` |
| `global gitignore` | Checks `core.excludesfile` and standard locations |
| `gpg signing` | If `commit.gpgsign=true`, verifies `user.signingkey` is set |
| `ssh key` | Looks for an SSH key in `~/.ssh` (ed25519, ecdsa, rsa, dsa) |
| `ssh-agent` | Checks `SSH_AUTH_SOCK` is set and at least one key is loaded |
| `ssh <host>` | Tests SSH connectivity against the repo's remote host (or `github.com` if not in a repo); recognises GitHub, GitLab, Bitbucket, and Gitea/Forgejo success messages |

## Local checks

| Check | What it looks for |
|-------|-------------------|
| `remote origin` | An `origin` remote must be configured |
| `upstream tracking` | Current branch must track a remote branch |
| `.gitignore` | A project-level `.gitignore` must exist |
| `merge/rebase state` | No interrupted merge, cherry-pick, or rebase |
| `uncommitted changes` | Reports how many files have uncommitted changes |
| `stale remote branches` | Detects refs that would be pruned by `git remote prune origin` |
| `branch conventions` | If `.bonsai.toml` is present, checks current branch name |
| `repo size` | Warns if the pack size exceeds 100 MB |

## Verbose mode

```sh
bonsai doctor --verbose
```

Adds a dimmed explanation line under each check:

```
  ⚠  pull.rebase             not set
     'git pull' merges by default, which creates noisy merge commits on every
     sync. With pull.rebase=true it rebases instead, keeping history linear.
     fix: run: git config --global pull.rebase true
```

## Fixing issues

Each warning and error includes a `fix:` line with the exact command to run. You can copy and run it directly in your terminal, or apply global settings from the **Configuration manager** inside the TUI (`C` key → Recommendations).
