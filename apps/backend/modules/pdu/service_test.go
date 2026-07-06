// Made by YTSworks
// YTS工作室製作
package pdu

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupPDUServiceTest(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE pdu_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			location TEXT,
			ip_address TEXT,
			port INTEGER,
			snmp_community TEXT,
			snmp_version INTEGER,
			device_type TEXT,
			manufacturer TEXT,
			model TEXT,
			is_enabled INTEGER,
			status TEXT,
			last_polled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO pdu_devices (
			name, location, ip_address, port, snmp_community, snmp_version,
			device_type, manufacturer, model, is_enabled, status
		) VALUES (
			'Rack PDU', 'Rack A', '10.0.0.20', 161, 'private', 2,
			'ups', 'APC', 'PDU9000', 1, 'online'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return NewService(db), db
}

func TestPDUJSONDoesNotExposeSNMPCommunity(t *testing.T) {
	service, _ := setupPDUServiceTest(t)

	device, err := service.GetDevice("1", false)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(device)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	if strings.Contains(text, "private") || strings.Contains(text, "snmp_community") {
		t.Fatalf("PDU response leaked SNMP community: %s", text)
	}
}

func TestUpdatePDUBlankSNMPCommunityDoesNotOverwrite(t *testing.T) {
	service, db := setupPDUServiceTest(t)

	err := service.UpdateDevice("1", DeviceRequest{
		Name:          "Rack PDU Updated",
		Location:      "Rack A",
		IPAddress:     "10.0.0.20",
		Port:          161,
		SNMPCommunity: "",
		SNMPVersion:   2,
		DeviceType:    "ups",
		Manufacturer:  "APC",
		Model:         "PDU9000",
		IsEnabled:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var community string
	if err := db.QueryRow(`SELECT snmp_community FROM pdu_devices WHERE id=1`).Scan(&community); err != nil {
		t.Fatal(err)
	}
	if community != "private" {
		t.Fatalf("expected existing SNMP community to be preserved, got %q", community)
	}
}
