// Made by YTSworks
// YTS工作室製作
package syslog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type NormalizedDeviceEvent struct {
	Facility          string
	Severity          string
	RawMessage        string
	NormalizedMessage string
	MatchedRule       string
	Context           map[string]interface{}
}

var facilityNames = []string{
	"kern",
	"user",
	"mail",
	"daemon",
	"auth",
	"syslog",
	"lpr",
	"news",
	"uucp",
	"clock",
	"authpriv",
	"ftp",
	"ntp",
	"audit",
	"alert",
	"clock2",
	"local0",
	"local1",
	"local2",
	"local3",
	"local4",
	"local5",
	"local6",
	"local7",
}

var severityNames = []string{
	"emergency",
	"alert",
	"critical",
	"error",
	"warning",
	"notice",
	"info",
	"debug",
}

func NormalizeSyslogMessage(message string) NormalizedDeviceEvent {
	priority, facility, severity, stripped := extractSyslogPriority(message)
	normalized, rule := normalizeDeviceMessage(stripped)
	context := map[string]interface{}{
		"source_type": "syslog",
	}
	if priority >= 0 {
		context["priority"] = priority
	}
	if rule != "" {
		context["matched_rule"] = rule
	}
	return NormalizedDeviceEvent{
		Facility:          facility,
		Severity:          severity,
		RawMessage:        strings.TrimSpace(message),
		NormalizedMessage: normalized,
		MatchedRule:       rule,
		Context:           context,
	}
}

func NormalizeTrapEvent(enterpriseOID, trapOID string, bindings map[string]string) NormalizedDeviceEvent {
	rawParts := []string{}
	if enterpriseOID != "" {
		rawParts = append(rawParts, "enterprise="+enterpriseOID)
	}
	if trapOID != "" {
		rawParts = append(rawParts, "trap_oid="+trapOID)
	}
	if len(bindings) > 0 {
		keys := make([]string, 0, len(bindings))
		for key := range bindings {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			rawParts = append(rawParts, fmt.Sprintf("%s=%s", key, bindings[key]))
		}
	}
	rawMessage := strings.Join(rawParts, " ")
	normalized, rule := normalizeTrapMessage(trapOID, bindings)
	context := map[string]interface{}{
		"source_type":    "snmp_trap",
		"enterprise_oid": enterpriseOID,
		"trap_oid":       trapOID,
	}
	if len(bindings) > 0 {
		context["bindings"] = bindings
	}
	if rule != "" {
		context["matched_rule"] = rule
	}
	return NormalizedDeviceEvent{
		Facility:          "snmp-trap",
		Severity:          trapSeverity(rule),
		RawMessage:        rawMessage,
		NormalizedMessage: normalized,
		MatchedRule:       rule,
		Context:           context,
	}
}

func extractSyslogPriority(message string) (int, string, string, string) {
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "<") {
		return -1, "syslog", "info", trimmed
	}
	end := strings.IndexByte(trimmed, '>')
	if end <= 1 {
		return -1, "syslog", "info", trimmed
	}
	pri, err := strconv.Atoi(trimmed[1:end])
	if err != nil {
		return -1, "syslog", "info", trimmed
	}
	facilityCode := pri / 8
	severityCode := pri % 8
	facility := lookupName(facilityCode, facilityNames, "syslog")
	severity := lookupName(severityCode, severityNames, "info")
	return pri, facility, severity, strings.TrimSpace(trimmed[end+1:])
}

func lookupName(index int, values []string, fallback string) string {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return fallback
}

func normalizeDeviceMessage(message string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case containsAny(lower, "link down", "ifdown", "port down"):
		return "Network link down", "network_link_down"
	case containsAny(lower, "link up", "ifup", "port up"):
		return "Network link up", "network_link_up"
	case containsAny(lower, "poe off", "poe disabled", "power inline denied"):
		return "PoE disabled", "poe_disabled"
	case containsAny(lower, "poe on", "poe enabled", "power inline granted"):
		return "PoE enabled", "poe_enabled"
	case containsAny(lower, "authentication failure", "login failed", "wrong password", "auth fail"):
		return "Authentication failure", "authentication_failure"
	case containsAny(lower, "configuration changed", "config changed", "config saved", "write memory"):
		return "Configuration changed", "config_changed"
	case containsAny(lower, "reboot", "restarting system", "warm start", "cold start"):
		return "Device reboot event", "device_reboot"
	default:
		return strings.TrimSpace(message), ""
	}
}

func normalizeTrapMessage(trapOID string, bindings map[string]string) (string, string) {
	lowerOID := strings.ToLower(strings.TrimSpace(trapOID))
	switch {
	case strings.Contains(lowerOID, "linkdown"):
		return "SNMP trap: link down", "snmp_link_down"
	case strings.Contains(lowerOID, "linkup"):
		return "SNMP trap: link up", "snmp_link_up"
	case strings.Contains(lowerOID, "coldstart"):
		return "SNMP trap: cold start", "snmp_cold_start"
	case strings.Contains(lowerOID, "warmstart"):
		return "SNMP trap: warm start", "snmp_warm_start"
	case strings.Contains(lowerOID, "authenticationfailure"):
		return "SNMP trap: authentication failure", "snmp_auth_failure"
	}
	if len(bindings) > 0 {
		if ifAdmin, ok := bindings["ifAdminStatus"]; ok {
			return "SNMP trap: interface admin status " + ifAdmin, "snmp_interface_admin_status"
		}
	}
	return "SNMP trap received", ""
}

func trapSeverity(rule string) string {
	switch rule {
	case "snmp_link_down", "snmp_auth_failure":
		return "warning"
	case "snmp_cold_start", "snmp_warm_start":
		return "notice"
	default:
		return "info"
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func MarshalContextJSON(context map[string]interface{}) string {
	if len(context) == 0 {
		return ""
	}
	b, err := json.Marshal(context)
	if err != nil {
		return ""
	}
	return string(b)
}
