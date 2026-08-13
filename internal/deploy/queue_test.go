package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/database"
	"github.com/matrixn/zion-release-station/internal/sites"
)

func TestQueueDeduplicatesActiveCommit(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	installationID := int64(7)
	store := sites.NewStore(db)
	site, err := store.Create(context.Background(), sites.Input{
		Name: "Queued", Slug: "queued", ProjectRoot: t.TempDir(), Framework: "php", Strategy: "atomic", Status: "active",
		Repository: &sites.RepositoryInput{Provider: "github", CloneURL: "https://github.com/example/queued.git", Branch: "main", GitHubInstallationID: &installationID, GitHubFullName: "example/queued"},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	runner := NewRunner(db, fakeDownloader{})
	queue := NewQueue(db, runner, store.Get, 1)
	first, err := queue.Enqueue(context.Background(), site, "abc123", "manual", "manual")
	if err != nil {
		t.Fatalf("enqueue first deployment: %v", err)
	}
	second, err := queue.Enqueue(context.Background(), site, "abc123", "manual", "manual")
	if err != nil {
		t.Fatalf("enqueue duplicate deployment: %v", err)
	}
	if first.ID == "" || second.ID != first.ID || first.Status != "queued" {
		t.Fatalf("unexpected queue records: %#v %#v", first, second)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM deployments WHERE site_id = ?`, site.ID).Scan(&count); err != nil {
		t.Fatalf("count deployments: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one active deployment, got %d", count)
	}
}
