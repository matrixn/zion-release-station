package githubconnector

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matrixn/zion-release-station/internal/config"
)

func TestDownloadArchiveUsesInstanceCredentialAndRepositoryQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/rs_test/github/repositories/acme/site/archive" {
			t.Fatalf("unexpected archive path %q", r.URL.Path)
		}
		if r.URL.Query().Get("installation_id") != "42" || r.URL.Query().Get("ref") != "main" {
			t.Fatalf("unexpected archive query %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer credential" {
			t.Fatalf("unexpected authorization %q", got)
		}
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = io.WriteString(w, "archive-bytes")
	}))
	defer server.Close()

	client := NewClient(config.Config{GitHubConnectorURL: server.URL, GitHubConnectorToken: "credential", InstanceID: "rs_test"})
	var output bytes.Buffer
	if err := client.DownloadArchive(context.Background(), 42, "acme/site", "main", &output); err != nil {
		t.Fatalf("download archive: %v", err)
	}
	if output.String() != "archive-bytes" {
		t.Fatalf("unexpected archive body %q", output.String())
	}
}

func TestCommitsDecodesConnectorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/rs_test/github/repositories/acme/site/commits" {
			t.Fatalf("unexpected commits path %q", r.URL.Path)
		}
		if r.URL.Query().Get("installation_id") != "42" || r.URL.Query().Get("branch") != "main" {
			t.Fatalf("unexpected commits query %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"commits":[{"sha":"abc123","message":"Initial commit","branch":"main"}]}`)
	}))
	defer server.Close()

	client := NewClient(config.Config{GitHubConnectorURL: server.URL, GitHubConnectorToken: "credential", InstanceID: "rs_test"})
	commits, err := client.Commits(context.Background(), 42, "acme/site", "main", 50)
	if err != nil {
		t.Fatalf("read commits: %v", err)
	}
	if len(commits) != 1 || commits[0].SHA != "abc123" {
		t.Fatalf("unexpected commits: %#v", commits)
	}
}
