package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/deploy"
	"github.com/matrixn/zion-release-station/internal/detection"
	gittransport "github.com/matrixn/zion-release-station/internal/git"
	"github.com/matrixn/zion-release-station/internal/githubconnector"
	"github.com/matrixn/zion-release-station/internal/sites"
	"github.com/matrixn/zion-release-station/internal/systemchecks"
	"github.com/matrixn/zion-release-station/internal/webstation"
)

type Server struct {
	config        config.Config
	db            *sql.DB
	logger        *slog.Logger
	http          *http.Server
	sites         *sites.Store
	webStation    webstation.WebStationAdapter
	git           *gittransport.Client
	githubManaged *githubconnector.Client
	deployer      *deploy.Runner
	deployQueue   *deploy.Queue
}

const webAccessSettingKey = "web_access_enabled"
const systemOverviewChecksSettingKey = "system_overview_checks"
const workspaceRouteSettingKey = "workspace_route"

func NewServer(cfg config.Config, db *sql.DB, logger *slog.Logger) *Server {
	server := &Server{
		config:        cfg,
		db:            db,
		logger:        logger,
		sites:         sites.NewStore(db),
		webStation:    webstation.NewFilesystemAdapter(cfg.WebStationRoots, detection.Registry{}),
		git:           gittransport.NewClient(cfg.DataDir),
		githubManaged: githubconnector.NewClient(cfg),
	}
	events := deploy.NewEventHub()
	server.deployer = deploy.NewRunnerWithHub(db, server.githubManaged, events)
	server.deployQueue = deploy.NewQueue(db, server.deployer, server.sites.Get, 2)
	if _, err := db.Exec(`INSERT OR IGNORE INTO settings(key, value_json, updated_at) VALUES (?, 'true', datetime('now'))`, webAccessSettingKey); err != nil {
		logger.Error("initialize web access setting", "error", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/releasestation/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/releasestation/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/releasestation/api/v1/system/capabilities", server.handleCapabilities)
	mux.HandleFunc("/releasestation/api/v1/system/metrics", server.handleMetrics)
	mux.HandleFunc("/releasestation/api/v1/system/checks", server.handleSystemChecks)
	mux.HandleFunc("/releasestation/api/v1/settings/web-access", server.handleWebAccess)
	mux.HandleFunc("/releasestation/api/v1/settings/workspace", server.handleWorkspaceSettings)
	mux.HandleFunc("/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/api/v1/system/capabilities", server.handleCapabilities)
	mux.HandleFunc("/api/v1/system/metrics", server.handleMetrics)
	mux.HandleFunc("/api/v1/system/checks", server.handleSystemChecks)
	mux.HandleFunc("/api/v1/settings/web-access", server.handleWebAccess)
	mux.HandleFunc("/api/v1/settings/workspace", server.handleWorkspaceSettings)
	server.registerSiteRoutes(mux, "/releasestation/api/v1")
	server.registerSiteRoutes(mux, "/api/v1")
	server.registerIntegrationRoutes(mux, "/releasestation/api/v1")
	server.registerIntegrationRoutes(mux, "/api/v1")
	server.registerGitRoutes(mux, "/releasestation/api/v1")
	server.registerGitRoutes(mux, "/api/v1")
	mux.Handle("/releasestation/", server.staticHandler())
	server.http = &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           withSecurityHeaders(withRequestLogging(server.workspaceRouteHandler(mux), logger)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server
}

// workspaceRouteHandler keeps the API and SPA contract stable when the DSM
// resource forwards a custom validated route to the local service.
func (s *Server) workspaceRouteHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, err := s.workspaceRoute(r.Context())
		if err == nil && route != "/releasestation/" && strings.HasPrefix(r.URL.Path, route) {
			cloned := r.Clone(r.Context())
			cloned.URL.Path = "/releasestation/" + strings.TrimPrefix(r.URL.Path, route)
			next.ServeHTTP(w, cloned)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) registerIntegrationRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/integrations/github", s.handleGitHubConnection)
	mux.HandleFunc(prefix+"/integrations/github/install", s.handleGitHubInstall)
	mux.HandleFunc(prefix+"/integrations/github/setup", s.handleGitHubSetupRedirect)
	mux.HandleFunc(prefix+"/integrations/github/complete", s.handleGitHubPairingComplete)
	mux.HandleFunc(prefix+"/integrations/github/pairing-status", s.handleGitHubPairingStatus)
	mux.HandleFunc(prefix+"/integrations/github/repositories", s.handleGitHubRepositories)
	mux.HandleFunc(prefix+"/integrations/github/repositories/", s.handleGitHubRepositories)
}

func (s *Server) registerSiteRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/sites", s.handleSites)
	mux.HandleFunc(prefix+"/sites/", s.handleSites)
	mux.HandleFunc(prefix+"/webstation/status", s.handleWebStationStatus)
	mux.HandleFunc(prefix+"/webstation/discover", s.handleWebStationDiscover)
	mux.HandleFunc(prefix+"/webstation/import", s.handleWebStationImport)
}

func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Redirect(w, r, "/releasestation/", http.StatusTemporaryRedirect)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	if err := s.db.PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"data": map[string]any{
			"status":   "unhealthy",
			"database": "unavailable",
		}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"status":       "healthy",
		"version":      s.config.Version,
		"platform":     "synology",
		"architecture": "x86_64",
		"database":     "ready",
	}})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"product":      "Zion Release Station",
		"package":      "zion-releasestation",
		"version":      s.config.Version,
		"bind_address": s.config.BindAddress,
	}})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	metrics, err := s.dashboardMetrics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "METRICS_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": metrics})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"webstation": false,
		"git":        commandAvailable("git"),
		"database":   true,
		"deployment": "atomic-github",
	}})
}

