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
        public string $databasePath,
        public string $adminToken,
        public string $githubAppId,
        public string $githubAppSlug,
        public string $githubClientId,
        public string $githubClientSecret,
        public string $githubPrivateKeyPath,
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
            databasePath: getenv('CONNECTOR_DATABASE_PATH') ?: dirname(__DIR__) . '/var/connector.sqlite',
            adminToken: $adminToken,
            githubAppId: trim((string) getenv('CONNECTOR_GITHUB_APP_ID')),
            githubAppSlug: trim((string) getenv('CONNECTOR_GITHUB_APP_SLUG')),
            githubClientId: trim((string) getenv('CONNECTOR_GITHUB_CLIENT_ID')),
            githubClientSecret: trim((string) getenv('CONNECTOR_GITHUB_CLIENT_SECRET')),
            githubPrivateKeyPath: trim((string) getenv('CONNECTOR_GITHUB_PRIVATE_KEY_PATH')),
            returnHosts: self::csv(getenv('CONNECTOR_RETURN_HOSTS') ?: ''),
        );
    }

    public function callbackUrl(): string
    {
        return $this->publicBaseUrl . '/github/callback';
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
        return $this->returnHosts === [] || in_array($host, array_map('strtolower', $this->returnHosts), true);
    }

    /** @return list<string> */
    private static function csv(string $value): array
    {
        return array_values(array_filter(array_map('trim', explode(',', $value)), static fn (string $item): bool => $item !== ''));
    }
}
