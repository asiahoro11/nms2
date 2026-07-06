//go:build windows

// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// createRestoreScript for Windows
func createRestoreScript(dbPath string) error {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("[Restore] Failed to get executable path: %v", err)
		return err
	}

	// Get current working directory (where the script should be created)
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("[Restore] Failed to get working directory: %v", err)
		cwd = filepath.Dir(exe) // Fallback to exe directory
	}

	if dbPath == "" {
		dbPath = "data/nms.db"
	}

	// Determine DB directory
	// In Batch, we can handle paths, but separators might be tricky if mixed.
	// Go uses /, Windows likes \.
	dbPath = filepath.FromSlash(dbPath)

	// Create a safe detached runner script
	// This helps in properly creating a detached "start" command that handles spaces in paths

	log.Printf("[Restore] Executable: %s", exe)
	log.Printf("[Restore] Working directory: %s", cwd)
	log.Printf("[Restore] DB Path: %s", dbPath)

	// Build script line by line with proper CRLF endings
	var sb strings.Builder
	lines := []string{
		"@echo off",
		"chcp 65001 >nul 2>&1",
		"setlocal EnableDelayedExpansion",
		"",
		"echo ========================================",
		"echo NMS System Restore Script",
		"echo ========================================",
		"echo.",
		"",
		"echo Waiting for server to shut down (5s)...",
		"timeout /t 5 /nobreak >nul",
		"",
		"set RETRY=0",
		":RETRY_LOOP",
		"echo.",
		"echo [Step 1] Cleaning up old status...",
		"",
		"REM Delete live WAL files if no pending restore for them (fresh restore)",
		"if not exist \"data\\nms.db-wal.pending\" (",
		"    if exist \"" + dbPath + "-wal\" del /f /q \"" + dbPath + "-wal\" 2>nul",
		"    if exist \"data\\nms.db-wal\" del /f /q \"data\\nms.db-wal\" 2>nul",
		")",
		"if not exist \"data\\nms.db-shm.pending\" (",
		"    if exist \"" + dbPath + "-shm\" del /f /q \"" + dbPath + "-shm\" 2>nul",
		"    if exist \"data\\nms.db-shm\" del /f /q \"data\\nms.db-shm\" 2>nul",
		")",
		"",
		"echo [Step 2] Restoring database...",
		"if exist \"data\\nms.db.pending\" (",
		"    if exist \"" + dbPath + "\" del /f /q \"" + dbPath + "\" >nul 2>&1",
		"    move /y \"data\\nms.db.pending\" \"" + dbPath + "\" >nul 2>&1",
		"    if errorlevel 1 goto LOCKED",
		")",
		"",
		"if exist \"data\\nms.db-wal.pending\" (",
		"    if exist \"" + dbPath + "-wal\" del /f /q \"" + dbPath + "-wal\" >nul 2>&1",
		"    move /y \"data\\nms.db-wal.pending\" \"" + dbPath + "-wal\" >nul 2>&1",
		")",
		"",
		"if exist \"data\\nms.db-shm.pending\" (",
		"    if exist \"" + dbPath + "-shm\" del /f /q \"" + dbPath + "-shm\" >nul 2>&1",
		"    move /y \"data\\nms.db-shm.pending\" \"" + dbPath + "-shm\" >nul 2>&1",
		")",
		"",
		"goto STEP3",
		"",
		":LOCKED",
		"    set /a RETRY+=1",
		"    echo Database locked, retry !RETRY! of 10...",
		"    if !RETRY! geq 10 (",
		"        echo ERROR: Could not restore database after 10 retries - File Locked",
		"        echo Please close any software accessing the DB.",
		"        pause",
		"        goto END",
		"    )",
		"    timeout /t 2 /nobreak >nul",
		"    goto RETRY_LOOP",
		"",
		":STEP3",
		"echo Database restored successfully.",
		"",
		"echo [Step 3] Restoring config...",
		"if exist \"config\\config.yaml.pending\" (",
		"    move /y \"config\\config.yaml.pending\" \"config.yaml\" >nul 2>&1",
		"    echo Config restored.",
		")",
		"",
		"echo [Step 4] Restoring uploads...",
		"if exist \"data\\uploads_pending\" (",
		"    if not exist \"data\\uploads\" mkdir \"data\\uploads\"",
		"    xcopy /s /e /y /q \"data\\uploads_pending\\*\" \"data\\uploads\\\" >nul 2>&1",
		"    rmdir /s /q \"data\\uploads_pending\" 2>nul",
		"    echo Uploads restored.",
		")",
		"",
		"echo.",
		"echo ========================================",
		"echo [Step 5] Starting NMS Server...",
		"echo ========================================",
		"echo.",
		"",
		"REM Start executable in a new independent window",
		"start \"NMS Server\" \"" + exe + "\"",
		"",
		"echo Server start command issued.",
		"timeout /t 3 /nobreak >nul",
		"",
		":END",
		"exit",
	}

	// Join with CRLF
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString("\r\n")
	}

	scriptPath := filepath.Join(cwd, "restore.bat")
	log.Printf("[Restore] Creating restore script at: %s", scriptPath)

	err = os.WriteFile(scriptPath, []byte(sb.String()), 0755)
	if err != nil {
		log.Printf("[Restore] Failed to write script: %v", err)
		return err
	}

	return nil
}

// executeRestoreScript for Windows
func executeRestoreScript() {
	cwd, _ := os.Getwd()
	scriptPath := filepath.Join(cwd, "restore.bat")

	log.Printf("[Restore] Executing restore script: %s", scriptPath)

	// Use 'cmd /c start ...' to launch the batch file in a separate detached console
	// that survives the parent process's exit. The window is left visible
	// (no /MIN) since a hidden window running a script that deletes files and
	// relaunches the executable is exactly the shape antivirus heuristics
	// flag as dropper/self-updater behavior; showing it is also more
	// transparent for an admin-initiated restore.
	cmd := exec.Command("cmd", "/C", "start", "cmd", "/C", scriptPath)
	cmd.Dir = cwd

	err := cmd.Start()
	if err != nil {
		log.Printf("[Restore] Failed to start script: %v", err)
	} else {
		log.Printf("[Restore] Script started, PID: %d", cmd.Process.Pid)
		// Release process to ensure detachment
		cmd.Process.Release()
	}
}
