# Shelly Build Script (PowerShell)
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

$ROOT = $PSScriptRoot
$BACKEND = Join-Path $ROOT "backend"
$FRONTEND = Join-Path $ROOT "frontend"
$WEB_DIR = Join-Path $BACKEND "cmd\server\web"
$OUTPUT = Join-Path $ROOT "shelly.exe"

Write-Host "=== Building Shelly ===" -ForegroundColor Green

# Step 1: Build Frontend
Write-Host "`n[1/3] Building frontend..." -ForegroundColor Cyan
Push-Location $FRONTEND
try {
    npm install
    npm run build
} finally {
    Pop-Location
}

# Step 2: Copy frontend dist to embed directory
Write-Host "`n[2/3] Copying frontend to embed directory..." -ForegroundColor Cyan
if (Test-Path $WEB_DIR) {
    Remove-Item -Recurse -Force $WEB_DIR
}
New-Item -ItemType Directory -Force -Path $WEB_DIR | Out-Null
Copy-Item -Recurse -Force (Join-Path $FRONTEND "dist\*") $WEB_DIR

# Step 3: Build Backend
Write-Host "`n[3/3] Building backend..." -ForegroundColor Cyan
Push-Location $BACKEND
try {
    $env:CGO_ENABLED = "0"
    go build -a -ldflags="-s -w" -o $OUTPUT ./cmd/server
} finally {
    Pop-Location
}

Write-Host "`n=== Build complete: $OUTPUT ===" -ForegroundColor Green
Write-Host "Run with: $OUTPUT" -ForegroundColor Yellow
