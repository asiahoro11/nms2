package main

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"management-server/services/license"
)

var featureProfiles = map[string][]string{
	"1": {"device_management", "camera_viewer", "camera_recording", "access_control", "pdu", "iot", "line", "telegram", "whatsapp", "discord", "slack"},
	"2": {"device_management"},
	"3": {"camera_viewer", "camera_recording"},
	"4": {"iot"},
	"5": {"pdu"},
	"6": {"access_control"},
	"7": {"line", "telegram", "whatsapp", "discord", "slack"},
}

var supportedFeatures = map[string]bool{
	"device_management": true, "camera_viewer": true, "camera_recording": true,
	"access_control": true, "pdu": true, "iot": true, "line": true,
	"telegram": true, "whatsapp": true, "discord": true, "slack": true,
}

func runWizard(reader *bufio.Reader, privateKey []byte, requestedOutput string) {
	fmt.Println("Management System License 產生器（Ed25519）")
	fmt.Println("請在離線授權工作站使用；私鑰不會寫入 License 檔。")
	fmt.Println()

	modeChoice := promptChoice(reader, "授權模式 [1] 正式授權  [2] PoC：", map[string]bool{"1": true, "2": true})
	mode := license.FormalLicenseMode
	if modeChoice == "2" {
		mode = license.PoCLicenseMode
	}

	machineID := ""
	if mode == license.FormalLicenseMode {
		machineID = strings.ToLower(promptRequired(reader, "Machine ID："))
	}

	fmt.Println("授權套件 [1] 完整  [2] 設備  [3] 攝影機  [4] IoT  [5] PDU/UPS  [6] 門禁  [7] 通知  [8] 自訂")
	profileChoice := promptChoice(reader, "請選擇：", map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": true, "6": true, "7": true, "8": true})
	features := append([]string(nil), featureProfiles[profileChoice]...)
	if profileChoice == "8" {
		fmt.Println("可用功能：device_management,camera_viewer,camera_recording,access_control,pdu,iot,line,telegram,whatsapp,discord,slack")
		features = parseFeatures(promptRequired(reader, "輸入功能名稱，以逗號分隔："))
		if len(features) == 0 {
			fail("至少要選擇一項有效功能")
		}
	}

	deviceCount := 0
	if contains(features, "device_management") {
		deviceCount = promptInt(reader, "設備授權數量：", 1, 1000000)
	}
	cameraCount := 0
	if contains(features, "camera_viewer") || contains(features, "camera_recording") {
		cameraCount = promptInt(reader, "攝影機授權數量：", 1, 1000000)
	}

	now := time.Now().UTC()
	payload := license.SignedLicense{
		LicenseMode: mode, MachineID: machineID, DeviceCount: deviceCount,
		CameraCount: cameraCount, Features: features, IssuedAt: now.Format(time.RFC3339),
	}
	if mode == license.PoCLicenseMode {
		payload.DurationDays = promptInt(reader, "PoC 天數：", 1, license.MaxPoCDurationDays)
	} else {
		years := promptInt(reader, "授權年限（輸入 0 代表永久）：", 0, 49)
		if years > 0 {
			payload.ValidUntil = now.AddDate(years, 0, 0).Format(time.RFC3339)
		}
	}

	printSummary(payload)
	if promptChoice(reader, "確認簽發？[Y] 是  [N] 否：", map[string]bool{"y": true, "n": true}) != "y" {
		fmt.Println("已取消，未產生 License。")
		return
	}

	key, err := issue(payload, privateKey)
	if err != nil {
		fail("簽發失敗：%v", err)
	}
	output := strings.TrimSpace(requestedOutput)
	if output == "" {
		output = defaultOutputPath(payload)
	}
	if err := writeLicense(output, key); err != nil {
		fail("寫入 License 失敗：%v", err)
	}
	fmt.Printf("\n完成。License 檔案：%s\n", mustAbs(output))
	fmt.Println("請只把 License 檔交付給客戶，不要交付 issuer_private.key。")
}

func loadPrivateKey(explicit string) ([]byte, error) {
	encoded := strings.TrimSpace(os.Getenv("NMS_LICENSE_PRIVATE_KEY_B64"))
	if encoded == "" {
		path := strings.TrimSpace(explicit)
		if path == "" {
			path = defaultPrivateKeyPath()
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("找不到 %s；請放置私鑰檔或設定 NMS_LICENSE_PRIVATE_KEY_B64", path)
		}
		encoded = strings.TrimSpace(string(data))
		for _, line := range strings.Split(encoded, "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if strings.HasPrefix(line, "NMS_LICENSE_PRIVATE_KEY_B64=") {
				encoded = strings.TrimSpace(strings.TrimPrefix(line, "NMS_LICENSE_PRIVATE_KEY_B64="))
				break
			}
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != ed25519.PrivateKeySize {
		return nil, errors.New("私鑰格式不正確")
	}
	return decoded, nil
}

func defaultPrivateKeyPath() string {
	if executable, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(executable), "issuer_private.key")
	}
	return "issuer_private.key"
}

