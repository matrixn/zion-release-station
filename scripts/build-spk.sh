#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${VERSION:-0.1.0}"
build_number="${BUILD_NUMBER:-$(date +%s)}"
package_name="zion-releasestation"
staging="$repo_root/build/package"
package_root="$staging/package"
dist_dir="$repo_root/dist"

rm -rf "$staging" "$dist_dir"
mkdir -p "$package_root/bin" "$package_root/web" "$package_root/migrations" "$package_root/nginx" "$package_root/port_conf" "$package_root/ui" "$dist_dir"

"$repo_root/scripts/build-frontend.sh"
VERSION="$version" "$repo_root/scripts/build-backend.sh"

cp "$repo_root/build/backend/zion-releasestation" "$package_root/bin/zion-releasestation"
cp -R "$repo_root/build/frontend/." "$package_root/web/"
cp "$repo_root/synology/nginx/releasestation.conf" "$package_root/nginx/"
cp "$repo_root/synology/port_conf/zion-releasestation.sc" "$package_root/port_conf/"
cp "$repo_root/synology/ui/config" "$package_root/ui/"
cp "$repo_root/internal/database/migrations/0001_foundation.sql" "$package_root/migrations/"
chmod 0755 "$package_root/bin/zion-releasestation"

python3 "$repo_root/scripts/generate-icons.py" "$staging"

package_tgz="$staging/package.tgz"
tar -C "$package_root" -czf "$package_tgz" .

info_file="$staging/INFO"
PACKAGE="$package_name" VERSION="$version" BUILD_NUMBER="$build_number" \
  bash "$repo_root/synology/INFO.sh" > "$info_file"

mkdir -p "$staging/scripts" "$staging/conf" "$staging/WIZARD_UIFILES"
cp "$repo_root/synology/scripts/"* "$staging/scripts/"
cp "$repo_root/synology/conf/"* "$staging/conf/"
cp "$repo_root/synology/WIZARD_UIFILES/install_uifile" "$staging/WIZARD_UIFILES/"
chmod 0755 "$staging/scripts/"*

artifact="$dist_dir/${package_name}-${version}-${build_number}-x86_64.spk"
cp "$repo_root/LICENSE" "$staging/LICENSE"
tar -C "$staging" -cf "$artifact" INFO package.tgz scripts conf WIZARD_UIFILES PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG LICENSE

"$repo_root/scripts/validate-spk.sh" "$artifact"
printf 'SPK created: %s\n' "$artifact"
