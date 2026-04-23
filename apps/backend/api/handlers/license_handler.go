package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"management-server/services/license"

	"github.com/gin-gonic/gin"
)

// LicenseStatus response structure
type LicenseStatus struct {
	CurrentDevices int           `json:"current_devices"`
	MaxDevices     int           `json:"max_devices"`
	MaxCameras     int           `json:"max_cameras"`
	DefaultLimit   int           `json:"default_limit"`
	TrialAvailable bool          `json:"trial_available"`
	Licenses       []LicenseInfo `json:"licenses"`
}

// LicenseInfo represents a single license entry
type LicenseInfo struct {
	ID          int      `json:"id"`
	LicenseKey  string   `json:"license_key"`
	LicenseType string   `json:"license_type"`
	DeviceCount int      `json:"device_count"`
	CameraCount int      `json:"camera_count"`
	Features    []string `json:"features"`
	ValidFrom   string   `json:"valid_from"`
	ValidUntil  string   `json:"valid_until"`
	IsPermanent bool     `json:"is_permanent"`
	IsActive    bool     `json:"is_active"`
	Status      string   `json:"status"` // active, expired, trial
}

// ActivateLicenseInput for license activation request
type ActivateLicenseInput struct {
	LicenseKey string `json:"license_key" binding:"required"`
}

// GetLicenseStatus returns current license status
func (h *Handler) GetLicenseStatus(c *gin.Context) {
	h.ensureLicenseRuntimeFresh(0)
	status, err := h.license.Status()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    status,
	})
}

// GetEncryptedMachineID returns encrypted machine ID for customer
func (h *Handler) legacyGetEncryptedMachineID(c *gin.Context) {
	machineID := getSystemUUID()
	secretKey := h.getSecretKey()

	encryptedID, err := license.EncryptMachineID(machineID, secretKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法加密機器識別碼"})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"encrypted_id": encryptedID,
			"raw_id":       machineID, // For debugging, can be removed in production
		},
	})
}

