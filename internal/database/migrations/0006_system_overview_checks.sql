INSERT OR IGNORE INTO settings(key, value_json, updated_at)
VALUES ('system_overview_checks', '["php","composer","node","npm","git","rsync","unzip","tar","curl","mysql"]', datetime('now'));
