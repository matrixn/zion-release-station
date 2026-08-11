# Zion ReleaseStation v1

## Professional MVP — Master Technical Specification for Codex

**Product:** Zion ReleaseStation
**Package ID:** `zion-releasestation`
**Repository:** `zion-releasestation`
**Binary:** `zion-releasestation`
**DSM package user:** `zionrelease`
**Primary target:** Synology DSM 7.2.2+, initially tested on DS1019+
**Architecture:** `x86_64`, build/test platform `apollolake`

---

# 0. INSTRUCTIONS FOR CODEX

Treat this document as the authoritative implementation specification.

The objective is not to create a proof of concept. Build a production-oriented Professional MVP with:

* maintainable architecture;
* strong security boundaries;
* clean package lifecycle;
* production-quality Go;
* production-quality Vue/TypeScript;
* database migrations;
* tests;
* structured logging;
* automatic build;
* automatic SPK packaging;
* automatic deployment to development Synology;
* manual deployment from VS Code Tasks;
* exceptional UI/UX.

Do not replace required functionality with placeholders unless explicitly marked as an extension point.

Do not implement shortcuts that create long-term architectural debt.

Do not:

* run the application as root;
* execute HTTP input directly in a shell;
* disable SSH host verification;
* store plaintext credentials;
* put NAS passwords in Git;
* allow arbitrary repository-controlled shell code without explicit administrator approval;
* modify DSM internal configuration files directly when an official DSM mechanism exists;
* depend on PHP, Laravel, Node or Docker for ReleaseStation runtime;
* require Internet access to start the package;
* bundle development tooling in the final SPK;
* build a generic Bootstrap/admin-template UI.

The final runtime application must primarily be:

```text
Go binary
+
SQLite database
+
static Vue frontend
+
DSM/SPK integration
```

---

# 1. PRODUCT DEFINITION

Zion ReleaseStation is a self-hosted Git deployment and release-management application built specifically for Synology DSM.

Primary workflow:

```text
GitHub / GitLab / Git repository
             │
             ▼
       ReleaseStation
             │
       deployment queue
             │
       secure executor
             │
             ▼
      Synology filesystem
             │
             ▼
       Web Station site
```

Main differentiators:

1. native Synology DSM package;
2. Web Station site discovery;
3. Git-based deployments;
4. automatic deployments from webhooks;
5. deployment pipelines;
6. live deployment logs;
7. rollback;
8. Laravel / WordPress / Symfony / generic PHP detection;
9. atomic deployment where supported;
10. safe in-place deployment for existing Web Station sites;
11. premium DSM-native UX.

---

# 2. PROFESSIONAL MVP SCOPE

The v1 MVP MUST include:

### Core

* first-run setup;
* authentication;
* dashboard;
* site management;
* Git repository configuration;
* SSH deploy keys;
* branch selection;
* Git repository connectivity test;
* Web Station discovery abstraction;
* application/framework detection;
* manual deployment;
* webhook deployment;
* deployment queue;
* persistent deployment history;
* structured pipeline;
* live logs;
* configurable deployment steps;
* `deployment.yml`;
* secrets/environment management;
* in-place deployment;
* atomic deployment;
* release retention;
* rollback;
* HTTP health checks;
* per-site deployment lock;
* automatic rollback after failed post-activation health check;
* audit log.

### Developer tooling

* Go tests;
* frontend tests;
* package validation;
* reproducible SPK build;
* `make spk`;
* `make deploy-nas`;
* VS Code Tasks;
* GitHub Actions build;
* release artifact generation.

### Licensing-ready

Architecture must support:

```text
1 Pro license
1 NAS activation
5 included managed sites
+
capacity packs
```

Do not tightly couple licensing logic to UI or site repositories.

Implement a `LicenseProvider` abstraction.

---

# 3. NON-GOALS FOR V1

Do NOT implement in v1:

* DNS management;
* Let's Encrypt issuance;
* DSM firewall management;
* database provisioning;
* MariaDB provisioning;
* Docker orchestration;
* Kubernetes;
* generic server provisioning;
* email server;
* SSH shell exposed to browser;
* remote interactive terminal;
* multi-user RBAC beyond administrator-level application authentication;
* automatic database migration rollback;
* management of Web Station configuration until read-only discovery is proven stable.

---

# 4. SYNOLOGY TARGET

Target:

```text
DSM >= 7.2.2
DS1019+
CPU family: x86_64
platform: apollolake
```

Synology officially maps `apollolake` into the `x86_64` architecture family.

The SPK should therefore publish:

```text
arch="x86_64"
```

while the initial Synology Toolkit environment should use:

```text
apollolake
```

The application backend must build:

```bash
CGO_ENABLED=0
GOOS=linux
GOARCH=amd64
```

This allows the final application runtime to remain independent of NAS system libraries as far as practicable.

---

# 5. REQUIRED DSM SECURITY MODEL

DSM 7 requires packages to explicitly use the reduced-privilege package model. `conf/privilege` must use `run-as: package`; privileged work should use DSM resource mechanisms rather than a root daemon.

ReleaseStation MUST NOT run as root.

Use:

```json
{
  "defaults": {
    "run-as": "package"
  },
  "username": "zionrelease",
  "join-groupname": "http"
}
```

The application must check filesystem permissions before accepting a project.

Example:

```text
Web root
/volume1/www/example

zionrelease:
READ  ✓
WRITE ✕

Deployment unavailable.

[ Show permission instructions ]
```

Never silently attempt privileged ACL modifications.

---

# 6. HIGH-LEVEL ARCHITECTURE

