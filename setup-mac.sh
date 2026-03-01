#!/usr/bin/env bash
set -euo pipefail

OUT="./makers"
URL="https://github.com/sagiegurari/cargo-make/releases/download/0.37.24/cargo-make-v0.37.24-aarch64-apple-darwin.zip"
DIR="cargo-make-v0.37.24-aarch64-apple-darwin"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fL "$URL" -o "$tmp/cargo-make.zip"
unzip -q "$tmp/cargo-make.zip" -d "$tmp"

install -m 0755 "$tmp/$DIR/makers" "$OUT"

mkdir -p dist log out

echo "installed: $OUT"
echo "try: ./makers --list-all-steps"