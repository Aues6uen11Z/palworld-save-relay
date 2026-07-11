# Copy real save fixtures from a local Palworld install into this folder.
# Usage: pwsh fetch.ps1
$ErrorActionPreference = "Stop"
$root = Join-Path $env:LOCALAPPDATA "Pal\Saved\SaveGames"
$steam = Get-ChildItem $root -Directory | Where-Object { Get-ChildItem $_.FullName -Directory | Test-Path } | Select-Object -First 1
if (-not $steam) { Write-Error "No Palworld save folder under $root"; exit 1 }
$worlds = Get-ChildItem $steam.FullName -Directory | Where-Object { Test-Path (Join-Path $_.FullName "Level.sav") -or (Get-ChildItem (Join-Path $_.FullName "backup\world") -Directory -ErrorAction SilentlyContinue) }
Write-Host "Found worlds:"; $worlds | ForEach-Object { Write-Host "  $($_.Name)" }
# Pick one PlZ and one PlM world by inspecting magic.
foreach ($w in $worlds) {
  $lvl = Join-Path $w.FullName "Level.sav"
  if (-not (Test-Path $lvl)) {
    $snap = Get-ChildItem (Join-Path $w.FullName "backup\world") -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | Select-Object -First 1
    if ($snap) { $lvl = Join-Path $snap.FullName "Level.sav" }
  }
  if (-not (Test-Path $lvl)) { continue }
  $b = [System.IO.File]::ReadAllBytes($lvl)
  $magic = [System.Text.Encoding]::ASCII.GetString($b[8..10])
  $src = Split-Path $lvl -Parent
  if ($magic -eq "PlZ" -and -not (Test-Path "level_plz.sav")) {
    Copy-Item $lvl "level_plz.sav"
    $p = Get-ChildItem (Join-Path $src "Players") -Filter *.sav -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($p) { Copy-Item $p.FullName "player_plz.sav" }
  }
  if ($magic -eq "PlM" -and -not (Test-Path "level_plm.sav")) {
    Copy-Item $lvl "level_plm.sav"
    $p = Get-ChildItem (Join-Path $src "Players") -Filter *.sav -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($p) { Copy-Item $p.FullName "player_plm.sav" }
  }
}
Write-Host "Done. Present fixtures:"; Get-ChildItem *.sav -ErrorAction SilentlyContinue | ForEach-Object { Write-Host "  $($_.Name)" }
