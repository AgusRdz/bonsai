package tui

import (
	"os"
	"path/filepath"
)

// detectPostCreateCmds scans dir for known dependency manifests and returns
// a suggested list of post-create commands as a starting point for the user.
func detectPostCreateCmds(dir string) []string {
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	globExists := func(pattern string) bool {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		return err == nil && len(matches) > 0
	}

	var cmds []string

	// Copy .env.example → .env if not already present.
	if exists(".env.example") && !exists(".env") {
		cmds = append(cmds, "cp .env.example .env")
	}

	// PHP / Composer
	if exists("composer.json") {
		cmds = append(cmds, "composer install")
	}

	// Node.js — detect package manager by lock file.
	if exists("package.json") {
		switch {
		case exists("pnpm-lock.yaml"):
			cmds = append(cmds, "pnpm install")
		case exists("yarn.lock"):
			cmds = append(cmds, "yarn install")
		default:
			cmds = append(cmds, "npm install")
		}
	}

	// Go
	if exists("go.mod") {
		cmds = append(cmds, "go mod download")
	}

	// Python
	if exists("requirements.txt") {
		cmds = append(cmds, "pip install -r requirements.txt")
	} else if exists("pyproject.toml") {
		cmds = append(cmds, "pip install -e .")
	}

	// Ruby
	if exists("Gemfile") {
		cmds = append(cmds, "bundle install")
	}

	// .NET
	if globExists("*.sln") || globExists("*.csproj") {
		cmds = append(cmds, "dotnet restore")
	}

	return cmds
}
