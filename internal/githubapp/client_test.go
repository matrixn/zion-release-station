package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/matrixn/zion-release-station/internal/config"
)

func TestClientRequiresReadableGitHubAppConfiguration(t *testing.T) {
	client := NewClient(config.Config{GitHubAppID: "123", GitHubAppSlug: "zion", GitHubSetupURL: "https://example.test/setup", GitHubPrivateKeyPath: filepath.Join(t.TempDir(), "missing.pem")})
	if client.Configured() {
		t.Fatal("expected missing private key to leave the App unconfigured")
	}
	if client.ConfigurationError() != "GitHub App private key is not readable" {
		t.Fatalf("unexpected configuration error: %q", client.ConfigurationError())
	}
}

func TestClientParsesPKCS1AndGeneratesAppJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "github-app.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	client := NewClient(config.Config{GitHubAppID: "123", GitHubAppSlug: "zion", GitHubSetupURL: "https://example.test/setup", GitHubPrivateKeyPath: keyPath})
	if !client.Configured() {
		t.Fatalf("expected configured client, error=%q", client.ConfigurationError())
	}
	token, err := client.appJWT()
	if err != nil || len(token) < 20 {
		t.Fatalf("expected signed JWT, token=%q err=%v", token, err)
	}
}
