# Builds a single, self-contained native Linux binary from Windows: builds
# the React frontend, embeds it into the Go binary, and cross-compiles for
# Linux. You do NOT need a Linux machine to produce this — Go cross-compiles
# regardless of which OS you're building on.
#
# Usage:
#   .\build-linux.ps1
#
# Output: dist\snippet-memory (no .exe extension — it's a Linux binary,
# won't run on Windows; copy it to the Linux machine you want to run it on)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "==> Building frontend..."
Push-Location frontend
npm install
npm run build
Pop-Location

Write-Host "==> Embedding frontend into backend\static..."
Remove-Item -Recurse -Force backend\static -ErrorAction SilentlyContinue
Copy-Item -Recurse frontend\dist backend\static

Write-Host "==> Cross-compiling Linux binary..."
New-Item -ItemType Directory -Force -Path dist | Out-Null
Push-Location backend
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -tags release -o ..\dist\snippet-memory .
Pop-Location

Write-Host "==> Cleaning up embedded copy..."
Remove-Item -Recurse -Force backend\static

Write-Host ""
Write-Host "Done: dist\snippet-memory"
Write-Host "Copy that single file to a Linux (x86-64) machine and run it with:"
Write-Host "  chmod +x snippet-memory && ./snippet-memory"
Write-Host "It listens on http://localhost:8080 and stores its database in a"
Write-Host "'database' folder created next to the binary on first run."
