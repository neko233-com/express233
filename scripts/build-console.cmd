@echo off
setlocal
set "FRONTEND=%~dp0..\frontend"
call npm --prefix "%FRONTEND%" ci
if errorlevel 1 exit /b 1
call npm --prefix "%FRONTEND%" run build
exit /b %errorlevel%
