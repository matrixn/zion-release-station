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
        SQL);
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
