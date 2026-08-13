package sites

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var ErrNotFound = errors.New("site not found")

type Site struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Slug                string          `json:"slug"`
	Hostname            string          `json:"hostname"`
	ProjectRoot         string          `json:"project_root"`
	WebRoot             string          `json:"web_root"`
	Framework           string          `json:"framework"`
	CustomFramework     string          `json:"custom_framework,omitempty"`
	Strategy            string          `json:"strategy"`
	Status              string          `json:"status"`
	Tags                []string        `json:"tags"`
	Color               string          `json:"color"`
	PushToDeploy        bool            `json:"push_to_deploy"`
	DeployScript        string          `json:"deploy_script"`
	DeploymentRetention int             `json:"deployment_retention"`
	Runtime             json.RawMessage `json:"runtime,omitempty"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
	ArchivedAt          *string         `json:"archived_at,omitempty"`
	Repository          *Repository     `json:"repository,omitempty"`
}

type Repository struct {
	ID                   string  `json:"id"`
	SiteID               string  `json:"site_id"`
	Provider             string  `json:"provider"`
	CloneURL             string  `json:"clone_url"`
	Branch               string  `json:"branch"`
	CredentialID         *string `json:"credential_id,omitempty"`
	GitHubInstallationID *int64  `json:"github_installation_id,omitempty"`
	GitHubRepositoryID   *int64  `json:"github_repository_id,omitempty"`
	GitHubFullName       string  `json:"github_full_name,omitempty"`
	GitHubDefaultBranch  string  `json:"github_default_branch,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type RepositoryInput struct {
	Provider             string
	CloneURL             string
	Branch               string
	GitHubInstallationID *int64
	GitHubRepositoryID   *int64
	GitHubFullName       string
	GitHubDefaultBranch  string
}

type Input struct {
	Name                string
	Slug                string
	Hostname            string
	ProjectRoot         string
	WebRoot             string
	Framework           string
	CustomFramework     string
	Strategy            string
	Status              string
	Tags                []string
	Color               string
	PushToDeploy        bool
	DeployScript        string
	DeploymentRetention int
	Runtime             any
	Repository          *RepositoryInput
}

type Store struct {
	db *sql.DB
}

// DefaultAtomicDeployScript is intentionally small and reviewable. It copies the
// prepared .current release into a temporary document-root directory and then
// swaps that directory into place with a same-filesystem rename.
const DefaultAtomicDeployScript = `#!/bin/sh
set -eu

SOURCE_DIR="${CURRENT_DIR:-${PROJECT_ROOT}/.current}"
TARGET_DIR="${WEB_ROOT:?WEB_ROOT is required}"
RELEASE_ID="${RELEASE_ID:-manual}"
TARGET_PARENT=$(dirname "$TARGET_DIR")
TARGET_NAME=$(basename "$TARGET_DIR")
STAGING_DIR="${TARGET_PARENT}/.${TARGET_NAME}.staging-${RELEASE_ID}"
BACKUP_DIR="${TARGET_PARENT}/.${TARGET_NAME}.previous-${RELEASE_ID}"

cleanup() {
    if [ ! -e "$TARGET_DIR" ] && [ -e "$BACKUP_DIR" ]; then
        mv "$BACKUP_DIR" "$TARGET_DIR"
    fi
    rm -rf "$STAGING_DIR"
}
trap cleanup EXIT

rm -rf "$STAGING_DIR"
mkdir -p "$STAGING_DIR"
cp -a "$SOURCE_DIR"/. "$STAGING_DIR"/
rm -rf "$BACKUP_DIR"
if [ -e "$TARGET_DIR" ] || [ -L "$TARGET_DIR" ]; then
    mv "$TARGET_DIR" "$BACKUP_DIR"
fi
mv "$STAGING_DIR" "$TARGET_DIR"
rm -rf "$BACKUP_DIR"
trap - EXIT
`

