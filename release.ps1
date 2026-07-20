# release.ps1 - One-command release to GitHub + Gitee
# Usage: .\release.ps1 v0.5.8 "release notes here"
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

$token = Get-Content ".gitee_token" -Raw
if (-not $token) { throw "Gitee token not found in .gitee_token" }

# 1. Build.
Write-Output "=== Build $binary ($Version) ==="
$env:CGO_ENABLED = 1
Push-Location frontend; npm run build 2>&1 | Out-Null; Pop-Location
go build -ldflags="-w -s -H windowsgui -X main.Version=$Version" -trimpath -o $binary .
if ($LASTEXITCODE -ne 0) { throw "Build failed" }
Write-Output "  built $([math]::Round((Get-Item $binary).Length/1MB,1))MB"

# 2. Update version.txt in repo.
Set-Content -Path "version.txt" -Value $Version -NoNewline -Encoding ASCII
git add version.txt
git commit -m "chore: bump version to $Version"

# 3. Tag and push to GitHub + Gitee.
Write-Output "`n=== Push to GitHub + Gitee ==="
git tag $Version
git push origin master $Version
$giteeUrl = "https://${owner}:${token}@gitee.com/${owner}/${repo}.git"
git remote remove gitee 2>$null
git remote add gitee $giteeUrl
git push gitee master $Version

# 4. GitHub release (binary only).
Write-Output "`n=== GitHub release ==="
$ghArgs = @("release", "create", $Version, "--repo", "Aues6uen11Z/palworld-save-relay", "--title", $Version)
if ($Notes) { $ghArgs += @("--notes", $Notes) } else { $ghArgs += @("--notes", $Version) }
$ghArgs += $binary
& gh @ghArgs

# 5. Gitee release (binary only).
Write-Output "`n=== Gitee release ==="
$body = "access_token=$token&tag_name=$Version&name=$Version&body=$([Uri]::EscapeDataString($Notes))&target_commitish=master"
$r = Invoke-WebRequest -Uri "$giteeApi/repos/$owner/$repo/releases" -Method POST -Body $body -ContentType "application/x-www-form-urlencoded" -UseBasicParsing -TimeoutSec 15
$releaseId = ($r.Content | ConvertFrom-Json).id
Write-Output "  release id=$releaseId"
$result = curl.exe -s -X POST "$giteeApi/repos/$owner/$repo/releases/$releaseId/attach_files" -F "access_token=$token" -F "file=@$binary" 2>&1
Write-Output "  binary: $(if ($result -match '"id"') {'OK'} else {$result})"

# 6. Verify.
Write-Output "`n=== Verify ==="
$gv = curl.exe -sL "https://gitee.com/$owner/$repo/raw/master/version.txt" 2>&1
$ghv = curl.exe -sL "https://raw.githubusercontent.com/Aues6uen11Z/$repo/main/version.txt" 2>&1
Write-Output "Gitee raw version.txt:  $gv"
Write-Output "GitHub raw version.txt: $ghv"
Write-Output "`nDone!"
Write-Output "GitHub: https://github.com/Aues6uen11Z/$repo/releases/tag/$Version"
Write-Output "Gitee:  https://gitee.com/$owner/$repo/releases/tag/$Version"
