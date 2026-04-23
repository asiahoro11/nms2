[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$env:LANG = "zh_TW.UTF-8"
$env:LC_ALL = "zh_TW.UTF-8"

$serverPath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "nms_server.exe"))
$runningInstance = Get-Process nms_server -ErrorAction SilentlyContinue | Where-Object {
    $_.Path -and ([System.StringComparer]::OrdinalIgnoreCase.Equals([System.IO.Path]::GetFullPath($_.Path), $serverPath))
} | Select-Object -First 1

if ($null -ne $runningInstance) {
    Write-Host "Management System is already running." -ForegroundColor Yellow
    Write-Host "PID: $($runningInstance.Id)" -ForegroundColor Yellow
    Write-Host "Executable: $serverPath" -ForegroundColor Yellow
    exit 1
}

Write-Host "Starting Management System (Windows)..."
$env:NMS_LAUNCHED_VIA_SCRIPT = "1"
& $serverPath
