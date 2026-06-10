package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	cameramodule "management-server/modules/camera"
	logsmodule "management-server/modules/logs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"management-server/services/logging"
)

type AuditEntry struct {
	ID           int    `json:"id"`
	OccurredAt   string `json:"occurred_at"`
	Username     string `json:"username"`
	SourceIP     string `json:"source_ip"`
	SourceMAC    string `json:"source_mac"`
	Module       string `json:"module"`
	Action       string `json:"action"`
	Resource     string `json:"resource"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceIP   string `json:"resource_ip"`
	ResourceMAC  string `json:"resource_mac"`
	Status       string `json:"status"`
	Detail       string `json:"detail"`
	DetailJSON   string `json:"detail_json"`
	RecordsetID  string `json:"recordset_id"`
	Correlation  string `json:"correlation_id"`
	ReviewStatus string `json:"review_status"`
	ReviewedBy   string `json:"reviewed_by"`
	ReviewedAt   string `json:"reviewed_at"`
	ReviewNote   string `json:"review_note"`
}

func (h *Handler) WriteAudit(username, sourceIP, sourceMac, action, resource, status string, details map[string]interface{}) {
	details = redactAuditDetails(details)
	logging.AuditLog(username, sourceIP, action, resource, status, details)

	module := inferAuditModule(action, resource, details)
	resourceType, resourceID, resourceName, resourceIP, resourceMAC := inferAuditResourceFields(resource, details)
	detailStr := marshalAuditDetails(details)
	recordsetID := firstNonEmpty(
		stringDetail(details, "recordset_id"),
		stringDetail(details, "correlation_id"),
		fmt.Sprintf("audit-%d", time.Now().UnixNano()),
	)
	correlationID := firstNonEmpty(
		stringDetail(details, "correlation_id"),
		recordsetID,
	)

	h.db.Exec(`
		INSERT INTO audit_logs (
			occurred_at, username, source_ip, source_mac, module, action, resource,
			resource_type, resource_id, resource_name, resource_ip, resource_mac,
			status, detail, detail_json, recordset_id, correlation_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		time.Now().Format("2006-01-02 15:04:05"),
		username,
		sourceIP,
		sourceMac,
		module,
		action,
		resource,
		resourceType,
		resourceID,
		resourceName,
		resourceIP,
		resourceMAC,
		status,
		detailStr,
		detailStr,
		recordsetID,
		correlationID,
	)

	if shouldWriteConfigChange(details) {
		h.writeConfigChangeLog(username, sourceIP, module, action, status, resourceType, resourceID, resourceName, resourceIP, resourceMAC, recordsetID, correlationID, details)
	}
}

