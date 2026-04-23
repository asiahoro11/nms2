package logs

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func sqliteTimeToRFC3339(s string) string {
	if s == "" {
		return s
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, layout := range formats {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return s
}

func ParsePageLimit(pageRaw, limitRaw string, defaultLimit int) (int, int, int) {
	page, _ := strconv.Atoi(pageRaw)
	limit, _ := strconv.Atoi(limitRaw)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = defaultLimit
	}
	offset := (page - 1) * limit
	return page, limit, offset
}

func countRows(db *sql.DB, query string, args ...interface{}) int {
	var total int
	_ = db.QueryRow(query, args...).Scan(&total)
	return total
}

func distinctStrings(db *sql.DB, query string, args ...interface{}) []string {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err == nil && value.Valid {
			trimmed := strings.TrimSpace(value.String)
			if trimmed != "" {
				values = append(values, trimmed)
			}
		}
	}
	return values
}

func (s *Service) ListSystemLogs(filters QueryFilters, page, limit int) ([]SystemLogEntry, int, error) {
	offset := (page - 1) * limit
	query := `SELECT id, occurred_at, level, service, event_code, message, context_json, node_name, build_version,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM system_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM system_logs WHERE 1=1`
	var args []interface{}

	if filters.Level != "" {
		query += " AND level = ?"
		countQuery += " AND level = ?"
		args = append(args, filters.Level)
	}
	if filters.Service != "" {
		query += " AND service = ?"
		countQuery += " AND service = ?"
		args = append(args, filters.Service)
	}
	if filters.ReviewStatus != "" {
		query += " AND COALESCE(review_status, 'pending') = ?"
		countQuery += " AND COALESCE(review_status, 'pending') = ?"
		args = append(args, filters.ReviewStatus)
	}
	if filters.Search != "" {
		query += " AND (message LIKE ? OR service LIKE ? OR event_code LIKE ?)"
		countQuery += " AND (message LIKE ? OR service LIKE ? OR event_code LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.DateFrom != "" {
		query += " AND occurred_at >= ?"
		countQuery += " AND occurred_at >= ?"
		args = append(args, filters.DateFrom)
	}
	if filters.DateTo != "" {
		query += " AND occurred_at <= ?"
		countQuery += " AND occurred_at <= ?"
		args = append(args, filters.DateTo+" 23:59:59")
	}

	total := countRows(s.db, countQuery, args...)
	query += " ORDER BY occurred_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []SystemLogEntry{}
	for rows.Next() {
		var entry SystemLogEntry
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Level, &entry.Service, &entry.EventCode, &entry.Message, &entry.ContextJSON, &entry.NodeName, &entry.BuildVersion, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, total, nil
}

func (s *Service) ListDeviceLogs(filters QueryFilters, page, limit int) ([]DeviceLogEntry, int, error) {
	offset := (page - 1) * limit
	query := `SELECT id, occurred_at, device_id, device_name, ip_address, mac_address, facility, severity,
		raw_message, normalized_message, matched_rule, ack_status, context_json,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM device_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM device_logs WHERE 1=1`
	var args []interface{}

	if filters.DeviceID != "" {
		query += " AND device_id = ?"
		countQuery += " AND device_id = ?"
		args = append(args, filters.DeviceID)
	}
	if filters.Device != "" {
		query += " AND (COALESCE(device_name, '') LIKE ? OR COALESCE(ip_address, '') LIKE ? OR COALESCE(mac_address, '') LIKE ?)"
		countQuery += " AND (COALESCE(device_name, '') LIKE ? OR COALESCE(ip_address, '') LIKE ? OR COALESCE(mac_address, '') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Severity != "" {
		query += " AND severity = ?"
		countQuery += " AND severity = ?"
		args = append(args, filters.Severity)
	}
	if filters.Facility != "" {
		query += " AND facility = ?"
		countQuery += " AND facility = ?"
		args = append(args, filters.Facility)
	}
	if filters.AckStatus != "" {
		query += " AND ack_status = ?"
		countQuery += " AND ack_status = ?"
		args = append(args, filters.AckStatus)
	}
	if filters.ReviewStatus != "" {
		query += " AND COALESCE(review_status, 'pending') = ?"
		countQuery += " AND COALESCE(review_status, 'pending') = ?"
		args = append(args, filters.ReviewStatus)
	}
	if filters.Search != "" {
		query += " AND (raw_message LIKE ? OR normalized_message LIKE ? OR device_name LIKE ? OR ip_address LIKE ?)"
		countQuery += " AND (raw_message LIKE ? OR normalized_message LIKE ? OR device_name LIKE ? OR ip_address LIKE ?)"
		for i := 0; i < 4; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.DateFrom != "" {
		query += " AND occurred_at >= ?"
		countQuery += " AND occurred_at >= ?"
		args = append(args, filters.DateFrom)
	}
	if filters.DateTo != "" {
		query += " AND occurred_at <= ?"
		countQuery += " AND occurred_at <= ?"
		args = append(args, filters.DateTo+" 23:59:59")
	}

	total := countRows(s.db, countQuery, args...)
	query += " ORDER BY occurred_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []DeviceLogEntry{}
	for rows.Next() {
		var entry DeviceLogEntry
		var id sql.NullInt64
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &id, &entry.DeviceName, &entry.IPAddress, &entry.MACAddress, &entry.Facility, &entry.Severity, &entry.RawMessage, &entry.NormalizedMessage, &entry.MatchedRule, &entry.AckStatus, &entry.ContextJSON, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			if id.Valid {
				parsed := int(id.Int64)
				entry.DeviceID = &parsed
			}
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, total, nil
}

func (s *Service) ListConfigChangeLogs(filters QueryFilters, page, limit int) ([]ConfigChangeLogEntry, int, error) {
	offset := (page - 1) * limit
	query := `SELECT id, occurred_at, username, source_ip, module, target_type, target_id, target_name,
		target_ip, target_mac, action, change_scope, status, change_source, recordset_id,
		correlation_id, old_values_json, new_values_json, detail_json,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM config_change_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM config_change_logs WHERE 1=1`
	var args []interface{}

	if filters.Module != "" {
		query += " AND module = ?"
		countQuery += " AND module = ?"
		args = append(args, filters.Module)
	}
	if filters.TargetType != "" {
		query += " AND target_type = ?"
		countQuery += " AND target_type = ?"
		args = append(args, filters.TargetType)
	}
	if filters.Username != "" {
		query += " AND username LIKE ?"
		countQuery += " AND username LIKE ?"
		args = append(args, "%"+filters.Username+"%")
	}
	if filters.Device != "" {
		query += " AND (COALESCE(target_name, '') LIKE ? OR COALESCE(target_ip, '') LIKE ? OR COALESCE(target_mac, '') LIKE ?)"
		countQuery += " AND (COALESCE(target_name, '') LIKE ? OR COALESCE(target_ip, '') LIKE ? OR COALESCE(target_mac, '') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Status != "" {
		query += " AND status = ?"
		countQuery += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.ReviewStatus != "" {
		query += " AND COALESCE(review_status, 'pending') = ?"
		countQuery += " AND COALESCE(review_status, 'pending') = ?"
		args = append(args, filters.ReviewStatus)
	}
	if filters.Search != "" {
		query += " AND (target_name LIKE ? OR username LIKE ? OR action LIKE ?)"
		countQuery += " AND (target_name LIKE ? OR username LIKE ? OR action LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.DateFrom != "" {
		query += " AND occurred_at >= ?"
		countQuery += " AND occurred_at >= ?"
		args = append(args, filters.DateFrom)
	}
	if filters.DateTo != "" {
		query += " AND occurred_at <= ?"
		countQuery += " AND occurred_at <= ?"
		args = append(args, filters.DateTo+" 23:59:59")
	}

	total := countRows(s.db, countQuery, args...)
	query += " ORDER BY occurred_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []ConfigChangeLogEntry{}
	for rows.Next() {
		var entry ConfigChangeLogEntry
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Username, &entry.SourceIP, &entry.Module, &entry.TargetType, &entry.TargetID, &entry.TargetName, &entry.TargetIP, &entry.TargetMAC, &entry.Action, &entry.ChangeScope, &entry.Status, &entry.ChangeSource, &entry.RecordsetID, &entry.CorrelationID, &entry.OldValuesJSON, &entry.NewValuesJSON, &entry.DetailJSON, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, total, nil
}

func (s *Service) ListAuditLogs(filters QueryFilters, page, limit int) ([]AuditEntry, int, error) {
	offset := (page - 1) * limit
	baseWhere := " WHERE 1=1"
	var args []interface{}

	if filters.Username != "" {
		baseWhere += " AND username LIKE ?"
		args = append(args, "%"+filters.Username+"%")
	}
	if filters.Module != "" {
		baseWhere += " AND COALESCE(module, '') = ?"
		args = append(args, filters.Module)
	}
	if filters.Action != "" {
		baseWhere += " AND action = ?"
		args = append(args, filters.Action)
	}
	if filters.Device != "" {
		baseWhere += " AND (COALESCE(resource_name,'') LIKE ? OR COALESCE(resource_ip,'') LIKE ? OR COALESCE(resource_mac,'') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Status != "" {
		baseWhere += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.ReviewStatus != "" {
		baseWhere += " AND COALESCE(review_status, 'pending') = ?"
		args = append(args, filters.ReviewStatus)
	}
	if filters.Search != "" {
		baseWhere += " AND (username LIKE ? OR action LIKE ? OR COALESCE(resource_name,'') LIKE ? OR COALESCE(resource,'') LIKE ?)"
		for i := 0; i < 4; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.DateFrom != "" {
		baseWhere += " AND occurred_at >= ?"
		args = append(args, filters.DateFrom)
	}
	if filters.DateTo != "" {
		baseWhere += " AND occurred_at <= ?"
		args = append(args, filters.DateTo+" 23:59:59")
	}

	total := countRows(s.db, "SELECT COUNT(*) FROM audit_logs"+baseWhere, args...)
	query := `SELECT id, occurred_at, username, COALESCE(source_ip,''), COALESCE(source_mac,''),
		COALESCE(module,''), action, COALESCE(resource,''), COALESCE(resource_type,''), COALESCE(resource_id,''),
		COALESCE(resource_name,''), COALESCE(resource_ip,''), COALESCE(resource_mac,''), status,
		COALESCE(detail,''), COALESCE(detail_json, COALESCE(detail,'')), COALESCE(recordset_id,''), COALESCE(correlation_id,''),
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM audit_logs` + baseWhere + " ORDER BY occurred_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.Username, &e.SourceIP, &e.SourceMAC, &e.Module, &e.Action, &e.Resource, &e.ResourceType, &e.ResourceID, &e.ResourceName, &e.ResourceIP, &e.ResourceMAC, &e.Status, &e.Detail, &e.DetailJSON, &e.RecordsetID, &e.Correlation, &e.ReviewStatus, &e.ReviewedBy, &e.ReviewedAt, &e.ReviewNote); err == nil {
			e.OccurredAt = sqliteTimeToRFC3339(e.OccurredAt)
			e.ReviewedAt = sqliteTimeToRFC3339(e.ReviewedAt)
			list = append(list, e)
		}
	}
	return list, total, nil
}

func (s *Service) ListAuditActionKeys() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT action FROM audit_logs ORDER BY action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err == nil {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *Service) Options() Options {
	return Options{
		SystemServices:   distinctStrings(s.db, `SELECT DISTINCT service FROM system_logs WHERE COALESCE(service,'') <> '' ORDER BY service`),
		DeviceFacilities: distinctStrings(s.db, `SELECT DISTINCT facility FROM device_logs WHERE COALESCE(facility,'') <> '' ORDER BY facility`),
		AuditModules:     distinctStrings(s.db, `SELECT DISTINCT module FROM audit_logs WHERE COALESCE(module,'') <> '' ORDER BY module`),
		ConfigModules:    distinctStrings(s.db, `SELECT DISTINCT module FROM config_change_logs WHERE COALESCE(module,'') <> '' ORDER BY module`),
	}
}

func NormalizeReviewStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "reviewed", "flagged", "pending":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func NormalizeAckStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "acked", "unacked":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func (s *Service) UpdateReview(table, id, reviewStatus, reviewNote, reviewedBy string) (bool, error) {
	res, err := s.db.Exec(
		fmt.Sprintf("UPDATE %s SET review_status=?, reviewed_by=?, reviewed_at=?, review_note=? WHERE id=?", table),
		reviewStatus,
		reviewedBy,
		time.Now().Format("2006-01-02 15:04:05"),
		strings.TrimSpace(reviewNote),
		id,
	)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *Service) AckDeviceLog(id, ackStatus, reviewNote, reviewedBy string) (bool, error) {
	res, err := s.db.Exec(
		"UPDATE device_logs SET ack_status=?, review_status=?, reviewed_by=?, reviewed_at=?, review_note=? WHERE id=?",
		ackStatus,
		"reviewed",
		reviewedBy,
		time.Now().Format("2006-01-02 15:04:05"),
		strings.TrimSpace(reviewNote),
		id,
	)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *Service) ExportRows(logType string, filters LogCenterFilters) ([]string, [][]string, string, error) {
	query, headers, filename, args, err := buildLogCenterExportQuery(logType, filters)
	if err != nil {
		return nil, nil, "", err
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, nil, "", err
	}
	defer rows.Close()

	records := make([][]string, 0)
	for rows.Next() {
		record, ok := scanLogCenterExportRow(rows)
		if ok {
			records = append(records, record)
		}
	}

	return headers, records, filename, nil
}

func (s *Service) FetchSystemLogsForBundle(filters LogCenterFilters) ([]SystemLogEntry, error) {
	return s.fetchSystemLogsForBundle(filters)
}

func (s *Service) FetchDeviceLogsForBundle(filters LogCenterFilters) ([]DeviceLogEntry, error) {
	return s.fetchDeviceLogsForBundle(filters)
}

func (s *Service) FetchAuditLogsForBundle(filters LogCenterFilters) ([]AuditEntry, error) {
	return s.fetchAuditLogsForBundle(filters)
}

func (s *Service) FetchConfigChangeLogsForBundle(filters LogCenterFilters) ([]ConfigChangeLogEntry, error) {
	return s.fetchConfigChangeLogsForBundle(filters)
}

func buildLogCenterExportQuery(logType string, filters LogCenterFilters) (string, []string, string, []interface{}, error) {
	var query string
	var args []interface{}
	headers := []string{"ID", "Source/Type", "Severity", "Message", "Time"}
	filename := logType

	switch logType {
	case "system_logs":
		query = `SELECT id, service, level, event_code, message, occurred_at FROM system_logs WHERE 1=1`
		headers = []string{"ID", "Service", "Level", "Event Code", "Message", "Time"}
	case "device_logs":
		query = `SELECT id, device_name, severity, facility, normalized_message, occurred_at FROM device_logs WHERE 1=1`
		headers = []string{"ID", "Device", "Severity", "Facility", "Message", "Time"}
	case "audit_logs":
		query = `SELECT id, username, action, status, COALESCE(resource_name, resource, ''), occurred_at FROM audit_logs WHERE 1=1`
		headers = []string{"ID", "User", "Action", "Status", "Resource", "Time"}
	case "config_change_logs":
		query = `SELECT id, module, action, status, COALESCE(target_name, target_type || ':' || target_id, ''), occurred_at FROM config_change_logs WHERE 1=1`
		headers = []string{"ID", "Module", "Action", "Status", "Target", "Time"}
	default:
		return "", nil, "", nil, fmt.Errorf("unsupported log type")
	}

	if filters.Start != "" {
		query += " AND occurred_at >= ?"
		args = append(args, filters.Start)
	}
	if filters.End != "" {
		query += " AND occurred_at <= ?"
		args = append(args, filters.End)
	}
	if filters.Severity != "" {
		if logType == "system_logs" {
			query += " AND level = ?"
			args = append(args, filters.Severity)
		} else if logType == "device_logs" {
			query += " AND severity = ?"
			args = append(args, filters.Severity)
		}
	}
	if filters.Scope != "" {
		switch logType {
		case "system_logs":
			query += " AND service = ?"
			args = append(args, filters.Scope)
		case "device_logs":
			query += " AND facility = ?"
			args = append(args, filters.Scope)
		case "audit_logs":
			query += " AND COALESCE(module, '') = ?"
			args = append(args, filters.Scope)
		case "config_change_logs":
			query += " AND module = ?"
			args = append(args, filters.Scope)
		}
	}
	if filters.Actor != "" {
		switch logType {
		case "audit_logs", "config_change_logs":
			query += " AND username LIKE ?"
			args = append(args, "%"+filters.Actor+"%")
		}
	}
	if filters.Device != "" {
		switch logType {
		case "device_logs":
			query += " AND (COALESCE(device_name, '') LIKE ? OR COALESCE(ip_address, '') LIKE ? OR COALESCE(mac_address, '') LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Device+"%")
			}
		case "audit_logs":
			query += " AND (COALESCE(resource_name, '') LIKE ? OR COALESCE(resource_ip, '') LIKE ? OR COALESCE(resource_mac, '') LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Device+"%")
			}
		case "config_change_logs":
			query += " AND (COALESCE(target_name, '') LIKE ? OR COALESCE(target_ip, '') LIKE ? OR COALESCE(target_mac, '') LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Device+"%")
			}
		}
	}
	if filters.Status != "" {
		switch logType {
		case "system_logs":
			query += " AND COALESCE(review_status, 'pending') = ?"
			args = append(args, filters.Status)
		case "device_logs":
			query += " AND ack_status = ?"
			args = append(args, filters.Status)
		case "audit_logs", "config_change_logs":
			query += " AND status = ?"
			args = append(args, filters.Status)
		}
	}
	if filters.Search != "" {
		switch logType {
		case "system_logs":
			query += " AND (message LIKE ? OR service LIKE ? OR event_code LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Search+"%")
			}
		case "device_logs":
			query += " AND (normalized_message LIKE ? OR raw_message LIKE ? OR device_name LIKE ? OR ip_address LIKE ?)"
			for i := 0; i < 4; i++ {
				args = append(args, "%"+filters.Search+"%")
			}
		case "audit_logs":
			query += " AND (username LIKE ? OR action LIKE ? OR COALESCE(resource_name, resource, '') LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Search+"%")
			}
		case "config_change_logs":
			query += " AND (COALESCE(target_name, '') LIKE ? OR action LIKE ? OR module LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+filters.Search+"%")
			}
		}
	}

	query += " ORDER BY occurred_at DESC LIMIT 10000"
	return query, headers, filename, args, nil
}

