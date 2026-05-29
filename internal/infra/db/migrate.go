package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrDisabled is returned when a DB-only operation is requested while DB is disabled.
var ErrDisabled = errors.New("database is disabled")

// MigrateUp applies all pending *.up.sql migrations in lexical order.
func MigrateUp(ctx context.Context, db *sql.DB, dir string) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	files, err := migrationFiles(dir, ".up.sql")
	if err != nil {
		return err
	}
	for _, file := range files {
		version := migrationVersion(file, ".up.sql")
		applied, err := migrationApplied(ctx, db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, db, file, version, true); err != nil {
			return err
		}
	}
	return nil
}

// MigrateDown reverts the latest applied migration if a matching down file exists.
func MigrateDown(ctx context.Context, db *sql.DB, dir string) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	version, ok, err := latestMigration(ctx, db)
	if err != nil || !ok {
		return err
	}
	file := filepath.Join(dir, version+".down.sql")
	if _, err := os.Stat(file); err != nil {
		return err
	}
	return applyMigration(ctx, db, file, version, false)
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(255) PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`)
	return err
}

func migrationFiles(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func latestMigration(ctx context.Context, db *sql.DB) (string, bool, error) {
	var version string
	err := db.QueryRowContext(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return version, true, nil
}

func applyMigration(ctx context.Context, db *sql.DB, file, version string, up bool) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", file, err)
	}
	if up {
		_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", version)
	} else {
		_, err = tx.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ?", version)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func migrationVersion(file, suffix string) string {
	return strings.TrimSuffix(filepath.Base(file), suffix)
}
