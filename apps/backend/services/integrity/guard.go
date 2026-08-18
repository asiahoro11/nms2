// Made by YTSworks
// YTS工作室製作
package integrity

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const sqliteHeader = "SQLite format 3\x00"

type Options struct {
	DatabasePath             string
	ExpectedExecutableSHA256 string
	ExecutablePath           string
	CheckInterval            time.Duration
}

type Status struct {
	Locked                    bool   `json:"locked"`
	Reason                    string `json:"reason,omitempty"`
	LockedAt                  string `json:"locked_at,omitempty"`
	LastCheckAt               string `json:"last_check_at,omitempty"`
	ExecutableCheckConfigured bool   `json:"executable_check_configured"`
}

type lockMarker struct {
	Reason   string `json:"reason"`
	LockedAt string `json:"locked_at"`
}

type Guard struct {
	db      *sql.DB
	options Options

	mu          sync.RWMutex
	locked      bool
	reason      string
	lockedAt    time.Time
	lastCheckAt time.Time
	stopOnce    sync.Once
	stop        chan struct{}
}

func New(db *sql.DB, options Options) *Guard {
	if options.CheckInterval <= 0 {
		options.CheckInterval = 5 * time.Minute
	}
	options.ExpectedExecutableSHA256 = strings.ToLower(strings.TrimSpace(options.ExpectedExecutableSHA256))
	return &Guard{
		db:      db,
		options: options,
		stop:    make(chan struct{}),
	}
}

func MarkerPath(databasePath string) string {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" || databasePath == ":memory:" {
		return ""
	}
	return filepath.Clean(databasePath) + ".integrity-lock.json"
}

func MarkerExists(databasePath string) bool {
	path := MarkerPath(databasePath)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Preflight checks integrity signals that do not require an open database.
// It runs before schema migrations and background writers are allowed to start.
func Preflight(options Options) (locked bool, reason string, err error) {
	guard := New(nil, options)
	if markerPath := MarkerPath(options.DatabasePath); markerPath != "" {
		if data, readErr := os.ReadFile(markerPath); readErr == nil {
			var marker lockMarker
			reason = "persistent_integrity_lock"
			if json.Unmarshal(data, &marker) == nil && strings.TrimSpace(marker.Reason) != "" {
				reason = marker.Reason
			}
			return true, reason, nil
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return false, "", fmt.Errorf("read integrity lock marker: %w", readErr)
		}
	}

	reason, err = guard.checkExecutable()
	if err != nil {
		return false, reason, err
	}
	if reason == "" {
		reason, err = checkSQLiteHeader(options.DatabasePath)
		if err != nil {
			return false, "", err
		}
	}
	if reason == "" {
		return false, "", nil
	}
	if err := guard.writeMarker(reason, time.Now()); err != nil {
		return false, "", fmt.Errorf("persist preflight integrity lock: %w", err)
	}
	return true, reason, nil
}

func (g *Guard) Check() error {
	if g == nil {
		return nil
	}
	g.recordCheck()
	if g.Locked() {
		return nil
	}

	if markerPath := MarkerPath(g.options.DatabasePath); markerPath != "" {
		if data, err := os.ReadFile(markerPath); err == nil {
			var marker lockMarker
			reason := "persistent_integrity_lock"
			if json.Unmarshal(data, &marker) == nil && strings.TrimSpace(marker.Reason) != "" {
				reason = marker.Reason
			}
			return g.activateLock(reason, false)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read integrity lock marker: %w", err)
		}
	}

	if reason, err := g.checkExecutable(); err != nil {
		return err
	} else if reason != "" {
		return g.activateLock(reason, true)
	}

	if reason, err := g.checkDatabase(); err != nil {
		return err
	} else if reason != "" {
		return g.activateLock(reason, true)
	}
	return nil
}

func (g *Guard) Start() {
	if g == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(g.options.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := g.Check(); err != nil {
					log.Printf("[Integrity] check deferred: %v", err)
				}
			case <-g.stop:
				return
			}
		}
	}()
}

