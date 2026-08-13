package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

const deploymentColumns = `id, site_id, trigger_type, COALESCE(trigger_reference, ''), COALESCE(branch, ''), COALESCE(commit_sha, ''), COALESCE(commit_message, ''), COALESCE(commit_url, ''), deployment_method, status, COALESCE(error_code, ''), COALESCE(error_summary, ''), queued_at, COALESCE(started_at, ''), COALESCE(finished_at, ''), COALESCE(duration_ms, 0), created_at`

func (r *Runner) ListDeployments(ctx context.Context, siteID, search string, page, perPage int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	pattern := "%" + search + "%"
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deployments WHERE site_id = ? AND (commit_sha LIKE ? OR commit_message LIKE ? OR branch LIKE ?)`, siteID, pattern, pattern, pattern).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count deployments: %w", err)
	}
	offset := (page - 1) * perPage
	rows, err := r.db.QueryContext(ctx, `SELECT `+deploymentColumns+` FROM deployments WHERE site_id = ? AND (commit_sha LIKE ? OR commit_message LIKE ? OR branch LIKE ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`, siteID, pattern, pattern, pattern, perPage, offset)
	if err != nil {
		return Page{}, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()
	items := make([]Deployment, 0)
	for rows.Next() {
		var item Deployment
		if err := rows.Scan(&item.ID, &item.SiteID, &item.TriggerType, &item.TriggerReference, &item.Branch, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.DeploymentMethod, &item.Status, &item.ErrorCode, &item.ErrorSummary, &item.QueuedAt, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.CreatedAt); err != nil {
			return Page{}, fmt.Errorf("scan deployment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate deployments: %w", err)
	}
	return Page{Items: items, Page: page, PerPage: perPage, Total: total, TotalPages: int(math.Ceil(float64(total) / float64(perPage)))}, nil
}

func (r *Runner) GetDeployment(ctx context.Context, siteID, deploymentID string) (Deployment, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+deploymentColumns+` FROM deployments WHERE site_id = ? AND id = ?`, siteID, deploymentID)
	item, err := scanDeployment(row)
	if err == sql.ErrNoRows {
		return Deployment{}, sql.ErrNoRows
	}
	if err != nil {
		return Deployment{}, fmt.Errorf("get deployment: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT channel, content FROM deployment_logs WHERE deployment_id = ?`, deploymentID)
	if err != nil {
		return Deployment{}, fmt.Errorf("read deployment logs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var channel, content string
		if err := rows.Scan(&channel, &content); err != nil {
			return Deployment{}, fmt.Errorf("scan deployment log: %w", err)
		}
		if channel == "build" {
			item.BuildLog = content
		} else if channel == "deployment" {
			item.DeploymentLog = content
		}
	}
	if item.Steps, err = readDeploymentSteps(ctx, r.db, deploymentID); err != nil {
		return Deployment{}, err
	}
	return item, rows.Err()
}

func (r *Runner) DeployedCommitIDs(ctx context.Context, siteID string) (map[string]string, error) {
	statuses, err := r.CommitStatuses(ctx, siteID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for sha, item := range statuses {
		if item.Status == "deployed" {
			result[sha] = item.DeploymentID
		}
	}
	return result, nil
}

type CommitStatus struct {
	DeploymentID string
	Status       string
}

func (r *Runner) CommitStatuses(ctx context.Context, siteID string) (map[string]CommitStatus, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT commit_sha, id, status FROM deployments WHERE site_id = ? AND commit_sha IS NOT NULL AND commit_sha <> '' ORDER BY created_at DESC`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list commit statuses: %w", err)
	}
	defer rows.Close()
	result := make(map[string]CommitStatus)
	for rows.Next() {
		var sha, id, status string
		if err := rows.Scan(&sha, &id, &status); err != nil {
			return nil, err
		}
		if _, exists := result[sha]; !exists {
			result[sha] = CommitStatus{DeploymentID: id, Status: status}
		}
	}
	return result, rows.Err()
}
