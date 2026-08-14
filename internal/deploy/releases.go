package deploy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/githubconnector"
	"github.com/matrixn/zion-release-station/internal/sites"
)

type Release struct {
	ID            string `json:"id"`
	SiteID        string `json:"site_id"`
	DeploymentID  string `json:"deployment_id"`
	ReleaseName   string `json:"release_name"`
	ReleasePath   string `json:"release_path"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	CommitURL     string `json:"commit_url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	Active        bool   `json:"active"`
	HealthStatus  string `json:"health_status,omitempty"`
	CreatedAt     string `json:"created_at"`
	ActivatedAt   string `json:"activated_at,omitempty"`
}

func (r *Runner) ListReleases(ctx context.Context, siteID string) ([]Release, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.site_id, r.deployment_id, r.release_name, r.release_path, COALESCE(r.commit_sha, ''), COALESCE(d.commit_message, ''), COALESCE(d.commit_url, ''), COALESCE(d.branch, ''), r.active, COALESCE(r.health_status, ''), r.created_at, COALESCE(r.activated_at, '') FROM releases r LEFT JOIN deployments d ON d.id = r.deployment_id WHERE r.site_id = ? ORDER BY r.created_at DESC`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}
	defer rows.Close()
	items := make([]Release, 0)
	for rows.Next() {
		var item Release
		var active int
		if err := rows.Scan(&item.ID, &item.SiteID, &item.DeploymentID, &item.ReleaseName, &item.ReleasePath, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.Branch, &active, &item.HealthStatus, &item.CreatedAt, &item.ActivatedAt); err != nil {
			return nil, fmt.Errorf("scan release: %w", err)
		}
		item.Active = active != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Runner) getRelease(ctx context.Context, siteID, releaseID string) (Release, error) {
	var item Release
	var active int
	err := r.db.QueryRowContext(ctx, `SELECT r.id, r.site_id, r.deployment_id, r.release_name, r.release_path, COALESCE(r.commit_sha, ''), COALESCE(d.commit_message, ''), COALESCE(d.commit_url, ''), COALESCE(d.branch, ''), r.active, COALESCE(r.health_status, ''), r.created_at, COALESCE(r.activated_at, '') FROM releases r LEFT JOIN deployments d ON d.id = r.deployment_id WHERE r.site_id = ? AND r.id = ?`, siteID, releaseID).Scan(&item.ID, &item.SiteID, &item.DeploymentID, &item.ReleaseName, &item.ReleasePath, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.Branch, &active, &item.HealthStatus, &item.CreatedAt, &item.ActivatedAt)
	if err != nil {
		return Release{}, err
	}
	item.Active = active != 0
	return item, nil
}

func (r *Runner) activeRelease(ctx context.Context, siteID string) (Release, error) {
	var item Release
	var active int
	err := r.db.QueryRowContext(ctx, `SELECT r.id, r.site_id, r.deployment_id, r.release_name, r.release_path, COALESCE(r.commit_sha, ''), COALESCE(d.commit_message, ''), COALESCE(d.commit_url, ''), COALESCE(d.branch, ''), r.active, COALESCE(r.health_status, ''), r.created_at, COALESCE(r.activated_at, '') FROM releases r LEFT JOIN deployments d ON d.id = r.deployment_id WHERE r.site_id = ? AND r.active = 1 LIMIT 1`, siteID).Scan(&item.ID, &item.SiteID, &item.DeploymentID, &item.ReleaseName, &item.ReleasePath, &item.CommitSHA, &item.CommitMessage, &item.CommitURL, &item.Branch, &active, &item.HealthStatus, &item.CreatedAt, &item.ActivatedAt)
	if err != nil {
		return Release{}, err
	}
	item.Active = active != 0
	return item, nil
}

func (r *Runner) setReleaseHealth(ctx context.Context, releaseID, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE releases SET health_status = ? WHERE id = ?`, status, releaseID)
	return err
}

