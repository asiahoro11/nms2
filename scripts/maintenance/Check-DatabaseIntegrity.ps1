# UTF-8 ç·¨ç¢¼è¨­å?
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8
$ConfirmPreference = "None"
# Management Server è³‡æ?åº«å??´æ€§æª¢?¥å·¥??
# ?¨é€? æª¢æŸ¥ nms_sync è³‡æ?åº«ç?çµæ??‡å??´æ€?

param(
    [string]$DatabasePath = ".\data\nms.db",
    [string]$OutputFile = "database_check_report.txt"
)

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Management Server è³‡æ?åº«å??´æ€§æª¢?¥å·¥?? -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# æª¢æŸ¥ SQLite ?¯å¦å·²å?è£?
$sqliteCmd = Get-Command sqlite3 -ErrorAction SilentlyContinue

if (-not $sqliteCmd) {
    Write-Host "???¯èª¤: ?¾ä???sqlite3 ?½ä»¤" -ForegroundColor Red
    Write-Host ""
    Write-Host "è«‹å?å®‰è? SQLite:" -ForegroundColor Yellow
    Write-Host "  ?¹æ? 1: ä½¿ç”¨ Chocolatey" -ForegroundColor Yellow
    Write-Host "    choco install sqlite" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  ?¹æ? 2: ?‹å?ä¸‹è?" -ForegroundColor Yellow
    Write-Host "    https://www.sqlite.org/download.html" -ForegroundColor Gray
    Write-Host ""
    exit 1
}

# æª¢æŸ¥è³‡æ?åº«æ?æ¡ˆæ˜¯?¦å???
if (-not (Test-Path $DatabasePath)) {
    Write-Host "???¯èª¤: ?¾ä??°è??™åº«æª”æ?: $DatabasePath" -ForegroundColor Red
    Write-Host ""
    Write-Host "?¯èƒ½?„å???" -ForegroundColor Yellow
    Write-Host "  1. è³‡æ?åº«å??ªå?å§‹å?ï¼ˆé?æ¬¡å??•æ??ƒè‡ª?•å»ºç«‹ï?" -ForegroundColor Gray
    Write-Host "  2. è³‡æ?åº«è·¯å¾‘ä?æ­?¢º" -ForegroundColor Gray
    Write-Host ""
    Write-Host "å»ºè­°?ä?:" -ForegroundColor Yellow
    Write-Host "  1. ?ˆå???Management Server ä¼ºæ??¨ä»¥?å??–è??™åº«" -ForegroundColor Gray
    Write-Host "  2. ç¢ºè?è³‡æ?åº«è·¯å¾‘è¨­å®šï??è¨­: ./data/nms.dbï¼? -ForegroundColor Gray
    Write-Host ""
    exit 1
}

Write-Host "???¾åˆ°è³‡æ?åº«æ?æ¡? $DatabasePath" -ForegroundColor Green

# ?–å?è³‡æ?åº«æ?æ¡ˆè?è¨?
$dbFile = Get-Item $DatabasePath
Write-Host "??è³‡æ?åº«å¤§å°? $([math]::Round($dbFile.Length / 1KB, 2)) KB" -ForegroundColor Green
Write-Host "???€å¾Œä¿®?¹æ??? $($dbFile.LastWriteTime)" -ForegroundColor Green
Write-Host ""

# ?·è?å®Œæ•´?§æª¢??
Write-Host "æ­?œ¨?·è?è³‡æ?åº«å??´æ€§æª¢??.." -ForegroundColor Cyan
Write-Host ""

