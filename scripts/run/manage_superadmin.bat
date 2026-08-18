@echo off
setlocal
chcp 65001 >nul
cd /d "%~dp0"
title NMS SuperAdmin 本機維護

if not exist "%~dp0tools\superadmin-local.exe" (
  echo 找不到 tools\superadmin-local.exe，請確認發布包完整。
  echo.
  pause
  exit /b 1
)

echo 請先停止 NMS 服務，再繼續操作。
echo 此工具會自動尋找 data\nms.db 並判斷要初始化或變更密碼。
echo.
"%~dp0tools\superadmin-local.exe"
set "EXIT_CODE=%ERRORLEVEL%"
echo.
pause
exit /b %EXIT_CODE%
