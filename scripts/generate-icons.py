#!/usr/bin/env python3
"""Generate the small deterministic PNG icon set used by the SPK."""

from __future__ import annotations

import os
import struct
import sys
import zlib


def chunk(kind: bytes, payload: bytes) -> bytes:
    return struct.pack(">I", len(payload)) + kind + payload + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)


def write_png(path: str, size: int) -> None:
    pixels = bytearray()
    for y in range(size):
        pixels.append(0)
        for x in range(size):
            nx = (x + 0.5) / size * 2 - 1
            ny = (y + 0.5) / size * 2 - 1
            distance = (nx * nx + ny * ny) ** 0.5
            if distance > 0.94:
                color = (8, 15, 30, 0)
            elif distance > 0.82:
                color = (255, 156, 70, 255)
            elif abs(nx) < 0.10 and ny < 0.35:
                color = (255, 255, 255, 255)
            elif abs(ny) < 0.08 and nx > -0.35:
                color = (111, 225, 190, 255)
            else:
                color = (14, 25, 48, 255)
            pixels.extend(color)

    png = b"\x89PNG\r\n\x1a\n"
    png += chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0))
    png += chunk(b"IDAT", zlib.compress(bytes(pixels), 9))
    png += chunk(b"IEND", b"")
    with open(path, "wb") as handle:
        handle.write(png)


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: generate-icons.py <staging-directory>")
    staging = sys.argv[1]
    os.makedirs(os.path.join(staging, "package", "ui", "images"), exist_ok=True)
    write_png(os.path.join(staging, "PACKAGE_ICON.PNG"), 64)
    write_png(os.path.join(staging, "PACKAGE_ICON_256.PNG"), 256)
    for size in (16, 24, 32, 48, 64, 72, 256):
        write_png(os.path.join(staging, "package", "ui", "images", f"app_{size}.png"), size)


if __name__ == "__main__":
    main()
