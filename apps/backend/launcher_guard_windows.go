//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const launcherEnvVar = "NMS_LAUNCHED_VIA_SCRIPT"
const launcherDialogSkipEnvVar = "NMS_SKIP_LAUNCHER_MESSAGE_BOX"

func enforceLauncher() error {
	if os.Getenv(launcherEnvVar) == "1" {
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = "nms_server.exe"
	}

	baseDir := filepath.Dir(exePath)
	batPath := filepath.Join(baseDir, "start_nms.bat")
	ps1Path := filepath.Join(baseDir, "start_nms.ps1")

	message := fmt.Sprintf(
		"Please start Management System with one of the launcher scripts instead of running nms_server.exe directly.\n\nUse:\n%s\nor\n%s",
		batPath,
		ps1Path,
	)

	if os.Getenv(launcherDialogSkipEnvVar) != "1" {
		showLauncherMessage(message)
	}
	return errors.New(message)
}

func showLauncherMessage(message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("Management System Startup")
	messageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(text)),
		uintptr(unsafe.Pointer(title)),
		0x00000000|0x00000030,
	)
}
