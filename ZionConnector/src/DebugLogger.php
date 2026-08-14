<?php

declare(strict_types=1);

namespace ZionConnector;

use JsonException;

final class DebugLogger
{
    private const MAX_BODY_BYTES = 131072;

    public function __construct(
        private readonly Database $database,
        private readonly bool $enabled,
    ) {
    }

    public function enabled(): bool
    {
        return $this->enabled;
    }

    public function requestId(): string
    {
        return gmdate('YmdHis') . '_' . bin2hex(random_bytes(6));
    }

    /** @return array<string,string> */
    public function serverHeaders(array $server): array
    {
        $headers = [];
        foreach ($server as $key => $value) {
            if (str_starts_with($key, 'HTTP_')) {
                $name = str_replace('_', '-', substr($key, 5));
                $headers[$name] = (string) $value;
            }
        }
        if (isset($server['CONTENT_TYPE'])) {
            $headers['Content-Type'] = (string) $server['CONTENT_TYPE'];
        }
        if (isset($server['CONTENT_LENGTH'])) {
            $headers['Content-Length'] = (string) $server['CONTENT_LENGTH'];
        }
        return $this->redactHeaders($headers);
    }

    /** @param list<string> $headers @return array<string,string> */
    public function curlHeaders(array $headers): array
    {
        $result = [];
        foreach ($headers as $header) {
            $parts = explode(':', $header, 2);
            if (count($parts) === 2) {
                $result[trim($parts[0])] = trim($parts[1]);
            }
        }
        return $this->redactHeaders($result);
    }

    /** @param list<string> $headers @return array<string,string> */
    public function responseHeaders(array $headers): array
    {
        $result = [];
        foreach ($headers as $header) {
            $parts = explode(':', $header, 2);
            if (count($parts) === 2) {
                $result[trim($parts[0])] = trim($parts[1]);
            }
        }
        return $this->redactHeaders($result);
    }

    /** @param array<string,mixed> $headers */
    public function logInbound(string $requestId, string $method, string $url, array $headers, string $requestBody, int $status, array $responseHeaders, string $responseBody, int $durationMs): void
    {
        $context = $this->requestContext($url, $headers, $requestBody);
        $this->record([
            'request_id' => $requestId,
            'direction' => 'inbound',
            'source' => $this->sourceForPath($url),
            'method' => $method,
            'url' => $url,
            'status' => $status,
            'request_headers_json' => $this->encode($this->redactHeaders($headers)),
            'request_body' => $this->redactBody($requestBody),
            'response_headers_json' => $this->encode(array_is_list($responseHeaders) ? $this->responseHeaders(array_values(array_filter($responseHeaders, 'is_string'))) : $this->redactHeaders($responseHeaders)),
            'response_body' => $this->redactBody($responseBody),
            'duration_ms' => $durationMs,
            ...$context,
            'created_at' => gmdate('c'),
        ]);
    }

    /** @param list<string> $requestHeaders @param list<string> $responseHeaders */
    public function logOutbound(string $requestId, string $method, string $url, array $requestHeaders, ?string $requestBody, int $status, array $responseHeaders, ?string $responseBody, int $durationMs, string $source = 'github'): void
    {
        $context = $this->requestContext($url, $this->curlHeaders($requestHeaders), $requestBody ?? '');
        $this->record([
            'request_id' => $requestId,
            'direction' => 'outbound',
            'source' => $source,
            'method' => $method,
            'url' => $url,
            'status' => $status,
            'request_headers_json' => $this->encode($this->curlHeaders($requestHeaders)),
            'request_body' => $this->redactBody($requestBody ?? ''),
            'response_headers_json' => $this->encode($this->responseHeaders($responseHeaders)),
            'response_body' => $responseBody === null ? '[binary response omitted]' : $this->redactBody($responseBody),
            'duration_ms' => $durationMs,
            ...$context,
            'created_at' => gmdate('c'),
        ]);
    }

    public function sourceForPath(string $path): string
    {
        return str_starts_with($path, '/github/') ? 'github' : 'synology';
    }

