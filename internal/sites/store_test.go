package sites

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/database"
)

func TestStoreCreatesUpdatesListsAndArchivesSite(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	site, err := store.Create(context.Background(), Input{Name: "Example", Slug: "example", Hostname: "example.test", ProjectRoot: "/volume1/www/example", WebRoot: "/volume1/www/example", Framework: "wordpress", Strategy: "in_place", Status: "active", Runtime: map[string]any{"source": "manual"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if site.ID == "" || site.Runtime == nil {
		t.Fatalf("unexpected site: %#v", site)
	}

	site, err = store.Update(context.Background(), site.ID, Input{Name: "Example Updated", Slug: "example-updated", Hostname: "example.test", ProjectRoot: site.ProjectRoot, WebRoot: site.WebRoot, Framework: "wordpress", Strategy: "atomic", Status: "active"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if site.Name != "Example Updated" || site.Strategy != "atomic" {
		t.Fatalf("unexpected updated site: %#v", site)
	}

	listed, err := store.List(context.Background())
	if err != nil || len(listed) != 1 {
		t.Fatalf("list: sites=%#v err=%v", listed, err)
	}
	if err := store.Archive(context.Background(), site.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	listed, err = store.List(context.Background())
	if err != nil || len(listed) != 0 {
		t.Fatalf("expected archived site to be hidden, sites=%#v err=%v", listed, err)
	}
}

func TestStorePersistsRepositorySelectionWithSite(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	site, err := store.Create(context.Background(), Input{
		Name:        "Example",
		Slug:        "example",
		ProjectRoot: "/volume1/www/example",
		WebRoot:     "/volume1/www/example",
		Framework:   "wordpress",
		Strategy:    "in_place",
		Status:      "active",
		Repository:  &RepositoryInput{Provider: "github", CloneURL: "https://github.com/example/site.git", Branch: "main"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if site.Repository == nil || site.Repository.Provider != "github" || site.Repository.Branch != "main" {
		t.Fatalf("repository was not persisted: %#v", site.Repository)
	}

	listed, err := store.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Repository == nil {
		t.Fatalf("repository was not loaded with site list: sites=%#v err=%v", listed, err)
	}
}

func TestStorePersistsReleaseManagementSettings(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	site, err := store.Create(context.Background(), Input{
		Name:              "Release site",
		Slug:              "release-site",
		ProjectRoot:       "/volume1/www/release-site",
		WebRoot:           "/volume1/www/release-site",
		Framework:         "php",
		Strategy:          "atomic",
		Status:            "active",
		HealthCheckURL:    "https://release-site.test/healthz",
		SharedDirectories: []string{"storage", "bootstrap/cache"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	loaded, err := store.Get(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded.HealthCheckURL != "https://release-site.test/healthz" {
		t.Fatalf("unexpected health check URL: %q", loaded.HealthCheckURL)
	}
	if len(loaded.SharedDirectories) != 2 || loaded.SharedDirectories[1] != "bootstrap/cache" {
		t.Fatalf("unexpected shared directories: %#v", loaded.SharedDirectories)
	}

	if _, err := store.Update(context.Background(), site.ID, Input{
		Name:              loaded.Name,
		Slug:              loaded.Slug,
		ProjectRoot:       loaded.ProjectRoot,
		WebRoot:           loaded.WebRoot,
		Framework:         loaded.Framework,
		Strategy:          loaded.Strategy,
		Status:            loaded.Status,
		HealthCheckURL:    "",
		SharedDirectories: []string{"storage"},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	loaded, err = store.Get(context.Background(), site.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if loaded.HealthCheckURL != "" || len(loaded.SharedDirectories) != 1 || loaded.SharedDirectories[0] != "storage" {
		t.Fatalf("release management settings were not updated: %#v", loaded)
	}
}

func TestStoreRejectsUnsafeReleaseManagementSettings(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	_, err = store.Create(context.Background(), Input{
		Name:              "Unsafe",
		Slug:              "unsafe",
		ProjectRoot:       "/volume1/www/unsafe",
		WebRoot:           "/volume1/www/unsafe",
		Framework:         "php",
		Strategy:          "atomic",
		Status:            "active",
		HealthCheckURL:    "file:///etc/passwd",
		SharedDirectories: []string{"../secrets"},
	})
	if err == nil {
		t.Fatal("expected unsafe release management settings to be rejected")
	}
}
