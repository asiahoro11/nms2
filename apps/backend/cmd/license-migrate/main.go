package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"

	"management-server/services/license"
)

// license-migrate is an offline issuer-side tool. It validates a legacy AES
// license first, then reissues the same entitlement as a signed Ed25519 V1
// license. Never install this tool or its private key on an NMS host.
func main() {
	legacyPath := flag.String("legacy-file", "", "file containing one legacy license key")
	machineID := flag.String("machine-id", "", "machine ID for a formal legacy license")
	flag.Parse()

	if strings.TrimSpace(*legacyPath) == "" {
		fail("-legacy-file is required")
	}
	legacyKey, err := os.ReadFile(*legacyPath)
	if err != nil {
		fail("read legacy license: %v", err)
	}

	privateKey, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("NMS_LICENSE_PRIVATE_KEY_B64")))
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		fail("NMS_LICENSE_PRIVATE_KEY_B64 is not a valid Ed25519 private key")
	}

	trimmedMachineID := strings.TrimSpace(*machineID)
	payload, err := license.ValidateLicenseKey(
		strings.TrimSpace(string(legacyKey)),
		trimmedMachineID,
		license.DeriveKey("NMS-LICENSE-"+trimmedMachineID),
		license.DeriveKey("NMS-POC-LICENSE-v1.2.1-PoC"),
	)
	if err != nil {
		fail("legacy license validation failed: %v", err)
	}

	signed := license.SignedLicense{
		LicenseMode:  payload.LicenseMode,
		MachineID:    payload.MachineID,
		DeviceCount:  payload.DeviceCount,
		CameraCount:  payload.CameraCount,
		DurationDays: payload.DurationDays,
		Features:     payload.Features,
		IssuedAt:     payload.IssuedAt,
		ValidUntil:   payload.ValidUntil,
	}
	encoded, err := license.SignEd25519License(signed, privateKey)
	if err != nil {
		fail("sign migrated license: %v", err)
	}
	fmt.Println(encoded)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
