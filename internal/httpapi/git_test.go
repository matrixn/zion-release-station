package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/database"
)

func TestGitDeployKeyAPIOnlyReturnsPublicKey(t *testing.T) {
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), filepath.Join(dataDir, "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	server := NewServer(config.Config{DataDir: dataDir, WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/git/generate-deploy-key", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"algorithm":"ed25519"`) || !strings.Contains(recorder.Body.String(), `"public_key":"ssh-ed25519 `) {
		t.Fatalf("unexpected deploy key response: %d %q", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "PRIVATE KEY") {
		t.Fatal("private key leaked in API response")
	}
}

func TestGitTestAPIRejectsInvalidRemoteBeforeRunningGit(t *testing.T) {
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), filepath.Join(dataDir, "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	server := NewServer(config.Config{DataDir: dataDir, WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/git/test", strings.NewReader(`{"clone_url":"https://github.com/org/repo.git;echo hacked","branch":"main"}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"code":"INVALID_REPOSITORY"`) {
		t.Fatalf("unexpected invalid remote response: %d %q", recorder.Code, recorder.Body.String())
	}
}