// ActivateLicense activates a license key or trial
func (h *Handler) legacyActivateLicense(c *gin.Context) {
	var input ActivateLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "license", "activate_license_failed", "license activation rejected", map[string]interface{}{
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_license", "license", "failed", map[string]interface{}{
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請輸入授權碼"})
		return
	}

	key := strings.TrimSpace(input.LicenseKey)
	// Remove all whitespace/newlines that might come from copy-paste
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, " ", "")

	// Check if this is a trial activation
	if license.IsTrialKey(key) {
		h.activateTrial(c)
		return
	}

	machineID := getSystemUUID()
	payload, err := license.ValidateLicenseKey(key, machineID, h.getFormalSecretKey(), h.getPOCSecretKey())
	if err != nil {
		h.WriteSystemLog("warning", "license", "activate_license_failed", "license validation failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"machine_id":         machineID,
			"reason":             "validation_failed",
			"error":              err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_license", "license", "failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"machine_id":         machineID,
			"reason":             "validation_failed",
			"error":              err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	// Convert features to JSON
	featuresJSON, _ := json.Marshal(payload.Features)
	licenseType := "standard"
	if payload.LicenseMode == license.PoCLicenseMode {
		licenseType = "poc"
	}

	// Check if license already exists in DB
	var existingID int
	alreadyExists := h.db.QueryRow("SELECT id FROM licenses WHERE license_key = ?", key).Scan(&existingID) == nil

	var result sql.Result
	if alreadyExists {
		// Key already in DB (same machine, valid signature) ??re-activate it.
		// This handles the case where the database was wiped/replaced and the user
		// needs to re-enter their existing license key.
		result, err = h.db.Exec(`
			UPDATE licenses SET
				license_type=?, device_count=?, camera_count=?, enabled_features=?,
				valid_from=?, valid_until=?, is_active=1
			WHERE license_key=?
		`, licenseType, payload.DeviceCount, payload.CameraCount, string(featuresJSON), payload.IssuedAt, payload.ValidUntil, key)
		log.Printf("[License] Re-activating existing license key (DB was likely reset).")
	} else {
		// Insert new license
		result, err = h.db.Exec(`
			INSERT INTO licenses (license_key, license_type, device_count, camera_count, enabled_features, valid_from, valid_until, is_active)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		`, key, licenseType, payload.DeviceCount, payload.CameraCount, string(featuresJSON), payload.IssuedAt, payload.ValidUntil)
	}

	if err != nil {
		h.WriteSystemLog("error", "license", "activate_license_failed", "license activation failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"license_mode":       payload.LicenseMode,
			"license_type":       licenseType,
			"device_count":       payload.DeviceCount,
			"camera_count":       payload.CameraCount,
			"valid_until":        payload.ValidUntil,
			"is_permanent":       license.IsPermanentValidity(payload.IssuedAt, payload.ValidUntil),
			"reason":             "database_write_failed",
			"error":              err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_license", auditResource("license", key), "failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"license_mode":       payload.LicenseMode,
			"license_type":       licenseType,
			"device_count":       payload.DeviceCount,
			"camera_count":       payload.CameraCount,
			"features":           payload.Features,
			"valid_until":        payload.ValidUntil,
			"is_permanent":       license.IsPermanentValidity(payload.IssuedAt, payload.ValidUntil),
			"reason":             "database_write_failed",
			"error":              err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "授權啟用失敗: " + err.Error()})
		return
	}

	if payload.DeviceCount > 0 || hasFeature(payload.Features, "device_management") {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('device_management_enabled', '1', 'Device management module enabled')`)
	}

	// Initialize Camera Module if license contains camera feature, but don't auto-enable
	hasCameraFeature := false
	for _, feature := range payload.Features {
		if feature == "camera_viewer" {
			hasCameraFeature = true
			break
		}
	}
	if payload.CameraCount > 0 {
		hasCameraFeature = true
	}

	messages := []string{"授權啟用成功"}
	if hasCameraFeature {
		log.Printf("[License] Camera feature detected (CameraCount: %d). Initializing database tables (module remains disabled by default).", payload.CameraCount)

		// Initialize database tables and auto-enable the camera module
		h.initializeCameraModule()

		maxCameras := payload.CameraCount
		if maxCameras == 0 {
			maxCameras = 4 // Default free tier
		}

		tier := "free"
		if maxCameras >= 16 {
			tier = "enterprise"
		} else if maxCameras >= 9 {
			tier = "standard"
		}

		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_license', ?, 'Camera License Tier')`, tier)

		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_max_cameras', ?, 'Maximum cameras allowed')`, maxCameras)

		// Auto-enable the camera module upon license activation
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_enabled', '1', 'Camera viewer module enabled')`)

		log.Printf("[License] Camera module enabled (tier: %s, max: %d).", tier, maxCameras)
		messages = append(messages, "攝影機監控模組已啟用")
	}

	// Auto-enable Access Control module if license contains access_control feature
	hasACFeature := false
	for _, feature := range payload.Features {
		if feature == "access_control" {
			hasACFeature = true
			break
		}
	}
	if hasACFeature {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('access_control_enabled', '1', 'Access Control module enabled')`)
		log.Printf("[License] Access Control module enabled.")
		messages = append(messages, "門禁管理模組已啟用")
	}

	// Auto-enable PDU module if license contains pdu feature
	hasPDUFeature := false
	for _, feature := range payload.Features {
		if feature == "pdu" {
			hasPDUFeature = true
			break
		}
	}
	if hasPDUFeature {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('pdu_enabled', '1', 'PDU/UPS module enabled')`)
		log.Printf("[License] PDU/UPS module enabled.")
		messages = append(messages, "PDU/UPS 模組已解鎖")
	}

	// Auto-enable Camera Recording if license contains camera_recording feature
	hasRecordingFeature := false
	for _, feature := range payload.Features {
		if feature == "camera_recording" {
			hasRecordingFeature = true
			break
		}
	}
	if hasRecordingFeature {
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_recording_enabled', '1', 'Camera NVR recording enabled')`)
		log.Printf("[License] Camera Recording feature enabled.")
		messages = append(messages, "攝影機錄影功能已啟用")
	}

	id, _ := result.LastInsertId()
	finalMsg := messages[0]
	if len(messages) > 1 {
		finalMsg = strings.Join(messages, "，")
	}
	h.WriteSystemLog("notice", "license", "activate_license", "license activated", map[string]interface{}{
		"license_key_prefix": truncateAuditLicenseKey(key),
		"license_mode":       payload.LicenseMode,
		"license_type":       licenseType,
		"device_count":       payload.DeviceCount,
		"camera_count":       payload.CameraCount,
		"features":           payload.Features,
		"valid_until":        payload.ValidUntil,
		"is_permanent":       license.IsPermanentValidity(payload.IssuedAt, payload.ValidUntil),
		"reactivated":        alreadyExists,
	})
	h.WriteAuditFromContext(c, "activate_license", auditResource("license", key), "success", map[string]interface{}{
		"license_key_prefix": truncateAuditLicenseKey(key),
		"license_mode":       payload.LicenseMode,
		"license_type":       licenseType,
		"device_count":       payload.DeviceCount,
		"camera_count":       payload.CameraCount,
		"features":           payload.Features,
		"valid_until":        payload.ValidUntil,
		"is_permanent":       license.IsPermanentValidity(payload.IssuedAt, payload.ValidUntil),
		"reactivated":        alreadyExists,
	})
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: finalMsg,
		Data:    map[string]int64{"id": id},
	})
}

