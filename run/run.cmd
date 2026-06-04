@echo off
setlocal
cd /d "%~dp0.."
if not defined EXPRESS233_DATA set "EXPRESS233_DATA=%CD%\.data"
if not exist "%EXPRESS233_DATA%" mkdir "%EXPRESS233_DATA%"
echo express233-server on :23380 (data: %EXPRESS233_DATA%)
go run ./cmd/express233-server -addr :23380 -data "%EXPRESS233_DATA%"
