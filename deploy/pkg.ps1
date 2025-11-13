param(
  [string]$Targets = "node20-win-x64,node20-linux-x64,node20-macos-arm64"
)
$ErrorActionPreference = "Stop"
if (!(Get-Command pkg -ErrorAction SilentlyContinue)) { Write-Host "Install pkg: npm i -g pkg"; exit 1 }
pushd backend
npm run build
popd
pkg backend\dist\index.js --targets $Targets --output dist\airshare
