# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in bonsai, please **do not open a public issue**.
Send a report to the maintainer directly via email or GitHub's private security advisory feature.

We aim to acknowledge reports within 48 hours and release a fix within 7 days for critical issues.

---

## Trust Model

### Git hooks and the `.githooks/` directory

bonsai supports a **shared hooks** scope (`.githooks/`) that is committed to the repository
and shared with the whole team. Hook scripts written there are given `0755` permissions so
git can execute them.

**Important:** hooks in `.githooks/` are executable code that runs on every developer's
machine as part of normal git operations (commit, push, merge, etc.). This is intentional
and mirrors the behavior of `.git/hooks/`, but with the added property that the scripts
travel with the repo.

#### What this means in practice

- Any collaborator with write access to the repository can modify hook scripts.
- When a teammate pulls and the hooks execute, the new code runs with their privileges.
- This is the same trust level you extend to any code you pull from a remote — treat hook
  changes with the same scrutiny as any other code change.

#### Recommended mitigations

1. **Require code review for `.githooks/` changes** — enforce branch protection rules so
   modifications to that directory always go through a PR and a reviewer.

2. **Enable commit signing** — sign commits in `.githooks/` (GPG or SSH signing via
   `gpg.program` / `gpgsm`) so you can verify the author of hook changes.

3. **Audit hook changes in CI** — add a CI step that diffs `.githooks/` on every PR and
   flags any changes for manual review.

4. **Principle of least privilege** — hooks should only do what they need (lint, format,
   run tests). Avoid giving them broad filesystem or network access.

#### What bonsai does

bonsai installs a thin dispatcher script in `.git/bonsai-hooks/` that calls the shared
hooks. The dispatcher itself contains no logic — it only delegates to the scripts in
`.githooks/`. The dispatcher is regenerated on every bonsai startup from a fixed template,
so it cannot be tampered with via the repository.

---

## Scope of git command execution

bonsai runs git commands on your behalf. All arguments are passed as separate slice
elements to `exec.Command` — they are never interpolated into a shell string. This prevents
command injection via crafted branch names, tag names, or file paths entered in the TUI.

### Exception: interactive rebase editor

During interactive rebase, bonsai sets `GIT_SEQUENCE_EDITOR` to a `cp` command that
copies a pre-built rebase todo file into place. The temp file path is single-quote-escaped
before being embedded in the environment variable, protecting against paths that contain
spaces or special characters.

---

## File permissions

| File | Permissions | Reason |
|------|-------------|--------|
| `~/.config/bonsai/config.toml` (Linux/macOS) | `0600` | Config is owner-readable only |
| `%LOCALAPPDATA%\bonsai\config.toml` (Windows) | ACL-controlled by OS | Windows does not use Unix permission bits |
| Hook scripts in `.git/bonsai-hooks/` | `0755` | Must be executable by git |
| Hook scripts in `.githooks/` | `0755` | Must be executable by git; see trust model above |

---

## Locale and output parsing

bonsai forces `LC_ALL=C LANG=C` on all git subprocesses. This ensures that git output
(branch tracking info, status lines, worktree state) is always in English regardless of
the system locale, preventing silent parsing failures in non-English environments.

---

## Diff and log size limits

To prevent memory exhaustion from unusually large diffs or generated files, bonsai
truncates diff output at **5,000 lines** and displays a notice when truncation occurs.
The git log graph is limited to **150 commits**.

---

## Supported versions

Only the latest release receives security fixes. There is no long-term support for older
versions. Update with:

```
bonsai update
```
