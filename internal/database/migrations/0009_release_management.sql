ALTER TABLE sites ADD COLUMN health_check_url TEXT NOT NULL DEFAULT '';
ALTER TABLE sites ADD COLUMN shared_directories_json TEXT NOT NULL DEFAULT '[]';
