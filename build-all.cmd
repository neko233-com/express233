@echo off
setlocal
cd /d "%~dp0"

for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set COMMIT=%%i
if not defined COMMIT set COMMIT=local
if not defined VERSION set VERSION=0.0.0-local
for /f "tokens=1-3 delims=/ " %%a in ('echo %date%') do set BUILD_DATE=%%c-%%a-%%b
set LDFLAGS=-s -w -X github.com/neko233-com/express233/internal/version.Version=%VERSION% -X github.com/neko233-com/express233/internal/version.Commit=%COMMIT%

echo == build-all ==
if not exist bin mkdir bin
go build -ldflags "%LDFLAGS%" -o bin\express233.exe .\cmd\express233
if errorlevel 1 exit /b 1
go build -ldflags "%LDFLAGS%" -o bin\express233-server.exe .\cmd\express233-server
if errorlevel 1 exit /b 1
echo OK: bin\express233.exe bin\express233-server.exe
