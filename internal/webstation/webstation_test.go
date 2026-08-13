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

func TestFilesystemAdapterSkipsPermissionDeniedRootWhenAnotherRootIsReadable(t *testing.T) {
	readableRoot := t.TempDir()
	restrictedRoot := t.TempDir()
	site := filepath.Join(readableRoot, "example.test")
	if err := os.Mkdir(site, 0o755); err != nil {
		t.Fatalf("mkdir site: %v", err)
	}
	if err := os.WriteFile(filepath.Join(site, "wp-config.php"), []byte("<?php"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.Chmod(restrictedRoot, 0o000); err != nil {
		t.Skipf("chmod unavailable: %v", err)
	}
	defer os.Chmod(restrictedRoot, 0o755)
	if _, err := os.ReadDir(restrictedRoot); !os.IsPermission(err) {
		t.Skip("test process can still read the restricted directory")
	}

	adapter := NewFilesystemAdapter([]string{restrictedRoot, readableRoot}, detection.Registry{})
	sites, err := adapter.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(sites) != 1 || sites[0].ProjectRoot != site {
		t.Fatalf("expected readable root to be discovered, got %#v", sites)
	}
}