func (h *Handler) writeConfigChangeLog(username, sourceIP, module, action, status, targetType, targetID, targetName, targetIP, targetMAC, recordsetID, correlationID string, details map[string]interface{}) {
	oldValues := marshalAuditDetails(mapDetail(details, "old_values"))
	newValues := marshalAuditDetails(mapDetail(details, "new_values"))
	detailJSON := marshalAuditDetails(details)
	changeSource := firstNonEmpty(stringDetail(details, "change_source"), "manual")
	changeScope := firstNonEmpty(stringDetail(details, "change_scope"), action)

	h.db.Exec(`
		INSERT INTO config_change_logs (
			occurred_at, username, source_ip, module, target_type, target_id, target_name,
			target_ip, target_mac, action, change_scope, status, change_source,
			recordset_id, correlation_id, old_values_json, new_values_json, detail_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		time.Now().Format("2006-01-02 15:04:05"),
		username,
		sourceIP,
		module,
		targetType,
		targetID,
		targetName,
		targetIP,
		targetMAC,
		action,
		changeScope,
		status,
		changeSource,
		recordsetID,
		correlationID,
		oldValues,
		newValues,
		detailJSON,
	)
}

func shouldWriteConfigChange(details map[string]interface{}) bool {
	if len(mapDetail(details, "old_values")) > 0 || len(mapDetail(details, "new_values")) > 0 {
		return true
	}
	if v, ok := details["config_change"].(bool); ok {
		return v
	}
	return false
}

func marshalAuditDetails(details map[string]interface{}) string {
	if len(details) == 0 {
		return ""
	}
	b, err := json.Marshal(redactAuditDetails(details))
	if err != nil {
		return ""
	}
	return string(b)
}

func redactAuditDetails(details map[string]interface{}) map[string]interface{} {
	if details == nil {
		return nil
	}
	redacted := make(map[string]interface{}, len(details))
	for key, value := range details {
		redacted[key] = redactSensitiveValue(key, value)
	}
	return redacted
}

func redactSensitiveValue(key string, value interface{}) interface{} {
	if value == nil {
		return nil
	}
	if isSensitiveAuditKey(key) {
		return "<redacted>"
	}
	switch v := value.(type) {
	case string:
		return cameramodule.RedactSensitiveText(v)
	case map[string]interface{}:
		return redactAuditDetails(v)
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = redactSensitiveValue(key, item)
		}
		return out
	case []map[string]interface{}:
		out := make([]map[string]interface{}, len(v))
		for i, item := range v {
			out[i] = redactAuditDetails(item)
		}
		return out
	default:
		return value
	}
}

func isSensitiveAuditKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "password", "pass", "passwd", "pwd", "password_encrypted",
		"cli_password", "community", "snmp_ro_community", "snmp_rw_community", "snmp_community",
		"snmpv3_auth_password", "snmpv3_priv_password",
		"token", "access_token", "refresh_token", "auth_token", "api_key",
		"secret", "client_secret", "private_key", "authorization":
		return true
	}
	return strings.HasSuffix(key, "_password") ||
		strings.HasSuffix(key, "_token") ||
		strings.HasSuffix(key, "_secret") ||
		strings.HasSuffix(key, "_community") ||
		strings.Contains(key, "password_encrypted") ||
		strings.Contains(key, "snmp_community")
}

func mapDetail(details map[string]interface{}, key string) map[string]interface{} {
	if details == nil {
		return nil
	}
	if value, ok := details[key]; ok {
		if m, ok := value.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func stringDetail(details map[string]interface{}, key string) string {
	if details == nil {
		return ""
	}
	value, ok := details[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func inferAuditModule(action, resource string, details map[string]interface{}) string {
	if module := stringDetail(details, "module"); module != "" {
		return module
	}

	resourceType, _, _, _, _ := inferAuditResourceFields(resource, details)
	if resourceType != "" {
		return resourceType
	}

	switch {
	case strings.Contains(action, "license"):
		return "license"
	case strings.Contains(action, "camera"):
		return "camera"
	case strings.Contains(action, "device"), strings.Contains(action, "poe"), strings.Contains(action, "port"):
		return "device"
	case strings.Contains(action, "topology"):
		return "topology"
	case strings.Contains(action, "user"), strings.Contains(action, "password"), strings.Contains(action, "login"), strings.Contains(action, "logout"):
		return "auth"
	default:
		return "system"
	}
}

func inferAuditResourceFields(resource string, details map[string]interface{}) (resourceType, resourceID, resourceName, resourceIP, resourceMAC string) {
	resource = strings.TrimSpace(resource)
	if resource != "" {
		parts := strings.SplitN(resource, ":", 2)
		resourceType = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			resourceID = strings.TrimSpace(parts[1])
		}
	}

	resourceType = firstNonEmpty(resourceType, stringDetail(details, "resource_type"), stringDetail(details, "target_type"))
	resourceID = firstNonEmpty(resourceID, stringDetail(details, "resource_id"), stringDetail(details, "target_id"))
	resourceName = firstNonEmpty(
		stringDetail(details, "resource_name"),
		stringDetail(details, "target_name"),
		stringDetail(details, "device_name"),
		stringDetail(details, "camera_name"),
		stringDetail(details, "name"),
	)
	resourceIP = firstNonEmpty(
		stringDetail(details, "resource_ip"),
		stringDetail(details, "target_ip"),
		stringDetail(details, "ip_address"),
	)
	resourceMAC = firstNonEmpty(
		stringDetail(details, "resource_mac"),
		stringDetail(details, "target_mac"),
		stringDetail(details, "mac_address"),
	)
	return
}

func (h *Handler) auditUsername(c *gin.Context) string {
	username := strings.TrimSpace(c.GetString("username"))
	if username == "" {
		username = "anonymous"
	}
	return username
}

func (h *Handler) auditSourceMAC(c *gin.Context) string {
	for _, key := range []string{"X-Client-MAC", "X-Source-MAC", "X-Device-MAC"} {
		if value := strings.TrimSpace(c.GetHeader(key)); value != "" {
			return value
		}
	}
	return ""
}

func (h *Handler) WriteAuditFromContext(c *gin.Context, action, resource, status string, details map[string]interface{}) {
	h.WriteAudit(
		h.auditUsername(c),
		c.ClientIP(),
		h.auditSourceMAC(c),
		action,
		resource,
		status,
		details,
	)
}

func auditResource(kind string, id interface{}) string {
	return fmt.Sprintf("%s:%v", kind, id)
}

func auditDetails(base map[string]interface{}, kv ...interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range base {
		result[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok || key == "" {
			continue
		}
		result[key] = kv[i+1]
	}
	return result
}

func (h *Handler) deviceAuditDetails(deviceID int) map[string]interface{} {
	details := map[string]interface{}{
		"device_id":     deviceID,
		"resource_id":   strconv.Itoa(deviceID),
		"resource_type": "device",
		"target_id":     strconv.Itoa(deviceID),
		"target_type":   "device",
	}

	var name, ipAddress, deviceType string
	var macAddress sql.NullString
	err := h.db.QueryRow(
		"SELECT name, ip_address, mac_address, device_type FROM devices WHERE id = ?",
		deviceID,
	).Scan(&name, &ipAddress, &macAddress, &deviceType)
	if err != nil {
		details["lookup_error"] = err.Error()
		return details
	}

	details["device_name"] = name
	details["resource_name"] = name
	details["target_name"] = name
	details["ip_address"] = ipAddress
	details["resource_ip"] = ipAddress
	details["target_ip"] = ipAddress
	details["device_type"] = deviceType
	details["mac_address"] = macAddress.String
	details["resource_mac"] = macAddress.String
	details["target_mac"] = macAddress.String
	return details
}

func (h *Handler) GetAuditLogs(c *gin.Context) {
	page, limit, _ := logsmodule.ParsePageLimit(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "50"), 50)
	filters := logsmodule.QueryFilters{
		Username:     strings.TrimSpace(c.Query("username")),
		Module:       strings.TrimSpace(c.Query("module")),
		Action:       strings.TrimSpace(c.Query("action")),
		Device:       strings.TrimSpace(c.Query("device")),
		Status:       strings.TrimSpace(c.Query("status")),
		ReviewStatus: strings.TrimSpace(c.Query("review_status")),
		Search:       strings.TrimSpace(c.Query("search")),
		DateFrom:     strings.TrimSpace(c.Query("date_from")),
		DateTo:       strings.TrimSpace(c.Query("date_to")),
	}

	list, total, err := h.logs.ListAuditLogs(filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    list,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

func (h *Handler) GetAuditActionKeys(c *gin.Context) {
	keys, err := h.logs.ListAuditActionKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": keys})
}
