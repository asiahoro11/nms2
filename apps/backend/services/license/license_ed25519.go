package license

// Ed25519 License V1. The private key must remain in the issuer only;
// the NMS runtime should be configured with the public key.

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const Ed25519LicensePrefix = "ED25519-V1."

type SignedLicense struct {
	LicenseMode  string   `json:"license_mode,omitempty"`
	MachineID    string   `json:"machine_id,omitempty"`
	DeviceCount  int      `json:"device_count"`
	CameraCount  int      `json:"camera_count"`
	DurationDays int      `json:"duration_days,omitempty"`
	Features     []string `json:"features"`
	IssuedAt     string   `json:"issued_at"`
	ValidUntil   string   `json:"valid_until"`
}

var b64url = base64.RawURLEncoding

var supportedSignedLicenseFeatures = map[string]struct{}{
	"device_management": {},
	"camera_viewer":     {},
	"camera_recording":  {},
	"access_control":    {},
	"pdu":               {},
	"iot":               {},
	"line":              {},
	"telegram":          {},
	"whatsapp":          {},
	"discord":           {},
	"slack":             {},
}

// GenerateEd25519KeyPair is intended for an offline issuer setup tool only.
func GenerateEd25519KeyPair() (publicKey, privateKey []byte, err error) {
	publicKey, privateKey, err = ed25519.GenerateKey(rand.Reader)
	return
}

func SignEd25519License(payload SignedLicense, privateKey []byte) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("invalid ed25519 private key")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal license payload: %w", err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(privateKey), data)
	return Ed25519LicensePrefix + b64url.EncodeToString(data) + "." + b64url.EncodeToString(sig), nil
}

func VerifyEd25519License(encoded string, publicKey []byte) (SignedLicense, error) {
	var payload SignedLicense
	if len(publicKey) != ed25519.PublicKeySize {
		return payload, errors.New("invalid ed25519 public key")
	}
	value := strings.TrimSpace(encoded)
	if !strings.HasPrefix(value, Ed25519LicensePrefix) {
		return payload, errors.New("unsupported license format")
	}
	parts := strings.Split(strings.TrimPrefix(value, Ed25519LicensePrefix), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return payload, errors.New("malformed ed25519 license")
	}
	data, err := b64url.DecodeString(parts[0])
	if err != nil {
		return payload, errors.New("invalid license payload encoding")
	}
	sig, err := b64url.DecodeString(parts[1])
	if err != nil || len(sig) != ed25519.SignatureSize {
		return payload, errors.New("invalid license signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), data, sig) {
		return payload, errors.New("invalid license signature")
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, errors.New("invalid license payload")
	}
	return payload, nil
}

// ValidateSignedLicensePayload applies runtime schema and entitlement
// invariants after the Ed25519 signature has been verified.
func ValidateSignedLicensePayload(payload SignedLicense) error {
	if payload.LicenseMode != FormalLicenseMode && payload.LicenseMode != PoCLicenseMode {
		return errors.New("invalid license mode")
	}
	if payload.DeviceCount < 0 || payload.CameraCount < 0 {
		return errors.New("license counts cannot be negative")
	}
	if len(payload.Features) == 0 {
		return errors.New("license must contain at least one feature")
	}
	seen := make(map[string]struct{}, len(payload.Features))
	for _, feature := range payload.Features {
		if feature != strings.ToLower(strings.TrimSpace(feature)) {
			return errors.New("license feature is not normalized")
		}
		if _, ok := supportedSignedLicenseFeatures[feature]; !ok {
			return fmt.Errorf("unsupported license feature: %s", feature)
		}
		if _, duplicate := seen[feature]; duplicate {
			return fmt.Errorf("duplicate license feature: %s", feature)
		}
		seen[feature] = struct{}{}
	}
	if _, ok := seen["device_management"]; ok && payload.DeviceCount < 1 {
		return errors.New("device management requires a positive device count")
	}
	if _, ok := seen["camera_viewer"]; ok && payload.CameraCount < 1 {
		return errors.New("camera viewer requires a positive camera count")
	}
	if _, ok := seen["camera_recording"]; ok && payload.CameraCount < 1 {
		return errors.New("camera recording requires a positive camera count")
	}

	issuedAt, err := time.Parse(time.RFC3339, payload.IssuedAt)
	if err != nil {
		return errors.New("invalid issued_at")
	}
	if payload.LicenseMode == FormalLicenseMode {
		if strings.TrimSpace(payload.MachineID) == "" {
			return errors.New("formal license requires a machine ID")
		}
		if payload.DurationDays != 0 {
			return errors.New("formal license cannot use duration_days")
		}
		if payload.ValidUntil != "" {
			expiresAt, err := time.Parse(time.RFC3339, payload.ValidUntil)
			if err != nil || !expiresAt.After(issuedAt) {
				return errors.New("invalid valid_until")
			}
		}
		return nil
	}

	if strings.TrimSpace(payload.MachineID) != "" {
		return errors.New("PoC license cannot be machine-bound")
	}
	if payload.ValidUntil != "" {
		return errors.New("PoC license expiry must be resolved on first activation")
	}
	return ValidatePoCDurationDays(payload.DurationDays)
}
