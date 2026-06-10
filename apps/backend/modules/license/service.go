package license

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"management-server/config"
	licensesvc "management-server/services/license"
)

const ActiveLicenseWindowSQL = "(valid_until IS NULL OR valid_until = '' OR CASE WHEN length(valid_until) <= 10 THEN datetime(valid_until || ' 23:59:59') ELSE datetime(valid_until) END >= datetime('now'))"

type Service struct {
	db     *sql.DB
	config *config.Config
}

func NewService(db *sql.DB, cfg *config.Config) *Service {
	return &Service{db: db, config: cfg}
}

func baseVersionLabel(version string) string {
	trimmed := strings.TrimSpace(version)
	if strings.HasSuffix(strings.ToLower(trimmed), "-poc") {
		return strings.TrimSpace(trimmed[:len(trimmed)-len("-poc")])
	}
	return trimmed
}

func pocVersionLabel(version string) string {
	base := baseVersionLabel(version)
	if base == "" {
		return strings.TrimSpace(version)
	}
	return base + "-PoC"
}

func (s *Service) HasActiveStandardLicense() bool {
	var count int
	query := `
		SELECT COUNT(*) FROM licenses
		WHERE is_active = 1
		  AND ` + ActiveLicenseWindowSQL + `
		  AND CASE
		        WHEN TRIM(COALESCE(license_type, '')) = '' THEN 'standard'
		        ELSE LOWER(TRIM(license_type))
		      END NOT IN ('poc', 'trial')
	`
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (s *Service) HasActivePoCLicense() bool {
	var count int
	query := `
		SELECT COUNT(*) FROM licenses
		WHERE is_active = 1
		  AND ` + ActiveLicenseWindowSQL + `
		  AND LOWER(TRIM(COALESCE(license_type, ''))) = 'poc'
	`
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return false
	}
	return count > 0
}

func (s *Service) HasActiveLicense() bool {
	return licensesvc.HasActiveLicense(s.db)
}

func (s *Service) distributionIsPoC() bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(s.config.System.Version)), "poc") {
		return true
	}

	var edition string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'nms_edition'").Scan(&edition); err == nil {
		return strings.Contains(strings.ToLower(strings.TrimSpace(edition)), "poc")
	}

	return false
}

func (s *Service) ShouldLockSession() bool {
	return licensesvc.ShouldRuntimeLockdown(s.db, s.config.System.Version)
}

func (s *Service) LockReason() string {
	return licensesvc.RuntimeLockReason(s.db, s.config.System.Version)
}

func (s *Service) IsPoCEdition() bool {
	if s.HasActiveStandardLicense() {
		return false
	}
	if s.HasActivePoCLicense() {
		return true
	}

	return s.distributionIsPoC()
}

func (s *Service) DisplayVersion() string {
	version := strings.TrimSpace(s.config.System.Version)
	base := baseVersionLabel(version)
	if base == "" {
		return version
	}
	if s.HasActiveStandardLicense() {
		return base
	}
	if s.HasActivePoCLicense() {
		return pocVersionLabel(version)
	}
	if s.distributionIsPoC() {
		return pocVersionLabel(version)
	}
	return base
}

func (s *Service) DefaultDeviceLimit() int {
	var configValue string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'default_device_limit'").Scan(&configValue); err != nil {
		return 0
	}
	return parseConfigInt(configValue)
}

func (s *Service) ActiveLicensedDeviceCount() int {
	var licensedDevices int
	query := "SELECT COALESCE(SUM(device_count), 0) FROM licenses WHERE is_active = 1 AND " + ActiveLicenseWindowSQL
	_ = s.db.QueryRow(query).Scan(&licensedDevices)
	return licensedDevices
}

func (s *Service) LicenseFeatureEnabled(feature string) bool {
	rows, err := s.db.Query("SELECT enabled_features FROM licenses WHERE is_active = 1 AND " + ActiveLicenseWindowSQL)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var featuresJSON sql.NullString
		if err := rows.Scan(&featuresJSON); err != nil || !featuresJSON.Valid {
			continue
		}
		if strings.Contains(featuresJSON.String, feature) {
			return true
		}
	}

	return false
}

