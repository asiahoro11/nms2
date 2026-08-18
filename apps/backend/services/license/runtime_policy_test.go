package license

import (
	"encoding/base64"
	"testing"
	"time"
)

func withBuildPublicKey(t *testing.T, encoded string) {
	t.Helper()
	previous := BuildPublicKeyB64
	BuildPublicKeyB64 = encoded
	t.Cleanup(func() { BuildPublicKeyB64 = previous })
}

func validFormalPayload(machineID string) SignedLicense {
	return SignedLicense{
		LicenseMode: FormalLicenseMode,
		MachineID:   machineID,
		DeviceCount: 10,
		Features:    []string{"device_management"},
		IssuedAt:    time.Now().UTC().Format(time.RFC3339),
		ValidUntil:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}
}

func TestRuntimeTrustAnchorCannotBeOverriddenByEnvironment(t *testing.T) {
	trustedPublic, trustedPrivate, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	untrustedPublic, untrustedPrivate, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	withBuildPublicKey(t, base64.StdEncoding.EncodeToString(trustedPublic))
	t.Setenv("NMS_LICENSE_PUBLIC_KEY_B64", base64.StdEncoding.EncodeToString(untrustedPublic))
	if err := ConfigureRuntimeValidation("machine-1"); err != nil {
		t.Fatal(err)
	}

	untrustedKey, err := SignEd25519License(validFormalPayload("machine-1"), untrustedPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRuntimeLicenseKey(untrustedKey, "machine-1", RuntimePublicKey()); err == nil {
		t.Fatal("environment-overridden trust anchor was accepted")
	}
	trustedKey, err := SignEd25519License(validFormalPayload("machine-1"), trustedPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRuntimeLicenseKey(trustedKey, "machine-1", RuntimePublicKey()); err != nil {
		t.Fatalf("embedded trust anchor was rejected: %v", err)
	}
}

func TestConfigureRuntimeValidationRejectsWrongKeyLength(t *testing.T) {
	withBuildPublicKey(t, base64.StdEncoding.EncodeToString([]byte("too-short")))
	if err := ConfigureRuntimeValidation("machine-1"); err == nil {
		t.Fatal("expected invalid embedded public-key length to fail")
	}
}

func TestRuntimeRejectsLegacyLicenseEvenWhenEnvironmentRequestsIt(t *testing.T) {
	publicKey, _, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	withBuildPublicKey(t, base64.StdEncoding.EncodeToString(publicKey))
	t.Setenv("NMS_ALLOW_LEGACY_LICENSE", "1")
	if err := ConfigureRuntimeValidation("machine-1"); err != nil {
		t.Fatal(err)
	}
	legacyKey, err := GenerateLicenseKey(
		"machine-1",
		10,
		[]string{"device_management"},
		"",
		DeriveKey("NMS-LICENSE-machine-1"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRuntimeLicenseKey(legacyKey, "machine-1", RuntimePublicKey()); err == nil {
		t.Fatal("legacy runtime license was accepted")
	}
}

func TestRuntimeRejectsSignedInvalidEntitlement(t *testing.T) {
	publicKey, privateKey, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	payload := validFormalPayload("machine-1")
	payload.DeviceCount = -1
	key, err := SignEd25519License(payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRuntimeLicenseKey(key, "machine-1", publicKey); err == nil {
		t.Fatal("negative signed entitlement was accepted")
	}
}
