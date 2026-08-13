package httpapi

import (
	"errors"
	"net/http"
	"strings"

	gittransport "github.com/matrixn/zion-release-station/internal/git"
	"github.com/matrixn/zion-release-station/internal/sites"
)

func (s *Server) registerGitRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/git/test", s.handleGitTest)
	mux.HandleFunc(prefix+"/git/generate-deploy-key", s.handleGitGenerateDeployKey)
	mux.HandleFunc(prefix+"/git/deploy-key/", s.handleGitDeployKey)
	mux.HandleFunc(prefix+"/git/test-ssh", s.handleGitTestSSH)
	mux.HandleFunc(prefix+"/git/branches/", s.handleGitBranches)
}

type gitRequest struct {
	SiteID   string `json:"site_id"`
	CloneURL string `json:"clone_url"`
	Branch   string `json:"branch"`
}

func (s *Server) handleGitTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	payload, err := decodeGitRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GIT_REQUEST", err.Error())
		return
	}
	remoteURL, branch, err := s.gitRequestRepository(r, payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GIT_REQUEST", err.Error())
		return
	}
	remote, err := gittransport.ValidateRepository(remoteURL, branch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REPOSITORY", err.Error())
		return
	}
	if err := s.git.Test(r.Context(), remoteURL, branch); err != nil {
		writeError(w, http.StatusBadGateway, "GIT_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"valid": true, "remote": remote, "branch": branch,
	}})
}

func (s *Server) handleGitGenerateDeployKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	publicKey, err := s.git.KeyStore.Generate()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GIT_KEY_UNAVAILABLE", err.Error())
		return
	}
	fingerprint, err := s.git.KeyStore.Fingerprint()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GIT_KEY_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"algorithm": "ed25519", "public_key": publicKey, "fingerprint": fingerprint,
	}})
}

func (s *Server) handleGitDeployKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	siteID := strings.TrimPrefix(r.URL.Path, "/releasestation/api/v1/git/deploy-key/")
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		siteID = strings.TrimPrefix(r.URL.Path, "/api/v1/git/deploy-key/")
	}
	if siteID == "" || strings.Contains(siteID, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	if _, err := s.sites.Get(r.Context(), siteID); errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
		return
	}
	publicKey, err := s.git.KeyStore.PublicKey()
	if errors.Is(err, gittransport.ErrDeployKeyNotFound) {
		writeError(w, http.StatusNotFound, "DEPLOY_KEY_NOT_FOUND", "Generate a deploy key first.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GIT_KEY_UNAVAILABLE", err.Error())
		return
	}
	fingerprint, err := s.git.KeyStore.Fingerprint()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GIT_KEY_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"site_id": siteID, "algorithm": "ed25519", "public_key": publicKey, "fingerprint": fingerprint,
	}})
}

func (s *Server) handleGitTestSSH(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	payload, err := decodeGitRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GIT_REQUEST", err.Error())
		return
	}
	remoteURL, branch, err := s.gitRequestRepository(r, payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_GIT_REQUEST", err.Error())
		return
	}
	if err := s.git.TestSSH(r.Context(), remoteURL, branch); err != nil {
		writeError(w, http.StatusBadGateway, "GIT_SSH_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"valid": true, "branch": branch}})
}

func (s *Server) handleGitBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	siteID := strings.TrimPrefix(r.URL.Path, "/releasestation/api/v1/git/branches/")
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		siteID = strings.TrimPrefix(r.URL.Path, "/api/v1/git/branches/")
	}
	if siteID == "" || strings.Contains(siteID, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	site, err := s.sites.Get(r.Context(), siteID)
	if errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
		return
	}
	if site.Repository == nil {
		writeError(w, http.StatusBadRequest, "REPOSITORY_UNAVAILABLE", "The site has no repository configured.")
		return
	}
	branches, err := s.git.Branches(r.Context(), site.Repository.CloneURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GIT_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"branches": branches}})
}

func decodeGitRequest(w http.ResponseWriter, r *http.Request) (gitRequest, error) {
	var payload gitRequest
	if err := decodeJSON(w, r, &payload); err != nil {
		return gitRequest{}, err
	}
	return payload, nil
}

func (s *Server) gitRequestRepository(r *http.Request, payload gitRequest) (string, string, error) {
	if strings.TrimSpace(payload.SiteID) != "" {
		site, err := s.sites.Get(r.Context(), strings.TrimSpace(payload.SiteID))
		if err != nil {
			if errors.Is(err, sites.ErrNotFound) {
				return "", "", errors.New("site not found")
			}
			return "", "", err
		}
		if site.Repository == nil {
			return "", "", errors.New("site has no repository configured")
		}
		if strings.TrimSpace(payload.CloneURL) == "" {
			payload.CloneURL = site.Repository.CloneURL
		}
		if strings.TrimSpace(payload.Branch) == "" {
			payload.Branch = site.Repository.Branch
		}
	}
	if strings.TrimSpace(payload.CloneURL) == "" || strings.TrimSpace(payload.Branch) == "" {
		return "", "", errors.New("clone_url and branch are required")
	}
	return strings.TrimSpace(payload.CloneURL), strings.TrimSpace(payload.Branch), nil
}
