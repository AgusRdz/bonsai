package hooks

// Template is a built-in hook script with metadata.
type Template struct {
	Name        string // identifier used in --template=<name>
	HookName    string // which git hook this targets
	Description string
	Script      string
}

// Templates is the built-in template library.
var Templates = []Template{
	{
		Name:        "conventional-commits",
		HookName:    "commit-msg",
		Description: "enforce conventional commits format",
		Script: `#!/bin/sh
# Enforce conventional commits: https://www.conventionalcommits.org
MSG=$(cat "$1")
PATTERN="^(feat|fix|docs|style|refactor|perf|test|chore|ci|build|revert)(\(.+\))?(!)?: .+"
if ! printf '%s' "$MSG" | grep -qE "$PATTERN"; then
  echo ""
  echo "  bonsai: commit message does not follow conventional commits"
  echo ""
  echo "  expected:  <type>[scope]: <description>"
  echo "  types:     feat fix docs style refactor perf test chore ci build revert"
  echo "  example:   feat(auth): add OAuth2 support"
  echo ""
  printf '  your message: %s\n' "$MSG"
  exit 1
fi
`,
	},
	{
		Name:        "no-debug",
		HookName:    "pre-commit",
		Description: "block console.log, debugger, and common debug artifacts in staged files",
		Script: `#!/bin/sh
# Block common debug artifacts in staged files
PATTERNS='console\.log\|debugger\|binding\.pry\|byebug\|var_dump\|dd('
FILES=$(git diff --cached --name-only --diff-filter=ACM)
if [ -z "$FILES" ]; then exit 0; fi
MATCHES=$(printf '%s\n' $FILES | xargs grep -l "$PATTERNS" 2>/dev/null || true)
if [ -n "$MATCHES" ]; then
  echo ""
  echo "  bonsai: staged files contain debug statements:"
  printf '%s\n' "$MATCHES" | sed 's/^/    /'
  echo ""
  echo "  remove them before committing"
  exit 1
fi
`,
	},
	{
		Name:        "no-direct-push-main",
		HookName:    "pre-push",
		Description: "prevent direct pushes to main or master",
		Script: `#!/bin/sh
# Block direct pushes to main/master — open a PR instead
PROTECTED="main master"
while read -r local_ref local_sha remote_ref remote_sha; do
  for branch in $PROTECTED; do
    if printf '%s' "$remote_ref" | grep -q "refs/heads/${branch}$"; then
      echo ""
      echo "  bonsai: direct push to '${branch}' is not allowed"
      echo "  open a pull request instead"
      echo ""
      exit 1
    fi
  done
done
`,
	},
}

// TemplateByName returns a Template by its Name field, or nil if not found.
func TemplateByName(name string) *Template {
	for i := range Templates {
		if Templates[i].Name == name {
			return &Templates[i]
		}
	}
	return nil
}
