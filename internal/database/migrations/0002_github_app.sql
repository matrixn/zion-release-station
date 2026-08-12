CREATE TABLE IF NOT EXISTS github_installations (
    id TEXT PRIMARY KEY,
    github_installation_id INTEGER NOT NULL UNIQUE,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL,
    repository_selection TEXT NOT NULL,
    permissions_json TEXT,
    suspended_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS github_setup_states (
    state_hash TEXT PRIMARY KEY,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

ALTER TABLE repositories ADD COLUMN github_installation_id TEXT;
ALTER TABLE repositories ADD COLUMN github_repository_id INTEGER;
ALTER TABLE repositories ADD COLUMN github_full_name TEXT;
ALTER TABLE repositories ADD COLUMN github_default_branch TEXT;

CREATE INDEX IF NOT EXISTS idx_repositories_github_installation ON repositories(github_installation_id);
