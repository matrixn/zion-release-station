#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${RS_NAS_CONFIG:-$repo_root/.env.nas}"
NAS_PORT="${NAS_PORT:-22}"
NAS_PACKAGE="${NAS_PACKAGE:-zion-releasestation}"
ssh -p "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes "$NAS_USER@$NAS_HOST" "tail -n 160 /var/log/packages/$NAS_PACKAGE.log 2>/dev/null || true; tail -n 160 /var/log/synopkg.log 2>/dev/null || true"