```text
                      Browser / DSM
                           │
                           │ HTTPS
                           ▼
                    DSM Nginx Proxy
                           │
                    /releasestation/
                           │
                           ▼
                  127.0.0.1:24871
                           │
             ┌─────────────┴─────────────┐
             │                           │
             ▼                           ▼
         Vue SPA                      Go API
                                         │
                    ┌────────────────────┼────────────────────┐
                    │                    │                    │
                    ▼                    ▼                    ▼
                 SQLite             Job Queue          Deployment Engine
                                                              │
                                         ┌────────────────────┼──────────┐
                                         │                    │          │
                                         ▼                    ▼          ▼
                                        Git               Executor    Releases
                                         │
                                         ▼
                                  GitHub/GitLab/etc.
```

Use a single Go process.

Do NOT create independent microservices for v1.

---

# 7. TECHNOLOGY STACK

## Backend

Use:

```text
Go
net/http or chi
SQLite
pure-Go SQLite driver
structured slog logging
crypto/rand
crypto/aes
crypto/ed25519
golang.org/x/crypto
```

Recommended router:

```text
github.com/go-chi/chi/v5
```

Recommended SQLite strategy:

```text
modernc.org/sqlite
```

or another mature pure-Go driver.

Avoid CGO unless a strong justification is documented.

---

# 8. FRONTEND STACK

Use:

```text
Vue 3
TypeScript
Vite
Vue Router
Pinia
Tailwind CSS 4
PrimeVue 4 Unstyled OR Volt
VueUse
@vueuse/motion
Lucide Vue
Monaco Editor
@xterm/xterm
SortableJS or Vue-compatible Sortable wrapper
```

Heavy modules such as:

```text
Monaco
xterm
```

MUST be lazy-loaded.

---

# 9. MONOREPO STRUCTURE

Use:

```text
zion-releasestation/
│
├── cmd/
│   └── releasestation/
│       └── main.go
│
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   ├── responses/
│   │   └── routes.go
│   │
│   ├── auth/
│   ├── audit/
│   ├── config/
│   ├── crypto/
│   ├── database/
│   │   ├── migrations/
│   │   └── repository/
│   │
│   ├── deploy/
│   │   ├── engine.go
│   │   ├── planner.go
│   │   ├── worker.go
│   │   ├── executor.go
│   │   ├── atomic.go
│   │   ├── inplace.go
│   │   ├── rollback.go
│   │   ├── health.go
│   │   └── retention.go
│   │
│   ├── git/
│   │   ├── client.go
│   │   ├── ssh.go
│   │   └── knownhosts.go
│   │
│   ├── licensing/
│   ├── projects/
│   ├── releases/
│   ├── secrets/
│   ├── system/
│   ├── webhooks/
│   └── webstation/
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── assets/
│   │   ├── components/
│   │   ├── composables/
│   │   ├── layouts/
│   │   ├── router/
│   │   ├── stores/
│   │   ├── types/
│   │   ├── views/
│   │   └── App.vue
│   └── package.json
│
├── synology/
│   ├── INFO.sh
│   ├── PACKAGE_ICON.PNG
│   ├── PACKAGE_ICON_256.PNG
│   │
│   ├── conf/
│   │   ├── privilege
│   │   └── resource
│   │
│   ├── scripts/
│   │   ├── preinst
│   │   ├── postinst
│   │   ├── preupgrade
│   │   ├── postupgrade
│   │   ├── preuninst
│   │   ├── postuninst
│   │   └── start-stop-status
│   │
│   ├── WIZARD_UIFILES/
│   │   └── install_uifile
│   │
│   ├── ui/
│   │   ├── config
│   │   └── images/
│   │
│   ├── port_conf/
│   │   └── zion-releasestation.sc
│   │
│   └── nginx/
│       └── releasestation.conf
│
├── build/
│   └── synology/
│       ├── Dockerfile
│       └── docker-compose.yml
│
├── scripts/
│   ├── build-frontend.sh
│   ├── build-backend.sh
│   ├── build-spk.sh
│   ├── validate-spk.sh
│   ├── deploy-nas.sh
│   ├── nas-logs.sh
│   └── nas-health.sh
│
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
│
├── .vscode/
│   ├── tasks.json
│   ├── settings.json
│   └── extensions.json
│
├── deployment.example.yml
├── Makefile
├── go.mod
├── README.md
├── SECURITY.md
├── CONTRIBUTING.md
└── SPEC.md
```

---

# 10. APPLICATION DATA DIRECTORIES

Never place mutable data in the package target directory.

Use:

```text
/var/packages/zion-releasestation/var/
```

for application state.

Expected runtime structure:

```text
var/
├── releasestation.db
├── master.key
├── git/
│   ├── keys/
│   └── known_hosts
│
├── logs/
│   └── deployments/
│
├── locks/
├── cache/
└── runtime/
```

Permissions:

```text
master.key      0600
SSH private keys 0600
database        0600
directories     0700 where possible
```

---

# 11. SQLITE CONFIGURATION

On every connection initialize:

```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
```

All schema changes MUST use versioned migrations.

Never modify schema ad-hoc at startup.

---

# 12. DATABASE SCHEMA

## administrators

```sql
CREATE TABLE administrators (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_login_at TEXT
);
```

Passwords MUST be Argon2id hashed.

---

## sessions

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    administrator_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT,
    FOREIGN KEY(administrator_id)
        REFERENCES administrators(id)
        ON DELETE CASCADE
);
```

Do NOT store raw session tokens.

---

## settings

```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

