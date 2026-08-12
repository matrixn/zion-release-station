<?php

declare(strict_types=1);

namespace ZionConnector\Tests;

use PHPUnit\Framework\TestCase;
use ZionConnector\Environment;

final class EnvironmentTest extends TestCase
{
    private string $path;

    protected function setUp(): void
    {
        $path = tempnam(sys_get_temp_dir(), 'zion-env-');
        self::assertIsString($path);
        $this->path = $path;
    }

    protected function tearDown(): void
    {
        putenv('ZION_CONNECTOR_ENV_TEST');
        if (isset($this->path)) {
            @unlink($this->path);
        }
    }

    public function testEmptyProcessVariableFallsBackToEnvFile(): void
    {
        file_put_contents($this->path, "ZION_CONNECTOR_ENV_TEST=from-file\n");
        putenv('ZION_CONNECTOR_ENV_TEST=');

        Environment::load($this->path);

        self::assertSame('from-file', getenv('ZION_CONNECTOR_ENV_TEST'));
    }

    public function testNonEmptyProcessVariableRemainsAuthoritative(): void
    {
        file_put_contents($this->path, "ZION_CONNECTOR_ENV_TEST=from-file\n");
        putenv('ZION_CONNECTOR_ENV_TEST=from-process');

        Environment::load($this->path);

        self::assertSame('from-process', getenv('ZION_CONNECTOR_ENV_TEST'));
    }
}
