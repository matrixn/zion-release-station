package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/detection"
	"github.com/matrixn/zion-release-station/internal/pathsecurity"
	"github.com/matrixn/zion-release-station/internal/permissions"
	"github.com/matrixn/zion-release-station/internal/sites"
	"github.com/matrixn/zion-release-station/internal/webstation"
)

type sitePayload struct {
	Name                string   `json:"name"`
	Slug                string   `json:"slug"`
	URL                 string   `json:"url"`
	Hostname            string   `json:"hostname"`
	ProjectRoot         string   `json:"project_root"`
	WebRoot             string   `json:"web_root"`
	Framework           string   `json:"framework"`
	CustomFramework     string   `json:"custom_framework"`
	Strategy            string   `json:"strategy"`
	Status              string   `json:"status"`
	Tags                []string `json:"tags"`
	Color               string   `json:"color"`
	PushToDeploy        *bool    `json:"push_to_deploy"`
	DeployScript        string   `json:"deploy_script"`
	DeploymentRetention *int     `json:"deployment_retention"`
	Runtime             any      `json:"runtime"`
	Repository          *struct {
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
			input.CustomFramework = payload.CustomFramework
			input.Tags = payload.Tags
			input.Color = payload.Color
			if payload.PushToDeploy != nil {
				input.PushToDeploy = *payload.PushToDeploy
			}
			input.DeployScript = payload.DeployScript
			if payload.DeploymentRetention != nil {
				input.DeploymentRetention = *payload.DeploymentRetention
			}
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
	if strings.HasSuffix(id, "/repository") {
		if r.Method == http.MethodGet || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			s.handleSiteRepository(w, r, strings.TrimSuffix(id, "/repository"))
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET, PUT and DELETE are supported.")
		return
	}
	if strings.HasSuffix(id, "/settings") {
		if r.Method == http.MethodGet || r.Method == http.MethodPut {
			s.handleSiteSettings(w, r, strings.TrimSuffix(id, "/settings"))
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and PUT are supported.")
		return
	}
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
		if len(parts) == 4 && parts[1] == "deployments" && parts[3] == "logs" {
			s.handleDeploymentLogs(w, r, parts[0], parts[2])
			return
		}
		if len(parts) == 5 && parts[1] == "deployments" && parts[3] == "logs" && parts[4] == "stream" {
			s.handleDeploymentLogStream(w, r, parts[0], parts[2])
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
		input.CustomFramework = payload.CustomFramework
		input.Tags = payload.Tags
		input.Color = payload.Color
		if payload.PushToDeploy != nil {
			input.PushToDeploy = *payload.PushToDeploy
		}
		input.DeployScript = payload.DeployScript
		if payload.DeploymentRetention != nil {
			input.DeploymentRetention = *payload.DeploymentRetention
		}
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
	result, err := s.deployQueue.Enqueue(r.Context(), site, strings.TrimSpace(payload.Ref), "manual", "manual")
	if err != nil {
		writeError(w, http.StatusBadRequest, "DEPLOYMENT_QUEUE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"data": result})
}

func (s *Server) handleSiteRepository(w http.ResponseWriter, r *http.Request, siteID string) {
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
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"data": site.Repository})
	case http.MethodDelete:
		if _, err := s.db.ExecContext(r.Context(), `DELETE FROM repositories WHERE site_id = ?`, siteID); err != nil {
			writeError(w, http.StatusInternalServerError, "REPOSITORY_UNAVAILABLE", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"repository": nil}})
	case http.MethodPut:
		var payload struct {
			Strategy   string `json:"strategy"`
			Repository *struct {
				Provider             string `json:"provider"`
				CloneURL             string `json:"clone_url"`
				Branch               string `json:"branch"`
				GitHubInstallationID *int64 `json:"github_installation_id"`
				GitHubRepositoryID   *int64 `json:"github_repository_id"`
				GitHubFullName       string `json:"github_full_name"`
				GitHubDefaultBranch  string `json:"github_default_branch"`
			} `json:"repository"`
		}
		if err := decodeJSON(w, r, &payload); err != nil || payload.Repository == nil {
			writeError(w, http.StatusBadRequest, "INVALID_REPOSITORY", "Repository details are required.")
			return
		}
		strategy := strings.TrimSpace(payload.Strategy)
		if strategy == "" {
			strategy = site.Strategy
		}
		if strategy != "atomic" && strategy != "in_place" {
			writeError(w, http.StatusBadRequest, "INVALID_REPOSITORY", "Deployment strategy must be atomic or in_place.")
			return
		}
		webRoot := site.WebRoot
		if strategy == "atomic" {
			webRoot = site.ProjectRoot
		} else if strings.TrimSuffix(filepath.Clean(webRoot), string(filepath.Separator)) == filepath.Join(filepath.Clean(site.ProjectRoot), "current") {
			webRoot = site.ProjectRoot
		}
		updated, err := s.sites.Update(r.Context(), siteID, sites.Input{
			Name: site.Name, Slug: site.Slug, Hostname: site.Hostname, ProjectRoot: site.ProjectRoot, WebRoot: webRoot,
			Framework: site.Framework, CustomFramework: site.CustomFramework, Strategy: strategy, Status: site.Status, Runtime: site.Runtime,
			Tags: site.Tags, Color: site.Color, PushToDeploy: site.PushToDeploy, DeployScript: site.DeployScript, DeploymentRetention: site.DeploymentRetention,
			Repository: repositoryInput(payload.Repository),
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_REPOSITORY", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": updated})
	}
}

func (s *Server) handleSiteSettings(w http.ResponseWriter, r *http.Request, siteID string) {
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
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"framework": site.Framework, "custom_framework": site.CustomFramework, "tags": site.Tags, "color": site.Color,
			"push_to_deploy": site.PushToDeploy, "deploy_script": site.DeployScript, "deployment_retention": site.DeploymentRetention,
			"project_root": site.ProjectRoot, "web_root": site.WebRoot,
		}})
		return
	}
	var payload struct {
		Framework           string   `json:"framework"`
		CustomFramework     string   `json:"custom_framework"`
		Tags                []string `json:"tags"`
		Color               string   `json:"color"`
		PushToDeploy        *bool    `json:"push_to_deploy"`
		DeployScript        string   `json:"deploy_script"`
		DeploymentRetention *int     `json:"deployment_retention"`
	}
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SETTINGS", err.Error())
		return
	}
	framework := strings.TrimSpace(payload.Framework)
	if framework == "" {
		framework = site.Framework
	}
	customFramework := strings.TrimSpace(payload.CustomFramework)
	if len([]rune(customFramework)) > 100 {
		writeError(w, http.StatusBadRequest, "INVALID_SETTINGS", "Custom framework must be at most 100 characters.")
		return
	}
	retention := site.DeploymentRetention
	if payload.DeploymentRetention != nil {
		retention = *payload.DeploymentRetention
	}
	push := site.PushToDeploy
	if payload.PushToDeploy != nil {
		push = *payload.PushToDeploy
	}
	updated, err := s.sites.Update(r.Context(), siteID, sites.Input{
		Name: site.Name, Slug: site.Slug, Hostname: site.Hostname, ProjectRoot: site.ProjectRoot, WebRoot: site.WebRoot,
		Framework: framework, CustomFramework: customFramework, Strategy: site.Strategy, Status: site.Status, Runtime: site.Runtime,
		Tags: payload.Tags, Color: payload.Color, PushToDeploy: push, DeployScript: payload.DeployScript, DeploymentRetention: retention,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_SETTINGS", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": updated})
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

