// Made by YTSworks
// YTS工作室製作
package handlers

// stubs.go — 路由已宣告但尚未完整實作的 handler stub
// 已恢復的功能請直接提供實作，避免前端打到 501。

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	backupmodule "management-server/modules/backup"

	"github.com/gin-gonic/gin"
)

// Device management stubs
func (h *Handler) RebootDevice(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.WriteAuditFromContext(c, "reboot_device", auditResource("device", c.Param("id")), "failed", map[string]interface{}{
			"reason": "invalid_device_id",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	h.WriteDeviceLog(deviceID, "warning", "device-control", "device reboot requested but not implemented", "Device reboot requested but not implemented", map[string]interface{}{
		"reason": "not_implemented",
	})
	h.WriteAuditFromContext(c, "reboot_device", auditResource("device", deviceID), "failed", auditDetails(
		h.deviceAuditDetails(deviceID),
		"reason", "not_implemented",
	))
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}

func (h *Handler) SaveDeviceConfig(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	if err := h.backup.SaveDeviceConfig(deviceID); err != nil {
		status := http.StatusInternalServerError
		if backupmodule.Is(err, backupmodule.ErrBackupCollectorUnavailable) {
			status = http.StatusServiceUnavailable
		}
		h.WriteDeviceLog(deviceID, "error", "config-backup", "device config backup failed", "Device config backup failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAuditFromContext(c, "save_device_config", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"reason", "save_config_failed",
			"error", err.Error(),
		))
		c.JSON(status, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteDeviceLog(deviceID, "notice", "config-backup", "device config backup stored", "Device config backup stored", nil)
	h.WriteAuditFromContext(c, "save_device_config", auditResource("device", deviceID), "success", auditDetails(
		h.deviceAuditDetails(deviceID),
	))
	c.JSON(http.StatusOK, Response{Success: true, Message: "device config backup saved"})
}

func (h *Handler) GetDeviceConfigBackups(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	backups, err := h.backup.ListDeviceConfigBackups(deviceID)
	if err != nil {
		h.WriteDeviceLog(deviceID, "error", "config-backup", "device config backup list failed", "Device config backup list failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAuditFromContext(c, "list_device_config_backups", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"reason", "list_backups_failed",
			"error", err.Error(),
		))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "list_device_config_backups", auditResource("device", deviceID), "success", auditDetails(
		h.deviceAuditDetails(deviceID),
		"backup_count", len(backups),
	))
	c.JSON(http.StatusOK, Response{Success: true, Data: backups})
}

func (h *Handler) DownloadDeviceConfigBackup(c *gin.Context) {
	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}
	backupID, err := strconv.Atoi(c.Param("backupId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid backup id"})
		return
	}

	backup, err := h.backup.GetDeviceConfigBackup(deviceID, backupID)
	if err != nil {
		status := http.StatusInternalServerError
		if backupmodule.Is(err, backupmodule.ErrDeviceConfigBackupNotFound) {
			status = http.StatusNotFound
		}
		h.WriteDeviceLog(deviceID, "error", "config-backup", "device config backup download failed", "Device config backup download failed", map[string]interface{}{
			"backup_id": backupID,
			"error":     err.Error(),
		})
		h.WriteAuditFromContext(c, "download_device_config_backup", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"backup_id", backupID,
			"reason", "download_backup_failed",
			"error", err.Error(),
		))
		c.JSON(status, Response{Success: false, Error: err.Error()})
		return
	}

	filename := fmt.Sprintf("device_%d_backup_%d.txt", deviceID, backupID)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(backup.Content))

	h.WriteAuditFromContext(c, "download_device_config_backup", auditResource("device", deviceID), "success", auditDetails(
		h.deviceAuditDetails(deviceID),
		"backup_id", backupID,
	))
}

func (h *Handler) GetDevicePoE(c *gin.Context) {
	if h.snmpCollector == nil {
		c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "snmp collector unavailable"})
		return
	}

	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	ports, err := h.snmpCollector.GetDevicePortDetails(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: ports})
}

