param(
    [int[]]$Port,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$pidFile = Join-Path $repoRoot "data\server.pid.json"

function Stop-TrackedProcess {
    param(
        [string]$Path
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return $false
    }

    $tracked = Get-Content -LiteralPath $Path -Raw -Encoding UTF8 | ConvertFrom-Json
    $process = Get-Process -Id $tracked.ServerPid -ErrorAction SilentlyContinue
    if (-not $process) {
        Remove-Item -LiteralPath $Path -ErrorAction SilentlyContinue
        return $false
    }

    if ($DryRun) {
        $tracked | Format-List | Out-String | Write-Output
        Write-Output "Dry run only. No process stopped."
        return $true
    }

    Stop-Process -Id $tracked.ServerPid -Force
    Remove-Item -LiteralPath $Path -ErrorAction SilentlyContinue
    $tracked | Format-List | Out-String | Write-Output
    Write-Output ("Stopped tracked ServerPid: {0}" -f $tracked.ServerPid)
    return $true
}

function Get-ListenerProcess {
    param(
        [int[]]$Ports
    )

    $connections = if ($Ports -and $Ports.Count -gt 0) {
        @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $Ports -contains $_.LocalPort })
    } else {
        @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue)
    }

    $targets = foreach ($connection in $connections) {
        $process = Get-Process -Id $connection.OwningProcess -ErrorAction SilentlyContinue
        if (-not $process) {
            continue
        }

        [PSCustomObject]@{
            Port = $connection.LocalPort
            PID = $connection.OwningProcess
            Name = $process.ProcessName
        }
    }

    $targets | Sort-Object PID, Port -Unique
}

if (Stop-TrackedProcess -Path $pidFile) {
    exit 0
}

$targets = @(Get-ListenerProcess -Ports $Port)
if ($targets.Count -eq 0) {
    if ($Port -and $Port.Count -gt 0) {
        Write-Output "No tracked or listening server process found on port(s): $($Port -join ', ')."
    } else {
        Write-Output "No tracked or listening server process found."
    }
    exit 0
}

if ($DryRun) {
    $targets | Format-Table -AutoSize | Out-String | Write-Output
    Write-Output "Dry run only. No process stopped."
    exit 0
}

$stopped = @()
foreach ($processId in ($targets.PID | Sort-Object -Unique)) {
    Stop-Process -Id $processId -Force
    $stopped += $processId
}

$targets | Format-Table -AutoSize | Out-String | Write-Output
Write-Output ("Stopped PID(s): {0}" -f ($stopped -join ", "))
