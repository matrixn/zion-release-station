package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/githubconnector"
	"github.com/matrixn/zion-release-station/internal/sites"
)

const (
	githubWebhookCursorSettingKey = "github_webhook_last_event_id"
	githubWebhookPollInterval     = 5 * time.Second
)

// RunBackground polls the connector for verified GitHub deliveries. GitHub
// never calls the NAS directly: the connector authenticates the HMAC and the
// SPK reads only its own authenticated queue.
func (s *Server) RunBackground(ctx context.Context) {
	s.deployQueue.Start(ctx)
	s.processGitHubWebhookEvents(ctx)
	ticker := time.NewTicker(githubWebhookPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processGitHubWebhookEvents(ctx)
		}
	}
}

func (s *Server) processGitHubWebhookEvents(ctx context.Context) {
	if !s.githubManaged.Configured() {
		return
	}
	cursor, err := s.githubWebhookCursor(ctx)
	if err != nil {
		s.logger.Error("read GitHub webhook cursor", "error", err)
		return
	}
	events, err := s.githubManaged.WebhookEvents(ctx, cursor, 50)
	if err != nil {
		// Connector pairing and network outages are expected during setup. Keep
		// the worker alive and let the next poll retry without losing the cursor.
		s.logger.Warn("poll GitHub webhook events", "error", err)
		return
	}
	if len(events) == 0 {
		return
	}

	managedSites, err := s.sites.List(ctx)
	if err != nil {
		s.logger.Error("list sites for GitHub webhook", "error", err)
		return
	}
	for _, event := range events {
		if event.ID <= cursor {
			continue
		}
		if err := s.processGitHubWebhookEvent(ctx, managedSites, event); err != nil {
			s.logger.Error("process GitHub webhook event", "delivery_id", event.DeliveryID, "error", err)
		}
		if err := s.saveGitHubWebhookCursor(ctx, event.ID); err != nil {
			s.logger.Error("save GitHub webhook cursor", "error", err)
			return
		}
		cursor = event.ID
	}
}

func (s *Server) processGitHubWebhookEvent(ctx context.Context, managedSites []sites.Site, event githubconnector.WebhookEvent) error {
	if event.EventName != "push" || event.Deleted || event.AfterSHA == "" || event.RepositoryFullName == "" {
		return nil
	}
	const branchPrefix = "refs/heads/"
	if !strings.HasPrefix(event.RefName, branchPrefix) {
		return nil
	}
	branch := strings.TrimPrefix(event.RefName, branchPrefix)
	if branch == "" {
		return nil
	}

	for _, site := range managedSites {
		if !site.PushToDeploy || site.Repository == nil || strings.ToLower(site.Repository.Provider) != "github" {
			continue
		}
		if site.Repository.GitHubFullName != event.RepositoryFullName || site.Repository.GitHubInstallationID == nil || *site.Repository.GitHubInstallationID != event.GitHubInstallationID {
			continue
		}
		configuredBranch := strings.TrimSpace(site.Repository.Branch)
		if configuredBranch == "" {
			configuredBranch = strings.TrimSpace(site.Repository.GitHubDefaultBranch)
		}
		if configuredBranch != branch {
			continue
		}
		if site.Strategy != "atomic" {
			s.logger.Warn("skip GitHub push deployment because site strategy is not atomic", "site_id", site.ID, "strategy", site.Strategy)
			continue
		}

		queueContext, cancel := context.WithTimeout(ctx, 10*time.Second)
		result, err := s.deployQueue.Enqueue(queueContext, site, event.AfterSHA, "webhook", "webhook")
		cancel()
		if err != nil {
			return fmt.Errorf("queue deploy for site %s: %w", site.ID, err)
		}
		s.logger.Info("GitHub push queued", "site_id", site.ID, "repository", event.RepositoryFullName, "branch", branch, "commit", event.AfterSHA, "deployment_id", result.ID)
	}
	return nil
}

func (s *Server) githubWebhookCursor(ctx context.Context) (int64, error) {
	var encoded string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key = ?`, githubWebhookCursorSettingKey).Scan(&encoded)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var cursor int64
	if err := json.Unmarshal([]byte(encoded), &cursor); err == nil && cursor >= 0 {
		return cursor, nil
	}
	cursor, err = strconv.ParseInt(strings.TrimSpace(encoded), 10, 64)
	if err != nil || cursor < 0 {
		return 0, fmt.Errorf("invalid GitHub webhook cursor")
	}
	return cursor, nil
}

func (s *Server) saveGitHubWebhookCursor(ctx context.Context, cursor int64) error {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key, value_json, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`, githubWebhookCursorSettingKey, string(encoded))
	return err
}
