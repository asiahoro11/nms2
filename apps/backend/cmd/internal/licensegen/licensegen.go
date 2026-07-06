package licensegen

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"management-server/services/license"
)

const (
	ProductVersion   = "v1.2.4.9sp0001"
	FormalSecretSeed = "NMS-LICENSE-"
	// Keep the PoC seed stable so existing generated PoC keys remain compatible.
	PoCSecretSeed = "NMS-POC-LICENSE-v1.2.1-PoC"
)

type FeatureOption struct {
	Key         string
	Label       string
	Description string
}

var FeatureOptions = []FeatureOption{
	{Key: "device_management", Label: "設備管理", Description: "啟用設備管理功能，會使用設備數量額度。"},
	{Key: "camera_viewer", Label: "攝影機檢視", Description: "啟用攝影機監控頁面，會使用攝影機路數額度。"},
	{Key: "camera_recording", Label: "攝影機錄影", Description: "啟用 NVR 錄影流程，會使用攝影機路數額度。"},
	{Key: "access_control", Label: "門禁管理", Description: "啟用門禁管理模組。"},
	{Key: "pdu", Label: "PDU / UPS", Description: "啟用 PDU 與 UPS 監控模組。"},
	{Key: "iot", Label: "IoT / Modbus", Description: "啟用 IoT 與 Modbus 整合模組。"},
	{Key: "line", Label: "LINE 通知", Description: "啟用 LINE 告警通道。"},
	{Key: "telegram", Label: "Telegram", Description: "啟用 Telegram 告警通道。"},
	{Key: "whatsapp", Label: "WhatsApp", Description: "啟用 WhatsApp 告警通道。"},
	{Key: "discord", Label: "Discord", Description: "啟用 Discord 告警通道。"},
	{Key: "slack", Label: "Slack", Description: "啟用 Slack 告警通道。"},
}

type GenerateInput struct {
	Mode         string
	MachineID    string
	LicenseType  string
	Features     []string
	DeviceCount  int
	CameraCount  int
	Years        int
	DurationDays int
}

type GenerateResult struct {
	Key          string
	Description  string
	Mode         string
	LicenseType  string
	Features     []string
	Labels       []string
	DeviceCount  int
	CameraCount  int
	ValidUntil   string
	DurationDays int
}

type Profile struct {
	Features    []string
	Labels      []string
	DeviceCount int
	CameraCount int
}

func DefaultFeatureKeys() []string {
	return []string{"device_management"}
}

func NormalizeSelectedFeatures(raw []string) []string {
	selected := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		key := strings.ToLower(strings.TrimSpace(item))
		if key != "" {
			selected[key] = struct{}{}
		}
	}

	result := make([]string, 0, len(selected))
	for _, option := range FeatureOptions {
		if _, ok := selected[option.Key]; ok {
			result = append(result, option.Key)
		}
	}
	return result
}

func ContainsFeature(slice []string, item string) bool {
	for _, value := range slice {
		if value == item {
			return true
		}
	}
	return false
}

func FeatureKeysForType(licenseType string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(licenseType)) {
	case "", "custom":
		return nil, nil
	case "device":
		return []string{"device_management"}, nil
	case "alert", "alerts":
		return []string{"line", "telegram", "whatsapp", "discord", "slack"}, nil
	case "camera", "camera_viewer":
		return []string{"camera_viewer"}, nil
	case "camera_recording", "nvr":
		return []string{"camera_recording"}, nil
	case "access_control", "access":
		return []string{"access_control"}, nil
	case "pdu", "ups":
		return []string{"pdu"}, nil
	case "iot", "modbus":
		return []string{"iot"}, nil
	case "combined":
		return []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack"}, nil
	case "full", "all":
		return []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack", "camera_viewer", "camera_recording", "access_control", "pdu", "iot"}, nil
	default:
		return nil, fmt.Errorf("unknown license type: %s", licenseType)
	}
}

