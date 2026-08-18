package license

import "testing"

func TestEd25519LicenseRoundTrip(t *testing.T) {
	pub, priv, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	want := SignedLicense{LicenseMode: FormalLicenseMode, MachineID: "machine-1", DeviceCount: 10, Features: []string{"iot"}}
	key, err := SignEd25519License(want, priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyEd25519License(key, pub)
	if err != nil {
		t.Fatal(err)
	}
	if got.MachineID != want.MachineID || got.DeviceCount != want.DeviceCount {
		t.Fatalf("payload mismatch: %+v", got)
	}
}

func TestEd25519LicenseRejectsTampering(t *testing.T) {
	pub, priv, _ := GenerateEd25519KeyPair()
	key, _ := SignEd25519License(SignedLicense{MachineID: "machine-1"}, priv)
	if _, err := VerifyEd25519License(key+"x", pub); err == nil {
		t.Fatal("tampered license accepted")
	}
}
