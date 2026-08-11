package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/database"
)

func TestHealthEndpointReportsReadyDatabase(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	cfg := config.Config{
		BindAddress: "127.0.0.1:24871",
		DataDir:     t.TempDir(),
		WebRoot:     t.TempDir(),
		Version:     "0.1.0-test",
	}
	server := NewServer(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/system/health", nil)
	server.http.Handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	if body := recorder.Body.String(); body == "" || !strings.Contains(body, `"status":"healthy"`) {
		t.Fatalf("unexpected health body %q", body)
	}
}

func TestWebAccessSettingHidesWebWorkspace(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("workspace"), 0o644); err != nil {
		t.Fatalf("write test workspace: %v", err)
	}
	server := NewServer(config.Config{WebRoot: webRoot, Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	request := httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/settings/web-access", nil)
	recorder := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"enabled":true`) {
		t.Fatalf("expected web access to be enabled by default, got %d %q", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodPut, "/releasestation/api/v1/settings/web-access", strings.NewReader(`{"enabled":false}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected settings update to succeed, got %d %q", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/releasestation/", nil)
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected workspace to be hidden, got %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodPut, "/releasestation/api/v1/settings/web-access", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected settings re-enable to succeed, got %d %q", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/releasestation/", nil)
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "workspace" {
		t.Fatalf("expected workspace after re-enable, got %d %q", recorder.Code, recorder.Body.String())
	}
}
