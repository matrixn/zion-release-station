package webstation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/detection"
)

func TestFilesystemAdapterDiscoversSitesAndLaravelDocumentRoot(t *testing.T) {
	root := t.TempDir()
	site := filepath.Join(root, "example.test")
	for _, relative := range []string{"artisan", "composer.json", "public/index.php"} {
		path := filepath.Join(site, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	adapter := NewFilesystemAdapter([]string{root}, detection.Registry{})
	available, err := adapter.Available(context.Background())
	if err != nil || !available {
		t.Fatalf("expected adapter to be available, available=%t err=%v", available, err)
	}
	sites, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(sites) != 1 || sites[0].Framework != "laravel" || sites[0].WebRoot != filepath.Join(site, "public") {
		t.Fatalf("unexpected sites: %#v", sites)
	}
}

func TestFilesystemAdapterSkipsSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked.test")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	adapter := NewFilesystemAdapter([]string{root}, detection.Registry{})
	sites, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(sites) != 0 {
		t.Fatalf("expected symlinked site to be skipped, got %#v", sites)
	}
}
