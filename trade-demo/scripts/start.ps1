$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$binDir = Join-Path $root 'bin'
$logDir = Join-Path $root 'logs'
$dataDir = Join-Path $root 'data'
New-Item -ItemType Directory -Force -Path $binDir,$logDir,$dataDir | Out-Null

$services = @(
  @{ Name='product'; Port=8031 },
  @{ Name='inventory'; Port=8032 },
  @{ Name='order'; Port=8033 },
  @{ Name='payment'; Port=8034 },
  @{ Name='saga'; Port=8035 }
)
foreach ($service in $services) {
  foreach ($port in @($service.Port, ($service.Port + 1000))) {
    $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
    if ($listener) { throw "Port $port is already listening; stop the existing service before starting this stack." }
  }
}
Push-Location $root
try {
  go build -buildvcs=false -o (Join-Path $binDir 'trade-demo.exe') .
  if ($LASTEXITCODE -ne 0) { throw 'Go build failed; services were not started.' }
} finally { Pop-Location }
$pids = @()
try {
foreach ($service in $services) {
  $envVars = @{
    TRADE_ADDR = "127.0.0.1:$($service.Port)"
    TRADE_GRPC_ADDR = "127.0.0.1:$($service.Port + 1000)"
    TRADE_DATA_FILE = (Join-Path $dataDir "$($service.Name).json")
    PRODUCT_GRPC_ADDR = '127.0.0.1:9031'
    INVENTORY_GRPC_ADDR = '127.0.0.1:9032'
    ORDER_GRPC_ADDR = '127.0.0.1:9033'
    PAYMENT_GRPC_ADDR = '127.0.0.1:9034'
    SAGA_GRPC_ADDR = '127.0.0.1:9035'
  }
  $proc = Start-Process -FilePath (Join-Path $binDir 'trade-demo.exe') -ArgumentList "-service=$($service.Name)" -WorkingDirectory $root -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logDir "$($service.Name).out.log") -RedirectStandardError (Join-Path $logDir "$($service.Name).err.log") -Environment $envVars
  $pids += [PSCustomObject]@{ Name=$service.Name; Id=$proc.Id; Port=$service.Port; GrpcPort=($service.Port + 1000); Executable=(Join-Path $binDir 'trade-demo.exe') }
}
$pids | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $root 'data\service-pids.json')
Start-Sleep -Seconds 1
foreach ($service in $services) {
  $healthy = $false
  for ($attempt = 0; $attempt -lt 10; $attempt++) {
    try { Invoke-RestMethod "http://127.0.0.1:$($service.Port)/healthz" -TimeoutSec 2 | Out-Null; $healthy = $true; break }
    catch { Start-Sleep -Milliseconds 300 }
  }
  if (-not $healthy) { throw "$($service.Name) did not become healthy; inspect logs/$($service.Name).err.log" }
}
Write-Host 'Kratos services started: HTTP 8031-8035, gRPC 9031-9035.'
} catch {
  foreach ($saved in $pids) { Stop-Process -Id $saved.Id -ErrorAction SilentlyContinue }
  Remove-Item -LiteralPath (Join-Path $root 'data\service-pids.json') -ErrorAction SilentlyContinue
  throw
}