func checkHealth(ctx context.Context, healthURL string) error {
	healthURL = strings.TrimSpace(healthURL)
	if healthURL == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(healthURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("invalid health check URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("health check returned HTTP %d", response.StatusCode)
	}
	return nil
}

func (r *Runner) switchCurrent(projectRoot, releasePath string) error {
	relative, err := filepath.Rel(projectRoot, releasePath)
	if err != nil || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || relative == ".." {
		return fmt.Errorf("release path is outside project root")
	}
	currentPath := filepath.Join(projectRoot, ".current")
	nextPath := currentPath + ".next-" + filepath.Base(releasePath)
	if err := os.Remove(nextPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Symlink(relative, nextPath); err != nil {
		return fmt.Errorf("prepare current symlink: %w", err)
	}
	if err := replaceSymlink(nextPath, currentPath); err != nil {
		_ = os.Remove(nextPath)
		return fmt.Errorf("activate current symlink: %w", err)
	}
	return nil
}

func (r *Runner) restoreRelease(ctx context.Context, site sites.Site, release Release, deploymentID string, logs *deploymentLogs) error {
	if err := r.switchCurrent(site.ProjectRoot, release.ReleasePath); err != nil {
		return err
	}
	return runDeploymentScript(ctx, site, release.ReleasePath, filepath.Join(site.ProjectRoot, ".current"), deploymentID, release.ID, release.CommitSHA, logs)
}

func (r *Runner) pruneReleases(ctx context.Context, site sites.Site) error {
	keep := site.DeploymentRetention
	if keep < 2 {
		keep = 2
	}
	items, err := r.ListReleases(ctx, site.ID)
	if err != nil {
		return err
	}
	kept := 0
	for _, item := range items {
		if item.Active {
			kept++
			continue
		}
		if kept < keep {
			kept++
			continue
		}
		releasesRoot := filepath.Join(site.ProjectRoot, ".zion", "releases")
		cleanPath := filepath.Clean(item.ReleasePath)
		if filepath.Dir(cleanPath) != filepath.Clean(releasesRoot) {
			continue
		}
		if err := os.RemoveAll(cleanPath); err != nil {
			return fmt.Errorf("remove release %s: %w", item.ID, err)
		}
		if _, err := r.db.ExecContext(ctx, `DELETE FROM releases WHERE id = ? AND active = 0`, item.ID); err != nil {
			return fmt.Errorf("remove release record %s: %w", item.ID, err)
		}
	}
	return nil
}

func (r *Runner) Rollback(ctx context.Context, site sites.Site, releaseID string) (result Result, err error) {
	if site.Strategy != "atomic" {
		return Result{}, fmt.Errorf("site %q is not configured for atomic releases", site.ID)
	}
	target, err := r.getRelease(ctx, site.ID, releaseID)
	if err != nil {
		return Result{}, err
	}
	if target.Active {
		return Result{}, fmt.Errorf("release %s is already active", releaseID)
	}
	releasesRoot := filepath.Clean(filepath.Join(site.ProjectRoot, ".zion", "releases"))
	cleanTargetPath := filepath.Clean(target.ReleasePath)
	if filepath.Dir(cleanTargetPath) != releasesRoot {
		return Result{}, fmt.Errorf("release path is outside the managed release directory")
	}
	target.ReleasePath = cleanTargetPath
	if _, err := os.Stat(target.ReleasePath); err != nil {
		return Result{}, fmt.Errorf("release files are unavailable: %w", err)
	}
	lock, err := acquireLock(filepath.Join(site.ProjectRoot, ".zion", "deploy.lock"))
	if err != nil {
		return Result{}, err
	}
	defer releaseLock(lock)
	deploymentID, err := newID("dep_")
	if err != nil {
		return Result{}, err
	}
	commit := githubconnector.Commit{SHA: target.CommitSHA, Message: target.CommitMessage, URL: target.CommitURL}
	if err := r.startDeployment(ctx, deploymentID, site.ID, target.Branch, commit, "rollback", "rollback"); err != nil {
		return Result{}, err
	}
	logs := newDeploymentLogs(r.hub, deploymentID)
	logs.add("build", "Preparing rollback to "+target.CommitSHA)
	logs.add("deployment", "Application files will be rolled back; database migrations are not reversed")
	steps := newStepTracker(ctx, r, deploymentID)
	failed := true
	defer func() {
		if err != nil {
			logs.add("deployment", "ERROR: "+err.Error())
		}
		_ = r.saveDeploymentLogs(context.Background(), deploymentID, logs)
		if failed {
			_ = r.finishDeployment(context.Background(), deploymentID, "failed", "ROLLBACK_FAILED")
		} else {
			_ = r.finishDeployment(context.Background(), deploymentID, "deployed", "")
		}
	}()
	if err := steps.begin("rollback", "Switch current release", "deploy"); err != nil {
		return Result{}, err
	}
	if err := r.restoreRelease(ctx, site, target, deploymentID, logs); err != nil {
		return Result{}, err
	}
	if err := steps.finish("rollback", "completed", nil); err != nil {
		return Result{}, err
	}
	if err := r.activateRelease(ctx, site.ID, target.ID); err != nil {
		return Result{}, err
	}
	if site.HealthCheckURL != "" {
		if err := steps.begin("health_check", "Verify rolled back site", "health"); err != nil {
			return Result{}, err
		}
		if err := checkHealth(ctx, site.HealthCheckURL); err != nil {
			_ = r.setReleaseHealth(ctx, target.ID, "failed")
			_ = steps.finish("health_check", "failed", nil)
			return Result{}, fmt.Errorf("rollback health check failed: %w", err)
		}
		_ = r.setReleaseHealth(ctx, target.ID, "healthy")
		if err := steps.finish("health_check", "completed", nil); err != nil {
			return Result{}, err
		}
	} else {
		_ = r.setReleaseHealth(ctx, target.ID, "skipped")
	}
	_ = r.pruneReleases(ctx, site)
	failed = false
	return Result{DeploymentID: deploymentID, ReleaseID: target.ID, ReleasePath: target.ReleasePath, Commit: target.CommitSHA, Status: "deployed"}, nil
}
