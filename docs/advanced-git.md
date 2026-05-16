# Advanced Git Operations

All advanced operations are accessible from the main TUI panel. This page explains what each one does, how to reach it, and what to expect.

## Hunk staging (`h`)

Stage or unstage individual hunks within a file instead of the whole file.

1. Navigate to a changed or staged file in the main panel.
2. Press `h` to open the hunk panel.
3. Each hunk shows its `@@ ... @@` header and a short preview of the changed lines.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Move selection |
| `space` | Toggle the selected hunk on/off |
| `a` | Select all / deselect all |
| `enter` | Apply selected hunks (stage or unstage) |
| `esc` | Cancel |

All hunks are selected by default. Deselect the ones you want to leave out, then press `enter`.

Staging hunks uses `git apply --cached`. Unstaging reverses the patch with `git apply --cached --reverse`.

> Untracked files (new files not yet known to git) cannot be partially staged - press `space` to stage the full file first.

## Push menu (`p`)

Opens a menu instead of pushing immediately, so you can choose the push mode.

| Option | Git command | When to use |
|--------|-------------|-------------|
| Push | `git push` | Normal push to the tracking remote |
| Push --force-with-lease | `git push --force-with-lease` | Force-push after a rebase; safe because it fails if the remote has commits you have not fetched |
| Push --set-upstream origin `<branch>` | `git push --set-upstream origin <branch>` | First push of a new branch; sets the tracking remote in one step |

Navigate with `↑`/`↓` and press `enter` to execute.

## Reset (`z`)

Opens a menu with three reset modes. Resets apply to the current HEAD.

| Option | Git command | Effect |
|--------|-------------|--------|
| soft | `git reset --soft HEAD~1` | Undoes the last commit; changes remain staged |
| mixed | `git reset --mixed HEAD~1` | Undoes the last commit; changes remain unstaged |
| hard | `git reset --hard HEAD~1` | Undoes the last commit and discards all changes |

Hard reset is destructive and requires confirmation.

## Merge

Available from the log panel (`l` → select a commit → `m`). Merges the selected commit into the current branch.

If the merge produces conflicts they appear in the Conflicts section at the top of the main panel. Use the conflict panel (`d` on a conflicted file) to resolve them.

## Cherry-pick

Also available from the log panel. Select a commit and press `p` to cherry-pick it onto the current branch.

## Tags (`t`)

The tag list panel shows all local tags.

| Key | Action |
|-----|--------|
| `n` | Create a new lightweight tag at HEAD |
| `d` | Delete the selected tag |
| `esc` | Back |

To push a tag after creating it: `git push origin <tag-name>`.

## Interactive rebase (`R`)

1. Press `R` from the main panel.
2. Enter a base ref - the rebase will include all commits between that ref and HEAD. Examples: `HEAD~5`, `main`, a specific hash.
3. The commit list appears. Each commit shows its action, hash, and message.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Move selection |
| `enter` | Cycle action (pick → reword → squash → fixup → drop → pick) |
| `ctrl+↑`/`ctrl+↓` | Reorder commits |
| `r` | Start the rebase |
| `esc` | Cancel |

Actions:

| Action | Description |
|--------|-------------|
| `pick` | Keep the commit as-is |
| `reword` | Keep the commit but edit the message |
| `squash` | Merge into the previous commit, edit combined message |
| `fixup` | Merge into the previous commit, discard this message |
| `drop` | Remove the commit entirely |

## Amend (`A`)

Opens the amend panel for HEAD.

| Option | Description |
|--------|-------------|
| message | Change the commit message |
| author | Change the author name and email |
| date | Change the commit date |
| --no-edit | Amend without changing the message (absorbs staged changes) |

> Only amend commits that have not been pushed to a shared remote.

## Blame (`e`)

Select a file from the main panel and press `e` to open the blame view.

Each line shows the abbreviated hash, author, date, and source line. Scroll with `↑`/`↓`. Press `esc` to close.

## Bisect (`i`)

Binary search for the commit that introduced a bug.

1. Press `i` to open the bisect panel.
2. Press `s` to start.
3. Mark the current state as `b` (bad) or `g` (good). You can also enter a specific hash to mark.
4. git checks out the midpoint commit. Test it and mark again.
5. Repeat until bisect identifies the culprit.
6. Press `r` to reset and return to the original branch.

## Worktrees (`W`)

Linked worktrees let you check out a different branch in a separate directory without disturbing your current work.

| Key | Action |
|-----|--------|
| `a` | Add a new worktree (enter path and branch name) |
| `d` | Remove the selected worktree |
| `esc` | Back |

Example: check out a hotfix while keeping your feature branch work intact:

```
~
├── my-project/           ← main worktree (feat/login branch)
└── my-project-hotfix/    ← linked worktree (hotfix/critical-fix branch)
```

## Remotes (`O`)

| Key | Action |
|-----|--------|
| `a` | Add a new remote (enter name and URL) |
| `d` | Remove the selected remote |
| `r` | Rename the selected remote |
| `esc` | Back |

## Submodules (`M`)

