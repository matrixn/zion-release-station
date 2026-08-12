package deploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/database"
	"github.com/matrixn/zion-release-station/internal/sites"
)

type fakeDownloader struct{ archive []byte }

func (f fakeDownloader) DownloadArchive(_ context.Context, _ int64, _, _ string, target io.Writer) error {
	_, err := target.Write(f.archive)
	return err
}

func TestDeployGitHubExtractsAndSwitchesCurrentAtomically(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	root := t.TempDir()
	installationID := int64(42)
	store := sites.NewStore(db)
	site, err := store.Create(context.Background(), sites.Input{
		Name:        "Example",
		Slug:        "example",
		ProjectRoot: root,
		WebRoot:     filepath.Join(root, "current"),
		Framework:   "static",
		Strategy:    "atomic",
		Status:      "active",
		Repository: &sites.RepositoryInput{
			Provider:             "github",
			CloneURL:             "https://github.com/example/site.git",
			Branch:               "main",
			GitHubInstallationID: &installationID,
			GitHubFullName:       "example/site",
		},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	runner := NewRunner(db, fakeDownloader{archive: testArchive(t, map[string]string{"index.php": "release-one", "nested/file.txt": "nested"})})
	result, err := runner.DeployGitHub(context.Background(), site)
	if err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("unexpected result: %#v", result)
	}
	current, err := filepath.EvalSymlinks(filepath.Join(root, "current"))
	if err != nil {
		t.Fatalf("resolve current: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(current, "index.php"))
	if err != nil || string(content) != "release-one" {
		t.Fatalf("unexpected deployed file: %q %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".zion", "releases", result.ReleaseID)); err != nil {
		t.Fatalf("release was not retained: %v", err)
	}
}

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for name, content := range files {
		if err := writer.WriteHeader(&tar.Header{Name: "example-site-abc/" + name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("write archive header: %v", err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatalf("write archive content: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}
