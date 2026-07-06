# Made by YTSworks
# YTSå·¥ä½œå®¤è£½ä½œ
$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
# ============================================================================
# Management Server ç¹é?ä¸­æ?ç·¨ç¢¼ä¿®å¾©?³æœ¬
# è§?±º?·è??‚å‡º?¾ä?ç¢¼å??´ç?å¼å?æ­¢ç??é?
# ============================================================================

$ErrorActionPreference = "Continue"

Write-Host "================================================================" -ForegroundColor Green
Write-Host "         Management Server ç¹é?ä¸­æ?ç·¨ç¢¼ä¿®å¾©å·¥å…·                          " -ForegroundColor Green
Write-Host "================================================================" -ForegroundColor Green
Write-Host ""

# ============================================================================
# 1. è¨­å? PowerShell ?§åˆ¶?°ç·¨ç¢¼ç‚º UTF-8
# ============================================================================
Write-Host "æ­¥é? 1: è¨­å? PowerShell ?§åˆ¶?°ç·¨ç¢?.." -ForegroundColor Cyan

try {
    # è¨­å?è¼¸å‡ºç·¨ç¢¼??UTF-8
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    
    # è¨­å??§åˆ¶?°ä»£ç¢¼é???UTF-8 (65001)
    chcp 65001 | Out-Null
    
    Write-Host "??å·²è¨­å®šæ§?¶å°ç·¨ç¢¼??UTF-8" -ForegroundColor Green
}
catch {
    Write-Host "??è¨­å?ç·¨ç¢¼?‚ç™¼?ŸéŒ¯èª? $_" -ForegroundColor Red
}

Write-Host ""

# ============================================================================
# 2. æª¢æŸ¥ä¸¦ä¿®å¾?Go ç¨‹å??„ç·¨ç¢¼è¨­å®?
# ============================================================================
Write-Host "æ­¥é? 2: æª¢æŸ¥ Go ç¨‹å?ç·¨ç¢¼è¨­å?..." -ForegroundColor Cyan

$mainGoPath = Join-Path $PSScriptRoot "backend\main.go"

if (Test-Path $mainGoPath) {
    Write-Host "???¾åˆ° main.go æª”æ?" -ForegroundColor Green
    
    # æª¢æŸ¥?¯å¦å·²ç???UTF-8 è¨­å?
    $content = Get-Content $mainGoPath -Raw -Encoding UTF8
    
    if ($content -notmatch "SetConsoleOutputCP") {
        Write-Host "  ?€è¦æ·»??Windows UTF-8 ?¯æ´" -ForegroundColor Yellow
    }
    else {
        Write-Host "??å·²å???UTF-8 ?¯æ´ä»?¢¼" -ForegroundColor Green
    }
}
else {
    Write-Host "???¾ä???main.go æª”æ?" -ForegroundColor Red
}

Write-Host ""

# ============================================================================
# 3. ?µå»º?Ÿå??…è??³æœ¬ (UTF-8 ?°å?)
# ============================================================================
Write-Host "æ­¥é? 3: ?µå»º UTF-8 ?Ÿå??…è??³æœ¬..." -ForegroundColor Cyan

# Windows ?Ÿå??³æœ¬
$windowsStartScript = @'
@echo off
REM ============================================================================
REM Management Server Windows ?Ÿå??³æœ¬ (UTF-8 ?¯æ´)
REM ============================================================================

REM è¨­å??§åˆ¶?°ä»£ç¢¼é???UTF-8
chcp 65001 >nul

REM è¨­å??°å?è®Šæ•¸
set LANG=zh_TW.UTF-8
set LC_ALL=zh_TW.UTF-8

echo ================================================================
echo          Management Server ä¼ºæ??¨å??•ä¸­...
echo ================================================================
echo.

REM ?Ÿå?ä¼ºæ???
nms_server.exe

pause
'@

$startScriptPath = Join-Path $PSScriptRoot "start_nms_utf8.bat"
$windowsStartScript | Out-File -FilePath $startScriptPath -Encoding ASCII -Force

Write-Host "??å·²å‰µå»? start_nms_utf8.bat" -ForegroundColor Green

