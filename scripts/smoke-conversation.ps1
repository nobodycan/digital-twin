param(
    [string]$BaseUrl = "",
    [int]$Port = 8080,
    [string]$ApiKey = $env:DIGITAL_TWIN_SERVER_API_KEY,
    [string]$RuntimeDataDir = $env:DIGITAL_TWIN_RUNTIME_DATA,
    [string]$ConversationID = "smoke-conversation",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $BaseUrl = "http://localhost:$Port"
}
$BaseUrl = $BaseUrl.TrimEnd("/")

if ([string]::IsNullOrWhiteSpace($RuntimeDataDir)) {
    $RuntimeDataDir = Join-Path (Split-Path -Parent $PSScriptRoot) "data\runtime"
}
if (-not $PSBoundParameters.ContainsKey("ConversationID") -and $ConversationID -eq "smoke-conversation") {
    $ConversationID = "smoke-conversation-{0}" -f ([DateTime]::UtcNow.ToString("yyyyMMddHHmmss"))
}

$headers = @{
    "Content-Type" = "application/json"
}
if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
    $headers["Authorization"] = "Bearer $ApiKey"
}

function New-TurnBody {
    param(
        [string]$TurnID,
        [string]$AttemptID,
        [string]$MessageID,
        [string]$Content
    )

    $timestamp = [DateTime]::UtcNow.ToString("o")
    @{
        conversation_id = $ConversationID
        tenant_id = "tenant-1"
        user_id = "user-1"
        turn_id = $TurnID
        attempt_id = $AttemptID
        message = @{
            id = $MessageID
            role = "user"
            content = $Content
            created_at = $timestamp
        }
    } | ConvertTo-Json -Depth 4 -Compress
}

function Assert-Contains {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Label
    )

    if (-not $Text.Contains($Pattern)) {
        throw "$Label missing pattern: $Pattern"
    }
}

function Load-ConversationDocument {
    $matches = @(Get-ChildItem -Path $RuntimeDataDir -Recurse -Filter "$ConversationID.json" -ErrorAction SilentlyContinue)
    if ($matches.Count -eq 0) {
        throw "Conversation file not found: $ConversationID.json under $RuntimeDataDir"
    }
    if ($matches.Count -gt 1) {
        throw "Conversation file is ambiguous: found $($matches.Count) matches for $ConversationID.json under $RuntimeDataDir"
    }
    $path = $matches[0].FullName
    $raw = Get-Content -LiteralPath $path -Raw -Encoding UTF8
    return [PSCustomObject]@{
        Path = $path
        Json = $raw | ConvertFrom-Json
    }
}

function Get-RuntimeStatus {
    try {
        return Invoke-RestMethod -Method Get -Uri "$BaseUrl/runtime/status" -Headers $headers
    } catch {
        throw "Provider diagnostic unavailable. This smoke script only reports sanitized runtime status."
    }
}

function Invoke-StreamTurn {
    param(
        [string]$TurnID,
        [string]$AttemptID,
        [string]$MessageID,
        [string]$Content
    )

    $body = New-TurnBody -TurnID $TurnID -AttemptID $AttemptID -MessageID $MessageID -Content $Content
    $bodyFile = [System.IO.Path]::GetTempFileName()
    $stdoutFile = [System.IO.Path]::GetTempFileName()
    $stderrFile = [System.IO.Path]::GetTempFileName()
    [System.IO.File]::WriteAllText($bodyFile, $body, [System.Text.UTF8Encoding]::new($false))
    $arguments = @(
        "--silent",
        "--show-error",
        "--no-buffer",
        "-X", "POST",
        "-H", "Content-Type: application/json"
    )
    if (-not [string]::IsNullOrWhiteSpace($ApiKey)) {
        $arguments += @("-H", "Authorization: Bearer $ApiKey")
    }
    $arguments += @(
        "--data-binary", "@$bodyFile",
        "$BaseUrl/chat/stream"
    )
    try {
        $process = Start-Process -FilePath "curl.exe" `
            -ArgumentList $arguments `
            -NoNewWindow `
            -Wait `
            -PassThru `
            -RedirectStandardOutput $stdoutFile `
            -RedirectStandardError $stderrFile
        $response = Get-Content -LiteralPath $stdoutFile -Raw -Encoding UTF8
    } catch {
        throw "Provider diagnostic failed while opening the stream. Output is sanitized; check runtime status, API key, base URL, model, and fallback_policy."
    } finally {
        Remove-Item -LiteralPath $bodyFile -ErrorAction SilentlyContinue
        $stderr = if (Test-Path -LiteralPath $stderrFile) {
            Get-Content -LiteralPath $stderrFile -Raw -Encoding UTF8
        } else {
            ""
        }
        Remove-Item -LiteralPath $stdoutFile -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $stderrFile -ErrorAction SilentlyContinue
    }
    if ($process.ExitCode -ne 0) {
        throw "Turn $TurnID/$AttemptID failed to stream; curl exited with code $($process.ExitCode). $stderr"
    }
    return $response.Trim()
}

$summary = [PSCustomObject]@{
    BaseUrl = $BaseUrl
    RuntimeDataDir = $RuntimeDataDir
    ConversationID = $ConversationID
    HasApiKey = -not [string]::IsNullOrWhiteSpace($ApiKey)
}

if ($DryRun) {
    $summary | Format-List | Out-String | Write-Output
    Write-Output "Dry run only. No requests sent."
    exit 0
}

$summary | Format-List | Out-String | Write-Output

$runtimeStatus = Get-RuntimeStatus
Write-Output "Provider diagnostic"
Write-Output ("provider={0} model={1} generation_mode_hint={2} fallback_policy={3} sanitized=true" -f `
    $runtimeStatus.provider,
    $runtimeStatus.model,
    $runtimeStatus.generation_mode_hint,
    $runtimeStatus.fallback_policy)

$turn1 = Invoke-StreamTurn -TurnID "turn-1" -AttemptID "attempt-1" -MessageID "msg-1" -Content "smoke turn one"
Assert-Contains -Text $turn1 -Pattern "event: message_completed" -Label "turn-1 stream"
Assert-Contains -Text $turn1 -Pattern "event: done" -Label "turn-1 stream"

$turn2 = Invoke-StreamTurn -TurnID "turn-2" -AttemptID "attempt-1" -MessageID "msg-2" -Content "smoke turn two"
Assert-Contains -Text $turn2 -Pattern "event: message_completed" -Label "turn-2 stream"
Assert-Contains -Text $turn2 -Pattern "event: done" -Label "turn-2 stream"

$replay = Invoke-StreamTurn -TurnID "turn-2" -AttemptID "attempt-2" -MessageID "msg-2" -Content "smoke turn two"
Assert-Contains -Text $replay -Pattern '"replayed":true' -Label "replay stream"
Assert-Contains -Text $replay -Pattern "event: message_completed" -Label "replay stream"

$document = Load-ConversationDocument
$conversation = $document.Json

if ($conversation.turns.Count -ne 2) {
    throw "Expected 2 persisted turns, got $($conversation.turns.Count)"
}
if ($conversation.messages.Count -ne 4) {
    throw "Expected 4 persisted messages after replay check, got $($conversation.messages.Count)"
}
if ($conversation.turns[0].status -ne "completed" -or $conversation.turns[1].status -ne "completed") {
    throw "Expected completed turns, got [$($conversation.turns[0].status), $($conversation.turns[1].status)]"
}

Write-Output "PASS: streamed two turns, validated provider diagnostic output, and verified durable history."
Write-Output "Conversation file: $($document.Path)"
