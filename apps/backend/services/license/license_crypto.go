package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	FormalLicenseMode  = "formal"
	PoCLicenseMode     = "poc"
	PoCLicensePrefix   = "POC1-"
	PermanentYearCut   = 50
	MaxPoCDurationDays = 3650
)

var supportedTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// LicensePayload represents the decrypted license data.
type LicensePayload struct {
	LicenseMode   string   `json:"license_mode,omitempty"`
	MachineID     string   `json:"machine_id,omitempty"`
	DeviceCount   int      `json:"device_count"`
	CameraCount   int      `json:"camera_count"`
	DurationYears int      `json:"duration_years"`
	DurationDays  int      `json:"duration_days,omitempty"`
	Features      []string `json:"features"`
	IssuedAt      string   `json:"issued_at"`
	ValidUntil    string   `json:"valid_until"`
}

func normalizeLicenseMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), PoCLicenseMode) {
		return PoCLicenseMode
	}
	return FormalLicenseMode
}

func IsDeferredDurationPoC(payload *LicensePayload) bool {
	if payload == nil {
		return false
	}
	return payload.LicenseMode == PoCLicenseMode && payload.DurationDays > 0 && strings.TrimSpace(payload.ValidUntil) == ""
}

func ValidatePoCDurationDays(days int) error {
	if days < 1 || days > MaxPoCDurationDays {
		return errors.New("duration_days must be between 1 and 3650")
	}
	return nil
}

func ResolvePoCActivationWindow(start time.Time, durationDays int) (string, string, error) {
	if err := ValidatePoCDurationDays(durationDays); err != nil {
		return "", "", err
	}
	return start.Format(time.RFC3339), start.AddDate(0, 0, durationDays).Format(time.RFC3339), nil
}

// IsPermanentYears treats any year value at or above the cutoff as a perpetual license request.
func IsPermanentYears(years int) bool {
	return years >= PermanentYearCut
}

// IsPermanentValidity treats empty expiry or very long validity windows as perpetual.
func IsPermanentValidity(validFrom string, validUntil string) bool {
	if strings.TrimSpace(validUntil) == "" {
		return true
	}

	end, err := ParseLicenseTime(validUntil)
	if err != nil {
		return false
	}

	start, err := ParseLicenseTime(validFrom)
	if err != nil {
		start = time.Now()
	}

	return !end.Before(start.AddDate(PermanentYearCut, 0, 0))
}

// DeriveKey creates a 32-byte key from any string using SHA256.
func DeriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}

// Encrypt encrypts plaintext using AES-256-GCM and returns base64 encoded string.
func Encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64 encoded ciphertext using AES-256-GCM.
func Decrypt(ciphertext string, key []byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce, cipherData := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, cipherData, nil)
}

// EncryptMachineID encrypts the machine ID for customer display.
func EncryptMachineID(machineID string, secretKey []byte) (string, error) {
	return Encrypt([]byte(machineID), secretKey)
}

// DetectLicenseEnvelope determines which secret path to use and returns the raw ciphertext.
func DetectLicenseEnvelope(encryptedKey string) (string, string) {
	key := strings.TrimSpace(encryptedKey)
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.ReplaceAll(key, " ", "")

	if strings.HasPrefix(strings.ToUpper(key), PoCLicensePrefix) {
		return PoCLicenseMode, key[len(PoCLicensePrefix):]
	}

	return FormalLicenseMode, key
}

// ParseLicenseTime accepts both date-only and exact timestamp expiry values.
func ParseLicenseTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("empty timestamp")
	}

	var lastErr error
	for _, layout := range supportedTimeLayouts {
		ts, err := time.Parse(layout, trimmed)
		if err == nil {
			if layout == "2006-01-02" {
				return ts.Add(23*time.Hour + 59*time.Minute + 59*time.Second), nil
			}
			return ts, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("unsupported timestamp format")
	}
	return time.Time{}, lastErr
}

