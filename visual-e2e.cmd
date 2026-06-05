@echo off
setlocal
cd /d "%~dp0test\visual"
if not exist node_modules (
  echo == npm ci ==
  call npm install
  if errorlevel 1 exit /b 1
  call npx playwright install chromium
  if errorlevel 1 exit /b 1
)
echo == playwright visual e2e (not run on publish / git-deploy) ==
call npx playwright test %*
exit /b %errorlevel%
