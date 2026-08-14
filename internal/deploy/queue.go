package deploy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/matrixn/zion-release-station/internal/sites"
)

type SiteLoader func(context.Context, string) (sites.Site, error)

type Queue struct {
	db        *sql.DB
	runner    *Runner
	loadSite  SiteLoader
	jobs      chan queuedJob
	workers   int
	wg        sync.WaitGroup
	mu        sync.Mutex
	enqueueMu sync.Mutex
	siteGate  map[string]chan struct{}
}

type queuedJob struct {
	ID               string
	SiteID           string
	Site             sites.Site
	Ref              string
	TriggerType      string
	DeploymentMethod string
}

func NewQueue(db *sql.DB, runner *Runner, loadSite SiteLoader, workers int) *Queue {
	if workers < 1 {
		workers = 1
	}
	return &Queue{db: db, runner: runner, loadSite: loadSite, workers: workers, jobs: make(chan queuedJob, 100), siteGate: make(map[string]chan struct{})}
}

func (q *Queue) Start(ctx context.Context) {
	for index := 0; index < q.workers; index++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
	q.recoverQueued(ctx)
}

func (q *Queue) Wait() { q.wg.Wait() }

func (q *Queue) Enqueue(ctx context.Context, site sites.Site, ref, triggerType, deploymentMethod string) (Deployment, error) {
	if site.ID == "" {
		return Deployment{}, errors.New("site is required")
	}
	if site.Repository == nil {
		return Deployment{}, errors.New("site has no repository configured")
	}
	if site.Strategy != "atomic" {
		return Deployment{}, errors.New("queued GitHub deployment currently requires Atomic releases")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = strings.TrimSpace(site.Repository.Branch)
	}
	if triggerType == "" {
		triggerType = "manual"
	}
	if deploymentMethod == "" {
		deploymentMethod = "manual"
	}
	q.enqueueMu.Lock()
	defer q.enqueueMu.Unlock()
	if existing, ok, err := q.findActiveDuplicate(ctx, site.ID, ref); err != nil {
		return Deployment{}, err
	} else if ok {
		return existing, nil
	}
	if _, err := q.db.ExecContext(ctx, `UPDATE deployments SET status = 'superseded', error_code = 'SUPERSEDED', error_summary = 'Replaced by a newer pending deployment', finished_at = datetime('now') WHERE site_id = ? AND status = 'queued'`, site.ID); err != nil {
		return Deployment{}, fmt.Errorf("apply latest pending policy: %w", err)
	}
	id, err := newID("dep_")
	if err != nil {
		return Deployment{}, err
	}
	branch := strings.TrimSpace(site.Repository.Branch)
	if branch == "" {
		branch = strings.TrimSpace(site.Repository.GitHubDefaultBranch)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := q.db.ExecContext(ctx, `INSERT INTO deployments(id, site_id, trigger_type, trigger_reference, branch, deployment_method, status, queued_at, created_at) VALUES (?, ?, ?, ?, ?, ?, 'queued', ?, ?)`, id, site.ID, triggerType, ref, branch, deploymentMethod, now, now); err != nil {
		return Deployment{}, fmt.Errorf("queue deployment: %w", err)
	}
	queued, err := q.runner.GetDeployment(ctx, site.ID, id)
	if err != nil {
		return Deployment{}, err
	}
	q.runner.publishStatus(id, "queued", "Deployment queued")
	select {
	case q.jobs <- queuedJob{ID: id, SiteID: site.ID, Site: site, Ref: ref, TriggerType: triggerType, DeploymentMethod: deploymentMethod}:
		return queued, nil
	case <-ctx.Done():
		_ = q.runner.MarkFailed(context.Background(), id, "QUEUE_CANCELLED", ctx.Err().Error())
		return Deployment{}, ctx.Err()
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-q.jobs:
			q.runJob(ctx, job)
		}
	}
}

func (q *Queue) runJob(ctx context.Context, job queuedJob) {
	if !q.isQueued(ctx, job.ID) {
		return
	}
	release := q.lockSite(ctx, job.Site.ID)
	if release == nil {
		_ = q.runner.MarkFailed(context.Background(), job.ID, "QUEUE_CANCELLED", ctx.Err().Error())
		return
	}
	defer release()
	if !q.isQueued(ctx, job.ID) {
		return
	}
	_, err := q.runner.DeployGitHubRefWithID(ctx, job.Site, job.Ref, job.TriggerType, job.DeploymentMethod, job.ID)
	if err != nil {
		_ = q.runner.MarkFailed(context.Background(), job.ID, "DEPLOYMENT_FAILED", err.Error())
	}
}

func (q *Queue) isQueued(ctx context.Context, deploymentID string) bool {
	var status string
	if err := q.db.QueryRowContext(ctx, `SELECT status FROM deployments WHERE id = ?`, deploymentID).Scan(&status); err != nil {
		return false
	}
	return status == "queued"
}

func (q *Queue) lockSite(ctx context.Context, siteID string) func() {
	q.mu.Lock()
	gate := q.siteGate[siteID]
	if gate == nil {
		gate = make(chan struct{}, 1)
		q.siteGate[siteID] = gate
	}
	q.mu.Unlock()
	select {
	case gate <- struct{}{}:
		return func() { <-gate }
	case <-ctx.Done():
		return nil
	}
}

func (q *Queue) recoverQueued(ctx context.Context) {
	rows, err := q.db.QueryContext(ctx, `SELECT id, site_id, COALESCE(trigger_reference, ''), trigger_type, deployment_method FROM deployments WHERE status = 'queued' ORDER BY queued_at ASC LIMIT 100`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var job queuedJob
		if err := rows.Scan(&job.ID, &job.SiteID, &job.Ref, &job.TriggerType, &job.DeploymentMethod); err != nil {
			continue
		}
		if q.loadSite == nil {
			continue
		}
		job.Site, err = q.loadSite(ctx, job.SiteID)
		if err != nil {
			_ = q.runner.MarkFailed(context.Background(), job.ID, "SITE_UNAVAILABLE", err.Error())
			continue
		}
		select {
		case q.jobs <- job:
		default:
			return
		}
	}
}

func (q *Queue) findActiveDuplicate(ctx context.Context, siteID, ref string) (Deployment, bool, error) {
	row := q.db.QueryRowContext(ctx, `SELECT `+deploymentColumns+` FROM deployments WHERE site_id = ? AND status IN ('queued', 'running') AND COALESCE(trigger_reference, '') = ? ORDER BY queued_at DESC LIMIT 1`, siteID, ref)
	item, err := scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, false, nil
	}
	if err != nil {
		return Deployment{}, false, err
	}
	return item, true, nil
}
