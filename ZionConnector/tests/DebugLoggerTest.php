<?php

declare(strict_types=1);

namespace ZionConnector\Tests;

use PHPUnit\Framework\TestCase;
use ZionConnector\Database;
use ZionConnector\DebugLogger;

final class DebugLoggerTest extends TestCase
{
    public function testDebugEntriesArePersistedWithSensitiveValuesRedacted(): void
    {
        $path = tempnam(sys_get_temp_dir(), 'zion-debug-');
        try {
            $database = new Database($path);
            $database->migrate();
            $logger = new DebugLogger($database, true);
            $logger->logInbound(
                'request-001',
                'POST',
                '/v1/instances/demo/github/sessions',
                ['Authorization' => 'Bearer private', 'Content-Type' => 'application/json'],
                '{"access_token":"private","return_url":"https://nas.example.com/releasestation/"}',
                200,
                ['Content-Type' => 'application/json'],
                '{"state":"ok"}',
                12,
            );

            $result = $database->debugLogs(['source' => 'synology'], 1, 50);
            self::assertSame(1, $result['total']);
            self::assertSame('[REDACTED]', json_decode($result['items'][0]['request_headers_json'], true)['Authorization']);
            self::assertStringContainsString('[REDACTED]', $result['items'][0]['request_body']);
            self::assertStringNotContainsString('private', $result['items'][0]['request_body']);
        } finally {
            @unlink($path);
        }
    }

    public function testDebugEntriesExposeRepositoryAndReleaseStationContext(): void
    {
        $path = tempnam(sys_get_temp_dir(), 'zion-debug-context-');
        try {
            $database = new Database($path);
            $database->migrate();
            $logger = new DebugLogger($database, true);
            $logger->logOutbound(
                'request-002',
                'GET',
                'https://api.github.com/repos/matrixn/zion-release-station/commits',
                [
                    'Authorization: Bearer private',
                    'X-ReleaseStation-Site-ID: site_test',
                    'X-ReleaseStation-Site: test.com',
                    'X-ReleaseStation-Site-URL: https://test.com',
                    'X-ReleaseStation-Repository: matrixn/zion-release-station',
                ],
                null,
                200,
                ['Content-Type: application/json'],
                '{"ok":true}',
                8,
            );

            $item = $database->debugLogs([], 1, 1)['items'][0];
            self::assertSame('matrixn/zion-release-station', $item['github_repository']);
            self::assertSame('site_test', $item['release_station_site_id']);
            self::assertSame('test.com', $item['release_station_site']);
            self::assertSame('https://test.com', $item['release_station_url']);
        } finally {
            @unlink($path);
        }
    }
}
