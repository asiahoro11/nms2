$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$repoRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$configPath = Join-Path $repoRoot "apps\\backend\\config\\config.go"
$version = "latest"

if (Test-Path $configPath) {
    $configContent = Get-Content $configPath -Raw
    if ($configContent -match 'var Version = "([^"]+)"') {
        $version = $matches[1]
    }
}

Write-Host "Building version: $version"
$releaseProfile = [Environment]::GetEnvironmentVariable("NMS_RELEASE_PROFILE")
if ([string]::IsNullOrWhiteSpace($releaseProfile)) {
    $releaseProfile = "protected-release"
}
Write-Host "Release profile: $releaseProfile"

& (Join-Path $PSScriptRoot "build_windows_release.ps1")
& (Join-Path $PSScriptRoot "build_linux_release.ps1")

$windowsDir = Join-Path $repoRoot "artifacts\\windows\\$version"
$linuxDir = Join-Path $repoRoot "artifacts\\linux\\$version"

$windowsZip = Join-Path $repoRoot "artifacts\\windows\\$version.zip"
$linuxZip = Join-Path $repoRoot "artifacts\\linux\\$version.zip"

if (Test-Path $windowsZip) {
    Remove-Item -Force $windowsZip
}
if (Test-Path $linuxZip) {
    Remove-Item -Force $linuxZip
}

if (Test-Path $windowsDir) {
    Compress-Archive -Path (Join-Path $windowsDir "*") -DestinationPath $windowsZip -Force
}
if (Test-Path $linuxDir) {
    Compress-Archive -Path (Join-Path $linuxDir "*") -DestinationPath $linuxZip -Force
}

Write-Host "Windows artifact: $windowsDir"
Write-Host "Linux artifact:   $linuxDir"
Write-Host "Windows zip:      $windowsZip"
Write-Host "Linux zip:        $linuxZip"
