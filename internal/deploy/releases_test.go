package deploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/database"
	"github.com/matrixn/zion-release-station/internal/sites"
)

func TestRollbackRestoresPreviousReleaseAndRecordsDeployment(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	root := t.TempDir()
	store := sites.NewStore(db)
	site, err := store.Create(context.Background(), sites.Input{Name: "Example", Slug: "example", ProjectRoot: root, WebRoot: root, Framework: "php", Strategy: "atomic", Status: "active"})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	releasesRoot := filepath.Join(root, ".zion", "releases")
	if err := os.MkdirAll(filepath.Join(releasesRoot, "rel-old"), 0o755); err != nil {
		t.Fatalf("create old release: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(releasesRoot, "rel-new"), 0o755); err != nil {
		t.Fatalf("create new release: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releasesRoot, "rel-old", "index.php"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write old release: %v", err)
	}
	if err := os.WriteFile(filepath.Join(releasesRoot, "rel-new", "index.php"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write new release: %v", err)
	}
	if err := os.Symlink(filepath.Join(".zion", "releases", "rel-new"), filepath.Join(root, ".current")); err != nil {
		t.Fatalf("activate new current link: %v", err)
	}
	for _, deployment := range []struct{ id, sha, message string }{{"dep-old", "oldsha", "Old release"}, {"dep-new", "newsha", "New release"}} {
		if _, err := db.Exec(`INSERT INTO deployments(id, site_id, trigger_type, branch, commit_sha, commit_message, commit_url, deployment_method, status, queued_at, started_at, finished_at, created_at) VALUES (?, ?, 'manual', 'main', ?, ?, '', 'manual', 'deployed', datetime('now'), datetime('now'), datetime('now'), datetime('now'))`, deployment.id, site.ID, deployment.sha, deployment.message); err != nil {
			t.Fatalf("insert deployment: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO releases(id, site_id, deployment_id, release_name, release_path, commit_sha, active, health_status, created_at, activated_at) VALUES ('rel-old', ?, 'dep-old', 'rel-old', ?, 'oldsha', 0, 'healthy', datetime('now', '-1 hour'), datetime('now', '-1 hour')), ('rel-new', ?, 'dep-new', 'rel-new', ?, 'newsha', 1, 'healthy', datetime('now'), datetime('now'))`, site.ID, filepath.Join(releasesRoot, "rel-old"), site.ID, filepath.Join(releasesRoot, "rel-new")); err != nil {
		t.Fatalf("insert releases: %v", err)
	}
	runner := NewRunner(db, fakeDownloader{})
	result, err := runner.Rollback(context.Background(), site, "rel-old")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if result.Status != "deployed" || result.ReleaseID != "rel-old" {
		t.Fatalf("unexpected rollback result: %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(root, "index.php"))
	if err != nil || string(content) != "old" {
		t.Fatalf("previous release was not published: %q (%v)", content, err)
	}
	var active string
	if err := db.QueryRow(`SELECT id FROM releases WHERE site_id = ? AND active = 1`, site.ID).Scan(&active); err != nil || active != "rel-old" {
		t.Fatalf("unexpected active release %q (%v)", active, err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM deployments WHERE id = ?`, result.DeploymentID).Scan(&status); err != nil || status != "deployed" {
		t.Fatalf("rollback deployment was not recorded: %q (%v)", status, err)
	}
}

func TestCheckHealthAcceptsSuccessfulHTTPAndRejectsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := checkHealth(context.Background(), server.URL); err != nil {
		t.Fatalf("successful health check failed: %v", err)
	}
	if err := checkHealth(context.Background(), server.URL+"/missing"); err == nil {
		t.Fatal("failed health check unexpectedly passed")
	}
}

func TestCheckHealthRejectsUnsafeURL(t *testing.T) {
	if err := checkHealth(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("unsafe health URL unexpectedly accepted")
	}
}
