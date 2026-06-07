@echo off
REM 不含 test/visual Playwright；可视化验收请单独运行 visual-e2e.cmd
setlocal
cd /d "%~dp0.."

for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=local
if not defined VERSION set VERSION=0.0.0-local
set LDFLAGS=-s -w -X github.com/neko233-com/express233/internal/version.Version=%VERSION% -X github.com/neko233-com/express233/internal/version.Commit=%COMMIT%

echo == vet ==
go vet ./...
if errorlevel 1 exit /b 1

echo == test ==
go test ./... -count=1
if errorlevel 1 exit /b 1

echo == build ==
if not exist bin mkdir bin
go build -ldflags "%LDFLAGS%" -o bin\express233-cli.exe .\cmd\express233-cli
if errorlevel 1 exit /b 1
go build -ldflags "%LDFLAGS%" -o bin\express233-server.exe .\cmd\express233-server
if errorlevel 1 exit /b 1

if exist scripts\ci-smoke.sh (
  where bash >nul 2>&1
  if not errorlevel 1 (
    echo == smoke ==
    bash scripts/ci-smoke.sh
    if errorlevel 1 exit /b 1
  ) else (
    echo skip smoke: bash not found
  )
)

echo deploy-github: OK (binaries in bin\)
