$ErrorActionPreference = 'Stop'

# Default binds (override by setting env vars before calling this script)
if (-not $env:SIP_BIND_ADDR) { $env:SIP_BIND_ADDR = '0.0.0.0' }
if (-not $env:SIP_UDP_PORT) { $env:SIP_UDP_PORT = '5060' }
if (-not $env:API_BIND_ADDR) { $env:API_BIND_ADDR = '127.0.0.1' }
if (-not $env:API_PORT) { $env:API_PORT = '8081' }
if (-not $env:CONFIG_PATH) { $env:CONFIG_PATH = 'config.yaml' }
# Shared config across nodes (optional): same YAML from HTTPS, e.g. Git raw URL or internal config API.
# if ($env:CONFIG_HTTP_URL) { $env:CONFIG_HTTP_POLL_SECONDS = '60' }
# $env:CONFIG_HTTP_BEARER_TOKEN = '...'
# $env:CONFIG_HTTP_TLS_INSECURE = 'false'

Write-Host "[sipbridge] SIP_BIND_ADDR=$($env:SIP_BIND_ADDR)"
Write-Host "[sipbridge] SIP_UDP_PORT=$($env:SIP_UDP_PORT)"
Write-Host "[sipbridge] API_BIND_ADDR=$($env:API_BIND_ADDR)"
Write-Host "[sipbridge] API_PORT=$($env:API_PORT)"
Write-Host "[sipbridge] CONFIG_PATH=$($env:CONFIG_PATH)"
if ($env:CONFIG_HTTP_URL) {
  Write-Host "[sipbridge] CONFIG_HTTP_URL=$($env:CONFIG_HTTP_URL)"
  Write-Host "[sipbridge] CONFIG_HTTP_POLL_SECONDS=$($env:CONFIG_HTTP_POLL_SECONDS)"
}

$go = Get-Command go -ErrorAction SilentlyContinue

$goExe = $null
if ($go) {
  $goExe = $go.Source
} else {
  $fallback = 'C:\Program Files\Go\bin\go.exe'
  if (Test-Path $fallback) {
    $goExe = $fallback
  }
}

if (-not $goExe) {
  Write-Host "[sipbridge] ERROR: go.exe not found on PATH (and fallback path missing). Install Go 1.22+ and reopen your terminal/IDE." -ForegroundColor Red
  exit 1
}

Write-Host "[sipbridge] Starting..."
& $goExe run .\cmd\sipbridge
