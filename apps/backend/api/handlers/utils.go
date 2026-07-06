// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"strings"
	"unicode"
)

// cleanString 移除非 ASCII 字元以避免 PDF 生成錯誤
func cleanString(s string) string {
	return strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return -1 // 移除
		}
		if !unicode.IsPrint(r) {
			return -1 // 移除不可列印字元
		}
		return r
	}, s)
}
