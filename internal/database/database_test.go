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

	var migrationCount int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("read migration count: %v", err)
	}
	if migrationCount != 3 {
		t.Fatalf("unexpected migration count %d", migrationCount)
	}

	var managedCleanupApplied int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = '0003_managed_github_only'`).Scan(&managedCleanupApplied); err != nil {
		t.Fatalf("read managed GitHub migration: %v", err)
	}
	if managedCleanupApplied != 1 {
		t.Fatal("managed GitHub cleanup migration was not applied")
	}

	var tableCount int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'deployments'`).Scan(&tableCount); err != nil {
		t.Fatalf("read deployment table: %v", err)
	}
	if tableCount != 1 {
		t.Fatal("foundation schema did not create deployments table")
	}
}
