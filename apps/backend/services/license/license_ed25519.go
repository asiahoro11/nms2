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
