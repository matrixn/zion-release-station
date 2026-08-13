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
	if result.Status != "deployed" {
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
	if info, err := os.Lstat(filepath.Join(root, ".current")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".current staging link was not prepared: %v", err)
	}
	deployment, err := runner.GetDeployment(context.Background(), site.ID, result.DeploymentID)
	if err != nil {
		t.Fatalf("read deployment history: %v", err)
	}
	if deployment.Status != "deployed" || deployment.DeploymentMethod != "manual" || deployment.BuildLog == "" || deployment.DeploymentLog == "" {
		t.Fatalf("unexpected deployment history: %#v", deployment)
	}
}

func TestDeployGitHubRunsCustomScript(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	root := t.TempDir()
	installationID := int64(42)
	store := sites.NewStore(db)
	site, err := store.Create(context.Background(), sites.Input{
		Name: "Custom", Slug: "custom", ProjectRoot: root, WebRoot: filepath.Join(root, "current"), Framework: "php", Strategy: "atomic", Status: "active",
		DeployScript: "set -eu\nmkdir -p \"$WEB_ROOT\"\nprintf custom > \"$WEB_ROOT/custom.txt\"\n",
		Repository:   &sites.RepositoryInput{Provider: "github", CloneURL: "https://github.com/example/site.git", Branch: "main", GitHubInstallationID: &installationID, GitHubFullName: "example/site"},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	runner := NewRunner(db, fakeDownloader{archive: testArchive(t, map[string]string{"index.php": "release"})})
	result, err := runner.DeployGitHub(context.Background(), site)
	if err != nil {
		t.Fatalf("deploy with custom script: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(root, "current", "custom.txt"))
	if err != nil || string(content) != "custom" {
		t.Fatalf("custom script did not run: %q %v", content, err)
	}
	deployment, err := runner.GetDeployment(context.Background(), site.ID, result.DeploymentID)
	if err != nil || !bytes.Contains([]byte(deployment.BuildLog), []byte("custom deployment script")) {
		t.Fatalf("custom script was not recorded: %#v %v", deployment, err)
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
