#!/bin/bash
set -e

DIST_DIR=dist

if [ "$#" -ne 1 ]; then
    echo "Usage: ./upx.sh <GOOS>"
    exit 1
fi

GOOS="$1"

# Check if UPX is installed and accessible
if ! command -v upx &> /dev/null; then
    echo "ERROR: UPX not found! Please install it first."
    exit 1 
fi

UPX_OPTS="--fast"

echo "Compressing binaries in $DIST_DIR for $GOOS using options: $UPX_OPTS"

for bin in "$DIST_DIR"/*; do
    if [[ -f "$bin" ]]; then
        echo "Compressing $bin..."
        upx $UPX_OPTS "$bin"
    fi
done

echo "All binaries compressed with fast mode."