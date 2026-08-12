package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleGitHubConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	installations, err := s.githubStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read GitHub App installations.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"configured":          s.github.Configured(),
		"configuration_error": s.github.ConfigurationError(),
		"connected":           s.github.Configured() && len(installations) > 0,
		"app_slug":            s.config.GitHubAppSlug,
		"setup_url":           s.github.SetupURL(),
		"installations":       installations,
	}})
}

func (s *Server) handleGitHubInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
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
	branches, err := s.github.Branches(r.Context(), installationID, parts[0]+"/"+parts[1])
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_UNAVAILABLE", "The repository branches could not be read.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": branches})
}
