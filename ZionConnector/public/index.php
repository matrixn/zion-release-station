<?php

declare(strict_types=1);

require dirname(__DIR__) . '/vendor/autoload.php';

use ZionConnector\Config;
use ZionConnector\Database;
use ZionConnector\Environment;
use ZionConnector\GitHubClient;
use ZionConnector\HttpApp;

Environment::load(dirname(__DIR__) . '/.env');
$config = Config::fromEnvironment();
$database = new Database($config->databaseDsn(), $config->databaseUser, $config->databasePassword);
$database->migrate();
$github = new GitHubClient($config);
(new HttpApp($config, $database, $github))->run();
