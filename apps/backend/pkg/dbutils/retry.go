package dbutils

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// ExecWithRetry executes a statement with retry logic for SQLITE_BUSY
func ExecWithRetry(db *sql.DB, query string, args ...interface{}) (sql.Result, error) {
	var res sql.Result
	var err error
	maxRetries := 5 // Increased retries for better stability

	for i := 0; i < maxRetries; i++ {
		res, err = db.Exec(query, args...)
		if err == nil {
			return res, nil
		}

		if isLockedError(err) {
			// Exponential backoff: 50ms, 100ms, 200ms, 400ms, 800ms
			// Added jitter implicitly by variable execution time, but explicit block is fine
			sleepDuration := time.Duration(50*(1<<i)) * time.Millisecond
			if i > 2 {
				log.Printf("[DB] Locked, retrying in %v... (%d/%d)", sleepDuration, i+1, maxRetries)
			}
			time.Sleep(sleepDuration)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("failed after %d retries: %v", maxRetries, err)
}

// TxWithRetry executes a function within a transaction with retry logic
func TxWithRetry(db *sql.DB, fn func(tx *sql.Tx) error) error {
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		tx, err := db.Begin()
		if err != nil {
			if isLockedError(err) {
				time.Sleep(time.Duration(50*(1<<i)) * time.Millisecond)
				continue
			}
			return err
		}

		if err := fn(tx); err != nil {
			tx.Rollback()
			if isLockedError(err) {
				log.Printf("[DB] Transaction locked, rolling back and retrying... (%d/%d)", i+1, maxRetries)
				time.Sleep(time.Duration(50*(1<<i)) * time.Millisecond)
				continue
			}
			return err
		}

		if err := tx.Commit(); err != nil {
			if isLockedError(err) {
				log.Printf("[DB] Commit locked, retrying transaction... (%d/%d)", i+1, maxRetries)
				time.Sleep(time.Duration(50*(1<<i)) * time.Millisecond)
				continue
			}
			return err
		}

		return nil
	}

	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func isLockedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "SQLITE_BUSY")
}
