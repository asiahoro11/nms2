$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$backendSource = Join-Path $repoRoot "apps\backend"
$configPath = Join-Path $backendSource "config\config.go"
if (-not (Test-Path -LiteralPath $configPath)) {
    throw "config.go not found: $configPath"
}
$configContent = Get-Content -LiteralPath $configPath -Raw
$version = if ($configContent -match 'var Version = "([^"]+)"') { $matches[1] } else { "latest" }

# This operator-only workspace is ignored by Git and is never included in
# customer-facing NMS or License Generator packages.
$issuerWorkspace = Join-Path $repoRoot "runtime\data\license-issuer"
$privateKeyPath = Join-Path $issuerWorkspace "issuer_private.key"
$publicKeyPath = Join-Path $issuerWorkspace "issuer_public.key"
New-Item -ItemType Directory -Force -Path $issuerWorkspace | Out-Null

$privateExists = Test-Path -LiteralPath $privateKeyPath
$publicExists = Test-Path -LiteralPath $publicKeyPath
if ($privateExists -xor $publicExists) {
    throw "Issuer key pair is incomplete. Restore the missing matching key instead of generating a replacement."
}

if (-not $privateExists) {
    Write-Host "Creating a new offline issuer key pair..." -ForegroundColor Cyan
    Push-Location $backendSource
    try {
        go run ./cmd/license-keygen -private-output $privateKeyPath -public-output $publicKeyPath
        if ($LASTEXITCODE -ne 0) {
            throw "License key-pair generation failed"
        }
    }
    finally {
        Pop-Location
    }

    $currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    & icacls $privateKeyPath /inheritance:r /grant:r ($currentUser + ":(F)") | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to restrict issuer_private.key ACL"
    }
}

$publicLine = Get-Content -LiteralPath $publicKeyPath |
    Where-Object { $_.StartsWith("NMS_LICENSE_PUBLIC_KEY_B64=") } |
    Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($publicLine)) {
    throw "issuer_public.key does not contain NMS_LICENSE_PUBLIC_KEY_B64"
}
$licensePublicKeyB64 = ($publicLine -split "=", 2)[1].Trim()
try {
    $publicKeyBytes = [Convert]::FromBase64String($licensePublicKeyB64)
}
catch {
    throw "issuer_public.key contains invalid Base64"
}
if ($publicKeyBytes.Length -ne 32) {
    throw "issuer_public.key is not a 32-byte Ed25519 public key"
}

$oldProfile = [Environment]::GetEnvironmentVariable("NMS_RELEASE_PROFILE")
$oldPublicKey = [Environment]::GetEnvironmentVariable("NMS_LICENSE_PUBLIC_KEY_B64")
try {
    $env:NMS_RELEASE_PROFILE = "protected-release"
    $env:NMS_LICENSE_PUBLIC_KEY_B64 = $licensePublicKeyB64

    & (Join-Path $PSScriptRoot "build_windows_release.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "Windows NMS release build failed"
    }
    & (Join-Path $PSScriptRoot "build_license_generators.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "License Generator build failed"
    }
}
finally {
    $env:NMS_RELEASE_PROFILE = $oldProfile
    $env:NMS_LICENSE_PUBLIC_KEY_B64 = $oldPublicKey
}

$generatorSource = Join-Path $repoRoot "artifacts\license-generators\$version\windows-amd64\license-issuer.exe"
$generatorLauncher = Join-Path $repoRoot "artifacts\license-generators\$version\windows-amd64\start_license_generator.bat"
$generatorGuide = Join-Path $repoRoot "docs\LICENSE_GENERATOR_GUIDE_ZH-TW.md"
foreach ($required in @($generatorSource, $generatorLauncher, $generatorGuide)) {
    if (-not (Test-Path -LiteralPath $required)) {
        throw "Required License Generator output not found: $required"
    }
}
Copy-Item -LiteralPath $generatorSource -Destination (Join-Path $issuerWorkspace "license-issuer.exe") -Force
Copy-Item -LiteralPath $generatorLauncher -Destination (Join-Path $issuerWorkspace "start_license_generator.bat") -Force
Copy-Item -LiteralPath $generatorGuide -Destination (Join-Path $issuerWorkspace "LICENSE_GENERATOR_GUIDE_ZH-TW.md") -Force

$nmsLauncher = Join-Path $repoRoot "artifacts\windows\$version\start_nms.bat"
if (-not (Test-Path -LiteralPath $nmsLauncher)) {
    throw "NMS launcher was not produced: $nmsLauncher"
}

Write-Host ""
Write-Host "Ready-to-open Windows tools:" -ForegroundColor Green
Write-Host "NMS Server:        $nmsLauncher"
Write-Host "License Generator: $(Join-Path $issuerWorkspace 'start_license_generator.bat')"
Write-Host "Issuer private key remains only in: $privateKeyPath" -ForegroundColor Yellow
