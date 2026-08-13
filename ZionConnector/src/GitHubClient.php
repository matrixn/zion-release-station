<?php

declare(strict_types=1);

namespace ZionConnector;

use RuntimeException;

final class GitHubClient
{
    public function __construct(private readonly Config $config)
    {
    }

    public function authorizationUrl(string $state): string
    {
        $query = http_build_query(['state' => $state], '', '&', PHP_QUERY_RFC3986);
        return 'https://github.com/apps/' . rawurlencode($this->config->githubAppSlug) . '/installations/new?' . $query;
    }

    /** @return array<string,mixed> */
    public function exchangeCode(string $code): array
    {
        return $this->request('POST', 'https://github.com/login/oauth/access_token', [
            'client_id' => $this->config->githubClientId,
            'client_secret' => $this->config->githubClientSecret,
            'code' => $code,
            'redirect_uri' => $this->config->callbackUrl(),
        ], ['Accept: application/json']);
    }

    /** @return array<string,mixed> */
    public function authenticatedUser(string $accessToken): array
    {
        return $this->request('GET', 'https://api.github.com/user', null, $this->userHeaders($accessToken));
    }

    /** @return array<string,mixed> */
    public function userInstallation(string $accessToken, int $installationId): array
    {
        return $this->request(
            'GET',
            'https://api.github.com/user/installations/' . $installationId,
            null,
            $this->userHeaders($accessToken),
        );
    }

    /** @return array<string,mixed> */
    public function appInstallation(int $installationId): array
    {
        return $this->request(
            'GET',
            'https://api.github.com/app/installations/' . $installationId,
            null,
            $this->appHeaders(),
        );
    }

    /** @return list<array<string,mixed>> */
    public function repositories(int $installationId): array
    {
        $token = $this->installationToken($installationId);
        $repositories = [];
        for ($page = 1; $page <= 100; $page++) {
            $response = $this->request(
                'GET',
                'https://api.github.com/installation/repositories?per_page=100&page=' . $page,
                null,
                $this->installationHeaders($token),
            );
            $items = $response['repositories'] ?? [];
            if (!is_array($items)) {
                throw new RuntimeException('GitHub returned an invalid repository list.');
            }
            foreach ($items as $repository) {
                if (is_array($repository)) {
                    $repositories[] = $repository;
                }
            }
            if (count($items) < 100) {
                break;
            }
        }
        return $repositories;
    }

    /** @return list<string> */
    public function branches(int $installationId, string $owner, string $repo): array
    {
        foreach ($this->repositories($installationId) as $repository) {
            if (($repository['full_name'] ?? '') === $owner . '/' . $repo) {
                $token = $this->installationToken($installationId);
                $branches = [];
                for ($page = 1; $page <= 100; $page++) {
                    $items = $this->request(
                        'GET',
                        'https://api.github.com/repos/' . rawurlencode($owner) . '/' . rawurlencode($repo) . '/branches?per_page=100&page=' . $page,
                        null,
                        $this->installationHeaders($token),
                    );
                    if (!is_array($items)) {
                        throw new RuntimeException('GitHub returned an invalid branch list.');
                    }
                    foreach ($items as $branch) {
                        if (is_array($branch) && isset($branch['name']) && is_string($branch['name'])) {
                            $branches[] = $branch['name'];
                        }
                    }
                    if (count($items) < 100) {
                        break;
                    }
                }
                return $branches;
            }
        }
        throw new RuntimeException('Repository is not accessible by this GitHub installation.');
    }

    /** @return list<array<string,mixed>> */
    public function commits(int $installationId, string $owner, string $repo, string $branch, int $perPage = 30): array
    {
        $token = $this->installationToken($installationId);
        $query = http_build_query(['sha' => $branch, 'per_page' => min(100, max(1, $perPage))], '', '&', PHP_QUERY_RFC3986);
        $items = $this->request(
            'GET',
            'https://api.github.com/repos/' . rawurlencode($owner) . '/' . rawurlencode($repo) . '/commits?' . $query,
            null,
            $this->installationHeaders($token),
        );
        if (!is_array($items)) {
            throw new RuntimeException('GitHub returned an invalid commit list.');
        }
        return array_values(array_filter($items, static fn (mixed $item): bool => is_array($item)));
    }

    /** @return array<string,mixed> */
    public function commit(int $installationId, string $owner, string $repo, string $ref): array
    {
        $token = $this->installationToken($installationId);
        $item = $this->request(
            'GET',
            'https://api.github.com/repos/' . rawurlencode($owner) . '/' . rawurlencode($repo) . '/commits/' . rawurlencode($ref),
            null,
            $this->installationHeaders($token),
        );
        if (!is_array($item)) {
            throw new RuntimeException('GitHub returned an invalid commit.');
        }
        return $item;
    }

    /** @return string GitHub tar.gz archive bytes */
    public function archive(int $installationId, string $owner, string $repo, string $ref): string
    {
        $repositoryName = $owner . '/' . $repo;
        $accessible = false;
        foreach ($this->repositories($installationId) as $repository) {
            if (($repository['full_name'] ?? '') === $repositoryName) {
                $accessible = true;
                break;
            }
        }
        if (!$accessible) {
            throw new RuntimeException('Repository is not accessible by this GitHub installation.');
        }
        $token = $this->installationToken($installationId);
        return $this->binaryRequest(
            'GET',
            'https://api.github.com/repos/' . rawurlencode($owner) . '/' . rawurlencode($repo) . '/tarball/' . rawurlencode($ref),
            $this->installationHeaders($token),
        );
    }