func (g *Guard) Stop() {
	if g == nil {
		return
	}
	g.stopOnce.Do(func() { close(g.stop) })
}

func (g *Guard) Locked() bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.locked
}

func (g *Guard) Status() Status {
	if g == nil {
		return Status{}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	status := Status{
		Locked:                    g.locked,
		Reason:                    g.reason,
		ExecutableCheckConfigured: g.options.ExpectedExecutableSHA256 != "",
	}
	if !g.lockedAt.IsZero() {
		status.LockedAt = g.lockedAt.UTC().Format(time.RFC3339)
	}
	if !g.lastCheckAt.IsZero() {
		status.LastCheckAt = g.lastCheckAt.UTC().Format(time.RFC3339)
	}
	return status
}

func (g *Guard) recordCheck() {
	g.mu.Lock()
	g.lastCheckAt = time.Now()
	g.mu.Unlock()
}

func (g *Guard) checkExecutable() (string, error) {
	expected := g.options.ExpectedExecutableSHA256
	if expected == "" {
		return "", nil
	}
	if len(expected) != sha256.Size*2 {
		return "invalid_executable_integrity_configuration", nil
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return "invalid_executable_integrity_configuration", nil
	}

	executablePath := strings.TrimSpace(g.options.ExecutablePath)
	if executablePath == "" {
		var err error
		executablePath, err = os.Executable()
		if err != nil {
			return "executable_integrity_unverifiable", nil
		}
	}
	file, err := os.Open(executablePath)
	if err != nil {
		return "executable_integrity_unverifiable", nil
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "executable_integrity_unverifiable", nil
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return "executable_integrity_mismatch", nil
	}
	return "", nil
}

func (g *Guard) checkDatabase() (string, error) {
	if g.db == nil {
		return "database_integrity_unverifiable", nil
	}
	if reason, err := checkSQLiteHeader(g.options.DatabasePath); err != nil {
		return "", err
	} else if reason != "" {
		return reason, nil
	}

	rows, err := g.db.Query(`PRAGMA quick_check`)
	if err != nil {
		if isCorruptionError(err) {
			return "database_integrity_failure", nil
		}
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return "", err
		}
		if !strings.EqualFold(strings.TrimSpace(result), "ok") {
			return "database_integrity_failure", nil
		}
	}
	if err := rows.Err(); err != nil {
		if isCorruptionError(err) {
			return "database_integrity_failure", nil
		}
		return "", err
	}
	return "", nil
}

func checkSQLiteHeader(databasePath string) (string, error) {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" || databasePath == ":memory:" {
		return "", nil
	}
	file, err := os.Open(databasePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, len(sqliteHeader))
	n, err := io.ReadFull(file, header)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		if n == 0 {
			return "", nil
		}
		return "database_header_invalid", nil
	}
	if err != nil {
		return "", err
	}
	if string(header) != sqliteHeader {
		return "database_header_invalid", nil
	}
	return "", nil
}

func isCorruptionError(err error) bool {
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{
		"database disk image is malformed",
		"file is not a database",
		"database corrupt",
		"database corruption",
		"sqlite_corrupt",
		"sqlite_notadb",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func (g *Guard) activateLock(reason string, persist bool) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "integrity_failure"
	}
	now := time.Now()

	g.mu.Lock()
	if !g.locked {
		g.locked = true
		g.reason = reason
		g.lockedAt = now
	}
	g.mu.Unlock()

	var problems []string
	if g.db != nil {
		if _, err := g.db.Exec(`PRAGMA query_only = ON`); err != nil {
			problems = append(problems, "enable database query-only mode: "+err.Error())
		}
	}
	if persist {
		if err := g.writeMarker(reason, now); err != nil {
			problems = append(problems, "persist integrity lock: "+err.Error())
		}
	}
	log.Printf("[Integrity] LOCKED: %s", reason)
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (g *Guard) writeMarker(reason string, lockedAt time.Time) error {
	path := MarkerPath(g.options.DatabasePath)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lockMarker{
		Reason:   reason,
		LockedAt: lockedAt.UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
