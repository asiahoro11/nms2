#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/apps/frontend"

for TARGET_DIR in \
  "$ROOT_DIR/apps/backend/static" \
  "$ROOT_DIR/apps/backend/cmd/agent/static"
do
  mkdir -p "$(dirname "$TARGET_DIR")"
  rm -rf "$TARGET_DIR"
  cp -R "$SOURCE_DIR" "$TARGET_DIR"
done

echo "Static assets synced from apps/frontend."
