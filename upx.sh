#!/bin/bash
set -e

DIST_DIR=dist

# Check if UPX is installed and accessible
if ! command -v upx &> /dev/null; then
    echo "ERROR: UPX not found!"
    echo "Please ensure 'upx-ucl' or 'upx' is installed and available in your system's PATH."
    exit 1 
fi

echo "Compressing binaries in $DIST_DIR using fast compression..."

for bin in "$DIST_DIR"/*; do
    if [[ -f "$bin" ]]; then
        echo "Compressing $bin..."
        upx --fast "$bin"
    fi
done

echo "All binaries compressed with fast mode."