param(
    [int[]]$Port,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

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

        if ($process.ProcessName -notin @("server", "digital-twin")) {
            continue
        }

        [PSCustomObject]@{
            Port = $connection.LocalPort
            PID = $connection.OwningProcess
            Name = $process.ProcessName
        }
    }

    $targets |
        Sort-Object PID, Port -Unique
}

$targets = @(Get-ListenerProcess -Ports $Port)

if ($targets.Count -eq 0) {
    if ($Port -and $Port.Count -gt 0) {
        Write-Output "No digital-twin server process is listening on port(s): $($Port -join ', ')."
    } else {
        Write-Output "No listening digital-twin server process found."
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
