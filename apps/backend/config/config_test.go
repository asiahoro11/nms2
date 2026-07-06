package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfig(dbPath string) *Config {
	return &Config{
		Database: DatabaseConfig{Path: dbPath},
	}
}

func TestEnsureJWTSecretGeneratesAndPersistsOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(filepath.Join(dir, "nms.db"))

	if err := ensureJWTSecret(cfg); err != nil {
		t.Fatalf("ensureJWTSecret: %v", err)
	}
	if cfg.Security.JWTSecret == "" || cfg.Security.JWTSecret == legacyDefaultJWTSecret {
		t.Fatalf("expected generated secret, got %q", cfg.Security.JWTSecret)
	}

	// A second load must reuse the persisted secret.
	cfg2 := testConfig(filepath.Join(dir, "nms.db"))
	if err := ensureJWTSecret(cfg2); err != nil {
		t.Fatalf("ensureJWTSecret (reload): %v", err)
	}
	if cfg2.Security.JWTSecret != cfg.Security.JWTSecret {
		t.Fatalf("secret not stable across loads: %q vs %q", cfg2.Security.JWTSecret, cfg.Security.JWTSecret)
	}
}

func TestEnsureJWTSecretRespectsExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(filepath.Join(dir, "nms.db"))
	cfg.Security.JWTSecret = "operator-provided-secret"

	if err := ensureJWTSecret(cfg); err != nil {
		t.Fatalf("ensureJWTSecret: %v", err)
	}
	if cfg.Security.JWTSecret != "operator-provided-secret" {
		t.Fatalf("explicit secret overwritten: %q", cfg.Security.JWTSecret)
	}
	if _, err := os.Stat(filepath.Join(dir, jwtSecretFilename)); !os.IsNotExist(err) {
		t.Fatalf("secret file should not be created when config provides one")
	}
}

func TestEnsureJWTSecretKeepsLegacyDefaultForExistingDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nms.db")
	if err := os.WriteFile(dbPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(dbPath)
	cfg.Security.JWTSecret = legacyDefaultJWTSecret

	if err := ensureJWTSecret(cfg); err != nil {
		t.Fatalf("ensureJWTSecret: %v", err)
	}
	// Existing data may be encrypted with the legacy secret, so it must be kept.
	if cfg.Security.JWTSecret != legacyDefaultJWTSecret {
		t.Fatalf("legacy secret must be preserved for existing installs, got %q", cfg.Security.JWTSecret)
	}
}

func TestEnsureJWTSecretTreatsLegacyDefaultAsUnsetOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(filepath.Join(dir, "nms.db"))
	cfg.Security.JWTSecret = legacyDefaultJWTSecret

	if err := ensureJWTSecret(cfg); err != nil {
		t.Fatalf("ensureJWTSecret: %v", err)
	}
	if cfg.Security.JWTSecret == legacyDefaultJWTSecret {
		t.Fatal("fresh install must not run with the publicly known default secret")
	}
}
