package pathsecurity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalDirectoryRejectsRelativePath(t *testing.T) {
	if _, err := CanonicalDirectory("../../etc"); err == nil {
		t.Fatal("expected relative path to be rejected")
	}
}

func TestCanonicalDirectoryResolvesExistingDirectory(t *testing.T) {
	root := t.TempDir()
	resolved, err := CanonicalDirectory(root)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if !filepath.IsAbs(resolved) {
		t.Fatalf("expected absolute path, got %q", resolved)
	}
}

func TestIsWithinRejectsEscapesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "public")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !IsWithin(root, child) || IsWithin(root, filepath.Join(root, "public", "..", "..", "etc")) {
		t.Fatal("unexpected containment result")
	}
	outside := t.TempDir()
	symlink := filepath.Join(root, "escape")
	if err := os.Symlink(outside, symlink); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	canonical, err := CanonicalDirectory(symlink)
	if err != nil {
		t.Fatalf("canonicalize symlink: %v", err)
	}
	if IsWithin(root, canonical) {
		t.Fatal("symlink escape was accepted")
	}
}
