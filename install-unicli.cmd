@echo off
REM 安装 neko233-com/unicli（供 run-server 自动停止旧进程等）
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/neko233-com/unicli/main/scripts/install.ps1 | iex"
