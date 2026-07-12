<#
.SYNOPSIS
  Build palworld-save-relay.exe (frontend + icon + syso + Go binary).

.DESCRIPTION
  Run this after editing frontend, Go sources, or the app icon.
  If ./icon.png exists in the repo root it is used as the app icon source.
  Steps:
    1. frontend:  npm run build  (tsc type-check + vite build -> frontend/dist)
    2. icon:      copy ./icon.png (if present) -> build/appicon.png, then
                  wails3 generate icons  (-> build/icon.ico, build/icons.icns)
    3. syso:      wails3 generate syso  (icon + manifest -> wails.syso)
    4. go:        go build  (embeds frontend/dist + syso -> palworld-save-relay.exe)

.PARAMETER Arch
  Target architecture: amd64 (default) or arm64.

.EXAMPLE
  .\build.ps1
  .\build.ps1 -Arch arm64
#>
param(
  [ValidateSet("amd64", "arm64")]
  [string]$Arch = "amd64"
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
if (-not $root) { $root = Split-Path -Parent $MyInvocation.MyCommand.Path }
Set-Location $root

function Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }

# --- 1. Frontend ---
Step "Building frontend (tsc + vite)..."
Push-Location (Join-Path $root "frontend")
try {
  npm run build
  if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
} finally { Pop-Location }

# --- 2. App icon ---
$srcIcon = Join-Path $root "icon.png"
$dstIcon = Join-Path $root "build\appicon.png"
if (Test-Path $srcIcon) {
  Step "Updating app icon from ./icon.png ..."
  Copy-Item -LiteralPath $srcIcon -Destination $dstIcon -Force
}
Step "Generating icons (.ico / .icns) from appicon.png ..."
Push-Location (Join-Path $root "build")
try {
  wails3 generate icons -input appicon.png -windowsfilename icon.ico
  if ($LASTEXITCODE -ne 0) { throw "icon generation failed (is wails3 on PATH?)" }
} finally { Pop-Location }

# --- 3. Windows syso (icon + manifest) ---
Step "Generating Windows syso ($Arch)..."
Push-Location (Join-Path $root "build")
try {
  wails3 generate syso -arch $Arch -icon icon.ico -manifest wails.exe.manifest -info info.json -out ../wails.syso
  if ($LASTEXITCODE -ne 0) { throw "syso generation failed" }
} finally { Pop-Location }

# --- 4. Go binary ---
Step "Building Go binary ($Arch)..."
$env:CGO_ENABLED = "1"
$env:GOOS = "windows"
$env:GOARCH = $Arch
go build -tags production -trimpath -ldflags="-w -s -H windowsgui" -o (Join-Path $root "palworld-save-relay.exe") .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

$out = Join-Path $root "palworld-save-relay.exe"
Write-Host "`n==> Done: $out" -ForegroundColor Green
Get-Item $out | Select-Object Name, @{N="MB";E={[math]::Round($_.Length/1MB,1)}}, LastWriteTime | Format-Table -AutoSize