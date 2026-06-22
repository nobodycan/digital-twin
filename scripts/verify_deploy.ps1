param()

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$deploy = Join-Path $root "deploy"

$required = @(
  "Dockerfile",
  "docker-compose.yml",
  ".env.example",
  "README.md"
)

foreach ($name in $required) {
  $path = Join-Path $deploy $name
  if (-not (Test-Path $path)) {
    throw "missing deploy artifact: deploy/$name"
  }
}

$compose = Get-Content (Join-Path $deploy "docker-compose.yml") -Raw
foreach ($forbidden in @("postgres", "sqlite", "mysql", "redis")) {
  if ($compose -match $forbidden) {
    throw "compose contains forbidden external service or database reference: $forbidden"
  }
}
if ($compose -notmatch "/ready") {
  throw "compose healthcheck must probe /ready"
}

$envExample = Get-Content (Join-Path $deploy ".env.example") -Raw
if ($envExample -match "sk-[A-Za-z0-9]" -or $envExample -match "secret-[A-Za-z0-9]" -or $envExample -match "Bearer\s+\S+") {
  throw "deploy/.env.example appears to contain a real secret"
}

Write-Host "deploy static verification passed"