func (s *Server) handleDeploymentLogs(w http.ResponseWriter, r *http.Request, siteID, deploymentID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	s.handleDeploymentDetails(w, r, siteID, deploymentID)
}

func (s *Server) handleDeploymentLogStream(w http.ResponseWriter, r *http.Request, siteID, deploymentID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if _, err := s.sites.Get(r.Context(), siteID); errors.Is(err, sites.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Site not found.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "SITES_UNAVAILABLE", err.Error())
		return
	}
	if _, err := s.deployer.GetDeployment(r.Context(), siteID, deploymentID); errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Deployment not found.")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "DEPLOYMENTS_UNAVAILABLE", err.Error())
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "STREAM_UNAVAILABLE", "The server does not support event streaming.")
		return
	}
	stream, unsubscribe := s.deployer.Events().Subscribe(deploymentID)
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if item, err := s.deployer.GetDeployment(r.Context(), siteID, deploymentID); err == nil {
		if !writeSSE(w, "snapshot", item) {
			return
		}
		flusher.Flush()
		if deploymentTerminal(item.Status) {
			return
		}
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-stream:
			if !open {
				return
			}
			if !writeSSE(w, event.Type, event) {
				return
			}
			flusher.Flush()
			if deploymentTerminal(event.Status) {
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, eventName string, value any) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, payload); err != nil {
		return false
	}
	return true
}

