// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"management-server/services/license"
)

type runtimeLicenseRow struct {
	ID          int
	LicenseKey  string
	LicenseType string
	DeviceCount int
	CameraCount int
	Features    []string
	ValidFrom   string
	ValidUntil  string
	IsActive    bool
}

var licenseRuntimeState = struct {
	mu            sync.Mutex
	running       bool
	lastRun       time.Time
	licenseStatus map[int]string
}{
	licenseStatus: map[int]string{},
}

func (h *Handler) StartLicenseHealthLoop() {
	h.ensureLicenseRuntimeFresh(0)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			h.ensureLicenseRuntimeFresh(0)
		}
	}()
}

func (h *Handler) ensureLicenseRuntimeFresh(maxAge time.Duration) {
	licenseRuntimeState.mu.Lock()
	if licenseRuntimeState.running {
		licenseRuntimeState.mu.Unlock()
		return
	}
	if maxAge > 0 && !licenseRuntimeState.lastRun.IsZero() && time.Since(licenseRuntimeState.lastRun) < maxAge {
		licenseRuntimeState.mu.Unlock()
		return
	}
	licenseRuntimeState.running = true
	licenseRuntimeState.mu.Unlock()

	defer func() {
		licenseRuntimeState.mu.Lock()
		licenseRuntimeState.running = false
		licenseRuntimeState.lastRun = time.Now()
		licenseRuntimeState.mu.Unlock()
	}()

	h.syncLicenseRuntimeState()
}

func (h *Handler) syncLicenseRuntimeState() {
	rows, err := h.db.Query(`
		SELECT id, license_key, license_type, COALESCE(device_count, 0), COALESCE(camera_count, 0),
		       COALESCE(enabled_features, '[]'), COALESCE(valid_from, ''), COALESCE(valid_until, ''), COALESCE(is_active, 0)
		FROM licenses
		ORDER BY id ASC
	`)
	if err != nil {
		log.Printf("[LicenseRuntime] failed to query licenses: %v", err)
		return
	}
	defer rows.Close()

	now := time.Now()
	activeFlags := map[string]bool{
		"camera_viewer_enabled":     false,
		"camera_recording_enabled":  false,
		"access_control_enabled":    false,
		"pdu_enabled":               false,
		"iot_enabled":               false,
		"device_management_enabled": false,
	}

	currentStatuses := map[int]string{}
	for rows.Next() {
		var row runtimeLicenseRow
		var featuresJSON string
		var isActiveInt int
		if scanErr := rows.Scan(
			&row.ID,
			&row.LicenseKey,
			&row.LicenseType,
			&row.DeviceCount,
			&row.CameraCount,
			&featuresJSON,
			&row.ValidFrom,
			&row.ValidUntil,
			&isActiveInt,
		); scanErr != nil {
			continue
		}
		row.IsActive = isActiveInt == 1
		_ = json.Unmarshal([]byte(featuresJSON), &row.Features)

		status := "inactive"
		expired := false
		isPermanent := license.IsPermanentValidity(row.ValidFrom, row.ValidUntil)
		if row.IsActive {
			status = "active"
		}
		if !isPermanent && strings.TrimSpace(row.ValidUntil) != "" {
			if expiry, parseErr := license.ParseLicenseTime(row.ValidUntil); parseErr == nil && now.After(expiry) {
				expired = true
				status = "expired"
			}
		}

		if expired && row.IsActive {
			_, _ = h.db.Exec("UPDATE licenses SET is_active = 0 WHERE id = ?", row.ID)
			row.IsActive = false
		}

		currentStatuses[row.ID] = status
		h.logLicenseStatusTransition(row, status, expired, isPermanent)

		if row.IsActive && status != "expired" {
			if row.DeviceCount > 0 || hasFeature(row.Features, "device_management") {
				activeFlags["device_management_enabled"] = true
			}
			if row.CameraCount > 0 || hasFeature(row.Features, "camera_viewer") {
				activeFlags["camera_viewer_enabled"] = true
				activeFlags["camera_recording_enabled"] = true
			}
			if hasFeature(row.Features, "camera_recording") {
				activeFlags["camera_recording_enabled"] = true
			}
			if hasFeature(row.Features, "access_control") {
				activeFlags["access_control_enabled"] = true
			}
			if hasFeature(row.Features, "pdu") {
				activeFlags["pdu_enabled"] = true
			}
			if hasFeature(row.Features, "iot") {
				activeFlags["iot_enabled"] = true
			}
		}
	}

	licenseRuntimeState.mu.Lock()
	licenseRuntimeState.licenseStatus = currentStatuses
	licenseRuntimeState.mu.Unlock()

	h.reconcileLicenseModuleFlag("device_management_enabled", "device_management", activeFlags["device_management_enabled"])
	h.reconcileLicenseModuleFlag("camera_viewer_enabled", "camera_viewer", activeFlags["camera_viewer_enabled"])
	h.reconcileLicenseModuleFlag("camera_recording_enabled", "camera_recording", activeFlags["camera_recording_enabled"])
	h.reconcileLicenseModuleFlag("access_control_enabled", "access_control", activeFlags["access_control_enabled"])
	h.reconcileLicenseModuleFlag("pdu_enabled", "pdu", activeFlags["pdu_enabled"])
	h.reconcileLicenseModuleFlag("iot_enabled", "iot", activeFlags["iot_enabled"])
}