# PowerShell ?Ÿå??³æœ¬
$psStartScript = @'
# Management Server PowerShell ?Ÿå??³æœ¬ (UTF-8 ?¯æ´)

# è¨­å?ç·¨ç¢¼
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

# è¨­å??°å?è®Šæ•¸
$env:LANG = "zh_TW.UTF-8"
$env:LC_ALL = "zh_TW.UTF-8"

Write-Host "================================================================" -ForegroundColor Green
Write-Host "         Management Server ä¼ºæ??¨å??•ä¸­...                              " -ForegroundColor Green
Write-Host "================================================================" -ForegroundColor Green
Write-Host ""

# ?Ÿå?ä¼ºæ???
& ".\nms_server.exe"
'@

$psStartScriptPath = Join-Path $PSScriptRoot "start_nms_utf8.ps1"
$psStartScript | Out-File -FilePath $psStartScriptPath -Encoding UTF8 -Force

Write-Host "??å·²å‰µå»? start_nms_utf8.ps1" -ForegroundColor Green

Write-Host ""

# ============================================================================
# 4. ä¿®å¾©?€??PowerShell ?³æœ¬?„ç·¨ç¢?
# ============================================================================
Write-Host "æ­¥é? 4: æª¢æŸ¥ä¸¦ä¿®å¾?PowerShell ?³æœ¬ç·¨ç¢¼..." -ForegroundColor Cyan

$scripts = Get-ChildItem -Path $PSScriptRoot -Filter "*.ps1" -File

