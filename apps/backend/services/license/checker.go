package license

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"management-server/config"
	"strconv"
	"strings"
)

// GetMaxDevices calculates the total allowed devices based on licenses
func GetMaxDevices(db *sql.DB, cfg *config.Config) int {
	// 1. Get default limit
	defaultLimit := 0
	var configValue string
	if err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'default_device_limit'").Scan(&configValue); err == nil {
		if val, err := parseIntFromString(configValue); err == nil {
			defaultLimit = val
		}
	}

	// 2. Get licensed count
	var licensedDevices int
	db.QueryRow(`
		SELECT COALESCE(SUM(device_count), 0) FROM licenses 
		WHERE is_active = 1 AND (valid_until IS NULL OR valid_until = '' OR CASE WHEN length(valid_until) <= 10 THEN datetime(valid_until || ' 23:59:59') ELSE datetime(valid_until) END >= datetime('now'))
	`).Scan(&licensedDevices)

	return defaultLimit + licensedDevices
}

// CheckCompliance checks if current usage exceeds limits and alerts if necessary
func CheckCompliance(db *sql.DB, cfg *config.Config, onViolation func(string)) {
	maxDevices := GetMaxDevices(db, cfg)

	var currentCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&currentCount); err != nil {
		log.Printf("[License] Failed to count devices: %v", err)
		return
	}

	if currentCount > maxDevices {
		msg := fmt.Sprintf("授權違規警告：系統內設備數量 (%d) 已超過授權數量 (%d)！超出的設備將無法被正常顯示", currentCount, maxDevices)
		log.Println(msg)

		// Create system event
		db.Exec(`
			INSERT INTO events (event_type, severity, message, created_at) 
			VALUES ('license_violation', 'alert', ?, datetime('now'))
		`, msg)

		// Dispatch alert through callback
		if onViolation != nil {
			onViolation(msg)
		}
	}
}

// IsFeatureEnabled checks if a specific feature is allowed by the license
func IsFeatureEnabled(db *sql.DB, cfg *config.Config, feature string) bool {
	// Email is always allowed
	if feature == "email" {
		return true
	}

	// Check active licenses
	rows, err := db.Query(`
		SELECT enabled_features FROM licenses 
		WHERE is_active = 1 AND (valid_until IS NULL OR valid_until = '' OR CASE WHEN length(valid_until) <= 10 THEN datetime(valid_until || ' 23:59:59') ELSE datetime(valid_until) END >= datetime('now'))
	`)
	if err != nil {
		log.Printf("[License] Failed to query licenses: %v", err)
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var featuresJSON string
		if err := rows.Scan(&featuresJSON); err != nil {
			continue
		}
		var features []string
		if err := json.Unmarshal([]byte(featuresJSON), &features); err != nil {
			continue
		}
		for _, f := range features {
			if f == feature {
				return true
			}
		}
	}

	return false
}

func parseIntFromString(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}
