//go:build windows
// +build windows

// Made by YTSworks
// YTS工作室製作
package main

import (
	"log"
	"syscall"
)

// Windows UTF-8 控制台支援
func init() {
	// 設定 Windows 控制台為 UTF-8 模式 (Code Page 65001)
	// 這可以防止繁體中文顯示為亂碼並導致程式停止
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleOutputCP := kernel32.NewProc("SetConsoleOutputCP")
	setConsoleCP := kernel32.NewProc("SetConsoleCP")

	// 設定輸出和輸入代碼頁為 UTF-8 (65001)
	setConsoleOutputCP.Call(uintptr(65001))
	setConsoleCP.Call(uintptr(65001))

	log.Println("已設定 Windows 控制台為 UTF-8 模式")
}
