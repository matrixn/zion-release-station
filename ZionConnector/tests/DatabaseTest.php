<?php

declare(strict_types=1);

namespace ZionConnector\Tests;

use PHPUnit\Framework\TestCase;
use ZionConnector\Database;

final class DatabaseTest extends TestCase
{
    private string $path;

    protected function setUp(): void
    {
        $this->path = tempnam(sys_get_temp_dir(), 'zion-connector-');
    }

    protected function tearDown(): void
    {
        @unlink($this->path);
    }

    public function testSessionStateIsSingleUseAndCredentialsAreNotStoredPlaintext(): void
    {
        $database = new Database($this->path);
        $database->migrate();
        $instance = $database->createInstance('nas-001', 'lic-001', hash('sha256', 'secret'), 'nas.example.com');

        self::assertSame('nas-001', $instance['id']);
        self::assertSame('nas-001', $database->findInstance('nas-001')['id']);

        $database->createSession('session-001', 'nas-001', hash('sha256', 'state'), 'https://nas.example.com/releasestation/', gmdate('c', time() + 600));
        self::assertNotNull($database->consumeSession(hash('sha256', 'state')));
        self::assertNull($database->consumeSession(hash('sha256', 'state')));
    }

    public function testPairingSessionIsAuthorizedAndConsumedOnce(): void
    {
        $database = new Database($this->path);
        $database->migrate();
        $stateHash = hash('sha256', 'pairing-state');
        $database->createPairingSession(
            'pairing-001',
            'nas-002',
            $stateHash,
            'https://nas.example.com/releasestation/?github=connected',
            gmdate('c', time() + 600),
        );

        self::assertSame('pending', $database->findPairingSession($stateHash)['status']);
        $database->authorizePairingSession('pairing-001', 123456);
        $session = $database->consumePairingSession('nas-002', $stateHash);

        self::assertNotNull($session);
        self::assertSame(123456, (int) $session['github_installation_id']);
        self::assertNull($database->consumePairingSession('nas-002', $stateHash));
    }
}
