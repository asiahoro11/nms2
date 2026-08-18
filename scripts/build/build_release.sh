#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKEND_SOURCE="$REPO_ROOT/apps/backend"
FRONTEND_SOURCE="$REPO_ROOT/apps/frontend"
RUNTIME_BIN="$REPO_ROOT/runtime/bin"
RELEASE_NOTES="$REPO_ROOT/docs/RELEASE_NOTES.md"
TMP_ROOT="$REPO_ROOT/artifacts/.tmp"
# Release protection policy:
# - Web assets are not obfuscated/minified by default to reduce antivirus false positives.
# - Go binary obfuscation is opt-in for protected core code paths.
ENABLE_OBFUSCATION="${NMS_ENABLE_OBFUSCATION:-0}"
ENABLE_BINARY_OBFUSCATION="${NMS_ENABLE_BINARY_OBFUSCATION:-0}"
LICENSE_PUBLIC_KEY_B64="${NMS_LICENSE_PUBLIC_KEY_B64:-}"
if [[ ! "$LICENSE_PUBLIC_KEY_B64" =~ ^[A-Za-z0-9+/]{43}=$ ]]; then
  echo "error: NMS_LICENSE_PUBLIC_KEY_B64 must be a 32-byte Ed25519 public key encoded as Base64" >&2
  exit 1
fi
LICENSE_LDFLAGS="-s -w -X management-server/services/license.BuildPublicKeyB64=$LICENSE_PUBLIC_KEY_B64"

VERSION="$(grep -Eo 'var Version = "[^"]+"' "$BACKEND_SOURCE/config/config.go" | head -n1 | sed -E 's/var Version = "([^"]+)"/\1/')"
if [ -z "$VERSION" ]; then
  VERSION="latest"
fi

