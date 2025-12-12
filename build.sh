#!/bin/bash
set -euo pipefail

if [ "$#" -ne 3 ]; then
    echo "Usage: ./build.sh <GOOS> <GOARCH> <VERSION>"
    exit 1
fi

GOOS="$1"
GOARCH="$2"
VERSION="$3"

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT_HASH="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"

OUTPUT_DIR=dist
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"


get_out_name() {
    local os="$1"
    local arch="$2"
    local name="tg-up-v${VERSION}-${os}-${arch}"
    if [ "$os" == "windows" ]; then
        echo "$name.exe"
    else
        echo "$name"
    fi
}

OUT_NAME=$(get_out_name "$GOOS" "$GOARCH")

echo "-> Building for $GOOS/$GOARCH (v$VERSION) -> $OUT_NAME"

CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build \
  -ldflags="-s -w \
-X 'tg-up/version.Version=${VERSION}' \
-X 'tg-up/version.BuildDate=${BUILD_DATE}' \
-X 'tg-up/version.CommitHash=${COMMIT_HASH}'" \
  -o "$OUTPUT_DIR/$OUT_NAME" \
  main.go

echo "== Build complete for $GOOS/$GOARCH =="
ls -lh "$OUTPUT_DIR"