func (h *Handler) logLicenseStatusTransition(row runtimeLicenseRow, status string, expired bool, isPermanent bool) {
	licenseRuntimeState.mu.Lock()
	previous := licenseRuntimeState.licenseStatus[row.ID]
	licenseRuntimeState.mu.Unlock()

	if previous == status {
		return
	}

	if status == "expired" {
		context := map[string]interface{}{
			"license_id":         row.ID,
			"license_key_prefix": truncateAuditLicenseKey(row.LicenseKey),
			"license_type":       row.LicenseType,
			"device_count":       row.DeviceCount,
			"camera_count":       row.CameraCount,
			"features":           row.Features,
			"valid_until":        row.ValidUntil,
			"is_permanent":       isPermanent,
			"reason":             "license_expired",
		}
		h.WriteSystemLog("warning", "license", "license_expired", "license expired and was deactivated", context)
		h.WriteAudit("system", "127.0.0.1", "", "license_expired", auditResource("license", row.ID), "success", auditDetails(context,
			"module", "license",
			"resource_type", "license",
			"resource_id", row.ID,
			"resource_name", truncateAuditLicenseKey(row.LicenseKey),
			"change_source", "system",
			"change_scope", "license_expiry",
		))
		return
	}

	if previous == "expired" && status == "active" {
		h.WriteSystemLog("notice", "license", "license_revalidated", "license became active again", map[string]interface{}{
			"license_id":         row.ID,
			"license_key_prefix": truncateAuditLicenseKey(row.LicenseKey),
			"license_type":       row.LicenseType,
			"device_count":       row.DeviceCount,
			"camera_count":       row.CameraCount,
			"features":           row.Features,
			"valid_until":        row.ValidUntil,
			"is_permanent":       isPermanent,
		})
	}
}

func (h *Handler) reconcileLicenseModuleFlag(configKey, featureName string, shouldRemainEnabled bool) {
	var currentValue string
	err := h.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = ?", configKey).Scan(&currentValue)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	currentEnabled := strings.TrimSpace(currentValue) == "1"
	if !currentEnabled || shouldRemainEnabled {
		return
	}

	_, _ = h.db.Exec(`
		INSERT OR REPLACE INTO system_config (config_key, config_value, description)
		VALUES (?, '0', ?)
	`, configKey, "Auto-locked by license runtime sync")

	context := map[string]interface{}{
		"config_key":     configKey,
		"feature":        featureName,
		"reason":         "license_unavailable",
		"change_source":  "system",
		"change_scope":   "license_module_lock",
		"resource_type":  "license_module",
		"resource_id":    configKey,
		"resource_name":  featureName,
		"old_values":     map[string]interface{}{"enabled": "1"},
		"new_values":     map[string]interface{}{"enabled": "0"},
		"config_change":  true,
		"lock_enforced":  true,
		"module_feature": featureName,
	}

	h.WriteSystemLog("warning", "license", "license_module_locked", "license-gated module locked", context)
	h.WriteAudit("system", "127.0.0.1", "", "license_module_locked", auditResource("license-module", configKey), "success", context)
}
