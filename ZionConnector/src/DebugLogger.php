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
        $this->record([
            'request_id' => $requestId,
            'direction' => 'inbound',
            'source' => $this->sourceForPath($url),
            'method' => $method,
            'url' => $url,
            'status' => $status,
            'request_headers_json' => $this->encode($this->redactHeaders($headers)),
            'request_body' => $this->redactBody($requestBody),
            'response_headers_json' => $this->encode($this->redactHeaders($responseHeaders)),
            'response_body' => $this->redactBody($responseBody),
            'duration_ms' => $durationMs,
            'created_at' => gmdate('c'),
        ]);
    }

    /** @param list<string> $requestHeaders @param list<string> $responseHeaders */
    public function logOutbound(string $requestId, string $method, string $url, array $requestHeaders, ?string $requestBody, int $status, array $responseHeaders, ?string $responseBody, int $durationMs, string $source = 'github'): void
    {
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
            'created_at' => gmdate('c'),
        ]);
    }

    public function sourceForPath(string $path): string
    {
        return str_starts_with($path, '/github/') ? 'github' : 'synology';
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

    private function truncate(string $value): string
    {
        if (strlen($value) <= self::MAX_BODY_BYTES) {
            return $value;
        }
        return substr($value, 0, self::MAX_BODY_BYTES) . "\n… [truncated by debug console]";
    }
}
