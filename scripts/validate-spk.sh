#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
artifact="${1:-}"
if [[ -z "$artifact" ]]; then
  artifact="$(find "$repo_root/dist" -maxdepth 1 -type f -name '*.spk' -print -quit 2>/dev/null || true)"
fi
if [[ -z "$artifact" || ! -f "$artifact" ]]; then
  printf 'No SPK artifact found.\n' >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() { rm -rf "$tmp_dir"; }
trap cleanup EXIT

tar -C "$tmp_dir" -xf "$artifact"
for required in INFO package.tgz scripts/start-stop-status conf/privilege WIZARD_UIFILES/install_uifile PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG; do
  if [[ ! -e "$tmp_dir/$required" ]]; then
    printf 'SPK is missing %s\n' "$required" >&2
    exit 1
  fi
done

tar -tzf "$tmp_dir/package.tgz" | grep -Fx './bin/zion-releasestation' >/dev/null || {
  printf 'package.tgz is missing the runtime binary.\n' >&2
  exit 1
}
tar -tzf "$tmp_dir/package.tgz" | grep -Fx './web/index.html' >/dev/null || {
  printf 'package.tgz is missing the frontend entry point.\n' >&2
  exit 1
}

printf 'SPK valid: %s\n' "$artifact"
