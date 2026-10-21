param([string]$Protoc='protoc')
$ErrorActionPreference='Stop'
$root=(Resolve-Path (Join-Path $PSScriptRoot '..')).Path
Push-Location $root
try {
  & $Protoc -I . -I ../review-service/third_party --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. --go-http_out=paths=source_relative:. api/trade/v1/trade.proto
  if ($LASTEXITCODE -ne 0) { throw 'Proto generation failed.' }
} finally { Pop-Location }