func BuildProfile(selectedFeatures []string, deviceCount, cameraCount int) (Profile, error) {
	if len(selectedFeatures) == 0 {
		return Profile{}, fmt.Errorf("select at least one feature")
	}

	known := make(map[string]FeatureOption, len(FeatureOptions))
	for _, option := range FeatureOptions {
		known[option.Key] = option
	}

	unique := map[string]struct{}{}
	for _, feature := range selectedFeatures {
		key := strings.ToLower(strings.TrimSpace(feature))
		if _, ok := known[key]; ok {
			unique[key] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return Profile{}, fmt.Errorf("select at least one supported feature")
	}

	keys := make([]string, 0, len(unique))
	for _, option := range FeatureOptions {
		if _, ok := unique[option.Key]; ok {
			keys = append(keys, option.Key)
		}
	}
	sort.Strings(keys)

	profile := Profile{
		Features: make([]string, 0, len(keys)),
		Labels:   make([]string, 0, len(keys)),
	}
	needsDeviceCount := false
	needsCameraCount := false
	for _, option := range FeatureOptions {
		if _, ok := unique[option.Key]; !ok {
			continue
		}
		profile.Features = append(profile.Features, option.Key)
		profile.Labels = append(profile.Labels, option.Label)
		if option.Key == "device_management" {
			needsDeviceCount = true
		}
		if option.Key == "camera_viewer" || option.Key == "camera_recording" {
			needsCameraCount = true
		}
	}

	if needsDeviceCount {
		if deviceCount <= 0 {
			return Profile{}, fmt.Errorf("device_count must be greater than 0 when device management is selected")
		}
		profile.DeviceCount = deviceCount
	}
	if needsCameraCount {
		if cameraCount <= 0 {
			return Profile{}, fmt.Errorf("camera_count must be greater than 0 when camera features are selected")
		}
		profile.CameraCount = cameraCount
	}

	return profile, nil
}

func Generate(input GenerateInput) (GenerateResult, error) {
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = license.FormalLicenseMode
	}

	features := input.Features
	if len(features) == 0 {
		typeFeatures, err := FeatureKeysForType(input.LicenseType)
		if err != nil {
			return GenerateResult{}, err
		}
		features = typeFeatures
	}

	if mode != license.PoCLicenseMode && strings.TrimSpace(input.MachineID) == "" {
		return GenerateResult{}, fmt.Errorf("formal license requires machine_id")
	}
	if mode == license.PoCLicenseMode && input.DurationDays <= 0 {
		return GenerateResult{}, fmt.Errorf("poc license requires duration_days")
	}

	profile, err := BuildProfile(features, input.DeviceCount, input.CameraCount)
	if err != nil {
		return GenerateResult{}, err
	}

	validUntil := ""
	durationDays := 0
	if mode == license.PoCLicenseMode {
		if err := license.ValidatePoCDurationDays(input.DurationDays); err != nil {
			return GenerateResult{}, err
		}
		durationDays = input.DurationDays
	} else if !license.IsPermanentYears(input.Years) {
		years := input.Years
		if years < 1 {
			years = 1
		}
		validUntil = time.Now().AddDate(years, 0, 0).Format("2006-01-02")
	}

	machineID := strings.ToLower(strings.TrimSpace(input.MachineID))
	secretKey := license.DeriveKey(FormalSecretSeed + machineID)
	if mode == license.PoCLicenseMode {
		secretKey = license.DeriveKey(PoCSecretSeed)
	}

	key, err := license.GenerateLicenseKeyAdvancedWithDuration(
		mode,
		machineID,
		profile.DeviceCount,
		profile.CameraCount,
		profile.Features,
		validUntil,
		durationDays,
		secretKey,
	)
	if err != nil {
		return GenerateResult{}, err
	}

	modeLabel := "Formal"
	if mode == license.PoCLicenseMode {
		modeLabel = "PoC"
	}

	validity := validUntil
	if mode == license.PoCLicenseMode {
		validity = fmt.Sprintf("starts on first activation, %d day(s)", durationDays)
	} else if validUntil == "" {
		validity = "permanent"
	}

	return GenerateResult{
		Key:          key,
		Description:  fmt.Sprintf("%s | features: %s | devices: %d | cameras: %d | validity: %s", modeLabel, strings.Join(profile.Labels, ", "), profile.DeviceCount, profile.CameraCount, validity),
		Mode:         mode,
		LicenseType:  strings.ToLower(strings.TrimSpace(input.LicenseType)),
		Features:     profile.Features,
		Labels:       profile.Labels,
		DeviceCount:  profile.DeviceCount,
		CameraCount:  profile.CameraCount,
		ValidUntil:   validUntil,
		DurationDays: durationDays,
	}, nil
}
