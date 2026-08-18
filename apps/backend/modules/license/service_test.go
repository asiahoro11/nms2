// Made by YTSworks
// YTS工作室製作
package license

import (
	"database/sql"
	"testing"
	"time"

	"management-server/config"
	licensesvc "management-server/services/license"

	_ "modernc.org/sqlite"
)

func newTestService(t *testing.T, version string) (*Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, stmt := range []string{
		`CREATE TABLE licenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			license_key TEXT,
			license_type TEXT,
			device_count INTEGER DEFAULT 0,
			camera_count INTEGER DEFAULT 0,
			enabled_features TEXT DEFAULT '[]',
			valid_from TEXT,
			valid_until TEXT,
			is_active INTEGER DEFAULT 1
		)`,
		`CREATE TABLE system_config (
			config_key TEXT PRIMARY KEY,
			config_value TEXT,
			description TEXT
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	cfg := &config.Config{}
	cfg.System.Version = version
	return NewService(db, cfg), db
}

func insertActiveLicense(t *testing.T, db *sql.DB, licenseType string) {
	t.Helper()
	pub, priv, err := licensesvc.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatal(err)
	}
	licensesvc.SetRuntimeValidationForTest("test-machine", pub, false)
	mode := licensesvc.FormalLicenseMode
	machineID := "test-machine"
	if licenseType == "poc" {
		mode = licensesvc.PoCLicenseMode
		machineID = ""
	}
	key, err := licensesvc.SignEd25519License(licensesvc.SignedLicense{
		LicenseMode: mode, MachineID: machineID, DeviceCount: 10,
		IssuedAt:   time.Now().UTC().Format(time.RFC3339),
		ValidUntil: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}, priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO licenses (license_key, license_type, enabled_features, is_active)
		VALUES (?, ?, '[]', 1)
	`, key, licenseType); err != nil {
		t.Fatalf("insert %s license: %v", licenseType, err)
	}
}

func TestDisplayVersionUsesPoCSuffixForPoCLicense(t *testing.T) {
	svc, db := newTestService(t, "v1.2.4.2")

	insertActiveLicense(t, db, "poc")

	if got := svc.DisplayVersion(); got != "v1.2.4.2-PoC" {
		t.Fatalf("expected PoC display version, got %q", got)
	}
	if !svc.IsPoCEdition() {
		t.Fatal("expected active PoC license to mark runtime as PoC edition")
	}
}

func TestDisplayVersionDropsPoCSuffixForFormalLicense(t *testing.T) {
	svc, db := newTestService(t, "v1.2.4.2-PoC")

	insertActiveLicense(t, db, "standard")

	if got := svc.DisplayVersion(); got != "v1.2.4.2" {
		t.Fatalf("expected formal license to display base version, got %q", got)
	}
	if svc.IsPoCEdition() {
		t.Fatal("expected active formal license to suppress PoC edition mode")
	}
}
