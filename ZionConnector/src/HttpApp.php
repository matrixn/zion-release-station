<?php

declare(strict_types=1);

namespace ZionConnector;

use JsonException;
use RuntimeException;

final class HttpApp
{
    public function __construct(
        private readonly Config $config,
        private readonly Database $database,
        private readonly GitHubClient $github,
    ) {
    }

    public function run(): void
    {
        try {
            $method = strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET');
            $path = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH) ?: '/';

            if ($method === 'GET' && $path === '/') {
                $this->json(200, [
                    'service' => 'Zion Connector',
                    'status' => 'ok',
                    'health_url' => '/healthz',
                ]);
                return;
            }
            if ($method === 'GET' && $path === '/healthz') {
                $this->json(200, ['status' => 'ok', 'github_configured' => $this->config->githubConfigured()]);
                return;
            }
            if ($method === 'POST' && $path === '/internal/instances') {
                $this->provisionInstance();
                return;
            }
            if ($method === 'POST' && $path === '/pairing/sessions') {
                $this->pairingSession();
                return;
            }
            if ($method === 'POST' && $path === '/pairing/exchange') {
                $this->exchangePairing();
                return;
            }
            if ($method === 'GET' && $path === '/github/callback') {
                $this->callback();
                return;
            }

            if (preg_match('#^/v1/instances/([^/]+)/github/(sessions|status|repositories)(?:/(.*))?$#', $path, $matches) === 1) {
                $instanceId = rawurldecode($matches[1]);
                $resource = $matches[2];
                $suffix = $matches[3] ?? '';
                $instance = $this->authenticateInstance($instanceId);
                if ($instance === null) {
                    return;
                }
                $this->database->touchInstance($instanceId);
                if ($resource === 'sessions' && $method === 'POST') {
                    $this->startSession($instance);
                    return;
                }
                if ($resource === 'status' && $method === 'GET') {
                    $this->status($instanceId);
                    return;
                }
                if ($resource === 'repositories' && $method === 'GET' && $suffix === '') {
                    $this->repositories($instanceId);
                    return;
                }
                if ($resource === 'repositories' && $method === 'GET' && str_ends_with($suffix, '/branches')) {
                    $this->branches($instanceId, $suffix);
                    return;
                }
                if ($resource === 'repositories' && $method === 'GET' && str_ends_with($suffix, '/archive')) {
                    $this->archive($instanceId, $suffix);
                    return;
                }
            }

            $this->json(404, ['error' => ['code' => 'NOT_FOUND', 'message' => 'Not found.']]);
        } catch (\Throwable $exception) {
            error_log('Zion Connector request failed: ' . $exception->getMessage());
            $message = $this->config->debug ? $exception->getMessage() : 'The connector could not complete the request.';
            $this->json(500, ['error' => ['code' => 'INTERNAL_ERROR', 'message' => $message]]);
        }
    }

    private function provisionInstance(): void
    {
        if (!$this->adminAuthorized()) {
            $this->json(401, ['error' => ['code' => 'UNAUTHORIZED', 'message' => 'Admin authentication is required.']]);
            return;
        }
        $body = $this->requestBody();
        $instanceId = trim((string) ($body['instance_id'] ?? ''));
        $returnHost = strtolower(trim((string) ($body['return_host'] ?? '')));
        if (!preg_match('/^[A-Za-z0-9][A-Za-z0-9._-]{1,127}$/', $instanceId) || !preg_match('/^[a-z0-9.-]+$/', $returnHost)) {
            $this->json(422, ['error' => ['code' => 'INVALID_INSTANCE', 'message' => 'instance_id and return_host are invalid.']]);
            return;
        }
        if ($this->database->findInstance($instanceId) !== null) {
            $this->json(409, ['error' => ['code' => 'INSTANCE_EXISTS', 'message' => 'The instance is already provisioned.']]);
            return;
        }
        $credential = bin2hex(random_bytes(32));
        $instance = $this->database->createInstance(
            $instanceId,
            isset($body['license_id']) ? trim((string) $body['license_id']) : null,
            hash('sha256', $credential),
            $returnHost,
        );
        $this->json(201, ['data' => $instance + ['credential' => $credential]]);
    }

    private function pairingSession(): void
    {
        if (!$this->config->githubConfigured()) {
            $this->json(503, ['error' => ['code' => 'GITHUB_NOT_CONFIGURED', 'message' => $this->config->githubConfigurationError()]]);
            return;
        }
        $body = $this->requestBody();
        $instanceId = trim((string) ($body['instance_id'] ?? ''));
        $returnUrl = trim((string) ($body['return_url'] ?? ''));
        $parsed = parse_url($returnUrl);
        $returnHost = is_array($parsed) ? strtolower((string) ($parsed['host'] ?? '')) : '';
        if (!preg_match('/^[A-Za-z0-9][A-Za-z0-9._-]{1,127}$/', $instanceId) || $returnHost === '' || !$this->config->isAllowedReturnUrl($returnUrl, $returnHost)) {
            $this->json(422, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The ReleaseStation instance and HTTPS return URL are invalid.']]);
            return;
        }
        if ($this->database->findInstance($instanceId) !== null) {
            $this->json(409, ['error' => ['code' => 'INSTANCE_EXISTS', 'message' => 'This ReleaseStation instance is already paired.']]);
            return;
        }
        $state = self::randomToken(32);
        $this->database->createPairingSession(
            self::randomToken(18),
            $instanceId,
            hash('sha256', $state),
            $returnUrl,
            gmdate('c', time() + 600),
        );
        $this->json(200, [
            'id' => $instanceId,
            'authorize_url' => $this->github->authorizationUrl($state),
            'expires_in' => 600,
        ]);
    }

    private function exchangePairing(): void
    {
        $body = $this->requestBody();
        $instanceId = trim((string) ($body['instance_id'] ?? ''));
        $pairingCode = trim((string) ($body['pairing_code'] ?? ''));
        if ($instanceId === '' || $pairingCode === '') {
            $this->json(422, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The pairing code is required.']]);
            return;
        }
        $session = $this->database->consumePairingSession($instanceId, hash('sha256', $pairingCode));
        if ($session === null || $this->database->findInstance($instanceId) === null) {
            $this->json(401, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The pairing code is invalid or expired.']]);
            return;
        }
        $this->json(200, ['credential' => $this->config->pairingCredential($instanceId, $pairingCode)]);
    }

    /** @param array<string,mixed> $instance */
    private function startSession(array $instance): void
    {
        if (!$this->config->githubConfigured()) {
            $this->json(503, ['error' => ['code' => 'GITHUB_NOT_CONFIGURED', 'message' => $this->config->githubConfigurationError()]]);
            return;
        }
        $body = $this->requestBody();
        $returnUrl = trim((string) ($body['return_url'] ?? ''));
        if (!$this->config->isAllowedReturnUrl($returnUrl, (string) $instance['return_host'])) {
            $this->json(422, ['error' => ['code' => 'INVALID_RETURN_URL', 'message' => 'The ReleaseStation return URL is not allowed.']]);
            return;
        }
        $state = self::randomToken(32);
        $sessionId = self::randomToken(18);
        $this->database->createSession(
            $sessionId,
            (string) $instance['id'],
            hash('sha256', $state),
            $returnUrl,
            gmdate('c', time() + 600),
        );
        $this->json(200, [
            'id' => $sessionId,
            'authorize_url' => $this->github->authorizationUrl($state),
            'expires_in' => 600,
        ]);
    }

    private function callback(): void
    {
        $state = trim((string) ($_GET['state'] ?? ''));
        $code = trim((string) ($_GET['code'] ?? ''));
        $installationId = filter_var($_GET['installation_id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        if ($state === '' || $code === '' || !is_int($installationId)) {
            $this->json(400, ['error' => ['code' => 'INVALID_CALLBACK', 'message' => 'GitHub callback parameters are incomplete.']]);
            return;
        }
        $stateHash = hash('sha256', $state);
        $pairingSession = $this->database->findPairingSession($stateHash);
        $session = $pairingSession ?? $this->database->consumeSession($stateHash);
        if ($session === null) {
            $this->json(400, ['error' => ['code' => 'INVALID_STATE', 'message' => 'The GitHub connection state is invalid or expired.']]);
            return;
        }
        $tokens = $this->github->exchangeCode($code);
        $userToken = (string) ($tokens['access_token'] ?? '');
        if ($userToken === '') {
            throw new RuntimeException('GitHub did not return an OAuth access token.');
        }
        $this->github->authenticatedUser($userToken);
        $userInstallation = $this->github->userInstallation($userToken, $installationId);
        if ((int) ($userInstallation['id'] ?? 0) !== $installationId) {
            throw new RuntimeException('GitHub installation verification failed.');
        }
        $appInstallation = $this->github->appInstallation($installationId);
        $account = is_array($appInstallation['account'] ?? null) ? $appInstallation['account'] : [];
        $instanceId = (string) $session['instance_id'];
        $isPairing = $pairingSession !== null;
        if ($isPairing && $this->database->findInstance($instanceId) === null) {
            $credential = $this->config->pairingCredential($instanceId, $state);
            $this->database->createInstance($instanceId, null, hash('sha256', $credential), (string) parse_url((string) $session['return_url'], PHP_URL_HOST));
        }
        $this->database->saveInstallation([
            'id' => self::randomToken(18),
            'instance_id' => $instanceId,
            'github_installation_id' => $installationId,
            'account_login' => (string) ($account['login'] ?? $userInstallation['account']['login'] ?? 'unknown'),
            'account_type' => (string) ($account['type'] ?? 'User'),
            'repository_selection' => (string) ($appInstallation['repository_selection'] ?? 'selected'),
            'permissions' => is_array($appInstallation['permissions'] ?? null) ? $appInstallation['permissions'] : [],
        ]);
        if ($isPairing) {
            $this->database->authorizePairingSession((string) $session['id'], $installationId);
            header('Location: ' . $this->withQuery((string) $session['return_url'], ['github' => 'connected', 'pairing_code' => $state]), true, 303);
            return;
        }
        header('Location: ' . $session['return_url'], true, 303);
    }

    private function status(string $instanceId): void
    {
        $rows = $this->database->installations($instanceId);
        $installations = array_map(static function (array $row): array {
            return [
                'github_installation_id' => (int) $row['github_installation_id'],
                'account_login' => $row['account_login'],
                'account_type' => $row['account_type'],
                'repository_selection' => $row['repository_selection'],
                'permissions' => json_decode((string) $row['permissions_json'], true) ?: [],
            ];
        }, $rows);
        $this->json(200, [
            'state' => $installations === [] ? 'disconnected' : 'connected',
            'account_login' => $installations[0]['account_login'] ?? null,
            'installations' => $installations,
            'message' => $installations === [] ? 'No GitHub installation is connected.' : null,
        ]);
    }

    private function repositories(string $instanceId): void
    {
        $result = [];
        foreach ($this->database->installations($instanceId) as $installation) {
            $installationId = (int) $installation['github_installation_id'];
            foreach ($this->github->repositories($installationId) as $repository) {
                $result[] = [
                    'installation_id' => $installationId,
                    'account_login' => $installation['account_login'],
                    'id' => (int) ($repository['id'] ?? 0),
                    'name' => $repository['name'] ?? '',
                    'full_name' => $repository['full_name'] ?? '',
                    'private' => (bool) ($repository['private'] ?? false),
                    'default_branch' => $repository['default_branch'] ?? '',
                    'clone_url' => $repository['clone_url'] ?? '',
                    'ssh_url' => $repository['ssh_url'] ?? '',
                ];
            }
        }
        $this->json(200, ['repositories' => $result]);
    }

    private function branches(string $instanceId, string $suffix): void
    {
        if (preg_match('#^([^/]+)/([^/]+)/branches$#', $suffix, $matches) !== 1) {
            $this->json(422, ['error' => ['code' => 'INVALID_REPOSITORY', 'message' => 'Repository must use owner/name format.']]);
            return;
        }
        $installationId = filter_var($_GET['installation_id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        if (!is_int($installationId) || !$this->database->hasInstallation($instanceId, $installationId)) {
            $this->json(422, ['error' => ['code' => 'INVALID_INSTALLATION', 'message' => 'The GitHub installation is not connected to this instance.']]);
            return;
        }
        $this->json(200, ['branches' => $this->github->branches($installationId, rawurldecode($matches[1]), rawurldecode($matches[2]))]);
    }

    private function archive(string $instanceId, string $suffix): void
    {
        if (preg_match('#^([^/]+)/([^/]+)/archive$#', $suffix, $matches) !== 1) {
            $this->json(422, ['error' => ['code' => 'INVALID_REPOSITORY', 'message' => 'Repository must use owner/name format.']]);
            return;
        }
        $installationId = filter_var($_GET['installation_id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        $ref = trim((string) ($_GET['ref'] ?? ''));
        if (!is_int($installationId) || !$this->database->hasInstallation($instanceId, $installationId) || $ref === '' || strlen($ref) > 256) {
            $this->json(422, ['error' => ['code' => 'INVALID_ARCHIVE_REQUEST', 'message' => 'A connected installation and a valid Git reference are required.']]);
            return;
        }
        $archive = $this->github->archive($installationId, rawurldecode($matches[1]), rawurldecode($matches[2]), $ref);
        header('Content-Type: application/gzip');
        header('Cache-Control: no-store');
        header('Content-Length: ' . strlen($archive));
        echo $archive;
    }

    /** @return array<string,mixed> */
    private function requestBody(): array
    {
        $decoded = json_decode(file_get_contents('php://input') ?: '{}', true, 512, JSON_THROW_ON_ERROR);
        if (!is_array($decoded)) {
            throw new JsonException('Request body must be an object.');
        }
        return $decoded;
    }

    /** @return array<string,mixed>|null */
    private function authenticateInstance(string $instanceId): ?array
    {
        $token = $this->bearerToken();
        $instance = $this->database->findInstance($instanceId);
        if ($token === '' || $instance === null || !hash_equals((string) $instance['credential_hash'], hash('sha256', $token))) {
            $this->json(401, ['error' => ['code' => 'UNAUTHORIZED', 'message' => 'Instance authentication failed.']]);
            return null;
        }
        return $instance;
    }

    private function adminAuthorized(): bool
    {
        return $this->bearerToken() !== '' && hash_equals($this->config->adminToken, $this->bearerToken());
    }

    private function bearerToken(): string
    {
        $header = trim((string) ($_SERVER['HTTP_AUTHORIZATION'] ?? ''));
        return preg_match('/^Bearer\s+(.+)$/i', $header, $matches) === 1 ? trim($matches[1]) : '';
    }

    /** @param array<string,string> $parameters */
    private function withQuery(string $url, array $parameters): string
    {
        $separator = str_contains($url, '?') ? '&' : '?';
        return $url . $separator . http_build_query($parameters, '', '&', PHP_QUERY_RFC3986);
    }

    /** @param array<string,mixed> $payload */
    private function json(int $status, array $payload): void
    {
        http_response_code($status);
        header('Content-Type: application/json; charset=utf-8');
        header('Cache-Control: no-store');
        echo json_encode($payload, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES);
    }

    private static function randomToken(int $bytes): string
    {
        return rtrim(strtr(base64_encode(random_bytes($bytes)), '+/', '-_'), '=');
    }
}
