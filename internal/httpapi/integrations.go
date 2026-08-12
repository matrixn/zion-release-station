package httpapi

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (s *Server) handleGitHubConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}

	if !s.githubManaged.PairingConfigured() {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"configured":             false,
			"mode":                   "managed",
			"configuration_error":    s.githubManaged.ConfigurationError(),
			"connected":              false,
			"private_key_configured": false,
			"installations":          []any{},
		}})
		return
	}
	if !s.githubManaged.Configured() {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"configured":             true,
			"mode":                   "managed",
			"configuration_error":    "GitHub connector pairing is required",
			"connected":              false,
			"private_key_configured": false,
			"installations":          []any{},
		}})
		return
	}

	s.handleManagedGitHubConnection(w, r)
}

func (s *Server) handleManagedGitHubConnection(w http.ResponseWriter, r *http.Request) {
	status, err := s.githubManaged.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"configured":             true,
			"mode":                   "managed",
			"configuration_error":    err.Error(),
			"connected":              false,
			"private_key_configured": false,
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
		"private_key_configured": false,
		"installations":          installations,
		"account_login":          status.AccountLogin,
	}})
}

func (s *Server) handleGitHubInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	if !s.githubManaged.PairingConfigured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_CONNECTOR_UNAVAILABLE", s.githubManaged.ConfigurationError())
		return
	}

	if !s.githubManaged.Configured() {
		pairing, err := s.githubManaged.StartPairingSession(r.Context(), "")
		if err != nil {
			writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"mode":       "pairing",
			"session_id": pairing.ID,
			"poll_token": pairing.PollToken,
			"url":        pairing.AuthorizeURL,
			"expires_in": pairing.ExpiresIn,
		}})
		return
	}
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
}

func (s *Server) handleGitHubPairingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	var payload struct {
		SessionID string `json:"session_id"`
		PollToken string `json:"poll_token"`
	}
	if err := decodeJSON(w, r, &payload); err != nil || strings.TrimSpace(payload.SessionID) == "" || strings.TrimSpace(payload.PollToken) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PAIRING", "The pairing session and token are required.")
		return
	}
	status, err := s.githubManaged.PairingStatus(r.Context(), payload.SessionID, payload.PollToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
		return
	}
	if status.State != "authorized" {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"state": status.State, "connected": false}})
		return
	}
	credential, err := s.githubManaged.CompletePairing(r.Context(), status.PairingCode)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
		return
	}
	if err := s.config.SaveConnectorCredential(credential); err != nil {
		writeError(w, http.StatusInternalServerError, "CONNECTOR_STATE_UNAVAILABLE", "The connector credential could not be stored securely.")
		return
	}
	s.githubManaged.SetCredential(credential)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"state": "connected", "connected": true}})
}

func (s *Server) handleGitHubPairingComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	var payload struct {
		PairingCode string `json:"pairing_code"`
	}
	if err := decodeJSON(w, r, &payload); err != nil || strings.TrimSpace(payload.PairingCode) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PAIRING", "The pairing code is required.")
		return
	}
	credential, err := s.githubManaged.CompletePairing(r.Context(), payload.PairingCode)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
		return
	}
	if err := s.config.SaveConnectorCredential(credential); err != nil {
		writeError(w, http.StatusInternalServerError, "CONNECTOR_STATE_UNAVAILABLE", "The connector credential could not be stored securely.")
		return
	}
	s.githubManaged.SetCredential(credential)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"connected": true}})
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

func (s *Server) handleGitHubRepositories(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/branches") {
		s.handleGitHubBranches(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if !s.githubManaged.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_CONNECTOR_UNAVAILABLE", s.githubManaged.ConfigurationError())
		return
	}

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
}

func (s *Server) handleGitHubBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if !s.githubManaged.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_CONNECTOR_UNAVAILABLE", s.githubManaged.ConfigurationError())
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
	branches, err := s.githubManaged.Branches(r.Context(), installationID, parts[0]+"/"+parts[1])
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", "The repository branches could not be read.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": branches})
}
