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

set "UNICLI=unicli"
where unicli >nul 2>&1 || set "UNICLI=%USERPROFILE%\.local\bin\unicli.exe"

if not defined EXPRESS233_NO_KILL call :stop_previous_server %PORT%

call :warn_port_conflict %PORT%
if errorlevel 2 (
  echo.
  echo [错误] 无法停止已在运行的 express233-server ^(http://%HOST%:%PORT%^)
  echo        可手动: unicli psports %PORT%  然后  unicli kill ^<pid^>
  echo        或设置 EXPRESS233_NO_KILL=1 跳过自动停止
  echo.
  exit /b 1
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

:stop_previous_server
set "STOP_PORT=%~1"
powershell -NoProfile -Command ^
  "$port=%STOP_PORT%; $unicli='%UNICLI%';" ^
  "try { $r = Invoke-WebRequest -Uri \"http://127.0.0.1:$port/healthz\" -TimeoutSec 2 -UseBasicParsing; if ($r.Content.Trim() -ne 'ok') { exit 0 } } catch { exit 0 };" ^
  "$pids = @(Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique);" ^
  "if (-not $pids.Count) { $pids = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique) };" ^
  "foreach ($procId in $pids) {" ^
  "  $p = Get-Process -Id $procId -ErrorAction SilentlyContinue; if (-not $p) { continue };" ^
  "  $name = $p.ProcessName; $cmd = (Get-CimInstance Win32_Process -Filter \"ProcessId=$procId\" -ErrorAction SilentlyContinue).CommandLine;" ^
  "  if ($name -notlike '*express233-server*' -and $cmd -notlike '*express233-server*') { continue };" ^
  "  Write-Host \"[停止] express233-server PID $procId\";" ^
  "  if (Test-Path -LiteralPath $unicli) { & $unicli kill $procId 2>$null } else { Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue }" ^
  "}; Start-Sleep -Seconds 1"
exit /b 0

:warn_port_conflict
powershell -NoProfile -Command ^
  "try { $r = Invoke-WebRequest -Uri 'http://127.0.0.1:%1/healthz' -TimeoutSec 2 -UseBasicParsing; if ($r.Content.Trim() -eq 'ok') { exit 2 } else { exit 1 } } catch { exit 0 }"
exit /b %errorlevel%
