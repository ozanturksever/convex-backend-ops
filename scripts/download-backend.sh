#!/bin/bash
set -e

# Download convex-local-backend binary for testing
# Usage: ./scripts/download-backend.sh [platform]
# Platforms: darwin-arm64, darwin-x64, linux-x64, linux-arm64

PLATFORM=${1:-""}
OUTPUT_DIR="testdata/sample-bundle"

# Map simple platform names to Rust target triples
map_platform() {
    case "$1" in
        darwin-arm64|aarch64-apple-darwin)
            echo "aarch64-apple-darwin"
            ;;
        darwin-x64|x86_64-apple-darwin)
            echo "x86_64-apple-darwin"
            ;;
        linux-x64|x86_64-unknown-linux-gnu)
            echo "x86_64-unknown-linux-gnu"
            ;;
        linux-arm64|aarch64-unknown-linux-gnu)
            echo "aarch64-unknown-linux-gnu"
            ;;
        *)
            echo "$1"
            ;;
    esac
}

# Auto-detect platform if not specified
if [ -z "$PLATFORM" ]; then
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case "$ARCH" in
        x86_64) ARCH="x64" ;;
        aarch64|arm64) ARCH="arm64" ;;
    esac
    
    PLATFORM="${OS}-${ARCH}"
fi

# Map to Rust target triple
RUST_TARGET=$(map_platform "$PLATFORM")

echo "Downloading convex-local-backend for platform: $PLATFORM (target: $RUST_TARGET)"

# Get latest release from convex-backend releases
RELEASE_URL="https://github.com/get-convex/convex-backend/releases/latest/download/convex-local-backend-${RUST_TARGET}.zip"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Download and extract
TEMP_FILE=$(mktemp)
TEMP_DIR=$(mktemp -d)
echo "Downloading from: $RELEASE_URL"
curl -fSL "$RELEASE_URL" -o "$TEMP_FILE" || {
    echo "Failed to download from: $RELEASE_URL"
    echo "Available platforms: darwin-arm64, darwin-x64, linux-x64, linux-arm64"
    rm -f "$TEMP_FILE"
    rm -rf "$TEMP_DIR"
    exit 1
}

# Extract zip file
echo "Extracting..."
unzip -o "$TEMP_FILE" -d "$TEMP_DIR"

# Find the binary (might be in root or subdirectory)
BINARY_PATH=$(find "$TEMP_DIR" -name "convex-local-backend" -type f | head -1)
if [ -z "$BINARY_PATH" ]; then
    # Try without the exact name
    BINARY_PATH=$(find "$TEMP_DIR" -type f -executable | head -1)
fi

if [ -n "$BINARY_PATH" ]; then
    mv "$BINARY_PATH" "${OUTPUT_DIR}/backend"
    chmod +x "${OUTPUT_DIR}/backend"
    echo "Downloaded to: ${OUTPUT_DIR}/backend"
else
    echo "Error: Could not find binary in archive"
    ls -la "$TEMP_DIR"
    rm -f "$TEMP_FILE"
    rm -rf "$TEMP_DIR"
    exit 1
fi

rm -f "$TEMP_FILE"
rm -rf "$TEMP_DIR"

# Create empty convex.db for testing (will be replaced by real pre-deployed db)
if [ ! -f "${OUTPUT_DIR}/convex.db" ]; then
    touch "${OUTPUT_DIR}/convex.db"
    echo "Created placeholder: ${OUTPUT_DIR}/convex.db"
fi

echo "Done! Sample bundle is ready at: $OUTPUT_DIR"
