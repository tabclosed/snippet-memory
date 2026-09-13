#!/usr/bin/env bash
# Builds a single, self-contained Windows .exe: builds the React frontend,
# embeds it into the Go binary, and cross-compiles for Windows. Works from
# macOS or Linux — you do NOT need a Windows machine to produce the .exe.
#
# Usage:
#   ./build.sh
#
# Output: dist/snippet-memory.exe
set -euo pipefail
cd "$(dirname "$0")"

echo "==> Building frontend..."
(cd frontend && npm install && npm run build)

echo "==> Embedding frontend into backend/static..."
rm -rf backend/static
cp -r frontend/dist backend/static

echo "==> Cross-compiling Windows .exe..."
mkdir -p dist
(cd backend && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags release -o ../dist/snippet-memory.exe .)

echo "==> Cleaning up embedded copy..."
rm -rf backend/static

echo ""
echo "Done: dist/snippet-memory.exe"
echo "Copy that single file anywhere on a Windows machine and double-click it."
echo "It listens on http://localhost:8080 and stores its database in a"
echo "\"database\" folder created next to the .exe on first run."
