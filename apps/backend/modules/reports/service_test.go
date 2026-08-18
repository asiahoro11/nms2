// Made by YTSworks
// YTS工作室製作
package reports

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func setupReportTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	schema := []string{
		`CREATE TABLE devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			ip_address TEXT UNIQUE NOT NULL,
			mac_address TEXT,
			device_type TEXT DEFAULT 'unknown',
			vendor TEXT,
			model TEXT,
			firmware TEXT,
			sys_name TEXT,
			sys_uptime TEXT,
			sys_location TEXT,
			is_online BOOLEAN DEFAULT 0,
			last_seen DATETIME,
			is_name_custom BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE device_interfaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			if_index INTEGER,
			if_name TEXT,
			if_desc TEXT,
			if_speed BIGINT,
			if_mac TEXT,
			if_status INTEGER,
			if_admin_status INTEGER,
			in_errors BIGINT DEFAULT 0,
			out_errors BIGINT DEFAULT 0,
			bandwidth_in BIGINT DEFAULT 0,
			bandwidth_out BIGINT DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			poe_enabled INTEGER DEFAULT 0
		)`,
		`CREATE TABLE events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			event_type TEXT,
			severity TEXT,
			message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE syslogs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			source_ip TEXT,
			severity TEXT,
			facility TEXT,
			message TEXT,
			received_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE device_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			cpu_usage REAL,
			memory_usage REAL,
			disk_usage REAL,
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			username TEXT NOT NULL,
			source_ip TEXT,
			module TEXT DEFAULT '',
			action TEXT NOT NULL,
			resource TEXT,
			resource_type TEXT DEFAULT '',
			resource_name TEXT DEFAULT '',
			resource_ip TEXT DEFAULT '',
			status TEXT DEFAULT 'success',
			review_status TEXT DEFAULT 'pending'
		)`,
		`CREATE TABLE licenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			license_key TEXT UNIQUE NOT NULL,
			license_type TEXT DEFAULT 'standard',
			device_count INTEGER DEFAULT 0,
			camera_count INTEGER DEFAULT 0,
			enabled_features TEXT DEFAULT '[]',
			valid_from DATETIME DEFAULT CURRENT_TIMESTAMP,
			valid_until DATETIME,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE cameras (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT,
			ip_address TEXT,
			port INTEGER DEFAULT 554,
			manufacturer TEXT,
			model TEXT,
			firmware TEXT,
			supports_ptz BOOLEAN DEFAULT 0,
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			stream_type TEXT DEFAULT 'mjpeg'
		)`,
		`CREATE TABLE pdu_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			port INTEGER DEFAULT 161,
			device_type TEXT DEFAULT 'ups',
			manufacturer TEXT DEFAULT '',
			model TEXT DEFAULT '',
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_polled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ac_doors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			port INTEGER DEFAULT 80,
			manufacturer TEXT DEFAULT '',
			model TEXT DEFAULT '',
			protocol TEXT DEFAULT 'http',
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE topology_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_device_id INTEGER NOT NULL,
			target_device_id INTEGER NOT NULL,
			source_if_name TEXT,
			target_if_name TEXT,
			link_speed BIGINT DEFAULT 0,
			bandwidth_usage BIGINT DEFAULT 0,
			link_type TEXT DEFAULT 'auto',
			link_label TEXT,
			is_manual BOOLEAN DEFAULT 0,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			severity TEXT DEFAULT 'warning',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			is_read BOOLEAN DEFAULT 0,
			device_id INTEGER DEFAULT 0,
			category TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE iot_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			host TEXT DEFAULT '',
			port INTEGER DEFAULT 0,
			unit_id INTEGER DEFAULT 1,
			topic TEXT DEFAULT '',
			enabled BOOLEAN DEFAULT 1,
			last_value REAL,
			last_seen DATETIME,
			last_error TEXT DEFAULT '',
			serial_port TEXT DEFAULT ''
		)`,
		`CREATE TABLE iot_device_signals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			name TEXT DEFAULT '',
			metric TEXT DEFAULT 'value'
		)`,
		`CREATE TABLE iot_measurements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			external_id TEXT DEFAULT '',
			metric TEXT DEFAULT 'value',
			value REAL NOT NULL,
			sample_id TEXT DEFAULT '',
			forward_status TEXT DEFAULT 'pending',
			forward_attempts INTEGER DEFAULT 0,
			forwarded_at DATETIME,
			last_error TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE camera_recordings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			camera_id INTEGER NOT NULL,
			camera_name TEXT DEFAULT '',
			file_size INTEGER DEFAULT 0,
			duration_sec INTEGER DEFAULT 0,
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			label TEXT DEFAULT '',
			status TEXT DEFAULT 'recording',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ac_cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_number TEXT NOT NULL UNIQUE,
			holder_name TEXT NOT NULL,
			department TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 1,
			valid_from DATE,
			valid_until DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ac_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			door_id INTEGER,
			card_id INTEGER,
			card_number TEXT DEFAULT '',
			holder_name TEXT DEFAULT '',
			event_type TEXT DEFAULT 'access',
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE ac_card_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER,
			card_number TEXT NOT NULL,
			door_id INTEGER,
			holder_name TEXT DEFAULT '',
			department TEXT DEFAULT '',
			allow_days TEXT DEFAULT '1,2,3,4,5,6,7',
			time_from TEXT DEFAULT '00:00',
			time_until TEXT DEFAULT '23:59',
			valid_from DATE NOT NULL,
			valid_until DATE NOT NULL,
			status TEXT DEFAULT 'pending',
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			approved_at DATETIME,
			approved_by TEXT DEFAULT ''
		)`,
		`CREATE TABLE device_config_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	if _, err := db.Exec(`
		INSERT INTO devices (name, ip_address, mac_address, device_type, vendor, model, firmware, sys_name, sys_uptime, sys_location, is_online, last_seen, created_at, updated_at)
		VALUES ('Core Switch', '10.0.0.1', '00:11:22:33:44:55', 'switch', 'Cisco', 'C9300', '17.9.5', 'core-sw-01', '12 days', 'MDF', 1, '2026-05-15 10:00:00', '2026-05-01 09:00:00', '2026-05-15 10:01:00')
	`); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO device_interfaces (device_id, if_index, if_name, if_desc, if_speed, if_mac, if_status, if_admin_status, in_errors, out_errors, bandwidth_in, bandwidth_out, poe_enabled)
		VALUES (1, 1, 'Gi1/0/1', 'uplink', 1000000000, '00:11:22:33:44:55', 1, 1, 0, 1, 1000, 2000, 1)
	`); err != nil {
		t.Fatalf("insert interface: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO events (device_id, event_type, severity, message, created_at)
		VALUES (1, 'device_up', 'info', 'Device is online', '2026-05-15 10:02:00')
	`); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, disk_usage, collected_at)
		VALUES (1, 11.5, 42.0, 68.25, '2026-05-15 10:03:00')
	`); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO audit_logs (username, source_ip, module, action, resource, resource_type, resource_name, resource_ip, status) VALUES ('admin', '127.0.0.1', 'device', 'update_device', 'device:1', 'device', 'Core Switch', '10.0.0.1', 'success')`); err != nil {
		t.Fatalf("insert audit: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO licenses (license_key, license_type, device_count, camera_count, enabled_features, valid_from, valid_until, is_active) VALUES ('ABCDEF1234567890', 'full', 10, 4, '["device_management","camera"]', '2026-01-01', '2027-01-01', 1)`); err != nil {
		t.Fatalf("insert license: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cameras (name, location, ip_address, port, manufacturer, model, firmware, supports_ptz, is_enabled, status, stream_type) VALUES ('Lobby Cam', 'Lobby', '10.0.0.10', 554, 'Axis', 'M30', '1.0', 1, 1, 'online', 'mjpeg')`); err != nil {
		t.Fatalf("insert camera: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO pdu_devices (name, location, ip_address, device_type, manufacturer, model, is_enabled, status) VALUES ('Rack PDU', 'Rack A', '10.0.0.20', 'pdu', 'APC', 'PDU9000', 1, 'online')`); err != nil {
		t.Fatalf("insert pdu: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO ac_doors (name, location, ip_address, manufacturer, model, protocol, is_enabled, status) VALUES ('Front Door', 'Lobby', '10.0.0.30', 'HID', 'Edge', 'http', 1, 'online')`); err != nil {
		t.Fatalf("insert access control: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO devices (name, ip_address, sys_name, is_online) VALUES ('Edge Switch', '10.0.0.2', 'edge-sw-01', 1)`); err != nil {
		t.Fatalf("insert target device: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO topology_links (source_device_id, target_device_id, source_if_name, target_if_name, link_speed, bandwidth_usage, link_type, link_label, is_manual) VALUES (1, 2, 'Gi1/0/1', 'Gi0/1', 1000000000, 3000, 'manual', 'Core uplink', 1)`); err != nil {
		t.Fatalf("insert topology link: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO syslogs (device_id, source_ip, severity, facility, message) VALUES (1, '10.0.0.1', 'warning', 'daemon', 'Link state changed')`); err != nil {
		t.Fatalf("insert syslog: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO notifications (severity, title, message, is_read, device_id, category) VALUES ('warning', 'Traffic anomaly', 'High interface traffic', 0, 1, 'traffic_anomaly')`); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO iot_devices (name, protocol, host, port, unit_id, topic, enabled, last_value, last_seen) VALUES ('Chiller PLC', 'modbus_tcp', '192.168.1.10', 502, 1, 'DEV001', 1, 7.0, '2026-05-15 10:04:00')`); err != nil {
		t.Fatalf("insert iot device: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO iot_device_signals (device_id, name, metric) VALUES (1, 'Chilled water supply', 'chwSupplyTempC')`); err != nil {
		t.Fatalf("insert iot signal: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO iot_measurements (device_id, external_id, metric, value, sample_id, forward_status, forward_attempts, created_at) VALUES (1, 'DEV001', 'chwSupplyTempC', 7.0, 'sample-001', 'sent', 1, '2026-05-15 10:04:00')`); err != nil {
		t.Fatalf("insert iot measurement: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO camera_recordings (camera_id, camera_name, file_size, duration_sec, started_at, ended_at, label, status) VALUES (1, 'Lobby Cam', 4096, 60, '2026-05-15 09:00:00', '2026-05-15 09:01:00', 'Motion', 'done')`); err != nil {
		t.Fatalf("insert camera recording: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO ac_cards (card_number, holder_name, department, is_active, valid_from, valid_until) VALUES ('CARD-001', 'Alice', 'IT', 1, '2026-01-01', '2026-12-31')`); err != nil {
		t.Fatalf("insert access card: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO ac_events (door_id, card_id, card_number, holder_name, event_type, occurred_at) VALUES (1, 1, 'CARD-001', 'Alice', 'access', '2026-05-15 08:00:00')`); err != nil {
		t.Fatalf("insert access event: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO ac_card_schedules (card_id, card_number, door_id, holder_name, department, allow_days, time_from, time_until, valid_from, valid_until, status, approved_by) VALUES (1, 'CARD-001', 1, 'Alice', 'IT', '1,2,3,4,5', '08:00', '18:00', '2026-01-01', '2026-12-31', 'approved', 'admin')`); err != nil {
		t.Fatalf("insert access schedule: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO device_config_backups (device_id, content, note) VALUES (1, 'secret-config-content', 'Before upgrade')`); err != nil {
		t.Fatalf("insert config backup: %v", err)
	}

	return db
}

func performReportRequest(t *testing.T, target string, handler func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	handler(ctx)
	return recorder
}

func TestExportDevicesReportPDFFormatParameter(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/devices?format=pdf", service.ExportDevicesReport)

	if got := recorder.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("content type = %q, want application/pdf", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, ".pdf") {
		t.Fatalf("content disposition = %q, want pdf filename", got)
	}
	if !strings.HasPrefix(recorder.Body.String(), "%PDF") {
		t.Fatalf("body does not look like a PDF")
	}
}

func TestExportDevicesReportCSVIncludesExpandedFields(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/devices?format=csv", service.ExportDevicesReport)
	body := recorder.Body.String()

	if got := recorder.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("content type = %q, want text/csv; charset=utf-8", got)
	}
	for _, want := range []string{"韌體", "位置", "SNMP 運行時間", "介面數", "PoE 埠數", "17.9.5", "MDF", "12 days", "線上"} {
		if !strings.Contains(body, want) {
			t.Fatalf("csv body missing %q:\n%s", want, body)
		}
	}
}

func TestExportDevicesReportUsesRequestedLanguage(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/devices?format=csv&lang=en-US", service.ExportDevicesReport)
	body := recorder.Body.String()

	for _, want := range []string{"Firmware", "Location", "SNMP Uptime", "Interface Count", "PoE Ports", "Online"} {
		if !strings.Contains(body, want) {
			t.Fatalf("english csv body missing %q:\n%s", want, body)
		}
	}
}

func TestExportLogsReportPDFFormatParameter(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/logs?format=pdf&type=events", service.ExportLogsReport)

	if got := recorder.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("content type = %q, want application/pdf", got)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, ".pdf") {
		t.Fatalf("content disposition = %q, want pdf filename", got)
	}
	if !strings.HasPrefix(recorder.Body.String(), "%PDF") {
		t.Fatalf("body does not look like a PDF")
	}
}

func TestAdditionalReportsExportCSV(t *testing.T) {
	service := NewService(setupReportTestDB(t))
	cases := []struct {
		name    string
		target  string
		handler func(*gin.Context)
		want    string
	}{
		{"interfaces", "/reports/interfaces?format=csv", service.ExportInterfaceReport, "PoE"},
		{"health trend", "/reports/health-trend?format=csv", service.ExportHealthTrendReport, "告警樣本"},
		{"sla", "/reports/sla?format=csv", service.ExportSLAReport, "目前估算"},
		{"audit", "/reports/audit?format=csv", service.ExportAuditReport, "update_device"},
		{"license capacity", "/reports/license-capacity?format=csv", service.ExportLicenseCapacityReport, "ABCDEF12..."},
		{"cameras", "/reports/cameras?format=csv", service.ExportCameraReport, "Lobby Cam"},
		{"pdu", "/reports/pdu?format=csv", service.ExportPDUReport, "Rack PDU"},
		{"access control", "/reports/access-control?format=csv", service.ExportAccessControlReport, "Front Door"},
		{"topology", "/reports/topology?format=csv", service.ExportTopologyReport, "Core uplink"},
		{"events", "/reports/events?format=csv", service.ExportEventReport, "Device is online"},
		{"syslog", "/reports/syslog?format=csv", service.ExportSyslogReport, "Link state changed"},
		{"notifications", "/reports/notifications?format=csv", service.ExportNotificationReport, "Traffic anomaly"},
		{"iot devices", "/reports/iot-devices?format=csv", service.ExportIoTDeviceReport, "chwSupplyTempC"},
		{"iot measurements", "/reports/iot-measurements?format=csv", service.ExportIoTMeasurementReport, "sample-001"},
		{"camera recordings", "/reports/camera-recordings?format=csv", service.ExportCameraRecordingReport, "Motion"},
		{"access events", "/reports/access-events?format=csv", service.ExportAccessEventReport, "CARD-001"},
		{"access cards", "/reports/access-cards?format=csv", service.ExportAccessCardReport, "Alice"},
		{"access schedules", "/reports/access-schedules?format=csv", service.ExportAccessScheduleReport, "08:00 - 18:00"},
		{"config backups", "/reports/config-backups?format=csv", service.ExportConfigBackupReport, "Before upgrade"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performReportRequest(t, tc.target, tc.handler)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
				t.Fatalf("content type = %q, want text/csv; charset=utf-8", got)
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Fatalf("body missing %q:\n%s", tc.want, recorder.Body.String())
			}
		})
	}
}

func TestConfigBackupReportDoesNotExposeContent(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/config-backups?format=csv&lang=en-US", service.ExportConfigBackupReport)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-config-content") {
		t.Fatalf("configuration content was exposed:\n%s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "21") {
		t.Fatalf("configuration size missing:\n%s", recorder.Body.String())
	}
}

func TestAdditionalReportJSONIsDownloadable(t *testing.T) {
	service := NewService(setupReportTestDB(t))

	recorder := performReportRequest(t, "/reports/iot-measurements?format=json&lang=en-US", service.ExportIoTMeasurementReport)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, ".json") {
		t.Fatalf("content disposition = %q, want json filename", got)
	}
	if !strings.Contains(recorder.Body.String(), "sample-001") {
		t.Fatalf("json body missing sample:\n%s", recorder.Body.String())
	}
}
