package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"management-server/services/license"
)

func TestIssueFormalLicense(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := license.SignedLicense{
		LicenseMode: license.FormalLicenseMode, MachineID: "ABC-123",
		DeviceCount: 10, Features: []string{"device_management"},
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}
	encoded, err := issue(payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := license.VerifyEd25519License(encoded, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if verified.MachineID != "abc-123" || verified.DeviceCount != 10 {
		t.Fatalf("unexpected payload: %+v", verified)
	}
}

func TestIssueRejectsUnknownFeature(t *testing.T) {
	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	_, err := issue(license.SignedLicense{
		LicenseMode: license.FormalLicenseMode, MachineID: "machine",
		Features: []string{"unknown"}, IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}, privateKey)
	if err == nil {
		t.Fatal("expected unsupported feature to be rejected")
	}
}

func TestIssueRejectsMissingDeviceCount(t *testing.T) {
	_, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	_, err := issue(license.SignedLicense{
		LicenseMode: license.FormalLicenseMode, MachineID: "machine",
		Features: []string{"device_management"}, IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}, privateKey)
	if err == nil {
		t.Fatal("expected missing device count to be rejected")
	}
}

func TestIssuePoCUsesDeferredActivation(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	encoded, err := issue(license.SignedLicense{
		LicenseMode: license.PoCLicenseMode, DurationDays: 14,
		Features: []string{"iot"}, IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := license.VerifyEd25519License(encoded, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ValidUntil != "" || verified.DurationDays != 14 {
		t.Fatalf("unexpected PoC payload: %+v", verified)
	}
}

func TestLoadPrivateKeyFromEnvironmentFile(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	path := filepath.Join(t.TempDir(), "issuer.env")
	content := "NMS_LICENSE_PUBLIC_KEY_B64=" + base64.StdEncoding.EncodeToString(publicKey) + "\n" +
		"NMS_LICENSE_PRIVATE_KEY_B64=" + base64.StdEncoding.EncodeToString(privateKey) + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(privateKey, got) {
		t.Fatal("loaded private key does not match")
	}
}
