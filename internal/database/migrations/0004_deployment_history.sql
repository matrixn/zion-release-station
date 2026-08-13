ALTER TABLE deployments ADD COLUMN commit_message TEXT;
ALTER TABLE deployments ADD COLUMN commit_url TEXT;
ALTER TABLE deployments ADD COLUMN deployment_method TEXT NOT NULL DEFAULT 'manual';

CREATE TABLE IF NOT EXISTS deployment_logs (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    channel TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(deployment_id) REFERENCES deployments(id) ON DELETE CASCADE,
    UNIQUE(deployment_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_deployments_site_created ON deployments(site_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_site_commit ON deployments(site_id, commit_sha);