func deploymentTerminal(status string) bool {
	return status == "deployed" || status == "failed" || status == "cancelled"
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
	latestDeployedIndex := -1
	for index, commit := range commits {
		if deployment, ok := statuses[commit.SHA]; ok && deployment.Status == "deployed" {
			latestDeployedIndex = index
			break
		}
	}
	for index, commit := range commits {
		deployment, ok := statuses[commit.SHA]
		status := "not_deployed"
		deploymentID := ""
		if ok {
			status = deployment.Status
			deploymentID = deployment.DeploymentID
		}
		included := latestDeployedIndex >= 0 && index > latestDeployedIndex && status != "deployed"
		if included {
			status = "included"
		}
		items = append(items, map[string]any{"sha": commit.SHA, "message": commit.Message, "branch": commit.Branch, "author": commit.Author, "url": commit.URL, "created_at": commit.CreatedAt, "deployed": status == "deployed", "included_in_deployed": included, "deployment_id": deploymentID, "status": status})
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
			webRoot = projectRoot
		} else {
			webRoot = projectRoot
			if detectionResult.DocumentRoot != "" {
				webRoot = filepath.Join(projectRoot, filepath.FromSlash(detectionResult.DocumentRoot))
			}
		}
	}
	webRoot = filepath.Clean(webRoot)
	webRoot, err = pathsecurity.CanonicalDirectory(webRoot)
	if err != nil {
		return sites.Input{}, nil, fmt.Errorf("web root: %w", err)
	}
	if !pathsecurity.IsWithin(projectRoot, webRoot) {
		return sites.Input{}, nil, fmt.Errorf("web root must remain inside project root")
	}
	permission, err := permissions.Check(webRoot)
	if err != nil {
		return sites.Input{}, nil, err
	}
	if !permission.Readable {
		status = "permission_required"
	}
	runtime := map[string]any{"permissions": permission}
	runtime["detection"] = detectionResult
	runtime["http_server"] = detection.DetectHTTPServer(projectRoot, webRoot)
	if publicIP := resolvePublicIP(hostname); publicIP != "" {
		runtime["public_ip"] = publicIP
	}
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
	runtime := map[string]any{
		"source":      candidate.Source,
		"detection":   candidate.Detection,
		"permissions": candidate.Permissions,
		"http_server": detection.DetectHTTPServer(candidate.ProjectRoot, candidate.WebRoot),
	}
	if publicIP := resolvePublicIP(candidate.Hostname); publicIP != "" {
		runtime["public_ip"] = publicIP
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
		Runtime:     runtime,
	}
}

func resolvePublicIP(hostname string) string {
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return ""
	}
	addresses, err := net.LookupIP(hostname)
	if err != nil {
		return ""
	}
	for _, address := range addresses {
		if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() {
			continue
		}
		return address.String()
	}
	return ""
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