func (h *Handler) GetDeviceApStatus(c *gin.Context) {
	if h.snmpCollector == nil {
		c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "snmp collector unavailable"})
		return
	}

	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	status, err := h.snmpCollector.GetApStatus(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) ControlPoEPort(c *gin.Context) {
	if h.snmpCollector == nil {
		if deviceID, err := strconv.Atoi(c.Param("id")); err == nil {
			h.WriteDeviceLog(deviceID, "error", "poe", "poe control failed", "PoE control failed", map[string]interface{}{
				"reason": "snmp_collector_unavailable",
			})
		}
		h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", c.Param("id")), "failed", map[string]interface{}{
			"reason": "snmp_collector_unavailable",
		})
		c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "snmp collector unavailable"})
		return
	}

	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", c.Param("id")), "failed", map[string]interface{}{
			"reason": "invalid_device_id",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	portIndex, err := strconv.Atoi(c.Param("portIndex"))
	if err != nil {
		h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"reason", "invalid_port_index",
			"error", err.Error(),
		))
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid port index"})
		return
	}

	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"port_index", portIndex,
			"reason", "invalid_request",
			"error", err.Error(),
		))
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "unknown"
	}

	if err := h.snmpCollector.ControlPoEPort(deviceID, portIndex, req.Action); err != nil {
		h.WriteDeviceLog(deviceID, "error", "poe", "poe control failed", "PoE control failed", map[string]interface{}{
			"port_index": portIndex,
			"poe_action": action,
			"reason":     "snmp_command_failed",
			"error":      err.Error(),
		})
		h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"port_index", portIndex,
			"poe_action", action,
			"reason", "snmp_command_failed",
			"error", err.Error(),
		))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteDeviceLog(deviceID, "notice", "poe", "poe control success", "PoE control success", map[string]interface{}{
		"port_index": portIndex,
		"poe_action": action,
	})
	h.WriteAuditFromContext(c, "control_poe_port", auditResource("device", deviceID), "success", auditDetails(
		h.deviceAuditDetails(deviceID),
		"port_index", portIndex,
		"poe_action", action,
	))
	c.JSON(http.StatusOK, Response{Success: true, Message: "PoE command sent"})
}

