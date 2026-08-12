package githubconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
)

// Client talks to the Zion-managed connector. The connector owns the GitHub
// App private key and returns only authorization URLs, metadata, and short-lived
// credentials needed by the ReleaseStation instance.
type Client struct {
	baseURL    string
	token      string
	instanceID string
	http       *http.Client
}

type Session struct {
	ID           string `json:"id"`
	AuthorizeURL string `json:"authorize_url"`
	ExpiresIn    int    `json:"expires_in"`
}

type Installation struct {
	GitHubID            int64             `json:"github_installation_id"`
	AccountLogin        string            `json:"account_login"`
	AccountType         string            `json:"account_type"`
	RepositorySelection string            `json:"repository_selection"`
	Permissions         map[string]string `json:"permissions,omitempty"`
}

type Repository struct {
	InstallationID int64  `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	FullName       string `json:"full_name"`
	Private        bool   `json:"private"`
	DefaultBranch  string `json:"default_branch"`
	CloneURL       string `json:"clone_url"`
	SSHURL         string `json:"ssh_url"`
}

type Status struct {
	State         string         `json:"state"`
	AccountLogin  string         `json:"account_login,omitempty"`
	Installations []Installation `json:"installations,omitempty"`
	Message       string         `json:"message,omitempty"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(cfg.GitHubConnectorURL), "/"),
		token:      strings.TrimSpace(cfg.GitHubConnectorToken),
		instanceID: strings.TrimSpace(cfg.InstanceID),
		http:       &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.token != "" && c.instanceID != ""
}

func (c *Client) ConfigurationError() string {
	switch {
	case c.baseURL == "":
		return "RS_GITHUB_CONNECTOR_URL is not configured"
	case c.token == "":
		return "The Zion connector credential is not provisioned"
	case c.instanceID == "":
		return "RS_INSTANCE_ID is not configured"
	default:
		return ""
	}
}

func (c *Client) StartSession(ctx context.Context, returnURL string) (Session, error) {
	if !c.Configured() {
		return Session{}, fmt.Errorf("managed GitHub connector is not configured: %s", c.ConfigurationError())
	}
	var response Session
	err := c.request(ctx, http.MethodPost, c.path("github/sessions"), map[string]string{"return_url": strings.TrimSpace(returnURL)}, &response)
	if err != nil {
		return Session{}, fmt.Errorf("start managed GitHub session: %w", err)
	}
	if response.ID == "" || response.AuthorizeURL == "" {
		return Session{}, fmt.Errorf("managed GitHub connector returned an incomplete session")
	}
	return response, nil
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	if !c.Configured() {
		return Status{}, fmt.Errorf("managed GitHub connector is not configured: %s", c.ConfigurationError())
	}
	var response Status
	if err := c.request(ctx, http.MethodGet, c.path("github/status"), nil, &response); err != nil {
		return Status{}, fmt.Errorf("read managed GitHub status: %w", err)
	}
	return response, nil
}

func (c *Client) Repositories(ctx context.Context) ([]Repository, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("managed GitHub connector is not configured: %s", c.ConfigurationError())
	}
	var response struct {
		Repositories []Repository `json:"repositories"`
	}
	if err := c.request(ctx, http.MethodGet, c.path("github/repositories"), nil, &response); err != nil {
		return nil, fmt.Errorf("read managed GitHub repositories: %w", err)
	}
	return response.Repositories, nil
}

func (c *Client) Branches(ctx context.Context, installationID int64, fullName string) ([]string, error) {
	parts := strings.Split(strings.Trim(fullName, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid GitHub repository name")
	}
	var response struct {
		Branches []string `json:"branches"`
	}
	path := c.path("github/repositories/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) + "/branches")
	query := url.Values{}
	query.Set("installation_id", fmt.Sprintf("%d", installationID))
	if err := c.request(ctx, http.MethodGet, path+"?"+query.Encode(), nil, &response); err != nil {
		return nil, fmt.Errorf("read managed GitHub branches: %w", err)
	}
	return response.Branches, nil
}

func (c *Client) path(suffix string) string {
	return c.baseURL + "/v1/instances/" + url.PathEscape(c.instanceID) + "/" + strings.TrimLeft(suffix, "/")
}

func (c *Client) request(ctx context.Context, method, endpoint string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("connector returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode connector response: %w", err)
	}
	return nil
}
