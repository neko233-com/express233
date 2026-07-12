# express233-cli installer (Windows PowerShell)
# iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.ps1 | iex
# iwr ... | iex; Install-Express233Cli -Version v0.1.0

param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$BinaryName = "express233-cli"
$Repo = "neko233-com/express233"

function Get-LatestVersion {
    try {
        $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return ($r.tag_name -replace '^[vV]', '')
    } catch {
        return "0.1.0"
    }
}

function Install-Express233Cli {
    param([string]$Ver)

    $arch = "amd64"
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }

    $asset = "$BinaryName-windows-$arch.exe"
    $url = "https://github.com/$Repo/releases/download/v$Ver/$asset"
    $installDir = Join-Path $env:LOCALAPPDATA "express233"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    $dest = Join-Path $installDir "$BinaryName.exe"

    Write-Host "Downloading $url ..."
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
    $runnerUrl = "https://raw.githubusercontent.com/$Repo/v$Ver/scripts/safe-deploy.ps1"
    $runnerDest = Join-Path $installDir "safe-deploy.ps1"
    Invoke-WebRequest -Uri $runnerUrl -OutFile $runnerDest -UseBasicParsing
    Write-Host "Installed to $dest"
    Write-Host "Installed safe deployment runner to $runnerDest"
    Write-Host "Add to PATH: $installDir"
    Write-Host "Run: express233-cli version"
}

if ($Version -eq "latest") {
    $Version = Get-LatestVersion
}
$Version = $Version -replace '^[vV]', ''

Write-Host "Installing express233-cli v$Version ..."
Install-Express233Cli -Ver $Version
