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
$browserUrl = "http://127.0.0.1:$Port/app"
$conversationUrl = "http://127.0.0.1:$Port"
$smokeCommand = ".\scripts\smoke-conversation.ps1 -BaseUrl $conversationUrl"
$healthUrl = "http://127.0.0.1:$Port/health"

function Wait-ServerReady {
    param(
        [int]$ServerPort,
        [string]$Url,
        [System.Diagnostics.Process]$Process,
        [int]$Attempts = 40,
        [int]$DelayMilliseconds = 250
    )

    for ($attempt = 0; $attempt -lt $Attempts; $attempt++) {
        if ($Process.HasExited) {
            return $false
        }
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                return $true
            }
        } catch {
        }
        Start-Sleep -Milliseconds $DelayMilliseconds
    }

    return $false
}

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

@{
    DIGITAL_TWIN_SERVER_PORT = "$Port"
    DIGITAL_TWIN_LLM_PROVIDER = "openai-compatible"
    DIGITAL_TWIN_LLM_BASE_URL = $BaseUrl
    DIGITAL_TWIN_LLM_MODEL = $Model
    DIGITAL_TWIN_LLM_API_KEY = $ApiKey
    DIGITAL_TWIN_LLM_FALLBACK_POLICY = $FallbackPolicy
} | ForEach-Object {
    $script:originalEnv = @{}
    foreach ($entry in $_.GetEnumerator()) {
        $script:originalEnv[$entry.Key] = [Environment]::GetEnvironmentVariable($entry.Key, "Process")
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, "Process")
    }
}

try {
    $process = Start-Process -FilePath "go" `
        -ArgumentList @("run", "./cmd/server") `
        -WorkingDirectory $repoRoot `
        -WindowStyle Hidden `
        -RedirectStandardOutput $serverLog `
        -RedirectStandardError $errorLog `
        -PassThru
} finally {
    foreach ($entry in $originalEnv.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, "Process")
    }
}

if (-not (Wait-ServerReady -ServerPort $Port -Url $healthUrl -Process $process)) {
    $stderr = if (Test-Path -LiteralPath $errorLog) { Get-Content -LiteralPath $errorLog -Raw -Encoding UTF8 } else { "" }
    $stdout = if (Test-Path -LiteralPath $serverLog) { Get-Content -LiteralPath $serverLog -Raw -Encoding UTF8 } else { "" }
    if (-not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    }
    throw "digital-twin server failed readiness on $healthUrl. See $errorLog`n$stderr`n$stdout"
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