func (s *Service) DeviceManagementEnabled() bool {
	if !s.IsPoCEdition() {
		return true
	}
	if s.ActiveLicensedDeviceCount() > 0 {
		return true
	}
	if s.LicenseFeatureEnabled("device_management") {
		return true
	}

	var enabled string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'device_management_enabled'").Scan(&enabled); err == nil {
		return strings.TrimSpace(enabled) == "1"
	}

	return false
}

func (s *Service) MaxDeviceLimit() int {
	return s.DefaultDeviceLimit() + s.ActiveLicensedDeviceCount()
}

func (s *Service) FeatureFlags() map[string]bool {
	features := map[string]bool{
		"email": true,
	}

	rows, err := s.db.Query(`
		SELECT enabled_features FROM licenses
		WHERE is_active = 1 AND ` + ActiveLicenseWindowSQL + `
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var featuresJSON sql.NullString
			if err := rows.Scan(&featuresJSON); err == nil && featuresJSON.Valid {
				var licenseFeatures []string
				if json.Unmarshal([]byte(featuresJSON.String), &licenseFeatures) == nil {
					for _, f := range licenseFeatures {
						features[f] = true
					}
				}
			}
		}
	}

	features["device_management"] = s.DeviceManagementEnabled()
	return features
}

func (s *Service) Status() (Status, error) {
	var currentDevices int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&currentDevices)

	defaultLimit := s.DefaultDeviceLimit()
	trialAvailable := true

	var configValue string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'trial_activated'").Scan(&configValue); err == nil {
		trialAvailable = configValue != "true"
	}

	rows, err := s.db.Query(`
		SELECT id, license_key, license_type, device_count, COALESCE(camera_count, 0), enabled_features, valid_from, valid_until, is_active
		FROM licenses
		ORDER BY id DESC
	`)
	if err != nil {
		return Status{}, err
	}
	defer rows.Close()

	var licenses []Info
	totalLicensedDevices := 0
	now := time.Now()

	for rows.Next() {
		var li Info
		var featuresJSON sql.NullString
		var validFrom, validUntil sql.NullString
		var deviceCount, cameraCount int

		if err := rows.Scan(&li.ID, &li.LicenseKey, &li.LicenseType, &deviceCount, &cameraCount, &featuresJSON, &validFrom, &validUntil, &li.IsActive); err != nil {
			continue
		}

		li.DeviceCount = deviceCount
		li.CameraCount = cameraCount
		li.ValidFrom = validFrom.String
		li.ValidUntil = validUntil.String
		li.IsPermanent = licensesvc.IsPermanentValidity(li.ValidFrom, li.ValidUntil)

		if featuresJSON.Valid {
			_ = json.Unmarshal([]byte(featuresJSON.String), &li.Features)
		}

		if li.IsPermanent {
			li.Status = "active"
		} else if li.ValidUntil != "" {
			expiry, err := licensesvc.ParseLicenseTime(li.ValidUntil)
			if err != nil || now.After(expiry) {
				li.Status = "expired"
				li.IsActive = false
			} else if li.LicenseType == "trial" {
				li.Status = "trial"
			} else {
				li.Status = "active"
			}
		} else {
			li.Status = "active"
		}

		if li.IsActive && li.Status != "expired" {
			totalLicensedDevices += li.DeviceCount
		}

		if len(li.LicenseKey) > 20 {
			li.LicenseKey = li.LicenseKey[:20] + "..."
		}

		licenses = append(licenses, li)
	}

	var totalLicensedCameras int
	_ = s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_max_cameras'").Scan(&configValue)
	if configValue != "" {
		totalLicensedCameras = parseConfigInt(configValue)
	}
	if totalLicensedCameras == 0 {
		totalLicensedCameras = 4
	}

	return Status{
		CurrentDevices: currentDevices,
		MaxDevices:     defaultLimit + totalLicensedDevices,
		MaxCameras:     totalLicensedCameras,
		DefaultLimit:   defaultLimit,
		TrialAvailable: trialAvailable,
		Licenses:       licenses,
	}, nil
}

func (s *Service) EncryptedMachineID(machineID string, secretKey []byte) (EncryptedMachineIDResult, error) {
	encryptedID, err := licensesvc.EncryptMachineID(machineID, secretKey)
	if err != nil {
		return EncryptedMachineIDResult{}, err
	}
	return EncryptedMachineIDResult{
		EncryptedID: encryptedID,
		RawID:       machineID,
	}, nil
}

func (s *Service) ActivateLicense(rawKey, machineID string, formalSecret, pocSecret []byte, ensureCameraSchema func()) (ActivateResult, error) {
	key := strings.TrimSpace(rawKey)
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, " ", "")

	payload, err := licensesvc.ValidateLicenseKey(key, machineID, formalSecret, pocSecret)
	if err != nil {
		return ActivateResult{}, err
	}

	featuresJSON, _ := json.Marshal(payload.Features)
	licenseType := "standard"
	if payload.LicenseMode == licensesvc.PoCLicenseMode {
		licenseType = "poc"
	}

	var result sql.Result
	var existingID int
	var existingValidFrom sql.NullString
	var existingValidUntil sql.NullString
	alreadyExists := s.db.QueryRow(
		"SELECT id, COALESCE(valid_from, ''), COALESCE(valid_until, '') FROM licenses WHERE license_key = ?",
		key,
	).Scan(&existingID, &existingValidFrom, &existingValidUntil) == nil

	now := time.Now()
	validFrom := payload.IssuedAt
	validUntil := payload.ValidUntil
	if licensesvc.IsDeferredDurationPoC(payload) {
		storedValidFrom := strings.TrimSpace(existingValidFrom.String)
		storedValidUntil := strings.TrimSpace(existingValidUntil.String)
		switch {
		case storedValidUntil != "":
			expiry, err := licensesvc.ParseLicenseTime(storedValidUntil)
			if err != nil {
				return ActivateResult{}, errors.New("invalid stored license expiry time")
			}
			if now.After(expiry) {
				return ActivateResult{}, errors.New("license has expired")
			}
			validFrom = storedValidFrom
			validUntil = storedValidUntil
		case storedValidFrom != "":
			start := now
			if parsedStart, err := licensesvc.ParseLicenseTime(storedValidFrom); err == nil {
				start = parsedStart
			}
			validFrom, validUntil, err = licensesvc.ResolvePoCActivationWindow(start, payload.DurationDays)
			if err != nil {
				return ActivateResult{}, err
			}
		default:
			validFrom, validUntil, err = licensesvc.ResolvePoCActivationWindow(now, payload.DurationDays)
			if err != nil {
				return ActivateResult{}, err
			}
		}
	}

	resultPayload := *payload
	resultPayload.IssuedAt = validFrom
	resultPayload.ValidUntil = validUntil
	if alreadyExists {
		result, err = s.db.Exec(`
			UPDATE licenses SET
				license_type=?, device_count=?, camera_count=?, enabled_features=?,
				valid_from=?, valid_until=?, is_active=1
			WHERE license_key=?
		`, licenseType, payload.DeviceCount, payload.CameraCount, string(featuresJSON), validFrom, validUntil, key)
	} else {
		result, err = s.db.Exec(`
			INSERT INTO licenses (license_key, license_type, device_count, camera_count, enabled_features, valid_from, valid_until, is_active)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		`, key, licenseType, payload.DeviceCount, payload.CameraCount, string(featuresJSON), validFrom, validUntil)
	}
	if err != nil {
		return ActivateResult{}, err
	}

	if payload.DeviceCount > 0 || hasFeature(payload.Features, "device_management") {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('device_management_enabled', '1', 'Device management module enabled')`)
	}

	if payload.CameraCount > 0 || hasFeature(payload.Features, "camera_viewer") {
		if ensureCameraSchema != nil {
			ensureCameraSchema()
		}
		maxCameras := payload.CameraCount
		if maxCameras == 0 {
			maxCameras = 4
		}
		tier := "free"
		if maxCameras >= 16 {
			tier = "enterprise"
		} else if maxCameras >= 9 {
			tier = "standard"
		}
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_license', ?, 'Camera License Tier')`, tier)
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_max_cameras', ?, 'Maximum cameras allowed')`, maxCameras)
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_enabled', '1', 'Camera viewer module enabled')`)
		log.Printf("[License] Camera module enabled (tier: %s, max: %d).", tier, maxCameras)
	}

	if hasFeature(payload.Features, "access_control") {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('access_control_enabled', '1', 'Access Control module enabled')`)
	}
	if hasFeature(payload.Features, "pdu") {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('pdu_enabled', '1', 'PDU/UPS module enabled')`)
	}
	if hasFeature(payload.Features, "camera_recording") {
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_recording_enabled', '1', 'Camera NVR recording enabled')`)
	}

	id, _ := result.LastInsertId()
	return ActivateResult{
		ID:          id,
		Message:     activationMessage(&resultPayload),
		Payload:     &resultPayload,
		LicenseType: licenseType,
		Reactivated: alreadyExists,
	}, nil
}

