@echo off
REM Made by YTSworks
REM YTS工作室製作
chcp 65001 >nul
set LANG=zh_TW.UTF-8
set LC_ALL=zh_TW.UTF-8

set "SCRIPT_DIR=%~dp0"
for %%I in ("%SCRIPT_DIR%nms_server.exe") do set "TARGET_EXE=%%~fI"

powershell -NoProfile -ExecutionPolicy Bypass -Command "$target=[System.IO.Path]::GetFullPath('%TARGET_EXE%'); $running=Get-Process nms_server -ErrorAction SilentlyContinue | Where-Object { $_.Path -and ([System.StringComparer]::OrdinalIgnoreCase.Equals([System.IO.Path]::GetFullPath($_.Path), $target)) } | Select-Object -First 1; if ($running) { Write-Host 'Management System is already running.' -ForegroundColor Yellow; Write-Host ('PID: ' + $running.Id) -ForegroundColor Yellow; Write-Host ('Executable: ' + $target) -ForegroundColor Yellow; exit 10 }"
if %ERRORLEVEL% EQU 10 (
    pause
    exit /b 1
)

echo Starting Management System (Windows)...
set "NMS_LAUNCHED_VIA_SCRIPT=1"
"%TARGET_EXE%"
pause
