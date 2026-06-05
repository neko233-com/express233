@echo off
REM 启动中央服：自动停止本机已在跑的 express233-server，便于反复测试
REM 跳过自动停止: set EXPRESS233_NO_KILL=1
call "%~dp0run\run.cmd" %*
