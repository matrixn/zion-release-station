#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/frontend"

npm ci
npm run build

rm -rf "$repo_root/build/frontend"
mkdir -p "$repo_root/build/frontend"
cp -R dist/. "$repo_root/build/frontend/"
printf 'Frontend built in %s\n' "$repo_root/build/frontend"
