#!/usr/bin/env bash
# Builds a single, self-contained native Linux binary: builds the React
# frontend, embeds it into the Go binary, and compiles for the machine
# you're running this on (e.g. Manjaro/Arch, Ubuntu, etc).
#
# Usage:
#   ./build-linux.sh
#
# Output: dist/snippet-memory
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building frontend..."
(cd frontend && npm install && npm run build)

echo "==> Embedding frontend into backend/static..."
rm -rf backend/static
cp -r frontend/dist backend/static

echo "==> Compiling native Linux binary..."
mkdir -p dist
(cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags release -o ../dist/snippet-memory .)
chmod +x dist/snippet-memory

echo "==> Cleaning up embedded copy..."
rm -rf backend/static

echo ""
echo "Done: dist/snippet-memory"
echo "Run it with: ./dist/snippet-memory"
echo "It listens on http://localhost:8080 and stores its database in a"
echo "\"database\" folder created next to the binary on first run."
