# Synthetic-only lifecycle verification; never opens the retained preview database.
param(
    [string]$Image = 'energything:local',
    [ValidateSet('linux/amd64', 'linux/arm64')][string]$Platform = 'linux/amd64',
    [string]$BuildID = 'local'
)
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$arch = $Platform.Split('/')[-1]
$previous = @{}
Push-Location $root
try {
    foreach ($key in @('GOOS', 'GOARCH', 'CGO_ENABLED')) {
        $previous[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    }
    $env:GOOS = 'linux'
    $env:GOARCH = $arch
    $env:CGO_ENABLED = '0'
    go build -o "bin/probes/$arch/operations-probe" ./scripts/operations-probe
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic probe build failed' }
    python scripts/verify-operations.py --image $Image --platform $Platform --build-id $BuildID
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic container verification failed' }
} finally {
    foreach ($key in $previous.Keys) {
        [Environment]::SetEnvironmentVariable($key, $previous[$key], 'Process')
    }
    Pop-Location
}
