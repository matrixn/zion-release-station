# Zion Connector

Zion Connector este serviciul PHP public care păstrează cheia privată a GitHub App și face legătura dintre ReleaseStation instalat pe Synology și GitHub. Este separat de SPK: NAS-ul primește doar un `instance_id`, un credential per instanță și URL-ul HTTPS al connectorului.

## Cerințe și instalare

- PHP 8.2+ cu `curl`, `openssl`, `pdo_mysql`;
- Composer 2;
- MariaDB 10.5+ sau MySQL 8;
- HTTPS reverse proxy în fața procesului PHP;
- o GitHub App publică, cu `Contents: Read-only` și `Metadata: Read-only`;
- cheia PEM montată ca secret, cu permisiuni `0600`.

```bash
cd ZionConnector
cp .env.example .env
composer install
php -S 127.0.0.1:8787 -t public public/index.php
```

În producție, rulează `public/index.php` prin PHP-FPM/Nginx sau Apache și expune numai reverse proxy-ul HTTPS. Setează `CONNECTOR_DB_DRIVER=mysql` și variabilele `CONNECTOR_DB_*`; schema este creată automat în MariaDB cu InnoDB și utf8mb4.

`.htaccess` din root este fallback pentru Apache/Web Station. Nginx nu citește `.htaccess`; folosește configurația din `deploy/nginx/connector.raduta.synology.me.conf` și setează document root-ul la `/volume1/www/connector.raduta.synology.me`.

## Provisionarea NAS-ului

`CONNECTOR_ADMIN_TOKEN` este folosit numai de administratorul serviciului:

```bash
curl -X POST https://connect.example.com/internal/instances \
  -H 'Authorization: Bearer ADMIN_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"instance_id":"nas-zion-001","license_id":"license-123","return_host":"nas.example.com"}'
```

Răspunsul conține `credential` o singură dată. Pe NAS se configurează:

```dotenv
RS_INSTANCE_ID=nas-zion-001
RS_GITHUB_CONNECTOR_URL=https://connect.example.com
RS_GITHUB_CONNECTOR_TOKEN=credential-returned-by-provisioning
RS_PUBLIC_URL=https://nas.example.com
```

Nu pune credentialul în Git și nu îl introduce în interfața utilizatorului final.

## Contract HTTP

- `POST /v1/instances/{id}/github/sessions` — creează sesiunea GitHub;
- `GET /github/callback` — validează OAuth `state`, verifică installation-ul și redirecționează în ReleaseStation;
- `GET /v1/instances/{id}/github/status` — statusul conexiunii;
- `GET /v1/instances/{id}/github/repositories` — repository-uri publice și private accesibile;
- `GET /v1/instances/{id}/github/repositories/{owner}/{repo}/branches` — branch-uri;
- `GET /healthz` — health check fără secrete.

Endpoint-urile `/v1/instances/...` cer `Authorization: Bearer <credential>` și verifică asocierea credentialului cu `instance_id`. Cheia GitHub App și tokenurile temporare rămân pe connector.

Fluxul OAuth folosește `state` aleator, hash-uit și single-use, cu expirare de 10 minute. Callback-ul verifică utilizatorul și installation-ul prin GitHub înainte să salveze asocierea cu instanța. `return_url` acceptă doar HTTPS și hosturile configurate în `CONNECTOR_RETURN_HOSTS`.

## Configurarea GitHub App

- slug-ul trebuie să corespundă cu `CONNECTOR_GITHUB_APP_SLUG`;
- user authorization la instalare: activat;
- callback URL: `https://connect.example.com/github/callback`;
- repository permissions: `Contents: Read-only`, `Metadata: Read-only`;
- private key-ul se montează din secret manager, de exemplu `/run/secrets/zion-github-app.pem`.

Nu salva PEM-ul în acest repository și nu îl returna către NAS.

## Testare

```bash
composer lint
composer test
```

SQLite rămâne disponibil numai pentru teste locale. Producția folosește MariaDB prin PDO MySQL.

## Deploy pe Synology

Din rădăcina repository-ului:

```bash
chmod +x scripts/deploy-connector-nas.sh
./scripts/deploy-connector-nas.sh
```

Scriptul arhivează și trimite exclusiv conținutul `./ZionConnector`, inclusiv `.env` și `key/github-private-key.pem`, către `/volume1/www/connector.raduta.synology.me/`. Nu trimite restul repository-ului, ZIP-uri, baza de date persistentă sau cache-uri.
