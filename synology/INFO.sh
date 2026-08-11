#!/usr/bin/env bash
set -euo pipefail

package="${PACKAGE:-zion-releasestation}"
version="${VERSION:-0.1.0}-${BUILD_NUMBER:-0}"

cat <<EOF
package="$package"
version="$version"
displayname="Zion ReleaseStation"
description="Git deployment and release management for Synology DSM."
maintainer="Zion"
arch="x86_64"
os_min_ver="7.2.2-72806"
thirdparty="yes"
silent_upgrade="yes"
dsmuidir="ui"
dsmappname="SYNO.ZionReleaseStation.Instance"
startstop_restart_services="nginx.service"
instuninst_restart_services="nginx.service"
EOF