func (h *Handler) ControlPortStatus(c *gin.Context) {
	if h.snmpCollector == nil {
		if deviceID, err := strconv.Atoi(c.Param("id")); err == nil {
			h.WriteDeviceLog(deviceID, "error", "port-control", "port status control failed", "Port status control failed", map[string]interface{}{
				"reason": "snmp_collector_unavailable",
			})
		}
		h.WriteAuditFromContext(c, "control_port_status", auditResource("device", c.Param("id")), "failed", map[string]interface{}{
			"reason": "snmp_collector_unavailable",
		})
		c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "snmp collector unavailable"})
		return
	}

	deviceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.WriteAuditFromContext(c, "control_port_status", auditResource("device", c.Param("id")), "failed", map[string]interface{}{
			"reason": "invalid_device_id",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid device id"})
		return
	}

	portIndex, err := strconv.Atoi(c.Param("ifIndex"))
	if err != nil {
		h.WriteAuditFromContext(c, "control_port_status", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"reason", "invalid_interface_index",
			"error", err.Error(),
		))
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid interface index"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.WriteAuditFromContext(c, "control_port_status", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"port_index", portIndex,
			"reason", "invalid_request",
			"error", err.Error(),
		))
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if err := h.snmpCollector.ControlPortStatus(deviceID, portIndex, req.Status); err != nil {
		h.WriteDeviceLog(deviceID, "error", "port-control", "port status control failed", "Port status control failed", map[string]interface{}{
			"port_index":  portIndex,
			"port_status": strings.ToLower(strings.TrimSpace(req.Status)),
			"reason":      "snmp_command_failed",
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "control_port_status", auditResource("device", deviceID), "failed", auditDetails(
			h.deviceAuditDetails(deviceID),
			"port_index", portIndex,
			"port_status", strings.ToLower(strings.TrimSpace(req.Status)),
			"reason", "snmp_command_failed",
			"error", err.Error(),
		))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteDeviceLog(deviceID, "notice", "port-control", "port status control success", "Port status control success", map[string]interface{}{
		"port_index":  portIndex,
		"port_status": strings.ToLower(strings.TrimSpace(req.Status)),
	})
	h.WriteAuditFromContext(c, "control_port_status", auditResource("device", deviceID), "success", auditDetails(
		h.deviceAuditDetails(deviceID),
		"port_index", portIndex,
		"port_status", strings.ToLower(strings.TrimSpace(req.Status)),
	))
	c.JSON(http.StatusOK, Response{Success: true, Message: "Port status command sent"})
}

// Alert stubs
func (h *Handler) SaveAlertSettings(c *gin.Context) {
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}

// Branding stubs
func (h *Handler) legacyDeleteBrandingLogo(c *gin.Context) {
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}

// Security/Config stubs
func (h *Handler) legacyGetSecuritySettings(c *gin.Context) {
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}

func (h *Handler) legacyUpdateSecuritySettings(c *gin.Context) {
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}

func (h *Handler) legacyUpdateSystemConfig(c *gin.Context) {
	key := strings.TrimSpace(c.Param("key"))
	if key == "" {
		h.WriteSystemLog("warning", "system-config", "update_system_config_failed", "system config update rejected", map[string]interface{}{
			"reason": "missing_config_key",
		})
		h.WriteAuditFromContext(c, "update_system_config", "system_config", "failed", map[string]interface{}{
			"module": "system",
			"reason": "missing_config_key",
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "missing config key"})
		return
	}

	var req struct {
		Value       interface{} `json:"value"`
		ConfigValue interface{} `json:"config_value"`
		Description string      `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.WriteSystemLog("warning", "system-config", "update_system_config_failed", "system config update rejected", map[string]interface{}{
			"config_key": key,
			"reason":     "invalid_request",
			"error":      err.Error(),
		})
		h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "failed", map[string]interface{}{
			"module": "system",
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	newValue := strings.TrimSpace(fmt.Sprint(req.Value))
	if newValue == "<nil>" || newValue == "" {
		newValue = strings.TrimSpace(fmt.Sprint(req.ConfigValue))
	}
	if newValue == "<nil>" {
		newValue = ""
	}

	var oldValue, oldDescription string
	var existed bool
	if err := h.db.QueryRow("SELECT config_value, COALESCE(description, '') FROM system_config WHERE config_key = ?", key).Scan(&oldValue, &oldDescription); err == nil {
		existed = true
	}

	description := req.Description
	if description == "" {
		description = oldDescription
	}

	_, err := h.db.Exec(`
		INSERT INTO system_config (config_key, config_value, description)
		VALUES (?, ?, ?)
		ON CONFLICT(config_key) DO UPDATE SET config_value = excluded.config_value, description = excluded.description
	`, key, newValue, description)
	if err != nil {
		h.WriteSystemLog("error", "system-config", "update_system_config_failed", "system config update failed", map[string]interface{}{
			"config_key": key,
			"error":      err.Error(),
		})
		h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "failed", map[string]interface{}{
			"module":     "system",
			"reason":     "db_upsert_failed",
			"error":      err.Error(),
			"config_key": key,
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "success", map[string]interface{}{
		"module":        "system",
		"resource_type": "system_config",
		"resource_id":   key,
		"resource_name": key,
		"config_key":    key,
		"old_values": map[string]interface{}{
			"config_value": oldValue,
			"description":  oldDescription,
			"existed":      existed,
		},
		"new_values": map[string]interface{}{
			"config_value": newValue,
			"description":  description,
		},
		"config_change": true,
		"change_scope":  "system_config",
	})

	h.WriteSystemLog("notice", "system-config", "update_system_config", "system config updated", map[string]interface{}{
		"config_key":   key,
		"config_value": newValue,
		"description":  description,
	})

	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]string{
		"config_key":   key,
		"config_value": newValue,
		"description":  description,
	}})
}

// Encryption stubs
func (h *Handler) GetEncryptionStatus(c *gin.Context) {
	c.JSON(501, Response{Success: false, Error: "not implemented"})
}
