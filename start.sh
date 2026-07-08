#!/bin/bash
set -e

cd "$(dirname "$0")"

# Default config
CONFIG="${CONFIG:-config.yaml}"
PORT="${PORT:-8080}"

echo "=== MCP Gateway ==="

# Check for config
if [ ! -f "$CONFIG" ]; then
    echo "Copying default config..."
    cp config.yaml "$CONFIG"
fi

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

# Install dependencies
echo "Installing dependencies..."
go mod tidy

# Build
echo "Building..."
go build -o mcp-gateway ./cmd/server/

# Run
echo "Starting MCP Gateway on port $PORT..."
export PORT="$PORT"
exec ./mcp-gateway
