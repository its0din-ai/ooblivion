// Package db manages the SQLite database and its embedded migrations.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
		// #nosec G302 -- 0700 is the correct permission for a directory
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("chmod data dir: %w", err)
		}
	}

	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	handle, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := handle.Ping(); err != nil {
		return nil, err
	}
	if err := migrate(handle); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("chmod db: %w", err)
	}
	return handle, nil
}

func migrate(handle *sql.DB) error {
	if _, err := handle.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return err
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		var applied int
		if err := handle.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", entry.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}

		raw, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := handle.Exec(string(raw)); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				log.Printf("migration %s already applied, skipping", entry.Name())
			} else {
				return fmt.Errorf("migrate %s: %w", entry.Name(), err)
			}
		}
		if _, err := handle.Exec(
			"INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)",
			entry.Name(), time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return err
		}
	}
	return nil
}
