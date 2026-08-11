#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
config_file="${RS_NAS_CONFIG:-$repo_root/.env.nas}"
if [[ ! -f "$config_file" ]]; then
  printf 'Missing %s. Copy .env.nas.example or create .env.nas.\n' "$config_file" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$config_file"

: "${NAS_HOST:?NAS_HOST is required}"
: "${NAS_USER:?NAS_USER is required}"
NAS_PORT="${NAS_PORT:-22}"
NAS_PACKAGE="${NAS_PACKAGE:-zion-releasestation}"
NAS_REMOTE_SPK="${NAS_REMOTE_SPK:-/tmp/zion-releasestation-dev.spk}"
NAS_SUDO="${NAS_SUDO:-sudo}"

if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  make -C "$repo_root" spk
fi
artifact="${SPK_ARTIFACT:-$(find "$repo_root/dist" -maxdepth 1 -type f -name '*.spk' -printf '%T@ %p\n' | sort -nr | cut -d' ' -f2- | head -n1)}"
[[ -n "$artifact" && -f "$artifact" ]] || { printf 'No SPK artifact available.\n' >&2; exit 1; }

ssh_opts=(-p "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes)
scp_opts=(-P "$NAS_PORT" -o BatchMode=yes -o StrictHostKeyChecking=yes)
remote="$NAS_USER@$NAS_HOST"

printf 'Checking SSH access to %s...\n' "$remote"
ssh "${ssh_opts[@]}" "$remote" 'printf "%s\n" ssh-ok'
printf 'Uploading %s...\n' "$artifact"
scp "${scp_opts[@]}" "$artifact" "$remote:$NAS_REMOTE_SPK"

printf 'Installing %s on Synology...\n' "$NAS_PACKAGE"
if ! ssh -tt "${ssh_opts[@]}" "$remote" "$NAS_SUDO synopkg install '$NAS_REMOTE_SPK'"; then
  ssh "${ssh_opts[@]}" "$remote" "tail -n 120 /var/log/packages/$NAS_PACKAGE.log 2>/dev/null || true; tail -n 120 /var/log/synopkg.log 2>/dev/null || true" || true
  exit 1
fi
ssh "${ssh_opts[@]}" "$remote" "$NAS_SUDO synopkg start '$NAS_PACKAGE' >/dev/null 2>&1 || true"
"$repo_root/scripts/nas-health.sh"
printf 'Deployment completed successfully.\n'
