@echo off
REM 安装 GitHub CLI (gh) — 发布门禁、PR、Actions 日志
where gh >nul 2>&1 && (
  echo gh already installed:
  gh --version
  exit /b 0
)
where winget >nul 2>&1 && (
  echo Installing gh via winget...
  winget install --id GitHub.cli -e --accept-source-agreements --accept-package-agreements
  exit /b %errorlevel%
)
echo Install GitHub CLI manually: https://cli.github.com/
echo Then run: gh auth login
exit /b 1
