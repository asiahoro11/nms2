// Made by YTSworks
// YTS工作室製作
package alert

import (
	"database/sql"
	"log"
	"management-server/config"
	"management-server/services/license"
)

// CloudAlertHook is called when an alert is dispatched, used by the cloud edge connector.
var CloudAlertHook func(message string)

// DispatchToEnabledChannels sends a message to all enabled alert channels
func DispatchToEnabledChannels(db *sql.DB, cfg *config.Config, message string) {
	// periodic alert rule check
	var globalEnabled string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'alerts_global_enabled'").Scan(&globalEnabled)
	if err == nil && globalEnabled == "false" {
		log.Printf("[Alert] Dispatch skipped - Global Alerts are DISABLED")
		return
	}

	// Query configured alert settings
	rows, err := db.Query("SELECT alert_type, config_json, is_enabled FROM alert_settings")
	if err != nil {
		log.Printf("[Alert] Failed to fetch alert settings: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var alertType string
		var configJSON string
		var isEnabled bool

		if err := rows.Scan(&alertType, &configJSON, &isEnabled); err != nil {
			continue
		}

		// CRITICAL: Only send if enabled
		if !isEnabled {
			continue
		}

		// LICENSE CHECK: Ensure feature is allowed
		if !license.IsFeatureEnabled(db, cfg, alertType) {
			// Log once per startup/day ideally, but for now just skip
			// log.Printf("[Alert] Skipping %s alert - license restricted", alertType)
			continue
		}

		// Send alert asynchronously to prevent blocking
		go func(aType, cJSON, msg string) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Alert] Panic sending %s alert: %v", aType, r)
				}
			}()
			if err := SendAlert(aType, cJSON, msg); err != nil {
				log.Printf("[Alert] Failed to send %s alert: %v", aType, err)
			} else {
				log.Printf("[Alert] Sent notification via %s", aType)
			}
		}(alertType, configJSON, message)
	}

	// Notify cloud connector (if configured)
	if CloudAlertHook != nil {
		go CloudAlertHook(message)
	}
}
