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

function Invoke-StreamTurn {
    param(
        [string]$TurnID,
        [string]$AttemptID,
        [string]$MessageID,
        [string]$Content
    )

    $body = New-TurnBody -TurnID $TurnID -AttemptID $AttemptID -MessageID $MessageID -Content $Content
    $response = Invoke-WebRequest -Method Post -Uri "$BaseUrl/chat/stream" -Headers $headers -Body $body
    if ($response.StatusCode -ne 200) {
        throw "Turn $TurnID/$AttemptID failed with status $($response.StatusCode)"
    }
    return [string]$response.Content
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
    $path = Join-Path $RuntimeDataDir "tenants\tenant-1\users\user-1\conversations\$ConversationID.json"
    if (-not (Test-Path -LiteralPath $path)) {
        throw "Conversation file not found: $path"
    }
    $raw = Get-Content -LiteralPath $path -Raw -Encoding UTF8
    return [PSCustomObject]@{
        Path = $path
        Json = $raw | ConvertFrom-Json
    }
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

$turn1 = Invoke-StreamTurn -TurnID "turn-1" -AttemptID "attempt-1" -MessageID "msg-1" -Content "smoke turn one"
Assert-Contains -Text $turn1 -Pattern "event: assistant_text_delta" -Label "turn-1 stream"
Assert-Contains -Text $turn1 -Pattern "event: message_completed" -Label "turn-1 stream"
Assert-Contains -Text $turn1 -Pattern "event: done" -Label "turn-1 stream"

$turn2 = Invoke-StreamTurn -TurnID "turn-2" -AttemptID "attempt-1" -MessageID "msg-2" -Content "smoke turn two"
Assert-Contains -Text $turn2 -Pattern "event: assistant_text_delta" -Label "turn-2 stream"
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
if ($conversation.turns[1].attempts.Count -lt 1) {
    throw "Expected attempts for turn-2"
}

Write-Output "PASS: streamed two turns, preserved durable history, and verified completed replay."
Write-Output "Conversation file: $($document.Path)"
