#!/bin/bash
set -euo pipefail

# Read version from VERSION.txt (single line)
if [ ! -f VERSION.txt ]; then
  echo "VERSION.txt file not found. Create VERSION.txt (e.g. '0.1.0')."
  exit 1
fi
VERSION="$(tr -d ' \t\n\r' < VERSION.txt)"

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT_HASH="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"

OUTPUT_DIR=dist
mkdir -p "$OUTPUT_DIR"

build() {
    local GOOS="$1"
    local GOARCH="$2"
    local OUT_NAME="$3"

    echo "-> Building for $GOOS/$GOARCH -> $OUT_NAME"

    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
      -ldflags="-s -w \
-X tg-up/version.Version=${VERSION} \
-X tg-up/version.BuildDate=${BUILD_DATE} \
-X tg-up/version.CommitHash=${COMMIT_HASH}" \
      -o "$OUTPUT_DIR/$OUT_NAME" \
      main.go
}

# Linux
build linux amd64 "tg-up-v${VERSION}-linux-amd64"
build linux arm64 "tg-up-v${VERSION}-linux-arm64"

# macOS
build darwin amd64 "tg-up-v${VERSION}-darwin-amd64"
build darwin arm64 "tg-up-v${VERSION}-darwin-arm64"

# Windows
build windows amd64 "tg-up-v${VERSION}-windows-amd64.exe"
build windows 386  "tg-up-v${VERSION}-windows-386.exe"

echo "== All builds complete -> $OUTPUT_DIR/ =="
ls -lh "$OUTPUT_DIR"
