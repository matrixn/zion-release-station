#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${RS_NAS_CONFIG:-$repo_root/.env.nas}"
NAS_PORT="${NAS_PORT:-22}"
health_url="${NAS_HEALTH_URL:-http://127.0.0.1:24871/releasestation/api/v1/system/health}"
NAS_IDENTITY_FILE="${NAS_IDENTITY_FILE:-}"
ssh_opts=(-p "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes)
if [[ -n "$NAS_IDENTITY_FILE" ]]; then
  [[ -r "$NAS_IDENTITY_FILE" ]] || { printf 'NAS identity file is not readable: %s\n' "$NAS_IDENTITY_FILE" >&2; exit 1; }
  ssh_opts+=(-i "$NAS_IDENTITY_FILE" -o IdentitiesOnly=yes)
fi
ssh "${ssh_opts[@]}" "$NAS_USER@$NAS_HOST" "curl --fail --silent --show-error '$health_url'"
printf '\nNAS health check passed.\n'
