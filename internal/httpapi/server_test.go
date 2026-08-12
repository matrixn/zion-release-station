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
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"mode":"managed"`) || !strings.Contains(recorder.Body.String(), `"connected":false`) {
		t.Fatalf("unexpected initial GitHub status: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/install", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), `"GITHUB_CONNECTOR_UNAVAILABLE"`) {
		t.Fatalf("unexpected GitHub install response: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestLegacyGitHubAppConfigurationRoutesAreRemoved(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	server := NewServer(config.Config{WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/releasestation/api/v1/integrations/github/config", strings.NewReader(`{"app_id":"123"}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected legacy GitHub config route to be removed, got %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/private-key", strings.NewReader("not-a-pem"))
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected legacy private key route to be removed, got %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestManagedGitHubConnectorStartsSessionAndReportsConnection(t *testing.T) {
	connector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer connector-test-token" {
			t.Fatalf("unexpected connector authorization %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/instances/test-instance/github/sessions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"session_123","authorize_url":"https://github.com/apps/zion/installations/new?state=test","expires_in":600}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/instances/test-instance/github/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"state":"connected","account_login":"matrixn","installations":[{"github_installation_id":42,"account_login":"matrixn","account_type":"User","repository_selection":"selected"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer connector.Close()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	server := NewServer(config.Config{
		WebRoot:              t.TempDir(),
		Version:              "0.1.0-test",
		GitHubConnectorURL:   connector.URL,
		GitHubConnectorToken: "connector-test-token",
		InstanceID:           "test-instance",
	}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/install", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"session_id":"session_123"`) || !strings.Contains(recorder.Body.String(), `"mode":"managed"`) {
		t.Fatalf("unexpected managed install response: %d %q", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/releasestation/api/v1/integrations/github", nil)
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"mode":"managed"`) || !strings.Contains(recorder.Body.String(), `"connected":true`) || !strings.Contains(recorder.Body.String(), `"account_login":"matrixn"`) {
		t.Fatalf("unexpected managed GitHub status: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestGitHubConnectorStartsPairingWithoutProvisionedCredential(t *testing.T) {
	connector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("pairing endpoint must not receive an instance credential, got %q", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/pairing/sessions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "https://raduta.synology.me:5001/webman/3rdparty/zion-releasestation/index.html") {
			t.Fatalf("pairing request did not preserve native callback URL: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"rs_test","authorize_url":"https://github.com/apps/zion/installations/new?state=test","expires_in":600}`)
	}))
	defer connector.Close()

	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	server := NewServer(config.Config{
		WebRoot:            t.TempDir(),
		DataDir:            t.TempDir(),
		Version:            "0.1.0-test",
		GitHubConnectorURL: connector.URL,
		InstanceID:         "rs_test",
	}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/integrations/github/install", strings.NewReader(`{"return_url":"https://raduta.synology.me:5001/webman/3rdparty/zion-releasestation/index.html"}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"mode":"pairing"`) || !strings.Contains(recorder.Body.String(), `"session_id":"rs_test"`) {
		t.Fatalf("unexpected pairing install response: %d %q", recorder.Code, recorder.Body.String())
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
