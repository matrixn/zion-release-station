package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/database"
	"github.com/matrixn/zion-release-station/internal/sites"
)

func TestGitHubWebhookValidatesHMACQueuesAndRejectsReplay(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "releasestation.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	server := NewServer(config.Config{DataDir: t.TempDir(), WebRoot: t.TempDir(), Version: "0.1.0-test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	installationID := int64(9)
	site, err := server.sites.Create(context.Background(), sites.Input{
		Name: "Webhook site", Slug: "webhook-site", ProjectRoot: t.TempDir(), Framework: "php", Strategy: "atomic", Status: "active", PushToDeploy: true,
		Repository: &sites.RepositoryInput{Provider: "github", CloneURL: "https://github.com/example/webhook-site.git", Branch: "main", GitHubInstallationID: &installationID, GitHubFullName: "example/webhook-site"},
	})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/releasestation/api/v1/sites/"+site.ID+"/webhook", strings.NewReader(`{"provider":"github"}`))
	request.Header.Set("Content-Type", "application/json")
	server.http.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("rotate webhook: %d %s", recorder.Code, recorder.Body.String())
	}
	var rotate struct {
		Data struct {
			Webhook struct {
				Endpoint string `json:"endpoint"`
			} `json:"webhook"`
			Secret string `json:"secret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &rotate); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if rotate.Data.Secret == "" || rotate.Data.Webhook.Endpoint == "" {
		t.Fatalf("incomplete webhook credentials: %#v", rotate)
	}

	body := []byte(`{"ref":"refs/heads/main","after":"abc123","repository":{"full_name":"example/webhook-site"}}`)
	hasher := hmac.New(sha256.New, []byte(rotate.Data.Secret))
	_, _ = hasher.Write(body)
	delivery := "delivery-test-1"
	webhookRequest := httptest.NewRequest(http.MethodPost, rotate.Data.Webhook.Endpoint, strings.NewReader(string(body)))
	webhookRequest.Header.Set("X-GitHub-Event", "push")
	webhookRequest.Header.Set("X-GitHub-Delivery", delivery)
	webhookRequest.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(hasher.Sum(nil)))
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, webhookRequest)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"status":"queued"`) {
		t.Fatalf("valid webhook: %d %s", recorder.Code, recorder.Body.String())
	}

	webhookRequest = httptest.NewRequest(http.MethodPost, rotate.Data.Webhook.Endpoint, strings.NewReader(string(body)))
	webhookRequest.Header.Set("X-GitHub-Event", "push")
	webhookRequest.Header.Set("X-GitHub-Delivery", delivery)
	webhookRequest.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(hasher.Sum(nil)))
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, webhookRequest)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"already_processed"`) {
		t.Fatalf("replayed webhook: %d %s", recorder.Code, recorder.Body.String())
	}

	badRequest := httptest.NewRequest(http.MethodPost, rotate.Data.Webhook.Endpoint, strings.NewReader(string(body)))
	badRequest.Header.Set("X-GitHub-Event", "push")
	badRequest.Header.Set("X-GitHub-Delivery", "delivery-test-2")
	badRequest.Header.Set("X-Hub-Signature-256", "sha256="+strings.Repeat("0", 64))
	recorder = httptest.NewRecorder()
	server.http.Handler.ServeHTTP(recorder, badRequest)
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), `"INVALID_WEBHOOK_SIGNATURE"`) {
		t.Fatalf("invalid webhook: %d %s", recorder.Code, recorder.Body.String())
	}

	var deploymentCount int
	if err := db.QueryRow(`SELECT count(*) FROM deployments WHERE site_id = ?`, site.ID).Scan(&deploymentCount); err != nil {
		t.Fatalf("count deployments: %v", err)
	}
	if deploymentCount != 1 {
		t.Fatalf("expected one queued deployment after replay protection, got %d", deploymentCount)
	}
}
