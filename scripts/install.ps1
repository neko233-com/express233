# express233 CLI installer (Windows PowerShell)
# iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.ps1 | iex
# iwr ... | iex; install-express233 -Version v0.1.0

param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$BinaryName = "express233"
$Repo = "neko233-com/express233"

function Get-LatestVersion {
    try {
        $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return ($r.tag_name -replace '^[vV]', '')
    } catch {
        return "0.1.0"
    }
}

function Install-Express233 {
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
    Write-Host "Installed to $dest"
    Write-Host "Add to PATH: $installDir"
    Write-Host "Run: express233 version"
}

if ($Version -eq "latest") {
    $Version = Get-LatestVersion
}
$Version = $Version -replace '^[vV]', ''

Write-Host "Installing express233 v$Version ..."
Install-Express233 -Ver $Version
