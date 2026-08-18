#!/usr/bin/env bash
# Made by YTSworks
# Linux release initializer and foreground launcher.
set -Eeuo pipefail
IFS=$'\n\t'

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$SCRIPT_DIR"

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: ./init.sh [--prepare-only]

  no option       Initialize permissions/directories, then start NMS.
  --prepare-only  Initialize without starting NMS.
  -h, --help      Show this help.
EOF
}

case "${1:-}" in
  "") ;;
  --prepare-only) PREPARE_ONLY=1 ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

[[ "$(uname -s)" == "Linux" ]] || fail "init.sh supports Linux release packages only"

case "$(uname -m)" in
  x86_64|amd64)
    ARCH="amd64"
    SERVER_BIN="nms_server_linux_amd64"
    HELPER_SUFFIX="linux_amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    SERVER_BIN="nms_server_linux_arm64"
    HELPER_SUFFIX="linux_arm64"
    ;;
  *)
    fail "unsupported CPU architecture: $(uname -m)"
    ;;
esac

[[ -f "$SERVER_BIN" ]] || fail "missing NMS binary: $SCRIPT_DIR/$SERVER_BIN"
[[ -f "start_nms.sh" ]] || fail "missing launcher: $SCRIPT_DIR/start_nms.sh"

# A signing private key must never be present on an NMS runtime host.
[[ ! -e "issuer_private.key" ]] || fail "issuer_private.key must not be stored in the NMS directory"
[[ -z "${NMS_LICENSE_PRIVATE_KEY_B64:-}" ]] || fail "remove NMS_LICENSE_PRIVATE_KEY_B64 from the NMS runtime environment"

umask 077
mkdir -p "data" "data/uploads"
chmod 700 "data" "data/uploads"
chmod u+x "$SERVER_BIN" "start_nms.sh" "$0"

for helper in \
  "bin/ffmpeg_${HELPER_SUFFIX}" \
  "bin/go2rtc_${HELPER_SUFFIX}" \
  "tools/superadmin-local_${HELPER_SUFFIX}" \
  "superadmin-local_${HELPER_SUFFIX}"; do
  if [[ -f "$helper" ]]; then
    chmod u+x "$helper"
  fi
done

printf 'NMS Linux initialization complete.\n'
printf 'Architecture: %s\n' "$ARCH"
printf 'Working directory: %s\n' "$SCRIPT_DIR"
printf 'Server binary: %s\n' "$SERVER_BIN"

if [[ "${PREPARE_ONLY:-0}" == "1" ]]; then
  printf 'Preparation only; NMS was not started.\n'
  exit 0
fi

printf 'Starting NMS in foreground. Press Ctrl+C to stop.\n'
exec "$SCRIPT_DIR/start_nms.sh"
