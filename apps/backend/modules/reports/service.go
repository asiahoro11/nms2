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
	"github.com/jung-kurt/gofpdf"
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

// ExportDevicesReportPDF exports the device report as PDF.
func (s *Service) ExportDevicesReportPDF(c *gin.Context) {
	rows, err := s.db.Query(`
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			device_type, 
			is_online, 
			last_seen 
		FROM devices ORDER BY device_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Device List Report")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 12)
	pdf.SetFillColor(240, 240, 240)

	// Table Header
	headers := []string{"Name", "IP Address", "Type", "Status", "Last Seen"}
	widths := []float64{60, 40, 40, 30, 60}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	// Table Body
	pdf.SetFont("Arial", "", 10)
	pdf.SetFillColor(255, 255, 255)

	for rows.Next() {
		var name, ipAddress, deviceType string
		var lastSeen *string
		var isOnline bool

		if err := rows.Scan(&name, &ipAddress, &deviceType, &isOnline, &lastSeen); err == nil {
			status := "Offline"
			if isOnline {
				status = "Online"
			}
			seen := ""
			if lastSeen != nil {
				seen = *lastSeen
			}

			pdf.CellFormat(widths[0], 8, cleanString(name), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[1], 8, cleanString(ipAddress), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[2], 8, cleanString(deviceType), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[3], 8, cleanString(status), "1", 0, "", false, 0, "")
			pdf.CellFormat(widths[4], 8, cleanString(seen), "1", 0, "", false, 0, "")
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

	pdf := gofpdf.New("L", "mm", "A4", "") // Landscape
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "System Log Report")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 12)
	pdf.SetFillColor(240, 240, 240)

	// Table Header
	headers := []string{"Time", "Severity", "Source/Type", "Message"}
	widths := []float64{50, 25, 45, 150}

	for i, h := range headers {
		pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	// Table Body
	pdf.SetFont("Arial", "", 10)
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

		pdf.CellFormat(widths[0], 8, cleanString(col1), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[1], 8, cleanString(col2), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[2], 8, cleanString(col3), "1", 0, "", false, 0, "")
		pdf.CellFormat(widths[3], 8, cleanString(cleanMsg), "1", 0, "", false, 0, "")
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
	format := c.DefaultQuery("format", "csv")

	rows, err := s.db.Query(`
		SELECT 
			id, 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
			device_type, 
			vendor, 
			model, 
			is_online, 
			last_seen, 
			created_at 
		FROM devices ORDER BY device_name
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "csv" {
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=devices_%s.csv", time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// 撖怠璅?
		writer.Write([]string{"ID", "Name", "IP Address", "MAC Address", "Type", "Vendor", "Model", "Status", "Last Seen", "Created At"})

		// 撖怠鞈?
		for rows.Next() {
			var id int
			var name, ipAddress, deviceType, createdAt string
			var macAddress, vendor, model, lastSeen *string
			var isOnline bool

			if err := rows.Scan(&id, &name, &ipAddress, &macAddress, &deviceType, &vendor, &model, &isOnline, &lastSeen, &createdAt); err == nil {
				status := "Offline"
				if isOnline {
					status = "Online"
				}

				mac := ""
				if macAddress != nil {
					mac = *macAddress
				}
				vend := ""
				if vendor != nil {
					vend = *vendor
				}
				mod := ""
				if model != nil {
					mod = *model
				}
				ls := ""
				if lastSeen != nil {
					ls = *lastSeen
				}

				writer.Write([]string{
					fmt.Sprintf("%d", id),
					name,
					ipAddress,
					mac,
					deviceType,
					vend,
					mod,
					status,
					ls,
					createdAt,
				})
			}
		}
		return
	}

	// JSON ?澆?
	var devices []map[string]interface{}
	rows2, _ := s.db.Query(`
		SELECT 
			id, 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
			device_type, 
			vendor, 
			model, 
			is_online, 
			last_seen, 
			created_at 
		FROM devices ORDER BY device_name
	`)
	defer rows2.Close()

	for rows2.Next() {
		var id int
		var name, ipAddress, deviceType, createdAt string
		var macAddress, vendor, model, lastSeen *string
		var isOnline bool

		if err := rows2.Scan(&id, &name, &ipAddress, &macAddress, &deviceType, &vendor, &model, &isOnline, &lastSeen, &createdAt); err == nil {
			devices = append(devices, map[string]interface{}{
				"id":          id,
				"name":        name,
				"ip_address":  ipAddress,
				"mac_address": macAddress,
				"device_type": deviceType,
				"vendor":      vendor,
				"model":       model,
				"is_online":   isOnline,
				"last_seen":   lastSeen,
				"created_at":  createdAt,
			})
		}
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

// ExportLogsReport exports the log report.
func (s *Service) ExportLogsReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
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
		c.Header("Content-Type", "text/csv")
		filename := "syslogs"
		if logType == "events" {
			filename = "events"
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.csv", filename, time.Now().Format("20060102_150405")))

		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// 撖怠璅?
		writer.Write([]string{"ID", "Source/Type", "Severity", "Message", "Time"})

		// 撖怠鞈?
		for rows.Next() {
			var id int
			var message, timeVal string
			var sourceIP, severity, facility *string

			if err := rows.Scan(&id, &sourceIP, &severity, &facility, &message, &timeVal); err == nil {
				src := "System"
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
	rows2, _ := s.db.Query(query, args...)
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
	format := c.DefaultQuery("format", "csv")
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
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, fmt.Sprintf("Top %d Interface Traffic Report", limit))
		pdf.Ln(12)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := []string{"Device", "IP", "Interface", "In (bps)", "Out (bps)", "Total (bps)"}
		widths := []float64{40, 35, 40, 25, 25, 25}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Arial", "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, ifName string
			var in, out, total int64
			if err := rows.Scan(&devName, &devIP, &ifName, &in, &out, &total); err == nil {
				// gofpdf handles ASCII more reliably for generated tables.
				devName = cleanString(devName)
				ifName = cleanString(ifName)

				pdf.CellFormat(widths[0], 8, devName, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, devIP, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, ifName, "1", 0, "", false, 0, "")
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
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=top_traffic_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"Device Name", "IP Address", "Interface", "Inbound (bps)", "Outbound (bps)", "Total (bps)"})

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
	format := c.DefaultQuery("format", "csv")

	query := `
		SELECT 
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name, 
			d.ip_address, 
			m.cpu_usage, 
			m.memory_usage, 
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
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, "Device Health Report (Top CPU)")
		pdf.Ln(12)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := []string{"Device", "IP", "CPU (%)", "Mem (%)", "Last Check"}
		widths := []float64{60, 50, 25, 25, 40}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Arial", "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, createdAt string
			var cpu, mem float64
			if err := rows.Scan(&devName, &devIP, &cpu, &mem, &createdAt); err == nil {
				pdf.CellFormat(widths[0], 8, devName, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, devIP, "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, fmt.Sprintf("%.1f", cpu), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, fmt.Sprintf("%.1f", mem), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, createdAt, "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=health_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=health_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"Device Name", "IP Address", "CPU Usage", "Memory Usage", "Last Check"})

	for rows.Next() {
		var devName, devIP, createdAt string
		var cpu, mem float64
		if err := rows.Scan(&devName, &devIP, &cpu, &mem, &createdAt); err == nil {
			writer.Write([]string{
				devName, devIP,
				fmt.Sprintf("%.2f", cpu),
				fmt.Sprintf("%.2f", mem),
				createdAt,
			})
		}
	}
}

// ExportAvailabilityReport exports device availability.
func (s *Service) ExportAvailabilityReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")

	query := `
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			is_online, 
			last_seen, 
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
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, "Device Availability Report")
		pdf.Ln(12)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := []string{"Device", "IP Address", "Status", "Last Seen", "Uptime Estim."}
		widths := []float64{50, 40, 25, 45, 30}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Arial", "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, createdAt string
			var lastSeen *string
			var isOnline bool
			if err := rows.Scan(&devName, &devIP, &isOnline, &lastSeen, &createdAt); err == nil {
				status := "Offline"
				if isOnline {
					status = "Online"
				}
				ls := "Never"
				if lastSeen != nil {
					ls = *lastSeen
				}

				// Keep the legacy estimated uptime behavior unchanged.
				uptime := "0%"
				if isOnline {
					uptime = "100%"
				}

				pdf.CellFormat(widths[0], 8, cleanString(devName), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, cleanString(devIP), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, cleanString(status), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, cleanString(ls), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, cleanString(uptime), "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=availability_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=availability_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"Device Name", "IP Address", "Status", "Last Seen", "Uptime Estimation"})

	for rows.Next() {
		var devName, devIP, createdAt string
		var lastSeen *string
		var isOnline bool
		if err := rows.Scan(&devName, &devIP, &isOnline, &lastSeen, &createdAt); err == nil {
			status := "Offline"
			if isOnline {
				status = "Online"
			}
			ls := "Never"
			if lastSeen != nil {
				ls = *lastSeen
			}
			uptime := "0%"
			if isOnline {
				uptime = "100%"
			}
			writer.Write([]string{devName, devIP, status, ls, uptime})
		}
	}
}

// ExportInventoryReport exports the network asset inventory.
func (s *Service) ExportInventoryReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	query := `
		SELECT 
			CASE WHEN is_name_custom = 1 THEN name ELSE COALESCE(NULLIF(sys_name, ''), name) END as device_name, 
			ip_address, 
			vendor, 
			model, 
			device_type, 
			COALESCE(NULLIF(mac_address, ''), (SELECT if_mac FROM device_interfaces WHERE device_id = devices.id AND if_mac != '' LIMIT 1)) as mac_addr,
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
		pdf := gofpdf.New("L", "mm", "A4", "") // Landscape
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, "Network Asset Inventory")
		pdf.Ln(12)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetFillColor(240, 240, 240)
		headers := []string{"Device", "IP", "Vendor", "Model", "Type", "MAC Address"}
		widths := []float64{50, 35, 30, 40, 30, 40}

		for i, h := range headers {
			pdf.CellFormat(widths[i], 10, h, "1", 0, "", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Arial", "", 9)
		pdf.SetFillColor(255, 255, 255)

		for rows.Next() {
			var devName, devIP, devType, createdAt string
			var vendor, model, mac *string
			if err := rows.Scan(&devName, &devIP, &vendor, &model, &devType, &mac, &createdAt); err == nil {
				v := ""
				if vendor != nil {
					v = *vendor
				}
				m := ""
				if model != nil {
					m = *model
				}
				ma := ""
				if mac != nil {
					ma = *mac
				}

				pdf.CellFormat(widths[0], 8, cleanString(devName), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[1], 8, cleanString(devIP), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[2], 8, cleanString(v), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[3], 8, cleanString(m), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[4], 8, cleanString(devType), "1", 0, "", false, 0, "")
				pdf.CellFormat(widths[5], 8, cleanString(ma), "1", 0, "", false, 0, "")
				pdf.Ln(-1)
			}
		}

		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=inventory_%s.pdf", time.Now().Format("20060102_150405")))
		pdf.Output(c.Writer)
		return
	}

	// CSV Export
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=inventory_%s.csv", time.Now().Format("20060102_150405")))
	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"Device Name", "IP Address", "Vendor", "Model", "Type", "MAC Address", "Added Date"})

	for rows.Next() {
		var devName, devIP, devType, createdAt string
		var vendor, model, mac *string
		if err := rows.Scan(&devName, &devIP, &vendor, &model, &devType, &mac, &createdAt); err == nil {
			v := ""
			if vendor != nil {
				v = *vendor
			}
			m := ""
			if model != nil {
				m = *model
			}
			ma := ""
			if mac != nil {
				ma = *mac
			}

			writer.Write([]string{devName, devIP, v, m, devType, ma, createdAt})
		}
	}
}