func EffectiveDeployScript(strategy, script string) string {
	if strategy == "atomic" && strings.TrimSpace(script) == "" {
		return DefaultAtomicDeployScript
	}
	return strings.TrimSpace(script)
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, slug, COALESCE(hostname, ''), project_root, COALESCE(web_root, ''), framework, COALESCE(custom_framework, ''), strategy, status, COALESCE(runtime_json, ''), COALESCE(tags_json, '[]'), COALESCE(color, '#f28c3b'), COALESCE(push_to_deploy, 0), COALESCE(deploy_script, ''), COALESCE(deployment_retention, 4), created_at, updated_at, archived_at FROM sites WHERE archived_at IS NULL ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	var result []Site
	for rows.Next() {
		var site Site
		var runtime, tags string
		var push int
		if err := rows.Scan(&site.ID, &site.Name, &site.Slug, &site.Hostname, &site.ProjectRoot, &site.WebRoot, &site.Framework, &site.CustomFramework, &site.Strategy, &site.Status, &runtime, &tags, &site.Color, &push, &site.DeployScript, &site.DeploymentRetention, &site.CreatedAt, &site.UpdatedAt, &site.ArchivedAt); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		if runtime != "" {
			site.Runtime = json.RawMessage(runtime)
		}
		site.DeployScript = EffectiveDeployScript(site.Strategy, site.DeployScript)
		site.PushToDeploy = push != 0
		_ = json.Unmarshal([]byte(tags), &site.Tags)
		result = append(result, site)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate sites: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close sites: %w", err)
	}
	for index := range result {
		if err := s.attachRepository(ctx, &result[index]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) Get(ctx context.Context, id string) (Site, error) {
	var site Site
	var runtime, tags string
	var push int
	err := s.db.QueryRowContext(ctx, `SELECT id, name, slug, COALESCE(hostname, ''), project_root, COALESCE(web_root, ''), framework, COALESCE(custom_framework, ''), strategy, status, COALESCE(runtime_json, ''), COALESCE(tags_json, '[]'), COALESCE(color, '#f28c3b'), COALESCE(push_to_deploy, 0), COALESCE(deploy_script, ''), COALESCE(deployment_retention, 4), created_at, updated_at, archived_at FROM sites WHERE id = ? AND archived_at IS NULL`, id).Scan(&site.ID, &site.Name, &site.Slug, &site.Hostname, &site.ProjectRoot, &site.WebRoot, &site.Framework, &site.CustomFramework, &site.Strategy, &site.Status, &runtime, &tags, &site.Color, &push, &site.DeployScript, &site.DeploymentRetention, &site.CreatedAt, &site.UpdatedAt, &site.ArchivedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Site{}, ErrNotFound
		}
		return Site{}, fmt.Errorf("get site: %w", err)
	}
	if runtime != "" {
		site.Runtime = json.RawMessage(runtime)
	}
	site.DeployScript = EffectiveDeployScript(site.Strategy, site.DeployScript)
	site.PushToDeploy = push != 0
	_ = json.Unmarshal([]byte(tags), &site.Tags)
	if err := s.attachRepository(ctx, &site); err != nil {
		return Site{}, err
	}
	return site, nil
}

func (s *Store) FindByProjectRoot(ctx context.Context, projectRoot string) (Site, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM sites WHERE project_root = ? AND archived_at IS NULL`, projectRoot).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return Site{}, ErrNotFound
		}
		return Site{}, fmt.Errorf("find site: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *Store) Create(ctx context.Context, input Input) (Site, error) {
	if err := validateInput(input); err != nil {
		return Site{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id, err := newID()
	if err != nil {
		return Site{}, err
	}
	runtime, err := runtimeJSON(input.Runtime)
	if err != nil {
		return Site{}, err
	}
	tags, err := json.Marshal(normalizeTags(input.Tags))
	if err != nil {
		return Site{}, fmt.Errorf("encode site tags: %w", err)
	}
	retention := input.DeploymentRetention
	if retention == 0 {
		retention = 4
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sites(id, name, slug, hostname, project_root, web_root, framework, custom_framework, strategy, status, runtime_json, tags_json, color, push_to_deploy, deploy_script, deployment_retention, created_at, updated_at) VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)`, id, input.Name, input.Slug, input.Hostname, input.ProjectRoot, input.WebRoot, input.Framework, input.CustomFramework, input.Strategy, input.Status, runtime, string(tags), normalizeColor(input.Color), boolInt(input.PushToDeploy), EffectiveDeployScript(input.Strategy, input.DeployScript), retention, now, now)
	if err != nil {
		return Site{}, fmt.Errorf("create site: %w", err)
	}
	if input.Repository != nil {
		if err := s.SaveRepository(ctx, id, *input.Repository); err != nil {
			_, _ = s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
			return Site{}, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Store) Update(ctx context.Context, id string, input Input) (Site, error) {
	if err := validateInput(input); err != nil {
		return Site{}, err
	}
	runtime, err := runtimeJSON(input.Runtime)
	if err != nil {
		return Site{}, err
	}
	tags, err := json.Marshal(normalizeTags(input.Tags))
	if err != nil {
		return Site{}, fmt.Errorf("encode site tags: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE sites SET name = ?, slug = ?, hostname = NULLIF(?, ''), project_root = ?, web_root = NULLIF(?, ''), framework = ?, custom_framework = NULLIF(?, ''), strategy = ?, status = ?, runtime_json = NULLIF(?, ''), tags_json = ?, color = ?, push_to_deploy = ?, deploy_script = ?, deployment_retention = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL`, input.Name, input.Slug, input.Hostname, input.ProjectRoot, input.WebRoot, input.Framework, input.CustomFramework, input.Strategy, input.Status, runtime, string(tags), normalizeColor(input.Color), boolInt(input.PushToDeploy), EffectiveDeployScript(input.Strategy, input.DeployScript), input.DeploymentRetention, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return Site{}, fmt.Errorf("update site: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Site{}, ErrNotFound
	}
	if input.Repository != nil {
		if err := s.SaveRepository(ctx, id, *input.Repository); err != nil {
			return Site{}, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Store) GetRepository(ctx context.Context, siteID string) (*Repository, error) {
	var repository Repository
	err := s.db.QueryRowContext(ctx, `SELECT id, site_id, provider, clone_url, branch, credential_id, github_installation_id, github_repository_id, COALESCE(github_full_name, ''), COALESCE(github_default_branch, ''), created_at, updated_at FROM repositories WHERE site_id = ?`, siteID).Scan(&repository.ID, &repository.SiteID, &repository.Provider, &repository.CloneURL, &repository.Branch, &repository.CredentialID, &repository.GitHubInstallationID, &repository.GitHubRepositoryID, &repository.GitHubFullName, &repository.GitHubDefaultBranch, &repository.CreatedAt, &repository.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get repository: %w", err)
	}
	return &repository, nil
}

func (s *Store) SaveRepository(ctx context.Context, siteID string, input RepositoryInput) error {
	if err := validateRepository(input); err != nil {
		return err
	}
	id, err := newRepositoryID()
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO repositories(id, site_id, provider, clone_url, branch, github_installation_id, github_repository_id, github_full_name, github_default_branch, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?) ON CONFLICT(site_id) DO UPDATE SET provider = excluded.provider, clone_url = excluded.clone_url, branch = excluded.branch, github_installation_id = excluded.github_installation_id, github_repository_id = excluded.github_repository_id, github_full_name = excluded.github_full_name, github_default_branch = excluded.github_default_branch, updated_at = excluded.updated_at`, id, siteID, strings.ToLower(strings.TrimSpace(input.Provider)), strings.TrimSpace(input.CloneURL), strings.TrimSpace(input.Branch), input.GitHubInstallationID, input.GitHubRepositoryID, strings.TrimSpace(input.GitHubFullName), strings.TrimSpace(input.GitHubDefaultBranch), now, now)
	if err != nil {
		return fmt.Errorf("save repository: %w", err)
	}
	return nil
}

func (s *Store) attachRepository(ctx context.Context, site *Site) error {
	repository, err := s.GetRepository(ctx, site.ID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	site.Repository = repository
	return nil
}

func (s *Store) Archive(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE sites SET status = 'archived', archived_at = ?, updated_at = ? WHERE id = ? AND archived_at IS NULL`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("archive site: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	return nil
}

func validateInput(input Input) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("site name is required")
	}
	if !slugPattern.MatchString(input.Slug) {
		return fmt.Errorf("slug must contain lowercase letters, numbers and hyphens")
	}
	if input.ProjectRoot == "" {
		return fmt.Errorf("project root is required")
	}
	if input.Framework == "" {
		return fmt.Errorf("framework is required")
	}
	if input.Strategy != "in_place" && input.Strategy != "atomic" {
		return fmt.Errorf("strategy must be in_place or atomic")
	}
	if input.DeploymentRetention < 0 || input.DeploymentRetention > 100 {
		return fmt.Errorf("deployment retention must be between 0 and 100")
	}
	if len(input.DeployScript) > 64*1024 {
		return fmt.Errorf("deployment script must be at most 64 KB")
	}
	if input.Repository != nil {
		if err := validateRepository(*input.Repository); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len([]rune(tag)) > 50 {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func normalizeColor(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 7 && value[0] == '#' {
		return value
	}
	return "#f28c3b"
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func validateRepository(input RepositoryInput) error {
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider != "github" && provider != "gitlab" && provider != "bitbucket" && provider != "generic" {
		return fmt.Errorf("repository provider must be github, gitlab, bitbucket or generic")
	}
	if strings.TrimSpace(input.CloneURL) == "" {
		return fmt.Errorf("repository URL is required")
	}
	if strings.TrimSpace(input.Branch) == "" {
		return fmt.Errorf("repository branch is required")
	}
	return nil
}

func runtimeJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode site runtime: %w", err)
	}
	return string(encoded), nil
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate site id: %w", err)
	}
	return "site_" + hex.EncodeToString(value[:]), nil
}

func newRepositoryID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate repository id: %w", err)
	}
	return "repo_" + hex.EncodeToString(value[:]), nil
}
