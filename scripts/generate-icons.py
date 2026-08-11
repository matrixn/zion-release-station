#!/usr/bin/env python3
"""Copy the ReleaseStation icon set into the SPK staging directory."""

from __future__ import annotations

import shutil
import sys
from pathlib import Path


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: generate-icons.py <staging-directory>")
    staging = Path(sys.argv[1])
    repository_root = Path(__file__).resolve().parent.parent
    source = repository_root / "assets" / "icons"
    target = staging / "package" / "ui" / "images"
    target.mkdir(parents=True, exist_ok=True)

    required = ["PACKAGE_ICON.PNG", "PACKAGE_ICON_256.PNG"] + [
        f"app_{size}.png" for size in (16, 24, 32, 48, 64, 72, 256)
    ]
    missing = [name for name in required if not (source / name).is_file()]
    if missing:
        raise SystemExit(f"missing icon assets: {', '.join(missing)}")

    shutil.copyfile(source / "PACKAGE_ICON.PNG", staging / "PACKAGE_ICON.PNG")
    shutil.copyfile(source / "PACKAGE_ICON_256.PNG", staging / "PACKAGE_ICON_256.PNG")
    for size in (16, 24, 32, 48, 64, 72, 256):
        shutil.copyfile(source / f"app_{size}.png", target / f"app_{size}.png")


if __name__ == "__main__":
    main()