func (s *Service) ActivateTrial(machineID string) (TrialActivateResult, error) {
	var trialActivated string
	_ = s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'trial_activated'").Scan(&trialActivated)
	if trialActivated == "true" {
		return TrialActivateResult{}, errors.New("trial_already_used")
	}

	payload := licensesvc.GenerateTrialLicense(machineID)
	featuresJSON, _ := json.Marshal(payload.Features)
	if _, err := s.db.Exec(`
		INSERT INTO licenses (license_key, license_type, device_count, enabled_features, valid_from, valid_until, is_active)
		VALUES (?, ?, ?, ?, ?, ?, 1)
	`, "TRIAL-"+machineID[:8], "trial", payload.DeviceCount, string(featuresJSON), payload.IssuedAt, payload.ValidUntil); err != nil {
		return TrialActivateResult{}, err
	}

	_, _ = s.db.Exec("INSERT OR REPLACE INTO system_config (config_key, config_value) VALUES ('trial_activated', 'true')")
	return TrialActivateResult{
		Message: "trial license activated",
		Payload: payload,
	}, nil
}

func (s *Service) GenerateLicenseKey(input GenerateInput, formalSecret, pocSecret []byte) (GenerateResult, error) {
	licenseMode := strings.ToLower(strings.TrimSpace(input.LicenseMode))
	if licenseMode == "" {
		licenseMode = licensesvc.FormalLicenseMode
	}

	licenseType := input.LicenseType
	if licenseType == "" {
		if input.AlertOnly || (input.DeviceCount <= 0 && input.CameraCount <= 0) {
			licenseType = "alert"
		} else if input.CameraCount > 0 && input.DeviceCount <= 0 {
			licenseType = "camera"
		} else {
			licenseType = "device"
		}
	}

	if licenseMode != licensesvc.PoCLicenseMode && strings.TrimSpace(input.MachineID) == "" {
		return GenerateResult{}, errors.New("formal license requires machine_id")
	}
	if licenseMode != licensesvc.PoCLicenseMode && input.DurationDays > 0 {
		return GenerateResult{}, errors.New("duration_days is only supported for poc licenses")
	}

	isAnnual := licenseType == "device" || licenseType == "combined" || licenseType == "camera" || licenseType == "access_control" || licenseType == "full"
	if isAnnual && licenseMode != licensesvc.PoCLicenseMode {
		if input.Years < 1 || (input.Years > 5 && !licensesvc.IsPermanentYears(input.Years)) {
			return GenerateResult{}, errors.New("years must be between 1 and 5, or 50 and above for permanent license")
		}
	}
	if (licenseType == "device" || licenseType == "combined" || licenseType == "full") && input.DeviceCount <= 0 {
		return GenerateResult{}, errors.New("device_count must be greater than 0")
	}
	if (licenseType == "camera" || licenseType == "full") && input.CameraCount <= 0 {
		return GenerateResult{}, errors.New("camera_count must be greater than 0")
	}

	secretKey := formalSecret
	if licenseMode == licensesvc.PoCLicenseMode {
		secretKey = pocSecret
	}

	validUntil := strings.TrimSpace(input.ValidUntil)
	if licenseMode == licensesvc.PoCLicenseMode {
		if input.DurationDays > 0 && validUntil != "" {
			return GenerateResult{}, errors.New("use either duration_days or valid_until for poc license")
		}
		if input.DurationDays > 0 {
			if err := licensesvc.ValidatePoCDurationDays(input.DurationDays); err != nil {
				return GenerateResult{}, err
			}
			validUntil = ""
		} else if validUntil == "" {
			return GenerateResult{}, errors.New("poc license requires duration_days or valid_until")
		}
		if validUntil != "" {
			if _, err := licensesvc.ParseLicenseTime(validUntil); err != nil {
				return GenerateResult{}, errors.New("invalid valid_until format")
			}
		}
	} else if isAnnual {
		if licensesvc.IsPermanentYears(input.Years) {
			validUntil = ""
		} else {
			validUntil = time.Now().AddDate(input.Years, 0, 0).Format("2006-01-02")
		}
	}

	features := input.Features
	cameraCount := 0
	switch licenseType {
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
	if licenseType == "alert" || licenseType == "camera" || licenseType == "access_control" || licenseType == "pdu" {
		deviceCount = 0
	}

	licenseKey, err := licensesvc.GenerateLicenseKeyAdvancedWithDuration(
		licenseMode,
		input.MachineID,
		deviceCount,
		cameraCount,
		features,
		validUntil,
		input.DurationDays,
		secretKey,
	)
	if err != nil {
		return GenerateResult{}, err
	}

	typeDisplay := map[string]string{
		"device":         "device management",
		"alert":          "alert channels",
		"camera":         "camera viewer",
		"access_control": "access control",
		"pdu":            "pdu",
		"combined":       "device + alerts",
		"full":           "full suite",
	}[licenseType]
	if typeDisplay == "" {
		typeDisplay = licenseType
	}

	return GenerateResult{
		LicenseKey:   licenseKey,
		LicenseMode:  licenseMode,
		LicenseType:  licenseType,
		TypeDisplay:  typeDisplay,
		DeviceCount:  deviceCount,
		CameraCount:  cameraCount,
		Years:        input.Years,
		DurationDays: input.DurationDays,
		Features:     features,
		ValidUntil:   validUntil,
		IsPermanent:  validUntil == "" && input.DurationDays == 0,
	}, nil
}

func (s *Service) ResetIdentity() error {
	if _, err := s.db.Exec("DELETE FROM system_config WHERE config_key = 'aes_secret_key_v2'"); err != nil {
		return err
	}
	_, _ = s.db.Exec("DELETE FROM licenses")
	_, _ = s.db.Exec("DELETE FROM system_config WHERE config_key = 'trial_activated'")
	return nil
}

func (s *Service) ReissueLicense(input ReissueInput, currentMachineID string, currentSecret, pocSecret []byte) (ReissueResult, error) {
	oldSecretKey := licensesvc.DeriveKey("NMS-LICENSE-" + input.OldMachineID)
	oldPayload, err := licensesvc.ValidateLicenseKey(input.OldLicenseKey, input.OldMachineID, oldSecretKey, pocSecret)
	if err != nil {
		return ReissueResult{}, err
	}

	if oldPayload.ValidUntil != "" {
		validUntil, err := time.Parse("2006-01-02", oldPayload.ValidUntil)
		if err == nil && time.Now().After(validUntil) {
			return ReissueResult{}, errors.New("license_expired")
		}
	}

	newLicenseKey, err := licensesvc.GenerateLicenseKey(currentMachineID, oldPayload.DeviceCount, oldPayload.Features, oldPayload.ValidUntil, currentSecret)
	if err != nil {
		return ReissueResult{}, err
	}

	var existingID int
	if err := s.db.QueryRow("SELECT id FROM licenses WHERE license_key = ?", newLicenseKey).Scan(&existingID); err != nil {
		featuresJSON, _ := json.Marshal(oldPayload.Features)
		if _, err := s.db.Exec(`
			INSERT INTO licenses (license_key, license_type, device_count, enabled_features, valid_from, valid_until, is_active)
			VALUES (?, ?, ?, ?, ?, ?, 1)
		`, newLicenseKey, "reissue", oldPayload.DeviceCount, string(featuresJSON), oldPayload.IssuedAt, oldPayload.ValidUntil); err != nil {
			return ReissueResult{}, err
		}
	}

	return ReissueResult{NewLicenseKey: newLicenseKey}, nil
}

func (s *Service) ListAdminLicenses(systemUUID string) (AdminLicenseList, error) {
	var deviceCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&deviceCount); err != nil {
		return AdminLicenseList{}, err
	}

	rows, err := s.db.Query(`
		SELECT id, license_key, license_type, COALESCE(device_count, 0), valid_until, is_active
		FROM licenses
		ORDER BY id DESC
	`)
	if err != nil {
		return AdminLicenseList{}, err
	}
	defer rows.Close()

	licenses := []AdminLicenseRecord{}
	for rows.Next() {
		var item AdminLicenseRecord
		var validUntil sql.NullString
		if err := rows.Scan(&item.ID, &item.LicenseKey, &item.LicenseType, &item.MaxDevices, &validUntil, &item.IsActive); err != nil {
			continue
		}
		item.ValidUntil = validUntil.String
		licenses = append(licenses, item)
	}

	return AdminLicenseList{
		DeviceCount: deviceCount,
		SystemUUID:  systemUUID,
		Licenses:    licenses,
	}, rows.Err()
}

