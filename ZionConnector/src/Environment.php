<?php

declare(strict_types=1);

namespace ZionConnector;

final class Environment
{
    public static function load(string $path): void
    {
        if (!is_readable($path)) {
            return;
        }
        foreach (file($path, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) ?: [] as $line) {
            $line = trim($line);
            if ($line === '' || str_starts_with($line, '#') || !str_contains($line, '=')) {
                continue;
            }
            [$name, $value] = explode('=', $line, 2);
            $name = trim($name);
            $value = trim($value);
            // Web Station may expose an empty variable from the PHP-FPM
            // profile. Treat that as unset so the connector can load the
            // configured value from its protected .env file. A non-empty
            // process environment variable still takes precedence.
            $existing = getenv($name);
            if ($name === '' || ($existing !== false && $existing !== '')) {
                continue;
            }
            if (strlen($value) >= 2 && (($value[0] === '"' && str_ends_with($value, '"')) || ($value[0] === "'" && str_ends_with($value, "'")))) {
                $value = substr($value, 1, -1);
            }
            putenv($name . '=' . $value);
        }
    }
}