// activateTrial handles trial license activation
func (h *Handler) activateTrial(c *gin.Context) {
	// Check if trial already used
	var trialActivated string
	h.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'trial_activated'").Scan(&trialActivated)
	if trialActivated == "true" {
		h.WriteSystemLog("warning", "license", "activate_trial_license_failed", "trial activation rejected", map[string]interface{}{
			"reason": "trial_already_used",
		})
		h.WriteAuditFromContext(c, "activate_trial_license", "license:trial", "failed", map[string]interface{}{
			"reason": "trial_already_used",
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "試用版已使用，無法再次啟用"})
		return
	}

	// Generate trial license
	machineID := getSystemUUID()
	trialPayload := license.GenerateTrialLicense(machineID)
	featuresJSON, _ := json.Marshal(trialPayload.Features)

	// Insert trial license
	_, err := h.db.Exec(`
		INSERT INTO licenses (license_key, license_type, device_count, enabled_features, valid_from, valid_until, is_active)
		VALUES (?, ?, ?, ?, ?, ?, 1)
	`, "TRIAL-"+machineID[:8], "trial", trialPayload.DeviceCount, string(featuresJSON), trialPayload.IssuedAt, trialPayload.ValidUntil)

	if err != nil {
		h.WriteSystemLog("error", "license", "activate_trial_license_failed", "trial activation failed", map[string]interface{}{
			"reason": "database_write_failed",
			"error":  err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_trial_license", "license:trial", "failed", map[string]interface{}{
			"reason": "database_write_failed",
			"error":  err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "試用啟動失敗"})
		return
	}

	// Mark trial as used
	h.db.Exec("INSERT OR REPLACE INTO system_config (config_key, config_value) VALUES ('trial_activated', 'true')")
	h.WriteSystemLog("notice", "license", "activate_trial_license", "trial license activated", map[string]interface{}{
		"device_count": trialPayload.DeviceCount,
		"features":     trialPayload.Features,
		"valid_until":  trialPayload.ValidUntil,
	})
	h.WriteAuditFromContext(c, "activate_trial_license", "license:trial", "success", map[string]interface{}{
		"device_count": trialPayload.DeviceCount,
		"features":     trialPayload.Features,
		"valid_until":  trialPayload.ValidUntil,
	})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "14天試用期已啟動，額外增加10台設備管理額度",
	})
}

func truncateAuditLicenseKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if len(trimmed) <= 16 {
		return trimmed
	}
	return trimmed[:16] + "..."
}

// GetLicenseFeatures returns enabled features for current licenses
func (h *Handler) GetLicenseFeatures(c *gin.Context) {
	h.ensureLicenseRuntimeFresh(0)
	features := h.license.FeatureFlags()

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"email":             features["email"],
			"line":              features["line"],
			"telegram":          features["telegram"],
			"whatsapp":          features["whatsapp"],
			"discord":           features["discord"],
			"slack":             features["slack"],
			"device_management": features["device_management"],
			"camera_viewer":     features["camera_viewer"],
			"pdu":               features["pdu"],
			"access_control":    features["access_control"],
		},
	})
}

// CheckDeviceLimitExceeded checks if device limit is exceeded
func (h *Handler) CheckDeviceLimitExceeded() (bool, int, int) {
	var deviceCount int
	h.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&deviceCount)

	maxDevices := h.license.MaxDeviceLimit()
	return deviceCount >= maxDevices, deviceCount, maxDevices
}

