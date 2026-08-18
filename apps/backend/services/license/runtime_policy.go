package license

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

// BuildPublicKeyB64 is injected with -ldflags for production releases. An
// environment value can override it to support controlled public-key rotation.
var BuildPublicKeyB64 string

type RuntimeLicense struct {
	Key         string
	Mode        string
	Type        string
	DeviceCount int
	CameraCount int
	Features    []string
}

type runtimeValidationPolicy struct {
	machineID    string
	publicKey    []byte
	legacyFormal []byte
	legacyPoC    []byte
	allowLegacy  bool
}

var runtimePolicy struct {
	sync.RWMutex
	runtimeValidationPolicy
}

func ConfigureRuntimeValidation(machineID string, legacyFormal, legacyPoC []byte) error {
	encoded := strings.TrimSpace(os.Getenv("NMS_LICENSE_PUBLIC_KEY_B64"))
	if encoded == "" {
		encoded = strings.TrimSpace(BuildPublicKeyB64)
	}
	var publicKey []byte
	if encoded != "" {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		publicKey = decoded
	}
	legacyFlag := strings.ToLower(strings.TrimSpace(os.Getenv("NMS_ALLOW_LEGACY_LICENSE")))
	runtimePolicy.Lock()
	runtimePolicy.runtimeValidationPolicy = runtimeValidationPolicy{
		machineID: strings.TrimSpace(machineID), publicKey: append([]byte(nil), publicKey...),
		legacyFormal: append([]byte(nil), legacyFormal...), legacyPoC: append([]byte(nil), legacyPoC...),
		allowLegacy: legacyFlag == "1" || legacyFlag == "true" || legacyFlag == "yes",
	}
	runtimePolicy.Unlock()
	return nil
}

func RuntimePublicKeyConfigured() bool {
	runtimePolicy.RLock()
	defer runtimePolicy.RUnlock()
	return len(runtimePolicy.publicKey) > 0
}

func RuntimePublicKey() []byte {
	runtimePolicy.RLock()
	defer runtimePolicy.RUnlock()
	return append([]byte(nil), runtimePolicy.publicKey...)
}

func RuntimeLegacyAllowed() bool {
	runtimePolicy.RLock()
	defer runtimePolicy.RUnlock()
	return runtimePolicy.allowLegacy
}

// SetRuntimeValidationForTest replaces runtime validation state without
// environment variables. It accepts public verification material only.
func SetRuntimeValidationForTest(machineID string, publicKey []byte, allowLegacy bool) {
	runtimePolicy.Lock()
	runtimePolicy.runtimeValidationPolicy = runtimeValidationPolicy{
		machineID: strings.TrimSpace(machineID), publicKey: append([]byte(nil), publicKey...), allowLegacy: allowLegacy,
	}
	runtimePolicy.Unlock()
}

func VerifiedActiveLicenses(db *sql.DB) []RuntimeLicense {
	if db == nil {
		return nil
	}
	runtimePolicy.RLock()
	policy := runtimeValidationPolicy{
		machineID: runtimePolicy.machineID, publicKey: append([]byte(nil), runtimePolicy.publicKey...),
		legacyFormal: append([]byte(nil), runtimePolicy.legacyFormal...), legacyPoC: append([]byte(nil), runtimePolicy.legacyPoC...),
		allowLegacy: runtimePolicy.allowLegacy,
	}
	runtimePolicy.RUnlock()
	rows, err := db.Query(`SELECT license_key, COALESCE(license_type,'standard'), COALESCE(valid_until,''), COALESCE(enabled_features,'[]') FROM licenses WHERE is_active = 1 AND ` + ActiveLicenseWindowSQL)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var result []RuntimeLicense
	for rows.Next() {
		var key, licenseType, validUntil, featuresJSON string
		if rows.Scan(&key, &licenseType, &validUntil, &featuresJSON) != nil {
			continue
		}
		if strings.EqualFold(licenseType, "trial") {
			prefix := policy.machineID
			if len(prefix) > 8 {
				prefix = prefix[:8]
			}
			if policy.machineID != "" && key == "TRIAL-"+prefix {
				result = append(result, RuntimeLicense{Key: key, Mode: FormalLicenseMode, Type: "trial", DeviceCount: 10, Features: []string{"email"}})
			}
			continue
		}
		payload, err := ValidateLicenseKeyWithPolicy(key, policy.machineID, policy.publicKey, policy.legacyFormal, policy.legacyPoC, policy.allowLegacy)
		if err != nil {
			continue
		}
		features := payload.Features
		if len(features) == 0 {
			_ = json.Unmarshal([]byte(featuresJSON), &features)
		}
		result = append(result, RuntimeLicense{Key: key, Mode: payload.LicenseMode, Type: licenseType, DeviceCount: payload.DeviceCount, CameraCount: payload.CameraCount, Features: features})
	}
	return result
}

func ActiveDeviceCount(db *sql.DB) int {
	total := 0
	for _, item := range VerifiedActiveLicenses(db) {
		total += item.DeviceCount
	}
	return total
}

func ActiveCameraCount(db *sql.DB) int {
	total := 0
	for _, item := range VerifiedActiveLicenses(db) {
		total += item.CameraCount
	}
	return total
}
