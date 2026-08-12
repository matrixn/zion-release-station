-- ReleaseStation no longer stores GitHub App credentials on the NAS.
-- The managed Zion Connector owns the public App and its private key.
DELETE FROM settings WHERE key = 'github_app_config';

-- Existing self-hosted installation records and repository bindings belong to
-- the removed local App configuration and must not be reused by managed mode.
DELETE FROM github_setup_states;
DELETE FROM github_installations;
UPDATE repositories
SET github_installation_id = NULL,
    github_repository_id = NULL,
    github_full_name = NULL,
    github_default_branch = NULL
WHERE github_installation_id IS NOT NULL
   OR github_repository_id IS NOT NULL
   OR github_full_name IS NOT NULL
   OR github_default_branch IS NOT NULL;
