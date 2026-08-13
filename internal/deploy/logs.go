package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

type deploymentLogs struct {
	channels     map[string]*strings.Builder
	hub          *EventHub
	deploymentID string
}

type liveLogWriter struct {
	logs    *deploymentLogs
	channel string
	mu      sync.Mutex
	pending string
}

func (w *liveLogWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pending += string(value)
	for {
		index := strings.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}
		w.logs.add(w.channel, strings.TrimSuffix(w.pending[:index], "\r"))
		w.pending = w.pending[index+1:]
	}
	return len(value), nil
}

func (w *liveLogWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if strings.TrimSpace(w.pending) != "" {
		w.logs.add(w.channel, strings.TrimSuffix(w.pending, "\r"))
	}
	w.pending = ""
}

func newDeploymentLogs(hub *EventHub, deploymentID string) *deploymentLogs {
	return &deploymentLogs{channels: map[string]*strings.Builder{
		"build":      {},
		"deployment": {},
	}, hub: hub, deploymentID: deploymentID}
}

func (l *deploymentLogs) add(channel, message string) {
	if _, ok := l.channels[channel]; !ok {
		l.channels[channel] = &strings.Builder{}
	}
	l.channels[channel].WriteString(time.Now().UTC().Format(time.RFC3339) + "  " + message + "\n")
	if l.hub != nil {
		l.hub.Publish(Event{Type: "log", DeploymentID: l.deploymentID, Channel: channel, Message: message})
	}
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
	err := row.Scan(&item.ID, &item.SiteID, &item.TriggerType, &item.TriggerReference, &item.Branch, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.DeploymentMethod, &item.Status, &item.ErrorCode, &item.ErrorSummary, &item.QueuedAt, &item.StartedAt, &item.FinishedAt, &item.DurationMS, &item.CreatedAt)
	return item, err
}