// getSecretKey retrieves or generates the encryption secret key
func (h *Handler) getSecretKey() []byte {
	var secretKeyB64 string
	// Use v2 key to force regeneration and ensure base64 storage
	err := h.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'aes_secret_key_v2'").Scan(&secretKeyB64)

	if err != nil || secretKeyB64 == "" {
		// Generate new key based on machine ID
		machineID := getSystemUUID()
		// DeriveKey returns raw bytes
		secretKey := license.DeriveKey("NMS-LICENSE-" + machineID)

		// Encode to Base64 for safe storage in TEXT column
		secretKeyB64 = base64.StdEncoding.EncodeToString(secretKey)

		h.db.Exec("INSERT OR REPLACE INTO system_config (config_key, config_value) VALUES ('aes_secret_key_v2', ?)", secretKeyB64)
		return secretKey
	}

	// Decode from Base64
	secretKey, err := base64.StdEncoding.DecodeString(secretKeyB64)
	if err != nil {
		// If decode fails (shouldn't happen with v2), regenerate
		machineID := getSystemUUID()
		secretKey = license.DeriveKey("NMS-LICENSE-" + machineID)
		secretKeyB64 = base64.StdEncoding.EncodeToString(secretKey)
		h.db.Exec("INSERT OR REPLACE INTO system_config (config_key, config_value) VALUES ('aes_secret_key_v2', ?)", secretKeyB64)
		return secretKey
	}

	return secretKey
}

// Helper function to parse int from string
func parseIntFromString(s string) (int, error) {
	var result int
	_, err := strings.NewReader(s).Read(make([]byte, 0))
	if err != nil {
		return 0, err
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}

// GenerateLicenseKeyInput for license key generation request
type GenerateLicenseKeyInput struct {
	MachineID   string   `json:"machine_id"`
	LicenseMode string   `json:"license_mode"`
	LicenseType string   `json:"license_type"` // "device", "alert", "camera", "access_control", "combined", "full"
	DeviceCount int      `json:"device_count"` // Required for device/combined/full
	CameraCount int      `json:"camera_count"` // Required for camera/combined/full (4/9/16)
	Years       int      `json:"years"`        // 1-5 years
	ValidUntil  string   `json:"valid_until"`
	Features    []string `json:"features"`   // Alert features: line, telegram, whatsapp
	AlertOnly   bool     `json:"alert_only"` // One-time alert license (no device)
}

// GenerateLicenseKey generates a new license key for a customer (Admin only)
//
// License types (?��??�購 / ?��?�?:
//   - "device"         ??設�??��??��? (yearly)
//   - "alert"          ???�警?�知管�? (one-time, permanent)
//   - "camera"         ???�影機監?�模�?(yearly, camera_count = 4/9/16)
//   - "access_control" ???�禁管?�模�?(yearly)
//   - "combined"       ??設�? + ?�警 (yearly)
//   - "full"           ??設�? + ?�警 + ?�影�?(yearly)
func (h *Handler) legacyGenerateLicenseKey(c *gin.Context) {
	var input GenerateLicenseKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid license request"})
		return
	}

	licenseMode := strings.ToLower(strings.TrimSpace(input.LicenseMode))
	if licenseMode == "" {
		licenseMode = license.FormalLicenseMode
	}

	if input.LicenseType == "" {
		if input.AlertOnly || input.DeviceCount <= 0 && input.CameraCount <= 0 {
			input.LicenseType = "alert"
		} else if input.CameraCount > 0 && input.DeviceCount <= 0 {
			input.LicenseType = "camera"
		} else {
			input.LicenseType = "device"
		}
	}

	if licenseMode != license.PoCLicenseMode && strings.TrimSpace(input.MachineID) == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "formal license requires machine_id"})
		return
	}

	isAnnual := input.LicenseType == "device" || input.LicenseType == "combined" ||
		input.LicenseType == "camera" || input.LicenseType == "access_control" || input.LicenseType == "full"
	if isAnnual && licenseMode != license.PoCLicenseMode {
		if input.Years < 1 || (input.Years > 5 && !license.IsPermanentYears(input.Years)) {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "years must be between 1 and 5, or 50 and above for permanent license"})
			return
		}
	}
	if (input.LicenseType == "device" || input.LicenseType == "combined" || input.LicenseType == "full") && input.DeviceCount <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "device_count must be greater than 0"})
		return
	}
	if (input.LicenseType == "camera" || input.LicenseType == "full") && input.CameraCount <= 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "camera_count must be greater than 0"})
		return
	}

	secretKey := h.getFormalSecretKey()
	if licenseMode == license.PoCLicenseMode {
		secretKey = h.getPOCSecretKey()
	}

	validUntil := strings.TrimSpace(input.ValidUntil)
	if licenseMode == license.PoCLicenseMode {
		if validUntil == "" {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "poc license requires valid_until"})
			return
		}
		if _, err := license.ParseLicenseTime(validUntil); err != nil {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid valid_until format"})
			return
		}
	} else if isAnnual {
		if license.IsPermanentYears(input.Years) {
			validUntil = ""
		} else {
			validUntil = time.Now().AddDate(input.Years, 0, 0).Format("2006-01-02")
		}
	}

	features := input.Features
	cameraCount := 0

	switch input.LicenseType {
	case "device":
		features = []string{"device_management"}
	case "alert":
		if len(features) == 0 {
			features = []string{"line", "telegram", "whatsapp", "discord", "slack"}
		}
	case "camera":
		features = []string{"camera_viewer"}
		cameraCount = input.CameraCount
	case "access_control":
		features = []string{"access_control"}
	case "pdu":
		features = []string{"pdu"}
	case "combined":
		features = []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack"}
	case "full":
		features = []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack", "camera_viewer", "access_control", "pdu"}
		cameraCount = input.CameraCount
	}

	deviceCount := input.DeviceCount
	if input.LicenseType == "alert" || input.LicenseType == "camera" || input.LicenseType == "access_control" || input.LicenseType == "pdu" {
		deviceCount = 0
	}

	licenseKey, err := license.GenerateLicenseKeyAdvanced(
		licenseMode,
		input.MachineID,
		deviceCount,
		cameraCount,
		features,
		validUntil,
		secretKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to generate license: " + err.Error()})
		return
	}

	typeDisplayMap := map[string]string{
		"device":         "device management",
		"alert":          "alert channels",
		"camera":         "camera viewer",
		"access_control": "access control",
		"pdu":            "pdu",
		"combined":       "device + alerts",
		"full":           "full suite",
	}
	licenseTypeDisplay := typeDisplayMap[input.LicenseType]
	if licenseTypeDisplay == "" {
		licenseTypeDisplay = input.LicenseType
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "license generated",
		Data: map[string]interface{}{
			"license_key":  licenseKey,
			"license_mode": licenseMode,
			"license_type": input.LicenseType,
			"type_display": licenseTypeDisplay,
			"device_count": deviceCount,
			"camera_count": cameraCount,
			"years":        input.Years,
			"features":     features,
			"valid_until":  validUntil,
			"is_permanent": validUntil == "",
		},
	})
}

