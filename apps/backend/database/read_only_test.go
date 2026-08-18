package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenReadOnlyAllowsReadsAndRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nms.db")
	writable, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open writable database: %v", err)
	}
	if _, err := writable.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		_ = writable.Close()
		t.Fatalf("create table: %v", err)
	}
	if _, err := writable.Exec(`INSERT INTO sample (value) VALUES ('preserved')`); err != nil {
		_ = writable.Close()
		t.Fatalf("insert fixture: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("close writable database: %v", err)
	}

	readOnly, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("open read-only database: %v", err)
	}
	defer readOnly.Close()

	var value string
	if err := readOnly.QueryRow(`SELECT value FROM sample WHERE id = 1`).Scan(&value); err != nil {
		t.Fatalf("read preserved data: %v", err)
	}
	if value != "preserved" {
		t.Fatalf("unexpected value: %q", value)
	}
	if _, err := readOnly.Exec(`INSERT INTO sample (value) VALUES ('blocked')`); err == nil {
		t.Fatal("write unexpectedly succeeded in read-only mode")
	}
}
