package devices

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDeviceServiceTest(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			sys_name TEXT,
			ip_address TEXT,
			mac_address TEXT,
			device_type TEXT,
			snmp_community TEXT,
			snmp_version INTEGER,
			vendor TEXT,
			model TEXT,
			firmware TEXT,
			is_online INTEGER DEFAULT 0,
			last_seen TEXT,
			image_path TEXT,
			pos_x REAL DEFAULT 0,
			pos_y REAL DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO devices (name, ip_address, device_type, snmp_community, snmp_version)
		VALUES ('Core', '10.0.0.1', 'switch', 'private', 2);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(db), db
}

func TestDeviceJSONDoesNotExposeSNMPCommunity(t *testing.T) {
	service, _ := setupDeviceServiceTest(t)

	device, err := service.GetDevice("1")
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	if strings.Contains(text, "private") || strings.Contains(text, "snmp_community") {
		t.Fatalf("device response leaked SNMP community: %s", text)
	}
}

func TestUpdateDeviceBlankSNMPCommunityDoesNotOverwrite(t *testing.T) {
	service, db := setupDeviceServiceTest(t)

	_, fields, err := service.UpdateDevice("1", map[string]interface{}{
		"name":           "Core Updated",
		"snmp_community": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range fields {
		if strings.Contains(field, "snmp_community") {
			t.Fatalf("blank SNMP community should not be updated, fields=%v", fields)
		}
	}

	var community string
	if err := db.QueryRow(`SELECT snmp_community FROM devices WHERE id=1`).Scan(&community); err != nil {
		t.Fatal(err)
	}
	if community != "private" {
		t.Fatalf("expected existing SNMP community to be preserved, got %q", community)
	}
}
