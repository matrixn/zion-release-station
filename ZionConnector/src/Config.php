<?php

declare(strict_types=1);

namespace ZionConnector;

use RuntimeException;

final readonly class Config
{
    /** @param list<string> $returnHosts */
    public function __construct(
        public string $environment,
        public bool $debug,
        public string $publicBaseUrl,
        public string $adminToken,
        public string $databaseDriver,
        public string $databaseHost,
        public int $databasePort,
        public string $databaseName,
        public string $databaseUser,
        public string $databasePassword,
        public string $databasePath,
        public string $githubAppId,
        public string $githubAppSlug,
        public string $githubClientId,
        public string $githubClientSecret,
        public string $githubPrivateKeyPath,
        public string $githubWebhookSecret,
        public array $returnHosts,
    ) {
    }

    public static function fromEnvironment(): self
    {
        $publicBaseUrl = rtrim(trim((string) getenv('CONNECTOR_PUBLIC_BASE_URL')), '/');
        if ($publicBaseUrl === '' || !str_starts_with($publicBaseUrl, 'https://')) {
            throw new RuntimeException('CONNECTOR_PUBLIC_BASE_URL must be an HTTPS URL.');
        }
        $adminToken = trim((string) getenv('CONNECTOR_ADMIN_TOKEN'));
        if ($adminToken === '') {
            throw new RuntimeException('CONNECTOR_ADMIN_TOKEN is required.');
        }
        return new self(
            environment: getenv('APP_ENV') ?: 'production',
            debug: filter_var(getenv('APP_DEBUG') ?: false, FILTER_VALIDATE_BOOL),
            publicBaseUrl: $publicBaseUrl,
            adminToken: $adminToken,
            databaseDriver: strtolower(trim(getenv('CONNECTOR_DB_DRIVER') ?: 'mysql')),
            databaseHost: trim(getenv('CONNECTOR_DB_HOST') ?: '127.0.0.1'),
            databasePort: max(1, (int) (getenv('CONNECTOR_DB_PORT') ?: 3306)),
            databaseName: trim(getenv('CONNECTOR_DB_NAME') ?: 'zion_connector'),
            databaseUser: trim(getenv('CONNECTOR_DB_USER') ?: 'zion_connector'),
            databasePassword: (string) getenv('CONNECTOR_DB_PASSWORD'),
            databasePath: getenv('CONNECTOR_DATABASE_PATH') ?: dirname(__DIR__) . '/var/connector.sqlite',
            githubAppId: trim((string) getenv('CONNECTOR_GITHUB_APP_ID')),
            githubAppSlug: trim((string) getenv('CONNECTOR_GITHUB_APP_SLUG')),
            githubClientId: trim((string) getenv('CONNECTOR_GITHUB_CLIENT_ID')),
            githubClientSecret: trim((string) getenv('CONNECTOR_GITHUB_CLIENT_SECRET')),
            githubPrivateKeyPath: trim((string) (getenv('CONNECTOR_GITHUB_PRIVATE_KEY_PATH') ?: dirname(__DIR__) . '/key/github-private-key.pem')),
            githubWebhookSecret: trim((string) getenv('CONNECTOR_GITHUB_WEBHOOK_SECRET')),
            returnHosts: self::csv(getenv('CONNECTOR_RETURN_HOSTS') ?: ''),
        );
    }

    public function callbackUrl(): string
    {
        return $this->publicBaseUrl . '/github/callback';
    }

    public function pairingCredential(string $instanceId, string $state): string
    {
        return hash_hmac('sha256', 'pairing|' . $instanceId . '|' . $state, $this->adminToken);
    }

    public function databaseDsn(): string
    {
        if ($this->databaseDriver === 'sqlite') {
            return 'sqlite:' . $this->databasePath;
        }
        if (!in_array($this->databaseDriver, ['mysql', 'mariadb'], true) || $this->databaseName === '') {
            throw new RuntimeException('CONNECTOR_DB_DRIVER must be mysql, mariadb or sqlite, with a database name.');
        }
        return sprintf(
            'mysql:host=%s;port=%d;dbname=%s;charset=utf8mb4',
            $this->databaseHost,
            $this->databasePort,
            $this->databaseName,
        );
    }

    public function githubConfigured(): bool
    {
        return $this->githubAppId !== ''
            && $this->githubAppSlug !== ''
            && $this->githubClientId !== ''
            && $this->githubClientSecret !== ''
            && is_readable($this->githubPrivateKeyPath);
    }

    public function githubConfigurationError(): string
    {
        if ($this->githubAppId === '' || $this->githubAppSlug === '') {
            return 'GitHub App ID and slug are not configured.';
        }
        if ($this->githubClientId === '' || $this->githubClientSecret === '') {
            return 'GitHub OAuth client credentials are not configured.';
        }
        if (!is_readable($this->githubPrivateKeyPath)) {
            return 'The GitHub App private key is missing or unreadable.';
        }
        return '';
    }

    public function githubWebhookConfigured(): bool
    {
        return $this->githubWebhookSecret !== '';
    }

    public function isAllowedReturnUrl(string $returnUrl, string $returnHost): bool
    {
        $parsed = parse_url($returnUrl);
        if (!is_array($parsed) || ($parsed['scheme'] ?? '') !== 'https' || empty($parsed['host'])) {
            return false;
        }
        $host = strtolower((string) $parsed['host']);
        $expected = strtolower(trim($returnHost));
        if ($expected !== '' && $host !== $expected) {
            return false;
        }
        if ($this->returnHosts === []) {
            return true;
        }
        $allowedHosts = array_map(static function (string $value): string {
            $value = strtolower(trim($value));
            $parsed = parse_url($value);
            return is_array($parsed) && isset($parsed['host']) ? strtolower((string) $parsed['host']) : $value;
        }, $this->returnHosts);
        return in_array($host, $allowedHosts, true);
    }

    /** @return list<string> */
    private static function csv(string $value): array
    {
        return array_values(array_filter(array_map('trim', explode(',', $value)), static fn (string $item): bool => $item !== ''));
    }
}