func (s *Service) CreateLegacyLicense(input CreateLegacyLicenseInput) (int64, error) {
	licenseType := strings.TrimSpace(input.LicenseType)
	if licenseType == "" {
		licenseType = "basic"
	}
	maxDevices := input.MaxDevices
	if maxDevices == 0 {
		maxDevices = 100
	}
	validFrom := strings.TrimSpace(input.ValidFrom)
	if validFrom == "" {
		validFrom = time.Now().Format("2006-01-02 15:04:05")
	}
	result, err := s.db.Exec(`
		INSERT INTO licenses (license_key, license_type, device_count, valid_from, valid_until)
		VALUES (?, ?, ?, ?, ?)
	`, input.LicenseKey, licenseType, maxDevices, validFrom, input.ValidUntil)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func activationMessage(payload *licensesvc.LicensePayload) string {
	messages := []string{"license activated"}
	if payload.CameraCount > 0 || hasFeature(payload.Features, "camera_viewer") {
		messages = append(messages, "camera viewer enabled")
	}
	if hasFeature(payload.Features, "access_control") {
		messages = append(messages, "access control enabled")
	}
	if hasFeature(payload.Features, "pdu") {
		messages = append(messages, "pdu enabled")
	}
	if hasFeature(payload.Features, "camera_recording") {
		messages = append(messages, "camera recording enabled")
	}
	return strings.Join(messages, " | ")
}

func hasFeature(features []string, target string) bool {
	for _, feature := range features {
		if strings.EqualFold(strings.TrimSpace(feature), target) {
			return true
		}
	}
	return false
}

func parseConfigInt(s string) int {
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}
