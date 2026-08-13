<?php

declare(strict_types=1);

namespace ZionConnector;

use PDO;
use RuntimeException;

final class Database
{
    private PDO $pdo;
    private bool $mysql;

    public function __construct(string $dsn, ?string $username = null, ?string $password = null)
    {
        if (!str_contains($dsn, ':')) {
            $dsn = 'sqlite:' . $dsn;
        }
        $this->mysql = str_starts_with($dsn, 'mysql:');
        if (str_starts_with($dsn, 'sqlite:')) {
            $path = substr($dsn, strlen('sqlite:'));
            $directory = dirname($path);
            if (!is_dir($directory) && !mkdir($directory, 0700, true) && !is_dir($directory)) {
                throw new RuntimeException('Unable to create connector data directory.');
            }
        }
        $this->pdo = new PDO($dsn, $username, $password, [
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
        ]);
        if (str_starts_with($dsn, 'sqlite:')) {
            $this->pdo->exec('PRAGMA foreign_keys = ON');
            $this->pdo->exec('PRAGMA busy_timeout = 5000');
        } else {
            $this->pdo->exec("SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci");
        }
    }

    public function migrate(): void
    {
        if ($this->mysql) {
            $this->migrateMariaDb();
            return;
        }
        $this->pdo->exec(<<<'SQL'
            CREATE TABLE IF NOT EXISTS connector_instances (
                id TEXT PRIMARY KEY,
                license_id TEXT NULL,
                credential_hash TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'active',
                return_host TEXT NOT NULL,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL,
                last_seen_at TEXT NULL
            );
            CREATE TABLE IF NOT EXISTS github_connect_sessions (
                id TEXT PRIMARY KEY,
                instance_id TEXT NOT NULL REFERENCES connector_instances(id) ON DELETE CASCADE,
                state_hash TEXT UNIQUE NOT NULL,
                return_url TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'pending',
                expires_at TEXT NOT NULL,
                github_installation_id INTEGER NULL,
                error_code TEXT NULL,
                created_at TEXT NOT NULL,
                consumed_at TEXT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_sessions_instance_status ON github_connect_sessions(instance_id, status);
            CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON github_connect_sessions(expires_at);
            CREATE TABLE IF NOT EXISTS connector_pairing_sessions (
                id TEXT PRIMARY KEY,
                instance_id TEXT NOT NULL,
                state_hash TEXT UNIQUE NOT NULL,
                return_url TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'pending',
                expires_at TEXT NOT NULL,
                github_installation_id INTEGER NULL,
                created_at TEXT NOT NULL,
                authorized_at TEXT NULL,
                completed_at TEXT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_pairing_instance_status ON connector_pairing_sessions(instance_id, status);
            CREATE INDEX IF NOT EXISTS idx_pairing_expires_at ON connector_pairing_sessions(expires_at);
            CREATE TABLE IF NOT EXISTS github_installations (
                id TEXT PRIMARY KEY,
                instance_id TEXT NOT NULL REFERENCES connector_instances(id) ON DELETE CASCADE,
                github_installation_id INTEGER NOT NULL UNIQUE,
                account_login TEXT NOT NULL,
                account_type TEXT NOT NULL,
                repository_selection TEXT NOT NULL,
                permissions_json TEXT NOT NULL,
                suspended_at TEXT NULL,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_installations_instance ON github_installations(instance_id);
            CREATE TABLE IF NOT EXISTS github_webhook_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                instance_id TEXT NOT NULL REFERENCES connector_instances(id) ON DELETE CASCADE,
                delivery_id TEXT NOT NULL UNIQUE,
                event_name TEXT NOT NULL,
                action TEXT NULL,
                github_installation_id INTEGER NULL,
                repository_full_name TEXT NULL,
                ref_name TEXT NULL,
                before_sha TEXT NULL,
                after_sha TEXT NULL,
                deleted INTEGER NOT NULL DEFAULT 0,
                payload_json TEXT NOT NULL,
                created_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_webhook_events_instance_id ON github_webhook_events(instance_id, id);
            CREATE TABLE IF NOT EXISTS debug_logs (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                request_id TEXT NOT NULL,
                direction TEXT NOT NULL,
                source TEXT NOT NULL,
                method TEXT NOT NULL,
                url TEXT NOT NULL,
                status INTEGER NOT NULL DEFAULT 0,
                request_headers_json TEXT NOT NULL,
                request_body TEXT NOT NULL,
                response_headers_json TEXT NOT NULL,
                response_body TEXT NOT NULL,
                duration_ms INTEGER NOT NULL DEFAULT 0,
                created_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS idx_debug_logs_created_at ON debug_logs(created_at DESC);
            CREATE INDEX IF NOT EXISTS idx_debug_logs_request_id ON debug_logs(request_id);
        SQL);
    }

    private function migrateMariaDb(): void
    {
        $this->pdo->exec(<<<'SQL'
            CREATE TABLE IF NOT EXISTS connector_instances (
                id VARCHAR(128) PRIMARY KEY,
                license_id VARCHAR(255) NULL,
                credential_hash CHAR(64) NOT NULL,
                status VARCHAR(32) NOT NULL DEFAULT 'active',
                return_host VARCHAR(255) NOT NULL,
                created_at VARCHAR(32) NOT NULL,
                updated_at VARCHAR(32) NOT NULL,
                last_seen_at VARCHAR(32) NULL
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
            CREATE TABLE IF NOT EXISTS github_connect_sessions (
                id VARCHAR(128) PRIMARY KEY,
                instance_id VARCHAR(128) NOT NULL,
                state_hash CHAR(64) UNIQUE NOT NULL,
                return_url VARCHAR(2048) NOT NULL,
                status VARCHAR(32) NOT NULL DEFAULT 'pending',
                expires_at VARCHAR(32) NOT NULL,
                github_installation_id BIGINT NULL,
                error_code VARCHAR(64) NULL,
                created_at VARCHAR(32) NOT NULL,
                consumed_at VARCHAR(32) NULL,
                CONSTRAINT fk_sessions_instance FOREIGN KEY (instance_id) REFERENCES connector_instances(id) ON DELETE CASCADE,
                INDEX idx_sessions_instance_status (instance_id, status),
                INDEX idx_sessions_expires_at (expires_at)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
            CREATE TABLE IF NOT EXISTS connector_pairing_sessions (
                id VARCHAR(128) PRIMARY KEY,
                instance_id VARCHAR(128) NOT NULL,
                state_hash CHAR(64) UNIQUE NOT NULL,
                return_url VARCHAR(2048) NOT NULL,
                status VARCHAR(32) NOT NULL DEFAULT 'pending',
                expires_at VARCHAR(32) NOT NULL,
                github_installation_id BIGINT NULL,
                created_at VARCHAR(32) NOT NULL,
                authorized_at VARCHAR(32) NULL,
                completed_at VARCHAR(32) NULL,
                INDEX idx_pairing_instance_status (instance_id, status),
                INDEX idx_pairing_expires_at (expires_at)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
            CREATE TABLE IF NOT EXISTS github_installations (
                id VARCHAR(128) PRIMARY KEY,
                instance_id VARCHAR(128) NOT NULL,
                github_installation_id BIGINT NOT NULL UNIQUE,
                account_login VARCHAR(255) NOT NULL,
                account_type VARCHAR(32) NOT NULL,
                repository_selection VARCHAR(32) NOT NULL,
                permissions_json LONGTEXT NOT NULL,
                suspended_at VARCHAR(32) NULL,
                created_at VARCHAR(32) NOT NULL,
                updated_at VARCHAR(32) NOT NULL,
                CONSTRAINT fk_installations_instance FOREIGN KEY (instance_id) REFERENCES connector_instances(id) ON DELETE CASCADE,
                INDEX idx_installations_instance (instance_id)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
            CREATE TABLE IF NOT EXISTS github_webhook_events (
                id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
                instance_id VARCHAR(128) NOT NULL,
                delivery_id VARCHAR(255) NOT NULL UNIQUE,
                event_name VARCHAR(128) NOT NULL,
                action VARCHAR(128) NULL,
                github_installation_id BIGINT NULL,
                repository_full_name VARCHAR(512) NULL,
                ref_name VARCHAR(512) NULL,
                before_sha CHAR(64) NULL,
                after_sha CHAR(64) NULL,
                deleted TINYINT(1) NOT NULL DEFAULT 0,
                payload_json LONGTEXT NOT NULL,
                created_at VARCHAR(32) NOT NULL,
                CONSTRAINT fk_webhook_events_instance FOREIGN KEY (instance_id) REFERENCES connector_instances(id) ON DELETE CASCADE,
                INDEX idx_webhook_events_instance_id (instance_id, id)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
            CREATE TABLE IF NOT EXISTS debug_logs (
                id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
                request_id VARCHAR(128) NOT NULL,
                direction VARCHAR(32) NOT NULL,
                source VARCHAR(64) NOT NULL,
                method VARCHAR(16) NOT NULL,
                url VARCHAR(2048) NOT NULL,
                status INT NOT NULL DEFAULT 0,
                request_headers_json LONGTEXT NOT NULL,
                request_body LONGTEXT NOT NULL,
                response_headers_json LONGTEXT NOT NULL,
                response_body LONGTEXT NOT NULL,
                duration_ms BIGINT NOT NULL DEFAULT 0,
                created_at VARCHAR(32) NOT NULL,
                INDEX idx_debug_logs_created_at (created_at),
                INDEX idx_debug_logs_request_id (request_id)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
        SQL);
    }

    /** @param array<string,mixed> $entry */
    public function recordDebugLog(array $entry): int
    {
        $statement = $this->pdo->prepare(<<<'SQL'
            INSERT INTO debug_logs (request_id, direction, source, method, url, status, request_headers_json, request_body, response_headers_json, response_body, duration_ms, created_at)
            VALUES (:request_id, :direction, :source, :method, :url, :status, :request_headers_json, :request_body, :response_headers_json, :response_body, :duration_ms, :created_at)
        SQL);
        $statement->execute([
            'request_id' => (string) ($entry['request_id'] ?? ''),
            'direction' => (string) ($entry['direction'] ?? 'inbound'),
            'source' => (string) ($entry['source'] ?? 'unknown'),
            'method' => (string) ($entry['method'] ?? 'GET'),
            'url' => (string) ($entry['url'] ?? ''),
            'status' => (int) ($entry['status'] ?? 0),
            'request_headers_json' => (string) ($entry['request_headers_json'] ?? '{}'),
            'request_body' => (string) ($entry['request_body'] ?? ''),
            'response_headers_json' => (string) ($entry['response_headers_json'] ?? '{}'),
            'response_body' => (string) ($entry['response_body'] ?? ''),
            'duration_ms' => (int) ($entry['duration_ms'] ?? 0),
            'created_at' => (string) ($entry['created_at'] ?? gmdate('c')),
        ]);
        return (int) $this->pdo->lastInsertId();
    }

    /** @return array{items:list<array<string,mixed>>,total:int} */
    public function debugLogs(array $filters, int $page = 1, int $perPage = 50): array
    {
        $page = max(1, $page);
        $perPage = min(100, max(1, $perPage));
        $where = [];
        $parameters = [];
        foreach (['direction', 'source', 'method'] as $field) {
            $value = trim((string) ($filters[$field] ?? ''));
            if ($value !== '') {
                $where[] = $field . ' = :' . $field;
                $parameters[$field] = $value;
            }
        }
        $status = filter_var($filters['status'] ?? null, FILTER_VALIDATE_INT);
        if ($status !== false && $status !== null) {
            if (in_array($status, [2, 4, 5], true)) {
                $where[] = 'status >= :status_min AND status < :status_max';
                $parameters['status_min'] = $status * 100;
                $parameters['status_max'] = ($status + 1) * 100;
            } else {
                $where[] = 'status = :status';
                $parameters['status'] = $status;
            }
        }
        $query = trim((string) ($filters['q'] ?? ''));
        if ($query !== '') {
            $where[] = '(request_id LIKE :query OR url LIKE :query OR request_body LIKE :query OR response_body LIKE :query)';
            $parameters['query'] = '%' . $query . '%';
        }
        $clause = $where === [] ? '' : ' WHERE ' . implode(' AND ', $where);
        $count = $this->pdo->prepare('SELECT COUNT(*) FROM debug_logs' . $clause);
        $count->execute($parameters);
        $total = (int) $count->fetchColumn();
        $offset = ($page - 1) * $perPage;
        $statement = $this->pdo->prepare('SELECT * FROM debug_logs' . $clause . ' ORDER BY id DESC LIMIT ' . $perPage . ' OFFSET ' . $offset);
        $statement->execute($parameters);
        return ['items' => $statement->fetchAll() ?: [], 'total' => $total];
    }

    public function clearDebugLogs(): void
    {
        $this->pdo->exec('DELETE FROM debug_logs');
    }

    /** @return array<string,mixed>|null */
    public function findInstanceByInstallation(int $installationId): ?array
    {
        $statement = $this->pdo->prepare('SELECT ci.* FROM connector_instances ci INNER JOIN github_installations gi ON gi.instance_id = ci.id WHERE gi.github_installation_id = :installation_id AND ci.status = \'active\' AND gi.suspended_at IS NULL LIMIT 1');
        $statement->execute(['installation_id' => $installationId]);
        $row = $statement->fetch();
        return is_array($row) ? $row : null;
    }

    /** @param array<string,mixed> $event */
    public function recordWebhookEvent(array $event): bool
    {
        $sql = $this->mysql
            ? 'INSERT IGNORE INTO github_webhook_events (instance_id, delivery_id, event_name, action, github_installation_id, repository_full_name, ref_name, before_sha, after_sha, deleted, payload_json, created_at) VALUES (:instance_id, :delivery_id, :event_name, :action, :github_installation_id, :repository_full_name, :ref_name, :before_sha, :after_sha, :deleted, :payload_json, :created_at)'
            : 'INSERT OR IGNORE INTO github_webhook_events (instance_id, delivery_id, event_name, action, github_installation_id, repository_full_name, ref_name, before_sha, after_sha, deleted, payload_json, created_at) VALUES (:instance_id, :delivery_id, :event_name, :action, :github_installation_id, :repository_full_name, :ref_name, :before_sha, :after_sha, :deleted, :payload_json, :created_at)';
        $statement = $this->pdo->prepare($sql);
        $statement->execute([
            'instance_id' => $event['instance_id'],
            'delivery_id' => $event['delivery_id'],
            'event_name' => $event['event_name'],
            'action' => $event['action'],
            'github_installation_id' => $event['github_installation_id'],
            'repository_full_name' => $event['repository_full_name'],
            'ref_name' => $event['ref_name'],
            'before_sha' => $event['before_sha'],
            'after_sha' => $event['after_sha'],
            'deleted' => $event['deleted'] ? 1 : 0,
            'payload_json' => $event['payload_json'],
            'created_at' => gmdate('c'),
        ]);
        return $statement->rowCount() === 1;
    }

    /** @return list<array<string,mixed>> */
    public function webhookEvents(string $instanceId, int $afterId, int $limit = 50): array
    {
        $limit = min(100, max(1, $limit));
        $statement = $this->pdo->prepare('SELECT id, delivery_id, event_name, action, github_installation_id, repository_full_name, ref_name, before_sha, after_sha, deleted, created_at FROM github_webhook_events WHERE instance_id = :instance_id AND id > :after_id ORDER BY id ASC LIMIT ' . $limit);
        $statement->execute(['instance_id' => $instanceId, 'after_id' => $afterId]);
        return $statement->fetchAll() ?: [];
    }

    /** @return array<string,mixed> */
    public function webhookStatus(?string $instanceId = null): array
    {
        if ($instanceId === null) {
            $row = $this->pdo->query('SELECT COUNT(*) AS accepted_events, MAX(created_at) AS last_event_at FROM github_webhook_events')->fetch();
        } else {
            $statement = $this->pdo->prepare('SELECT COUNT(*) AS accepted_events, MAX(created_at) AS last_event_at FROM github_webhook_events WHERE instance_id = :instance_id');
            $statement->execute(['instance_id' => $instanceId]);
            $row = $statement->fetch();
        }
        return [
            'accepted_events' => (int) ($row['accepted_events'] ?? 0),
            'last_event_at' => $row['last_event_at'] ?? null,
        ];
    }

    /** @return array<string,mixed> */
    public function createInstance(string $instanceId, ?string $licenseId, string $credentialHash, string $returnHost): array
    {
        $now = gmdate('c');
        $statement = $this->pdo->prepare(<<<'SQL'
            INSERT INTO connector_instances (id, license_id, credential_hash, return_host, created_at, updated_at)
            VALUES (:id, :license_id, :credential_hash, :return_host, :created_at, :updated_at)
        SQL);
        $statement->execute([
            'id' => $instanceId,
            'license_id' => $licenseId,
            'credential_hash' => $credentialHash,
            'return_host' => $returnHost,
            'created_at' => $now,
            'updated_at' => $now,
        ]);
        return ['id' => $instanceId, 'license_id' => $licenseId, 'return_host' => $returnHost, 'created_at' => $now];
    }

    /** @return array<string,mixed>|null */
    public function findInstance(string $instanceId): ?array
    {
        $statement = $this->pdo->prepare('SELECT * FROM connector_instances WHERE id = :id AND status = \'active\'');
        $statement->execute(['id' => $instanceId]);
        $row = $statement->fetch();
        return is_array($row) ? $row : null;
    }

    public function touchInstance(string $instanceId): void
    {
        $statement = $this->pdo->prepare('UPDATE connector_instances SET last_seen_at = :seen, updated_at = :updated WHERE id = :id');
        $now = gmdate('c');
        $statement->execute(['seen' => $now, 'updated' => $now, 'id' => $instanceId]);
    }

    public function updateInstanceCredential(string $instanceId, string $credentialHash): void
    {
        $statement = $this->pdo->prepare('UPDATE connector_instances SET credential_hash = :credential_hash, updated_at = :updated_at WHERE id = :id AND status = \'active\'');
        $statement->execute(['credential_hash' => $credentialHash, 'updated_at' => gmdate('c'), 'id' => $instanceId]);
        if ($statement->rowCount() !== 1) {
            throw new RuntimeException('The ReleaseStation instance could not be re-paired.');
        }
    }

    public function createSession(string $id, string $instanceId, string $stateHash, string $returnUrl, string $expiresAt): void
    {
        $statement = $this->pdo->prepare(<<<'SQL'
            INSERT INTO github_connect_sessions (id, instance_id, state_hash, return_url, expires_at, created_at)
            VALUES (:id, :instance_id, :state_hash, :return_url, :expires_at, :created_at)
        SQL);
        $statement->execute([
            'id' => $id,
            'instance_id' => $instanceId,
            'state_hash' => $stateHash,
            'return_url' => $returnUrl,
            'expires_at' => $expiresAt,
            'created_at' => gmdate('c'),
        ]);
    }

    public function createPairingSession(string $id, string $instanceId, string $stateHash, string $returnUrl, string $expiresAt): void
    {
        $statement = $this->pdo->prepare(<<<'SQL'
            INSERT INTO connector_pairing_sessions (id, instance_id, state_hash, return_url, expires_at, created_at)
            VALUES (:id, :instance_id, :state_hash, :return_url, :expires_at, :created_at)
        SQL);
        $statement->execute([
            'id' => $id,
            'instance_id' => $instanceId,
            'state_hash' => $stateHash,
            'return_url' => $returnUrl,
            'expires_at' => $expiresAt,
            'created_at' => gmdate('c'),
        ]);
    }

    /** @return array<string,mixed>|null */
    public function findPairingSession(string $stateHash, string $status = 'pending'): ?array
    {
        $statement = $this->pdo->prepare('SELECT * FROM connector_pairing_sessions WHERE state_hash = :state_hash AND status = :status AND expires_at > :now LIMIT 1');
        $statement->execute(['state_hash' => $stateHash, 'status' => $status, 'now' => gmdate('c')]);
        $row = $statement->fetch();
        return is_array($row) ? $row : null;
    }

    /** @return array<string,mixed>|null */
    public function findPairingSessionById(string $id, string $stateHash): ?array
    {
        $statement = $this->pdo->prepare('SELECT * FROM connector_pairing_sessions WHERE id = :id AND state_hash = :state_hash AND status IN (\'pending\', \'authorized\') AND expires_at > :now LIMIT 1');
        $statement->execute(['id' => $id, 'state_hash' => $stateHash, 'now' => gmdate('c')]);
        $row = $statement->fetch();
        return is_array($row) ? $row : null;
    }

    public function authorizePairingSession(string $id, int $installationId): void
    {
        $statement = $this->pdo->prepare('UPDATE connector_pairing_sessions SET status = \'authorized\', github_installation_id = :installation_id, authorized_at = :authorized_at WHERE id = :id AND status = \'pending\'');
        $statement->execute([
            'installation_id' => $installationId,
            'authorized_at' => gmdate('c'),
            'id' => $id,
        ]);
        if ($statement->rowCount() !== 1) {
            throw new RuntimeException('The connector pairing session is no longer pending.');
        }
    }

    /** @return array<string,mixed>|null */
    public function consumePairingSession(string $instanceId, string $stateHash): ?array
    {
        $this->pdo->beginTransaction();
        try {
            $forUpdate = $this->mysql ? ' FOR UPDATE' : '';
            $statement = $this->pdo->prepare(<<<SQL
                SELECT * FROM connector_pairing_sessions
                WHERE instance_id = :instance_id AND state_hash = :state_hash AND status = 'authorized' AND expires_at > :now
                LIMIT 1{$forUpdate}
            SQL);
            $statement->execute(['instance_id' => $instanceId, 'state_hash' => $stateHash, 'now' => gmdate('c')]);
            $session = $statement->fetch();
            if (!is_array($session)) {
                $this->pdo->commit();
                return null;
            }
            $update = $this->pdo->prepare('UPDATE connector_pairing_sessions SET status = \'completed\', completed_at = :completed_at WHERE id = :id');
            $update->execute(['completed_at' => gmdate('c'), 'id' => $session['id']]);
            $this->pdo->commit();
            return $session;
        } catch (\Throwable $exception) {
            $this->pdo->rollBack();
            throw $exception;
        }
    }

    /** @return array<string,mixed>|null */
    public function consumeSession(string $stateHash): ?array
    {
        $this->pdo->beginTransaction();
        try {
            $forUpdate = $this->mysql ? ' FOR UPDATE' : '';
            $statement = $this->pdo->prepare(<<<SQL
                SELECT * FROM github_connect_sessions
                WHERE state_hash = :state_hash AND status = 'pending' AND expires_at > :now
                LIMIT 1{$forUpdate}
            SQL);
            $statement->execute(['state_hash' => $stateHash, 'now' => gmdate('c')]);
            $session = $statement->fetch();
            if (!is_array($session)) {
                $this->pdo->commit();
                return null;
            }
            $update = $this->pdo->prepare('UPDATE github_connect_sessions SET status = \'consumed\', consumed_at = :consumed WHERE id = :id');
            $update->execute(['consumed' => gmdate('c'), 'id' => $session['id']]);
            $this->pdo->commit();
            return $session;
        } catch (\Throwable $exception) {
            $this->pdo->rollBack();
            throw $exception;
        }
    }

    /** @param array<string,mixed> $installation */
    public function saveInstallation(array $installation): void
    {
        $sql = $this->mysql ? <<<'SQL'
            INSERT INTO github_installations
                (id, instance_id, github_installation_id, account_login, account_type, repository_selection, permissions_json, created_at, updated_at)
            VALUES (:id, :instance_id, :github_installation_id, :account_login, :account_type, :repository_selection, :permissions_json, :created_at, :updated_at)
            ON DUPLICATE KEY UPDATE
                instance_id = VALUES(instance_id),
                account_login = VALUES(account_login),
                account_type = VALUES(account_type),
                repository_selection = VALUES(repository_selection),
                permissions_json = VALUES(permissions_json),
                suspended_at = NULL,
                updated_at = VALUES(updated_at)
        SQL : <<<'SQL'
            INSERT INTO github_installations
                (id, instance_id, github_installation_id, account_login, account_type, repository_selection, permissions_json, created_at, updated_at)
            VALUES (:id, :instance_id, :github_installation_id, :account_login, :account_type, :repository_selection, :permissions_json, :created_at, :updated_at)
            ON CONFLICT(github_installation_id) DO UPDATE SET
                instance_id = excluded.instance_id,
                account_login = excluded.account_login,
                account_type = excluded.account_type,
                repository_selection = excluded.repository_selection,
                permissions_json = excluded.permissions_json,
                suspended_at = NULL,
                updated_at = excluded.updated_at
        SQL;
        $statement = $this->pdo->prepare($sql);
        $now = gmdate('c');
        $statement->execute([
            'id' => $installation['id'],
            'instance_id' => $installation['instance_id'],
            'github_installation_id' => $installation['github_installation_id'],
            'account_login' => $installation['account_login'],
            'account_type' => $installation['account_type'],
            'repository_selection' => $installation['repository_selection'],
            'permissions_json' => json_encode($installation['permissions'], JSON_THROW_ON_ERROR),
            'created_at' => $now,
            'updated_at' => $now,
        ]);
    }

    /** @return list<array<string,mixed>> */
    public function installations(string $instanceId): array
    {
        $statement = $this->pdo->prepare('SELECT * FROM github_installations WHERE instance_id = :instance_id AND suspended_at IS NULL ORDER BY account_login');
        $statement->execute(['instance_id' => $instanceId]);
        return $statement->fetchAll();
    }

    public function hasInstallation(string $instanceId, int $installationId): bool
    {
        $statement = $this->pdo->prepare('SELECT 1 FROM github_installations WHERE instance_id = :instance_id AND github_installation_id = :installation_id AND suspended_at IS NULL');
        $statement->execute(['instance_id' => $instanceId, 'installation_id' => $installationId]);
        return $statement->fetchColumn() !== false;
    }
}
