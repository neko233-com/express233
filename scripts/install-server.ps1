# express233-server installer (Windows PowerShell)
# iwr -useb https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install-server.ps1 | iex
# iwr ... | iex; Install-Express233Server -Version v0.1.0

param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"
$BinaryName = "express233-server"
$Repo = "neko233-com/express233"

function Get-LatestVersion {
    try {
        $r = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        return ($r.tag_name -replace '^[vV]', '')
    } catch {
        return "0.1.0"
    }
}

function Install-Express233Server {
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
    Write-Host "Run: `$env:EXPRESS233_DATA=`"$env:USERPROFILE\\.express233-server`"; express233-server start"
    Write-Host "Status: express233-server status"
    Write-Host "Change port: express233-server set-port 32380"
    Write-Host "Hot reload server.yaml: express233-server reload-config"
    Write-Host "Force reset root password: express233-server reset-root-password --password <NEW_PASSWORD>"
}

if ($Version -eq "latest") {
    $Version = Get-LatestVersion
}
$Version = $Version -replace '^[vV]', ''

Write-Host "Installing express233-server v$Version ..."
Install-Express233Server -Ver $Version