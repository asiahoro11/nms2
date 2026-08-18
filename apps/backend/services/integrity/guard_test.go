// Made by YTSworks
// YTS工作室製作
package integrity

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openIntegrityTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nms.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db, path
}

func TestPreflightPersistsExecutableMismatchBeforeDatabaseOpen(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "nms.db")
	executable := filepath.Join(t.TempDir(), "server.bin")
	if err := os.WriteFile(executable, []byte("modified executable"), 0600); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	sum := sha256.Sum256([]byte("trusted executable"))
	locked, reason, err := Preflight(Options{
		DatabasePath:             databasePath,
		ExecutablePath:           executable,
		ExpectedExecutableSHA256: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if !locked || reason != "executable_integrity_mismatch" {
		t.Fatalf("unexpected preflight result: locked=%v reason=%q", locked, reason)
	}
	if _, err := os.Stat(MarkerPath(databasePath)); err != nil {
		t.Fatalf("lock marker missing: %v", err)
	}
}

func TestPreflightLocksInvalidDatabaseHeader(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "nms.db")
	if err := os.WriteFile(databasePath, []byte("not a sqlite database"), 0600); err != nil {
		t.Fatalf("write invalid database: %v", err)
	}
	locked, reason, err := Preflight(Options{DatabasePath: databasePath})
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if !locked || reason != "database_header_invalid" {
		t.Fatalf("unexpected preflight result: locked=%v reason=%q", locked, reason)
	}
}

func TestGuardLeavesHealthyDatabaseWritable(t *testing.T) {
	db, path := openIntegrityTestDB(t)
	guard := New(db, Options{DatabasePath: path})
	if err := guard.Check(); err != nil {
		t.Fatalf("check: %v", err)
	}
	if guard.Locked() {
		t.Fatalf("healthy database was locked: %+v", guard.Status())
	}
	if _, err := db.Exec(`INSERT INTO sample (value) VALUES ('ok')`); err != nil {
		t.Fatalf("healthy database is not writable: %v", err)
	}
}

func TestGuardLocksOnExecutableHashMismatch(t *testing.T) {
	db, path := openIntegrityTestDB(t)
	executable := filepath.Join(t.TempDir(), "server.bin")
	if err := os.WriteFile(executable, []byte("trusted executable"), 0600); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	sum := sha256.Sum256([]byte("different executable"))
	guard := New(db, Options{
		DatabasePath:             path,
		ExecutablePath:           executable,
		ExpectedExecutableSHA256: hex.EncodeToString(sum[:]),
	})
	if err := guard.Check(); err != nil {
		t.Fatalf("check: %v", err)
	}
	if !guard.Locked() || guard.Status().Reason != "executable_integrity_mismatch" {
		t.Fatalf("unexpected status: %+v", guard.Status())
	}
	if _, err := db.Exec(`INSERT INTO sample (value) VALUES ('blocked')`); err == nil {
		t.Fatal("database write succeeded after integrity lock")
	}
	if _, err := os.Stat(MarkerPath(path)); err != nil {
		t.Fatalf("lock marker missing: %v", err)
	}
}

func TestGuardRestoresPersistentLockFromMarker(t *testing.T) {
	db, path := openIntegrityTestDB(t)
	marker := []byte(`{"reason":"database_integrity_failure","locked_at":"2026-07-24T00:00:00Z"}`)
	if err := os.WriteFile(MarkerPath(path), marker, 0600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	guard := New(db, Options{DatabasePath: path})
	if err := guard.Check(); err != nil {
		t.Fatalf("check: %v", err)
	}
	if !guard.Locked() || guard.Status().Reason != "database_integrity_failure" {
		t.Fatalf("unexpected status: %+v", guard.Status())
	}
}