| Key | Action |
|-----|--------|
| `a` | Add a submodule (enter URL and optional local path) |
| `u` | Run `git submodule update --init` on all submodules |
| `d` | Deinit the selected submodule |
| `esc` | Back |

Status icons:

| Icon | Meaning |
|------|---------|
| ` ` (space) | Clean, matches parent recorded hash |
| `M` | Checked out commit differs from the recorded hash |
| `?` | Not initialised |
| `!` | Merge conflict |

## Restore (`o`)

Select a file and press `o` to restore it to a specific state.

Enter a ref in the input (defaults to `HEAD`). Valid values:

- `HEAD` - discard all changes and return to the last commit
- `HEAD~2` - restore to two commits ago
- `abc1234` - restore to a specific commit hash
- `main` - restore to the tip of another branch

The restore appears as a modification in your working tree; you still need to stage and commit it if you want to keep it.

## Reflog (`L`)

Shows all recent HEAD movements - commits, checkouts, resets, merges, rebases.

| Key | Action |
|-----|--------|
| `↑`/`↓` | Scroll |
| `r` | Reset HEAD to the selected entry (mixed reset, confirm required) |
| `y` | Copy the hash to clipboard |
| `esc` | Back |

Use the reflog to recover commits that appear to be lost after a hard reset or accidental branch deletion.

## Notes (`n`)

Git notes attach metadata to a commit without changing the commit itself.

| Key | Action |
|-----|--------|
| `e` | Edit the note (opens inline input) |
| `d` | Delete the note (confirm required) |
| `esc` | Back |

Notes are stored in `refs/notes/commits` and are not transferred on push/pull by default. To share notes:

```sh
git push origin refs/notes/*
git fetch origin refs/notes/*:refs/notes/*
```

## Clean (`X`)

Shows a preview of all untracked files and directories that would be removed by `git clean -fd`. Requires confirmation before deleting anything.

> This is a destructive operation. Untracked files are not in Git history and cannot be recovered once removed.

## Conflict resolution

When a merge, cherry-pick, or rebase produces conflicts:

1. The conflicts appear at the top of the main panel.
2. Select a conflicted file and press `d` to open the conflict viewer.
3. The viewer shows the file with `<<<<<<<`, `=======`, and `>>>>>>>` markers colour-coded.

| Key | Action |
|-----|--------|
| `o` | Accept ours - keep our version, discard theirs |
| `t` | Accept theirs - keep their version, discard ours |
| `r` | Remove markers - keep both sides concatenated |

After resolving all conflicts, stage the files and commit (or `git rebase --continue` / `git cherry-pick --continue` if mid-operation).

Press `a` from the main panel to abort an in-progress operation entirely.

## File history (`H`)

Shows the commit history for a single file - every commit that touched it, oldest at the bottom.

1. Select any tracked file in the main panel.
2. Press `H` to open the file history panel.
3. Each line shows the hash, date, author, and subject of the commit.
4. Press `enter` on a commit to open the commit detail panel for that commit.
5. From the commit detail panel press `d` to see the full diff, or `esc` to return to file history.

## Branch graph (`g`)

Opens a full `git log --graph --all --oneline --decorate` view showing the commit and branch topology of the whole repository.

Scroll with `↑`/`↓` and press `esc` to close.

## Commit diff (`d` in commit detail)

From the log panel (`l`), open any commit with `enter`. In the commit detail panel press `d` to view the complete diff for that commit (`git show`). Useful for reviewing a specific change without leaving bonsai.

## Branch operations (from branch list `B`)

| Key | Action |
|-----|--------|
| `enter` | Switch to selected branch |
| `m` | Merge selected branch into current |
| `r` | Rebase current onto selected branch |
| `d` | Delete selected local branch |
| `n` | Rename selected branch |
| `D` | Delete the remote tracking branch for the selected branch |

`d` uses `git branch -d` (safe delete - fails on unmerged work). `D` uses `git push <remote> --delete <branch>` and requires the branch to have a configured upstream. Both require confirmation.

## Stash operations (from stash list `S`)

| Key | Action |
|-----|--------|
| `enter` | Pop the selected stash (`git stash pop`) |
| `a` | Apply without removing (`git stash apply`) |
| `d` | Drop the selected stash (`git stash drop`) |

Use `a` when you want to apply a stash to multiple branches. Use `d` to discard a stash permanently. Both `d` require confirmation.

## Tag push (`p` in tag list `t`)

Select a tag in the tag list and press `p` to push it to `origin` (`git push origin <tag>`). Requires confirmation. Useful after creating an annotated release tag locally.

## Configuration manager (`C`)

The configuration manager panel gives you a read/edit view of all config files without leaving bonsai.

| Section | File |
|---------|------|
| Global config | `~/.gitconfig` |
| Local config | `.git/config` |
| Global gitignore | `~/.config/git/ignore` or `core.excludesfile` |
| Local .gitignore | `.gitignore` |
| Recommendations | Best-practice settings with one-key apply |
| Profiles (includeIf) | Conditional config includes |

Press `enter` on a section to view it. Press `e` to open it in your configured editor. In the Recommendations section press `enter` to apply a setting directly.
