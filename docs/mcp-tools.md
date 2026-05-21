# bonsai MCP Tools

bonsai exposes a read-only MCP server that AI coding assistants (Claude Code, Cursor, etc.) can call instead of shelling out to git directly. The tools return structured JSON optimized for consumption by language models — no text parsing required.

## When to use MCP vs git directly

| Need | Use |
|---|---|
| Reading repo state for analysis or review | MCP tool |
| Structured per-file stats without parsing | MCP tool |
| Any write operation (commit, push, merge, rebase…) | `git` directly |
| Arbitrary `git log --format` or `--graph` | `git` directly |
| Commands outside the nine tools below | `git` directly |

The practical rule: if you are reading state for review or analysis, reach for an MCP tool — you get typed output instead of text to parse. If you are changing state, use git directly.

## Diff operator semantics

All bonsai diff tools use the **two-dot operator** (`source..target`), not three-dot (`source...target`).

- `A..B` — direct diff between two refs: what B has that A does not. This is the literal delta between the current state of both refs, which is what code review consumers expect.
- `A...B` — diff from the merge-base of A and B up to B. This hides changes that landed in A after the branch was created. A clean `A...B` diff does not mean the branch is conflict-free against the current A.

**Rule: always use `..` for diff and review operations.** Three-dot silently drops upstream changes from the result, which is misleading for review purposes. Three-dot is only appropriate for symmetric commit enumeration in `git log`.

---

## Tools

### git_review

Diff and commit context for code review. Compares `source..target` (two-dot). Returns structured per-file stats, total line counts, and commit list.

**Parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `source` | string | — | Ref to diff from (e.g. `main`, `origin/main`). Alias: `base`. |
| `base` | string | — | Alias for `source`, kept for backward compatibility. |
| `target` | string | `HEAD` | Ref to diff to. Allows comparing two arbitrary refs without checking out either. |
| `paths` | string[] | — | Restrict diff to these file paths. Always passed after a literal `--` separator. |
| `detailed` | boolean | `false` | Include full patch hunks. |
| `context` | integer | 3 | Context lines around each hunk. |

**Output: `ReviewOut`**

```json
{
  "base": "origin/main",
  "head": "feature/auth",
  "lines": {
    "added": 162,
    "removed": 23,
    "total_changed": 185
  },
  "files_changed": 4,
  "commits_count": 3,
  "commits": [
    { "hash": "a1b2c3d", "subject": "feat: add JWT validation", "author": "Ada", "date": "2026-05-21" }
  ],
  "diff": [
    { "path": "src/auth/auth.service.ts", "status": "M", "additions": 42, "deletions": 18 },
    { "path": "src/api/users.controller.ts", "status": "M", "additions": 120, "deletions": 5 }
  ],
  "status": { ... }
}
```

`FileDiff.status` values: `M` modified, `A` added, `D` deleted, `R` renamed.

When `detailed: true`, each `FileDiff` includes a `hunks` array of `{ header, lines[] }`.

**Examples**

```json
// feature branch vs main (HEAD is the feature branch)
{ "source": "origin/main" }

// two remote refs — no working-tree assumption
{ "source": "origin/main", "target": "origin/feature/auth" }

// path-filtered chunk for large diffs
{ "source": "origin/main", "paths": ["src/auth/auth.service.ts", "src/api/users.controller.ts"] }

// full patch for a subset of files
{ "source": "origin/main", "target": "origin/feature/auth", "paths": ["src/auth/auth.service.ts"], "detailed": true }
```

---

### git_diff

Working-tree changes grouped by scope: staged, unstaged, untracked. Returns file-level counts by default; use `detailed: true` for patch hunks.

**Parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `staged` | boolean | — | Include staged changes. |
| `unstaged` | boolean | — | Include unstaged changes. |
| `untracked` | boolean | — | Include untracked files. |
| `file` | string | — | Restrict to a single file path. |
| `detailed` | boolean | `false` | Include patch hunks. |
| `context` | integer | 3 | Context lines around each hunk. |

When none of `staged`/`unstaged`/`untracked` are set, all scopes are returned.

**Output: `DiffOut`**

```json
{
  "staged":    [ { "path": "src/foo.ts", "status": "M", "additions": 5, "deletions": 2 } ],
  "unstaged":  [ { "path": "src/bar.ts", "status": "M", "additions": 1, "deletions": 0 } ],
  "untracked": [ { "path": "notes.txt" } ]
}
```