build_platform() {
  local platform="$1"
  local target_dir="$2"
  local temp_dir
  temp_dir="$(mktemp -d "$TMP_ROOT/${platform}-XXXXXX")"
  local temp_backend="$temp_dir/backend"

  mkdir -p "$target_dir"
  rm -rf "$target_dir"
  mkdir -p "$target_dir"

  cp -R "$BACKEND_SOURCE" "$temp_backend"

  rm -rf "$temp_backend/static" "$temp_backend/cmd/agent/static"
  cp -R "$FRONTEND_SOURCE" "$temp_backend/static"
  cp -R "$FRONTEND_SOURCE" "$temp_backend/cmd/agent/static"

  if [ "$ENABLE_OBFUSCATION" = "1" ] && command -v npx >/dev/null 2>&1; then
    while IFS= read -r -d '' file; do
      npx -y javascript-obfuscator "$file" --output "$file" --compact true --string-array true --string-array-encoding base64 --identifier-names-generator hexadecimal --rename-globals false >/dev/null
    done < <(find "$temp_backend/static/js" "$temp_backend/cmd/agent/static/js" -type f -name '*.js' -print0 2>/dev/null)

    for target in "$temp_backend/static" "$temp_backend/cmd/agent/static"; do
      for html in index.html login.html monitor.html mode-selection.html; do
        if [ -f "$target/$html" ]; then
          npx -y html-minifier-terser "$target/$html" -o "$target/$html" --collapse-whitespace --remove-comments --remove-redundant-attributes --remove-script-type-attributes --use-short-doctype --minify-css true --minify-js true >/dev/null
        fi
      done
    done
  elif [ "$ENABLE_OBFUSCATION" = "1" ]; then
    echo "error: NMS_ENABLE_OBFUSCATION=1 but npx not found, refusing to build a non-obfuscated release" >&2
    exit 1
  else
    echo "info: building without JS obfuscation or HTML minification (default clean build)" >&2
  fi

  if [ "$platform" = "windows" ]; then
    (
      cd "$temp_backend"
      if [ "$ENABLE_BINARY_OBFUSCATION" = "1" ] && command -v garble >/dev/null 2>&1; then
        echo "info: building with Go binary obfuscation enabled via garble" >&2
        GOWORK=off GOOS=windows GOARCH=amd64 garble build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server.exe" .
      elif [ "$ENABLE_BINARY_OBFUSCATION" = "1" ]; then
        echo "error: NMS_ENABLE_BINARY_OBFUSCATION=1 but garble not found, refusing to build a non-obfuscated binary" >&2
        exit 1
      else
        GOWORK=off GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server.exe" .
      fi
    )
    printf '%s\n' \
      "Product: Management System" \
      "Version: $VERSION" \
      "Platform: windows-amd64" \
      "BuildDate: $(date '+%Y-%m-%d %H:%M:%S %z')" \
      "Artifact: nms_server.exe" \
      "ReleaseNotes: RELEASE_NOTES.md" > "$target_dir/RELEASE_NOTE.txt"
    cp "$REPO_ROOT/scripts/run/start_nms_utf8.bat" "$target_dir/start_nms.bat"
    cp "$REPO_ROOT/scripts/run/start_nms_utf8.ps1" "$target_dir/start_nms.ps1"
  else
    (
      cd "$temp_backend"
      if [ "$ENABLE_BINARY_OBFUSCATION" = "1" ] && command -v garble >/dev/null 2>&1; then
        echo "info: building with Go binary obfuscation enabled via garble" >&2
        GOWORK=off GOOS=linux GOARCH=amd64 garble build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server_linux_amd64" .
        GOWORK=off GOOS=linux GOARCH=arm64 garble build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server_linux_arm64" .
      elif [ "$ENABLE_BINARY_OBFUSCATION" = "1" ]; then
        echo "error: NMS_ENABLE_BINARY_OBFUSCATION=1 but garble not found, refusing to build a non-obfuscated binary" >&2
        exit 1
      else
        GOWORK=off GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server_linux_amd64" .
        GOWORK=off GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="$LICENSE_LDFLAGS" -o "$target_dir/nms_server_linux_arm64" .
      fi
    )
    printf '%s\n' \
      "Product: Management System" \
      "Version: $VERSION" \
      "Platform: linux-amd64,linux-arm64" \
      "BuildDate: $(date '+%Y-%m-%d %H:%M:%S %z')" \
      "Artifacts: nms_server_linux_amd64, nms_server_linux_arm64" \
      "ReleaseNotes: RELEASE_NOTES.md" > "$target_dir/RELEASE_NOTE.txt"
    cp "$REPO_ROOT/scripts/run/start_nms_utf8.sh" "$target_dir/start_nms.sh"
    cp "$REPO_ROOT/scripts/run/init.sh" "$target_dir/init.sh"
    chmod +x "$target_dir/start_nms.sh" "$target_dir/init.sh"
  fi

  mkdir -p "$target_dir/data"
  if [ -d "$RUNTIME_BIN" ] && [ "$(find "$RUNTIME_BIN" -mindepth 1 -maxdepth 1 | wc -l)" -gt 0 ]; then
    cp -R "$RUNTIME_BIN" "$target_dir/bin"
  fi
  if [ -f "$RELEASE_NOTES" ]; then
    cp "$RELEASE_NOTES" "$target_dir/RELEASE_NOTES.md"
  fi

  rm -rf "$temp_dir"
}

mkdir -p "$TMP_ROOT" "$REPO_ROOT/artifacts/windows" "$REPO_ROOT/artifacts/linux"

WINDOWS_DIR="$REPO_ROOT/artifacts/windows/$VERSION"
LINUX_DIR="$REPO_ROOT/artifacts/linux/$VERSION"

build_platform "windows" "$WINDOWS_DIR"
build_platform "linux" "$LINUX_DIR"

if command -v zip >/dev/null 2>&1; then
  rm -f "$REPO_ROOT/artifacts/windows/$VERSION.zip" "$REPO_ROOT/artifacts/linux/$VERSION.zip"
  (
    cd "$WINDOWS_DIR"
    zip -qr "../$VERSION.zip" .
  )
  (
    cd "$LINUX_DIR"
    zip -qr "../$VERSION.zip" .
  )
fi

echo "Windows artifact: $WINDOWS_DIR"
echo "Linux artifact:   $LINUX_DIR"