foreach ($script in $scripts) {
    if ($script.Name -eq "fix_chinese_encoding.ps1") {
        continue
    }
    
    try {
        # è®€?–ä¸¦?æ–°ä¿å???UTF-8 BOM
        $content = Get-Content $script.FullName -Raw -Encoding UTF8
        
        # ?¨è…³?¬é??­æ·»??UTF-8 è¨­å?ï¼ˆå??œé?æ²’æ?ï¼?
        if ($content -notmatch "\[Console\]::OutputEncoding") {
            $utf8Header = @"
# UTF-8 ç·¨ç¢¼è¨­å?
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
`$OutputEncoding = [System.Text.Encoding]::UTF8

"@
            $content = $utf8Header + $content
            Set-Content -Path $script.FullName -Value $content -Encoding UTF8 -Force
            Write-Host "  ??å·²ä¿®å¾? $($script.Name)" -ForegroundColor Green
        }
        else {
            Write-Host "  ??å·²æ­£ç¢? $($script.Name)" -ForegroundColor Gray
        }
    }
    catch {
        Write-Host "  ???•ç?å¤±æ?: $($script.Name) - $_" -ForegroundColor Red
    }
}

Write-Host ""

# ============================================================================
# 5. ?µå»º Linux ?Ÿå??³æœ¬
# ============================================================================
Write-Host "æ­¥é? 5: ?µå»º Linux UTF-8 ?Ÿå??³æœ¬..." -ForegroundColor Cyan

$linuxStartScript = @'
#!/bin/bash
# ============================================================================
# Management Server Linux ?Ÿå??³æœ¬ (UTF-8 ?¯æ´)
# ============================================================================

# è¨­å?èªè??°å??ºç?é«”ä¸­??UTF-8
export LANG=zh_TW.UTF-8
export LC_ALL=zh_TW.UTF-8

echo "================================================================"
echo "         Management Server ä¼ºæ??¨å??•ä¸­..."
echo "================================================================"
echo ""

# ?Ÿå?ä¼ºæ???
./nms_server
'@

$linuxStartScriptPath = Join-Path $PSScriptRoot "start_nms_utf8.sh"
$linuxStartScript | Out-File -FilePath $linuxStartScriptPath -Encoding UTF8 -Force

Write-Host "??å·²å‰µå»? start_nms_utf8.sh" -ForegroundColor Green

Write-Host ""

# ============================================================================
# 6. ?µå»ºæ¸¬è©¦?³æœ¬
# ============================================================================
Write-Host "æ­¥é? 6: ?µå»ºç·¨ç¢¼æ¸¬è©¦?³æœ¬..." -ForegroundColor Cyan

$testScript = @'
# UTF-8 ç·¨ç¢¼æ¸¬è©¦?³æœ¬
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

Write-Host "================================================================" -ForegroundColor Green
Write-Host "         ç¹é?ä¸­æ?ç·¨ç¢¼æ¸¬è©¦                                       " -ForegroundColor Green
Write-Host "================================================================" -ForegroundColor Green
Write-Host ""

Write-Host "æ¸¬è©¦å­—ä¸²:" -ForegroundColor Cyan
Write-Host "  ??ç¹é?ä¸­æ?é¡¯ç¤ºæ¸¬è©¦" -ForegroundColor Green
Write-Host "  ´ú¸Õ¦r¦ê: ÁcÅé¤¤¤å´ú¸Õ" -ForegroundColor Yellow
Write-Host "  ??Emoji: ????? ï? ?“¦ ??" -ForegroundColor Magenta
Write-Host ""

Write-Host "?§åˆ¶?°è?è¨?" -ForegroundColor Cyan
Write-Host "  ä»?¢¼?? $(chcp)" -ForegroundColor White
Write-Host "  è¼¸å‡ºç·¨ç¢¼: $([Console]::OutputEncoding.EncodingName)" -ForegroundColor White
Write-Host ""

Write-Host "¦pªG±z¥i¥H¬İ¨ì¥H¤W©Ò¦³¥¿½T¤å¦r¡Aªí¥Ü½s½X³]©w¦¨¥\¡I" -ForegroundColor Green
Write-Host ""
'@

$testScriptPath = Join-Path $PSScriptRoot "test_encoding.ps1"
$testScript | Out-File -FilePath $testScriptPath -Encoding UTF8 -Force

Write-Host "??å·²å‰µå»? test_encoding.ps1" -ForegroundColor Green

Write-Host ""

# ============================================================================
# 7. ?´æ–°?€??release ?®é??„å??•è…³??
# ============================================================================
Write-Host "æ­¥é? 7: ?´æ–° release ?®é??„å??•è…³??.." -ForegroundColor Cyan

$releaseDirs = @(
    "linux_release\v1.0.8sp1",
    "linux_release\v1.0.8sp999",
    "windows_release\v1.0.8sp1",
    "windows_release\v1.0.8sp999"
)

foreach ($releaseDir in $releaseDirs) {
    $fullPath = Join-Path $PSScriptRoot $releaseDir
    
    if (Test-Path $fullPath) {
        # Windows ?ˆæœ¬
        if ($releaseDir -like "windows_*") {
            $batPath = Join-Path $fullPath "start_nms.bat"
            $windowsStartScript | Out-File -FilePath $batPath -Encoding ASCII -Force
            
            $ps1Path = Join-Path $fullPath "start_nms.ps1"
            $psStartScript | Out-File -FilePath $ps1Path -Encoding UTF8 -Force
            
            Write-Host "  ??å·²æ›´?? $releaseDir (Windows)" -ForegroundColor Green
        }
        # Linux ?ˆæœ¬
        else {
            $shPath = Join-Path $fullPath "start_nms.sh"
            $linuxStartScript | Out-File -FilePath $shPath -Encoding UTF8 -Force
            
            Write-Host "  ??å·²æ›´?? $releaseDir (Linux)" -ForegroundColor Green
        }
    }
    else {
        Write-Host "  ??è·³é?: $releaseDir (?®é?ä¸å???" -ForegroundColor Gray
    }
}

Write-Host ""

# ============================================================================
# 8. ?µå»ºä½¿ç”¨èªªæ??‡ä»¶
# ============================================================================
Write-Host "æ­¥é? 8: ?µå»ºä½¿ç”¨èªªæ??‡ä»¶..." -ForegroundColor Cyan

$readmeContent = @'
# Management Server ç¹é?ä¸­æ?ç·¨ç¢¼ä¿®å¾©?‡å?

## ?é??è¿°
?¶åŸ·è¡?Management Server ?‚ï?å¦‚æ??ºç¾äº‚ç¢¼ä¸¦å??´ç?å¼è‡ª?•å?æ­¢ï??™æ˜¯?±æ–¼ Windows ?§åˆ¶?°ç·¨ç¢¼è¨­å®šä?æ­?¢º? æ??„ã€?

## è§?±º?¹æ?

### ?¹æ? 1: ä½¿ç”¨ UTF-8 ?Ÿå??³æœ¬ï¼ˆæ¨?¦ï?

#### Windows ?¨æˆ¶ï¼?
```batch
# ä½¿ç”¨?¹æ¬¡æª”å???
start_nms_utf8.bat

# ?–ä½¿??PowerShell ?³æœ¬
powershell -ExecutionPolicy Bypass -File start_nms_utf8.ps1
```

#### Linux ?¨æˆ¶ï¼?
```bash
# çµ¦ä??·è?æ¬Šé?
chmod +x start_nms_utf8.sh

# ?Ÿå?ä¼ºæ???
./start_nms_utf8.sh
```

### ?¹æ? 2: ?‹å?è¨­å?ç·¨ç¢¼

#### Windows PowerShellï¼?
```powershell
# è¨­å?ç·¨ç¢¼
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001

# ?Ÿå?ä¼ºæ???
.\nms_server.exe
```

#### Windows CMDï¼?
```batch
# è¨­å?ä»?¢¼??
chcp 65001

# ?Ÿå?ä¼ºæ???
nms_server.exe
```

#### Linux/macOSï¼?
```bash
# è¨­å?èªè??°å?
export LANG=zh_TW.UTF-8
export LC_ALL=zh_TW.UTF-8

# ?Ÿå?ä¼ºæ???
./nms_server
```

### ?¹æ? 3: æ°¸ä?è¨­å?ï¼ˆWindowsï¼?

1. **è¨­å?ç³»çµ±èªè??°å?ï¼?*
   - ?‹å??Œæ§?¶å°?â??Œæ??˜å??€?Ÿã€â??Œåœ°?€??
   - é»æ??Œç³»çµ±ç®¡?†ã€æ?ç±?
   - é»æ??Œè??´ç³»çµ±åœ°?€è¨­å???
   - ?¾é¸?ŒBeta: ä½¿ç”¨ Unicode UTF-8 ?ä??¨ç?èªè??¯æ´??
   - ?æ–°?Ÿå??»è…¦

2. **è¨­å? PowerShell ?è¨­ç·¨ç¢¼ï¼?*
   - ç·¨è¼¯ PowerShell è¨­å?æª”ï?`notepad $PROFILE`
   - æ·»å?ä»¥ä??§å®¹ï¼?
     ```powershell
     [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
     $OutputEncoding = [System.Text.Encoding]::UTF8
     ```
   - ?²å?ä¸¦é??°å???PowerShell

## æ¸¬è©¦ç·¨ç¢¼è¨­å?

?·è?æ¸¬è©¦?³æœ¬ç¢ºè?ç·¨ç¢¼?¯å¦æ­?¢ºï¼?
```powershell
powershell -ExecutionPolicy Bypass -File test_encoding.ps1
```

å¦‚æ??¨èƒ½?‹åˆ°?€?‰ç?é«”ä¸­?‡å??Œç‰¹æ®Šç¬¦?Ÿï?è¡¨ç¤ºç·¨ç¢¼è¨­å??å?ï¼?

## å¸¸è??é?

### Q: ?ºä?éº¼æ??ºç¾äº‚ç¢¼ï¼?
A: Windows ?è¨­ä½¿ç”¨ Big5 ??CP950 ç·¨ç¢¼ï¼Œè€?Management Server ä½¿ç”¨ UTF-8 ç·¨ç¢¼?‚ç•¶?©è€…ä??¹é??‚å°±?ƒå‡º?¾ä?ç¢¼ã€?

### Q: äº‚ç¢¼?ƒå½±?¿ç?å¼å??½å?ï¼?
A: ?¯èƒ½?ƒã€‚æ?äº›æ?æ³ä?ï¼Œä?ç¢¼æ?å°è‡´ç¨‹å??°å¸¸çµ‚æ­¢?–å??½ç•°å¸¸ã€?

### Q: Linux ä¹Ÿæ??‰é€™å€‹å?é¡Œå?ï¼?
A: Linux ç³»çµ±?šå¸¸?è¨­ä½¿ç”¨ UTF-8ï¼Œè?å°‘å‡º?¾æ­¤?é??‚ä?å¦‚æ??‡åˆ°ï¼Œå¯ä»¥è¨­å®?LANG ??LC_ALL ?°å?è®Šæ•¸??

### Q: å¦‚ä?ç¢ºè??¶å?ç·¨ç¢¼ï¼?
A: ??PowerShell ä¸­åŸ·è¡Œï?
```powershell
chcp
[Console]::OutputEncoding.EncodingName
```
?‰è©²?‹åˆ° 65001 (UTF-8)

## ?€è¡“ç´°ç¯€

### ç·¨ç¢¼ä»?¢¼?å??§ï?
- **65001**: UTF-8ï¼ˆæ¨?¦ï?
- **950**: Big5ï¼ˆç?é«”ä¸­?‡å‚³çµ±ç·¨ç¢¼ï?
- **936**: GBKï¼ˆç°¡é«”ä¸­?‡ï?
- **437**: OEM United Statesï¼ˆè‹±?‡ï?

### ä¿®å¾©?³æœ¬?šä?ä»€éº¼ï?
1. è¨­å? PowerShell ?§åˆ¶?°è¼¸?ºç·¨ç¢¼ç‚º UTF-8
2. è¨­å??§åˆ¶?°ä»£ç¢¼é???65001 (UTF-8)
3. ?µå»º?…å« UTF-8 è¨­å??„å??•è…³??
4. ä¿®å¾©?€??PowerShell ?³æœ¬?„ç·¨ç¢?
5. ?ºæ???release ?ˆæœ¬?µå»º?Ÿå??³æœ¬

## ?¯æ´

å¦‚æ??é?ä»ç„¶å­˜åœ¨ï¼Œè?æª¢æŸ¥ï¼?
1. Windows ?ˆæœ¬?¯å¦?¯æ´ UTF-8
2. ?¯å¦?‰è¶³å¤ ç?ç³»çµ±æ¬Šé?
3. ?²æ?è»Ÿé??¯å¦?»æ?äº†ç·¨ç¢¼è¨­å®?

---
?€å¾Œæ›´?? 2026-01-31
?ˆæœ¬: v1.0.8
'@

$readmePath = Join-Path $PSScriptRoot "UTF8_ENCODING_FIX_README.md"
$readmeContent | Out-File -FilePath $readmePath -Encoding UTF8 -Force

Write-Host "??å·²å‰µå»? UTF8_ENCODING_FIX_README.md" -ForegroundColor Green

Write-Host ""

# ============================================================================
# å®Œæ?
# ============================================================================
Write-Host "================================================================" -ForegroundColor Green
Write-Host "         ä¿®å¾©å®Œæ?ï¼?                                            " -ForegroundColor Green
Write-Host "================================================================" -ForegroundColor Green
Write-Host ""

Write-Host "å·²å‰µå»ºç?æª”æ?:" -ForegroundColor Cyan
Write-Host "  ? start_nms_utf8.bat         - Windows §å¦¸ÀÉ±Ò°Ê¸}¥»" -ForegroundColor White
Write-Host "  ? start_nms_utf8.ps1         - PowerShell ±Ò°Ê¸}¥»" -ForegroundColor White
Write-Host "  ? start_nms_utf8.sh          - Linux ±Ò°Ê¸}¥»" -ForegroundColor White
Write-Host "  ? test_encoding.ps1          - ½s½X´ú¸Õ¸}¥»" -ForegroundColor White
Write-Host "  ? UTF8_ENCODING_FIX_README.md - ¨Ï¥Î»¡©ú¤å¥ó" -ForegroundColor White
Write-Host ""

Write-Host "ä¸‹ä?æ­?" -ForegroundColor Yellow
Write-Host "  1. æ¸¬è©¦ç·¨ç¢¼è¨­å?:" -ForegroundColor White
Write-Host "     powershell -ExecutionPolicy Bypass -File test_encoding.ps1" -ForegroundColor Gray
Write-Host ""
Write-Host "  2. ä½¿ç”¨?°ç??Ÿå??³æœ¬:" -ForegroundColor White
Write-Host "     start_nms_utf8.bat  (??start_nms_utf8.ps1)" -ForegroundColor Gray
Write-Host "Process complete." -ForegroundColor Green
Write-Host "All scripts have been updated to support UTF-8." -ForegroundColor Gray