// getMaxDeviceLimit calculates total allowed devices
func (h *Handler) getMaxDeviceLimit() int {
	return h.license.MaxDeviceLimit()
}

// ResetLicenseIdentity resets the encryption key and clears licenses (for machine migration)
func (h *Handler) legacyResetLicenseIdentity(c *gin.Context) {
	// 1. Clear secret key
	if _, err := h.db.Exec("DELETE FROM system_config WHERE config_key = 'aes_secret_key_v2'"); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "?�置?�鑰失�?"})
		return
	}

	// 2. Clear trial status (optional, maybe we want to allow new trial on new machine?)
	// Let's keep trial status to prevent abuse, or clear it if we want a fresh start.
	// Since it's a "Reset", let's clear licenses but maybe keep trial history if possible?
	// Actually, if we reset identity, we should probably clear everything license related.
	h.db.Exec("DELETE FROM licenses")
	h.db.Exec("DELETE FROM system_config WHERE config_key = 'trial_activated'")

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "授權系統已重置。請重新啟用授權",
	})
}

// ReissueLicenseInput for license reissue request
type ReissueLicenseInput struct {
	OldMachineID  string `json:"old_machine_id" binding:"required"`
	OldLicenseKey string `json:"old_license_key" binding:"required"`
}

// ReissueLicense validates an old license and issues a new one for the current machine
func (h *Handler) legacyReissueLicense(c *gin.Context) {
	var input ReissueLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供機器碼及授權碼"})
		return
	}

	// 1. Validate Old License
	// Derive old secret key from old machine ID
	// Note: We assume the standard derivation formula was used
	oldSecretKey := license.DeriveKey("NMS-LICENSE-" + input.OldMachineID)

	oldPayload, err := license.ValidateLicenseKey(input.OldLicenseKey, input.OldMachineID, oldSecretKey, h.getPOCSecretKey())
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "授權驗證失敗: " + err.Error()})
		return
	}

	// Check if old license is expired
	if oldPayload.ValidUntil != "" {
		validUntil, err := time.Parse("2006-01-02", oldPayload.ValidUntil)
		if err == nil && time.Now().After(validUntil) {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "授權已過期，無法撤銷"})
			return
		}
	}

	// 2. Generate New License for Current Machine
	currentMachineID := getSystemUUID()
	currentSecretKey := h.getSecretKey()

	newLicenseKey, err := license.GenerateLicenseKey(
		currentMachineID,
		oldPayload.DeviceCount,
		oldPayload.Features,
		oldPayload.ValidUntil,
		currentSecretKey,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "授權解析失敗: " + err.Error()})
		return
	}

	// 3. Save New License to DB (Auto-Activate)
	// Check if this key already exists (unlikely)
	var existingID int
	if err := h.db.QueryRow("SELECT id FROM licenses WHERE license_key = ?", newLicenseKey).Scan(&existingID); err != nil {
		// New key, insert it
		featuresJSON, _ := json.Marshal(oldPayload.Features)
		_, err := h.db.Exec(`
			INSERT INTO licenses (license_key, license_type, device_count, enabled_features, valid_from, valid_until, is_active)
			VALUES (?, ?, ?, ?, ?, ?, 1)
		`, newLicenseKey, "reissue", oldPayload.DeviceCount, string(featuresJSON), oldPayload.IssuedAt, oldPayload.ValidUntil) // Keep original IssuedAt? Or Update? Let's keep original to track history.

		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "授權儲存失敗: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "授權已發送！已啟動此主機的使用授權",
		Data: map[string]interface{}{
			"new_license_key": newLicenseKey,
		},
	})
}

