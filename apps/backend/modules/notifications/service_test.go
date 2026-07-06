package notifications

import (
	"database/sql"
	"testing"
	"time"

	"management-server/config"

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
		enabled_features TEXT,
		valid_until TEXT,
		is_active BOOLEAN DEFAULT 1
	)`)
	mustExec(t, db, `CREATE TABLE system_config (
		config_key TEXT PRIMARY KEY,
		config_value TEXT
	)`)
	mustExec(t, db, `INSERT INTO licenses (enabled_features, valid_until, is_active) VALUES ('["telegram"]', '', 1)`)
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

func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
