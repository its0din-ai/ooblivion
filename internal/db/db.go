// Package db manages the SQLite database and its embedded migrations.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

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
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		raw, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := handle.Exec(string(raw)); err != nil {
			return fmt.Errorf("migrate %s: %w", entry.Name(), err)
		}
	}
	return nil
}