func (s *Server) handleSystemChecks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ids, err := s.enabledSystemChecks(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read System Overview checks.")
			return
		}
		selected := make(map[string]bool, len(ids))
		for _, id := range ids {
			selected[id] = true
		}
		items := make([]map[string]any, 0)
		for _, definition := range systemchecks.Definitions() {
			items = append(items, map[string]any{
				"id": definition.ID, "label": definition.Label, "command": definition.Command,
				"description": definition.Description, "install_hint": definition.InstallHint,
				"enabled": selected[definition.ID],
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"checks": items, "enabled": ids}})
	case http.MethodPut:
		var payload struct {
			Enabled []string `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SETTINGS", "Enabled checks must be an array of check IDs.")
			return
		}
		ids := systemchecks.NormalizeIDs(payload.Enabled)
		encoded, err := json.Marshal(ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to encode System Overview checks.")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), `INSERT INTO settings(key, value_json, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, systemOverviewChecksSettingKey, string(encoded)); err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save System Overview checks.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"enabled": ids}})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and PUT are supported.")
	}
}

func (s *Server) enabledSystemChecks(ctx context.Context) ([]string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, systemOverviewChecksSettingKey).Scan(&value); err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, err
	}
	return systemchecks.NormalizeIDs(ids), nil
}

func (s *Server) handleWebAccess(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		enabled, err := s.webAccessEnabled(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read web access settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"enabled": enabled}})
	case http.MethodPut:
		var payload struct {
			Enabled *bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Enabled == nil {
			writeError(w, http.StatusBadRequest, "INVALID_SETTINGS", "The enabled value must be a boolean.")
			return
		}
		value := "false"
		if *payload.Enabled {
			value = "true"
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE settings SET value_json = ?, updated_at = datetime('now') WHERE key = ?`, value, webAccessSettingKey); err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save web access settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"enabled": *payload.Enabled}})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and PUT are supported.")
	}
}

func (s *Server) handleWorkspaceSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		route, err := s.workspaceRoute(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read workspace settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"route": route, "active_route": "/releasestation/", "default_route": "/releasestation/", "requires_reload": route != "/releasestation/",
		}})
	case http.MethodPut:
		var payload struct {
			Route string `json:"route"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ROUTE", "DSM Route must be a valid path.")
			return
		}
		route, reason := normalizeWorkspaceRoute(payload.Route)
		if reason != "" {
			writeError(w, http.StatusUnprocessableEntity, "DSM_ROUTE_CONFLICT", reason)
			return
		}
		encoded, _ := json.Marshal(route)
		if _, err := s.db.ExecContext(r.Context(), `INSERT INTO settings(key, value_json, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, workspaceRouteSettingKey, string(encoded)); err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save workspace settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"route": route, "active_route": "/releasestation/", "default_route": "/releasestation/", "requires_reload": route != "/releasestation/",
		}})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and PUT are supported.")
	}
}

func (s *Server) workspaceRoute(ctx context.Context) (string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, workspaceRouteSettingKey).Scan(&value); err != nil {
		return "", err
	}
	var route string
	if err := json.Unmarshal([]byte(value), &route); err != nil {
		return "", err
	}
	normalized, reason := normalizeWorkspaceRoute(route)
	if reason != "" {
		return "", fmt.Errorf("stored workspace route is invalid: %s", reason)
	}
	return normalized, nil
}

func normalizeWorkspaceRoute(value string) (string, string) {
	route := strings.TrimSpace(value)
	if route == "" {
		return "", "DSM Route is required."
	}
	if !strings.HasPrefix(route, "/") {
		return "", "DSM Route must start with '/'."
	}
	route = "/" + strings.Trim(route, "/") + "/"
	if len(route) > 64 || strings.Contains(route, "..") || strings.ContainsAny(route, "?&#%\\\\\"'") {
		return "", "DSM Route contains invalid characters or is too long."
	}
	if route == "//" || !regexp.MustCompile(`^/[A-Za-z0-9][A-Za-z0-9/_-]*/$`).MatchString(route) {
		return "", "DSM Route may contain only letters, numbers, '/', '-' and '_'."
	}
	for _, reserved := range []string{"/", "/api/", "/webapi/", "/webman/", "/dsm/", "/file/", "/packagecenter/"} {
		if strings.EqualFold(route, reserved) || (reserved != "/" && strings.HasPrefix(strings.ToLower(route), strings.TrimSuffix(reserved, "/")+"/")) {
			return "", "DSM Route conflicts with a reserved DSM path."
		}
	}
	return route, ""
}

func (s *Server) webAccessEnabled(ctx context.Context) (bool, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, webAccessSettingKey).Scan(&value); err != nil {
		return false, err
	}
	var enabled bool
	if err := json.Unmarshal([]byte(value), &enabled); err != nil {
		return false, err
	}
	return enabled, nil
}

func (s *Server) staticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enabled, err := s.webAccessEnabled(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read web access settings.")
			return
		}
		if !enabled {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/releasestation/")
		if path == "" {
			path = "index.html"
		}
		path = filepath.ToSlash(filepath.Clean("/" + path))[1:]
		if path == "." || strings.Contains(path, "..") {
			http.NotFound(w, r)
			return
		}
		filePath := filepath.Join(s.config.WebRoot, filepath.FromSlash(path))
		if _, err := os.Stat(filePath); err != nil {
			if os.IsNotExist(err) && !strings.Contains(filepath.Base(path), ".") {
				filePath = filepath.Join(s.config.WebRoot, "index.html")
			} else {
				http.NotFound(w, r)
				return
			}
		}
		http.ServeFile(w, r, filePath)
	})
}

func commandAvailable(name string) bool {
	_, err := os.Stat(filepath.Join("/usr/bin", name))
	return err == nil
}

func withRequestLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info("HTTP request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
