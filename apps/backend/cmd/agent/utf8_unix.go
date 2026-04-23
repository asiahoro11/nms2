//go:build !windows
// +build !windows

package main

// Linux/Unix 系統不需要特殊的 UTF-8 初始化
// 這個檔案只是為了滿足編譯需求
func init() {
	// Linux 系統通常預設使用 UTF-8，不需要額外設定
}
