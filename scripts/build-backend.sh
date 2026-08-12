#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-0.1.0}"
mkdir -p "$repo_root/build/backend"

go_bin="${GO_BIN:-}"
if [[ -z "$go_bin" ]]; then
  if command -v go >/dev/null 2>&1; then
    go_bin="$(command -v go)"
  elif [[ -x /tmp/zion-go/bin/go ]]; then
    go_bin="/tmp/zion-go/bin/go"
  fi
fi
if [[ -z "$go_bin" || ! -x "$go_bin" ]]; then
  printf 'Go toolchain not found. Install Go or set GO_BIN to the go executable path.\n' >&2
  exit 127
fi

cd "$repo_root"
"$go_bin" test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "$go_bin" build \
  -trimpath \
  -ldflags "-s -w" \
  -o "$repo_root/build/backend/zion-releasestation" \
  ./cmd/releasestation

printf 'Backend built in %s (version %s)\n' "$repo_root/build/backend/zion-releasestation" "$version"
