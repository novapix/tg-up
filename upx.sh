#!/bin/bash
set -e

DIST_DIR=dist

# Check if UPX is installed
if ! command -v upx &> /dev/null; then
    echo "UPX not found! Install it first."
    exit 1
fi

echo "Compressing binaries in $DIST_DIR using fast compression..."

for bin in "$DIST_DIR"/*; do
    if [[ -f "$bin" ]]; then
        echo "Compressing $bin..."
        upx --fast "$bin"
    fi
done

echo "✅ All binaries compressed with fast mode."
