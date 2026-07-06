// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	logsmodule "management-server/modules/logs"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

// sqliteTimeToRFC3339 normalises SQLite DATETIME strings to RFC3339.
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

type SyslogEntry struct {
	ID         int     `json:"id"`
	DeviceID   *int    `json:"device_id"`
	SourceIP   *string `json:"source_ip"`
	Severity   *string `json:"severity"`
	Facility   *string `json:"facility"`
	Message    string  `json:"message"`
	ReceivedAt string  `json:"received_at"`
}

type EventEntry struct {
	ID        int    `json:"id"`
	DeviceID  *int   `json:"device_id"`
	EventType string `json:"event_type"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type SystemLogEntry struct {
	ID           int    `json:"id"`
	OccurredAt   string `json:"occurred_at"`
	Level        string `json:"level"`
	Service      string `json:"service"`
	EventCode    string `json:"event_code"`
	Message      string `json:"message"`
	ContextJSON  string `json:"context_json"`
	NodeName     string `json:"node_name"`
	BuildVersion string `json:"build_version"`
	ReviewStatus string `json:"review_status"`
	ReviewedBy   string `json:"reviewed_by"`
	ReviewedAt   string `json:"reviewed_at"`
	ReviewNote   string `json:"review_note"`
}

type DeviceLogEntry struct {
	ID                int    `json:"id"`
	OccurredAt        string `json:"occurred_at"`
	DeviceID          *int   `json:"device_id"`
	DeviceName        string `json:"device_name"`
	IPAddress         string `json:"ip_address"`
	MACAddress        string `json:"mac_address"`
	Facility          string `json:"facility"`
	Severity          string `json:"severity"`
	RawMessage        string `json:"raw_message"`
	NormalizedMessage string `json:"normalized_message"`
	MatchedRule       string `json:"matched_rule"`
	AckStatus         string `json:"ack_status"`
	ContextJSON       string `json:"context_json"`
	ReviewStatus      string `json:"review_status"`
	ReviewedBy        string `json:"reviewed_by"`
	ReviewedAt        string `json:"reviewed_at"`
	ReviewNote        string `json:"review_note"`
}

type ConfigChangeLogEntry struct {
	ID            int    `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	Username      string `json:"username"`
	SourceIP      string `json:"source_ip"`
	Module        string `json:"module"`
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	TargetName    string `json:"target_name"`
	TargetIP      string `json:"target_ip"`
	TargetMAC     string `json:"target_mac"`
	Action        string `json:"action"`
	ChangeScope   string `json:"change_scope"`
	Status        string `json:"status"`
	ChangeSource  string `json:"change_source"`
	RecordsetID   string `json:"recordset_id"`
	CorrelationID string `json:"correlation_id"`
	OldValuesJSON string `json:"old_values_json"`
	NewValuesJSON string `json:"new_values_json"`
	DetailJSON    string `json:"detail_json"`
	ReviewStatus  string `json:"review_status"`
	ReviewedBy    string `json:"reviewed_by"`
	ReviewedAt    string `json:"reviewed_at"`
	ReviewNote    string `json:"review_note"`
}

type LogReviewRequest struct {
	ReviewStatus string `json:"review_status"`
	ReviewNote   string `json:"review_note"`
}

type DeviceAckRequest struct {
	AckStatus  string `json:"ack_status"`
	ReviewNote string `json:"review_note"`
}

type logCenterFilters struct {
	Severity string `json:"severity"`
	Search   string `json:"search"`
	Scope    string `json:"scope"`
	Actor    string `json:"actor"`
	Device   string `json:"device"`
	Status   string `json:"status"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

func cleanLogExportText(s string) string {
	return strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return -1
		}
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
}

func marshalLogContext(context map[string]interface{}) string {
	if len(context) == 0 {
		return ""
	}
	b, err := json.Marshal(redactAuditDetails(context))
	if err != nil {
		return ""
	}
	return string(b)
}

func (h *Handler) WriteSystemLog(level, service, eventCode, message string, context map[string]interface{}) {
	_, _ = h.db.Exec(`
		INSERT INTO system_logs (occurred_at, level, service, event_code, message, context_json, node_name, build_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		time.Now().Format("2006-01-02 15:04:05"),
		firstNonEmpty(strings.ToLower(strings.TrimSpace(level)), "info"),
		service,
		eventCode,
		message,
		marshalLogContext(context),
		h.config.System.Name,
		h.config.System.Version,
	)
}

