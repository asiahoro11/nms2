#!/usr/bin/env bash
# Made by YTSworks
# YTS工作室製作
set -euo pipefail

export LANG=zh_TW.UTF-8
export LC_ALL=zh_TW.UTF-8

ARCH="$(uname -m)"
if [ "$ARCH" = "x86_64" ]; then
  SERVER_BIN="./nms_server_linux_amd64"
elif [ "$ARCH" = "aarch64" ]; then
  SERVER_BIN="./nms_server_linux_arm64"
else
  SERVER_BIN="./nms_server_linux_amd64"
fi

if [ ! -f "$SERVER_BIN" ]; then
  echo "error: missing binary $SERVER_BIN" >&2
  exit 1
fi

chmod +x "$SERVER_BIN"
if [ -f "./bin/ffmpeg" ]; then
  chmod +x "./bin/ffmpeg"
fi

echo "Starting Management System ($ARCH)..."
"$SERVER_BIN"
