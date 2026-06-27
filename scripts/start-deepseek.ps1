param(
    [int]$Port = 8080,
    [string]$BaseUrl = "https://api.deepseek.com",
    [string]$Model = "deepseek-v4-pro",
    [string]$ApiKey = $env:DIGITAL_TWIN_LLM_API_KEY,
    [string]$FallbackPolicy = "fallback_to_local",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot

if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    throw "Missing API key. Set DIGITAL_TWIN_LLM_API_KEY or pass -ApiKey."
}

$listeners = @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
if ($listeners.Count -gt 0) {
    $processRows = foreach ($listener in $listeners) {
        $process = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
        [PSCustomObject]@{
            Port = $listener.LocalPort
            PID = $listener.OwningProcess
            Name = if ($process) { $process.ProcessName } else { "unknown" }
        }
    }

    $details = $processRows | Format-Table -AutoSize | Out-String
    throw "Port $Port is already in use.`n$details`nRun .\scripts\stop-server.ps1 -Port $Port first."
}

$env:DIGITAL_TWIN_SERVER_PORT = "$Port"
$env:DIGITAL_TWIN_LLM_PROVIDER = "openai-compatible"
$env:DIGITAL_TWIN_LLM_BASE_URL = $BaseUrl
$env:DIGITAL_TWIN_LLM_MODEL = $Model
$env:DIGITAL_TWIN_LLM_API_KEY = $ApiKey
$env:DIGITAL_TWIN_LLM_FALLBACK_POLICY = $FallbackPolicy

$summary = [PSCustomObject]@{
    Port = $Port
    Provider = $env:DIGITAL_TWIN_LLM_PROVIDER
    BaseUrl = $BaseUrl
    Model = $Model
    FallbackPolicy = $FallbackPolicy
    RepoRoot = $repoRoot
}

if ($DryRun) {
    $summary | Format-List | Out-String | Write-Output
    Write-Output "Dry run only. Server not started."
    exit 0
}

Push-Location $repoRoot
try {
    $summary | Format-List | Out-String | Write-Output
    go run ./cmd/server
} finally {
    Pop-Location
}
