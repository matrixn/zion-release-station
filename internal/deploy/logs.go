package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type deploymentLogs struct {
	channels map[string]*strings.Builder
}

func newDeploymentLogs() *deploymentLogs {
	return &deploymentLogs{channels: map[string]*strings.Builder{
		"build":      {},
		"deployment": {},
	}}
}

func (l *deploymentLogs) add(channel, message string) {
	if _, ok := l.channels[channel]; !ok {
		l.channels[channel] = &strings.Builder{}
	}
	l.channels[channel].WriteString(time.Now().UTC().Format(time.RFC3339) + "  " + message + "\n")
}

func (r *Runner) saveDeploymentLogs(ctx context.Context, deploymentID string, logs *deploymentLogs) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for channel, content := range logs.channels {
		if _, err := r.db.ExecContext(ctx, `INSERT INTO deployment_logs(id, deployment_id, channel, content, created_at) VALUES (?, ?, ?, ?, ?) ON CONFLICT(deployment_id, channel) DO UPDATE SET content = excluded.content, created_at = excluded.created_at`, deploymentID+"_"+channel, deploymentID, channel, content.String(), now); err != nil {
			return fmt.Errorf("save %s deployment log: %w", channel, err)
		}
	}
	return nil
}

func scanDeployment(row *sql.Row) (Deployment, error) {
	var item Deployment
	err := row.Scan(&item.ID, &item.SiteID, &item.TriggerType, &item.Branch, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.DeploymentMethod, &item.Status, &item.ErrorCode, &item.ErrorSummary, &item.QueuedAt, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.CreatedAt)
	return item, err
}
