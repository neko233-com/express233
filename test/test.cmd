@echo off
setlocal
cd /d "%~dp0.."
echo == go test ./... ==
set "CGO_ENABLED=0"
go test ./... -count=1 %*
if errorlevel 1 exit /b 1
echo OK
