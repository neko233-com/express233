@echo off
setlocal
cd /d "%~dp0"
call scripts\build-console.cmd
if errorlevel 1 exit /b 1
call npm --prefix test/browser ci --ignore-scripts
if errorlevel 1 exit /b 1
call npm --prefix test/browser run verify
exit /b %errorlevel%
