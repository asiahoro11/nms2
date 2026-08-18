package notifications

import (
	"database/sql"
	"testing"
	"time"

	"management-server/config"
	licensesvc "management-server/services/license"

	_ "modernc.org/sqlite"
)

func TestGetAlertSettingsSingleConnectionDoesNotBlock(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()

	mustExec(t, db, `CREATE TABLE licenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		license_key TEXT,
		license_type TEXT,
		enabled_features TEXT,
		valid_from TEXT,
		valid_until TEXT,
		is_active BOOLEAN DEFAULT 1
	)`)
	mustExec(t, db, `CREATE TABLE system_config (
		config_key TEXT PRIMARY KEY,
		config_value TEXT
	)`)
	publicKey, privateKey, err := licensesvc.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	licensesvc.SetRuntimeValidationForTest("notification-test", publicKey)
	licenseKey, err := licensesvc.SignEd25519License(licensesvc.SignedLicense{
		LicenseMode: licensesvc.FormalLicenseMode,
		MachineID:   "notification-test",
		DeviceCount: 10,
		Features:    []string{"telegram"},
		IssuedAt:    time.Now().UTC().Format(time.RFC3339),
		ValidUntil:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}, privateKey)
	if err != nil {
		t.Fatalf("sign test license: %v", err)
	}
	mustExec(t, db, `INSERT INTO licenses (license_key, license_type, enabled_features, valid_until, is_active) VALUES (?, 'standard', '["telegram"]', ?, 1)`, licenseKey, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	mustExec(t, db, `INSERT INTO system_config (config_key, config_value) VALUES ('device_management_enabled', '1')`)

	svc := NewService(db, &config.Config{})
	type result struct {
		settings []AlertSettingView
		err      error
	}
	done := make(chan result, 1)
	go func() {
		settings, err := svc.GetAlertSettings()
		done <- result{settings: settings, err: err}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("GetAlertSettings returned error: %v", got.err)
		}
		if len(got.settings) == 0 {
			t.Fatal("expected default alert settings")
		}
		var foundTelegram bool
		for _, setting := range got.settings {
			if setting.AlertType == "telegram" {
				foundTelegram = true
				if !setting.HasLicense {
					t.Fatal("telegram should be licensed from active license features")
				}
			}
		}
		if !foundTelegram {
			t.Fatal("expected telegram alert setting")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetAlertSettings blocked with a single SQLite connection")
	}
}

func TestUpdateWorkflowPersistsAssigneeAndResolution(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	svc := NewService(db, &config.Config{})
	if err := svc.EnsureNotificationsTable(); err != nil {
		t.Fatalf("ensure notifications table: %v", err)
	}
	mustExec(t, db, `INSERT INTO notifications (severity, title, message) VALUES ('warning', 'Link down', 'Core switch uplink')`)
	if err := svc.UpdateWorkflow(1, UpdateWorkflowInput{Status: "acknowledged", AssignedTo: "operator"}, "operator"); err != nil {
		t.Fatalf("acknowledge workflow: %v", err)
	}
	if err := svc.UpdateWorkflow(1, UpdateWorkflowInput{Status: "resolved", AssignedTo: "operator", ResolutionNote: "Cable reseated"}, "operator"); err != nil {
		t.Fatalf("resolve workflow: %v", err)
	}
	items, _, err := svc.GetNotifications(false)
	if err != nil || len(items) != 1 {
		t.Fatalf("get notifications: items=%d err=%v", len(items), err)
	}
	if items[0].Status != "resolved" || items[0].AssignedTo != "operator" || items[0].ResolutionNote != "Cable reseated" || items[0].ResolvedAt == "" {
		t.Fatalf("unexpected workflow state: %+v", items[0])
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
