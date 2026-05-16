# Flows and Branch Conventions

## Workflow flows

A "flow" tells bonsai how your team organises branches and where pull requests land. It affects the branch creation picker and the contextual hints shown after each action.

### auto (default)

bonsai inspects your `[conventions.branches]` config and infers the flow:

- If all four gitflow types are present (feature, bugfix, release, hotfix) → gitflow
- Otherwise → trunk

Use `auto` if you want bonsai to adapt without manual configuration.

### trunk

Short-lived topic branches that merge directly into `main`. The branch picker shows a simple name input.

```toml
[flow]
type = "trunk"

[conventions.branches.feature]
prefix = "feat/"

[conventions.branches.bugfix]
prefix = "fix/"

[conventions.branches.hotfix]
prefix = "hotfix/"
```

### gitflow

Feature branches off `develop`; releases and hotfixes have their own prefixes. The branch picker shows four options:

```
  1) feature   feat/PROJ-123-description
  2) bugfix    fix/PROJ-456-description
  3) release   release/1.2.0
  4) hotfix    hotfix/critical-fix
```

```toml
[flow]
type = "gitflow"

[conventions.branches.feature]
prefix  = "feat/"
pattern = "feat/{ticket-id}-{description}"
example = "feat/PROJ-123-login-oauth"

[conventions.branches.bugfix]
prefix  = "fix/"
example = "fix/PROJ-456-crash-on-login"

[conventions.branches.release]
prefix = "release/"

[conventions.branches.hotfix]
prefix = "hotfix/"
```

### githubflow

Feature branches with PRs into `main`. Simpler than gitflow - only two branch types:

```toml
[flow]
type = "githubflow"

[conventions.branches.feature]
prefix = "feat/"

[conventions.branches.bugfix]
prefix = "fix/"
```

### forking

Same branch structure as githubflow but the hints remind you to push to your fork and open a PR upstream.

## Branch conventions

Conventions let you enforce a consistent naming scheme across your team.

### Configuration

```toml
[conventions.branches.feature]
prefix  = "feat/"
pattern = "feat/{ticket-id}-{description}"
example = "feat/PROJ-123-login-oauth"
```

| Field | Required | Description |
|-------|----------|-------------|
| `prefix` | yes | Branch names must start with this string |
| `pattern` | no | Human-readable template shown in the convention panel |
| `example` | no | Concrete example shown as a hint |

### Validation modes

```toml
[conventions.validation]
mode = "strict"   # strict | warn | off
```

| Mode | Behavior |
|------|----------|
| `strict` | Blocks actions that would result in a non-conforming branch name and shows the convention panel |
| `warn` | Shows a warning in the main panel header but does not block |
| `off` | No validation at all |

### Custom branch types

You can define as many types as you need:

```toml
[conventions.branches.chore]
prefix  = "chore/"
example = "chore/update-dependencies"

[conventions.branches.docs]
prefix  = "docs/"
example = "docs/api-reference"

[conventions.branches.experiment]
prefix  = "exp/"
example = "exp/new-auth-approach"
```

### Protected branch names

The following branch names are always allowed regardless of conventions: `main`, `master`, `develop`, `HEAD`, and any branch whose name matches a configured prefix exactly.

### Per-project conventions

Add a `.bonsai.toml` to override conventions for a specific repo without changing your global defaults:

```toml
# .bonsai.toml
[conventions.branches.feature]
prefix  = "feature/"
example = "feature/auth-oauth"

[conventions.validation]
mode = "warn"
```

## Interactive setup

Run the wizard to configure conventions interactively:

```sh
bonsai setup           # global
bonsai setup --local   # this repo only
```

The wizard asks for each branch type's prefix and lets you add custom types. It writes the result to the appropriate config file.