// ValidateLicenseKey validates and decrypts a license key.
func ValidateLicenseKey(encryptedKey string, systemMachineID string, formalSecretKey []byte, pocSecretKey []byte) (*LicensePayload, error) {
	mode, rawKey := DetectLicenseEnvelope(encryptedKey)

	secretKey := formalSecretKey
	if mode == PoCLicenseMode {
		secretKey = pocSecretKey
	}

	decrypted, err := Decrypt(rawKey, secretKey)
	if err != nil {
		return nil, errors.New("invalid license key")
	}

	var payload LicensePayload
	if err := json.Unmarshal(decrypted, &payload); err != nil {
		return nil, errors.New("invalid license payload")
	}

	payload.LicenseMode = normalizeLicenseMode(payload.LicenseMode)
	if mode == PoCLicenseMode {
		payload.LicenseMode = PoCLicenseMode
	}

	if payload.LicenseMode != PoCLicenseMode && payload.MachineID != systemMachineID {
		return nil, errors.New("license is bound to a different machine")
	}

	if payload.ValidUntil != "" {
		validUntil, err := ParseLicenseTime(payload.ValidUntil)
		if err != nil {
			return nil, errors.New("invalid license expiry time")
		}
		if time.Now().After(validUntil) {
			return nil, errors.New("license has expired")
		}
	}

	return &payload, nil
}

// GenerateTrialLicense creates a 14-day trial license.
func GenerateTrialLicense(machineID string) *LicensePayload {
	now := time.Now()
	return &LicensePayload{
		LicenseMode:   FormalLicenseMode,
		MachineID:     machineID,
		DeviceCount:   10,
		DurationYears: 0,
		Features:      []string{"email"},
		IssuedAt:      now.Format("2006-01-02"),
		ValidUntil:    now.AddDate(0, 0, 14).Format("2006-01-02"),
	}
}

// IsTrialKey checks if the input is a trial activation request.
func IsTrialKey(key string) bool {
	return strings.ToUpper(strings.TrimSpace(key)) == "TRIAL"
}

// GenerateLicenseKey generates a formal encrypted license key.
func GenerateLicenseKey(machineID string, deviceCount int, features []string, validUntil string, secretKey []byte) (string, error) {
	return GenerateLicenseKeyWithCameras(machineID, deviceCount, 0, features, validUntil, secretKey)
}

// GenerateLicenseKeyWithCameras generates a formal encrypted license key with camera support.
func GenerateLicenseKeyWithCameras(machineID string, deviceCount int, cameraCount int, features []string, validUntil string, secretKey []byte) (string, error) {
	return GenerateLicenseKeyAdvanced(FormalLicenseMode, machineID, deviceCount, cameraCount, features, validUntil, secretKey)
}

// GenerateLicenseKeyAdvanced generates a formal or PoC encrypted license key.
func GenerateLicenseKeyAdvanced(mode string, machineID string, deviceCount int, cameraCount int, features []string, validUntil string, secretKey []byte) (string, error) {
	return GenerateLicenseKeyAdvancedWithDuration(mode, machineID, deviceCount, cameraCount, features, validUntil, 0, secretKey)
}

// GenerateLicenseKeyAdvancedWithDuration generates a formal or PoC encrypted license key,
// optionally deferring PoC expiry calculation until activation time.
func GenerateLicenseKeyAdvancedWithDuration(mode string, machineID string, deviceCount int, cameraCount int, features []string, validUntil string, durationDays int, secretKey []byte) (string, error) {
	now := time.Now()
	payload := LicensePayload{
		LicenseMode:   normalizeLicenseMode(mode),
		MachineID:     strings.TrimSpace(machineID),
		DeviceCount:   deviceCount,
		CameraCount:   cameraCount,
		DurationYears: 0,
		DurationDays:  durationDays,
		Features:      features,
		IssuedAt:      now.Format(time.RFC3339),
		ValidUntil:    strings.TrimSpace(validUntil),
	}

	if payload.LicenseMode == PoCLicenseMode {
		payload.MachineID = ""
		if payload.ValidUntil != "" {
			payload.DurationDays = 0
		} else if err := ValidatePoCDurationDays(payload.DurationDays); err != nil {
			return "", err
		}
	} else {
		payload.DurationDays = 0
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	encrypted, err := Encrypt(jsonData, secretKey)
	if err != nil {
		return "", err
	}

	if payload.LicenseMode == PoCLicenseMode {
		return PoCLicensePrefix + encrypted, nil
	}

	return encrypted, nil
}
