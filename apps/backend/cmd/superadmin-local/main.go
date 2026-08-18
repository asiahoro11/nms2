package main

import (
	"crypto/subtle"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"management-server/config"
	"management-server/database"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	dbPath := flag.String("db", "", "optional NMS database path; auto-detected when omitted")
	mode := flag.String("mode", "", "optional init or reset; auto-detected when omitted")
	flag.Parse()
	cleanPath, err := resolveDatabasePath(*dbPath)
	if err != nil {
		fatal("invalid database path: %v", err)
	}
	requestedMode := strings.ToLower(strings.TrimSpace(*mode))
	if requestedMode != "" && requestedMode != "init" && requestedMode != "reset" {
		fatal("mode must be init or reset")
	}
	if _, err := os.Stat(cleanPath); err != nil {
		fatal("找不到 NMS 資料庫：%s\n請先啟動 NMS 完成建置，再停止 NMS 後重新執行此工具。", cleanPath)
	}
	fmt.Printf("NMS SuperAdmin 本機維護工具\n資料庫：%s\n\n", cleanPath)
	db, err := database.Initialize(cleanPath, config.Version)
	if err != nil {
		fatal("open database (stop NMS first): %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatal("database unavailable: %v", err)
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE lower(username) = 'superadmin'`).Scan(&exists); err != nil {
		fatal("query SuperAdmin: %v", err)
	}
	selectedMode := requestedMode
	if selectedMode == "" {
		if exists == 0 {
			selectedMode = "init"
			fmt.Println("狀態：SuperAdmin 尚未初始化，將建立帳號。")
		} else {
			selectedMode = "reset"
			fmt.Println("狀態：SuperAdmin 已存在，將變更密碼。")
		}
	}
	if selectedMode == "init" && exists != 0 {
		fatal("SuperAdmin is already initialized; use -mode reset")
	}
	if selectedMode == "reset" && exists != 1 {
		fatal("SuperAdmin is not initialized; use -mode init")
	}

	password := readPassword("請輸入新的 SuperAdmin 密碼：")
	confirm := readPassword("請再次輸入密碼：")
	if subtle.ConstantTimeCompare([]byte(password), []byte(confirm)) != 1 {
		fatal("password confirmation does not match")
	}
	if err := validatePassword(password); err != nil {
		fatal("password rejected: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		fatal("hash password: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		fatal("begin transaction: %v", err)
	}
	defer tx.Rollback()
	if selectedMode == "init" {
		_, err = tx.Exec(`INSERT INTO users (username, password_hash, role, is_active, force_change_password) VALUES ('SuperAdmin', ?, 'super_admin', 1, 0)`, string(hash))
	} else {
		_, err = tx.Exec(`UPDATE users SET password_hash = ?, role = 'super_admin', is_active = 1, force_change_password = 0, last_password_change = CURRENT_TIMESTAMP WHERE lower(username) = 'superadmin'`, string(hash))
	}
	if err != nil {
		fatal("write SuperAdmin: %v", err)
	}
	if _, err = tx.Exec(`UPDATE superadmin_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE revoked_at IS NULL`); err != nil {
		fatal("revoke sessions: %v", err)
	}
	if _, err = tx.Exec(`DELETE FROM superadmin_auth_challenges`); err != nil {
		fatal("clear challenges: %v", err)
	}
	if _, err = tx.Exec(`INSERT INTO audit_logs (username, source_ip, module, action, resource, resource_type, status, detail) VALUES ('local-maintenance', '127.0.0.1', 'security', ?, 'SuperAdmin', 'user', 'success', 'SuperAdmin credential changed by local maintenance tool')`, "superadmin_"+selectedMode); err != nil {
		fatal("write audit log: %v", err)
	}
	if err := tx.Commit(); err != nil {
		fatal("commit: %v", err)
	}
	fmt.Println("\n完成：SuperAdmin 密碼已安全更新。")
	fmt.Println("既有 TOTP 設定已保留，所有舊的 SuperAdmin 工作階段已撤銷。")
}

func resolveDatabasePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return filepath.Abs(filepath.Clean(explicit))
	}
	candidates := make([]string, 0, 3)
	if executable, err := os.Executable(); err == nil {
		base := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(base, "..", "data", "nms.db"),
			filepath.Join(base, "data", "nms.db"),
		)
	}
	candidates = append(candidates, filepath.Join("data", "nms.db"))
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(filepath.Clean(candidate))
		if err == nil {
			if info, statErr := os.Stat(absolute); statErr == nil && !info.IsDir() {
				return absolute, nil
			}
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no database candidate")
	}
	return filepath.Abs(filepath.Clean(candidates[0]))
}

func readPassword(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fatal("read password: %v", err)
	}
	return string(value)
}

func validatePassword(password string) error {
	if len([]rune(password)) < 12 {
		return fmt.Errorf("must contain at least 12 characters")
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
		digit = digit || unicode.IsDigit(r)
		symbol = symbol || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r))
	}
	if !upper || !lower || !digit || !symbol {
		return fmt.Errorf("must contain upper, lower, number, and symbol")
	}
	if strings.Contains(strings.ToLower(password), "superadmin") {
		return fmt.Errorf("must not contain the account name")
	}
	return nil
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
