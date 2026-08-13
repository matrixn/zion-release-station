<?php

declare(strict_types=1);

require dirname(__DIR__) . '/vendor/autoload.php';

use ZionConnector\Config;
use ZionConnector\Database;
use ZionConnector\DebugLogger;
use ZionConnector\Environment;
use ZionConnector\GitHubClient;
use ZionConnector\HttpApp;

Environment::load(dirname(__DIR__) . '/.env');
$config = Config::fromEnvironment();
$database = new Database($config->databaseDsn(), $config->databaseUser, $config->databasePassword);
$database->migrate();
$debug = new DebugLogger($database, $config->debug);
$github = new GitHubClient($config, $debug);
(new HttpApp($config, $database, $github, $debug))->run();
