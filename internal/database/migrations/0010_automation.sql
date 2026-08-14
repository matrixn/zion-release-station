ALTER TABLE webhooks ADD COLUMN encrypted_public_token BLOB NOT NULL DEFAULT X'';
ALTER TABLE webhooks ADD COLUMN last_error TEXT;

CREATE INDEX IF NOT EXISTS idx_webhooks_provider_token
    ON webhooks(provider, public_token_hash);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_received
    ON webhook_deliveries(received_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_created
    ON audit_logs(created_at DESC);
