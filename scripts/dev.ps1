param(
    [ValidateSet("build", "test", "test-race", "lint", "run", "clean")]
    [string]$Target = "test"
)

$ErrorActionPreference = "Stop"

switch ($Target) {
    "build" {
        New-Item -ItemType Directory -Force -Path "bin" | Out-Null
        go build -o "bin/digital-twin.exe" ./cmd/server
    }
    "test" {
        go test ./...
    }
    "test-race" {
        go test -race ./...
    }
    "lint" {
        go vet ./...
        if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
            golangci-lint run ./...
        } else {
            Write-Warning "golangci-lint is not installed; ran go vet only."
        }
    }
    "run" {
        go run ./cmd/server
    }
    "clean" {
        go clean
        Remove-Item -Recurse -Force -ErrorAction SilentlyContinue "bin"
        Remove-Item -Force -ErrorAction SilentlyContinue "*.exe"
    }
}
