package ignore

import "strings"

type seedEntry struct {
	comment string // section header; empty = no header, same section as previous
	pattern string
}

func basePatterns() []seedEntry {
	return []seedEntry{
		// OS
		{comment: "OS", pattern: ".DS_Store"},
		{pattern: ".DS_Store?"},
		{pattern: "Thumbs.db"},
		{pattern: "desktop.ini"},
		{pattern: "ehthumbs.db"},

		// Editor
		{comment: "Editor", pattern: ".idea/"},
		{pattern: ".vscode/"},
		{pattern: "*.swp"},
		{pattern: "*.swo"},
		{pattern: "*~"},
		{pattern: ".project"},
		{pattern: ".settings/"},
		{pattern: "*.suo"},
		{pattern: "*.user"},

		// Secrets — intentionally noisy so they're hard to miss
		{comment: "Secrets — do not remove these", pattern: ".env"},
		{pattern: ".env.local"},
		{pattern: ".env.*.local"},
		{pattern: "*.pem"},
		{pattern: "*.key"},
		{pattern: "*.p12"},
		{pattern: "*.pfx"},

		// Logs and temp
		{comment: "Logs & temp", pattern: "*.log"},
		{pattern: "*.tmp"},
		{pattern: "*.temp"},
		{pattern: "npm-debug.log*"},
		{pattern: "yarn-debug.log*"},
		{pattern: "yarn-error.log*"},
	}
}

// SupportedLangs lists the lang slugs accepted by --lang.
var SupportedLangs = []string{"go", "node", "python", "dotnet", "java"}

func langPatterns(lang string) []seedEntry {
	switch strings.ToLower(lang) {
	case "go":
		return []seedEntry{
			{comment: "Go", pattern: "bin/"},
			{pattern: "*.exe"},
			{pattern: "*.test"},
			{pattern: "*.out"},
			{pattern: "coverage.out"},
		}
	case "node", "nodejs", "js", "javascript", "typescript", "ts":
		return []seedEntry{
			{comment: "Node", pattern: "node_modules/"},
			{pattern: "dist/"},
			{pattern: ".next/"},
			{pattern: ".nuxt/"},
			{pattern: "build/"},
			{pattern: ".cache/"},
			{pattern: "coverage/"},
			{pattern: ".pnp.*"},
			{pattern: ".yarn/cache"},
		}
	case "python", "py":
		return []seedEntry{
			{comment: "Python", pattern: "__pycache__/"},
			{pattern: "*.py[cod]"},
			{pattern: ".venv/"},
			{pattern: "venv/"},
			{pattern: "*.egg-info/"},
			{pattern: "dist/"},
			{pattern: ".pytest_cache/"},
			{pattern: ".mypy_cache/"},
			{pattern: ".ruff_cache/"},
		}
	case "dotnet", ".net", "csharp", "cs":
		return []seedEntry{
			{comment: ".NET", pattern: "bin/"},
			{pattern: "obj/"},
			{pattern: "*.user"},
			{pattern: "*.suo"},
			{pattern: ".vs/"},
			{pattern: "TestResults/"},
			{pattern: "publish/"},
		}
	case "java":
		return []seedEntry{
			{comment: "Java", pattern: "target/"},
			{pattern: "*.class"},
			{pattern: "*.jar"},
			{pattern: "*.war"},
			{pattern: ".gradle/"},
			{pattern: "build/"},
			{pattern: ".idea/"},
		}
	}
	return nil
}
