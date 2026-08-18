$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$replacement = Join-Path $PSScriptRoot "build_license_generators.ps1"
Write-Warning "build_poc_license_generator.ps1 is deprecated; building the unified Ed25519 License Issuer."
& $replacement
if ($LASTEXITCODE -ne 0) {
    throw "Unified License Generator build failed"
}
