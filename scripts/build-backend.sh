#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-0.1.0}"
mkdir -p "$repo_root/build/backend"

cd "$repo_root"
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -trimpath \
  -ldflags "-s -w" \
  -o "$repo_root/build/backend/zion-releasestation" \
  ./cmd/releasestation

printf 'Backend built in %s (version %s)\n' "$repo_root/build/backend/zion-releasestation" "$version"
