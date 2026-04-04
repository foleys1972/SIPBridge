@echo off
setlocal

REM Default binds (override by setting env vars before calling this script)
if "%SIP_BIND_ADDR%"=="" set SIP_BIND_ADDR=0.0.0.0
if "%SIP_UDP_PORT%"=="" set SIP_UDP_PORT=5060
if "%API_BIND_ADDR%"=="" set API_BIND_ADDR=127.0.0.1
if "%API_PORT%"=="" set API_PORT=8081
if "%CONFIG_PATH%"=="" set CONFIG_PATH=config.yaml

echo [sipbridge] SIP_BIND_ADDR=%SIP_BIND_ADDR%
echo [sipbridge] SIP_UDP_PORT=%SIP_UDP_PORT%
echo [sipbridge] API_BIND_ADDR=%API_BIND_ADDR%
echo [sipbridge] API_PORT=%API_PORT%
echo [sipbridge] CONFIG_PATH=%CONFIG_PATH%

where go >nul 2>nul
set GOEXE=go
if errorlevel 1 (
  if exist "C:\Program Files\Go\bin\go.exe" (
    set GOEXE="C:\Program Files\Go\bin\go.exe"
  ) else (
    echo [sipbridge] ERROR: go.exe not found on PATH.
    echo [sipbridge] Install Go 1.22+ and reopen your terminal/IDE.
    exit /b 1
  )
)

echo [sipbridge] Starting...
%GOEXE% run .\cmd\sipbridge

endlocal
