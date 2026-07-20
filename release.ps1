# release.ps1 - One-command release to GitHub + Gitee
# Usage: .\release.ps1 v0.5.7 "release notes here"
param(
    [Parameter(Mandatory=$true)]
    [string]$Version,
    [string]$Notes = ""
)

$ErrorActionPreference = "Stop"
$owner = "aues6uen11z"
$repo = "palworld-save-relay"
$giteeApi = "https://gitee.com/api/v5"
$binary = "palworld-save-relay.exe"

# Read Gitee token.
$token = Get-Content ".gitee_token" -Raw
if (-not $token) { throw "Gitee token not found in .gitee_token" }

# 1. Build binary with version.
Write-Output "=== Building $binary ($Version) ==="
$env:CGO_ENABLED = 1
Push-Location frontend; npm run build 2>&1 | Out-Null; Pop-Location
go build -ldflags="-w -s -H windowsgui -X main.Version=$Version" -trimpath -o $binary .
if ($LASTEXITCODE -ne 0) { throw "Build failed" }
Write-Output "  built $([math]::Round((Get-Item $binary).Length/1MB,1))MB"

# 2. Create version.txt and release-note.txt.
Set-Content -Path "version.txt" -Value $Version -NoNewline -Encoding ASCII
if ($Notes) {
    Set-Content -Path "release-note.txt" -Value "$Version`n`n$Notes" -Encoding UTF8
}

# 3. Git tag and push to GitHub.
Write-Output "`n=== GitHub: tag + push ==="
git tag $Version
git push origin master $Version

# 4. GitHub release.
Write-Output "`n=== GitHub: release ==="
$ghArgs = @("release", "create", $Version, "--repo", "Aues6uen11Z/palworld-save-relay", "--title", $Version)
if ($Notes) { $ghArgs += @("--notes", $Notes) } else { $ghArgs += @("--notes", $Version) }
$ghArgs += @($binary, "version.txt")
if ($Notes) { $ghArgs += "release-note.txt" }
& gh @ghArgs
Write-Output "  GitHub release created"

# 5. Push to Gitee.
Write-Output "`n=== Gitee: push code + tags ==="
$giteeUrl = "https://${owner}:${token}@gitee.com/${owner}/${repo}.git"
git remote remove gitee 2>$null
git remote add gitee $giteeUrl
git push gitee master --force
git push gitee $Version
# Update 'latest' tag.
git tag -d latest 2>$null
git tag latest HEAD
git push gitee latest --force
git tag -d latest

# 6. Update version.txt raw file in Gitee repo.
Write-Output "`n=== Gitee: update version.txt raw file ==="
$ver = Get-Content "version.txt" -Raw
$b64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($ver))
# Get current file SHA (if exists).
try {
    $r = Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/contents/version.txt" -Method GET -Body @{access_token=$token} -UseBasicParsing -TimeoutSec 10
    $sha = ($r.Content | ConvertFrom-Json).sha
    $body = @{access_token=$token; content=$b64; message="Update version.txt to $Version"; branch="master"; sha=$sha} | ConvertTo-Json
    Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/contents/version.txt" -Method PUT -Body $body -ContentType "application/json" -UseBasicParsing -TimeoutSec 10 | Out-Null
    Write-Output "  version.txt updated (SHA=$sha)"
} catch {
    $body = @{access_token=$token; content=$b64; message="Add version.txt $Version"; branch="master"} | ConvertTo-Json
    Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/contents/version.txt" -Method POST -Body $body -ContentType "application/json" -UseBasicParsing -TimeoutSec 10 | Out-Null
    Write-Output "  version.txt created"
}

# 7. Update Gitee release assets.
Write-Output "`n=== Gitee: update release assets ==="
$r = Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/releases" -Method GET -Body @{access_token=$token} -UseBasicParsing -TimeoutSec 10
$releases = $r.Content | ConvertFrom-Json
$latest = $releases | Where-Object { $_.tag_name -eq "latest" } | Select-Object -First 1
if (-not $latest) { throw "Gitee 'latest' release not found" }
$releaseId = $latest.id
Write-Output "  release id=$releaseId"

# Delete old assets (except archives).
foreach ($asset in $latest.assets) {
    if ($asset.name -match "\.(zip|tar\.gz)$") { continue }
    try {
        Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/releases/$releaseId/attach_files/$($asset.id)?access_token=$token" -Method DELETE -UseBasicParsing -TimeoutSec 10 | Out-Null
        Write-Output "  deleted old $($asset.name)"
    } catch {}
}

# Upload new assets.
foreach ($file in @("version.txt", $binary)) {
    if (Test-Path $file) {
        $result = curl.exe -s -X POST "$giteeApi/repos/$owner/$repo/releases/$releaseId/attach_files" -F "access_token=$token" -F "file=@$file" 2>&1
        if ($result -match '"id"') {
            Write-Output "  uploaded $file"
        } else {
            Write-Output "  FAILED $file`: $result"
        }
    }
}
if ($Notes -and (Test-Path "release-note.txt")) {
    $result = curl.exe -s -X POST "$giteeApi/repos/$owner/$repo/releases/$releaseId/attach_files" -F "access_token=$token" -F "file=@release-note.txt" 2>&1
    Write-Output "  uploaded release-note.txt"
}

Write-Output "`n=== Done! ==="
Write-Output "GitHub: https://github.com/Aues6uen11Z/$repo/releases/tag/$Version"
Write-Output "Gitee:  https://gitee.com/$owner/$repo/releases/tag/latest"
Write-Output "Gitee raw: https://gitee.com/$owner/$repo/raw/master/version.txt"
