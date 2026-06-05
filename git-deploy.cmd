@echo off
REM 本地模拟 GitHub Actions：vet、测试、构建、冒烟（不含 visual-e2e；发布不依赖浏览器测试）
call "%~dp0deploy-github\deploy-github.cmd" %*
if errorlevel 1 exit /b 1
if /i "%~1"=="--push" (
  echo == git push ==
  git push -u origin HEAD
  if errorlevel 1 exit /b 1
)
