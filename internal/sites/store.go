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
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Hostname    string          `json:"hostname"`
	ProjectRoot string          `json:"project_root"`
	WebRoot     string          `json:"web_root"`
	Framework   string          `json:"framework"`
	Strategy    string          `json:"strategy"`
	Status      string          `json:"status"`
	Runtime     json.RawMessage `json:"runtime,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	ArchivedAt  *string         `json:"archived_at,omitempty"`
}

type Input struct {
	Name        string
	Slug        string
	Hostname    string
	ProjectRoot string
	WebRoot     string
	Framework   string
	Strategy    string
	Status      string
	Runtime     any
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context) ([]Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, slug, COALESCE(hostname, ''), project_root, COALESCE(web_root, ''), framework, strategy, status, COALESCE(runtime_json, ''), created_at, updated_at, archived_at FROM sites WHERE archived_at IS NULL ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list sites: %w", err)
	}
	defer rows.Close()

	var result []Site
	for rows.Next() {
		var site Site
		var runtime string
		if err := rows.Scan(&site.ID, &site.Name, &site.Slug, &site.Hostname, &site.ProjectRoot, &site.WebRoot, &site.Framework, &site.Strategy, &site.Status, &runtime, &site.CreatedAt, &site.UpdatedAt, &site.ArchivedAt); err != nil {
			return nil, fmt.Errorf("scan site: %w", err)
		}
		if runtime != "" {
			site.Runtime = json.RawMessage(runtime)
		}
		result = append(result, site)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sites: %w", err)
	}
	return result, nil
}

func (s *Store) Get(ctx context.Context, id string) (Site, error) {
	var site Site
	var runtime string
	err := s.db.QueryRowContext(ctx, `SELECT id, name, slug, COALESCE(hostname, ''), project_root, COALESCE(web_root, ''), framework, strategy, status, COALESCE(runtime_json, ''), created_at, updated_at, archived_at FROM sites WHERE id = ? AND archived_at IS NULL`, id).Scan(&site.ID, &site.Name, &site.Slug, &site.Hostname, &site.ProjectRoot, &site.WebRoot, &site.Framework, &site.Strategy, &site.Status, &runtime, &site.CreatedAt, &site.UpdatedAt, &site.ArchivedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Site{}, ErrNotFound
		}
		return Site{}, fmt.Errorf("get site: %w", err)
	}
	if runtime != "" {
		site.Runtime = json.RawMessage(runtime)
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
	_, err = s.db.ExecContext(ctx, `INSERT INTO sites(id, name, slug, hostname, project_root, web_root, framework, strategy, status, runtime_json, created_at, updated_at) VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?)`, id, input.Name, input.Slug, input.Hostname, input.ProjectRoot, input.WebRoot, input.Framework, input.Strategy, input.Status, runtime, now, now)
	if err != nil {
		return Site{}, fmt.Errorf("create site: %w", err)
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
	result, err := s.db.ExecContext(ctx, `UPDATE sites SET name = ?, slug = ?, hostname = NULLIF(?, ''), project_root = ?, web_root = NULLIF(?, ''), framework = ?, strategy = ?, status = ?, runtime_json = NULLIF(?, ''), updated_at = ? WHERE id = ? AND archived_at IS NULL`, input.Name, input.Slug, input.Hostname, input.ProjectRoot, input.WebRoot, input.Framework, input.Strategy, input.Status, runtime, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return Site{}, fmt.Errorf("update site: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Site{}, ErrNotFound
	}
	return s.Get(ctx, id)
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
