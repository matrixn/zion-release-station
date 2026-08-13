package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DeploymentStep struct {
	ID         string `json:"id"`
	Key        string `json:"step_key"`
	Name       string `json:"name"`
	Sequence   int    `json:"sequence"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type stepTracker struct {
	runner       *Runner
	ctx          context.Context
	deploymentID string
	sequence     int
	open         map[string]time.Time
}

func newStepTracker(ctx context.Context, runner *Runner, deploymentID string) *stepTracker {
	return &stepTracker{runner: runner, ctx: context.WithoutCancel(ctx), deploymentID: deploymentID, open: make(map[string]time.Time)}
}

func (t *stepTracker) begin(key, name, kind string) error {
	t.sequence++
	started := time.Now().UTC()
	id, err := newID("step_")
	if err != nil {
		return err
	}
	if _, err := t.runner.db.ExecContext(t.ctx, `INSERT INTO deployment_steps(id, deployment_id, step_key, name, sequence, type, status, started_at) VALUES (?, ?, ?, ?, ?, ?, 'running', ?)`, id, t.deploymentID, key, name, t.sequence, kind, started.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("start deployment step %s: %w", key, err)
	}
	t.open[key] = started
	if t.runner.hub != nil {
		t.runner.hub.Publish(Event{Type: "step", DeploymentID: t.deploymentID, Channel: key, Status: "running", Message: name})
	}
	return nil
}

func (t *stepTracker) finish(key, status string, exitCode *int) error {
	started, ok := t.open[key]
	if !ok {
		return nil
	}
	finished := time.Now().UTC()
	if _, err := t.runner.db.ExecContext(t.ctx, `UPDATE deployment_steps SET status = ?, exit_code = ?, finished_at = ?, duration_ms = CAST((julianday(?) - julianday(?)) * 86400000 AS INTEGER) WHERE deployment_id = ? AND step_key = ? AND status = 'running'`, status, exitCode, finished.Format(time.RFC3339Nano), finished.Format(time.RFC3339Nano), started.Format(time.RFC3339Nano), t.deploymentID, key); err != nil {
		return fmt.Errorf("finish deployment step %s: %w", key, err)
	}
	delete(t.open, key)
	if t.runner.hub != nil {
		t.runner.hub.Publish(Event{Type: "step", DeploymentID: t.deploymentID, Channel: key, Status: status, Message: key})
	}
	return nil
}

func (t *stepTracker) failOpen() {
	for key := range t.open {
		_ = t.finish(key, "failed", nil)
	}
}

func readDeploymentSteps(ctx context.Context, db *sql.DB, deploymentID string) ([]DeploymentStep, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, step_key, name, sequence, type, status, exit_code, COALESCE(started_at, ''), COALESCE(finished_at, ''), COALESCE(duration_ms, 0) FROM deployment_steps WHERE deployment_id = ? ORDER BY sequence ASC`, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("read deployment steps: %w", err)
	}
	defer rows.Close()
	steps := make([]DeploymentStep, 0)
	for rows.Next() {
		var step DeploymentStep
		var exitCode sql.NullInt64
		if err := rows.Scan(&step.ID, &step.Key, &step.Name, &step.Sequence, &step.Type, &step.Status, &exitCode, &step.StartedAt, &step.FinishedAt, &step.DurationMS); err != nil {
			return nil, fmt.Errorf("scan deployment step: %w", err)
		}
		if exitCode.Valid {
			value := int(exitCode.Int64)
			step.ExitCode = &value
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}