# æª¢æŸ¥ SQL ?³æœ¬?¯å¦å­˜åœ¨
$sqlScript = ".\check_database_integrity.sql"
if (-not (Test-Path $sqlScript)) {
    Write-Host "???¯èª¤: ?¾ä??°æª¢?¥è…³?? $sqlScript" -ForegroundColor Red
    Write-Host ""
    Write-Host "è«‹ç¢ºèª?check_database_integrity.sql æª”æ?å­˜åœ¨?¼ç•¶?ç›®?? -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

# ?·è? SQL æª¢æŸ¥?³æœ¬
try {
    $timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $reportFile = "database_check_report_$timestamp.txt"
    
    Write-Host "?·è?æª¢æŸ¥ä¸­ï?è«‹ç???.." -ForegroundColor Yellow
    
    # ?·è? SQLite ?½ä»¤ä¸¦è¼¸?ºåˆ°æª”æ?
    sqlite3 $DatabasePath ".read $sqlScript" > $reportFile 2>&1
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "??æª¢æŸ¥å®Œæ?ï¼? -ForegroundColor Green
        Write-Host ""
        Write-Host "?±å?å·²å„²å­˜è‡³: $reportFile" -ForegroundColor Cyan
        Write-Host ""
        
        # é¡¯ç¤º?±å??˜è?
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "?±å??˜è?" -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan
        
        # è®€?–ä¸¦é¡¯ç¤º?¨å??±å??§å®¹
        $reportContent = Get-Content $reportFile -Raw
        
        # æª¢æŸ¥?¯å¦?‰éŒ¯èª?
        if ($reportContent -match "Error|?¯èª¤|Failed|å¤±æ?") {
            Write-Host "? ï?  ?¼ç¾æ½›åœ¨?é?ï¼Œè??¥ç?å®Œæ•´?±å?" -ForegroundColor Yellow
        } else {
            Write-Host "???ªç™¼?¾æ?é¡¯å?é¡? -ForegroundColor Green
        }
        
        Write-Host ""
        Write-Host "è«‹ä½¿?¨æ?å­—ç·¨è¼¯å™¨?‹å??±å?æª”æ??¥ç?è©³ç´°è³‡è?:" -ForegroundColor Cyan
        Write-Host "  notepad $reportFile" -ForegroundColor Gray
        Write-Host ""
        
        # è©¢å??¯å¦ç«‹å³?‹å??±å?
        $openReport = Read-Host "?¯å¦ç«‹å³?‹å??±å?ï¼?(Y/N)"
        if ($openReport -eq "Y" -or $openReport -eq "y") {
            notepad $reportFile
        }
        
    } else {
        Write-Host "??æª¢æŸ¥?ç?ä¸­ç™¼?ŸéŒ¯èª? -ForegroundColor Red
        Write-Host ""
        Write-Host "?¯èª¤ä»?¢¼: $LASTEXITCODE" -ForegroundColor Red
        exit 1
    }
    
} catch {
    Write-Host "???·è?æª¢æŸ¥?‚ç™¼?Ÿç•°å¸? $_" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "å»ºè­°?„å?çºŒæ?ä½? -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "1. ?¥ç?å®Œæ•´?±å?ä»¥ä?è§???™åº«?€?? -ForegroundColor Yellow
Write-Host "2. ?¹æ??±å?ä¸­ç?å»ºè­°?·è??ªå??ä?" -ForegroundColor Yellow
Write-Host "3. å¦‚ç™¼?¾è??™å??´æ€§å?é¡Œï?è«‹å?ä»½è??™åº«å¾Œé€²è?ä¿®å¾©" -ForegroundColor Yellow
Write-Host ""
Write-Host "å¸¸ç”¨ç¶­è­·?½ä»¤:" -ForegroundColor Cyan
Write-Host "  # ?ªå?è³‡æ?åº? -ForegroundColor Gray
Write-Host "  sqlite3 $DatabasePath 'ANALYZE; VACUUM;'" -ForegroundColor Gray
Write-Host ""
Write-Host "  # æ¸…ç? 30 å¤©å??„æ??½æ?æ¨? -ForegroundColor Gray
Write-Host "  sqlite3 $DatabasePath `"DELETE FROM device_metrics WHERE collected_at < datetime('now', '-30 days');`"" -ForegroundColor Gray
Write-Host ""
Write-Host "  # æ¸…ç? 90 å¤©å???Syslog" -ForegroundColor Gray
Write-Host "  sqlite3 $DatabasePath `"DELETE FROM syslogs WHERE received_at < datetime('now', '-90 days');`"" -ForegroundColor Gray
Write-Host ""

