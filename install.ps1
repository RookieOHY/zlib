#Requires -Version 5.1
$ErrorActionPreference = "Stop"
try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 } catch {}

$Repo = "RookieOHY/zlib"
$Bin = "zlib.exe"
$InstallDir = Join-Path $env:USERPROFILE "bin"

# Detect architecture
switch ($env:PROCESSOR_ARCHITECTURE) {
    "ARM64" { $Arch = "arm64" }
    default { $Arch = "x86_64" }
}

# Newest non-draft release (pre-releases included)
$headers = @{ "User-Agent" = "zlib-installer" }
$rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases?per_page=1" -Headers $headers |
    Select-Object -First 1
$Version = $rel.tag_name
if (-not $Version) { throw "Failed to fetch the latest version" }

$want = "zlib_$($Version.TrimStart('v'))_windows_$Arch.zip"
$Asset = $rel.assets | Where-Object { $_.name -eq $want } | Select-Object -First 1
if (-not $Asset) { throw "Release $Version has no asset for windows/$Arch (looked for $want)" }

Write-Host "Installing $Bin $Version (windows/$Arch)..."

$tmp = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
try {
    $zip = Join-Path $tmp $Asset.name
    Invoke-WebRequest -Uri $Asset.browser_download_url -OutFile $zip -UserAgent "zlib-installer"
    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Move-Item (Join-Path $tmp $Bin) (Join-Path $InstallDir $Bin) -Force
}
finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# Add the install directory to the user PATH when missing
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ";") -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    Write-Host "Added $InstallDir to your user PATH. Open a new terminal for it to take effect."
}

Write-Host "Installed to $InstallDir\$Bin"
& (Join-Path $InstallDir $Bin) version
