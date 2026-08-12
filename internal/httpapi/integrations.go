package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/githubapp"
)

const githubAppConfigSettingKey = "github_app_config"

type githubAppConfig struct {
	AppID          string `json:"app_id"`
	AppSlug        string `json:"app_slug"`
	SetupURL       string `json:"setup_url"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
}

func (s *Server) loadGitHubAppSettings() {
	var encoded string
	if err := s.db.QueryRow(`SELECT value_json FROM settings WHERE key = ?`, githubAppConfigSettingKey).Scan(&encoded); err != nil {
		return
	}
	var saved githubAppConfig
	if json.Unmarshal([]byte(encoded), &saved) != nil {
		return
	}
	cfg := s.github.Config()
	if saved.AppID != "" {
		cfg.GitHubAppID = saved.AppID
	}
	if saved.AppSlug != "" {
		cfg.GitHubAppSlug = saved.AppSlug
	}
	if saved.SetupURL != "" {
		cfg.GitHubSetupURL = saved.SetupURL
	}
	if saved.PrivateKeyPath != "" {
		cfg.GitHubPrivateKeyPath = saved.PrivateKeyPath
	}
	s.github.UpdateConfig(cfg)
}

func (s *Server) handleGitHubConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if s.githubManaged.Configured() {
		s.handleManagedGitHubConnection(w, r)
		return
	}
	installations, err := s.githubStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read GitHub App installations.")
		return
	}
	cfg := s.github.Config()
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"configured":             s.github.Configured(),
		"mode":                   "self_hosted",
		"configuration_error":    s.github.ConfigurationError(),
		"connected":              s.github.Configured() && len(installations) > 0,
		"app_id":                 cfg.GitHubAppID,
		"app_slug":               cfg.GitHubAppSlug,
		"setup_url":              s.github.SetupURL(),
		"private_key_configured": cfg.GitHubPrivateKeyPath != "" && readablePath(cfg.GitHubPrivateKeyPath),
		"installations":          installations,
	}})
}

func (s *Server) handleManagedGitHubConnection(w http.ResponseWriter, r *http.Request) {
	status, err := s.githubManaged.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"configured":             true,
			"mode":                   "managed",
			"configuration_error":    err.Error(),
			"connected":              false,
			"private_key_configured": true,
			"installations":          []any{},
		}})
		return
	}
	installations := make([]map[string]any, 0, len(status.Installations))
	for _, installation := range status.Installations {
		installations = append(installations, map[string]any{
			"github_installation_id": installation.GitHubID,
			"account_login":          installation.AccountLogin,
			"account_type":           installation.AccountType,
			"repository_selection":   installation.RepositorySelection,
			"permissions":            installation.Permissions,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"configured":             true,
		"mode":                   "managed",
		"configuration_error":    status.Message,
		"connected":              status.State == "connected" && len(installations) > 0,
		"private_key_configured": true,
		"installations":          installations,
		"account_login":          status.AccountLogin,
	}})
}

func (s *Server) handleGitHubConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only PUT is supported.")
		return
	}
	var payload struct {
		AppID    string `json:"app_id"`
		AppSlug  string `json:"app_slug"`
		SetupURL string `json:"setup_url"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GITHUB_CONFIG", err.Error())
		return
	}
	payload.AppID = strings.TrimSpace(payload.AppID)
	payload.AppSlug = strings.TrimSpace(payload.AppSlug)
	payload.SetupURL = strings.TrimSpace(payload.SetupURL)
	if payload.AppID == "" || payload.AppSlug == "" {
		writeError(w, http.StatusBadRequest, "INVALID_GITHUB_CONFIG", "App ID and App slug are required.")
		return
	}
	parsed, err := url.Parse(payload.SetupURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		writeError(w, http.StatusBadRequest, "INVALID_GITHUB_CONFIG", "Setup URL must be a public HTTPS URL.")
		return
	}
	cfg := s.github.Config()
	cfg.GitHubAppID = payload.AppID
	cfg.GitHubAppSlug = payload.AppSlug
	cfg.GitHubSetupURL = payload.SetupURL
	if err := s.saveGitHubAppConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save GitHub App settings.")
		return
	}
	s.github.UpdateConfig(cfg)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"app_id": cfg.GitHubAppID, "app_slug": cfg.GitHubAppSlug, "setup_url": cfg.GitHubSetupURL, "private_key_configured": readablePath(cfg.GitHubPrivateKeyPath)}})
}

