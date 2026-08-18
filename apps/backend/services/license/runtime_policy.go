package license

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// BuildPublicKeyB64 is injected with -ldflags for production releases.
// The runtime trust anchor is intentionally not environment-overridable.
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
	machineID string
	publicKey []byte
}

var runtimePolicy struct {
	sync.RWMutex
	runtimeValidationPolicy
}

func ConfigureRuntimeValidation(machineID string) error {
	encoded := strings.TrimSpace(BuildPublicKeyB64)
	var publicKey []byte
	if encoded != "" {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("decode embedded license public key: %w", err)
		}
		if len(decoded) != ed25519.PublicKeySize {
			return fmt.Errorf("embedded license public key must be %d bytes", ed25519.PublicKeySize)
		}
		publicKey = decoded
	}
	runtimePolicy.Lock()
	runtimePolicy.runtimeValidationPolicy = runtimeValidationPolicy{
		machineID: strings.TrimSpace(machineID), publicKey: append([]byte(nil), publicKey...),
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

// SetRuntimeValidationForTest replaces runtime validation state without
// environment variables. It accepts public verification material only.
func SetRuntimeValidationForTest(machineID string, publicKey []byte) {
	runtimePolicy.Lock()
	runtimePolicy.runtimeValidationPolicy = runtimeValidationPolicy{
		machineID: strings.TrimSpace(machineID), publicKey: append([]byte(nil), publicKey...),
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
		payload, err := ValidateRuntimeLicenseKey(key, policy.machineID, policy.publicKey)
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
