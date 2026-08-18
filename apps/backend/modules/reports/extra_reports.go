// Made by YTSworks
// YTS工作室製作
package reports

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type queryReportColumn struct {
	Header string
	Width  float64
}

func exportQueryReport(c *gin.Context, db *sql.DB, title, filenameBase string, columns []queryReportColumn, query string, args ...interface{}) {
	rt := newReportTranslator(c)
	headers := make([]string, 0, len(columns))
	widths := make([]float64, 0, len(columns))
	for _, col := range columns {
		headers = append(headers, rt.T(col.Header))
		widths = append(widths, col.Width)
	}

	values, err := queryReportRows(db, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	format := reportFormat(c)
	switch format {
	case "pdf":
		writePDFReport(c, rt.T(title), filenameBase, headers, widths, values)
	case "json":
		writeJSONReport(c, filenameBase, headers, values)
	default:
		writeCSVReport(c, filenameBase, headers, values)
	}
}

func queryReportRows(db *sql.DB, query string, args ...interface{}) ([][]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([][]string, 0)
	for rows.Next() {
		rawValues := make([]interface{}, len(columnNames))
		scanTargets := make([]interface{}, len(columnNames))
		for i := range rawValues {
			scanTargets[i] = &rawValues[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}

		row := make([]string, len(columnNames))
		for i, value := range rawValues {
			row[i] = reportValueString(value)
		}
		values = append(values, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func reportValueString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprint(v)
	}
}

func translateReportValues(c *gin.Context, values [][]string) [][]string {
	rt := newReportTranslator(c)
	for rowIndex := range values {
		for colIndex := range values[rowIndex] {
			values[rowIndex][colIndex] = rt.DisplayValue(values[rowIndex][colIndex])
		}
	}
	return values
}

func writeCSVReport(c *gin.Context, filenameBase string, headers []string, values [][]string) {
	values = translateReportValues(c, values)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.csv", filenameBase, time.Now().Format("20060102_150405")))

	writer := csvWriter(c.Writer)
	defer writer.Flush()
	_ = writer.Write(headers)
	for _, row := range values {
		_ = writer.Write(row)
	}
}

func writePDFReport(c *gin.Context, title, filenameBase string, headers []string, widths []float64, values [][]string) {
	values = translateReportValues(c, values)
	pdf, fontFamily, unicodeFont := newReportPDF("L")
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()
	pdf.SetFont(fontFamily, "B", 15)
	pdf.Cell(40, 9, reportPDFText(title, unicodeFont))
	pdf.Ln(11)

	pdf.SetFont(fontFamily, "B", 8)
	pdf.SetFillColor(240, 240, 240)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 7, reportPDFText(header, unicodeFont), "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont(fontFamily, "", 7)
	pdf.SetFillColor(255, 255, 255)
	for _, row := range values {
		for i, value := range row {
			text := reportPDFText(value, unicodeFont)
			text = truncateReportText(text, 40)
			pdf.CellFormat(widths[i], 6, text, "1", 0, "", false, 0, "")
		}
		pdf.Ln(-1)
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.pdf", filenameBase, time.Now().Format("20060102_150405")))
	if err := pdf.Output(c.Writer); err != nil {
		fmt.Println("PDF Output Error:", err)
	}
}

func writeJSONReport(c *gin.Context, filenameBase string, headers []string, values [][]string) {
	values = translateReportValues(c, values)
	items := make([]map[string]string, 0, len(values))
	for _, row := range values {
		item := make(map[string]string, len(headers))
		for i, header := range headers {
			key := strings.ToLower(strings.ReplaceAll(header, " ", "_"))
			item[key] = row[i]
		}
		items = append(items, item)
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s_%s.json", filenameBase, time.Now().Format("20060102_150405")))
	c.JSON(http.StatusOK, Response{Success: true, Data: items})
}

func csvWriter(w gin.ResponseWriter) *csv.Writer {
	return csv.NewWriter(w)
}

func (s *Service) ExportInterfaceReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.interfaces", "interfaces", []queryReportColumn{
		{"col.device", 32}, {"col.ip", 25}, {"col.ifindex", 12}, {"col.interface", 28}, {"col.description", 38},
		{"col.speed", 18}, {"col.oper", 16}, {"col.admin", 16}, {"col.in_bps", 20}, {"col.out_bps", 20},
		{"col.in_err", 14}, {"col.out_err", 14}, {"col.poe", 12}, {"col.updated", 25},
	}, `
		SELECT
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name,
			d.ip_address,
			COALESCE(di.if_index, 0) as if_index,
			COALESCE(NULLIF(di.if_name, ''), '') as if_name,
			COALESCE(NULLIF(di.if_desc, ''), '') as if_desc,
			COALESCE(di.if_speed, 0) as if_speed,
			CASE COALESCE(di.if_status, 0) WHEN 1 THEN 'up' WHEN 2 THEN 'down' ELSE 'unknown' END as oper_status,
			CASE COALESCE(di.if_admin_status, 0) WHEN 1 THEN 'up' WHEN 2 THEN 'down' ELSE 'unknown' END as admin_status,
			COALESCE(di.bandwidth_in, 0) as bandwidth_in,
			COALESCE(di.bandwidth_out, 0) as bandwidth_out,
			COALESCE(di.in_errors, 0) as in_errors,
			COALESCE(di.out_errors, 0) as out_errors,
			CASE COALESCE(di.poe_enabled, 0) WHEN 1 THEN 'enabled' ELSE 'disabled' END as poe_status,
			COALESCE(di.updated_at, '') as updated_at
		FROM device_interfaces di
		JOIN devices d ON d.id = di.device_id
		ORDER BY d.name, di.if_index
	`)
}

func (s *Service) ExportHealthTrendReport(c *gin.Context) {
	joinFilter, args := reportDateJoinFilter(c, "m.collected_at")
	exportQueryReport(c, s.db, "title.health_trend", "health_trend", []queryReportColumn{
		{"col.device", 34}, {"col.ip", 26}, {"col.samples", 16}, {"col.avg_cpu", 18}, {"col.max_cpu", 18},
		{"col.avg_mem", 18}, {"col.max_mem", 18}, {"col.avg_disk", 18}, {"col.max_disk", 18},
		{"col.alert_samples", 20}, {"col.last_sample", 28},
	}, fmt.Sprintf(`
		SELECT
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name,
			d.ip_address,
			COUNT(m.id) as samples,
			ROUND(AVG(COALESCE(m.cpu_usage, 0)), 2) as avg_cpu,
			ROUND(MAX(COALESCE(m.cpu_usage, 0)), 2) as max_cpu,
			ROUND(AVG(COALESCE(m.memory_usage, 0)), 2) as avg_mem,
			ROUND(MAX(COALESCE(m.memory_usage, 0)), 2) as max_mem,
			ROUND(AVG(COALESCE(m.disk_usage, 0)), 2) as avg_disk,
			ROUND(MAX(COALESCE(m.disk_usage, 0)), 2) as max_disk,
			SUM(CASE WHEN COALESCE(m.cpu_usage, 0) >= 80 OR COALESCE(m.memory_usage, 0) >= 80 OR COALESCE(m.disk_usage, 0) >= 90 THEN 1 ELSE 0 END) as alert_samples,
			COALESCE(MAX(m.collected_at), '') as last_sample
		FROM devices d
		LEFT JOIN device_metrics m ON m.device_id = d.id %s
		GROUP BY d.id, d.name, d.sys_name, d.ip_address, d.is_name_custom
		ORDER BY alert_samples DESC, max_cpu DESC, device_name
	`, joinFilter), args...)
}

func (s *Service) ExportSLAReport(c *gin.Context) {
	joinFilter, args := reportDateJoinFilter(c, "e.created_at")
	rt := newReportTranslator(c)
	exportQueryReport(c, s.db, "title.sla", "sla", []queryReportColumn{
		{"col.device", 38}, {"col.ip", 28}, {"col.current", 18}, {"col.last_seen", 30}, {"col.down_events", 20},
		{"col.up_events", 18}, {"col.last_down", 30}, {"col.last_up", 30}, {"col.current_estimate", 24},
	}, fmt.Sprintf(`
		SELECT
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as device_name,
			d.ip_address,
			CASE COALESCE(d.is_online, 0) WHEN 1 THEN 'online' ELSE 'offline' END as current_status,
			COALESCE(d.last_seen, '') as last_seen,
			SUM(CASE WHEN lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%offline%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%down%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%unreachable%%' THEN 1 ELSE 0 END) as down_events,
			SUM(CASE WHEN lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%online%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%up%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%recovered%%' THEN 1 ELSE 0 END) as up_events,
			COALESCE(MAX(CASE WHEN lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%offline%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%down%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%unreachable%%' THEN e.created_at END), '') as last_down,
			COALESCE(MAX(CASE WHEN lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%online%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%up%%'
				OR lower(COALESCE(e.event_type, '') || ' ' || COALESCE(e.message, '')) LIKE '%%recovered%%' THEN e.created_at END), '') as last_up,
			CASE COALESCE(d.is_online, 0) WHEN 1 THEN '100%% %s' ELSE '0%% %s' END as current_estimate
		FROM devices d
		LEFT JOIN events e ON e.device_id = d.id %s
		GROUP BY d.id, d.name, d.sys_name, d.ip_address, d.is_name_custom, d.is_online, d.last_seen
		ORDER BY current_status, down_events DESC, device_name
	`, rt.T("value.online_now"), rt.T("value.online_now"), joinFilter), args...)
}

func (s *Service) ExportAuditReport(c *gin.Context) {
	where, args := reportDateWhere(c, "occurred_at")
	exportQueryReport(c, s.db, "title.audit", "audit_changes", []queryReportColumn{
		{"col.time", 30}, {"col.user", 24}, {"col.source_ip", 26}, {"col.module", 22}, {"col.action", 34},
		{"col.resource_type", 26}, {"col.resource", 36}, {"col.resource_ip", 26}, {"col.status", 18}, {"col.review", 18},
	}, fmt.Sprintf(`
		SELECT occurred_at, username, COALESCE(source_ip, ''), COALESCE(module, ''), action,
			COALESCE(resource_type, ''), COALESCE(NULLIF(resource_name, ''), resource, ''),
			COALESCE(resource_ip, ''), COALESCE(status, ''), COALESCE(review_status, '')
		FROM audit_logs
		%s
		ORDER BY occurred_at DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportLicenseCapacityReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.license_capacity", "license_capacity", []queryReportColumn{
		{"col.license", 34}, {"col.type", 22}, {"col.active", 16}, {"col.devices", 18}, {"col.used_devices", 22},
		{"col.device_remain", 22}, {"col.cameras", 18}, {"col.used_cameras", 22}, {"col.camera_remain", 22},
		{"col.features", 46}, {"col.valid_from", 28}, {"col.valid_until", 28},
	}, `
		SELECT
			substr(license_key, 1, 8) || '...' as license_prefix,
			COALESCE(license_type, ''),
			CASE COALESCE(is_active, 0) WHEN 1 THEN 'active' ELSE 'inactive' END as active,
			COALESCE(device_count, 0) as device_count,
			(SELECT COUNT(*) FROM devices) as used_devices,
			MAX(COALESCE(device_count, 0) - (SELECT COUNT(*) FROM devices), 0) as remaining_devices,
			COALESCE(camera_count, 0) as camera_count,
			(SELECT COUNT(*) FROM cameras) as used_cameras,
			MAX(COALESCE(camera_count, 0) - (SELECT COUNT(*) FROM cameras), 0) as remaining_cameras,
			COALESCE(enabled_features, '') as enabled_features,
			COALESCE(valid_from, '') as valid_from,
			COALESCE(valid_until, '') as valid_until
		FROM licenses
		ORDER BY is_active DESC, valid_until DESC, id DESC
	`)
}

func (s *Service) ExportCameraReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.cameras", "cameras", []queryReportColumn{
		{"col.name", 34}, {"col.location", 30}, {"col.ip", 26}, {"col.port", 14}, {"col.maker", 26},
		{"col.model", 30}, {"col.firmware", 28}, {"col.ptz", 12}, {"col.enabled", 16}, {"col.status", 18},
		{"col.stream", 18}, {"col.last_seen", 28}, {"col.created", 28},
	}, `
		SELECT name, COALESCE(location, ''), COALESCE(ip_address, ''), COALESCE(port, 0),
			COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(firmware, ''),
			CASE COALESCE(supports_ptz, 0) WHEN 1 THEN 'yes' ELSE 'no' END,
			CASE COALESCE(is_enabled, 0) WHEN 1 THEN 'enabled' ELSE 'disabled' END,
			COALESCE(status, ''), COALESCE(stream_type, ''), COALESCE(last_seen, ''), COALESCE(created_at, '')
		FROM cameras
		ORDER BY name
	`)
}

func (s *Service) ExportPDUReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.pdu", "pdu", []queryReportColumn{
		{"col.name", 34}, {"col.location", 30}, {"col.ip", 26}, {"col.port", 14}, {"col.type", 18},
		{"col.maker", 26}, {"col.model", 30}, {"col.enabled", 16}, {"col.status", 18}, {"col.last_poll", 28}, {"col.created", 28},
	}, `
		SELECT name, COALESCE(location, ''), COALESCE(ip_address, ''), COALESCE(port, 0),
			COALESCE(device_type, ''), COALESCE(manufacturer, ''), COALESCE(model, ''),
			CASE COALESCE(is_enabled, 0) WHEN 1 THEN 'enabled' ELSE 'disabled' END,
			COALESCE(status, ''), COALESCE(last_polled_at, ''), COALESCE(created_at, '')
		FROM pdu_devices
		ORDER BY name
	`)
}

func (s *Service) ExportAccessControlReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.access_control", "access_control", []queryReportColumn{
		{"col.door", 34}, {"col.location", 30}, {"col.ip", 26}, {"col.port", 14}, {"col.maker", 26},
		{"col.model", 30}, {"col.protocol", 20}, {"col.enabled", 16}, {"col.status", 18}, {"col.last_seen", 28}, {"col.created", 28},
	}, `
		SELECT name, COALESCE(location, ''), COALESCE(ip_address, ''), COALESCE(port, 0),
			COALESCE(manufacturer, ''), COALESCE(model, ''), COALESCE(protocol, ''),
			CASE COALESCE(is_enabled, 0) WHEN 1 THEN 'enabled' ELSE 'disabled' END,
			COALESCE(status, ''), COALESCE(last_seen, ''), COALESCE(created_at, '')
		FROM ac_doors
		ORDER BY name
	`)
}

func (s *Service) ExportTopologyReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.topology", "topology_links", []queryReportColumn{
		{"col.source_device", 34}, {"col.source_ip", 25}, {"col.source_interface", 28},
		{"col.target_device", 34}, {"col.target_ip", 25}, {"col.target_interface", 28},
		{"col.link_speed", 20}, {"col.bandwidth_usage", 22}, {"col.link_type", 18},
		{"col.label", 28}, {"col.manual", 14}, {"col.discovered_at", 28},
	}, `
		SELECT
			CASE WHEN sd.is_name_custom = 1 THEN sd.name ELSE COALESCE(NULLIF(sd.sys_name, ''), sd.name) END,
			sd.ip_address,
			COALESCE(tl.source_if_name, ''),
			CASE WHEN td.is_name_custom = 1 THEN td.name ELSE COALESCE(NULLIF(td.sys_name, ''), td.name) END,
			td.ip_address,
			COALESCE(tl.target_if_name, ''),
			COALESCE(tl.link_speed, 0),
			COALESCE(tl.bandwidth_usage, 0),
			COALESCE(tl.link_type, ''),
			COALESCE(tl.link_label, ''),
			CASE COALESCE(tl.is_manual, 0) WHEN 1 THEN 'yes' ELSE 'no' END,
			COALESCE(tl.discovered_at, '')
		FROM topology_links tl
		JOIN devices sd ON sd.id = tl.source_device_id
		JOIN devices td ON td.id = tl.target_device_id
		ORDER BY sd.name, td.name, tl.id
	`)
}

func (s *Service) ExportEventReport(c *gin.Context) {
	where, args := reportDateWhere(c, "e.created_at")
	exportQueryReport(c, s.db, "title.events", "device_events", []queryReportColumn{
		{"col.time", 28}, {"col.device", 34}, {"col.ip", 25}, {"col.event_type", 24},
		{"col.severity", 18}, {"col.message", 80},
	}, fmt.Sprintf(`
		SELECT e.created_at,
			COALESCE(CASE WHEN d.is_name_custom = 1 THEN d.name ELSE NULLIF(d.sys_name, '') END, d.name, ''),
			COALESCE(d.ip_address, ''), COALESCE(e.event_type, ''), COALESCE(e.severity, ''), COALESCE(e.message, '')
		FROM events e
		LEFT JOIN devices d ON d.id = e.device_id
		%s
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportSyslogReport(c *gin.Context) {
	where, args := reportDateWhere(c, "s.received_at")
	exportQueryReport(c, s.db, "title.syslog", "syslog", []queryReportColumn{
		{"col.time", 28}, {"col.device", 34}, {"col.source_ip", 25}, {"col.facility", 18},
		{"col.severity", 18}, {"col.message", 90},
	}, fmt.Sprintf(`
		SELECT s.received_at,
			COALESCE(CASE WHEN d.is_name_custom = 1 THEN d.name ELSE NULLIF(d.sys_name, '') END, d.name, ''),
			COALESCE(s.source_ip, ''), COALESCE(s.facility, ''), COALESCE(s.severity, ''), COALESCE(s.message, '')
		FROM syslogs s
		LEFT JOIN devices d ON d.id = s.device_id
		%s
		ORDER BY s.received_at DESC, s.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportNotificationReport(c *gin.Context) {
	where, args := reportDateWhere(c, "n.created_at")
	exportQueryReport(c, s.db, "title.notifications", "notifications", []queryReportColumn{
		{"col.time", 28}, {"col.severity", 18}, {"col.category", 24}, {"col.title", 42},
		{"col.message", 80}, {"col.device", 34}, {"col.read", 14},
	}, fmt.Sprintf(`
		SELECT n.created_at, COALESCE(n.severity, ''), COALESCE(n.category, ''),
			COALESCE(n.title, ''), COALESCE(n.message, ''),
			COALESCE(CASE WHEN d.is_name_custom = 1 THEN d.name ELSE NULLIF(d.sys_name, '') END, d.name, ''),
			CASE COALESCE(n.is_read, 0) WHEN 1 THEN 'yes' ELSE 'no' END
		FROM notifications n
		LEFT JOIN devices d ON d.id = n.device_id
		%s
		ORDER BY n.created_at DESC, n.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportIoTDeviceReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.iot_devices", "iot_devices", []queryReportColumn{
		{"col.name", 32}, {"col.external_id", 28}, {"col.protocol", 20}, {"col.connection", 34},
		{"col.unit_id", 14}, {"col.signal_count", 18}, {"col.signals", 64}, {"col.enabled", 16},
		{"col.last_value", 20}, {"col.last_seen", 28}, {"col.last_error", 50},
	}, `
		SELECT d.name, COALESCE(d.topic, ''), COALESCE(d.protocol, ''),
			CASE
				WHEN lower(COALESCE(d.protocol, '')) IN ('rs485', 'modbus_rs485', 'modbus-rs485', 'modbus_rtu', 'modbus-rtu', 'rtu')
					THEN COALESCE(d.serial_port, '')
				ELSE COALESCE(d.host, '') || CASE WHEN COALESCE(d.port, 0) > 0 THEN ':' || d.port ELSE '' END
			END,
			COALESCE(d.unit_id, 1), COUNT(s.id),
			COALESCE(GROUP_CONCAT(NULLIF(COALESCE(s.name, '') || CASE WHEN COALESCE(s.name, '') <> '' THEN ' (' || s.metric || ')' ELSE s.metric END, ''), ', '), ''),
			CASE COALESCE(d.enabled, 0) WHEN 1 THEN 'enabled' ELSE 'disabled' END,
			COALESCE(d.last_value, ''), COALESCE(d.last_seen, ''), COALESCE(d.last_error, '')
		FROM iot_devices d
		LEFT JOIN iot_device_signals s ON s.device_id = d.id
		GROUP BY d.id
		ORDER BY d.name
	`)
}

func (s *Service) ExportIoTMeasurementReport(c *gin.Context) {
	where, args := reportDateWhere(c, "m.created_at")
	exportQueryReport(c, s.db, "title.iot_measurements", "iot_measurements", []queryReportColumn{
		{"col.time", 28}, {"col.device", 32}, {"col.external_id", 26}, {"col.metric", 26},
		{"col.value", 20}, {"col.sample_id", 34}, {"col.forward_status", 22}, {"col.attempts", 16},
		{"col.forwarded_at", 28}, {"col.last_error", 55},
	}, fmt.Sprintf(`
		SELECT m.created_at, COALESCE(d.name, ''), COALESCE(NULLIF(m.external_id, ''), d.topic, ''),
			COALESCE(m.metric, ''), m.value, COALESCE(m.sample_id, ''),
			COALESCE(m.forward_status, 'pending'), COALESCE(m.forward_attempts, 0),
			COALESCE(m.forwarded_at, ''), COALESCE(m.last_error, '')
		FROM iot_measurements m
		LEFT JOIN iot_devices d ON d.id = m.device_id
		%s
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportCameraRecordingReport(c *gin.Context) {
	where, args := reportDateWhere(c, "r.started_at")
	exportQueryReport(c, s.db, "title.camera_recordings", "camera_recordings", []queryReportColumn{
		{"col.camera", 34}, {"col.label", 34}, {"col.status", 18}, {"col.file_size", 20},
		{"col.duration", 18}, {"col.started_at", 28}, {"col.ended_at", 28}, {"col.created", 28},
	}, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(r.camera_name, ''), c.name, ''), COALESCE(r.label, ''), COALESCE(r.status, ''),
			COALESCE(r.file_size, 0), COALESCE(r.duration_sec, 0), r.started_at,
			COALESCE(r.ended_at, ''), COALESCE(r.created_at, '')
		FROM camera_recordings r
		LEFT JOIN cameras c ON c.id = r.camera_id
		%s
		ORDER BY r.started_at DESC, r.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportAccessEventReport(c *gin.Context) {
	where, args := reportDateWhere(c, "e.occurred_at")
	exportQueryReport(c, s.db, "title.access_events", "access_events", []queryReportColumn{
		{"col.time", 28}, {"col.door", 32}, {"col.location", 28}, {"col.card_number", 26},
		{"col.holder", 30}, {"col.event_type", 24},
	}, fmt.Sprintf(`
		SELECT e.occurred_at, COALESCE(d.name, ''), COALESCE(d.location, ''),
			COALESCE(NULLIF(e.card_number, ''), c.card_number, ''),
			COALESCE(NULLIF(e.holder_name, ''), c.holder_name, ''), COALESCE(e.event_type, '')
		FROM ac_events e
		LEFT JOIN ac_doors d ON d.id = e.door_id
		LEFT JOIN ac_cards c ON c.id = e.card_id
		%s
		ORDER BY e.occurred_at DESC, e.id DESC
		LIMIT 5000
	`, where), args...)
}

func (s *Service) ExportAccessCardReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.access_cards", "access_cards", []queryReportColumn{
		{"col.card_number", 28}, {"col.holder", 34}, {"col.department", 30}, {"col.active", 16},
		{"col.valid_from", 26}, {"col.valid_until", 26}, {"col.created", 28},
	}, `
		SELECT card_number, holder_name, COALESCE(department, ''),
			CASE COALESCE(is_active, 0) WHEN 1 THEN 'active' ELSE 'inactive' END,
			COALESCE(valid_from, ''), COALESCE(valid_until, ''), COALESCE(created_at, '')
		FROM ac_cards
		ORDER BY holder_name, card_number
	`)
}

func (s *Service) ExportAccessScheduleReport(c *gin.Context) {
	exportQueryReport(c, s.db, "title.access_schedules", "access_schedules", []queryReportColumn{
		{"col.card_number", 26}, {"col.holder", 30}, {"col.department", 28}, {"col.door", 30},
		{"col.allow_days", 24}, {"col.time_range", 24}, {"col.valid_from", 24}, {"col.valid_until", 24},
		{"col.status", 18}, {"col.approved_by", 24}, {"col.approved_at", 28}, {"col.note", 46},
	}, `
		SELECT s.card_number, COALESCE(NULLIF(s.holder_name, ''), c.holder_name, ''),
			COALESCE(NULLIF(s.department, ''), c.department, ''), COALESCE(d.name, ''),
			COALESCE(s.allow_days, ''), COALESCE(s.time_from, '') || ' - ' || COALESCE(s.time_until, ''),
			COALESCE(s.valid_from, ''), COALESCE(s.valid_until, ''), COALESCE(s.status, ''),
			COALESCE(s.approved_by, ''), COALESCE(s.approved_at, ''), COALESCE(s.note, '')
		FROM ac_card_schedules s
		LEFT JOIN ac_cards c ON c.id = s.card_id
		LEFT JOIN ac_doors d ON d.id = s.door_id
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT 5000
	`)
}

func (s *Service) ExportConfigBackupReport(c *gin.Context) {
	where, args := reportDateWhere(c, "b.created_at")
	exportQueryReport(c, s.db, "title.config_backups", "device_config_backups", []queryReportColumn{
		{"col.time", 28}, {"col.device", 36}, {"col.ip", 25}, {"col.note", 70}, {"col.content_bytes", 24},
	}, fmt.Sprintf(`
		SELECT b.created_at,
			CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END,
			d.ip_address, COALESCE(b.note, ''), LENGTH(COALESCE(b.content, ''))
		FROM device_config_backups b
		JOIN devices d ON d.id = b.device_id
		%s
		ORDER BY b.created_at DESC, b.id DESC
		LIMIT 5000
	`, where), args...)
}

func reportDateJoinFilter(c *gin.Context, column string) (string, []interface{}) {
	clauses := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)
	if start := strings.TrimSpace(c.Query("start")); start != "" {
		clauses = append(clauses, column+" >= ?")
		args = append(args, start)
	}
	if end := strings.TrimSpace(c.Query("end")); end != "" {
		clauses = append(clauses, column+" <= ?")
		args = append(args, end)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "AND " + strings.Join(clauses, " AND "), args
}

func reportDateWhere(c *gin.Context, column string) (string, []interface{}) {
	joinFilter, args := reportDateJoinFilter(c, column)
	if joinFilter == "" {
		return "", args
	}
	return "WHERE " + strings.TrimPrefix(joinFilter, "AND "), args
}
