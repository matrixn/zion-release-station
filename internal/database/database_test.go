package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesFoundationMigration(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow(`SELECT version FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != "0001_foundation" {
		t.Fatalf("unexpected migration version %q", version)
	}

	var tableCount int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'deployments'`).Scan(&tableCount); err != nil {
		t.Fatalf("read deployment table: %v", err)
	}
	if tableCount != 1 {
		t.Fatal("foundation schema did not create deployments table")
	}
}