---

## sites

```sql
CREATE TABLE sites (
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
```

Valid strategies:

```text
in_place
atomic
```

---

## repositories

```sql
CREATE TABLE repositories (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL UNIQUE,
    provider TEXT NOT NULL,
    clone_url TEXT NOT NULL,
    branch TEXT NOT NULL,
    credential_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY(site_id)
        REFERENCES sites(id)
        ON DELETE CASCADE
);
```

Providers:

```text
github
gitlab
bitbucket
generic
```

---

## credentials

```sql
CREATE TABLE credentials (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    encrypted_payload BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

Kinds:

```text
ssh_private_key
personal_access_token
webhook_secret
environment_secret
```

---

## deployment_configs

```sql
CREATE TABLE deployment_configs (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    source TEXT NOT NULL,
    content TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    approved INTEGER NOT NULL DEFAULT 0,
    approved_at TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY(site_id)
        REFERENCES sites(id)
        ON DELETE CASCADE
);
```

Sources:

```text
ui
repository
generated
```

---

## deployments

```sql
CREATE TABLE deployments (
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
    FOREIGN KEY(site_id)
        REFERENCES sites(id)
        ON DELETE CASCADE
);
```

Statuses:

```text
queued
preparing
running
activating
health_check
success
failed
rolled_back
cancelled
interrupted
```

---

## deployment_steps

```sql
CREATE TABLE deployment_steps (
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
    FOREIGN KEY(deployment_id)
        REFERENCES deployments(id)
        ON DELETE CASCADE
);
```

---

## releases

```sql
CREATE TABLE releases (
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
    FOREIGN KEY(site_id)
        REFERENCES sites(id)
        ON DELETE CASCADE
);
```

---

## webhooks

```sql
CREATE TABLE webhooks (
    id TEXT PRIMARY KEY,
    site_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    public_token_hash TEXT NOT NULL,
    encrypted_secret BLOB NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    last_delivery_at TEXT,
    FOREIGN KEY(site_id)
        REFERENCES sites(id)
        ON DELETE CASCADE
);
```

---

## webhook_deliveries

```sql
CREATE TABLE webhook_deliveries (
    id TEXT PRIMARY KEY,
    webhook_id TEXT NOT NULL,
    provider_delivery_id TEXT,
    event TEXT,
    signature_valid INTEGER NOT NULL,
    payload_sha256 TEXT NOT NULL,
    received_at TEXT NOT NULL,
    deployment_id TEXT,
    FOREIGN KEY(webhook_id)
        REFERENCES webhooks(id)
        ON DELETE CASCADE
);
```

Create uniqueness protection for provider delivery IDs where available.

---

## deployment_locks

```sql
CREATE TABLE deployment_locks (
    site_id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    acquired_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
```

Only one deployment per site may execute simultaneously.

---

## audit_logs

```sql
CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    action TEXT NOT NULL,
    entity_type TEXT,
    entity_id TEXT,
    metadata_json TEXT,
    created_at TEXT NOT NULL
);
```

---

## license_state

```sql
CREATE TABLE license_state (
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
```

Enforce `site_limit` in backend APIs, never only in Vue.

---

# 13. SECRETS

Generate at first installation:

```text
master.key
```

Use cryptographically secure random bytes.

Encrypt stored secrets with:

```text
AES-256-GCM
```

Store:

```text
nonce
ciphertext
auth tag
```

Never return existing secret plaintext via API.

API behavior:

```text
SSH_PRIVATE_KEY
••••••••••••••

[ Replace ]
```

Never:

```text
GET secret → plaintext
```

---

# 14. REST API CONVENTIONS

Base:

```text
/releasestation/api/v1
```

Success:

```json
{
  "data": {}
}
```

Failure:

```json
{
  "error": {
    "code": "SITE_NOT_WRITABLE",
    "message": "ReleaseStation cannot write to this project directory.",
    "details": {}
  }
}
```

---

# 15. AUTH API

```text
POST /auth/login
POST /auth/logout
GET  /auth/me
POST /auth/change-password
```

Cookie:

```text
HttpOnly
Secure
SameSite=Strict
Path=/releasestation/
```

Implement CSRF protection for state-changing requests.

Use Origin validation.

Rate-limit authentication attempts.

---

# 16. SYSTEM API

```text
GET /system/health
GET /system/info
GET /system/capabilities
GET /system/tools
POST /system/tools/scan
```

Example:

```json
{
  "data": {
    "status": "healthy",
    "version": "0.1.0",
    "platform": "synology",
    "architecture": "x86_64",
    "webstation": true,
    "git": true
  }
}
```

---

# 17. SITE API

```text
GET    /sites
POST   /sites
GET    /sites/{id}
PATCH  /sites/{id}
DELETE /sites/{id}

POST /sites/{id}/validate-path
POST /sites/{id}/detect-framework
POST /sites/{id}/scan-tools
```

DELETE should archive by default.

Permanent removal must require confirmation.

---

# 18. WEB STATION API

```text
GET  /webstation/status
POST /webstation/discover
POST /webstation/import
```

Build:

```go
type WebStationAdapter interface {
    Available(context.Context) (bool, error)
    Discover(context.Context) ([]DiscoveredSite, error)
}
```

Do not couple Web Station implementation directly to API handlers.

The Web Station UI exposes web services and their document roots; Synology documentation explicitly treats the Document Root as part of Web Service configuration.

Implement read-only discovery first.

Never modify Web Station configuration in MVP without an explicitly supported and tested adapter.

---

# 19. GIT API

```text
POST /git/test
POST /git/generate-deploy-key
GET  /git/deploy-key/{siteId}
POST /git/test-ssh
GET  /git/branches/{siteId}
```

Generated key:

```text
Ed25519
```

Never expose the private key.

---

# 20. DEPLOYMENT API

```text
POST /sites/{id}/deployments

GET /deployments
GET /deployments/{id}

POST /deployments/{id}/cancel
POST /deployments/{id}/retry

GET /deployments/{id}/steps

GET /deployments/{id}/logs
GET /deployments/{id}/logs/stream
```

Use Server-Sent Events for real-time logs:

```text
Content-Type: text/event-stream
```

This is enough for xterm.js because deployment logging is one-directional.

Do NOT expose an interactive shell.

---

# 21. RELEASE API

```text
GET  /sites/{id}/releases
POST /sites/{id}/releases/{releaseId}/rollback
```

Rollback MUST require explicit confirmation.

---

# 22. WEBHOOK API

```text
POST /webhooks/github/{token}
POST /webhooks/gitlab/{token}
```

GitHub webhook requirements:

* HMAC verification;
* reject invalid signature;
* check repository;
* check configured branch;
* prevent replay using delivery ID;
* payload size limit;
* never log webhook secrets.

---

# 23. deployment.yml

Schema identifier:

```yaml
schema: zion.releasestation/v1
```

Example:

```yaml
schema: zion.releasestation/v1

project:
  type: laravel
  public_dir: public

git:
  branch: main

deployment:
  strategy: atomic
  retain_releases: 5
  pending_policy: latest

shared:
  files:
    - .env

  directories:
    - storage

pipeline:
  - id: composer
    name: Install Composer dependencies
    type: exec
    command: composer
    args:
      - install
      - --no-dev
      - --no-interaction
      - --prefer-dist
      - --optimize-autoloader

  - id: frontend-install
    name: Install frontend dependencies
    type: exec
    command: npm
    args:
      - ci

  - id: frontend-build
    name: Build frontend
    type: exec
    command: npm
    args:
      - run
      - build

  - id: migrate
    name: Run migrations
    type: exec
    command: php
    args:
      - artisan
      - migrate
      - --force

  - id: optimize
    name: Optimize Laravel
    type: exec
    command: php
    args:
      - artisan
      - optimize

health:
  url: https://example.com/up
  expected_status:
    - 200

  timeout: 15s
  retries: 5
  retry_delay: 2s
```

---

# 24. deployment.yml SECURITY

Treat `deployment.yml` from Git as **untrusted code-like configuration**.

Repository changes to:

```text
deployment.yml
```

must NOT silently alter an automatic production deployment pipeline.

Workflow:

```text
new deployment.yml hash
        ↓
different from approved hash
        ↓
deployment paused
        ↓
Configuration changed
        ↓
show diff
        ↓
administrator approval
        ↓
new hash becomes approved
```

Auto-deployment MUST continue using the last approved configuration until the new version is approved.

---

# 25. EXECUTOR SECURITY

Default deployment steps MUST use direct process execution:

```go
exec.CommandContext(...)
```

not:

```text
sh -c
eval
system()
```

Pipeline model:

```text
command
+
arguments[]
+
working directory
+
sanitized environment
```

Example:

```text
command: composer
args:
  - install
  - --no-dev
```

NOT:

```text
composer install --no-dev && rm -rf ...
```

---

# 26. TOOL RESOLUTION

Create a ToolRegistry.

Example:

```text
php
composer
node
npm
git
```

Configured mappings:

```text
PHP 8.4
→ /var/packages/PHP8.4/target/usr/local/bin/php84
```

Never trust repository YAML to provide arbitrary absolute executable paths.

The YAML specifies:

```text
command: php
```

ReleaseStation resolves it through the approved ToolRegistry.

---

# 27. PATH SECURITY

Before accessing any project path:

1. canonicalize path;
2. resolve symlinks;
3. verify it lies inside an administrator-approved project root;
4. reject traversal;
5. reject unexpected filesystem escapes.

Explicitly test:

```text
../../etc
/volume1/www/site/../../../etc
symlink → /etc
```

All must fail.

---

# 28. EXECUTION LIMITS

Each step supports:

```text
timeout
max_output
```

Defaults:

```text
command timeout: 15 minutes
deployment timeout: 45 minutes
```

Kill process groups when deployment is cancelled.

Mark interrupted deployments after daemon restart:

```text
running
↓ restart
interrupted
```

Do not automatically resume arbitrary commands.

---

# 29. DEPLOYMENT QUEUE

Persist queue in SQLite.

Per-site concurrency:

```text
1
```

Default global concurrency:

```text
2
```

Pending policy:

```text
latest
```

Example:

```text
currently deploying:
commit A

incoming:
B
C
D
```

Keep:

```text
D
```

if B/C have not started.

---

# 30. DEPLOYMENT STRATEGIES

## In-place

Default for imported existing Web Station sites.

Flow:

```text
git fetch
git checkout/reset
dependencies
build
migration
optimize
health check
```

Before destructive Git reset, validate clean deployment policy.

---

# 31. ATOMIC DEPLOYMENT

For ReleaseStation-managed projects:

```text
project/
├── current -> releases/20260811-234423/
│
├── releases/
│   ├── 20260811-234423/
│   └── 20260810-201102/
│
└── shared/
    ├── .env
    └── storage/
```

Flow:

```text
prepare release
      ↓
checkout
      ↓
dependencies
      ↓
build
      ↓
shared links
      ↓
pre-activation validation
      ↓
atomic symlink switch
      ↓
HTTP health check
      ↓
success
```

Switch implementation:

```text
create current.next symlink
↓
atomic rename current.next → current
```

Do not replace the live symlink through multiple non-atomic operations.

---

# 32. AUTOMATIC ROLLBACK

After activation:

```text
new release
   ↓
health check
   ↓
FAIL
   ↓
switch current to previous release
   ↓
mark deployment rolled_back
```

Important:

filesystem rollback DOES NOT automatically rollback database migrations.

UI must explicitly state:

```text
Application files were rolled back.
Database migrations were not reversed.
```

---

# 33. RELEASE RETENTION

Default:

```text
5 releases
```

Never delete:

* current release;
* immediately previous release while deployment is running;
* release involved in active rollback.

Cleanup happens only after successful deployment.

---

# 34. FRAMEWORK DETECTION

Implement detectors behind interface:

```go
type FrameworkDetector interface {
    Detect(root string) (DetectionResult, error)
}
```

### Laravel

Detect:

```text
artisan
composer.json
public/index.php
```

### Symfony

Detect:

```text
bin/console
composer.json
public/index.php
```

### WordPress

Detect:

```text
wp-config.php
wp-admin/
wp-content/
```

### Flarum

Detect:

```text
flarum
composer.json
public/index.php
```

### Generic Node

Detect:

```text
package.json
```

### Generic PHP

Detect:

```text
composer.json
```

---

# 35. PROFESSIONAL UI/UX — CRITICAL REQUIREMENT

The ReleaseStation UI is a primary product feature.

It must be visually impressive enough that a user opening it for the first time feels that this is a premium professional DevOps application.

Do NOT create:

```text
generic admin panel
Bootstrap dashboard
PrimeVue default theme
20 identical cards
giant whitespace
boring CRUD tables
```

Visual direction:

```text
Linear
+
Vercel
+
GitHub Actions
+
Raycast
+
modern DSM
```

It should feel unique to Zion ReleaseStation.

---

# 36. DESIGN SYSTEM

Use:

```text
PrimeVue Unstyled / Volt
+
Tailwind CSS 4
```

Build a custom ReleaseStation design system.

Create tokens:

```text
--rs-bg
--rs-surface
--rs-surface-elevated
--rs-border
--rs-text
--rs-text-muted

--rs-success
--rs-warning
--rs-danger
--rs-info
--rs-accent
```

Support:

```text
dark
light
system
```

Dark mode should be the hero experience.

---

# 37. UI PERFORMANCE

Transitions:

```text
120–220 ms
```

Use motion purposefully.

Examples:

* pipeline steps transition;
* deployment status pulse;
* drawers;
* command palette;
* release activation;
* success check animation.

Do not animate everything.

Respect:

```text
prefers-reduced-motion
```

Target smooth 60fps interactions.

---

# 38. APP SHELL

Desktop:

```text
┌───────────────┬──────────────────────────────────────────────┐
│               │                                              │
│ ReleaseStation│       Project / Deployment                   │
│               │                                              │
│ Dashboard     │                                              │
│ Sites         │                                              │
│ Deployments   │                                              │
│ Releases      │                                              │
│ Activity      │                                              │
│               │                                              │
│ Settings      │                                              │
│               │                                              │
└───────────────┴──────────────────────────────────────────────┘
```

Sidebar collapsible.

Show active site selector near top.

---

# 39. DASHBOARD

Hero section:

```text
Good evening

4 sites managed
3 healthy
1 deployment running
```

Live status:

```text
● Web Station
● Git
● ReleaseStation worker
● SQLite
```

Recent deployments:

```text
servazar.ro

main
a9f72cd

Deploying

████████████████░░░

Build frontend
17.3 seconds
```

---

# 40. SITE CARDS

Not generic rectangular cards.

Each site should show:

```text
favicon / framework icon

servazar.ro
Laravel

● Healthy

main
a93fd72

Last deployment
4 minutes ago

[ Deploy ]
```

Hover reveals:

```text
Open
Deploy
Settings
```

---

# 41. WEB STATION DISCOVERY UX

First run:

```text
Welcome to Zion ReleaseStation

Web Station detected.

[ Discover hosted applications ]
```

Scanning animation should show phases:

```text
Scanning Web Station
       ↓
Resolving document roots
       ↓
Detecting frameworks
       ↓
Checking Git repositories
```

Result:

```text
✓ servazar.ro
  Laravel
  /volume1/www/servazar.ro

✓ zion3d.ro
  WordPress
  /volume1/web/zion3d.ro

✓ support.zion3d.ro
  Flarum
  /volume1/www/support.zion3d.ro
```

Multi-select import.

Use PrimeVue MultiSelect/checkbox patterns where useful.

---

# 42. ADD PROJECT WIZARD

Steps:

```text
1 Site
2 Repository
3 Runtime
4 Deployment
5 Review
```

Step 1 can choose:

```text
Import from Web Station
Manual configuration
```

Framework automatically detected.

---

# 43. DEPLOYMENT PIPELINE UI

This is a signature feature.

Example:

```text
Deployment #184

✓ Checkout              1.1s
│
✓ Composer              7.4s
│
✓ NPM Install           8.8s
│
◉ Build                 running 5.3s
│
○ Migrations
│
○ Activate
│
○ Health Check
```

Each step expands:

```text
▶ Build
```

to reveal:

```text
command
duration
exit code
logs
```

---

# 44. LIVE TERMINAL EXPERIENCE

Use:

```text
xterm.js
+
SSE
```

Display:

```text
┌ Deployment #184 ──────────────────────────────────────────┐
│ $ composer install --no-dev                               │
│ Installing dependencies...                                │
│                                                          │
│ ✓ 124 packages installed                                 │
│                                                          │
│ $ npm run build                                           │
│ vite building for production...                           │
│ ✓ built in 4.31s                                          │
└───────────────────────────────────────────────────────────┘
```

Features:

```text
follow output
pause scrolling
search
copy
clear viewport
download log
```

NO terminal input.

NO remote shell.

---

# 45. MONACO EDITOR

Use Monaco for:

```text
deployment.yml
custom local deployment script
environment templates
config diff
```

Features:

```text
syntax highlighting
line numbers
search
diff
validation diagnostics
Ctrl+S
```

deployment.yml validation errors should appear directly in editor.

---

# 46. PIPELINE BUILDER

Visual builder:

```text
☰ Git Checkout
      ↓
☰ Composer Install
      ↓
☰ NPM Build
      ↓
☰ Tests
      ↓
☰ Migration
      ↓
☰ Health Check
```

Use drag & drop.

Each item opens a Drawer.

Example:

```text
Build frontend

Type
Exec

Command
npm

Arguments
[ run ] [ build ]

Timeout
[ 10 min ]

[ Save ]
```

---

# 47. COMMAND PALETTE

Keyboard:

```text
Ctrl+K
```

UI:

```text
Search or run a command...

Sites
  servazar.ro
  zion3d.ro

Actions
  Deploy servazar.ro
  Rollback servazar.ro
  Add site
  Scan Web Station
  Open settings
```

All major navigation/action paths should be keyboard-accessible.

---

# 48. MULTI-SELECTS

Use high-quality multi-selection controls for:

```text
branches
notification events
shared directories
environment scopes
pipeline templates
Web Station imports
```

Selected items displayed as chips.

Search/filter when list can become large.

---

# 49. RELEASE HISTORY UX

Visual timeline:

```text
● Current
│
│  a9182fc
│  11 Aug 23:47
│  31.2 sec
│
● Previous
│
│  b716abe
│  11 Aug 21:16
│
●
```

Rollback action must clearly indicate destination commit.

---

# 50. FAILURE UX

Do not show:

```text
Error 500
```

Show:

```text
Deployment failed

Frontend build exited with code 1.

npm run build
              ↓

src/App.vue:48
Property 'foo' does not exist.

[ Open logs ]
[ Retry deployment ]
```

Technical details expandable.

---

# 51. EMPTY STATES

Every empty state should explain the next action.

Example:

```text
No sites yet

Import sites already hosted in Web Station
or connect a Git repository manually.

[ Discover Web Station ]
[ Add manually ]
```

---

# 52. ACCESSIBILITY

Target WCAG AA where practical.

Requirements:

* keyboard navigation;
* visible focus states;
* ARIA labels;
* status not represented only by color;
* good text contrast;
* reduced motion support.

---

# 53. DSM INTEGRATION

Synology supports application shortcuts through `dsmuidir`, including customizable icon, target URL and privileges.

Set:

```text
dsmuidir="ui"
```

DSM launcher should open:

```text
/releasestation/
```

Use ReleaseStation icon assets.

---

# 54. DSM WEB ACCESS

Daemon:

```text
127.0.0.1:24871
```

Do NOT bind by default to:

```text
0.0.0.0
```

Expose via DSM Nginx.

DSM 7.2's `web-config` resource officially supports Nginx static configurations for DSM blocks, including 5000/5001.

Use:

```text
type: dsm
```

Proxy:

```text
/releasestation/
```

to:

```text
http://127.0.0.1:24871/
```

---

# 55. SPK STRUCTURE

Synology's package structure requires `INFO`, `package.tgz`, lifecycle scripts, configuration files and package icons. For DSM 7+, `PACKAGE_ICON.PNG` is 64×64 and `PACKAGE_ICON_256.PNG` is 256×256.

Final:

```text
zion-releasestation.spk

├── INFO
├── package.tgz
├── scripts/
│   ├── preinst
│   ├── postinst
│   ├── preupgrade
│   ├── postupgrade
│   ├── preuninst
│   ├── postuninst
│   └── start-stop-status
│
├── conf/
│   ├── privilege
│   └── resource
│
├── WIZARD_UIFILES/
│   └── install_uifile
│
├── PACKAGE_ICON.PNG
├── PACKAGE_ICON_256.PNG
└── LICENSE
```

---

# 56. package.tgz

```text
package.tgz

├── bin/
│   └── zion-releasestation
│
├── web/
│   ├── index.html
│   └── assets/
│
├── ui/
│   ├── config
│   └── images/
│
├── nginx/
│   └── releasestation.conf
│
├── port_conf/
│   └── zion-releasestation.sc
│
└── migrations/
```

---

# 57. INFO.sh

Generate INFO rather than maintaining it manually.

Example target:

```bash
package="zion-releasestation"
version="${VERSION}-${BUILD_NUMBER}"

os_min_ver="7.2.2-72806"

displayname="Zion ReleaseStation"

description="Git deployment and release management for Synology DSM."

arch="x86_64"

maintainer="Zion"

dsmuidir="ui"

thirdparty="yes"

startstop_restart_services="nginx.service"
instuninst_restart_services="nginx.service"
```

Synology requires numeric version components and an increasing build number.

---

# 58. DEVELOPMENT BUILD NUMBER

Use:

```bash
UNIX_TIME=$(date +%s)
BUILD_NUMBER=$((UNIX_TIME - 1577836800))
```

This provides a monotonic numeric build value based on seconds since 2020.

Example:

```text
0.1.0-208483917
```

Must remain inside Synology's allowed integer range.

---

# 59. PORT REGISTRATION

Register:

```text
24871/tcp
```

as internal ReleaseStation daemon port.

Port config:

```ini
[zion_releasestation_api]
title="Zion ReleaseStation"
desc="Zion ReleaseStation backend service"
port_forward="no"
dst.ports="24871/tcp"
```

Synology expects service ports to be registered through its resource mechanisms.

---

# 60. INSTALLATION WIZARD

DSM 7.2.2 supports custom installation wizard values passed into lifecycle scripts.

Installation wizard should ask:

```text
ReleaseStation administrator

Username
[                   ]

Password
[                   ]

Confirm password
[                   ]
```

Validate password strength.

postinst invokes:

```text
zion-releasestation setup
```

and sends password through stdin/environment provided by DSM.

Store only Argon2id hash.

---

# 61. PACKAGE START / STOP

`start-stop-status` must:

### start

```text
start binary
write PID
wait briefly
health check
return success
```

### stop

```text
SIGTERM
wait
SIGKILL only after timeout
remove PID
```

### status

verify actual process, not only PID file.

Handle stale PID files.

---

# 62. SHUTDOWN BEHAVIOR

SIGTERM:

1. stop accepting new deployments;
2. mark active worker as shutting down;
3. cancel running command context;
4. wait bounded duration;
5. flush database;
6. exit.

---

# 63. BUILD ENVIRONMENT

Recommended laptop:

```text
Windows
└── WSL2 Ubuntu
    ├── Go
    ├── Node
    ├── npm/pnpm
    ├── Git
    ├── Make
    └── Docker
```

Synology Toolkit should run inside a controlled Dockerized build environment.

Synology's official Toolkit uses `EnvDeploy` and `PkgCreate.py`.

Initial toolkit target:

```bash
./EnvDeploy -v 7.2 -p apollolake
```

Packaging should eventually use:

```text
PkgCreate.py
```

rather than a hand-crafted tar-only implementation.

---

# 64. MAKEFILE TARGETS

Implement:

```text
make help
make frontend
make backend
make test
make lint
make spk
make spk-validate

make deploy-nas
make install-nas
make nas-status
make nas-restart
make nas-logs
make nas-health
```

`make spk` must be sufficient to produce:

```text
dist/*.spk
```

---

# 65. AUTOMATED DEVELOPMENT DEPLOYMENT TO DS1019+

Local config:

```text
.env.nas
```

Never commit it.

Example:

```bash
NAS_HOST=192.168.0.10
NAS_PORT=22
NAS_USER=matrixn

NAS_PACKAGE=zion-releasestation

NAS_REMOTE_SPK=/tmp/zion-releasestation-dev.spk

NAS_SUDO_MODE=interactive
```

Add:

```text
.env.nas
```

to `.gitignore`.

---

# 66. deploy-nas.sh

Process:

```text
validate .env.nas
        ↓
build SPK
        ↓
SCP SPK
        ↓
SSH
        ↓
synopkg install
        ↓
synopkg start if required
        ↓
health check
        ↓
show package logs on failure
```

DSM 7's `synopkg install` applies the same installation constraints as UI installation, so it is suitable for development automation.

Concept:

```bash
scp -P "$NAS_PORT" \
    "$SPK" \
    "$NAS_USER@$NAS_HOST:$NAS_REMOTE_SPK"

ssh -tt -p "$NAS_PORT" \
    "$NAS_USER@$NAS_HOST" \
    "sudo synopkg install '$NAS_REMOTE_SPK'"
```

Never embed sudo password.

---

# 67. PASSWORDLESS DEV DEPLOYMENT

Optional DEVELOPMENT-ONLY setup may permit passwordless execution of a tightly restricted package-management command.

Do NOT grant:

```text
NOPASSWD: ALL
```

If used, first determine actual Synology path:

```bash
command -v synopkg
```

Then restrict sudo permission to exact required executable(s).

Document this as development setup only.

---

# 68. VS CODE INTEGRATION

The recommended workflow is to open the repository inside WSL:

```bash
cd ~/workspace/zion-releasestation
code .
```

Then VS Code tasks execute directly in Linux.

---

# 69. .vscode/tasks.json

Create:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "ReleaseStation: Test All",
      "type": "shell",
      "command": "make test",
      "group": "test",
      "problemMatcher": [],
      "presentation": {
        "reveal": "always",
        "panel": "dedicated"
      }
    },
    {
      "label": "ReleaseStation: Build SPK",
      "type": "shell",
      "command": "make spk",
      "group": "build",
      "problemMatcher": [],
      "presentation": {
        "reveal": "always",
        "panel": "dedicated"
      }
    },
    {
      "label": "ReleaseStation: Deploy SPK to DS1019+",
      "type": "shell",
      "command": "make deploy-nas",
      "problemMatcher": [],
      "presentation": {
        "reveal": "always",
        "panel": "dedicated",
        "focus": true,
        "clear": true
      }
    },
    {
      "label": "ReleaseStation: NAS Status",
      "type": "shell",
      "command": "make nas-status",
      "problemMatcher": []
    },
    {
      "label": "ReleaseStation: Restart NAS Package",
      "type": "shell",
      "command": "make nas-restart",
      "problemMatcher": []
    },
    {
      "label": "ReleaseStation: NAS Logs",
      "type": "shell",
      "command": "make nas-logs",
      "problemMatcher": [],
      "presentation": {
        "reveal": "always",
        "panel": "dedicated",
        "focus": true
      }
    },
    {
      "label": "ReleaseStation: NAS Health Check",
      "type": "shell",
      "command": "make nas-health",
      "problemMatcher": []
    }
  ]
}
```

User experience:

```text
VS Code
↓
Ctrl+Shift+P
↓
Tasks: Run Task
↓
ReleaseStation: Deploy SPK to DS1019+
```

This must:

```text
build
package
upload
upgrade
restart
health check
```

without requiring manual Package Center upload.

---

# 70. GITHUB ACTIONS — CI

On every PR/push:

```text
Go fmt
Go vet
Go test
Frontend lint
TypeScript check
Frontend tests
Frontend build
SPK validation
```

Do not deploy automatically to the private NAS from GitHub-hosted runners.

---

# 71. GITHUB RELEASE WORKFLOW

Trigger:

```text
tag v*
```

Workflow:

```text
tests
↓
frontend build
↓
Go linux/amd64 build
↓
SPK package
↓
SHA256
↓
GitHub Release
```

Artifacts:

```text
zion-releasestation-1.0.0-XXXX-x86_64.spk
SHA256SUMS
```

---

# 72. LOGGING

Application logs:

```text
structured JSON
```

Fields:

```text
timestamp
level
component
deployment_id
site_id
message
error
```

Do not log:

```text
password
PAT
private key
webhook secret
APP_KEY
DB_PASSWORD
```

Deployment raw output lives in filesystem:

```text
var/logs/deployments/{deployment-id}.log
```

Not inside SQLite.

---

# 73. TESTING REQUIREMENTS

## Backend unit tests

Required for:

```text
deployment planner
path security
executor
crypto
webhook signatures
release switching
rollback
retention
framework detection
deployment YAML parser
```

---

# 74. SECURITY TESTS

Must explicitly test:

```text
path traversal
symlink escape
invalid webhook signature
webhook replay
shell injection
argument injection
secret redaction
repository deployment.yml approval
concurrent deploy
stale lock recovery
```

---

# 75. FRONTEND TESTING

Use:

```text
Vitest
Vue Test Utils
```

Critical component tests:

```text
pipeline
site wizard
deployment status
license capacity
Web Station import
configuration editor
```

---

# 76. E2E TESTS

Use Playwright for:

```text
login
add site
configure repository
manual deployment
failed deployment
rollback
Web Station import mock
```

---

# 77. POST-INSTALL SMOKE TEST

After `make deploy-nas` verify:

```text
package installed
package running
HTTP /system/health returns 200
SQLite migration complete
frontend route returns 200
```

On failure automatically print:

```text
/var/log/packages/zion-releasestation.log
/var/log/synopkg.log
```

DSM 7 stores package lifecycle/control logs under `/var/log/packages/[package].log` and package operations in `/var/log/synopkg.log`.

---

# 78. DEFINITION OF DONE — MILESTONE 1

Infrastructure skeleton is complete when:

```text
git clone
↓
make test
↓
make spk
↓
make deploy-nas
```

results in:

```text
DSM
└── Zion ReleaseStation
       ↓
     Open
       ↓
ReleaseStation UI
● Healthy
```

No manual SPK upload.

---

# 79. MILESTONE 2 — SITES + WEB STATION

Must deliver:

```text
site CRUD
Web Station status
site discovery adapter
framework detection
permission validation
professional import wizard
```

---

# 80. MILESTONE 3 — GIT

Must deliver:

```text
Ed25519 deploy key
SSH host verification
clone
fetch
branches
repository validation
credentials
```

---

# 81. MILESTONE 4 — DEPLOYMENT ENGINE

Must deliver:

```text
queue
locks
pipeline
executor
manual deploy
logs
SSE
deployment history
```

---

# 82. MILESTONE 5 — RELEASE MANAGEMENT

Must deliver:

```text
atomic releases
shared dirs
activation
health checks
rollback
retention
```

---

# 83. MILESTONE 6 — AUTOMATION

Must deliver:

```text
GitHub webhook
GitLab webhook
HMAC validation
pending-policy latest
automatic deployment
audit logs
```

---

# 84. MILESTONE 7 — LICENSING

Implement:

```text
LicenseProvider
signed entitlement
NAS activation
5 site capacity
additional site capacity
site-limit enforcement
license UI
```

Core deployment data must remain available if license expires.

License failure must never delete sites or releases.

---

# 85. RELEASE QUALITY GATE

Before tagging v1.0:

All must pass:

```text
install clean
start clean
stop clean
restart clean
upgrade clean
uninstall clean
offline installation
no orphan process
no stale package files
port registered
no root daemon
no AppArmor denies
no coredumps
security tests
```

These align with Synology's package publication review requirements, which explicitly include lower privileges, install/start/stop/upgrade/uninstall behavior, offline install, port registration, cleanup, AppArmor errors and coredumps.

---

# 86. PRODUCT PHILOSOPHY

ReleaseStation should make this workflow feel effortless:

```text
Existing Synology site
        ↓
Discover
        ↓
Connect Git
        ↓
Deploy
```

and later:

```text
git push origin main
        ↓
GitHub webhook
        ↓
ReleaseStation
        ↓
Pipeline
        ↓
Health check
        ↓
Release live
```

A professional user should understand the state of every deployment in seconds.

The interface should make complex deployment infrastructure visually comprehensible without hiding important technical details.

ReleaseStation should feel like a purpose-built Synology developer product — not a shell script with a web page wrapped around it.
