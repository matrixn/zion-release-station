package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/config"
)

type Server struct {
	config config.Config
	db     *sql.DB
	logger *slog.Logger
	http   *http.Server
}

func NewServer(cfg config.Config, db *sql.DB, logger *slog.Logger) *Server {
	server := &Server{config: cfg, db: db, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/releasestation/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/releasestation/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/releasestation/api/v1/system/capabilities", server.handleCapabilities)
	mux.HandleFunc("/api/v1/system/health", server.handleHealth)
	mux.HandleFunc("/api/v1/system/info", server.handleInfo)
	mux.HandleFunc("/api/v1/system/capabilities", server.handleCapabilities)
	mux.Handle("/releasestation/", server.staticHandler())
	server.http = &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           withSecurityHeaders(withRequestLogging(mux, logger)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server
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

func (s *Server) staticHandler() http.Handler {
	root := http.Dir(s.config.WebRoot)
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/releasestation/")
		if path == "" {
			path = "index.html"
		}
		path = filepath.ToSlash(filepath.Clean("/" + path))[1:]
		if path == "." || strings.Contains(path, "..") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(os.DirFS(s.config.WebRoot), path); err != nil {
			if os.IsNotExist(err) && !strings.Contains(filepath.Base(path), ".") {
				r.URL.Path = "/releasestation/index.html"
				files.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		r.URL.Path = "/" + path
		files.ServeHTTP(w, r)
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
