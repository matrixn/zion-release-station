package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
	"github.com/matrixn/zion-release-station/internal/detection"
	"github.com/matrixn/zion-release-station/internal/sites"
	"github.com/matrixn/zion-release-station/internal/webstation"
)

type Server struct {
	config     config.Config
	db         *sql.DB
	logger     *slog.Logger
	http       *http.Server
	sites      *sites.Store
	webStation webstation.WebStationAdapter
}

const webAccessSettingKey = "web_access_enabled"

func NewServer(cfg config.Config, db *sql.DB, logger *slog.Logger) *Server {
	server := &Server{
		config:     cfg,
		db:         db,
		logger:     logger,
		sites:      sites.NewStore(db),
		webStation: webstation.NewFilesystemAdapter(cfg.WebStationRoots, detection.Registry{}),
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO settings(key, value_json, updated_at) VALUES (?, 'true', datetime('now'))`, webAccessSettingKey); err != nil {
		logger.Error("initialize web access setting", "error", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/releasestation/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/releasestation/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/releasestation/api/v1/system/capabilities", server.handleCapabilities)
	mux.HandleFunc("/releasestation/api/v1/settings/web-access", server.handleWebAccess)
	mux.HandleFunc("/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/api/v1/system/capabilities", server.handleCapabilities)
	mux.HandleFunc("/api/v1/settings/web-access", server.handleWebAccess)
	server.registerSiteRoutes(mux, "/releasestation/api/v1")
	server.registerSiteRoutes(mux, "/api/v1")
	mux.Handle("/releasestation/", server.staticHandler())
	server.http = &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           withSecurityHeaders(withRequestLogging(mux, logger)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server
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
		"product":      "Zion ReleaseStation",
		"package":      "zion-releasestation",
		"version":      s.config.Version,
		"bind_address": s.config.BindAddress,
	}})
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
		"deployment": "foundation",
	}})
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
