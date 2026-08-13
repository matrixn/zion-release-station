package httpapi

import (
	"context"
	"database/sql"
	"fmt"
)

type dashboardMetrics struct {
	SuccessfulDeploys int64            `json:"successful_deploys"`
	TotalDeploys      int64            `json:"total_deploys"`
	MedianDurationMS  int64            `json:"median_duration_ms"`
	RunningDeploys    int64            `json:"running_deploys"`
	QueueStatus       string           `json:"queue_status"`
	Latest            map[string]any   `json:"latest,omitempty"`
	Services          []map[string]any `json:"services"`
}

func (s *Server) dashboardMetrics(ctx context.Context) (dashboardMetrics, error) {
	var result dashboardMetrics
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'deployed' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0) FROM deployments`).Scan(&result.TotalDeploys, &result.SuccessfulDeploys, &result.RunningDeploys); err != nil {
		return dashboardMetrics{}, fmt.Errorf("read deployment metrics: %w", err)
	}
	result.QueueStatus = "idle"
	if result.RunningDeploys > 0 {
		result.QueueStatus = "running"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT duration_ms FROM deployments WHERE status = 'deployed' AND duration_ms IS NOT NULL ORDER BY duration_ms`)
	if err != nil {
		return dashboardMetrics{}, fmt.Errorf("read deployment durations: %w", err)
	}
	var durations []int64
	for rows.Next() {
		var value int64
		if err := rows.Scan(&value); err != nil {
			rows.Close()
			return dashboardMetrics{}, err
		}
		durations = append(durations, value)
	}
	rows.Close()
	if len(durations) > 0 {
		result.MedianDurationMS = durations[len(durations)/2]
		if len(durations)%2 == 0 {
			result.MedianDurationMS = (durations[len(durations)/2-1] + durations[len(durations)/2]) / 2
		}
	}
	var latest struct {
		SiteID, SiteName, Status, Branch, CommitSHA, CommitMessage, CreatedAt string
		DurationMS                                                            sql.NullInt64
	}
	err = s.db.QueryRowContext(ctx, `SELECT d.site_id, s.name, d.status, COALESCE(d.branch, ''), COALESCE(d.commit_sha, ''), COALESCE(d.commit_message, ''), d.created_at, d.duration_ms FROM deployments d JOIN sites s ON s.id = d.site_id ORDER BY d.created_at DESC LIMIT 1`).Scan(&latest.SiteID, &latest.SiteName, &latest.Status, &latest.Branch, &latest.CommitSHA, &latest.CommitMessage, &latest.CreatedAt, &latest.DurationMS)
	if err == nil {
		result.Latest = map[string]any{"site_id": latest.SiteID, "site_name": latest.SiteName, "status": latest.Status, "branch": latest.Branch, "commit_sha": latest.CommitSHA, "commit_message": latest.CommitMessage, "created_at": latest.CreatedAt, "duration_ms": latest.DurationMS.Int64}
	} else if err != sql.ErrNoRows {
		return dashboardMetrics{}, fmt.Errorf("read latest deployment: %w", err)
	}
	webAvailable, webErr := s.webStation.Available(ctx)
	githubReady := s.githubManaged.Configured()
	result.Services = []map[string]any{
		{"label": "Web Station", "state": state(webAvailable && webErr == nil), "detail": serviceDetail(webAvailable, webErr, "Configured roots available", "No readable roots detected")},
		{"label": "GitHub connector", "state": state(githubReady), "detail": serviceDetail(githubReady, nil, "Connected and ready", s.githubManaged.ConfigurationError())},
		{"label": "Git transport", "state": state(commandAvailable("git")), "detail": serviceDetail(commandAvailable("git"), nil, "Git executable available", "Git executable unavailable")},
		{"label": "SQLite", "state": state(s.db.PingContext(ctx) == nil), "detail": "Database connection"},
	}
	return result, nil
}

func state(ready bool) string {
	if ready {
		return "ready"
	}
	return "error"
}

func serviceDetail(ready bool, err error, ok, fallback string) string {
	if ready {
		return ok
	}
	if err != nil {
		return err.Error()
	}
	return fallback
}