---

### git_status

Current repository state: branch, upstream tracking, staged/unstaged/untracked file lists, conflicts, and stash count.

**Parameters:** none.

**Output: `StatusOut`**

```json
{
  "repo": "bonsai",
  "branch": "feature/auth",
  "upstream": "origin/feature/auth",
  "ahead": 2,
  "behind": 0,
  "staged":    [ { "status": "M", "path": "src/auth.ts" } ],
  "unstaged":  [],
  "conflicts": [],
  "untracked": [],
  "stash_count": 1,
  "merge_state": ""
}
```

`merge_state` is non-empty during an in-progress merge, rebase, or cherry-pick.

---

### git_log

Recent commit history.

**Parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | 20 | Maximum commits to return. |
| `since` | string | — | Start date or expression, e.g. `yesterday`, `1 week ago`, `2026-05-01`. |
| `until` | string | — | End date, e.g. `2026-05-17`. |

**Output: `LogEntry[]`**

```json
[
  { "hash": "a1b2c3d", "subject": "feat: add JWT validation", "author": "Ada", "date": "2026-05-21" }
]
```

---

### git_show

Details for a single commit: metadata, changed-file counts, and optionally patch hunks.

**Parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `ref` | string | `HEAD` | Commit ref (hash, `HEAD`, `HEAD~N`, tag). |
| `detailed` | boolean | `false` | Include patch hunks. |
| `context` | integer | 3 | Context lines around each hunk. |

**Output: `ShowOut`**

```json
{
  "hash": "a1b2c3d",
  "subject": "feat: add JWT validation",
  "author": "Ada",
  "date": "2026-05-21",
  "additions": 42,
  "deletions": 18,
  "files_changed": 3,
  "diff": [ { "path": "src/auth.ts", "status": "M", "additions": 42, "deletions": 18 } ]
}
```

---

### git_blame

Line-by-line authorship for a file.

**Parameters**

| Name | Type | Required | Description |
|---|---|---|---|
| `file` | string | yes | File path to blame. |
| `start_line` | integer | no | First line of range (1-based; requires `end_line`). |
| `end_line` | integer | no | Last line of range (1-based; requires `start_line`). |

**Output: `BlameEntry[]`**

```json
[
  { "hash": "a1b2c3d", "author": "Ada", "date": "2026-05-21", "line": 42, "content": "  return jwt.verify(token, secret);" }
]
```

---

### git_branches

All local branches with current-branch marker and upstream tracking info.

**Parameters:** none.

**Output: `BranchEntry[]`**

```json
[
  {
    "name": "feature/auth",
    "current": true,
    "upstream": "origin/feature/auth",
    "ahead": 2,
    "behind": 0,
    "date": "2026-05-21"
  }
]
```

`gone: true` when the upstream branch has been deleted.

---

### git_stash_list

All stash entries.

**Parameters:** none.

**Output: `StashEntry[]`**

```json
[
  { "ref": "stash@{0}", "description": "WIP on feature/auth: a1b2c3d feat: add JWT validation" }
]
```

---

### git_context

Full repository snapshot in one call: status + working-tree diff + recent commits. Use this as the first call when you need to understand the current state of the repo.

**Parameters**

| Name | Type | Default | Description |
|---|---|---|---|
| `limit` | integer | 10 | Number of recent commits to include. |
| `detailed` | boolean | `false` | Include patch hunks in the diff. |
| `context` | integer | 3 | Context lines around each hunk. |

**Output: `ContextOut`**

```json
{
  "status": { ... },
  "diff":   { "staged": [...], "unstaged": [...], "untracked": [...] },
  "log":    [ { "hash": "...", "subject": "...", "author": "...", "date": "..." } ]
}
```

---

## Operations not covered by MCP

Use `git` directly for anything not in the table above:

- All write operations: `commit`, `push`, `pull`, `fetch`, `merge`, `rebase`, `checkout`, `reset`, `cherry-pick`, `tag`, `stash push/pop`, `clean`, `restore`
- `git log` with arbitrary `--format`, `--graph`, or `--decorate`
- Relative-ref diffs not between named branches (e.g. `HEAD~3..HEAD~1`)
- `git remote`, `git submodule`, `git bisect`, `git worktree`
- Any cross-repo operation
