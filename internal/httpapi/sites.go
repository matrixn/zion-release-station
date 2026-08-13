package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/matrixn/zion-release-station/internal/detection"
	"github.com/matrixn/zion-release-station/internal/pathsecurity"
	"github.com/matrixn/zion-release-station/internal/permissions"
	"github.com/matrixn/zion-release-station/internal/sites"
	"github.com/matrixn/zion-release-station/internal/webstation"
)

type sitePayload struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	Hostname    string `json:"hostname"`
	ProjectRoot string `json:"project_root"`
	WebRoot     string `json:"web_root"`
	Framework   string `json:"framework"`
	Strategy    string `json:"strategy"`
	Status      string `json:"status"`
	Runtime     any    `json:"runtime"`
	Repository  *struct {
		Provider             string `json:"provider"`
		CloneURL             string `json:"clone_url"`
		Branch               string `json:"branch"`
		GitHubInstallationID *int64 `json:"github_installation_id"`
		GitHubRepositoryID   *int64 `json:"github_repository_id"`
		GitHubFullName       string `json:"github_full_name"`
		GitHubDefaultBranch  string `json:"github_default_branch"`
	} `json:"repository"`
}

func (s *Server) handleSites(w http.ResponseWriter, r *http.Request) {
	relative := siteRoutePath(r.URL.Path)
	if relative == "" || relative == "/" {
		switch r.Method {
		case http.MethodGet:
			items, err := s.sites.List(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"data": items})
		case http.MethodPost:
			payload, err := decodeSitePayload(w, r)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
				return
			}
			input, runtime, err := s.prepareSiteInput(r.Context(), payload)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
				return
			}
			input.Runtime = runtime
			if _, err := s.sites.FindByProjectRoot(r.Context(), input.ProjectRoot); err == nil {
				writeError(w, http.StatusConflict, "SITE_EXISTS", "A site with this project root is already managed.")
				return
			} else if !errors.Is(err, sites.ErrNotFound) {
				writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
				return
			}
			created, err := s.sites.Create(r.Context(), input)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"data": created})
		default:
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and POST are supported.")
		}
		return
	}

	id := strings.Trim(relative, "/")
	if strings.HasSuffix(id, "/deploy") {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
			return
		}
		s.handleSiteDeploy(w, r, strings.TrimSuffix(id, "/deploy"))
		return
	}
	if strings.HasSuffix(id, "/deployments") || strings.Contains(id, "/deployments/") {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
			return
		}
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(id, "/"), "/"), "/")
		if len(parts) == 2 && parts[1] == "deployments" {
			s.handleSiteDeployments(w, r, parts[0])
			return
		}
		if len(parts) == 3 && parts[1] == "deployments" && parts[2] != "" {
			s.handleDeploymentDetails(w, r, parts[0], parts[2])
			return
		}
	}
	if strings.HasSuffix(id, "/commits") {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
			return
		}
		s.handleSiteCommits(w, r, strings.TrimSuffix(id, "/commits"))
		return
	}
	if strings.Contains(id, "/") || id == "" {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.sites.Get(r.Context(), id)
		if errors.Is(err, sites.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": item})
	case http.MethodPut, http.MethodPatch:
		payload, err := decodeSitePayload(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
			return
		}
		input, runtime, err := s.prepareSiteInput(r.Context(), payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
			return
		}
		input.Runtime = runtime
		updated, err := s.sites.Update(r.Context(), id, input)
		if errors.Is(err, sites.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SITE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": updated})
	case http.MethodDelete:
		if err := s.sites.Archive(r.Context(), id); errors.Is(err, sites.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Unsupported site method.")
	}
}

func (s *Server) handleSiteDeploy(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	site, err := s.sites.Get(r.Context(), id)
	if errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
		return
	}
	var payload struct {
		Ref string `json:"ref"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&payload)
	}
	result, err := s.deployer.DeployGitHubRef(r.Context(), site, strings.TrimSpace(payload.Ref))
	if err != nil {
		writeError(w, http.StatusBadRequest, "DEPLOYMENT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (s *Server) handleSiteDeployments(w http.ResponseWriter, r *http.Request, siteID string) {
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
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	result, err := s.deployer.ListDeployments(r.Context(), siteID, strings.TrimSpace(r.URL.Query().Get("q")), page, perPage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DEPLOYMENTS_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (s *Server) handleDeploymentDetails(w http.ResponseWriter, r *http.Request, siteID, deploymentID string) {
	item, err := s.deployer.GetDeployment(r.Context(), siteID, deploymentID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Deployment not found.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DEPLOYMENTS_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (s *Server) handleSiteCommits(w http.ResponseWriter, r *http.Request, siteID string) {
	site, err := s.sites.Get(r.Context(), siteID)
	if errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	}
	if err != nil || site.Repository == nil || site.Repository.GitHubInstallationID == nil || site.Repository.GitHubFullName == "" {
		writeError(w, http.StatusBadRequest, "REPOSITORY_UNAVAILABLE", "The site does not have a connected GitHub repository.")
		return
	}
	if !s.githubManaged.Configured() {
		writeError(w, http.StatusServiceUnavailable, "GITHUB_CONNECTOR_UNAVAILABLE", s.githubManaged.ConfigurationError())
		return
	}
	branch := site.Repository.Branch
	if branch == "" {
		branch = site.Repository.GitHubDefaultBranch
	}
	commits, err := s.githubManaged.Commits(r.Context(), *site.Repository.GitHubInstallationID, site.Repository.GitHubFullName, branch, 50)
	if err != nil {
		writeError(w, http.StatusBadGateway, "GITHUB_CONNECTOR_UNAVAILABLE", err.Error())
		return
	}
	statuses, err := s.deployer.CommitStatuses(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DEPLOYMENTS_UNAVAILABLE", err.Error())
		return
	}
	items := make([]map[string]any, 0, len(commits))
	for _, commit := range commits {
		deployment, ok := statuses[commit.SHA]
		status := "not_deployed"
		deploymentID := ""
		if ok {
			status = deployment.Status
			deploymentID = deployment.DeploymentID
		}
		items = append(items, map[string]any{"sha": commit.SHA, "message": commit.Message, "branch": commit.Branch, "author": commit.Author, "url": commit.URL, "created_at": commit.CreatedAt, "deployed": status == "deployed", "deployment_id": deploymentID, "status": status})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "branch": branch})
}

func (s *Server) handleWebStationStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	available, err := s.webStation.Available(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBSTATION_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"available": available,
		"adapter":   "filesystem-read-only",
		"mutating":  false,
	}})
}

func (s *Server) handleWebStationDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	items, err := s.discoverSites(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBSTATION_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (s *Server) handleWebStationImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported.")
		return
	}
	var payload struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSON(w, r, &payload); err != nil || len(payload.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_IMPORT", "Select at least one discovered site.")
		return
	}
	items, err := s.discoverSites(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "WEBSTATION_UNAVAILABLE", err.Error())
		return
	}
	selected := make(map[string]struct{}, len(payload.Paths))
	for _, path := range payload.Paths {
		canonical, err := pathsecurity.CanonicalDirectory(path)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_IMPORT", "One of the selected paths is no longer available.")
			return
		}
		selected[canonical] = struct{}{}
	}

	var imported []sites.Site
	var skipped []string
	for _, candidate := range items {
		if _, ok := selected[candidate.ProjectRoot]; !ok {
			continue
		}
		if _, err := s.sites.FindByProjectRoot(r.Context(), candidate.ProjectRoot); err == nil {
			skipped = append(skipped, candidate.ProjectRoot)
			continue
		} else if !errors.Is(err, sites.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
			return
		}
		input := discoveredInput(candidate)
		created, err := s.sites.Create(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_IMPORT", err.Error())
			return
		}
		imported = append(imported, created)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"imported": imported,
		"skipped":  skipped,
	}})
}

func (s *Server) discoverSites(ctx context.Context) ([]webstation.DiscoveredSite, error) {
	items, err := s.webStation.Discover(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if _, err := s.sites.FindByProjectRoot(ctx, items[i].ProjectRoot); err == nil {
			items[i].AlreadyManaged = true
		} else if !errors.Is(err, sites.ErrNotFound) {
			return nil, err
		}
	}
	return items, nil
}

func (s *Server) prepareSiteInput(ctx context.Context, payload sitePayload) (sites.Input, map[string]any, error) {
	hostname := strings.TrimSpace(payload.Hostname)
	if strings.TrimSpace(payload.URL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(payload.URL))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
			return sites.Input{}, nil, fmt.Errorf("site URL must be a valid http or https URL")
		}
		if hostname == "" {
			hostname = parsed.Hostname()
		}
	}
	projectRoot, err := pathsecurity.CanonicalDirectory(payload.ProjectRoot)
	if err != nil {
		return sites.Input{}, nil, fmt.Errorf("project root: %w", err)
	}
	detectionResult, err := (detection.Registry{}).Detect(projectRoot)
	if err != nil {
		return sites.Input{}, nil, err
	}
	framework := payload.Framework
	if framework == "" || framework == "auto" {
		framework = detectionResult.Framework
	}
	strategy := payload.Strategy
	if strategy == "" {
		strategy = "in_place"
	}
	status := payload.Status
	if status == "" {
		status = "active"
	}
	webRoot := payload.WebRoot
	if webRoot == "" {
		if strategy == "atomic" {
			webRoot = filepath.Join(projectRoot, "current")
		} else {
			webRoot = projectRoot
			if detectionResult.DocumentRoot != "" {
				webRoot = filepath.Join(projectRoot, filepath.FromSlash(detectionResult.DocumentRoot))
			}
		}
	}
	webRoot = filepath.Clean(webRoot)
	missingAtomicCurrent := strategy == "atomic" && webRoot == filepath.Join(projectRoot, "current")
	if missingAtomicCurrent {
		if _, statErr := os.Stat(webRoot); statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return sites.Input{}, nil, fmt.Errorf("web root: %w", statErr)
		}
	} else {
		webRoot, err = pathsecurity.CanonicalDirectory(webRoot)
		if err != nil {
			return sites.Input{}, nil, fmt.Errorf("web root: %w", err)
		}
	}
	if !pathsecurity.IsWithin(projectRoot, webRoot) {
		return sites.Input{}, nil, fmt.Errorf("web root must remain inside project root")
	}
	permissionPath := webRoot
	if missingAtomicCurrent {
		permissionPath = projectRoot
	}
	permission, err := permissions.Check(permissionPath)
	if err != nil {
		return sites.Input{}, nil, err
	}
	if !permission.Readable {
		status = "permission_required"
	}
	runtime := map[string]any{"permissions": permission}
	runtime["detection"] = detectionResult
	if payload.URL != "" {
		runtime["url"] = payload.URL
	}
	if payload.Runtime != nil {
		runtime["metadata"] = payload.Runtime
	}
	return sites.Input{
		Name:        payload.Name,
		Slug:        normalizeSlug(payload.Slug, payload.Name, hostname),
		Hostname:    hostname,
		ProjectRoot: projectRoot,
		WebRoot:     webRoot,
		Framework:   framework,
		Strategy:    strategy,
		Status:      status,
		Repository:  repositoryInput(payload.Repository),
	}, runtime, nil
}

func repositoryInput(payload *struct {
	Provider             string `json:"provider"`
	CloneURL             string `json:"clone_url"`
	Branch               string `json:"branch"`
	GitHubInstallationID *int64 `json:"github_installation_id"`
	GitHubRepositoryID   *int64 `json:"github_repository_id"`
	GitHubFullName       string `json:"github_full_name"`
	GitHubDefaultBranch  string `json:"github_default_branch"`
}) *sites.RepositoryInput {
	if payload == nil {
		return nil
	}
	return &sites.RepositoryInput{Provider: payload.Provider, CloneURL: payload.CloneURL, Branch: payload.Branch, GitHubInstallationID: payload.GitHubInstallationID, GitHubRepositoryID: payload.GitHubRepositoryID, GitHubFullName: payload.GitHubFullName, GitHubDefaultBranch: payload.GitHubDefaultBranch}
}

func discoveredInput(candidate webstation.DiscoveredSite) sites.Input {
	status := "active"
	if !candidate.Permissions.Readable {
		status = "permission_required"
	}
	return sites.Input{
		Name:        candidate.Name,
		Slug:        normalizeSlug("", candidate.Name, candidate.Hostname),
		Hostname:    candidate.Hostname,
		ProjectRoot: candidate.ProjectRoot,
		WebRoot:     candidate.WebRoot,
		Framework:   candidate.Framework,
		Strategy:    "in_place",
		Status:      status,
		Runtime: map[string]any{
			"source":      candidate.Source,
			"detection":   candidate.Detection,
			"permissions": candidate.Permissions,
		},
	}
}

func decodeSitePayload(w http.ResponseWriter, r *http.Request) (sitePayload, error) {
	var payload sitePayload
	if err := decodeJSON(w, r, &payload); err != nil {
		return sitePayload{}, err
	}
	return payload, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}

func siteRoutePath(path string) string {
	for _, prefix := range []string{"/releasestation/api/v1/sites", "/api/v1/sites"} {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	return ""
}

func normalizeSlug(value ...string) string {
	for _, candidate := range value {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate == "" {
			continue
		}
		var builder strings.Builder
		lastHyphen := false
		for _, character := range candidate {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
				builder.WriteRune(character)
				lastHyphen = false
			} else if !lastHyphen && builder.Len() > 0 {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
		return strings.Trim(builder.String(), "-")
	}
	return "site"
}