func (s *Server) handleGitHubPrivateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PRIVATE_KEY", "The private key upload is too large or invalid.")
		return
	}
	file, _, err := r.FormFile("private_key")
	if err != nil {
		file, _, err = r.FormFile("file")
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PRIVATE_KEY", "Upload a GitHub App PEM private key.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PRIVATE_KEY", "The private key could not be read.")
		return
	}
	if err := githubapp.ValidatePrivateKey(data); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PRIVATE_KEY", "The uploaded file is not a valid RSA GitHub App private key.")
		return
	}
	secretDir := filepath.Join(s.config.DataDir, "secrets")
	if err := os.MkdirAll(secretDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to prepare the private secrets directory.")
		return
	}
	keyPath := filepath.Join(secretDir, "github-app.pem")
	temporary, err := os.CreateTemp(secretDir, ".github-app-*.pem")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to store the private key.")
		return
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to protect the private key.")
		return
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to store the private key.")
		return
	}
	if err := temporary.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to store the private key.")
		return
	}
	if err := os.Rename(temporaryName, keyPath); err != nil {
		writeError(w, http.StatusInternalServerError, "SECRET_UNAVAILABLE", "Unable to activate the private key.")
		return
	}
	cfg := s.github.Config()
	cfg.GitHubPrivateKeyPath = keyPath
	if err := s.saveGitHubAppConfig(r.Context(), cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save the private key setting.")
		return
	}
	s.github.UpdateConfig(cfg)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"private_key_configured": true}})
}

func (s *Server) saveGitHubAppConfig(ctx context.Context, cfg config.Config) error {
	encoded, err := json.Marshal(githubAppConfig{AppID: cfg.GitHubAppID, AppSlug: cfg.GitHubAppSlug, SetupURL: cfg.GitHubSetupURL, PrivateKeyPath: cfg.GitHubPrivateKeyPath})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key, value_json, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, githubAppConfigSettingKey, encoded)
	return err
}

func readablePath(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

func (s *Server) handleGitHubInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	if s.githubManaged.Configured() {
		returnURL := s.publicReturnURL(r)
		session, err := s.githubManaged.StartSession(r.Context(), returnURL)
		if err != nil {
			writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"mode":       "managed",
			"session_id": session.ID,
			"url":        session.AuthorizeURL,
			"expires_in": session.ExpiresIn,
			"return_url": returnURL,
		}})
		return
	}
	if !s.github.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_APP_NOT_CONFIGURED", s.github.ConfigurationError())
		return
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		writeError(w, http.StatusInternalServerError, "STATE_UNAVAILABLE", "Unable to start GitHub App installation.")
		return
	}
	state := hex.EncodeToString(raw[:])
	if err := s.githubStore.CleanupSetupStates(r.Context()); err != nil {
		s.logger.Warn("cleanup GitHub setup states failed", "error", err)
	}
	if err := s.githubStore.CreateSetupState(r.Context(), state, time.Now().Add(10*time.Minute)); err != nil {
		writeError(w, http.StatusInternalServerError, "STATE_UNAVAILABLE", "Unable to start GitHub App installation.")
		return
	}
	installURL, err := s.github.InstallationURL(state)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_APP_NOT_CONFIGURED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"url": installURL, "expires_in": 600}})
}

