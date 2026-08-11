CREATE TABLE IF NOT EXISTS administrators (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_login_at TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    administrator_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT,
    FOREIGN KEY(administrator_id) REFERENCES administrators(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    hostname TEXT,
    project_root TEXT NOT NULL,
    web_root TEXT,
    framework TEXT NOT NULL DEFAULT 'unknown',
    strategy TEXT NOT NULL DEFAULT 'in_place',
    status TEXT NOT NULL DEFAULT 'active',
    runtime_json TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    archived_at TEXT
);

CREATE TABLE IF NOT EXISTS repositories (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL,
    clone_url TEXT NOT NULL,
    branch TEXT NOT NULL,
    credential_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS credentials (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    encrypted_payload BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS deployment_configs (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    source TEXT NOT NULL,
    content TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    approved INTEGER NOT NULL DEFAULT 0,
    approved_at TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    deployment_config_id TEXT,
    trigger_type TEXT NOT NULL,
    trigger_reference TEXT,
    branch TEXT,
    commit_sha TEXT,
    status TEXT NOT NULL,
    error_code TEXT,
    error_summary TEXT,
    queued_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT,
    duration_ms INTEGER,
    created_at TEXT NOT NULL,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS deployment_steps (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    step_key TEXT NOT NULL,
    name TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL,
    exit_code INTEGER,
    started_at TEXT,
    finished_at TEXT,
    duration_ms INTEGER,
    log_path TEXT,
    FOREIGN KEY(deployment_id) REFERENCES deployments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS releases (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    deployment_id TEXT NOT NULL,
    release_name TEXT NOT NULL,
    release_path TEXT NOT NULL,
    commit_sha TEXT,
    active INTEGER NOT NULL DEFAULT 0,
    health_status TEXT,
    created_at TEXT NOT NULL,
    activated_at TEXT,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    public_token_hash TEXT NOT NULL,
    encrypted_secret BLOB NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    last_delivery_at TEXT,
    FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    provider_delivery_id TEXT,
    event TEXT,
    signature_valid INTEGER NOT NULL,
    payload_sha256 TEXT NOT NULL,
    received_at TEXT NOT NULL,
    deployment_id TEXT,
    FOREIGN KEY(webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE,
    UNIQUE(webhook_id, provider_delivery_id)
);

CREATE TABLE IF NOT EXISTS deployment_locks (
    site_id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    acquired_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    action TEXT NOT NULL,
    entity_type TEXT,
    entity_id TEXT,
    metadata_json TEXT,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS license_state (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    license_id TEXT,
    edition TEXT,
    nas_limit INTEGER,
    site_limit INTEGER NOT NULL DEFAULT 5,
    expires_at TEXT,
    signed_payload TEXT,
    signature TEXT,
    refreshed_at TEXT
);
