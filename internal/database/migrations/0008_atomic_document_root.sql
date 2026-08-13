-- Atomic releases keep their staging and release metadata inside the site
-- root, while Web Station serves the site root itself. Older builds used a
-- nested `current` document root; migrate only that generated layout and keep
-- administrator-selected custom document roots unchanged.
UPDATE sites
SET web_root = project_root,
    updated_at = datetime('now')
WHERE strategy = 'atomic'
  AND web_root = project_root || '/current';