func scanLogCenterExportRow(rows *sql.Rows) ([]string, bool) {
	var id int
	var col1, col2, col3, col4, timeVal string
	if err := rows.Scan(&id, &col1, &col2, &col3, &col4, &timeVal); err != nil {
		return nil, false
	}
	return []string{fmt.Sprintf("%d", id), col1, col2, col3, col4, timeVal}, true
}

func (s *Service) fetchSystemLogsForBundle(filters LogCenterFilters) ([]SystemLogEntry, error) {
	query := `SELECT id, occurred_at, level, service, event_code, message, context_json, node_name, build_version,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM system_logs WHERE 1=1`
	var args []interface{}

	if filters.Severity != "" {
		query += " AND level = ?"
		args = append(args, filters.Severity)
	}
	if filters.Scope != "" {
		query += " AND service = ?"
		args = append(args, filters.Scope)
	}
	if filters.Status != "" {
		query += " AND COALESCE(review_status, 'pending') = ?"
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += " AND (message LIKE ? OR service LIKE ? OR event_code LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.Start != "" {
		query += " AND occurred_at >= ?"
		args = append(args, filters.Start)
	}
	if filters.End != "" {
		query += " AND occurred_at <= ?"
		args = append(args, filters.End+" 23:59:59")
	}

	query += " ORDER BY occurred_at DESC LIMIT 10000"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []SystemLogEntry{}
	for rows.Next() {
		var entry SystemLogEntry
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Level, &entry.Service, &entry.EventCode, &entry.Message, &entry.ContextJSON, &entry.NodeName, &entry.BuildVersion, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, nil
}

func (s *Service) fetchDeviceLogsForBundle(filters LogCenterFilters) ([]DeviceLogEntry, error) {
	query := `SELECT id, occurred_at, device_id, device_name, ip_address, mac_address, facility, severity,
		raw_message, normalized_message, matched_rule, ack_status, context_json,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM device_logs WHERE 1=1`
	var args []interface{}

	if filters.Severity != "" {
		query += " AND severity = ?"
		args = append(args, filters.Severity)
	}
	if filters.Scope != "" {
		query += " AND facility = ?"
		args = append(args, filters.Scope)
	}
	if filters.Device != "" {
		query += " AND (COALESCE(device_name, '') LIKE ? OR COALESCE(ip_address, '') LIKE ? OR COALESCE(mac_address, '') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Status != "" {
		query += " AND ack_status = ?"
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += " AND (raw_message LIKE ? OR normalized_message LIKE ? OR device_name LIKE ? OR ip_address LIKE ?)"
		for i := 0; i < 4; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.Start != "" {
		query += " AND occurred_at >= ?"
		args = append(args, filters.Start)
	}
	if filters.End != "" {
		query += " AND occurred_at <= ?"
		args = append(args, filters.End+" 23:59:59")
	}

	query += " ORDER BY occurred_at DESC LIMIT 10000"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []DeviceLogEntry{}
	for rows.Next() {
		var entry DeviceLogEntry
		var id sql.NullInt64
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &id, &entry.DeviceName, &entry.IPAddress, &entry.MACAddress, &entry.Facility, &entry.Severity, &entry.RawMessage, &entry.NormalizedMessage, &entry.MatchedRule, &entry.AckStatus, &entry.ContextJSON, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			if id.Valid {
				parsed := int(id.Int64)
				entry.DeviceID = &parsed
			}
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, nil
}

func (s *Service) fetchAuditLogsForBundle(filters LogCenterFilters) ([]AuditEntry, error) {
	query := `SELECT id, occurred_at, username, COALESCE(source_ip,''), COALESCE(source_mac,''),
		COALESCE(module,''), action, COALESCE(resource,''), COALESCE(resource_type,''), COALESCE(resource_id,''),
		COALESCE(resource_name,''), COALESCE(resource_ip,''), COALESCE(resource_mac,''), status,
		COALESCE(detail,''), COALESCE(detail_json, COALESCE(detail,'')), COALESCE(recordset_id,''), COALESCE(correlation_id,''),
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM audit_logs WHERE 1=1`
	var args []interface{}

	if filters.Scope != "" {
		query += " AND COALESCE(module, '') = ?"
		args = append(args, filters.Scope)
	}
	if filters.Actor != "" {
		query += " AND username LIKE ?"
		args = append(args, "%"+filters.Actor+"%")
	}
	if filters.Device != "" {
		query += " AND (COALESCE(resource_name, '') LIKE ? OR COALESCE(resource_ip, '') LIKE ? OR COALESCE(resource_mac, '') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Status != "" {
		query += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += " AND (username LIKE ? OR action LIKE ? OR COALESCE(resource_name,'') LIKE ? OR COALESCE(resource,'') LIKE ?)"
		for i := 0; i < 4; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.Start != "" {
		query += " AND occurred_at >= ?"
		args = append(args, filters.Start)
	}
	if filters.End != "" {
		query += " AND occurred_at <= ?"
		args = append(args, filters.End+" 23:59:59")
	}

	query += " ORDER BY occurred_at DESC LIMIT 10000"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []AuditEntry{}
	for rows.Next() {
		var entry AuditEntry
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Username, &entry.SourceIP, &entry.SourceMAC, &entry.Module, &entry.Action, &entry.Resource, &entry.ResourceType, &entry.ResourceID, &entry.ResourceName, &entry.ResourceIP, &entry.ResourceMAC, &entry.Status, &entry.Detail, &entry.DetailJSON, &entry.RecordsetID, &entry.Correlation, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, nil
}

func (s *Service) fetchConfigChangeLogsForBundle(filters LogCenterFilters) ([]ConfigChangeLogEntry, error) {
	query := `SELECT id, occurred_at, username, source_ip, module, target_type, target_id, target_name,
		target_ip, target_mac, action, change_scope, status, change_source, recordset_id,
		correlation_id, old_values_json, new_values_json, detail_json,
		COALESCE(review_status, 'pending'), COALESCE(reviewed_by, ''), COALESCE(reviewed_at, ''), COALESCE(review_note, '')
		FROM config_change_logs WHERE 1=1`
	var args []interface{}

	if filters.Scope != "" {
		query += " AND module = ?"
		args = append(args, filters.Scope)
	}
	if filters.Actor != "" {
		query += " AND username LIKE ?"
		args = append(args, "%"+filters.Actor+"%")
	}
	if filters.Device != "" {
		query += " AND (COALESCE(target_name, '') LIKE ? OR COALESCE(target_ip, '') LIKE ? OR COALESCE(target_mac, '') LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Device+"%")
		}
	}
	if filters.Status != "" {
		query += " AND status = ?"
		args = append(args, filters.Status)
	}
	if filters.Search != "" {
		query += " AND (COALESCE(target_name, '') LIKE ? OR action LIKE ? OR module LIKE ?)"
		for i := 0; i < 3; i++ {
			args = append(args, "%"+filters.Search+"%")
		}
	}
	if filters.Start != "" {
		query += " AND occurred_at >= ?"
		args = append(args, filters.Start)
	}
	if filters.End != "" {
		query += " AND occurred_at <= ?"
		args = append(args, filters.End+" 23:59:59")
	}

	query += " ORDER BY occurred_at DESC LIMIT 10000"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []ConfigChangeLogEntry{}
	for rows.Next() {
		var entry ConfigChangeLogEntry
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Username, &entry.SourceIP, &entry.Module, &entry.TargetType, &entry.TargetID, &entry.TargetName, &entry.TargetIP, &entry.TargetMAC, &entry.Action, &entry.ChangeScope, &entry.Status, &entry.ChangeSource, &entry.RecordsetID, &entry.CorrelationID, &entry.OldValuesJSON, &entry.NewValuesJSON, &entry.DetailJSON, &entry.ReviewStatus, &entry.ReviewedBy, &entry.ReviewedAt, &entry.ReviewNote); err == nil {
			entry.OccurredAt = sqliteTimeToRFC3339(entry.OccurredAt)
			entry.ReviewedAt = sqliteTimeToRFC3339(entry.ReviewedAt)
			list = append(list, entry)
		}
	}
	return list, nil
}
