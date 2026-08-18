package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDatabasePathFromWorkingDirectory(t *testing.T) {
	temp := t.TempDir()
	dataDir := filepath.Join(temp, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(dataDir, "nms.db")
	if err := os.WriteFile(expected, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	resolved, err := resolveDatabasePath("")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != expected {
		t.Fatalf("resolved %q, want %q", resolved, expected)
	}
}
