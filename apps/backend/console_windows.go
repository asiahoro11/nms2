//go:build windows

package main

import "syscall"

func setupConsole() {
	// Attempt to set console output to UTF-8
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	// 65001 is UTF-8
	setConsoleOutputCP.Call(65001)
}