func (s *Server) publicReturnURL(r *http.Request) string {
	if configured := strings.TrimRight(strings.TrimSpace(s.config.PublicURL), "/"); configured != "" {
		if parsed, err := url.Parse(configured); err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" {
			return configured + "/releasestation/?github=connected"
		}
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if proto != "https" {
		proto = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = "127.0.0.1:24871"
	}
	return proto + "://" + host + "/releasestation/?github=connected"
}

func (s *Server) handleGitHubSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	installationID, err := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	if state == "" || err != nil || installationID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_GITHUB_SETUP", "GitHub did not return a valid installation response.")
		return
	}
	valid, err := s.githubStore.ConsumeSetupState(r.Context(), state)
	if err != nil || !valid {
		writeError(w, http.StatusForbidden, "INVALID_GITHUB_STATE", "The GitHub installation link is expired or has already been used.")
		return
	}
	details, err := s.github.Installation(r.Context(), installationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_UNAVAILABLE", "The GitHub App installation could not be verified.")
		return
	}
	if _, err := s.githubStore.Save(r.Context(), details); err != nil {
		writeError(w, http.StatusInternalServerError, "INSTALLATION_UNAVAILABLE", "The GitHub App installation could not be saved.")
		return
	}
	http.Redirect(w, r, "/releasestation/?github=connected", http.StatusSeeOther)
}

func (s *Server) handleGitHubRepositories(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/branches") {
		s.handleGitHubBranches(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if s.githubManaged.Configured() {
		repositories, err := s.githubManaged.Repositories(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
			return
		}
		result := make([]map[string]any, 0, len(repositories))
		for _, repository := range repositories {
			result = append(result, map[string]any{
				"installation_id": repository.InstallationID,
				"account_login":   repository.AccountLogin,
				"id":              repository.ID,
				"name":            repository.Name,
				"full_name":       repository.FullName,
				"private":         repository.Private,
				"default_branch":  repository.DefaultBranch,
				"clone_url":       repository.CloneURL,
				"ssh_url":         repository.SSHURL,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": result})
		return
	}
	if !s.github.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_APP_NOT_CONFIGURED", s.github.ConfigurationError())
		return
	}
	requestedID := int64(0)
	if value := r.URL.Query().Get("installation_id"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_INSTALLATION", "installation_id must be a positive integer.")
			return
		}
		requestedID = parsed
	}
	installations, err := s.githubStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INSTALLATION_UNAVAILABLE", "Unable to read GitHub App installations.")
		return
	}
	var result []map[string]any
	for _, installation := range installations {
		if requestedID != 0 && installation.GitHubInstallationID != requestedID {
			continue
		}
		repositories, err := s.github.Repositories(r.Context(), installation.GitHubInstallationID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "GITHUB_UNAVAILABLE", "The repositories granted to the GitHub App could not be read.")
			return
		}
		for _, repository := range repositories {
			result = append(result, map[string]any{
				"installation_id": installation.GitHubInstallationID,
				"account_login":   installation.AccountLogin,
				"id":              repository.ID,
				"name":            repository.Name,
				"full_name":       repository.FullName,
				"private":         repository.Private,
				"default_branch":  repository.DefaultBranch,
				"clone_url":       repository.CloneURL,
				"ssh_url":         repository.SSHURL,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (s *Server) handleGitHubBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/releasestation/api/v1/integrations/github/repositories/")
	path = strings.TrimPrefix(path, "/api/v1/integrations/github/repositories/")
	path = strings.TrimSuffix(path, "/branches")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REPOSITORY", "Repository must use the owner/name format.")
		return
	}
	installationID, err := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	if err != nil || installationID <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_INSTALLATION", "installation_id must be a positive integer.")
		return
	}
	var branches []string
	if s.githubManaged.Configured() {
		branches, err = s.githubManaged.Branches(r.Context(), installationID, parts[0]+"/"+parts[1])
	} else {
		branches, err = s.github.Branches(r.Context(), installationID, parts[0]+"/"+parts[1])
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_UNAVAILABLE", "The repository branches could not be read.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": branches})
}
