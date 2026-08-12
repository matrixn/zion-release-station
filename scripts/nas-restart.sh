#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${RS_NAS_CONFIG:-$repo_root/.env.nas}"
NAS_PORT="${NAS_PORT:-22}"
NAS_PACKAGE="${NAS_PACKAGE:-zion-releasestation}"
NAS_SUDO="${NAS_SUDO:-sudo}"
NAS_SUDO_PASS="${NAS_SUDO_PASS:-}"
NAS_IDENTITY_FILE="${NAS_IDENTITY_FILE:-}"
ssh_opts=(-p "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes)
if [[ -n "$NAS_IDENTITY_FILE" ]]; then
  [[ -r "$NAS_IDENTITY_FILE" ]] || { printf 'NAS identity file is not readable: %s\n' "$NAS_IDENTITY_FILE" >&2; exit 1; }
  ssh_opts+=(-i "$NAS_IDENTITY_FILE" -o IdentitiesOnly=yes)
fi
remote="$NAS_USER@$NAS_HOST"
if [[ -n "$NAS_SUDO_PASS" ]]; then
  printf '%s\n' "$NAS_SUDO_PASS" | ssh "${ssh_opts[@]}" "$remote" "$NAS_SUDO -S -p '' synopkg restart '$NAS_PACKAGE'"
else
  ssh -tt "${ssh_opts[@]}" "$remote" "$NAS_SUDO synopkg restart '$NAS_PACKAGE'"
fi
