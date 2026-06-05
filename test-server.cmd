@echo off
REM 运行 go test ./...（与 make test 相同）
call "%~dp0test\test.cmd" %*
