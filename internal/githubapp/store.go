package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("GitHub installation not found")

type Store struct {
	db *sql.DB
}

type InstallationRecord struct {
	ID                   string            `json:"id"`
	GitHubInstallationID int64             `json:"github_installation_id"`
	AccountLogin         string            `json:"account_login"`
	AccountType          string            `json:"account_type"`
	RepositorySelection  string            `json:"repository_selection"`
	Permissions          map[string]string `json:"permissions,omitempty"`
	SuspendedAt          *string           `json:"suspended_at,omitempty"`
	CreatedAt            string            `json:"created_at"`
	UpdatedAt            string            `json:"updated_at"`
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context) ([]InstallationRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, github_installation_id, account_login, account_type, repository_selection, COALESCE(permissions_json, ''), suspended_at, created_at, updated_at FROM github_installations ORDER BY account_login COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list GitHub installations: %w", err)
	}
	defer rows.Close()
	var result []InstallationRecord
	for rows.Next() {
		var record InstallationRecord
		var permissions string
		if err := rows.Scan(&record.ID, &record.GitHubInstallationID, &record.AccountLogin, &record.AccountType, &record.RepositorySelection, &permissions, &record.SuspendedAt, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan GitHub installation: %w", err)
		}
		if permissions != "" {
			if err := json.Unmarshal([]byte(permissions), &record.Permissions); err != nil {
				return nil, fmt.Errorf("decode GitHub installation permissions: %w", err)
			}
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate GitHub installations: %w", err)
	}
	return result, nil
}

func (s *Store) Save(ctx context.Context, installation Installation) (InstallationRecord, error) {
	if installation.GitHubID <= 0 || installation.AccountLogin == "" {
		return InstallationRecord{}, fmt.Errorf("GitHub installation details are incomplete")
	}
	permissions, err := json.Marshal(installation.Permissions)
	if err != nil {
		return InstallationRecord{}, err
	}
	id, err := newID("ghinst_")
	if err != nil {
		return InstallationRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `INSERT INTO github_installations(id, github_installation_id, account_login, account_type, repository_selection, permissions_json, suspended_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(github_installation_id) DO UPDATE SET account_login = excluded.account_login, account_type = excluded.account_type, repository_selection = excluded.repository_selection, permissions_json = excluded.permissions_json, suspended_at = excluded.suspended_at, updated_at = excluded.updated_at`, id, installation.GitHubID, installation.AccountLogin, installation.AccountType, installation.RepositorySelection, permissions, installation.SuspendedAt, now, now)
	if err != nil {
		return InstallationRecord{}, fmt.Errorf("save GitHub installation: %w", err)
	}
	return s.GetByGitHubID(ctx, installation.GitHubID)
}

func (s *Store) GetByGitHubID(ctx context.Context, githubID int64) (InstallationRecord, error) {
	var record InstallationRecord
	var permissions string
	err := s.db.QueryRowContext(ctx, `SELECT id, github_installation_id, account_login, account_type, repository_selection, COALESCE(permissions_json, ''), suspended_at, created_at, updated_at FROM github_installations WHERE github_installation_id = ?`, githubID).Scan(&record.ID, &record.GitHubInstallationID, &record.AccountLogin, &record.AccountType, &record.RepositorySelection, &permissions, &record.SuspendedAt, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return InstallationRecord{}, ErrNotFound
	}
	if err != nil {
		return InstallationRecord{}, fmt.Errorf("get GitHub installation: %w", err)
	}
	if permissions != "" {
		if err := json.Unmarshal([]byte(permissions), &record.Permissions); err != nil {
			return InstallationRecord{}, fmt.Errorf("decode GitHub installation permissions: %w", err)
		}
	}
	return record, nil
}

func (s *Store) Delete(ctx context.Context, githubID int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM github_installations WHERE github_installation_id = ?`, githubID)
	if err != nil {
		return fmt.Errorf("delete GitHub installation: %w", err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateSetupState(ctx context.Context, state string, expiresAt time.Time) error {
	digest := sha256.Sum256([]byte(state))
	_, err := s.db.ExecContext(ctx, `INSERT INTO github_setup_states(state_hash, expires_at, created_at) VALUES (?, ?, datetime('now'))`, hex.EncodeToString(digest[:]), expiresAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ConsumeSetupState(ctx context.Context, state string) (bool, error) {
	digest := sha256.Sum256([]byte(state))
	result, err := s.db.ExecContext(ctx, `DELETE FROM github_setup_states WHERE state_hash = ? AND expires_at > ?`, hex.EncodeToString(digest[:]), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return false, err
	}
	count, _ := result.RowsAffected()
	return count == 1, nil
}

func (s *Store) CleanupSetupStates(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM github_setup_states WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func newID(prefix string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate GitHub installation id: %w", err)
	}
	return prefix + hex.EncodeToString(value[:]), nil
}