    /**
     * Extract safe correlation data for the debug console. The SPK sends
     * explicit X-ReleaseStation-* headers for site-scoped requests; GitHub
     * webhook payloads and API URLs provide the repository when available.
     *
     * @param array<string,mixed> $headers
     * @return array<string,string|null>
     */
    public function requestContext(string $url, array $headers, string $body): array
    {
        $normalizedHeaders = [];
        foreach ($headers as $name => $value) {
            $normalizedHeaders[strtolower(trim((string) $name))] = trim((string) $value);
        }
        $payload = [];
        if ($body !== '') {
            try {
                $decoded = json_decode($body, true, 512, JSON_THROW_ON_ERROR);
                if (is_array($decoded)) {
                    $payload = $decoded;
                }
            } catch (JsonException) {
                // Context is best-effort; the expanded row still shows body.
            }
        }

        $site = is_array($payload['site'] ?? null) ? $payload['site'] : [];
        $repository = is_array($payload['repository'] ?? null) ? $payload['repository'] : [];
        $instanceID = $this->firstNonEmpty([
            $normalizedHeaders['x-releasestation-instance-id'] ?? '',
            $normalizedHeaders['x-release-station-instance-id'] ?? '',
            (string) ($payload['instance_id'] ?? ''),
            $this->instanceFromUrl($url),
        ]);
        $repositoryName = $this->normalizeRepository($this->firstNonEmpty([
            $normalizedHeaders['x-releasestation-repository'] ?? '',
            $normalizedHeaders['x-release-station-repository'] ?? '',
            (string) ($payload['github_full_name'] ?? ''),
            (string) ($payload['repository_full_name'] ?? ''),
            (string) ($repository['full_name'] ?? ''),
            $this->repositoryFromUrl($url),
        ]));
        $siteID = $this->firstNonEmpty([
            $normalizedHeaders['x-releasestation-site-id'] ?? '',
            $normalizedHeaders['x-release-station-site-id'] ?? '',
            (string) ($payload['site_id'] ?? ''),
            (string) ($site['id'] ?? ''),
        ]);
        $siteName = $this->firstNonEmpty([
            $normalizedHeaders['x-releasestation-site'] ?? '',
            $normalizedHeaders['x-release-station-site'] ?? '',
            (string) ($payload['site_name'] ?? ''),
            (string) ($site['name'] ?? ''),
        ]);
        $siteURL = $this->firstNonEmpty([
            $normalizedHeaders['x-releasestation-site-url'] ?? '',
            $normalizedHeaders['x-release-station-site-url'] ?? '',
            (string) ($payload['site_url'] ?? ''),
            (string) ($site['url'] ?? ''),
        ]);

        return [
            'github_repository' => $this->limitContext($repositoryName, 512),
            'release_station_instance_id' => $this->limitContext($instanceID, 128),
            'release_station_site_id' => $this->limitContext($siteID, 255),
            'release_station_site' => $this->limitContext($siteName, 255),
            'release_station_url' => $this->limitContext($siteURL, 2048),
        ];
    }

    /** @param array<string,mixed> $entry */
    private function record(array $entry): void
    {
        if (!$this->enabled) {
            return;
        }
        try {
            $this->database->recordDebugLog($entry);
        } catch (\Throwable $exception) {
            error_log('Zion Connector debug logger failed: ' . $exception->getMessage());
        }
    }

    /** @param array<string,mixed> $headers @return array<string,string> */
    private function redactHeaders(array $headers): array
    {
        $result = [];
        foreach ($headers as $name => $value) {
            $normalized = strtolower((string) $name);
            $result[(string) $name] = preg_match('/authorization|cookie|set-cookie|secret|token|api[-_]?key|signature/i', $normalized) === 1
                ? '[REDACTED]'
                : $this->truncate((string) $value);
        }
        return $result;
    }

    private function redactBody(string $body): string
    {
        if ($body === '') {
            return '';
        }
        try {
            $decoded = json_decode($body, true, 512, JSON_THROW_ON_ERROR);
            return $this->encode($this->redactValue($decoded));
        } catch (JsonException) {
            $redacted = preg_replace_callback('/((?:client[_-]?secret|access[_-]?token|refresh[_-]?token|password|secret|token|code|authorization)=)([^&\s]+)/i', static fn (array $match): string => $match[1] . '[REDACTED]', $body);
            return $this->truncate($redacted ?? $body);
        }
    }

    private function redactValue(mixed $value): mixed
    {
        if (is_array($value)) {
            $result = [];
            foreach ($value as $key => $item) {
                $result[$key] = is_string($key) && preg_match('/password|secret|token|authorization|client[_-]?secret|private[_-]?key|access[_-]?token|refresh[_-]?token|pairing[_-]?code|code/i', $key) === 1
                    ? '[REDACTED]'
                    : $this->redactValue($item);
            }
            return $result;
        }
        return is_string($value) ? $this->truncate($value) : $value;
    }

    private function encode(mixed $value): string
    {
        try {
            return json_encode($value, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES | JSON_PRETTY_PRINT);
        } catch (JsonException) {
            return '[unserializable]';
        }
    }

    /** @param list<string> $values */
    private function firstNonEmpty(array $values): string
    {
        foreach ($values as $value) {
            $value = trim($value);
            if ($value !== '') {
                return $value;
            }
        }
        return '';
    }

    private function instanceFromUrl(string $url): string
    {
        $path = (string) (parse_url($url, PHP_URL_PATH) ?? $url);
        return preg_match('#/v1/instances/([^/]+)(?:/|$)#', $path, $matches) === 1
            ? rawurldecode($matches[1])
            : '';
    }

    private function repositoryFromUrl(string $url): string
    {
        $path = (string) (parse_url($url, PHP_URL_PATH) ?? $url);
        if (preg_match('#/(?:repos|repositories)/([^/]+)/([^/?]+)#', $path, $matches) !== 1) {
            return '';
        }
        return rawurldecode($matches[1]) . '/' . rawurldecode($matches[2]);
    }

    private function normalizeRepository(string $value): string
    {
        $value = trim(rawurldecode($value));
        $value = preg_replace('#\.git$#i', '', $value) ?? $value;
        return preg_match('#^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$#', $value) === 1 ? $value : '';
    }

    private function limitContext(string $value, int $maxBytes): ?string
    {
        $value = trim($value);
        return $value === '' ? null : substr($value, 0, $maxBytes);
    }

    private function truncate(string $value): string
    {
        if (strlen($value) <= self::MAX_BODY_BYTES) {
            return $value;
        }
        return substr($value, 0, self::MAX_BODY_BYTES) . "\n… [truncated by debug console]";
    }
}