func (h *Handler) WriteDeviceLog(deviceID int, severity, facility, rawMessage, normalizedMessage string, context map[string]interface{}) {
	details := h.deviceAuditDetails(deviceID)
	deviceName := stringDetail(details, "device_name")
	ipAddress := stringDetail(details, "ip_address")
	macAddress := stringDetail(details, "mac_address")

	if context == nil {
		context = map[string]interface{}{}
	}
	if deviceName != "" {
		context["device_name"] = deviceName
	}
	if ipAddress != "" {
		context["ip_address"] = ipAddress
	}
	if macAddress != "" {
		context["mac_address"] = macAddress
	}

	_, _ = h.db.Exec(`
		INSERT INTO device_logs (
			occurred_at, device_id, device_name, ip_address, mac_address, facility,
			severity, raw_message, normalized_message, matched_rule, ack_status, context_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		time.Now().Format("2006-01-02 15:04:05"),
		deviceID,
		deviceName,
		ipAddress,
		macAddress,
		firstNonEmpty(facility, "nms"),
		firstNonEmpty(strings.ToLower(strings.TrimSpace(severity)), "info"),
		rawMessage,
		firstNonEmpty(normalizedMessage, rawMessage),
		stringDetail(context, "matched_rule"),
		firstNonEmpty(stringDetail(context, "ack_status"), "unacked"),
		marshalLogContext(context),
	)
}

func parsePageLimit(c *gin.Context, defaultLimit int) (int, int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
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

// GetLogs returns legacy syslog records.
func (h *Handler) GetLogs(c *gin.Context) {
	page, limit, offset := parsePageLimit(c, 100)
	deviceID := c.Query("device_id")
	severity := c.Query("severity")
	search := c.Query("search")

	query := `SELECT id, device_id, source_ip, severity, facility, message, received_at FROM syslogs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM syslogs WHERE 1=1`
	var args []interface{}

	if deviceID != "" {
		query += " AND device_id = ?"
		countQuery += " AND device_id = ?"
		args = append(args, deviceID)
	}
	if severity != "" {
		query += " AND severity = ?"
		countQuery += " AND severity = ?"
		args = append(args, severity)
	}
	if search != "" {
		query += " AND message LIKE ?"
		countQuery += " AND message LIKE ?"
		args = append(args, "%"+search+"%")
	}

	total := countRows(h.db, countQuery, args...)
	query += " ORDER BY received_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var logs []SyslogEntry
	for rows.Next() {
		var l SyslogEntry
		var deviceID sql.NullInt64
		var sourceIP, severity, facility sql.NullString
		if err := rows.Scan(&l.ID, &deviceID, &sourceIP, &severity, &facility, &l.Message, &l.ReceivedAt); err == nil {
			if deviceID.Valid {
				id := int(deviceID.Int64)
				l.DeviceID = &id
			}
			if sourceIP.Valid {
				l.SourceIP = &sourceIP.String
			}
			if severity.Valid {
				l.Severity = &severity.String
			}
			if facility.Valid {
				l.Facility = &facility.String
			}
			l.ReceivedAt = sqliteTimeToRFC3339(l.ReceivedAt)
			logs = append(logs, l)
		}
	}

	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Data: logs, Total: total, Page: page, Limit: limit})
}

// GetEvents returns legacy system events.
func (h *Handler) GetEvents(c *gin.Context) {
	page, limit, offset := parsePageLimit(c, 100)
	deviceID := c.Query("device_id")
	eventType := c.Query("type")

	query := `SELECT id, device_id, event_type, severity, message, created_at FROM events WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM events WHERE 1=1`
	var args []interface{}

	if deviceID != "" {
		query += " AND device_id = ?"
		countQuery += " AND device_id = ?"
		args = append(args, deviceID)
	}
	if eventType != "" {
		query += " AND event_type = ?"
		countQuery += " AND event_type = ?"
		args = append(args, eventType)
	}

	total := countRows(h.db, countQuery, args...)
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var events []EventEntry
	for rows.Next() {
		var e EventEntry
		var deviceID sql.NullInt64
		if err := rows.Scan(&e.ID, &deviceID, &e.EventType, &e.Severity, &e.Message, &e.CreatedAt); err == nil {
			if deviceID.Valid {
				id := int(deviceID.Int64)
				e.DeviceID = &id
			}
			e.CreatedAt = sqliteTimeToRFC3339(e.CreatedAt)
			events = append(events, e)
		}
	}

	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Data: events, Total: total, Page: page, Limit: limit})
}

func (h *Handler) GetSystemLogs(c *gin.Context) {
	page, limit, _ := logsmodule.ParsePageLimit(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "100"), 100)
	filters := logsmodule.QueryFilters{
		Level:        strings.TrimSpace(c.Query("level")),
		Service:      strings.TrimSpace(c.Query("service")),
		ReviewStatus: strings.TrimSpace(c.Query("review_status")),
		Search:       strings.TrimSpace(c.Query("search")),
		DateFrom:     strings.TrimSpace(c.Query("date_from")),
		DateTo:       strings.TrimSpace(c.Query("date_to")),
	}

	list, total, err := h.logs.ListSystemLogs(filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if total == 0 {
		h.GetEvents(c)
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Data: list, Total: total, Page: page, Limit: limit})
}

func (h *Handler) GetDeviceLogs(c *gin.Context) {
	page, limit, _ := logsmodule.ParsePageLimit(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "100"), 100)
	filters := logsmodule.QueryFilters{
		DeviceID:     strings.TrimSpace(c.Query("device_id")),
		Device:       strings.TrimSpace(c.Query("device")),
		Severity:     strings.TrimSpace(c.Query("severity")),
		Facility:     strings.TrimSpace(c.Query("facility")),
		AckStatus:    strings.TrimSpace(c.Query("ack_status")),
		ReviewStatus: strings.TrimSpace(c.Query("review_status")),
		Search:       strings.TrimSpace(c.Query("search")),
		DateFrom:     strings.TrimSpace(c.Query("date_from")),
		DateTo:       strings.TrimSpace(c.Query("date_to")),
	}

	list, total, err := h.logs.ListDeviceLogs(filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if total == 0 {
		h.GetLogs(c)
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Data: list, Total: total, Page: page, Limit: limit})
}

func (h *Handler) GetConfigChangeLogs(c *gin.Context) {
	page, limit, _ := logsmodule.ParsePageLimit(c.DefaultQuery("page", "1"), c.DefaultQuery("limit", "100"), 100)
	filters := logsmodule.QueryFilters{
		Module:       strings.TrimSpace(c.Query("module")),
		TargetType:   strings.TrimSpace(c.Query("target_type")),
		Username:     strings.TrimSpace(c.Query("username")),
		Device:       strings.TrimSpace(c.Query("device")),
		Status:       strings.TrimSpace(c.Query("status")),
		ReviewStatus: strings.TrimSpace(c.Query("review_status")),
		Search:       strings.TrimSpace(c.Query("search")),
		DateFrom:     strings.TrimSpace(c.Query("date_from")),
		DateTo:       strings.TrimSpace(c.Query("date_to")),
	}

	list, total, err := h.logs.ListConfigChangeLogs(filters, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{Success: true, Data: list, Total: total, Page: page, Limit: limit})
}

func (h *Handler) GetLogCenterOptions(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Success: true, Data: h.logs.Options()})
}

func reviewActionName(table string) string {
	switch table {
	case "audit_logs":
		return "review_audit_log"
	case "system_logs":
		return "review_system_log"
	case "config_change_logs":
		return "review_config_change_log"
	default:
		return "review_log"
	}
}

func reviewResourceName(table string) string {
	switch table {
	case "audit_logs":
		return "audit_log"
	case "system_logs":
		return "system_log"
	case "config_change_logs":
		return "config_change_log"
	default:
		return "log"
	}
}

func (h *Handler) updateLogReview(c *gin.Context, table string) {
	var req LogReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	status := logsmodule.NormalizeReviewStatus(req.ReviewStatus)
	if status == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid review status"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	ok, err := h.logs.UpdateReview(table, id, status, req.ReviewNote, h.auditUsername(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "log not found"})
		return
	}

	resourceName := reviewResourceName(table)
	h.WriteAuditFromContext(c, reviewActionName(table), auditResource(resourceName, id), "success", map[string]interface{}{
		"module":        "logs",
		"resource_type": resourceName,
		"resource_id":   id,
		"review_status": status,
		"review_note":   strings.TrimSpace(req.ReviewNote),
		"change_source": "manual",
		"change_scope":  "review_workflow",
	})

	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) ReviewAuditLog(c *gin.Context) {
	h.updateLogReview(c, "audit_logs")
}

func (h *Handler) ReviewSystemLog(c *gin.Context) {
	h.updateLogReview(c, "system_logs")
}

func (h *Handler) ReviewConfigChangeLog(c *gin.Context) {
	h.updateLogReview(c, "config_change_logs")
}

func (h *Handler) AckDeviceLog(c *gin.Context) {
	var req DeviceAckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	ackStatus := logsmodule.NormalizeAckStatus(req.AckStatus)
	if ackStatus == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid ack status"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	ok, err := h.logs.AckDeviceLog(id, ackStatus, req.ReviewNote, h.auditUsername(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "log not found"})
		return
	}

	h.WriteAuditFromContext(c, "ack_device_log", auditResource("device_log", id), "success", map[string]interface{}{
		"module":        "logs",
		"resource_type": "device_log",
		"resource_id":   id,
		"ack_status":    ackStatus,
		"review_status": "reviewed",
		"review_note":   strings.TrimSpace(req.ReviewNote),
		"change_source": "manual",
		"change_scope":  "ack_workflow",
	})

	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) ExportLogCenter(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	logType := c.DefaultQuery("type", "system_logs")
	filters := collectLogCenterFilters(c)

	headers, records, filename, err := h.logs.ExportRows(logType, filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.csv", filename, time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()
		writer.Write(headers)
		for _, record := range records {
			if err := writer.Write(record); err != nil {
				c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
				return
			}
		}
		return
	}

	if format == "pdf" {
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.pdf", filename, time.Now().Format("20060102_150405")))
		content, err := buildLogExportPDF(filename, headers, records)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/pdf", content)
		return
	}

	if format == "json" {
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.json", filename, time.Now().Format("20060102_150405")))

		var list []map[string]string
		for _, record := range records {
			list = append(list, map[string]string{
				"id":        record[0],
				"column_1":  record[1],
				"column_2":  record[2],
				"column_3":  record[3],
				"column_4":  record[4],
				"timestamp": record[5],
			})
		}
		encoder := json.NewEncoder(c.Writer)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(map[string]interface{}{
			"type":    logType,
			"headers": headers,
			"data":    list,
		})
		return
	}

	c.JSON(http.StatusBadRequest, Response{Success: false, Error: "unsupported export format"})
}

func collectLogCenterFilters(c *gin.Context) logsmodule.LogCenterFilters {
	return logsmodule.LogCenterFilters{
		Severity: strings.TrimSpace(c.Query("severity")),
		Search:   strings.TrimSpace(c.Query("search")),
		Scope:    strings.TrimSpace(c.Query("scope")),
		Actor:    strings.TrimSpace(c.Query("actor")),
		Device:   strings.TrimSpace(c.Query("device")),
		Status:   strings.TrimSpace(c.Query("status")),
		Start:    strings.TrimSpace(c.Query("start")),
		End:      strings.TrimSpace(c.Query("end")),
	}
}

func writeBundleFile(zipWriter *zip.Writer, filename string, content []byte) (string, error) {
	fileWriter, err := zipWriter.Create(filename)
	if err != nil {
		return "", err
	}
	if _, err := fileWriter.Write(content); err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

func buildLogExportCSV(headers []string, records [][]string) ([]byte, error) {
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.Write(headers); err != nil {
		return nil, err
	}
	for _, record := range records {
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func buildLogExportPDF(title string, headers []string, records [][]string) ([]byte, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 15)
	pdf.Cell(40, 9, cleanLogExportText(title))
	pdf.Ln(11)

	widths := []float64{16, 38, 32, 38, 112, 42}
	pdf.SetFont("Arial", "B", 8)
	pdf.SetFillColor(240, 240, 240)
	for i, header := range headers {
		width := widths[min(i, len(widths)-1)]
		pdf.CellFormat(width, 7, cleanLogExportText(header), "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7)
	pdf.SetFillColor(255, 255, 255)
	for _, record := range records {
		for i, value := range record {
			width := widths[min(i, len(widths)-1)]
			text := []rune(cleanLogExportText(value))
			if len(text) > 80 {
				text = append(text[:77], '.', '.', '.')
			}
			pdf.CellFormat(width, 6, string(text), "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (h *Handler) fetchLogBundlePayload(bundleType string, filters logsmodule.LogCenterFilters) (interface{}, error) {
	switch bundleType {
	case "system_logs":
		return h.logs.FetchSystemLogsForBundle(filters)
	case "device_logs":
		return h.logs.FetchDeviceLogsForBundle(filters)
	case "audit_logs":
		return h.logs.FetchAuditLogsForBundle(filters)
	case "config_change_logs":
		return h.logs.FetchConfigChangeLogsForBundle(filters)
	default:
		return nil, fmt.Errorf("unsupported evidence bundle type")
	}
}

func logBundleRowCount(payload interface{}, records [][]string) int {
	switch list := payload.(type) {
	case []logsmodule.SystemLogEntry:
		return len(list)
	case []logsmodule.DeviceLogEntry:
		return len(list)
	case []logsmodule.AuditEntry:
		return len(list)
	case []logsmodule.ConfigChangeLogEntry:
		return len(list)
	default:
		return len(records)
	}
}

func (h *Handler) buildLogBundleFile(bundleType, format string, filters logsmodule.LogCenterFilters) (string, []byte, int, error) {
	filename := bundleType + "." + format
	if format == "json" {
		payload, err := h.fetchLogBundlePayload(bundleType, filters)
		if err != nil {
			return "", nil, 0, err
		}
		content, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", nil, 0, err
		}
		return filename, content, logBundleRowCount(payload, nil), nil
	}

	headers, records, _, err := h.logs.ExportRows(bundleType, filters)
	if err != nil {
		return "", nil, 0, err
	}

	switch format {
	case "csv":
		content, err := buildLogExportCSV(headers, records)
		return filename, content, len(records), err
	case "pdf":
		content, err := buildLogExportPDF(bundleType, headers, records)
		return filename, content, len(records), err
	default:
		return "", nil, 0, fmt.Errorf("unsupported evidence bundle format")
	}
}

func (h *Handler) ExportLogEvidenceBundle(c *gin.Context) {
	logType := strings.TrimSpace(c.DefaultQuery("type", "all"))
	bundleFormat := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "json")))
	if bundleFormat == "" {
		bundleFormat = "json"
	}
	switch bundleFormat {
	case "json", "csv", "pdf":
	default:
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "unsupported evidence bundle format"})
		return
	}

	filters := collectLogCenterFilters(c)
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	generatedBy := h.auditUsername(c)

	targetTypes := []string{}
	if logType == "" || logType == "all" {
		targetTypes = []string{"audit_logs", "system_logs", "device_logs", "config_change_logs"}
	} else {
		switch logType {
		case "audit_logs", "system_logs", "device_logs", "config_change_logs":
			targetTypes = []string{logType}
		default:
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "unsupported evidence bundle type"})
			return
		}
	}

	var bundle bytes.Buffer
	zipWriter := zip.NewWriter(&bundle)
	files := []map[string]interface{}{}

	for _, bundleType := range targetTypes {
		filename, content, rowCount, err := h.buildLogBundleFile(bundleType, bundleFormat, filters)
		if err != nil {
			_ = zipWriter.Close()
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}

		checksum, err := writeBundleFile(zipWriter, filename, content)
		if err != nil {
			_ = zipWriter.Close()
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}

		files = append(files, map[string]interface{}{
			"name":      filename,
			"type":      bundleType,
			"format":    bundleFormat,
			"row_count": rowCount,
			"sha256":    checksum,
		})
	}

	manifest := map[string]interface{}{
		"bundle_type":    "log_evidence_bundle",
		"generated_at":   generatedAt,
		"generated_by":   generatedBy,
		"requested_type": logType,
		"format":         bundleFormat,
		"filters":        filters,
		"files":          files,
	}

	manifestContent, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = zipWriter.Close()
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if _, err := writeBundleFile(zipWriter, "manifest.json", manifestContent); err != nil {
		_ = zipWriter.Close()
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	if err := zipWriter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "export_log_evidence_bundle", auditResource("log_bundle", logType), "success", map[string]interface{}{
		"module":        "logs",
		"resource_type": "log_bundle",
		"resource_id":   logType,
		"resource_name": "log_evidence_bundle",
		"bundle_type":   "log_evidence_bundle",
		"format":        bundleFormat,
		"filters":       filters,
		"file_count":    len(files),
		"change_source": "manual",
		"change_scope":  "export_workflow",
	})

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=log_evidence_bundle_%s_%s_%s.zip", logType, bundleFormat, time.Now().Format("20060102_150405")))
	c.Data(http.StatusOK, "application/zip", bundle.Bytes())
}
