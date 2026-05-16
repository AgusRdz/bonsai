// Package metrics provides optional local SQLite-based tracking of git habits,
// errors, convention violations, and commits. All tracking is off by default
// and nothing is sent anywhere - data stays in a local database file.
package metrics

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database for metrics storage.
type DB struct {
	db *sql.DB
}

// DefaultPath returns the path to the metrics database file.
// It lives next to the bonsai config file.
func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "bonsai", "metrics.db"), nil
}

// Open opens (or creates) the metrics database at path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("metrics: mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("metrics: open: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("metrics: migrate: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error { return d.db.Close() }

// RecordError records a git command failure and how it was resolved.
// resolution is a short string like "retry", "abort", "ignored", etc.
func (d *DB) RecordError(command, errorCode, resolution string) error {
	_, err := d.db.Exec(
		`INSERT INTO errors (occurred_at, command, error_code, resolution) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), command, errorCode, resolution,
	)
	return err
}

// RecordViolation records a branch naming convention violation.
func (d *DB) RecordViolation(repo, branch, rule string) error {
	_, err := d.db.Exec(
		`INSERT INTO violations (occurred_at, repo, branch, rule) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), repo, branch, rule,
	)
	return err
}

// RecordCommit records a commit event.
func (d *DB) RecordCommit(repo, branch, mode string) error {
	_, err := d.db.Exec(
		`INSERT INTO commits (occurred_at, repo, branch, mode) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), repo, branch, mode,
	)
	return err
}

// RecordHabit records a TUI session start so time-of-day and frequency can be derived.
func (d *DB) RecordHabit(repo string) error {
	_, err := d.db.Exec(
		`INSERT INTO habits (occurred_at, repo) VALUES (?, ?)`,
		time.Now().UTC().Format(time.RFC3339), repo,
	)
	return err
}

// Summary holds aggregated metrics for display.
type Summary struct {
	TotalCommits    int
	TotalErrors     int
	TotalViolations int
	Sessions        int
	TopErrorCmds    []CommandCount
	TopBranches     []CommandCount
	HourDist        [24]int // commits per hour of day
}

// CommandCount is a name-value pair for aggregated counts.
type CommandCount struct {
	Name  string
	Count int
}

// Summarize returns aggregated metrics.
func (d *DB) Summarize() (*Summary, error) {
	s := &Summary{}

	row := d.db.QueryRow(`SELECT COUNT(*) FROM commits`)
	_ = row.Scan(&s.TotalCommits)

	row = d.db.QueryRow(`SELECT COUNT(*) FROM errors`)
	_ = row.Scan(&s.TotalErrors)

	row = d.db.QueryRow(`SELECT COUNT(*) FROM violations`)
	_ = row.Scan(&s.TotalViolations)

	row = d.db.QueryRow(`SELECT COUNT(*) FROM habits`)
	_ = row.Scan(&s.Sessions)

	// Top error-producing commands.
	rows, err := d.db.Query(
		`SELECT command, COUNT(*) AS n FROM errors GROUP BY command ORDER BY n DESC LIMIT 5`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cc CommandCount
			if err := rows.Scan(&cc.Name, &cc.Count); err == nil {
				s.TopErrorCmds = append(s.TopErrorCmds, cc)
			}
		}
	}

	// Top violated branches.
	rows2, err := d.db.Query(
		`SELECT branch, COUNT(*) AS n FROM violations GROUP BY branch ORDER BY n DESC LIMIT 5`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var cc CommandCount
			if err := rows2.Scan(&cc.Name, &cc.Count); err == nil {
				s.TopBranches = append(s.TopBranches, cc)
			}
		}
	}

	// Hour distribution from commits table.
	rows3, err := d.db.Query(`SELECT occurred_at FROM commits`)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var ts string
			if err := rows3.Scan(&ts); err != nil {
				continue
			}
			t, err := time.Parse(time.RFC3339, ts)
			if err != nil {
				continue
			}
			s.HourDist[t.Local().Hour()]++
		}
	}

	return s, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS errors (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    command     TEXT,
    error_code  TEXT,
    resolution  TEXT
);

CREATE TABLE IF NOT EXISTS violations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    repo        TEXT,
    branch      TEXT,
    rule        TEXT
);

CREATE TABLE IF NOT EXISTS commits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    repo        TEXT,
    branch      TEXT,
    mode        TEXT
);

CREATE TABLE IF NOT EXISTS habits (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    repo        TEXT
);
`)
	return err
}
