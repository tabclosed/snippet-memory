# Builds a single, self-contained Windows .exe: builds the React frontend,
# embeds it into the Go binary, and compiles it. Run from Windows with
# PowerShell (Go and Node.js must both be installed).
#
# Usage:
#   .\build.ps1
#
# Output: dist\snippet-memory.exe
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

Write-Host "==> Compiling Windows .exe..."
New-Item -ItemType Directory -Force -Path dist | Out-Null
Push-Location backend
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -tags release -o ..\dist\snippet-memory.exe .
Pop-Location

Write-Host "==> Cleaning up embedded copy..."
Remove-Item -Recurse -Force backend\static

Write-Host ""
Write-Host "Done: dist\snippet-memory.exe"
Write-Host "Copy that single file anywhere and double-click it."
Write-Host "It listens on http://localhost:8080 and stores its database in a"
Write-Host "'database' folder created next to the .exe on first run."
