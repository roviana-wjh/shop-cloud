$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$pidFile = Join-Path $root 'data\service-pids.json'
if (-not (Test-Path -LiteralPath $pidFile)) { Write-Host 'No saved service PID file.'; exit 0 }
$services = Get-Content -Raw -LiteralPath $pidFile | ConvertFrom-Json
foreach ($service in $services) {
  $proc = Get-Process -Id $service.Id -ErrorAction SilentlyContinue
  if ($proc) {
    $expected = Join-Path $root 'bin\trade-demo.exe'
    if ($proc.Path -ne $expected) { throw "PID $($service.Id) no longer belongs to this trade-demo executable; refusing to stop it." }
    Stop-Process -Id $service.Id -Force
    Wait-Process -Id $service.Id -Timeout 10 -ErrorAction SilentlyContinue
    Write-Host "Stopped $($service.Name) ($($service.Id))."
  }
}
Remove-Item -LiteralPath $pidFile -ErrorAction SilentlyContinue
