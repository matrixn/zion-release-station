#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${RS_NAS_CONFIG:-$repo_root/.env.nas}"
NAS_PORT="${NAS_PORT:-22}"
NAS_PACKAGE="${NAS_PACKAGE:-zion-releasestation}"
NAS_IDENTITY_FILE="${NAS_IDENTITY_FILE:-}"
ssh_opts=(-p "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes)
if [[ -n "$NAS_IDENTITY_FILE" ]]; then
  [[ -r "$NAS_IDENTITY_FILE" ]] || { printf 'NAS identity file is not readable: %s\n' "$NAS_IDENTITY_FILE" >&2; exit 1; }
  ssh_opts+=(-i "$NAS_IDENTITY_FILE" -o IdentitiesOnly=yes)
fi
ssh "${ssh_opts[@]}" "$NAS_USER@$NAS_HOST" "tail -n 160 /var/log/packages/$NAS_PACKAGE.log 2>/dev/null || true; tail -n 160 /var/log/synopkg.log 2>/dev/null || true"
