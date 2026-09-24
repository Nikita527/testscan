$ErrorActionPreference = "Stop"
# Bypass: powershell -ExecutionPolicy Bypass -File python/scripts/embed_bin.ps1
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$OutDir = Join-Path $RepoRoot "python/src/testscan_py/bin"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$Out = Join-Path $OutDir "testscan.exe"
Push-Location $RepoRoot
try {
    go build -o $Out ./cmd/testscan
} finally {
    Pop-Location
}
Write-Host "embedded: $Out"
