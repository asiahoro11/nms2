//go:build windows

// Made by YTSworks
// YTS工作室製作
package main

import (
	"log"
	"os"
	"path/filepath"
)

const launcherEnvVar = "NMS_LAUNCHED_VIA_SCRIPT"

// enforceLauncher used to refuse to start (native MessageBoxW popup, then
// exit) unless launched via start_nms.bat/.ps1. That shape — check a
// condition set only by our own script, otherwise refuse to run — matches
// the sandbox/analysis-evasion pattern (MITRE T1497) that antivirus heuristics
// are trained to flag, and is the likely cause of Windows builds being
// misdetected as trojans. Direct execution is now always allowed; we just
// log a tip pointing at the launcher scripts (UTF-8 console setup and
// duplicate-instance detection).
func enforceLauncher() {
	if os.Getenv(launcherEnvVar) == "1" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = "nms_server.exe"
	}
	baseDir := filepath.Dir(exePath)
	log.Printf("Tip: start via %s or %s for UTF-8 console setup and duplicate-instance detection.",
		filepath.Join(baseDir, "start_nms.bat"), filepath.Join(baseDir, "start_nms.ps1"))
}
