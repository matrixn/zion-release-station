<?php

declare(strict_types=1);

namespace ZionConnector;

use JsonException;
use RuntimeException;

final class HttpApp
{
    private readonly DebugLogger $debug;
    private string $rawRequestBody = '';
    private string $lastResponseBody = '';
    private string $currentRequestId = '';

    public function __construct(
        private readonly Config $config,
        private readonly Database $database,
        private readonly GitHubClient $github,
        ?DebugLogger $debug = null,
    ) {
        $this->debug = $debug ?? new DebugLogger($database, false);
    }

    public function run(): void
    {
        $started = microtime(true);
        $this->currentRequestId = $this->debug->requestId();
        $this->rawRequestBody = file_get_contents('php://input') ?: '';
        try {
            $method = strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET');
            $requestUri = $_SERVER['REQUEST_URI'] ?? '/';
            $path = parse_url($requestUri, PHP_URL_PATH) ?: '/';

            if (str_starts_with($path, '/admin')) {
                if (!$this->config->debug) {
                    http_response_code(404);
                    $this->lastResponseBody = 'Not found.';
                    return;
                }
                if ($method === 'GET' && $path === '/admin') {
                    $this->adminPage();
                    return;
                }
                if ($path === '/admin/api/logs' && $method === 'GET') {
                    $this->adminLogs();
                    return;
                }
                if ($path === '/admin/api/logs' && $method === 'DELETE') {
                    $this->database->clearDebugLogs();
                    $this->json(204, []);
                    return;
                }
                $this->json(404, ['error' => ['code' => 'NOT_FOUND', 'message' => 'Admin endpoint not found.']]);
                return;
            }

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
            if ($method === 'GET' && preg_match('#^/pairing/sessions/([^/]+)/status$#', $path, $matches) === 1) {
                $this->pairingStatus(rawurldecode($matches[1]));
                return;
            }
            if ($method === 'GET' && $path === '/pairing/complete') {
                $this->pairingCompletePage();
                return;
            }
            if ($method === 'GET' && $path === '/github/callback') {
                $this->callback();
                return;
            }
            if ($method === 'POST' && $path === '/github/webhook') {
                $this->githubWebhook();
                return;
            }

            if (preg_match('#^/v1/instances/([^/]+)/github/(sessions|status|repositories|webhooks)(?:/(.*))?$#', $path, $matches) === 1) {
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
                if ($resource === 'webhooks' && $method === 'GET' && ($suffix === '' || $suffix === 'status')) {
                    $this->webhookStatus($instanceId);
                    return;
                }
                if ($resource === 'webhooks' && $method === 'GET' && $suffix === 'events') {
                    $this->webhookEvents($instanceId);
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
                if ($resource === 'repositories' && $method === 'GET' && str_ends_with($suffix, '/commits')) {
                    $this->commits($instanceId, $suffix);
                    return;
                }
                if ($resource === 'repositories' && $method === 'GET' && str_ends_with($suffix, '/commit')) {
                    $this->commit($instanceId, $suffix);
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
        } finally {
            $loggedPath = parse_url((string) ($_SERVER['REQUEST_URI'] ?? '/'), PHP_URL_PATH) ?: '/';
            if ($this->debug->enabled() && !str_starts_with($loggedPath, '/admin')) {
                $this->debug->logInbound(
                    $this->currentRequestId,
                    strtoupper($_SERVER['REQUEST_METHOD'] ?? 'GET'),
                    (string) ($_SERVER['REQUEST_URI'] ?? '/'),
                    $this->debug->serverHeaders($_SERVER),
                    $this->rawRequestBody,
                    http_response_code() ?: 200,
                    header_list(),
                    $this->lastResponseBody,
                    (int) round((microtime(true) - $started) * 1000),
                );
            }
        }
    }

    private function adminPage(): void
    {
        http_response_code(200);
        header('Content-Type: text/html; charset=utf-8');
        header('Cache-Control: no-store');
        $this->lastResponseBody = AdminConsole::render();
        echo $this->lastResponseBody;
    }

    private function adminLogs(): void
    {
        $page = filter_var($_GET['page'] ?? 1, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]) ?: 1;
        $perPage = filter_var($_GET['per_page'] ?? 50, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1, 'max_range' => 100]]) ?: 50;
        $status = trim((string) ($_GET['status'] ?? ''));
        $filters = [
            'q' => trim((string) ($_GET['q'] ?? '')),
            'direction' => trim((string) ($_GET['direction'] ?? '')),
            'source' => trim((string) ($_GET['source'] ?? '')),
            'method' => strtoupper(trim((string) ($_GET['method'] ?? ''))),
            'status' => $status,
        ];
        $result = $this->database->debugLogs($filters, $page, $perPage);
        $this->json(200, ['data' => $result + ['page' => $page, 'per_page' => $perPage]]);
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
        $completionUrl = $this->config->publicBaseUrl . '/pairing/complete';
        if (!preg_match('/^[A-Za-z0-9][A-Za-z0-9._-]{1,127}$/', $instanceId)) {
            $this->json(422, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The ReleaseStation instance is invalid.']]);
            return;
        }
        $state = self::randomToken(32);
        $sessionId = self::randomToken(18);
        $this->database->createPairingSession(
            $sessionId,
            $instanceId,
            hash('sha256', $state),
            $completionUrl,
            gmdate('c', time() + 600),
        );
        $this->json(200, [
            'id' => $sessionId,
            'instance_id' => $instanceId,
            'authorize_url' => $this->github->authorizationUrl($state),
            'poll_token' => $state,
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

    private function pairingStatus(string $sessionId): void
    {
        $pairingToken = trim((string) ($_GET['pairing_token'] ?? ''));
        if ($sessionId === '' || $pairingToken === '') {
            $this->json(422, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The pairing session and token are required.']]);
            return;
        }
        $session = $this->database->findPairingSessionById($sessionId, hash('sha256', $pairingToken));
        if ($session === null) {
            $this->json(401, ['error' => ['code' => 'INVALID_PAIRING', 'message' => 'The pairing session is invalid or expired.']]);
            return;
        }
        $payload = ['state' => (string) $session['status'], 'expires_in' => 600];
        if ((string) $session['status'] === 'authorized') {
            $payload['pairing_code'] = $pairingToken;
        }
        $this->json(200, $payload);
    }

    private function pairingCompletePage(): void
    {
        http_response_code(200);
        header('Content-Type: text/html; charset=utf-8');
        header('Cache-Control: no-store');
        echo '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Zion Connector</title><style>body{font:16px system-ui,sans-serif;background:#f5f7fb;color:#182230;display:grid;place-items:center;min-height:100vh;margin:0}.card{max-width:520px;margin:24px;padding:32px;background:#fff;border:1px solid #dce3ec;border-radius:14px;box-shadow:0 12px 40px #1e33421a}h1{font-size:24px;margin:0 0 12px}p{line-height:1.55;color:#5d6b7a}.ok{color:#16845b;font-weight:700}</style></head><body><main class="card"><div class="ok">GitHub authorization completed</div><h1>Return to Zion ReleaseStation</h1><p>You can close this window. The ReleaseStation application is checking the connection and will update automatically.</p></main></body></html>';
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
        if ($state === '' || !is_int($installationId)) {
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
        $pairingSession = $pairingSession !== null;
        $userInstallation = [];
        if ($code !== '') {
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
        } elseif (!$pairingSession) {
            $this->json(400, ['error' => ['code' => 'INVALID_CALLBACK', 'message' => 'GitHub OAuth callback parameters are incomplete.']]);
            return;
        }
        $appInstallation = $this->github->appInstallation($installationId);
        $account = is_array($appInstallation['account'] ?? null) ? $appInstallation['account'] : [];
        $instanceId = (string) $session['instance_id'];
        $isPairing = $pairingSession;
        if ($isPairing) {
            $credential = $this->config->pairingCredential($instanceId, $state);
            if ($this->database->findInstance($instanceId) === null) {
                $this->database->createInstance($instanceId, null, hash('sha256', $credential), (string) parse_url((string) $session['return_url'], PHP_URL_HOST));
            } else {
                $this->database->updateInstanceCredential($instanceId, hash('sha256', $credential));
            }
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
            header('Location: ' . $this->withQuery((string) $session['return_url'], ['session_id' => $session['id']]), true, 303);
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
            'webhook_configured' => $this->config->githubWebhookConfigured(),
            'webhook' => $this->database->webhookStatus($instanceId),
        ]);
    }

    private function webhookStatus(string $instanceId): void
    {
        $status = $this->database->webhookStatus($instanceId);
        $this->json(200, [
            'configured' => $this->config->githubWebhookConfigured(),
            'endpoint' => $this->config->publicBaseUrl . '/github/webhook',
            'accepted_events' => $status['accepted_events'],
            'last_event_at' => $status['last_event_at'],
        ]);
    }

    private function webhookEvents(string $instanceId): void
    {
        $afterID = max(0, (int) ($_GET['after_id'] ?? 0));
        $events = $this->database->webhookEvents($instanceId, $afterID, (int) ($_GET['limit'] ?? 50));
        $this->json(200, ['events' => $events]);
    }

    private function githubWebhook(): void
    {
        if (!$this->config->githubWebhookConfigured()) {
            $this->json(503, ['error' => ['code' => 'WEBHOOK_NOT_CONFIGURED', 'message' => 'CONNECTOR_GITHUB_WEBHOOK_SECRET is not configured.']]);
            return;
        }
        $rawBody = $this->rawRequestBody;
        $signature = trim((string) ($_SERVER['HTTP_X_HUB_SIGNATURE_256'] ?? ''));
        $expected = 'sha256=' . hash_hmac('sha256', $rawBody, $this->config->githubWebhookSecret);
        if ($signature === '' || !hash_equals($expected, $signature)) {
            $this->json(401, ['error' => ['code' => 'INVALID_WEBHOOK_SIGNATURE', 'message' => 'Webhook signature validation failed.']]);
            return;
        }
        $deliveryID = trim((string) ($_SERVER['HTTP_X_GITHUB_DELIVERY'] ?? ''));
        $eventName = trim((string) ($_SERVER['HTTP_X_GITHUB_EVENT'] ?? ''));
        if ($deliveryID === '' || $eventName === '') {
            $this->json(400, ['error' => ['code' => 'INVALID_WEBHOOK', 'message' => 'GitHub delivery and event headers are required.']]);
            return;
        }
        try {
            $body = json_decode($rawBody, true, 512, JSON_THROW_ON_ERROR);
        } catch (JsonException) {
            $this->json(400, ['error' => ['code' => 'INVALID_WEBHOOK', 'message' => 'Webhook body must be valid JSON.']]);
            return;
        }
        if (!is_array($body)) {
            $this->json(400, ['error' => ['code' => 'INVALID_WEBHOOK', 'message' => 'Webhook body must be a JSON object.']]);
            return;
        }
        $installationID = filter_var($body['installation']['id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        if (!is_int($installationID)) {
            $this->json(202, ['accepted' => false, 'reason' => 'No installation identifier.']);
            return;
        }
        $instance = $this->database->findInstanceByInstallation($installationID);
        if ($instance === null) {
            $this->json(202, ['accepted' => false, 'reason' => 'Installation is not paired to a ReleaseStation instance.']);
            return;
        }
        $repository = is_array($body['repository'] ?? null) ? $body['repository'] : [];
        $event = [
            'instance_id' => $instance['id'],
            'delivery_id' => $deliveryID,
            'event_name' => $eventName,
            'action' => isset($body['action']) ? trim((string) $body['action']) : null,
            'github_installation_id' => $installationID,
            'repository_full_name' => isset($repository['full_name']) ? trim((string) $repository['full_name']) : null,
            'ref_name' => trim((string) ($body['ref'] ?? '')),
            'before_sha' => isset($body['before']) ? trim((string) $body['before']) : null,
            'after_sha' => isset($body['after']) ? trim((string) $body['after']) : null,
            'deleted' => (bool) ($body['deleted'] ?? false),
            'payload_json' => $rawBody,
        ];
        $accepted = $this->database->recordWebhookEvent($event);
        $this->json(202, ['accepted' => true, 'duplicate' => !$accepted, 'instance_id' => $instance['id']]);
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

    private function commits(string $instanceId, string $suffix): void
    {
        if (preg_match('#^([^/]+)/([^/]+)/commits$#', $suffix, $matches) !== 1) {
            $this->json(422, ['error' => ['code' => 'INVALID_REPOSITORY', 'message' => 'Repository must use owner/name format.']]);
            return;
        }
        $installationId = filter_var($_GET['installation_id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        $branch = trim((string) ($_GET['branch'] ?? ''));
        $perPage = filter_var($_GET['per_page'] ?? 30, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1, 'max_range' => 100]]);
        if (!is_int($installationId) || !$this->database->hasInstallation($instanceId, $installationId) || $branch === '') {
            $this->json(422, ['error' => ['code' => 'INVALID_COMMIT_REQUEST', 'message' => 'A connected installation and branch are required.']]);
            return;
        }
        $items = [];
        foreach ($this->github->commits($installationId, rawurldecode($matches[1]), rawurldecode($matches[2]), $branch, is_int($perPage) ? $perPage : 30) as $commit) {
            $commitAuthor = is_array($commit['commit']['author'] ?? null) ? $commit['commit']['author'] : [];
            $items[] = [
                'sha' => (string) ($commit['sha'] ?? ''),
                'message' => trim((string) ($commit['commit']['message'] ?? '')),
                'branch' => $branch,
                'author' => (string) ($commit['author']['login'] ?? $commitAuthor['name'] ?? ''),
                'url' => (string) ($commit['html_url'] ?? ''),
                'created_at' => (string) ($commitAuthor['date'] ?? ''),
            ];
        }
        $this->json(200, ['commits' => $items]);
    }

    private function commit(string $instanceId, string $suffix): void
    {
        if (preg_match('#^([^/]+)/([^/]+)/commit$#', $suffix, $matches) !== 1) {
            $this->json(422, ['error' => ['code' => 'INVALID_REPOSITORY', 'message' => 'Repository must use owner/name format.']]);
            return;
        }
        $installationId = filter_var($_GET['installation_id'] ?? null, FILTER_VALIDATE_INT, ['options' => ['min_range' => 1]]);
        $ref = trim((string) ($_GET['ref'] ?? ''));
        if (!is_int($installationId) || !$this->database->hasInstallation($instanceId, $installationId) || $ref === '') {
            $this->json(422, ['error' => ['code' => 'INVALID_COMMIT_REQUEST', 'message' => 'A connected installation and commit reference are required.']]);
            return;
        }
        $commit = $this->github->commit($installationId, rawurldecode($matches[1]), rawurldecode($matches[2]), $ref);
        $commitAuthor = is_array($commit['commit']['author'] ?? null) ? $commit['commit']['author'] : [];
        $this->json(200, [
            'sha' => (string) ($commit['sha'] ?? ''),
            'message' => trim((string) ($commit['commit']['message'] ?? '')),
            'branch' => $ref,
            'author' => (string) ($commit['author']['login'] ?? $commitAuthor['name'] ?? ''),
            'url' => (string) ($commit['html_url'] ?? ''),
            'created_at' => (string) ($commitAuthor['date'] ?? ''),
        ]);
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
        $this->lastResponseBody = '[binary response omitted; bytes=' . strlen($archive) . ']';
        echo $archive;
    }

    /** @return array<string,mixed> */
    private function requestBody(): array
    {
        $decoded = json_decode($this->rawRequestBody !== '' ? $this->rawRequestBody : '{}', true, 512, JSON_THROW_ON_ERROR);
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
        $this->lastResponseBody = json_encode($payload, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES | JSON_PRETTY_PRINT);
        echo $this->lastResponseBody;
    }

    private static function randomToken(int $bytes): string
    {
        return rtrim(strtr(base64_encode(random_bytes($bytes)), '+/', '-_'), '=');
    }
}
