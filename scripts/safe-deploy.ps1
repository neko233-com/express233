# safe-deploy.ps1 - safe Windows pull deployment (stage -> stop -> backup -> swap -> start -> health gate).
[CmdletBinding()]
param(
    [string]$ServerId = $env:EXPRESS233_SERVER_ID,
    [string]$Version = $env:VERSION,
    [string]$Root = $(if ($env:GAME_ROOT) { $env:GAME_ROOT } else { 'C:\express233\game-servers' }),
    [int]$StopTimeout = 10,
    [int]$BackupKeep = 5,
    [switch]$Backup,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
if ($ServerId -notmatch '^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$') { throw 'invalid or missing server_id' }
if ($StopTimeout -lt 1 -or $BackupKeep -lt 1) { throw 'timeouts and retention must be positive' }

$finalDir = Join-Path $Root $ServerId
$tempDir = Join-Path (Join-Path $Root '.tmp') $ServerId
$backupRoot = Join-Path (Join-Path $Root '.backup') $ServerId
$pidFile = Join-Path $finalDir 'run\server.pid'
$backupDir = $null
$env:SERVER_ID = $ServerId
$env:EXPRESS233_SERVER_ID = $ServerId

function Write-DeployLog([string]$Message) {
    Write-Host "[$(Get-Date -Format HH:mm:ss)] [$ServerId] $Message"
}

function Stop-GameServer {
    if (-not (Test-Path -LiteralPath $pidFile)) { Write-DeployLog 'no PID file; stop skipped'; return }
    $processId = [int](Get-Content -LiteralPath $pidFile -Raw).Trim()
    $process = Get-Process -Id $processId -ErrorAction SilentlyContinue
    if ($null -eq $process) { Remove-Item -LiteralPath $pidFile -Force; return }
    Write-DeployLog "stopping PID=$processId"
    Stop-Process -Id $processId -ErrorAction SilentlyContinue
    try { Wait-Process -Id $processId -Timeout $StopTimeout -ErrorAction Stop } catch { Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
}

function Sync-Release([string]$Source, [string]$Target) {
    New-Item -ItemType Directory -Force -Path (Join-Path $Target 'logs'), (Join-Path $Target 'run') | Out-Null
    & robocopy $Source $Target /MIR /XD logs run /R:1 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Host
    if ($LASTEXITCODE -ge 8) { throw "robocopy failed with exit code $LASTEXITCODE" }
}

function Restore-Backup {
    if (-not $backupDir -or -not (Test-Path -LiteralPath $backupDir)) { throw 'rollback unavailable: no backup' }
    Write-DeployLog 'restoring previous release'
    Stop-GameServer
    Sync-Release $backupDir $finalDir
    $restart = Join-Path $finalDir 'scripts\restart.ps1'
    if (-not (Test-Path -LiteralPath $restart)) { throw 'rollback restart.ps1 missing' }
    & powershell -NoProfile -NonInteractive -File $restart
    if ($LASTEXITCODE -ne 0) { throw 'rollback restart failed' }
}

Write-DeployLog 'Step 1/5: pulling to staging area'
if ($DryRun) { Write-DeployLog "[dry-run] would deploy $Version into $finalDir"; exit 0 }
if (Test-Path -LiteralPath $tempDir) { Remove-Item -LiteralPath $tempDir -Recurse -Force }
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
$pullArgs = @('pull', '--server-id', $ServerId, '--dest', $tempDir, '--skip-hook', '--retries', '1')
if ($Version) { $pullArgs += @('--version', $Version) }
& express233-cli @pullArgs
if ($LASTEXITCODE -ne 0) { throw 'pull to staging failed' }

Write-DeployLog 'Step 2/5: stopping previous release'
Stop-GameServer

Write-DeployLog 'Step 3/5: creating rollback copy'
if ($Backup -and (Test-Path -LiteralPath $finalDir)) {
    New-Item -ItemType Directory -Force -Path $backupRoot | Out-Null
    $backupDir = Join-Path $backupRoot "${ServerId}_$(Get-Date -Format yyyyMMdd_HHmmss)_$PID"
    New-Item -ItemType Directory -Force -Path $backupDir | Out-Null
    Sync-Release $finalDir $backupDir
    Get-ChildItem -LiteralPath $backupRoot -Directory | Sort-Object LastWriteTime -Descending | Select-Object -Skip $BackupKeep | Remove-Item -Recurse -Force
}

try {
    Write-DeployLog 'Step 4/5: swapping release files'
    Sync-Release $tempDir $finalDir
    Write-DeployLog 'Step 5/5: starting and health-checking release'
    $restart = Join-Path $finalDir 'scripts\restart.ps1'
    if (-not (Test-Path -LiteralPath $restart)) { throw 'scripts\restart.ps1 missing' }
    & powershell -NoProfile -NonInteractive -File $restart
    if ($LASTEXITCODE -ne 0) { throw 'restart failed' }
    $health = Join-Path $finalDir 'scripts\healthcheck.ps1'
    if (Test-Path -LiteralPath $health) {
        & powershell -NoProfile -NonInteractive -File $health
        if ($LASTEXITCODE -ne 0) { throw 'health check failed' }
    }
} catch {
    Write-DeployLog 'new release failed; starting rollback'
    Restore-Backup
    throw
} finally {
    if (Test-Path -LiteralPath $tempDir) { Remove-Item -LiteralPath $tempDir -Recurse -Force }
}
Write-DeployLog 'deploy complete'
