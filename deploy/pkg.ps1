param(
  [string]$Targets = "node20-win-x64,node20-linux-x64,node20-macos-arm64"
)
$ErrorActionPreference = "Stop"
if (!(Get-Command pkg -ErrorAction SilentlyContinue)) { Write-Host "Install pkg: npm i -g pkg"; exit 1 }

if (!(Test-Path -Path "dist")) { New-Item -ItemType Directory -Path "dist" | Out-Null }

pushd backend
try {
  if (Get-Command npm -ErrorAction SilentlyContinue) { npm run build } else { Write-Host "Skip backend build: npm not found" }
} finally { popd }

pkg backend\dist\index.js --targets $Targets --output dist\airshare

$npmMissing = $false
if (!(Get-Command npm -ErrorAction SilentlyContinue)) { Write-Host "Skip frontend build: npm not found"; $npmMissing = $true }

if (-not $npmMissing) {
  pushd frontend
  try {
    npm ci
    npm run build
  } finally { popd }
} else {
  Write-Host "Frontend build skipped"
}

if (Test-Path -Path "frontend\dist") {
  if (!(Test-Path -Path "dist\frontend")) { New-Item -ItemType Directory -Path "dist\frontend" | Out-Null }
  Copy-Item -Recurse -Force frontend\dist dist\frontend\dist
} else {
  Write-Host "No frontend\\dist to copy"
}
