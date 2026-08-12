<?php

declare(strict_types=1);

namespace ZionConnector\Tests;

use PHPUnit\Framework\TestCase;
use ZionConnector\Config;

final class ConfigTest extends TestCase
{
    protected function setUp(): void
    {
        putenv('CONNECTOR_PUBLIC_BASE_URL=https://connect.example.com');
        putenv('CONNECTOR_ADMIN_TOKEN=admin-secret');
        putenv('CONNECTOR_DATABASE_PATH=' . sys_get_temp_dir() . '/zion-config-test.sqlite');
        putenv('CONNECTOR_GITHUB_APP_ID=123');
        putenv('CONNECTOR_GITHUB_APP_SLUG=zion');
        putenv('CONNECTOR_GITHUB_CLIENT_ID=client');
        putenv('CONNECTOR_GITHUB_CLIENT_SECRET=secret');
        putenv('CONNECTOR_GITHUB_PRIVATE_KEY_PATH=/does/not/exist');
        putenv('CONNECTOR_RETURN_HOSTS=nas.example.com');
    }

    public function testOnlyConfiguredHttpsReturnHostsAreAccepted(): void
    {
        $config = Config::fromEnvironment();

        self::assertTrue($config->isAllowedReturnUrl('https://nas.example.com/releasestation/?github=connected', 'nas.example.com'));
        self::assertFalse($config->isAllowedReturnUrl('http://nas.example.com/releasestation/', 'nas.example.com'));
        self::assertFalse($config->isAllowedReturnUrl('https://attacker.example/releasestation/', 'nas.example.com'));
    }

    public function testMariaDbDsnUsesConfiguredConnectionValues(): void
    {
        putenv('CONNECTOR_DB_DRIVER=mysql');
        putenv('CONNECTOR_DB_HOST=mariadb.internal');
        putenv('CONNECTOR_DB_PORT=3307');
        putenv('CONNECTOR_DB_NAME=connector');

        $config = Config::fromEnvironment();

        self::assertSame('mysql:host=mariadb.internal;port=3307;dbname=connector;charset=utf8mb4', $config->databaseDsn());
    }
}
