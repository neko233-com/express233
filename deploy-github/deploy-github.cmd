@echo off
setlocal
cd /d "%~dp0.."

for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=local
if not defined VERSION set VERSION=0.0.0-local

echo == vet ==
go vet ./...
if errorlevel 1 exit /b 1

echo == test ==
go test ./... -count=1
if errorlevel 1 exit /b 1

echo == build ==
if not exist bin mkdir bin
go build -o bin\express233.exe .\cmd\express233
if errorlevel 1 exit /b 1
go build -o bin\express233-server.exe .\cmd\express233-server
if errorlevel 1 exit /b 1

echo deploy-github: OK (binaries in bin\)
