@echo off
setlocal
cd /d "%~dp0.."
echo == go test ./... ==
go test ./... -count=1 %*
if errorlevel 1 exit /b 1
echo OK