// autoEnableCameraViewer automatically enables Camera Viewer module if license has camera feature
func (h *Handler) autoEnableCameraViewer(payload *license.LicensePayload) {
	// Check if license has camera_viewer feature OR has CameraCount > 0
	hasCameraFeature := false
	for _, feature := range payload.Features {
		if feature == "camera_viewer" {
			hasCameraFeature = true
			break
		}
	}

	// ?��? CameraCount > 0 ?�是 Features 裡面??camera_viewer 就�??��?
	if payload.CameraCount > 0 {
		hasCameraFeature = true
	}

	if !hasCameraFeature {
		log.Printf("[License] Camera Viewer feature not found in payload (Features: %v, CameraCount: %d)", payload.Features, payload.CameraCount)
		return // No camera feature, skip
	}

	log.Printf("[License] Enabling Camera Viewer module (CameraCount: %d)", payload.CameraCount)

	// Enable Camera Viewer module
	h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
		VALUES ('camera_viewer_enabled', '1', 'Enable Camera Viewer Module')`)

	// Set camera license type based on camera count
	licenseType := "free"
	if payload.CameraCount >= 16 {
		licenseType = "enterprise"
	} else if payload.CameraCount >= 9 {
		licenseType = "standard"
	}

	h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
		VALUES ('camera_viewer_license', ?, 'Camera Viewer License: free/standard/enterprise')`, licenseType)

	// Set max cameras based on license
	maxCameras := payload.CameraCount
	if maxCameras == 0 {
		maxCameras = 4 // Default to free tier
	}

	h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
		VALUES ('camera_viewer_max_cameras', ?, 'Maximum cameras (free:4, standard:9, enterprise:16)')`, maxCameras)

	// Initialize camera module database if not already done
	h.initializeCameraModule()
}

// initializeCameraModule runs the camera module SQL migration if needed
func (h *Handler) initializeCameraModule() {
	// Check if cameras table exists
	var tableExists int
	err := h.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cameras'").Scan(&tableExists)
	if err != nil || tableExists > 0 {
		return // Table already exists or error checking
	}

	// Run camera module migration (matches db.go schema)
	migrationSQL := `
CREATE TABLE IF NOT EXISTS cameras (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    location TEXT,
    ip_address TEXT,
    port INTEGER DEFAULT 554,
    username TEXT,
    password_encrypted TEXT,
    rtsp_url TEXT,
    onvif_url TEXT,
    manufacturer TEXT,
    model TEXT,
    firmware TEXT,
    supports_ptz BOOLEAN DEFAULT 0,
    is_enabled BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'unknown',
    last_seen DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

	_, err = h.db.Exec(migrationSQL)
	if err != nil {
		// Log error but don't fail the license activation
		return
	}
}
