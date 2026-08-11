package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := configure(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func configure(ctx context.Context, db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure sqlite with %q: %w", pragma, err)
		}
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	const version = "0001_foundation"
	var applied bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, version).Scan(&applied); err != nil {
		return fmt.Errorf("read migration state: %w", err)
	}
	if applied {
		return nil
	}

	schema, err := migrationFiles.ReadFile("migrations/0001_foundation.sql")
	if err != nil {
		return fmt.Errorf("read foundation migration: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, string(schema)); err != nil {
		tx.Rollback()
		return fmt.Errorf("apply foundation migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, version); err != nil {
		tx.Rollback()
		return fmt.Errorf("record foundation migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit foundation migration: %w", err)
	}
	return nil
}
