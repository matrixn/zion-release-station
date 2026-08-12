package githubapp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
)

const apiBaseURL = "https://api.github.com"
const apiVersion = "2022-11-28"

type Client struct {
	config   config.Config
	http     *http.Client
	mu       sync.Mutex
	configMu sync.RWMutex
	tokens   map[int64]cachedToken
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

type Installation struct {
	GitHubID            int64             `json:"id"`
	AccountLogin        string            `json:"account_login"`
	AccountType         string            `json:"account_type"`
	RepositorySelection string            `json:"repository_selection"`
	Permissions         map[string]string `json:"permissions"`
	SuspendedAt         *string           `json:"suspended_at,omitempty"`
}

type Repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{config: cfg, http: &http.Client{Timeout: 20 * time.Second}, tokens: make(map[int64]cachedToken)}
}

func (c *Client) Config() config.Config {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.config
}

func (c *Client) UpdateConfig(cfg config.Config) {
	c.configMu.Lock()
	c.config = cfg
	c.configMu.Unlock()
}

func (c *Client) Configured() bool {
	cfg := c.Config()
	return cfg.GitHubAppID != "" && cfg.GitHubAppSlug != "" && cfg.GitHubSetupURL != "" && cfg.GitHubPrivateKeyPath != "" && readableFile(cfg.GitHubPrivateKeyPath)
}

func (c *Client) ConfigurationError() string {
	cfg := c.Config()
	switch {
	case cfg.GitHubAppID == "":
		return "RS_GITHUB_APP_ID is not configured"
	case cfg.GitHubAppSlug == "":
		return "RS_GITHUB_APP_SLUG is not configured"
	case cfg.GitHubPrivateKeyPath == "":
		return "RS_GITHUB_APP_PRIVATE_KEY_PATH is not configured"
	case cfg.GitHubSetupURL == "":
		return "RS_GITHUB_SETUP_URL is not configured"
	case !readableFile(cfg.GitHubPrivateKeyPath):
		return "GitHub App private key is not readable"
	default:
		return ""
	}
}

func (c *Client) InstallationURL(state string) (string, error) {
	cfg := c.Config()
	if !c.Configured() {
		return "", fmt.Errorf("GitHub App is not configured: %s", c.ConfigurationError())
	}
	return "https://github.com/apps/" + url.PathEscape(cfg.GitHubAppSlug) + "/installations/new?state=" + url.QueryEscape(state), nil
}

func (c *Client) SetupURL() string {
	return c.Config().GitHubSetupURL
}

func (c *Client) Installation(ctx context.Context, id int64) (Installation, error) {
	var response struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
		RepositorySelection string            `json:"repository_selection"`
		Permissions         map[string]string `json:"permissions"`
		SuspendedAt         *string           `json:"suspended_at"`
	}
	if err := c.doApp(ctx, http.MethodGet, fmt.Sprintf("/app/installations/%d", id), nil, &response); err != nil {
		return Installation{}, err
	}
	return Installation{GitHubID: response.ID, AccountLogin: response.Account.Login, AccountType: response.Account.Type, RepositorySelection: response.RepositorySelection, Permissions: response.Permissions, SuspendedAt: response.SuspendedAt}, nil
}

func (c *Client) Repositories(ctx context.Context, installationID int64) ([]Repository, error) {
	var result []Repository
	for page := 1; page <= 10; page++ {
		var response struct {
			Repositories []Repository `json:"repositories"`
		}
		query := "?per_page=100&page=" + strconv.Itoa(page)
		if err := c.doInstallation(ctx, http.MethodGet, "/installation/repositories"+query, installationID, nil, &response); err != nil {
			return nil, err
		}
		result = append(result, response.Repositories...)
		if len(response.Repositories) < 100 {
			break
		}
	}
	return result, nil
}

func (c *Client) Branches(ctx context.Context, installationID int64, fullName string) ([]string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid GitHub repository name")
	}
	var response []struct {
		Name string `json:"name"`
	}
	endpoint := "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/branches?per_page=100"
	if err := c.doInstallation(ctx, http.MethodGet, endpoint, installationID, nil, &response); err != nil {
		return nil, err
	}
	branches := make([]string, 0, len(response))
	for _, branch := range response {
		if branch.Name != "" {
			branches = append(branches, branch.Name)
		}
	}
	return branches, nil
}

func (c *Client) installationToken(ctx context.Context, installationID int64) (string, error) {
	c.mu.Lock()
	cached, ok := c.tokens[installationID]
	if ok && time.Now().Before(cached.expiresAt.Add(-2*time.Minute)) {
		c.mu.Unlock()
		return cached.token, nil
	}
	c.mu.Unlock()

	var response struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := c.doApp(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installationID), nil, &response); err != nil {
		return "", err
	}
	if response.Token == "" {
		return "", fmt.Errorf("GitHub returned an empty installation token")
	}
	c.mu.Lock()
	c.tokens[installationID] = cachedToken{token: response.Token, expiresAt: response.ExpiresAt}
	c.mu.Unlock()
	return response.Token, nil
}

func (c *Client) doApp(ctx context.Context, method, endpoint string, body io.Reader, target any) error {
	token, err := c.appJWT()
	if err != nil {
		return err
	}
	return c.do(ctx, method, endpoint, body, "Bearer "+token, target)
}

func (c *Client) doInstallation(ctx context.Context, method, endpoint string, installationID int64, body io.Reader, target any) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	return c.do(ctx, method, endpoint, body, "Bearer "+token, target)
}

func (c *Client) do(ctx context.Context, method, endpoint string, body io.Reader, authorization string, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, apiBaseURL+endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("Authorization", authorization)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("GitHub returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func (c *Client) appJWT() (string, error) {
	cfg := c.Config()
	if !c.Configured() {
		return "", fmt.Errorf("GitHub App is not configured: %s", c.ConfigurationError())
	}
	keyBytes, err := os.ReadFile(cfg.GitHubPrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("read GitHub App private key: %w", err)
	}
	key, err := parsePrivateKey(keyBytes)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(map[string]any{"iat": now - 60, "exp": now + 540, "iss": cfg.GitHubAppID})
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	unsigned := header + "." + payload
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign GitHub App JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func ValidatePrivateKey(data []byte) error {
	_, err := parsePrivateKey(data)
	return err
}

func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("GitHub App private key is not PEM encoded")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub App private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("GitHub App private key must be RSA")
	}
	return key, nil
}

func readableFile(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}
