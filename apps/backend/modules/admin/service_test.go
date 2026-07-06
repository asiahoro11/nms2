// Made by YTSworks
// YTS工作室製作
package admin

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func newAdminTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`CREATE TABLE system_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		config_key TEXT UNIQUE NOT NULL,
		config_value TEXT NOT NULL,
		description TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return NewService(db), db
}

func TestBrandingSettingsUseDefaultCompanyNameOnlyWhenUnset(t *testing.T) {
	svc, _ := newAdminTestService(t)

	settings, err := svc.GetBrandingSettings()
	if err != nil {
		t.Fatalf("get branding settings: %v", err)
	}

	if settings.CompanyName != "Management Server" {
		t.Fatalf("expected unset company name to use default, got %q", settings.CompanyName)
	}
}

func TestBrandingSettingsPreserveBlankCompanyName(t *testing.T) {
	svc, _ := newAdminTestService(t)

	settings, err := svc.UpdateBrandingSettings(UpdateBrandingInput{
		CompanyName:  "",
		FontSize:     "18",
		FontColor:    "#ffffff",
		NamePosition: "right",
	})
	if err != nil {
		t.Fatalf("update branding settings: %v", err)
	}

	if settings.CompanyName != "" {
		t.Fatalf("expected blank company name to be preserved, got %q", settings.CompanyName)
	}
}

func TestDeleteBrandingLogoClearsLogoPath(t *testing.T) {
	svc, db := newAdminTestService(t)
	if _, err := db.Exec(`INSERT INTO system_config (config_key, config_value) VALUES ('company_logo_path', '/uploads/company_logo.png')`); err != nil {
		t.Fatalf("insert logo path: %v", err)
	}

	if err := svc.DeleteBrandingLogo(); err != nil {
		t.Fatalf("delete branding logo: %v", err)
	}

	settings, err := svc.GetBrandingSettings()
	if err != nil {
		t.Fatalf("get branding settings: %v", err)
	}
	if settings.LogoPath != "" {
		t.Fatalf("expected deleted logo path to be blank, got %q", settings.LogoPath)
	}
}
