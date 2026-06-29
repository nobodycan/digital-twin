param(
    [int]$Port = 18080,
    [string]$BaseUrl = "https://api.deepseek.com",
    [string]$Model = "deepseek-v4-pro",
    [string]$ApiKey = $env:DIGITAL_TWIN_LLM_API_KEY,
    [string]$FallbackPolicy = "fail_closed",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$dataDir = Join-Path $repoRoot "data"
$logDir = Join-Path $dataDir "logs"
$pidFile = Join-Path $dataDir "server.pid.json"
$serverLog = Join-Path $logDir ("server-{0}.log" -f $Port)
$errorLog = Join-Path $logDir ("server-{0}.err.log" -f $Port)
$browserUrl = "http://localhost:$Port/app"
$conversationUrl = "http://localhost:$Port"
$smokeCommand = ".\scripts\smoke-conversation.ps1 -BaseUrl $conversationUrl"

if ([string]::IsNullOrWhiteSpace($ApiKey)) {
    throw "Missing API key. Set DIGITAL_TWIN_LLM_API_KEY or pass -ApiKey."
}

New-Item -ItemType Directory -Path $logDir -Force | Out-Null

if (Test-Path -LiteralPath $pidFile) {
    $hasRunningTrackedProcess = $false
    $trackedServerPid = $null
    try {
        $existing = Get-Content -LiteralPath $pidFile -Raw -Encoding UTF8 | ConvertFrom-Json
        $existingProcess = Get-Process -Id $existing.ServerPid -ErrorAction SilentlyContinue
        if ($existingProcess) {
            $hasRunningTrackedProcess = $true
            $trackedServerPid = $existing.ServerPid
            throw "tracked process is still running"
        }
        Remove-Item -LiteralPath $pidFile -ErrorAction SilentlyContinue
    } catch {
        if ($hasRunningTrackedProcess) {
            throw "A digital-twin server is already tracked in $pidFile with PID $trackedServerPid. Run .\scripts\stop-server.ps1 first."
        }
        Remove-Item -LiteralPath $pidFile -ErrorAction SilentlyContinue
    }
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

$summary = [PSCustomObject]@{
    Port = $Port
    Provider = "openai-compatible"
    BaseUrl = $BaseUrl
    Model = $Model
    FallbackPolicy = $FallbackPolicy
    BrowserUrl = $browserUrl
    ConversationUrl = $conversationUrl
    SmokeCommand = $smokeCommand
    PidFile = $pidFile
    LogFile = $serverLog
}

if ($DryRun) {
    $summary | Format-List | Out-String | Write-Output
    Write-Output "Dry run only. Server not started."
    exit 0
}

$command = @"
Set-Location '$repoRoot'
`$env:DIGITAL_TWIN_SERVER_PORT='$Port'
`$env:DIGITAL_TWIN_LLM_PROVIDER='openai-compatible'
`$env:DIGITAL_TWIN_LLM_BASE_URL='$BaseUrl'
`$env:DIGITAL_TWIN_LLM_MODEL='$Model'
`$env:DIGITAL_TWIN_LLM_API_KEY='$ApiKey'
`$env:DIGITAL_TWIN_LLM_FALLBACK_POLICY='$FallbackPolicy'
go run ./cmd/server
"@

$process = Start-Process -FilePath "powershell" `
    -ArgumentList @("-NoProfile", "-Command", $command) `
    -WorkingDirectory $repoRoot `
    -WindowStyle Hidden `
    -RedirectStandardOutput $serverLog `
    -RedirectStandardError $errorLog `
    -PassThru

Start-Sleep -Milliseconds 800
if ($process.HasExited) {
    $stderr = if (Test-Path -LiteralPath $errorLog) { Get-Content -LiteralPath $errorLog -Raw -Encoding UTF8 } else { "" }
    throw "digital-twin server exited early. See $errorLog`n$stderr"
}

$record = [PSCustomObject]@{
    ServerPid = $process.Id
    Port = $Port
    BrowserUrl = $browserUrl
    ConversationUrl = $conversationUrl
    SmokeCommand = $smokeCommand
    FallbackPolicy = $FallbackPolicy
    BaseUrl = $BaseUrl
    Model = $Model
    LogFile = $serverLog
    ErrorLog = $errorLog
}
$record | ConvertTo-Json | Set-Content -LiteralPath $pidFile -Encoding UTF8

$record | Format-List | Out-String | Write-Output
Write-Output "ServerPid: $($process.Id)"
Write-Output "Open: $browserUrl"
Write-Output "Smoke: $smokeCommand"
