package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type auditEntry struct {
	ID         string `json:"id"`
	ActorType  string `json:"actor_type"`
	ActorID    string `json:"actor_id,omitempty"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
	Metadata   any    `json:"metadata,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (s *Server) recordAudit(ctx context.Context, actorType, actorID, action, entityType, entityID string, metadata map[string]any) error {
	encoded := "{}"
	if metadata != nil {
		value, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("encode audit metadata: %w", err)
		}
		encoded = string(value)
	}
	id, err := newAuditID()
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO audit_logs(id, actor_type, actor_id, action, entity_type, entity_id, metadata_json, created_at) VALUES (?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), ?, datetime('now'))`, id, actorType, actorID, action, entityType, entityID, encoded)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

func (s *Server) listAudit(ctx context.Context, limit int) ([]auditEntry, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, actor_type, COALESCE(actor_id, ''), action, COALESCE(entity_type, ''), COALESCE(entity_id, ''), COALESCE(metadata_json, '{}'), created_at FROM audit_logs ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	items := make([]auditEntry, 0)
	for rows.Next() {
		var item auditEntry
		var metadata string
		if err := rows.Scan(&item.ID, &item.ActorType, &item.ActorID, &item.Action, &item.EntityType, &item.EntityID, &metadata, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		var decoded any
		if err := json.Unmarshal([]byte(metadata), &decoded); err == nil {
			item.Metadata = decoded
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is supported.")
		return
	}
	items, err := s.listAudit(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AUDIT_UNAVAILABLE", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func newAuditID() (string, error) {
	value, err := newRandomToken(18)
	if err != nil {
		return "", err
	}
	return "audit_" + value, nil
}
