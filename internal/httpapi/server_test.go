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

func TestWebStationDiscoveryAndSiteCRUDAPI(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	webStationRoot := t.TempDir()
	siteRoot := filepath.Join(webStationRoot, "example.test")
	for _, relative := range []string{"wp-config.php", "wp-admin/index.php", "wp-content/index.php"} {
		path := filepath.Join(siteRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	server := NewServer(config.Config{WebRoot: t.TempDir(), WebStationRoots: []string{webStationRoot}, Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/webstation/discover", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"framework":"wordpress"`) {
		t.Fatalf("unexpected discovery response: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/webstation/import", strings.NewReader(`{"paths":["`+siteRoot+`"]}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"imported"`) {
		t.Fatalf("unexpected import response: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/sites", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"hostname":"example.test"`) {
		t.Fatalf("unexpected sites response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestGitHubConnectionSettingsAPI(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	server := NewServer(config.Config{WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/integrations/github", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"connected":false`) {
		t.Fatalf("unexpected initial GitHub status: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/install", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"GITHUB_APP_NOT_CONFIGURED"`) {
		t.Fatalf("unexpected GitHub install response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestGitHubAppConfigurationIsManagedByAPI(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	server := NewServer(config.Config{WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/releasestation/api/v1/integrations/github/config", strings.NewReader(`{"app_id":"123","app_slug":"zion-releasestation","setup_url":"https://example.test/releasestation/api/v1/integrations/github/setup"}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"app_slug":"zion-releasestation"`) {
		t.Fatalf("unexpected GitHub config response: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/integrations/github", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"app_id":"123"`) || !strings.Contains(recorder.Body.String(), `"private_key_configured":false`) {
		t.Fatalf("GitHub config was not persisted: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/private-key", strings.NewReader("not-a-pem"))
	request.Header.Set("Content-Type", "multipart/form-data")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid private key upload to fail, got %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestManualSiteAPIAutoDetectsFrameworkAndSavesRepository(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	projectRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(projectRoot, "public"), 0o755); err != nil {
		t.Fatalf("mkdir public: %v", err)
	}
	for _, relative := range []string{"artisan", "composer.json", "public/index.php"} {
		path := filepath.Join(projectRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	server := NewServer(config.Config{WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := `{"name":"Example","url":"https://example.test","project_root":"` + projectRoot + `","framework":"auto","strategy":"atomic","repository":{"provider":"github","clone_url":"https://github.com/example/site.git","branch":"main"}}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/sites", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"framework":"laravel"`) || !strings.Contains(recorder.Body.String(), `"clone_url":"https://github.com/example/site.git"`) {
		t.Fatalf("unexpected manual site response: %d %q", recorder.Code, recorder.Body.String())
	}
}