    /** @return array<string,mixed> */
    private function installationToken(int $installationId): array
    {
        return $this->request(
            'POST',
            'https://api.github.com/app/installations/' . $installationId . '/access_tokens',
            null,
            $this->appHeaders(),
        );
    }

    /** @return array<string,string> */
    private function appHeaders(): array
    {
        return [
            'Authorization: Bearer ' . $this->appJwt(),
            'Accept: application/vnd.github+json',
        ];
    }

    /** @return list<string> */
    private function userHeaders(string $token): array
    {
        return ['Authorization: Bearer ' . $token, 'Accept: application/vnd.github+json'];
    }

    /** @return list<string> */
    private function installationHeaders(array $token): array
    {
        if (!isset($token['token']) || !is_string($token['token'])) {
            throw new RuntimeException('GitHub did not return an installation token.');
        }
        return ['Authorization: Bearer ' . $token['token'], 'Accept: application/vnd.github+json'];
    }

    private function appJwt(): string
    {
        $key = file_get_contents($this->config->githubPrivateKeyPath);
        if ($key === false) {
            throw new RuntimeException('Unable to read the GitHub App private key.');
        }
        $header = self::base64Url(json_encode(['alg' => 'RS256', 'typ' => 'JWT'], JSON_THROW_ON_ERROR));
        $now = time();
        $payload = self::base64Url(json_encode([
            'iat' => $now - 60,
            'exp' => $now + 540,
            'iss' => $this->config->githubAppId,
        ], JSON_THROW_ON_ERROR));
        $signingInput = $header . '.' . $payload;
        if (!openssl_sign($signingInput, $signature, $key, OPENSSL_ALGO_SHA256)) {
            throw new RuntimeException('Unable to sign a GitHub App JWT.');
        }
        return $signingInput . '.' . self::base64Url($signature);
    }

    /** @param array<string,mixed>|null $body @param list<string> $headers @return array<string,mixed> */
    private function request(string $method, string $url, ?array $body, array $headers): array
    {
        $handle = curl_init($url);
        if ($handle === false) {
            throw new RuntimeException('Unable to initialize the GitHub HTTP client.');
        }
        $requestHeaders = array_merge(['User-Agent: ZionConnector/1.0', 'X-GitHub-Api-Version: 2022-11-28'], $headers);
        $options = [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_FOLLOWLOCATION => false,
            CURLOPT_CONNECTTIMEOUT => 10,
            CURLOPT_TIMEOUT => 30,
            CURLOPT_HTTPHEADER => $requestHeaders,
            CURLOPT_CUSTOMREQUEST => $method,
        ];
        if ($body !== null) {
            $options[CURLOPT_POSTFIELDS] = http_build_query($body, '', '&', PHP_QUERY_RFC3986);
            $options[CURLOPT_HTTPHEADER][] = 'Content-Type: application/x-www-form-urlencoded';
        }
        curl_setopt_array($handle, $options);
        $response = curl_exec($handle);
        $status = (int) curl_getinfo($handle, CURLINFO_RESPONSE_CODE);
        $error = curl_error($handle);
        curl_close($handle);
        if ($response === false) {
            throw new RuntimeException('GitHub request failed: ' . $error);
        }
        try {
            $decoded = json_decode($response, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $exception) {
            throw new RuntimeException('GitHub returned invalid JSON.', 0, $exception);
        }
        if ($status < 200 || $status >= 300 || !is_array($decoded)) {
            $message = is_array($decoded) && isset($decoded['message']) ? (string) $decoded['message'] : 'GitHub request was rejected.';
            throw new RuntimeException('GitHub HTTP ' . $status . ': ' . $message);
        }
        return $decoded;
    }

    /** @param list<string> $headers */
    private function binaryRequest(string $method, string $url, array $headers): string
    {
        $handle = curl_init($url);
        if ($handle === false) {
            throw new RuntimeException('Unable to initialize the GitHub HTTP client.');
        }
        curl_setopt_array($handle, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_FOLLOWLOCATION => true,
            CURLOPT_CONNECTTIMEOUT => 10,
            CURLOPT_TIMEOUT => 120,
            CURLOPT_HTTPHEADER => array_merge(['User-Agent: ZionConnector/1.0', 'X-GitHub-Api-Version: 2022-11-28'], $headers),
            CURLOPT_CUSTOMREQUEST => $method,
        ]);
        $response = curl_exec($handle);
        $status = (int) curl_getinfo($handle, CURLINFO_RESPONSE_CODE);
        $error = curl_error($handle);
        curl_close($handle);
        if ($response === false) {
            throw new RuntimeException('GitHub archive request failed: ' . $error);
        }
        if ($status < 200 || $status >= 300) {
            throw new RuntimeException('GitHub archive request was rejected with HTTP ' . $status . '.');
        }
        if (strlen($response) > 512 * 1024 * 1024) {
            throw new RuntimeException('GitHub archive exceeds the 512 MB safety limit.');
        }
        return $response;
    }

    private static function base64Url(string $value): string
    {
        return rtrim(strtr(base64_encode($value), '+/', '-_'), '=');
    }
}
