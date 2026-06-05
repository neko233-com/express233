@echo off
setlocal
cd /d "%~dp0.."

if not defined EXPRESS233_ADDR set "EXPRESS233_ADDR=127.0.0.1:23380"
if not defined EXPRESS233_DATA set "EXPRESS233_DATA=%CD%\.data"
if not defined EXPRESS233_WEB_DIR set "EXPRESS233_WEB_DIR=%CD%\internal\api\web"
if not exist "%EXPRESS233_DATA%" mkdir "%EXPRESS233_DATA%"

for /f "tokens=1,2 delims=:" %%a in ("%EXPRESS233_ADDR%") do (
  set "HOST=%%a"
  set "PORT=%%b"
)
if "%HOST%"=="" set "HOST=127.0.0.1"
if "%PORT%"=="" set "PORT=23380"

call :warn_port_conflict %PORT%
if errorlevel 2 (
  echo [提示] 中央服已在运行: http://%HOST%:%PORT%
  exit /b 0
)
if errorlevel 1 (
  echo.
  echo [错误] 127.0.0.1:%PORT% 已被其他程序占用（常见: proxysss.exe）
  echo        请关闭该程序，或执行: set EXPRESS233_ADDR=127.0.0.1:32380
  echo.
  exit /b 1
)

echo.
echo -----------------
echo 访问地址 = http://%HOST%:%PORT%
echo 数据目录 = %EXPRESS233_DATA%
echo 静态热重载 = %EXPRESS233_WEB_DIR%
echo 默认账号 = root / root
echo -----------------
echo.

go run ./cmd/express233-server -addr %EXPRESS233_ADDR% -data "%EXPRESS233_DATA%" %*
exit /b %errorlevel%

:warn_port_conflict
powershell -NoProfile -Command ^
  "try { $r = Invoke-WebRequest -Uri 'http://127.0.0.1:%1/healthz' -TimeoutSec 2 -UseBasicParsing; if ($r.Content.Trim() -eq 'ok') { exit 2 } else { exit 1 } } catch { exit 0 }"
exit /b %errorlevel%
