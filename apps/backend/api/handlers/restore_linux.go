//go:build linux

package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// createRestoreScript for Linux
func createRestoreScript(dbPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exeName := filepath.Base(exe) // e.g., nms_server

	if dbPath == "" {
		dbPath = "data/nms.db"
	}

	// exec based restore strategy.
	// We replace the current process with the shell script, keeping the PID and satisfying systemd.
	script := fmt.Sprintf(`#!/bin/sh
# Log all output
exec > /tmp/nms_restore.log 2>&1
echo "Restore started at $(date)"

# We are running AS the service process now (via exec).
# No need to stop server, we ARE the only thing running in this PID.

echo "Step 2: Restoring files..."
cd "$(dirname "$0")"

# Database Target Path: %s
DB_PATH="%s"
DB_DIR=$(dirname "$DB_PATH")

# Ensure DB directory exists
mkdir -p "$DB_DIR"

# Cleanup any previous stale files
rm -f "$DB_PATH-wal" "$DB_PATH-shm"

if [ -f data/nms.db.pending ]; then
    echo "Restoring Database..."
    mv -f data/nms.db.pending "$DB_PATH"
else
    echo "Error: data/nms.db.pending not found"
fi

# Restore WAL/SHM if they exist
if [ -f data/nms.db-wal.pending ]; then
    mv -f data/nms.db-wal.pending "$DB_PATH-wal"
fi
if [ -f data/nms.db-shm.pending ]; then
    mv -f data/nms.db-shm.pending "$DB_PATH-shm"
fi

# Force SQLite WAL checkpoint
if command -v sqlite3 > /dev/null 2>&1; then
    echo "Running SQLite WAL checkpoint..."
    if sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(TRUNCATE);" 2>&1 | tee -a /tmp/nms_checkpoint.log; then
        echo "✅ Database checkpoint completed successfully"
        rm -f "$DB_PATH-wal" "$DB_PATH-shm"
    else
        echo "❌ ERROR: WAL checkpoint FAILED!"
        rm -f "$DB_PATH-wal" "$DB_PATH-shm"
    fi
else
    echo "❌ WARNING: sqlite3 command not found"
    rm -f "$DB_PATH-wal" "$DB_PATH-shm"
fi

if [ -f config/config.yaml.pending ]; then
    echo "Restoring Config..."
    mv -f config/config.yaml.pending config.yaml
fi

if [ -d data/uploads_pending ]; then
    echo "Restoring Uploads..."
    mkdir -p data/uploads
    # Only clear and copy if we actually have pending content
    if [ "$(ls -A data/uploads_pending 2>/dev/null)" ]; then
        rm -rf data/uploads/*
        cp -rf data/uploads_pending/* data/uploads/
    fi
    rm -rf data/uploads_pending
fi

echo "Step 3: Reloading Server..."
# Replace shell process with the server binary
chmod +x "./%s"
echo "Execing ./nms_server..."
exec "./%s"
`, exeName, dbPath, dbPath, exeName, exeName)

	return os.WriteFile("restore.sh", []byte(script), 0755)
}

// executeRestoreScript for Linux
func executeRestoreScript() {
	// Determine absolute path
	exe, err := os.Executable()
	if err != nil {
		exe = "restore.sh"
	}
	dir := filepath.Dir(exe)
	scriptPath := filepath.Join(dir, "restore.sh")

	// Use syscall.Exec to replace the current process with the restore script.
	// This preserves the PID and prevents systemd from killing the process as "stopped".
	err = syscall.Exec("/bin/sh", []string{"sh", scriptPath}, os.Environ())
	if err != nil {
		// If exec fails, we log it and exit (systemd will likely restart us)
		fmt.Printf("Failed to exec restore script: %v\n", err)
		os.Exit(1)
	}
}
