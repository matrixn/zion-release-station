package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const githubConnectionSettingKey = "github_connection"

var githubAccountPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

type githubConnection struct {
	Connected bool   `json:"connected"`
	Account   string `json:"account"`
	Mode      string `json:"mode"`
}

func (s *Server) initializeIntegrationSettings() {
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO settings(key, value_json, updated_at) VALUES (?, ?, datetime('now'))`, githubConnectionSettingKey, `{"connected":false,"account":"","mode":"public"}`); err != nil {
		s.logger.Error("initialize GitHub connection setting", "error", err)
	}
}

func (s *Server) handleGitHubConnection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		connection, err := s.githubConnection(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to read GitHub connection settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": connection})
	case http.MethodPut:
		var payload struct {
			Account string `json:"account"`
		}
		if err := decodeJSON(w, r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_GITHUB", err.Error())
			return
		}
		account := strings.TrimSpace(payload.Account)
		if !githubAccountPattern.MatchString(account) {
			writeError(w, http.StatusBadRequest, "INVALID_GITHUB", "Enter a valid GitHub username or organization name.")
			return
		}
		connection := githubConnection{Connected: true, Account: account, Mode: "public"}
		if err := s.saveGitHubConnection(r.Context(), connection); err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to save GitHub connection settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": connection})
	case http.MethodDelete:
		connection := githubConnection{Connected: false, Mode: "public"}
		if err := s.saveGitHubConnection(r.Context(), connection); err != nil {
			writeError(w, http.StatusInternalServerError, "SETTINGS_UNAVAILABLE", "Unable to remove GitHub connection settings.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": connection})
	default:
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET, PUT and DELETE are supported.")
	}
}

func (s *Server) githubConnection(ctx context.Context) (githubConnection, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, githubConnectionSettingKey).Scan(&value); err != nil {
		return githubConnection{}, err
	}
	var connection githubConnection
	if err := json.Unmarshal([]byte(value), &connection); err != nil {
		return githubConnection{}, fmt.Errorf("decode GitHub connection: %w", err)
	}
	if connection.Mode == "" {
		connection.Mode = "public"
	}
	return connection, nil
}

func (s *Server) saveGitHubConnection(ctx context.Context, connection githubConnection) error {
	value, err := json.Marshal(connection)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE settings SET value_json = ?, updated_at = datetime('now') WHERE key = ?`, value, githubConnectionSettingKey)
	return err
}
