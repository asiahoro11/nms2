package reports

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func cleanString(s string) string {
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

func reportFormat(c *gin.Context) string {
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	if format == "" {
		return "csv"
	}
	return format
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullInt64Value(value sql.NullInt64) int64 {
	if value.Valid {
		return value.Int64
	}
	return 0
}

// ExportDevicesReportPDF exports the device report as PDF.
func (s *Service) ExportDevicesReportPDF(c *gin.Context) {
	rt := newReportTranslator(c)
	rows, err := s.db.Query(`
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			device_type,
			COALESCE(vendor, '') as vendor,
			COALESCE(model, '') as model,
			COALESCE(firmware, '') as firmware,
			COALESCE(sys_location, '') as sys_location,
			is_online,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id) as interface_count,
			(SELECT COALESCE(SUM(bandwidth_in + bandwidth_out), 0) FROM device_interfaces WHERE device_id = devices.id) as total_bps
		FROM devices ORDER BY device_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	pdf, fontFamily, unicodeFont := newReportPDF("L")
	pdf.AddPage()
	pdf.SetFont(fontFamily, "B", 16)
	pdf.Cell(40, 10, reportPDFText(rt.T("title.device_list"), unicodeFont))
	pdf.Ln(12)

	pdf.SetFont(fontFamily, "B", 12)
	pdf.SetFillColor(240, 240, 240)

	// Table Header
	headers := rt.Headers("col.name", "col.ip", "col.type", "col.vendor", "col.model", "col.firmware", "col.location", "col.status", "col.interface_count", "col.total_in_bps")
	widths := []float64{42, 28, 22, 23, 27, 28, 30, 20, 10, 25}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	// Table Body
	pdf.SetFont(fontFamily, "", 10)
	pdf.SetFillColor(255, 255, 255)

	for rows.Next() {
		var name, ipAddress, deviceType, vendor, model, firmware, location string
		var isOnline bool
		var interfaceCount, totalBps int64

		if err := rows.Scan(&name, &ipAddress, &deviceType, &vendor, &model, &firmware, &location, &isOnline, &interfaceCount, &totalBps); err == nil {
			status := rt.T("value.offline")
			if isOnline {
				status = rt.T("value.online")
			}

			pdf.CellFormat(widths[0], 8, reportPDFText(name, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[1], 8, reportPDFText(ipAddress, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[2], 8, reportPDFText(deviceType, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[3], 8, reportPDFText(vendor, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[4], 8, reportPDFText(model, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[5], 8, reportPDFText(firmware, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[6], 8, reportPDFText(location, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[7], 8, reportPDFText(status, unicodeFont), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[8], 8, fmt.Sprintf("%d", interfaceCount), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[9], 8, fmt.Sprintf("%d", totalBps), "1", 0, "", false, 0, "")
			pdf.Ln(-1)
		}
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=devices_%s.pdf", time.Now().Format("20060102_150405")))

	err = pdf.Output(c.Writer)
	if err != nil {
		// Log error but header already sent
		fmt.Println("PDF Output Error:", err)
	}
}

// ExportLogsReportPDF exports the log report as PDF.
func (s *Service) ExportLogsReportPDF(c *gin.Context) {
	rt := newReportTranslator(c)
	startDate := c.Query("start")
	endDate := c.Query("end")
	severity := c.Query("severity")
	logType := c.Query("type")

	var query string
	var args []interface{}

	if logType == "events" {
		query = `SELECT event_type, severity, message, created_at FROM events WHERE 1=1`
	} else {
		query = `SELECT source_ip, severity, message, received_at FROM syslogs WHERE 1=1`
	}

	if startDate != "" {
		if logType == "events" {
			query += " AND created_at >= ?"
		} else {
			query += " AND received_at >= ?"
		}
		args = append(args, startDate)
	}
	if endDate != "" {
		if logType == "events" {
			query += " AND created_at <= ?"
		} else {
			query += " AND received_at <= ?"
		}
		args = append(args, endDate)
	}
	if severity != "" {
		query += " AND severity = ?"
		args = append(args, severity)
	}

	if logType == "events" {
		query += " ORDER BY created_at DESC LIMIT 1000"
	} else {
		query += " ORDER BY received_at DESC LIMIT 1000"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	pdf, fontFamily, unicodeFont := newReportPDF("L")
	pdf.AddPage()
	pdf.SetFont(fontFamily, "B", 16)
	pdf.Cell(40, 10, reportPDFText(rt.T("title.system_logs"), unicodeFont))
	pdf.Ln(12)

	pdf.SetFont(fontFamily, "B", 12)
	pdf.SetFillColor(240, 240, 240)

	// Table Header
	headers := rt.Headers("col.time", "col.severity", "col.source_type", "col.message")
	widths := []float64{50, 25, 45, 150}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	// Table Body
	pdf.SetFont(fontFamily, "", 10)
	pdf.SetFillColor(255, 255, 255)

	for rows.Next() {
		var col1, col2, col3, col4 string
		// For syslogs: source_ip, severity, message, received_at
		// For events: event_type, severity, message, created_at
		// Start Query order:
		// Syslogs: source_ip(1), severity(2), message(3), received_at(4)
		// Events:  event_type(1), severity(2), message(3), created_at(4)

		var val1, val2, val3, val4 sql.NullString
		if err := rows.Scan(&val1, &val2, &val3, &val4); err != nil {
			continue
		}

		if logType == "events" {
			// val1=type, val2=severity, val3=msg, val4=time
			col1 = val4.String
			col2 = val2.String
			col3 = val1.String
			col4 = val3.String
		} else {
			// val1=ip, val2=severity, val3=msg, val4=time
			col1 = val4.String
			col2 = val2.String
			col3 = val1.String
			col4 = val3.String
		}

		// Clean message (basic cleanup)
		cleanMsg := col4
		if len(cleanMsg) > 80 {
			cleanMsg = cleanMsg[:77] + "..."
		}

		pdf.CellFormat(widths[0], 8, reportPDFText(col1, unicodeFont), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[1], 8, reportPDFText(col2, unicodeFont), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[2], 8, reportPDFText(rt.DisplayValue(col3), unicodeFont), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[3], 8, reportPDFText(cleanMsg, unicodeFont), "1", 0, "", false, 0, "")
		pdf.Ln(-1)
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=logs_%s.pdf", time.Now().Format("20060102_150405")))

	err = pdf.Output(c.Writer)
	if err != nil {
		fmt.Println("PDF Output Error:", err)
	}
}

// ExportDevicesReport exports the device report.
func (s *Service) ExportDevicesReport(c *gin.Context) {
	format := reportFormat(c)
	if format == "pdf" {
		s.ExportDevicesReportPDF(c)
		return
	}
	rt := newReportTranslator(c)

	rows, err := s.db.Query(`
		SELECT 
			id, 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			COALESCE(sys_name, '') as sys_name,
			ip_address, 
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
			device_type, 
			COALESCE(vendor, '') as vendor,
			COALESCE(model, '') as model,
			COALESCE(firmware, '') as firmware,
			COALESCE(sys_location, '') as sys_location,
			COALESCE(sys_uptime, '') as sys_uptime,
			is_online, 
			last_seen, 
			created_at,
			updated_at,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id) as interface_count,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id AND if_status = 1) as up_interface_count,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id AND poe_enabled = 1) as poe_port_count,
			(SELECT COALESCE(SUM(bandwidth_in), 0) FROM device_interfaces WHERE device_id = devices.id) as total_bandwidth_in,
			(SELECT COALESCE(SUM(bandwidth_out), 0) FROM device_interfaces WHERE device_id = devices.id) as total_bandwidth_out
		FROM devices ORDER BY device_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=devices_%s.csv", time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// 撖怠璅?
		writer.Write(rt.Headers(
			"col.id", "col.name", "col.sys_name", "col.ip_address", "col.mac_address", "col.type", "col.vendor", "col.model", "col.firmware",
			"col.location", "col.snmp_uptime", "col.status", "col.last_seen", "col.interface_count", "col.up_interfaces", "col.poe_ports",
			"col.total_in_bps", "col.total_out_bps", "col.created_at", "col.updated_at",
		))

		// 撖怠鞈?
		for rows.Next() {
			var id int
			var name, sysName, ipAddress, deviceType, vendor, model, firmware, location, sysUptime, createdAt, updatedAt string
			var macAddress, lastSeen sql.NullString
			var isOnline bool
			var interfaceCount, upInterfaceCount, poePortCount, totalIn, totalOut sql.NullInt64

			if err := rows.Scan(&id, &name, &sysName, &ipAddress, &macAddress, &deviceType, &vendor, &model, &firmware, &location, &sysUptime, &isOnline, &lastSeen, &createdAt, &updatedAt, &interfaceCount, &upInterfaceCount, &poePortCount, &totalIn, &totalOut); err == nil {
				status := rt.T("value.offline")
				if isOnline {
					status = rt.T("value.online")
				}

				writer.Write([]string{
					fmt.Sprintf("%d", id),
					name,
					sysName,
					ipAddress,
					nullStringValue(macAddress),
					deviceType,
					vendor,
					model,
					firmware,
					location,
					sysUptime,
					status,
					nullStringValue(lastSeen),
					fmt.Sprintf("%d", nullInt64Value(interfaceCount)),
					fmt.Sprintf("%d", nullInt64Value(upInterfaceCount)),
					fmt.Sprintf("%d", nullInt64Value(poePortCount)),
					fmt.Sprintf("%d", nullInt64Value(totalIn)),
					fmt.Sprintf("%d", nullInt64Value(totalOut)),
					createdAt,
					updatedAt,
				})
			}
		}
		return
	}

	// JSON ?澆?
	var devices []map[string]interface{}
	rows2, err := s.db.Query(`
		SELECT 
			id, 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			COALESCE(sys_name, '') as sys_name,
			ip_address, 
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
			device_type, 
			COALESCE(vendor, '') as vendor,
			COALESCE(model, '') as model,
			COALESCE(firmware, '') as firmware,
			COALESCE(sys_location, '') as sys_location,
			COALESCE(sys_uptime, '') as sys_uptime,
			is_online, 
			last_seen, 
			created_at,
			updated_at,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id) as interface_count,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id AND if_status = 1) as up_interface_count,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id AND poe_enabled = 1) as poe_port_count,
			(SELECT COALESCE(SUM(bandwidth_in), 0) FROM device_interfaces WHERE device_id = devices.id) as total_bandwidth_in,
			(SELECT COALESCE(SUM(bandwidth_out), 0) FROM device_interfaces WHERE device_id = devices.id) as total_bandwidth_out
		FROM devices ORDER BY device_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var id int
		var name, sysName, ipAddress, deviceType, vendor, model, firmware, location, sysUptime, createdAt, updatedAt string
		var macAddress, lastSeen sql.NullString
		var isOnline bool
		var interfaceCount, upInterfaceCount, poePortCount, totalIn, totalOut sql.NullInt64

		if err := rows2.Scan(&id, &name, &sysName, &ipAddress, &macAddress, &deviceType, &vendor, &model, &firmware, &location, &sysUptime, &isOnline, &lastSeen, &createdAt, &updatedAt, &interfaceCount, &upInterfaceCount, &poePortCount, &totalIn, &totalOut); err == nil {
			devices = append(devices, map[string]interface{}{
				"id":                  id,
				"name":                name,
				"sys_name":            sysName,
				"ip_address":          ipAddress,
				"mac_address":         nullStringValue(macAddress),
				"device_type":         deviceType,
				"vendor":              vendor,
				"model":               model,
				"firmware":            firmware,
				"sys_location":        location,
				"sys_uptime":          sysUptime,
				"is_online":           isOnline,
				"last_seen":           nullStringValue(lastSeen),
				"interface_count":     nullInt64Value(interfaceCount),
				"up_interface_count":  nullInt64Value(upInterfaceCount),
				"poe_port_count":      nullInt64Value(poePortCount),
				"total_bandwidth_in":  nullInt64Value(totalIn),
				"total_bandwidth_out": nullInt64Value(totalOut),
				"created_at":          createdAt,
				"updated_at":          updatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

// ExportLogsReport exports the log report.
func (s *Service) ExportLogsReport(c *gin.Context) {
	format := reportFormat(c)
	if format == "pdf" {
		s.ExportLogsReportPDF(c)
		return
	}
	rt := newReportTranslator(c)
	logType := c.DefaultQuery("type", "events") // Default to events
	startDate := c.Query("start")
	endDate := c.Query("end")
	severity := c.Query("severity")

	var query string
	var args []interface{}

	if logType == "events" {
		query = `SELECT id, CAST(NULL as TEXT) as source_ip, severity, CAST(NULL as TEXT) as facility, message, created_at as time_val FROM events WHERE 1=1`
	} else {
		query = `SELECT id, source_ip, severity, facility, message, received_at as time_val FROM syslogs WHERE 1=1`
	}

	if startDate != "" {
		if logType == "events" {
			query += " AND created_at >= ?"
		} else {
			query += " AND received_at >= ?"
		}
		args = append(args, startDate)
	}
	if endDate != "" {
		if logType == "events" {
			query += " AND created_at <= ?"
		} else {
			query += " AND received_at <= ?"
		}
		args = append(args, endDate)
	}
	if severity != "" {
		query += " AND severity = ?"
		args = append(args, severity)
	}

	if logType == "events" {
		query += " ORDER BY created_at DESC LIMIT 10000"
	} else {
		query += " ORDER BY received_at DESC LIMIT 10000"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		filename := "syslogs"
		if logType == "events" {
			filename = "events"
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.csv", filename, time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// 撖怠璅?
		writer.Write(rt.Headers("col.id", "col.source_type", "col.severity", "col.message", "col.time"))

		// 撖怠鞈?
		for rows.Next() {
			var id int
			var message, timeVal string
			var sourceIP, severity, facility *string

			if err := rows.Scan(&id, &sourceIP, &severity, &facility, &message, &timeVal); err == nil {
				src := rt.T("value.system")
				if sourceIP != nil && *sourceIP != "" {
					src = *sourceIP
				}
				sev := ""
				if severity != nil {
					sev = *severity
				}

				writer.Write([]string{
					fmt.Sprintf("%d", id),
					src,
					sev,
					message,
					timeVal,
				})
			}
		}
		return
	}

	// JSON ?澆?
	var logs []map[string]interface{}
	rows2, err := s.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var id int
		var message, timeVal string
		var sourceIP, severity, facility *string

		if err := rows2.Scan(&id, &sourceIP, &severity, &facility, &message, &timeVal); err == nil {
			logs = append(logs, map[string]interface{}{
				"id":       id,
				"source":   sourceIP,
				"severity": severity,
				"message":  message,
				"time":     timeVal,
			})
		}
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: logs})
}

// ExportTopTrafficReport exports the top interface traffic report.
func (s *Service) ExportTopTrafficReport(c *gin.Context) {
	format := reportFormat(c)
	rt := newReportTranslator(c)
	limitStr := c.DefaultQuery("limit", "10")

	// Convert limit to int to be safe with all drivers
	var limit int
	fmt.Sscanf(limitStr, "%d", &limit)
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT 
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name,
			d.ip_address, 
			COALESCE(di.if_name, '') as if_name, 
			COALESCE(di.bandwidth_in, 0) as bandwidth_in, 
			COALESCE(di.bandwidth_out, 0) as bandwidth_out, 
			(COALESCE(di.bandwidth_in, 0) + COALESCE(di.bandwidth_out, 0)) as total
		FROM device_interfaces di
		JOIN devices d ON di.device_id = d.id
		ORDER BY total DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "pdf" {
		pdf, fontFamily, unicodeFont := newReportPDF("P")
		pdf.AddPage()
		pdf.SetFont(fontFamily, "B", 16)
		pdf.Cell(40, 10, reportPDFText(fmt.Sprintf(rt.T("title.top_traffic"), limit), unicodeFont))
		pdf.Ln(12)

		pdf.SetFont(fontFamily, "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := rt.Headers("col.device", "col.ip", "col.interface", "col.total_in_bps", "col.total_out_bps", "col.total_bps")
		widths := []float64{40, 35, 40, 25, 25, 25}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont(fontFamily, "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, ifName string
			var in, out, total int64
			if err := rows.Scan(&devName, &devIP, &ifName, &in, &out, &total); err == nil {
				pdf.CellFormat(widths[0], 8, reportPDFText(devName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, devIP, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, reportPDFText(ifName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, fmt.Sprintf("%d", in), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, fmt.Sprintf("%d", out), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 8, fmt.Sprintf("%d", total), "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=top_traffic_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=top_traffic_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write(rt.Headers("col.device_name", "col.ip_address", "col.interface", "col.total_in_bps", "col.total_out_bps", "col.total_bps"))

	for rows.Next() {
		var devName, devIP, ifName string
		var in, out, total int64
		if err := rows.Scan(&devName, &devIP, &ifName, &in, &out, &total); err == nil {
			writer.Write([]string{
				devName, devIP, ifName,
				fmt.Sprintf("%d", in),
				fmt.Sprintf("%d", out),
				fmt.Sprintf("%d", total),
			})
		}
	}
}

// ExportDeviceHealthReport exports device health metrics.
func (s *Service) ExportDeviceHealthReport(c *gin.Context) {
	format := reportFormat(c)
	rt := newReportTranslator(c)

	query := `
		SELECT 
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name, 
			d.ip_address, 
			COALESCE(m.cpu_usage, 0) as cpu_usage,
			COALESCE(m.memory_usage, 0) as memory_usage,
			COALESCE(m.disk_usage, 0) as disk_usage,
			m.collected_at
		FROM devices d
		JOIN device_metrics m ON d.id = m.device_id
		JOIN (
			SELECT device_id, MAX(collected_at) as max_at 
			FROM device_metrics 
			GROUP BY device_id
		) max_m ON m.device_id = max_m.device_id AND m.collected_at = max_m.max_at
		ORDER BY m.cpu_usage DESC
		LIMIT 20
	`
	rows, err := s.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "pdf" {
		pdf, fontFamily, unicodeFont := newReportPDF("P")
		pdf.AddPage()
		pdf.SetFont(fontFamily, "B", 16)
		pdf.Cell(40, 10, reportPDFText(rt.T("title.device_health"), unicodeFont))
		pdf.Ln(12)

		pdf.SetFont(fontFamily, "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := rt.Headers("col.device", "col.ip", "col.cpu_percent", "col.mem_percent", "col.disk_percent", "col.last_check")
		widths := []float64{50, 38, 20, 20, 20, 35}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont(fontFamily, "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, createdAt string
			var cpu, mem, disk float64
			if err := rows.Scan(&devName, &devIP, &cpu, &mem, &disk, &createdAt); err == nil {
				pdf.CellFormat(widths[0], 8, reportPDFText(devName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, devIP, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, fmt.Sprintf("%.1f", cpu), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, fmt.Sprintf("%.1f", mem), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, fmt.Sprintf("%.1f", disk), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 8, createdAt, "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=health_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=health_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write(rt.Headers("col.device_name", "col.ip_address", "col.cpu_percent", "col.mem_percent", "col.disk_percent", "col.last_check"))

	for rows.Next() {
		var devName, devIP, createdAt string
		var cpu, mem, disk float64
		if err := rows.Scan(&devName, &devIP, &cpu, &mem, &disk, &createdAt); err == nil {
			writer.Write([]string{
				devName, devIP,
				fmt.Sprintf("%.2f", cpu),
				fmt.Sprintf("%.2f", mem),
				fmt.Sprintf("%.2f", disk),
				createdAt,
			})
		}
	}
}

// ExportAvailabilityReport exports device availability.
func (s *Service) ExportAvailabilityReport(c *gin.Context) {
	format := reportFormat(c)
	rt := newReportTranslator(c)

	query := `
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			is_online, 
			last_seen, 
			COALESCE(sys_uptime, '') as sys_uptime,
			created_at
		FROM devices ORDER BY device_name
	`
	rows, err := s.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "pdf" {
		pdf, fontFamily, unicodeFont := newReportPDF("P")
		pdf.AddPage()
		pdf.SetFont(fontFamily, "B", 16)
		pdf.Cell(40, 10, reportPDFText(rt.T("title.availability"), unicodeFont))
		pdf.Ln(12)

		pdf.SetFont(fontFamily, "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := rt.Headers("col.device", "col.ip_address", "col.status", "col.last_seen", "col.snmp_uptime", "col.uptime_estimation")
		widths := []float64{42, 32, 20, 36, 32, 22}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont(fontFamily, "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, sysUptime, createdAt string
			var lastSeen *string
			var isOnline bool
			if err := rows.Scan(&devName, &devIP, &isOnline, &lastSeen, &sysUptime, &createdAt); err == nil {
				status := rt.T("value.offline")
				if isOnline {
					status = rt.T("value.online")
				}
				ls := rt.T("value.never")
				if lastSeen != nil {
					ls = *lastSeen
				}

				// Keep the legacy estimated uptime behavior unchanged.
				uptime := "0%"
				if isOnline {
					uptime = "100%"
				}

				pdf.CellFormat(widths[0], 8, reportPDFText(devName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, reportPDFText(devIP, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, reportPDFText(status, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, reportPDFText(ls, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, reportPDFText(sysUptime, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 8, reportPDFText(uptime, unicodeFont), "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=availability_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=availability_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write(rt.Headers("col.device_name", "col.ip_address", "col.status", "col.last_seen", "col.snmp_uptime", "col.uptime_estimation"))

	for rows.Next() {
		var devName, devIP, sysUptime, createdAt string
		var lastSeen *string
		var isOnline bool
		if err := rows.Scan(&devName, &devIP, &isOnline, &lastSeen, &sysUptime, &createdAt); err == nil {
			status := rt.T("value.offline")
			if isOnline {
				status = rt.T("value.online")
			}
			ls := rt.T("value.never")
			if lastSeen != nil {
				ls = *lastSeen
			}
			uptime := "0%"
			if isOnline {
				uptime = "100%"
			}
			writer.Write([]string{devName, devIP, status, ls, sysUptime, uptime})
		}
	}
}

// ExportInventoryReport exports the network asset inventory.
func (s *Service) ExportInventoryReport(c *gin.Context) {
	format := reportFormat(c)
	rt := newReportTranslator(c)
	query := `
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			COALESCE(sys_name, '') as sys_name,
			ip_address, 
			COALESCE(vendor, '') as vendor,
			COALESCE(model, '') as model,
			COALESCE(firmware, '') as firmware,
			device_type, 
			COALESCE(sys_location, '') as sys_location,
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
			(SELECT COUNT(*) FROM device_interfaces WHERE device_id = devices.id) as interface_count,
			created_at
		FROM devices ORDER BY device_name
	`
	rows, err := s.db.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "pdf" {
		pdf, fontFamily, unicodeFont := newReportPDF("L")
		pdf.AddPage()
		pdf.SetFont(fontFamily, "B", 16)
		pdf.Cell(40, 10, reportPDFText(rt.T("title.inventory"), unicodeFont))
		pdf.Ln(12)

		pdf.SetFont(fontFamily, "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := rt.Headers("col.device", "col.sys_name", "col.ip", "col.vendor", "col.model", "col.firmware", "col.type", "col.location", "col.interface_count", "col.mac_address")
		widths := []float64{34, 30, 28, 22, 30, 28, 22, 30, 10, 32}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, reportPDFText(h, unicodeFont), "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont(fontFamily, "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, sysName, devIP, vendor, model, firmware, devType, location, createdAt string
			var mac *string
			var interfaceCount int64
			if err := rows.Scan(&devName, &sysName, &devIP, &vendor, &model, &firmware, &devType, &location, &mac, &interfaceCount, &createdAt); err == nil {
				ma := ""
				if mac != nil {
					ma = *mac
				}

				pdf.CellFormat(widths[0], 8, reportPDFText(devName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, reportPDFText(sysName, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, reportPDFText(devIP, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, reportPDFText(vendor, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, reportPDFText(model, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 8, reportPDFText(firmware, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[6], 8, reportPDFText(devType, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[7], 8, reportPDFText(location, unicodeFont), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[8], 8, fmt.Sprintf("%d", interfaceCount), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[9], 8, reportPDFText(ma, unicodeFont), "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=inventory_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=inventory_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write(rt.Headers("col.device_name", "col.sys_name", "col.ip_address", "col.vendor", "col.model", "col.firmware", "col.type", "col.location", "col.mac_address", "col.interface_count", "col.added_date"))

	for rows.Next() {
		var devName, sysName, devIP, vendor, model, firmware, devType, location, createdAt string
		var mac *string
		var interfaceCount int64
		if err := rows.Scan(&devName, &sysName, &devIP, &vendor, &model, &firmware, &devType, &location, &mac, &interfaceCount, &createdAt); err == nil {
			ma := ""
			if mac != nil {
				ma = *mac
			}

			writer.Write([]string{devName, sysName, devIP, vendor, model, firmware, devType, location, ma, fmt.Sprintf("%d", interfaceCount), createdAt})
		}
	}
}