func readPayload(path string) (license.SignedLicense, error) {
	var payload license.SignedLicense
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func issue(payload license.SignedLicense, privateKey []byte) (string, error) {
	payload.MachineID = strings.ToLower(strings.TrimSpace(payload.MachineID))
	for _, raw := range payload.Features {
		if !supportedFeatures[strings.ToLower(strings.TrimSpace(raw))] {
			return "", fmt.Errorf("不支援的功能：%s", raw)
		}
	}
	payload.Features = normalizeFeatures(payload.Features)
	if payload.LicenseMode != license.FormalLicenseMode && payload.LicenseMode != license.PoCLicenseMode {
		return "", errors.New("license_mode 必須是 formal 或 poc")
	}
	if payload.LicenseMode == license.FormalLicenseMode && payload.MachineID == "" {
		return "", errors.New("正式授權必須提供 Machine ID")
	}
	if payload.DeviceCount < 0 || payload.CameraCount < 0 {
		return "", errors.New("授權數量不可小於 0")
	}
	if len(payload.Features) == 0 {
		return "", errors.New("至少需要一項有效功能")
	}
	if contains(payload.Features, "device_management") && payload.DeviceCount < 1 {
		return "", errors.New("設備管理授權數量必須大於 0")
	}
	if (contains(payload.Features, "camera_viewer") || contains(payload.Features, "camera_recording")) && payload.CameraCount < 1 {
		return "", errors.New("攝影機授權數量必須大於 0")
	}
	if payload.IssuedAt == "" {
		payload.IssuedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, payload.IssuedAt); err != nil {
		return "", errors.New("issued_at 必須是 RFC3339 格式")
	}
	if payload.LicenseMode == license.PoCLicenseMode {
		if err := license.ValidatePoCDurationDays(payload.DurationDays); err != nil {
			return "", err
		}
		payload.ValidUntil = ""
	} else {
		payload.DurationDays = 0
		if payload.ValidUntil != "" {
			expiry, err := time.Parse(time.RFC3339, payload.ValidUntil)
			if err != nil {
				return "", errors.New("valid_until 必須是 RFC3339 格式")
			}
			issued, _ := time.Parse(time.RFC3339, payload.IssuedAt)
			if !expiry.After(issued) {
				return "", errors.New("valid_until 必須晚於 issued_at")
			}
		}
	}
	return license.SignEd25519License(payload, privateKey)
}

func normalizeFeatures(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if supportedFeatures[value] && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func parseFeatures(raw string) []string { return normalizeFeatures(strings.Split(raw, ",")) }

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func promptRequired(reader *bufio.Reader, label string) string {
	for {
		fmt.Print(label)
		value, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(value) == "" {
			fail("無法讀取輸入：%v", err)
		}
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
		fmt.Println("此欄位不可空白。")
	}
}

func promptChoice(reader *bufio.Reader, label string, allowed map[string]bool) string {
	for {
		value := strings.ToLower(promptRequired(reader, label))
		if allowed[value] {
			return value
		}
		fmt.Println("請輸入畫面列出的選項。")
	}
}

func promptInt(reader *bufio.Reader, label string, minValue, maxValue int) int {
	for {
		value, err := strconv.Atoi(promptRequired(reader, label))
		if err == nil && value >= minValue && value <= maxValue {
			return value
		}
		fmt.Printf("請輸入 %d 到 %d 的整數。\n", minValue, maxValue)
	}
}

func printSummary(payload license.SignedLicense) {
	fmt.Println("\n--- 簽發確認 ---")
	fmt.Printf("模式：%s\n", payload.LicenseMode)
	if payload.MachineID != "" {
		fmt.Printf("Machine ID：%s\n", payload.MachineID)
	}
	fmt.Printf("功能：%s\n", strings.Join(payload.Features, ", "))
	fmt.Printf("設備：%d，攝影機：%d\n", payload.DeviceCount, payload.CameraCount)
	if payload.LicenseMode == license.PoCLicenseMode {
		fmt.Printf("期限：首次啟用後 %d 天\n", payload.DurationDays)
	} else if payload.ValidUntil == "" {
		fmt.Println("期限：永久")
	} else {
		fmt.Printf("到期：%s\n", payload.ValidUntil)
	}
}

func defaultOutputPath(payload license.SignedLicense) string {
	identity := payload.LicenseMode
	if payload.MachineID != "" {
		identity = payload.MachineID
	}
	return fmt.Sprintf("license_%s_%s.txt", safeFilename(identity), time.Now().Format("20060102_150405"))
}

func safeFilename(value string) string {
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "output"
	}
	return b.String()
}

func writeLicense(path, key string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return errors.New("輸出路徑不可空白")
	}
	return os.WriteFile(cleanPath, []byte(strings.TrimSpace(key)+"\n"), 0600)
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
