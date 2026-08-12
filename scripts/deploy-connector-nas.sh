#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
PROJECT_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
CONNECTOR_DIR="$PROJECT_ROOT/ZionConnector"
CONFIG_FILE="${RS_NAS_CONFIG:-$PROJECT_ROOT/.env.nas}"

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Missing $CONFIG_FILE; create it from .env.nas.example before deploying." >&2
    exit 1
fi
# shellcheck disable=SC1090
source "$CONFIG_FILE"

NAS_HOST="${NAS_HOST:-192.168.0.10}"
NAS_PORT="${NAS_PORT:-2022}"
NAS_USER="${NAS_USER:-wordpress-deploy}"
NAS_IDENTITY_FILE="${NAS_IDENTITY_FILE:-/home/matrixn/.ssh/wordpress-plugin-deploy}"
NAS_SUDO="${NAS_SUDO:-sudo}"
NAS_SUDO_PASS="${NAS_SUDO_PASS:-}"
REMOTE_PATH="${CONNECTOR_REMOTE_PATH:-/volume1/www/connector.raduta.synology.me}"

if [[ ! -d "$CONNECTOR_DIR" ]]; then
    echo "Connector directory not found: $CONNECTOR_DIR" >&2
    exit 1
fi
if [[ ! -f "$CONNECTOR_DIR/.env" ]]; then
    echo "Missing ZionConnector/.env; refusing to deploy without runtime configuration." >&2
    exit 1
fi
if [[ ! -f "$CONNECTOR_DIR/key/github-private-key.pem" ]]; then
    echo "Missing ZionConnector/key/github-private-key.pem; refusing to deploy without the GitHub App key." >&2
    exit 1
fi
if [[ ! -f "$CONNECTOR_DIR/vendor/autoload.php" ]]; then
    echo "Missing ZionConnector/vendor/autoload.php; run composer install in ZionConnector first." >&2
    exit 1
fi
for required_name in CONNECTOR_DB_DRIVER CONNECTOR_DB_HOST CONNECTOR_DB_PORT CONNECTOR_DB_NAME CONNECTOR_DB_USER CONNECTOR_DB_PASSWORD; do
    if ! awk -F= -v key="$required_name" '$1 == key && $2 != "" { found = 1 } END { exit found ? 0 : 1 }' "$CONNECTOR_DIR/.env"; then
        echo "Missing non-empty $required_name in ZionConnector/.env; refusing to deploy an incomplete MariaDB configuration." >&2
        exit 1
    fi
done
if ! awk -F= '$1 == "CONNECTOR_DB_DRIVER" && ($2 == "mysql" || $2 == "mariadb") { found = 1 } END { exit found ? 0 : 1 }' "$CONNECTOR_DIR/.env"; then
    echo "CONNECTOR_DB_DRIVER must be mysql or mariadb in ZionConnector/.env." >&2
    exit 1
fi
if [[ ! "$REMOTE_PATH" =~ ^/volume1/www/[A-Za-z0-9._/-]+$ ]]; then
    echo "Unsafe connector remote path: $REMOTE_PATH" >&2
    exit 1
fi

chmod 600 "$CONNECTOR_DIR/.env" "$CONNECTOR_DIR/key/github-private-key.pem"

SSH_OPTIONS=(
    -p "$NAS_PORT"
    -i "$NAS_IDENTITY_FILE"
    -o BatchMode=yes
    -o StrictHostKeyChecking=yes
)
[[ -r "$NAS_IDENTITY_FILE" ]] || { echo "NAS identity file is not readable: $NAS_IDENTITY_FILE" >&2; exit 1; }
REMOTE="$NAS_USER@$NAS_HOST"
REMOTE_STAGE="/tmp/.zion-connector-deploy-$$"

echo "Deploying only $CONNECTOR_DIR"
echo "Target: $REMOTE:$REMOTE_PATH"

cleanup() {
    ssh "${SSH_OPTIONS[@]}" "$REMOTE" "rm -rf '$REMOTE_STAGE'" >/dev/null 2>&1 || true
}
trap cleanup EXIT

ssh "${SSH_OPTIONS[@]}" "$REMOTE" "mkdir -p '$REMOTE_STAGE' '$REMOTE_PATH'"

# The archive is created from ZionConnector only. .env and the PEM are
# intentionally included; local ZIP files, caches and the persistent DB are not.
tar \
    --exclude='./var' \
    --exclude='./var/*' \
    --exclude='./*.zip' \
    --exclude='./.phpunit.result.cache' \
    -C "$CONNECTOR_DIR" -czf - . \
    | ssh "${SSH_OPTIONS[@]}" "$REMOTE" "tar -xzf - -C '$REMOTE_STAGE'"

run_remote_sudo() {
    local remote_script="$1"
    if [[ -n "$NAS_SUDO_PASS" ]]; then
        {
            printf '%s\n' "$NAS_SUDO_PASS"
            printf '%s\n' "$remote_script"
        } | ssh "${SSH_OPTIONS[@]}" "$REMOTE" "$NAS_SUDO -S -p '' sh -s -- '$REMOTE_STAGE' '$REMOTE_PATH'"
        return
    fi
    printf '%s\n' "$remote_script" | ssh -tt "${SSH_OPTIONS[@]}" "$REMOTE" "$NAS_SUDO sh -s -- '$REMOTE_STAGE' '$REMOTE_PATH'"
}

run_remote_sudo '
set -eu
stage="$1"
target="$2"
mkdir -p "$target/var" "$target/key"
# Do not preserve source owner/mode metadata on the Synology volume.
cp -R "$stage"/. "$target"/
chmod 600 "$target/.env" "$target/key/github-private-key.pem"
chmod 700 "$target/var"
rm -rf "$stage"
'

echo "Connector deployed to $REMOTE_PATH"
