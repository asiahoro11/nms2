// Made by YTSworks
// YTS工作室製作
package iot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	modbusclient "github.com/simonvetter/modbus"

	"management-server/pkg/dbutils"
)

const (
	defaultPollIntervalSeconds   = 60
	defaultForwardBatchSize      = 50
	defaultSentRetentionMins     = 10
	defaultForwardIntervalMillis = 30000
	minForwardIntervalMillis     = 100
	maxForwardIntervalMillis     = 86400000
)

type Service struct {
	db *sql.DB

	loopOnce    sync.Once
	tablesMu    sync.Mutex
	tablesReady bool
	forwardMu   sync.Mutex
	stopCh      chan struct{}

	serialMu sync.Map // key: serial port → *sync.Mutex (Windows: exclusive open)
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, stopCh: make(chan struct{})}
}

func (s *Service) EnsureTables() error {
	s.tablesMu.Lock()
	defer s.tablesMu.Unlock()

	if s.tablesReady {
		return nil
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS iot_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			host TEXT DEFAULT '',
			port INTEGER DEFAULT 0,
			unit_id INTEGER DEFAULT 1,
			address INTEGER DEFAULT 0,
			quantity INTEGER DEFAULT 1,
			data_type TEXT DEFAULT 'uint16',
			scale REAL DEFAULT 1,
			topic TEXT DEFAULT '',
			enabled BOOLEAN DEFAULT 1,
			last_value REAL,
			last_raw TEXT DEFAULT '',
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS iot_measurements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			external_id TEXT DEFAULT '',
			metric TEXT NOT NULL DEFAULT 'value',
			value REAL NOT NULL,
			raw_json TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES iot_devices(id) ON DELETE SET NULL
		)
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS iot_device_signals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			metric TEXT NOT NULL DEFAULT 'value',
			address INTEGER NOT NULL DEFAULT 0,
			quantity INTEGER NOT NULL DEFAULT 1,
			function_code INTEGER NOT NULL DEFAULT 3,
			data_type TEXT NOT NULL DEFAULT 'uint16',
			byte_order TEXT NOT NULL DEFAULT 'big',
			word_order TEXT NOT NULL DEFAULT 'big',
			scale REAL NOT NULL DEFAULT 1,
			offset REAL NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			last_value REAL,
			last_raw TEXT NOT NULL DEFAULT '',
			last_seen DATETIME,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES iot_devices(id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}

	migrations := []string{
		`ALTER TABLE iot_devices ADD COLUMN function_code INTEGER DEFAULT 3`,
		`ALTER TABLE iot_devices ADD COLUMN byte_order TEXT DEFAULT 'big'`,
		`ALTER TABLE iot_devices ADD COLUMN word_order TEXT DEFAULT 'big'`,
		`ALTER TABLE iot_devices ADD COLUMN offset REAL DEFAULT 0`,
		`ALTER TABLE iot_devices ADD COLUMN metric TEXT DEFAULT 'value'`,
		`ALTER TABLE iot_devices ADD COLUMN poll_interval_seconds INTEGER DEFAULT 60`,
		`ALTER TABLE iot_devices ADD COLUMN last_polled_at DATETIME`,
		`ALTER TABLE iot_devices ADD COLUMN last_error TEXT DEFAULT ''`,
		`ALTER TABLE iot_measurements ADD COLUMN event_id TEXT DEFAULT ''`,
		`ALTER TABLE iot_measurements ADD COLUMN sample_id TEXT DEFAULT ''`,
		`ALTER TABLE iot_measurements ADD COLUMN forward_status TEXT DEFAULT 'pending'`,
		`ALTER TABLE iot_measurements ADD COLUMN forward_attempts INTEGER DEFAULT 0`,
		`ALTER TABLE iot_measurements ADD COLUMN forwarded_at DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN drop_after DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN next_attempt_at DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN last_error TEXT DEFAULT ''`,
		// RTU / RS485 serial fields
		`ALTER TABLE iot_devices ADD COLUMN sensor_type TEXT DEFAULT ''`,
		`ALTER TABLE iot_devices ADD COLUMN serial_port TEXT DEFAULT ''`,
		`ALTER TABLE iot_devices ADD COLUMN baud_rate INTEGER DEFAULT 9600`,
		`ALTER TABLE iot_devices ADD COLUMN data_bits INTEGER DEFAULT 8`,
		`ALTER TABLE iot_devices ADD COLUMN parity TEXT DEFAULT 'N'`,
		`ALTER TABLE iot_devices ADD COLUMN stop_bits INTEGER DEFAULT 1`,
	}
	for _, stmt := range migrations {
		_, _ = s.db.Exec(stmt)
	}

	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_iot_measurements_event_id ON iot_measurements(event_id) WHERE event_id <> ''`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_iot_measurements_forward ON iot_measurements(forward_status, id)`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_iot_device_signals_device ON iot_device_signals(device_id, sort_order, id)`); err != nil {
		return err
	}

	configs := []struct {
		key, value, desc string
	}{
		{"iot_forward_enabled", "false", "Enable IoT HTTP forwarder"},
		{"iot_forward_url", "", "IoT HTTP forward webhook URL"},
		{"iot_forward_token", "", "IoT HTTP forward bearer token"},
		{"iot_forward_batch_size", strconv.Itoa(defaultForwardBatchSize), "IoT forward batch size"},
		{"iot_forward_sent_retention_minutes", strconv.Itoa(defaultSentRetentionMins), "IoT sent record retention in minutes"},
		{"iot_forward_interval_seconds", strconv.Itoa(defaultForwardIntervalMillis / 1000), "Legacy IoT forward schedule interval in seconds"},
		{"iot_forward_last_attempt_at", "", "IoT forward last attempt timestamp"},
		{"iot_forward_last_success_at", "", "IoT forward last success timestamp"},
		{"iot_forward_last_error", "", "IoT forward last error"},
	}
	for _, cfg := range configs {
		if _, err := s.db.Exec(`
			INSERT OR IGNORE INTO system_config (config_key, config_value, description)
			VALUES (?, ?, ?)
		`, cfg.key, cfg.value, cfg.desc); err != nil {
			return err
		}
	}
	legacyIntervalMilliseconds := defaultForwardIntervalMillis
	var legacyIntervalRaw string
	if err := s.db.QueryRow(`
		SELECT config_value
		FROM system_config
		WHERE config_key = 'iot_forward_interval_seconds'
	`).Scan(&legacyIntervalRaw); err == nil {
		if seconds, err := strconv.Atoi(legacyIntervalRaw); err == nil && seconds > 0 {
			if seconds >= maxForwardIntervalMillis/1000 {
				legacyIntervalMilliseconds = maxForwardIntervalMillis
			} else {
				legacyIntervalMilliseconds = clampForwardIntervalMilliseconds(seconds * 1000)
			}
		}
	}
	if _, err := s.db.Exec(`
		INSERT OR IGNORE INTO system_config (config_key, config_value, description)
		VALUES (?, ?, ?)
	`, "iot_forward_interval_ms", strconv.Itoa(legacyIntervalMilliseconds), "IoT forward schedule interval in milliseconds"); err != nil {
		return err
	}

	s.tablesReady = true
	return nil
}

func (s *Service) StartBackgroundLoop() {
	s.loopOnce.Do(func() {
		go s.backgroundLoop()
	})
}

func (s *Service) backgroundLoop() {
	pollTicker := time.NewTicker(1 * time.Second)
	forwardTicker := time.NewTicker(time.Duration(minForwardIntervalMillis) * time.Millisecond)
	cleanupTicker := time.NewTicker(1 * time.Minute)
	defer pollTicker.Stop()
	defer forwardTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-pollTicker.C:
			if s.LicenseEnabled() {
				s.PollDueDevices()
			}
		case <-forwardTicker.C:
			if s.LicenseEnabled() {
				s.flushForwardQueueIfDue()
			}
		case <-cleanupTicker.C:
			if s.LicenseEnabled() {
				s.CleanupForwardedMeasurements()
			}
		}
	}
}

func (s *Service) LicenseEnabled() bool {
	var val string
	err := s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key='iot_enabled'`).Scan(&val)
	return err == nil && (val == "1" || strings.EqualFold(val, "true"))
}

func (s *Service) Status() (Status, error) {
	if err := s.EnsureTables(); err != nil {
		return Status{}, err
	}
	settings, _ := s.ForwarderSettings()
	status := Status{Enabled: s.LicenseEnabled()}
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices`).Scan(&status.DeviceCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices WHERE enabled = 1`).Scan(&status.EnabledCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements`).Scan(&status.MeasurementCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, 'pending') = 'pending'`).Scan(&status.ForwardPendingCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'failed'`).Scan(&status.ForwardFailedCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'sent'`).Scan(&status.ForwardSentHoldCount)
	status.ForwardEnabled = settings.Enabled
	status.ForwardURLConfigured = strings.TrimSpace(settings.URL) != ""
	status.ForwardLastSuccessAt = settings.LastSuccessAt
	status.ForwardLastAttemptAt = settings.LastAttemptAt
	status.ForwardLastError = settings.LastError
	_ = s.db.QueryRow(`
		SELECT COALESCE(last_error, '')
		FROM iot_measurements
		WHERE COALESCE(last_error, '') <> ''
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&status.ForwardLastError)
	return status, nil
}

func (s *Service) Capabilities() []ProtocolProfile {
	return []ProtocolProfile{
		{
			Protocol:    "modbus_tcp",
			Name:        "Modbus TCP",
			Mode:        "direct_poll",
			Status:      "ready",
			Description: "LAN/WAN active polling. Supports FC03/FC04, multi data types, endian config.",
			DataTypes:   []string{"uint16", "int16", "uint32", "int32", "float32", "float64"},
			Endpoint:    "/api/v1/iot/devices",
		},
		{
			Protocol:    "modbus_rtu",
			Name:        "Modbus RTU",
			Mode:        "direct_poll",
			Status:      "ready",
			Description: "Serial port active polling over RS232/RS485. Supports FC03/FC04 with CRC16 framing.",
			DataTypes:   []string{"uint16", "int16", "uint32", "int32", "float32", "float64"},
			Endpoint:    "/api/v1/iot/devices",
		},
		{
			Protocol:    "modbus_rs485",
			Name:        "Modbus RS485",
			Mode:        "direct_poll",
			Status:      "ready",
			Description: "RTU framing over RS485 half-duplex serial bus. Suitable for multi-drop sensor networks.",
			DataTypes:   []string{"uint16", "int16", "uint32", "int32", "float32", "float64"},
			Endpoint:    "/api/v1/iot/devices",
		},
		{
			Protocol:    "modbus_rtu_tcp",
			Name:        "Modbus RTU over TCP",
			Mode:        "direct_poll",
			Status:      "ready",
			Description: "RTU frames with CRC over a TCP transparent serial gateway. Use this for RS485-to-Ethernet converters that are not Modbus TCP gateways.",
			DataTypes:   []string{"uint16", "int16", "uint32", "int32", "float32", "float64"},
			Endpoint:    "/api/v1/iot/devices",
		},
		{
			Protocol:    "rest",
			Name:        "REST / Webhook",
			Mode:        "gateway_ingest",
			Status:      "ready",
			Description: "Generic HTTP push path for IoT gateways and custom integrations.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "http_forward",
			Name:        "HTTP Forward Queue",
			Mode:        "store_and_forward",
			Status:      "ready",
			Description: "Durable local buffering with offline retry and auto-cleanup.",
			Endpoint:    "/api/v1/iot/forwarder/settings",
		},
		{
			Protocol:    "mqtt",
			Name:        "MQTT Bridge",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Use an MQTT bridge to normalize topics into the REST ingest API.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "opcua",
			Name:        "OPC-UA Gateway",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Push OPC-UA node values via gateway script into the ingest API.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "bacnet",
			Name:        "BACnet Gateway",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Push BACnet/IP building automation points via collector into the ingest API.",
			Endpoint:    "/api/v1/iot/ingest",
		},
	}
}

func (s *Service) ListDevices() ([]Device, error) {
	if err := s.EnsureTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, name, protocol, COALESCE(sensor_type,''), host, port,
		       COALESCE(serial_port,''), COALESCE(baud_rate,9600), COALESCE(data_bits,8),
		       COALESCE(parity,'N'), COALESCE(stop_bits,1),
		       unit_id, address, quantity, COALESCE(function_code, 3),
		       data_type, COALESCE(byte_order, 'big'), COALESCE(word_order, 'big'), scale, COALESCE(offset, 0),
		       COALESCE(metric, 'value'), topic, enabled, last_value, COALESCE(last_raw, ''),
		       COALESCE(last_seen, ''), COALESCE(last_polled_at, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, ''), COALESCE(updated_at, ''),
		       COALESCE(poll_interval_seconds, ?)
		FROM iot_devices
		ORDER BY id DESC
	`, defaultPollIntervalSeconds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range devices {
		if err := s.populateDeviceSignals(&devices[i]); err != nil {
			return nil, err
		}
	}
	return devices, nil
}

func (s *Service) CreateDevice(input UpsertDeviceInput) (int, error) {
	if err := s.EnsureTables(); err != nil {
		return 0, err
	}
	normalized := normalizeInput(input)
	enabled := true
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		INSERT INTO iot_devices (
			name, protocol, sensor_type, host, port,
			serial_port, baud_rate, data_bits, parity, stop_bits,
			unit_id, address, quantity, function_code, data_type,
			byte_order, word_order, scale, offset, metric, topic, poll_interval_seconds, enabled
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, normalized.Name, normalized.Protocol, normalized.SensorType, normalized.Host, normalized.Port,
		normalized.SerialPort, normalized.BaudRate, normalized.DataBits, normalized.Parity, normalized.StopBits,
		normalized.UnitID, normalized.Address, normalized.Quantity, normalized.FunctionCode, normalized.DataType,
		normalized.ByteOrder, normalized.WordOrder, normalized.Scale, normalized.Offset,
		normalized.Metric, normalized.Topic, normalized.PollIntervalSeconds, enabled)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if err := replaceDeviceSignals(tx, int(id), normalized.Signals); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(id), nil
}

func (s *Service) UpdateDevice(id string, input UpsertDeviceInput) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	deviceID, err := strconv.Atoi(id)
	if err != nil || deviceID <= 0 {
		return sql.ErrNoRows
	}
	normalized := normalizeInput(input)
	enabled := true
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		UPDATE iot_devices
		SET name = ?, protocol = ?, sensor_type = ?, host = ?, port = ?,
		    serial_port = ?, baud_rate = ?, data_bits = ?, parity = ?, stop_bits = ?,
		    unit_id = ?, address = ?, quantity = ?, function_code = ?, data_type = ?,
		    byte_order = ?, word_order = ?, scale = ?, offset = ?, metric = ?, topic = ?,
		    poll_interval_seconds = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalized.Name, normalized.Protocol, normalized.SensorType, normalized.Host, normalized.Port,
		normalized.SerialPort, normalized.BaudRate, normalized.DataBits, normalized.Parity, normalized.StopBits,
		normalized.UnitID, normalized.Address, normalized.Quantity, normalized.FunctionCode, normalized.DataType,
		normalized.ByteOrder, normalized.WordOrder, normalized.Scale, normalized.Offset,
		normalized.Metric, normalized.Topic, normalized.PollIntervalSeconds, enabled, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	if err := replaceDeviceSignals(tx, deviceID, normalized.Signals); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) DeleteDevice(id string) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM iot_device_signals WHERE device_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM iot_devices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Service) PollDevice(id string) (Device, error) {
	if err := s.EnsureTables(); err != nil {
		return Device{}, err
	}
	d, err := s.getDevice(id)
	if err != nil {
		return Device{}, err
	}
	err = s.pollDevice(d)
	if err != nil {
		return Device{}, err
	}
	return s.getDevice(id)
}

func (s *Service) PollDueDevices() {
	if err := s.EnsureTables(); err != nil {
		return
	}
	rows, err := s.db.Query(`
		SELECT id, name, protocol, COALESCE(sensor_type,''), host, port,
		       COALESCE(serial_port,''), COALESCE(baud_rate,9600), COALESCE(data_bits,8),
		       COALESCE(parity,'N'), COALESCE(stop_bits,1),
		       unit_id, address, quantity, COALESCE(function_code, 3),
		       data_type, COALESCE(byte_order, 'big'), COALESCE(word_order, 'big'), scale, COALESCE(offset, 0),
		       COALESCE(metric, 'value'), topic, enabled, last_value, COALESCE(last_raw, ''),
		       COALESCE(last_seen, ''), COALESCE(last_polled_at, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, ''), COALESCE(updated_at, ''),
		       COALESCE(poll_interval_seconds, ?)
		FROM iot_devices
		WHERE enabled = 1
		  AND protocol IN ('modbus_tcp', 'modbus_rtu', 'modbus_rs485', 'modbus_rtu_tcp')
		  AND (
			last_polled_at IS NULL
			OR last_polled_at = ''
			OR datetime(last_polled_at, '+' || COALESCE(poll_interval_seconds, ?) || ' seconds') <= CURRENT_TIMESTAMP
		  )
		ORDER BY id ASC
	`, defaultPollIntervalSeconds, defaultPollIntervalSeconds)
	if err != nil {
		return
	}

	// Drain cursor before polling; RTU reads do DB writes that deadlock an open rows handle.
	var due []Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			continue
		}
		due = append(due, d)
	}
	rows.Close()

	for _, d := range due {
		_ = s.populateDeviceSignals(&d)
		_ = s.pollDevice(d)
	}
}

func (s *Service) serialPortMu(port string) *sync.Mutex {
	v, _ := s.serialMu.LoadOrStore(port, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (s *Service) pollDevice(d Device) error {
	if !d.Enabled {
		return errors.New("iot device is disabled")
	}
	if len(d.Signals) > 0 && strings.ToLower(strings.TrimSpace(d.SensorType)) != "temperature_humidity" {
		return s.pollDeviceSignals(d)
	}
	d = normalizeDeviceForPoll(d)
	value, raw, err := s.readConfiguredDevice(d)
	if err != nil {
		slog.Error("[IoT] poll failed", "device_id", d.ID, "name", d.Name, "protocol", d.Protocol, "error", err)
		_, _ = dbutils.ExecWithRetry(s.db, `
			UPDATE iot_devices
			SET last_error = ?, last_polled_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, err.Error(), d.ID)
		return err
	}
	if reading, ok := decodeTemperatureHumidityRaw(raw, d); ok {
		slog.Info("[IoT] poll ok", "device_id", d.ID, "name", d.Name, "raw", raw, "temperature", reading.Temperature, "humidity", reading.Humidity)
		sampleID := uuid.NewString()
		if _, err := s.recordMeasurementForSample(&d.ID, d.ExternalID, "temperature", reading.Temperature, raw, sampleID); err != nil {
			return err
		}
		if _, err := s.recordMeasurementForSample(&d.ID, d.ExternalID, "humidity", reading.Humidity, raw, sampleID); err != nil {
			return err
		}
		_, err = dbutils.ExecWithRetry(s.db, `
			UPDATE iot_devices
			SET last_value = ?, last_raw = ?, last_seen = CURRENT_TIMESTAMP,
			    last_polled_at = CURRENT_TIMESTAMP, last_error = '', updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, reading.Temperature, raw, d.ID)
		return err
	}

	value = (value * effectiveScale(d)) + d.Offset
	metric := strings.TrimSpace(d.Metric)
	if metric == "" {
		metric = "value"
	}
	slog.Info("[IoT] poll ok", "device_id", d.ID, "name", d.Name, "raw", raw, "value", value, "metric", metric)
	if _, err := s.recordMeasurementForSample(&d.ID, d.ExternalID, metric, value, raw, uuid.NewString()); err != nil {
		return err
	}
	_, err = dbutils.ExecWithRetry(s.db, `
		UPDATE iot_devices
		SET last_value = ?, last_raw = ?, last_seen = CURRENT_TIMESTAMP,
		    last_polled_at = CURRENT_TIMESTAMP, last_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, value, raw, d.ID)
	return err
}

func (s *Service) readConfiguredDevice(d Device) (float64, string, error) {
	d.Address = modbusWireAddress(d.Address, d.FunctionCode)
	switch d.Protocol {
	case "modbus_tcp":
		return readModbusTCP(d)
	case "modbus_rtu_tcp":
		return readModbusRTUOverTCP(d)
	case "modbus_rtu", "modbus_rs485":
		port := strings.TrimSpace(d.SerialPort)
		mu := s.serialPortMu(port)
		mu.Lock()
		defer mu.Unlock()
		return readModbusRTU(d)
	default:
		return 0, "", fmt.Errorf("protocol %q does not support active polling", d.Protocol)
	}
}

// Modbus documentation commonly labels holding register offset 0 as 40001
// and input register offset 0 as 30001. Keep that readable reference in the
// configuration, but send the zero-based address required by the wire protocol.
func modbusWireAddress(address, functionCode int) int {
	switch {
	case functionCode == 2 && address >= 10001 && address <= 19999:
		return address - 10001
	case functionCode == 4 && address >= 30001 && address <= 39999:
		return address - 30001
	case functionCode == 3 && address >= 40001 && address <= 49999:
		return address - 40001
	default:
		return address
	}
}

func (s *Service) pollDeviceSignals(d Device) error {
	sampleID := uuid.NewString()
	var firstValue *float64
	var firstRaw string
	var failures []string
	successes := 0
	for _, signal := range d.Signals {
		configured := d
		configured.SensorType = ""
		configured.Address = signal.Address
		configured.Quantity = signal.Quantity
		configured.FunctionCode = signal.FunctionCode
		configured.DataType = signal.DataType
		configured.ByteOrder = signal.ByteOrder
		configured.WordOrder = signal.WordOrder
		configured.Scale = signal.Scale
		configured.Offset = signal.Offset
		configured.Metric = signal.Metric
		value, raw, err := s.readConfiguredDevice(configured)
		if err != nil {
			message := fmt.Sprintf("%s: %v", signal.Metric, err)
			failures = append(failures, message)
			_, _ = dbutils.ExecWithRetry(s.db, `
				UPDATE iot_device_signals
				SET last_error = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, truncateError(err), signal.ID)
			continue
		}
		value = (value * signal.Scale) + signal.Offset
		if _, err := s.recordMeasurementForSample(&d.ID, d.ExternalID, signal.Metric, value, raw, sampleID); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", signal.Metric, err))
			continue
		}
		_, _ = dbutils.ExecWithRetry(s.db, `
			UPDATE iot_device_signals
			SET last_value = ?, last_raw = ?, last_seen = CURRENT_TIMESTAMP,
			    last_error = '', updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, value, raw, signal.ID)
		if firstValue == nil {
			copyValue := value
			firstValue = &copyValue
			firstRaw = raw
		}
		successes++
	}
	lastError := strings.Join(failures, "; ")
	if firstValue != nil {
		_, _ = dbutils.ExecWithRetry(s.db, `
			UPDATE iot_devices
			SET last_value = ?, last_raw = ?, last_seen = CURRENT_TIMESTAMP,
			    last_polled_at = CURRENT_TIMESTAMP, last_error = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, *firstValue, firstRaw, lastError, d.ID)
	} else {
		_, _ = dbutils.ExecWithRetry(s.db, `
			UPDATE iot_devices
			SET last_polled_at = CURRENT_TIMESTAMP, last_error = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, lastError, d.ID)
	}
	if successes == 0 {
		if lastError == "" {
			lastError = "no IoT signals configured"
		}
		return errors.New(lastError)
	}
	return nil
}

func (s *Service) Ingest(input IngestInput) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	metric := strings.TrimSpace(input.Metric)
	if metric == "" {
		metric = "value"
	}
	protocol := normalizeProtocol(input.Protocol)
	if protocol == "" {
		protocol = "rest"
	}

	rawBytes, _ := json.Marshal(input.Raw)
	raw := string(rawBytes)
	var deviceID *int
	if strings.TrimSpace(input.ExternalID) != "" {
		if id, err := s.upsertExternalDevice(input, protocol); err == nil {
			deviceID = &id
		}
	}
	_, err := s.recordMeasurement(deviceID, strings.TrimSpace(input.ExternalID), metric, input.Value, raw)
	return err
}

func (s *Service) RecentMeasurements(limit int) ([]Measurement, error) {
	if err := s.EnsureTables(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, COALESCE(event_id, ''), COALESCE(NULLIF(sample_id, ''), event_id, ''), device_id, external_id, metric, value, COALESCE(raw_json, ''),
		       COALESCE(forward_status, 'pending'), COALESCE(forward_attempts, 0),
		       COALESCE(forwarded_at, ''), COALESCE(drop_after, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, '')
		FROM iot_measurements
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Measurement, 0)
	for rows.Next() {
		item, err := scanMeasurement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) QueueStatus() (QueueStatus, error) {
	if err := s.EnsureTables(); err != nil {
		return QueueStatus{}, err
	}
	var status QueueStatus
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, 'pending') = 'pending'`).Scan(&status.Pending)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'failed'`).Scan(&status.Failed)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'sent'`).Scan(&status.SentHold)
	return status, nil
}

func (s *Service) ForwarderSettings() (ForwarderSettings, error) {
	if err := s.EnsureTables(); err != nil {
		return ForwarderSettings{}, err
	}
	settings := ForwarderSettings{
		BatchSize:        defaultForwardBatchSize,
		RetentionMinutes: defaultSentRetentionMins,
	}
	settings.Enabled = s.getConfig("iot_forward_enabled") == "true"
	settings.URL = s.getConfig("iot_forward_url")
	settings.TokenConfigured = s.getConfig("iot_forward_token") != ""
	settings.LastAttemptAt = s.getConfig("iot_forward_last_attempt_at")
	settings.LastSuccessAt = s.getConfig("iot_forward_last_success_at")
	settings.LastError = s.getConfig("iot_forward_last_error")
	if batch, err := strconv.Atoi(s.getConfig("iot_forward_batch_size")); err == nil && batch > 0 && batch <= 500 {
		settings.BatchSize = batch
	}
	if mins, err := strconv.Atoi(s.getConfig("iot_forward_sent_retention_minutes")); err == nil && mins > 0 {
		settings.RetentionMinutes = mins
	}
	settings.IntervalMilliseconds = defaultForwardIntervalMillis
	if milliseconds, err := strconv.Atoi(s.getConfig("iot_forward_interval_ms")); err == nil && milliseconds > 0 {
		settings.IntervalMilliseconds = clampForwardIntervalMilliseconds(milliseconds)
	} else if seconds, err := strconv.Atoi(s.getConfig("iot_forward_interval_seconds")); err == nil && seconds > 0 {
		if seconds >= maxForwardIntervalMillis/1000 {
			settings.IntervalMilliseconds = maxForwardIntervalMillis
		} else {
			settings.IntervalMilliseconds = clampForwardIntervalMilliseconds(seconds * 1000)
		}
	}
	settings.IntervalSeconds = (settings.IntervalMilliseconds + 999) / 1000
	return settings, nil
}

func (s *Service) UpdateForwarderSettings(input ForwarderSettingsInput) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	if input.Enabled != nil {
		if *input.Enabled {
			if err := s.setConfig("iot_forward_enabled", "true", "Enable IoT HTTP forwarder"); err != nil {
				return err
			}
		} else if err := s.setConfig("iot_forward_enabled", "false", "Enable IoT HTTP forwarder"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(input.URL) != "" {
		if err := s.setConfig("iot_forward_url", strings.TrimSpace(input.URL), "IoT HTTP forward webhook URL"); err != nil {
			return err
		}
	}
	if input.ClearToken {
		if err := s.setConfig("iot_forward_token", "", "IoT HTTP forward bearer token"); err != nil {
			return err
		}
	} else if strings.TrimSpace(input.Token) != "" {
		if err := s.setConfig("iot_forward_token", strings.TrimSpace(input.Token), "IoT HTTP forward bearer token"); err != nil {
			return err
		}
	}
	if input.BatchSize > 0 {
		if input.BatchSize > 500 {
			input.BatchSize = 500
		}
		if err := s.setConfig("iot_forward_batch_size", strconv.Itoa(input.BatchSize), "IoT forward batch size"); err != nil {
			return err
		}
	}
	intervalMilliseconds := input.IntervalMilliseconds
	if intervalMilliseconds <= 0 && input.IntervalSeconds > 0 {
		if input.IntervalSeconds >= maxForwardIntervalMillis/1000 {
			intervalMilliseconds = maxForwardIntervalMillis
		} else {
			intervalMilliseconds = input.IntervalSeconds * 1000
		}
	}
	if intervalMilliseconds > 0 {
		intervalMilliseconds = clampForwardIntervalMilliseconds(intervalMilliseconds)
		if err := s.setConfig("iot_forward_interval_ms", strconv.Itoa(intervalMilliseconds), "IoT forward schedule interval in milliseconds"); err != nil {
			return err
		}
		legacySeconds := (intervalMilliseconds + 999) / 1000
		if err := s.setConfig("iot_forward_interval_seconds", strconv.Itoa(legacySeconds), "Legacy IoT forward schedule interval in seconds"); err != nil {
			return err
		}
	}
	return nil
}

func clampForwardIntervalMilliseconds(value int) int {
	if value < minForwardIntervalMillis {
		return minForwardIntervalMillis
	}
	if value > maxForwardIntervalMillis {
		return maxForwardIntervalMillis
	}
	return value
}

func (s *Service) flushForwardQueueIfDue() {
	settings, err := s.ForwarderSettings()
	if err != nil || !settings.Enabled || strings.TrimSpace(settings.URL) == "" {
		return
	}
	if lastAttempt, err := time.Parse(time.RFC3339, settings.LastAttemptAt); err == nil {
		if time.Since(lastAttempt) < time.Duration(settings.IntervalMilliseconds)*time.Millisecond {
			return
		}
	}
	_, _ = s.FlushForwardQueue()
}

func (s *Service) FlushForwardQueue() (int, error) {
	s.forwardMu.Lock()
	defer s.forwardMu.Unlock()

	if err := s.EnsureTables(); err != nil {
		return 0, err
	}
	settings, err := s.ForwarderSettings()
	if err != nil {
		return 0, err
	}
	if !settings.Enabled || strings.TrimSpace(settings.URL) == "" {
		return 0, nil
	}
	attemptedAt := time.Now().UTC().Format(time.RFC3339)
	_ = s.setConfig("iot_forward_last_attempt_at", attemptedAt, "IoT forward last attempt timestamp")
	rows, err := s.db.Query(`
		WITH selected_samples AS (
			SELECT COALESCE(NULLIF(sample_id, ''), NULLIF(event_id, ''), 'legacy:' || id) AS sample_key
			FROM iot_measurements
			WHERE COALESCE(forward_status, 'pending') IN ('pending', 'failed')
			  AND (next_attempt_at IS NULL OR next_attempt_at = '' OR next_attempt_at <= CURRENT_TIMESTAMP)
			GROUP BY sample_key
			ORDER BY MIN(id) ASC
			LIMIT ?
		)
		SELECT id, COALESCE(event_id, ''),
		       COALESCE(NULLIF(sample_id, ''), NULLIF(event_id, ''), 'legacy:' || id),
		       device_id, external_id, metric, value, COALESCE(raw_json, ''),
		       COALESCE(forward_status, 'pending'), COALESCE(forward_attempts, 0),
		       COALESCE(forwarded_at, ''), COALESCE(drop_after, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, '')
		FROM iot_measurements
		WHERE COALESCE(forward_status, 'pending') IN ('pending', 'failed')
		  AND (next_attempt_at IS NULL OR next_attempt_at = '' OR next_attempt_at <= CURRENT_TIMESTAMP)
		  AND COALESCE(NULLIF(sample_id, ''), NULLIF(event_id, ''), 'legacy:' || id)
		      IN (SELECT sample_key FROM selected_samples)
		ORDER BY id ASC
	`, settings.BatchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	items := make([]Measurement, 0)
	for rows.Next() {
		item, err := scanMeasurement(rows)
		if err != nil {
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	sentIDs, forwardErr := s.forwardMeasurements(settings, items)
	if len(sentIDs) > 0 {
		if err := s.markMeasurementsSent(settings, sentIDs); err != nil {
			return 0, err
		}
		_ = s.setConfig("iot_forward_last_success_at", time.Now().UTC().Format(time.RFC3339), "IoT forward last success timestamp")
	}
	if forwardErr != nil {
		msg := truncateError(forwardErr)
		_ = s.setConfig("iot_forward_last_error", msg, "IoT forward last error")
		sentSet := make(map[int]struct{}, len(sentIDs))
		for _, id := range sentIDs {
			sentSet[id] = struct{}{}
		}
		for _, item := range items {
			if _, sent := sentSet[item.ID]; sent {
				continue
			}
			delay := retryDelaySeconds(item.ForwardAttempts + 1)
			_, _ = s.db.Exec(`
				UPDATE iot_measurements
				SET forward_status = 'failed',
				    forward_attempts = COALESCE(forward_attempts, 0) + 1,
				    next_attempt_at = datetime(CURRENT_TIMESTAMP, '+' || ? || ' seconds'),
				    last_error = ?
				WHERE id = ?
			`, delay, msg, item.ID)
		}
		return len(sentIDs), forwardErr
	}
	_ = s.setConfig("iot_forward_last_error", "", "IoT forward last error")
	return len(sentIDs), nil
}

func (s *Service) markMeasurementsSent(settings ForwarderSettings, measurementIDs []int) error {
	if len(measurementIDs) == 0 {
		return nil
	}
	ids := make([]interface{}, 0, len(measurementIDs))
	placeholders := make([]string, 0, len(measurementIDs))
	for _, id := range measurementIDs {
		ids = append(ids, id)
		placeholders = append(placeholders, "?")
	}
	query := fmt.Sprintf(`
		UPDATE iot_measurements
		SET forward_status = 'sent',
		    forwarded_at = CURRENT_TIMESTAMP,
		    drop_after = datetime(CURRENT_TIMESTAMP, '+%d minutes'),
		    last_error = ''
		WHERE id IN (%s)
	`, settings.RetentionMinutes, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, ids...)
	return err
}

func (s *Service) CleanupForwardedMeasurements() (int64, error) {
	if err := s.EnsureTables(); err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`
		DELETE FROM iot_measurements
		WHERE COALESCE(forward_status, '') = 'sent'
		  AND drop_after IS NOT NULL
		  AND drop_after <> ''
		  AND drop_after <= CURRENT_TIMESTAMP
	`)
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

func (s *Service) forwardMeasurements(settings ForwarderSettings, items []Measurement) ([]int, error) {
	type deviceReport struct {
		DeviceID       string             `json:"deviceId"`
		SendTime       int64              `json:"sendTime"`
		TagData        map[string]float64 `json:"tagData"`
		sampleID       string
		measurementIDs []int
	}
	reportsBySample := make(map[string]*deviceReport)
	order := make([]string, 0)
	for _, item := range items {
		sampleID := strings.TrimSpace(item.SampleID)
		if sampleID == "" {
			sampleID = item.EventID
		}
		deviceID := strings.TrimSpace(item.ExternalID)
		if deviceID == "" && item.DeviceID != nil {
			_ = s.db.QueryRow(`SELECT COALESCE(NULLIF(topic, ''), 'device:' || id) FROM iot_devices WHERE id = ?`, *item.DeviceID).Scan(&deviceID)
		}
		if deviceID == "" {
			deviceID = "unknown"
		}
		key := deviceID + "\x00" + sampleID
		report := reportsBySample[key]
		if report == nil {
			report = &deviceReport{
				DeviceID: deviceID,
				SendTime: formatForwardTime(item.CreatedAt),
				TagData:  make(map[string]float64),
				sampleID: sampleID,
			}
			reportsBySample[key] = report
			order = append(order, key)
		}
		report.TagData[item.Metric] = item.Value
		report.measurementIDs = append(report.measurementIDs, item.ID)
	}
	sentIDs := make([]int, 0, len(items))
	for _, key := range order {
		report := reportsBySample[key]
		body, err := json.Marshal(report)
		if err != nil {
			return sentIDs, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.URL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return sentIDs, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-NMS-Idempotency-Key", report.sampleID)
		if token := s.getConfig("iot_forward_token"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return sentIDs, err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		cancel()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return sentIDs, fmt.Errorf("forward target returned %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		sentIDs = append(sentIDs, report.measurementIDs...)
	}
	return sentIDs, nil
}

func formatForwardTime(value string) int64 {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UnixMilli()
		}
	}
	return time.Now().UnixMilli()
}

func retryDelaySeconds(attempts int) int {
	if attempts < 1 {
		return 10
	}
	delay := 10 * attempts
	if delay > 300 {
		return 300
	}
	return delay
}

func (s *Service) getDevice(id string) (Device, error) {
	row := s.db.QueryRow(`
		SELECT id, name, protocol, COALESCE(sensor_type,''), host, port,
		       COALESCE(serial_port,''), COALESCE(baud_rate,9600), COALESCE(data_bits,8),
		       COALESCE(parity,'N'), COALESCE(stop_bits,1),
		       unit_id, address, quantity, COALESCE(function_code, 3),
		       data_type, COALESCE(byte_order, 'big'), COALESCE(word_order, 'big'), scale, COALESCE(offset, 0),
		       COALESCE(metric, 'value'), topic, enabled, last_value, COALESCE(last_raw, ''),
		       COALESCE(last_seen, ''), COALESCE(last_polled_at, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, ''), COALESCE(updated_at, ''),
		       COALESCE(poll_interval_seconds, ?)
		FROM iot_devices
		WHERE id = ?
	`, defaultPollIntervalSeconds, id)
	d, err := scanDevice(row)
	if err != nil {
		return Device{}, err
	}
	if err := s.populateDeviceSignals(&d); err != nil {
		return Device{}, err
	}
	return d, nil
}

func (s *Service) upsertExternalDevice(input IngestInput, protocol string) (int, error) {
	externalID := strings.TrimSpace(input.ExternalID)
	var id int
	if err := s.db.QueryRow(`SELECT id FROM iot_devices WHERE topic = ? AND protocol = ?`, externalID, protocol).Scan(&id); err == nil {
		_, _ = s.db.Exec(`
			UPDATE iot_devices
			SET name = COALESCE(NULLIF(?, ''), name), last_value = ?, last_seen = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, strings.TrimSpace(input.Name), input.Value, id)
		return id, nil
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = externalID
	}
	res, err := s.db.Exec(`
		INSERT INTO iot_devices (name, protocol, topic, enabled, last_value, last_seen)
		VALUES (?, ?, ?, 1, ?, CURRENT_TIMESTAMP)
	`, name, protocol, externalID, input.Value)
	if err != nil {
		return 0, err
	}
	newID, _ := res.LastInsertId()
	return int(newID), nil
}

func (s *Service) recordMeasurement(deviceID *int, externalID, metric string, value float64, raw string) (string, error) {
	return s.recordMeasurementForSample(deviceID, externalID, metric, value, raw, uuid.NewString())
}

func (s *Service) recordMeasurementForSample(deviceID *int, externalID, metric string, value float64, raw, sampleID string) (string, error) {
	var id interface{}
	if deviceID != nil {
		id = *deviceID
	}
	eventID := uuid.NewString()
	_, err := dbutils.ExecWithRetry(s.db, `
		INSERT INTO iot_measurements (
			event_id, sample_id, device_id, external_id, metric, value, raw_json, forward_status, forward_attempts
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', 0)
	`, eventID, sampleID, id, externalID, metric, value, raw)
	return eventID, err
}

func replaceDeviceSignals(tx *sql.Tx, deviceID int, signals []DeviceSignalInput) error {
	if _, err := tx.Exec(`DELETE FROM iot_device_signals WHERE device_id = ?`, deviceID); err != nil {
		return err
	}
	for index, signal := range signals {
		if _, err := tx.Exec(`
			INSERT INTO iot_device_signals (
				device_id, name, metric, address, quantity, function_code, data_type,
				byte_order, word_order, scale, offset, unit, sort_order
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, deviceID, signal.Name, signal.Metric, signal.Address, signal.Quantity, signal.FunctionCode,
			signal.DataType, signal.ByteOrder, signal.WordOrder, signal.Scale, signal.Offset,
			signal.Unit, index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) populateDeviceSignals(device *Device) error {
	rows, err := s.db.Query(`
		SELECT id, device_id, name, metric, address, quantity, function_code, data_type,
		       byte_order, word_order, scale, offset, unit, sort_order, last_value,
		       COALESCE(last_raw, ''), COALESCE(last_seen, ''), COALESCE(last_error, '')
		FROM iot_device_signals
		WHERE device_id = ?
		ORDER BY sort_order ASC, id ASC
	`, device.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	signals := make([]DeviceSignal, 0)
	for rows.Next() {
		var signal DeviceSignal
		var lastValue sql.NullFloat64
		if err := rows.Scan(
			&signal.ID, &signal.DeviceID, &signal.Name, &signal.Metric, &signal.Address,
			&signal.Quantity, &signal.FunctionCode, &signal.DataType, &signal.ByteOrder,
			&signal.WordOrder, &signal.Scale, &signal.Offset, &signal.Unit, &signal.SortOrder,
			&lastValue, &signal.LastRaw, &signal.LastSeen, &signal.LastError,
		); err != nil {
			return err
		}
		if lastValue.Valid {
			value := lastValue.Float64
			signal.LastValue = &value
		}
		signals = append(signals, signal)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	device.Signals = signals
	if len(signals) == 0 {
		return nil
	}
	first := signals[0]
	device.Address = first.Address
	device.Quantity = first.Quantity
	device.FunctionCode = first.FunctionCode
	device.DataType = first.DataType
	device.ByteOrder = first.ByteOrder
	device.WordOrder = first.WordOrder
	device.Scale = first.Scale
	device.Offset = first.Offset
	device.Metric = first.Metric
	if first.LastValue != nil {
		device.LastValue = first.LastValue
		device.LastRaw = first.LastRaw
	}
	device.LastReadings = device.LastReadings[:0]
	for _, signal := range signals {
		if signal.LastValue != nil {
			device.LastReadings = append(device.LastReadings, SensorReading{
				Metric: signal.Metric, Value: *signal.LastValue, Unit: signal.Unit, DataType: signal.DataType,
			})
		}
	}
	return nil
}

func normalizeInput(input UpsertDeviceInput) UpsertDeviceInput {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "IoT Device"
	}
	input.Protocol = normalizeProtocol(input.Protocol)
	if input.Protocol == "" {
		input.Protocol = "modbus_tcp"
	}
	input.SensorType = strings.ToLower(strings.TrimSpace(input.SensorType))
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.Topic = strings.TrimSpace(input.Topic)
	if input.ExternalID == "" {
		input.ExternalID = input.Topic
	}
	input.Topic = input.ExternalID
	if input.Port == 0 && (input.Protocol == "modbus_tcp" || input.Protocol == "modbus_rtu_tcp") {
		input.Port = 502
	}
	if input.Protocol == "modbus_tcp" || input.Protocol == "modbus_rtu_tcp" {
		input.SerialPort = ""
		input.BaudRate = 0
		input.DataBits = 0
		input.Parity = ""
		input.StopBits = 0
	}
	// RTU / RS485 serial defaults. Serial transports never retain a stale TCP endpoint.
	if input.Protocol == "modbus_rtu" || input.Protocol == "modbus_rs485" {
		input.Host = ""
		input.Port = 0
		if input.BaudRate <= 0 {
			input.BaudRate = 9600
		}
		if input.DataBits <= 0 {
			input.DataBits = 8
		}
		input.Parity = strings.ToUpper(strings.TrimSpace(input.Parity))
		if input.Parity == "" {
			input.Parity = "N"
		}
		if input.StopBits <= 0 {
			input.StopBits = 1
		}
	}
	if input.UnitID <= 0 {
		input.UnitID = 1
	}
	if input.UnitID > 247 {
		input.UnitID = 247
	}
	if input.Address < 0 {
		input.Address = 0
	}
	if input.Address > 65535 {
		input.Address = 65535
	}
	if input.FunctionCode == 0 {
		input.FunctionCode = 3
	}
	switch input.FunctionCode {
	case 1, 2, 3, 4:
	default:
		input.FunctionCode = 3
	}
	input.DataType = strings.ToLower(strings.TrimSpace(input.DataType))
	if input.DataType == "" {
		input.DataType = "uint16"
	}
	if input.Quantity <= 0 {
		input.Quantity = quantityForType(input.DataType)
	}
	if input.Quantity > 125 {
		input.Quantity = 125
	}
	if input.SensorType == "temperature_humidity" && is16BitDataType(input.DataType) && input.Quantity < 2 {
		input.Quantity = 2
	}
	if input.Scale == 0 {
		input.Scale = 1
	}
	if isTemperatureHumiditySensor(input.SensorType) && is16BitDataType(input.DataType) && input.Scale == 1 {
		input.Scale = 0.1
	}
	input.ByteOrder = normalizeEndian(input.ByteOrder)
	input.WordOrder = normalizeEndian(input.WordOrder)
	input.Metric = strings.TrimSpace(input.Metric)
	if input.Metric == "" {
		input.Metric = "value"
	}
	if input.PollIntervalSeconds <= 0 {
		input.PollIntervalSeconds = defaultPollIntervalSeconds
	}
	if input.PollIntervalSeconds < 5 {
		input.PollIntervalSeconds = 5
	}
	if len(input.Signals) == 0 && input.SensorType != "temperature_humidity" {
		input.Signals = []DeviceSignalInput{{
			Name:         input.Metric,
			Metric:       input.Metric,
			Address:      input.Address,
			Quantity:     input.Quantity,
			FunctionCode: input.FunctionCode,
			DataType:     input.DataType,
			ByteOrder:    input.ByteOrder,
			WordOrder:    input.WordOrder,
			Scale:        input.Scale,
			Offset:       input.Offset,
		}}
	}
	if len(input.Signals) > 200 {
		input.Signals = input.Signals[:200]
	}
	for i := range input.Signals {
		input.Signals[i] = normalizeSignalInput(input.Signals[i], i)
	}
	if len(input.Signals) > 0 {
		first := input.Signals[0]
		input.Address = first.Address
		input.Quantity = first.Quantity
		input.FunctionCode = first.FunctionCode
		input.DataType = first.DataType
		input.ByteOrder = first.ByteOrder
		input.WordOrder = first.WordOrder
		input.Scale = first.Scale
		input.Offset = first.Offset
		input.Metric = first.Metric
	}
	return input
}

func normalizeSignalInput(signal DeviceSignalInput, index int) DeviceSignalInput {
	signal.Name = strings.TrimSpace(signal.Name)
	signal.Metric = strings.TrimSpace(signal.Metric)
	if signal.Metric == "" {
		signal.Metric = fmt.Sprintf("signal_%d", index+1)
	}
	if signal.Name == "" {
		signal.Name = signal.Metric
	}
	if signal.Address < 0 {
		signal.Address = 0
	}
	if signal.Address > 65535 {
		signal.Address = 65535
	}
	switch signal.FunctionCode {
	case 1, 2, 3, 4:
	default:
		signal.FunctionCode = 3
	}
	signal.DataType = strings.ToLower(strings.TrimSpace(signal.DataType))
	switch signal.DataType {
	case "uint16", "int16", "uint32", "int32", "float32", "float64":
	default:
		signal.DataType = "uint16"
	}
	if signal.FunctionCode == 1 || signal.FunctionCode == 2 {
		signal.DataType = "uint16"
	}
	signal.Quantity = quantityForType(signal.DataType)
	signal.ByteOrder = normalizeEndian(signal.ByteOrder)
	signal.WordOrder = normalizeEndian(signal.WordOrder)
	if signal.Scale == 0 {
		signal.Scale = 1
	}
	signal.Unit = strings.TrimSpace(signal.Unit)
	return signal
}

func normalizeProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "modbus", "modbus-tcp", "modbus_tcp":
		return "modbus_tcp"
	case "modbus-rtu", "modbus_rtu", "rtu":
		return "modbus_rtu"
	case "modbus-rs485", "modbus_rs485", "rs485":
		return "modbus_rs485"
	case "modbus-rtu-tcp", "modbus_rtu_tcp", "rtu_tcp", "rtu-over-tcp", "rtu_over_tcp":
		return "modbus_rtu_tcp"
	case "mqtt":
		return "mqtt"
	case "rest", "http", "webhook":
		return "rest"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func normalizeEndian(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "little", "le":
		return "little"
	default:
		return "big"
	}
}

func quantityForType(dataType string) int {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "uint32", "int32", "float32":
		return 2
	case "float64":
		return 4
	default:
		return 1
	}
}

func normalizeDeviceForPoll(d Device) Device {
	if strings.ToLower(strings.TrimSpace(d.SensorType)) == "temperature_humidity" && is16BitDataType(d.DataType) && d.Quantity < 2 {
		d.Quantity = 2
	}
	return d
}

func effectiveScale(d Device) float64 {
	if isTemperatureHumiditySensor(d.SensorType) && is16BitDataType(d.DataType) && d.Scale == 1 {
		return 0.1
	}
	if d.Scale == 0 {
		return 1
	}
	return d.Scale
}

func isTemperatureHumiditySensor(sensorType string) bool {
	switch strings.ToLower(strings.TrimSpace(sensorType)) {
	case "temperature", "humidity", "temperature_humidity":
		return true
	default:
		return false
	}
}

func is16BitDataType(dataType string) bool {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "uint16", "int16":
		return true
	default:
		return false
	}
}

func readModbusTCP(d Device) (float64, string, error) {
	if strings.TrimSpace(d.Host) == "" {
		return 0, "", errors.New("modbus host is required")
	}
	switch d.FunctionCode {
	case 1, 2, 3, 4:
	default:
		return 0, "", fmt.Errorf("unsupported modbus function code %d", d.FunctionCode)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.Host, d.Port), 4*time.Second)
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	request := make([]byte, 12)
	// #nosec G115 -- Modbus transaction IDs intentionally use the low 16 bits.
	binary.BigEndian.PutUint16(request[0:2], uint16(time.Now().UnixNano()))
	binary.BigEndian.PutUint16(request[2:4], 0)
	binary.BigEndian.PutUint16(request[4:6], 6)
	request[6] = byte(d.UnitID)                                    // #nosec G115 -- normalized to Modbus range 1..247.
	request[7] = byte(d.FunctionCode)                              // #nosec G115 -- normalized to supported range 1..4.
	binary.BigEndian.PutUint16(request[8:10], uint16(d.Address))   // #nosec G115 -- normalized to 0..65535.
	binary.BigEndian.PutUint16(request[10:12], uint16(d.Quantity)) // #nosec G115 -- normalized to 1..125.
	if _, err := conn.Write(request); err != nil {
		return 0, "", err
	}

	header := make([]byte, 7)
	if _, err := readFull(conn, header); err != nil {
		return 0, "", err
	}
	length := int(binary.BigEndian.Uint16(header[4:6]))
	if length < 2 || length > 260 {
		return 0, "", fmt.Errorf("invalid modbus response length %d", length)
	}
	pdu := make([]byte, length-1)
	if _, err := readFull(conn, pdu); err != nil {
		return 0, "", err
	}
	if pdu[0]&0x80 != 0 {
		return 0, fmt.Sprintf("%x", pdu), fmt.Errorf("modbus exception code %d", pdu[1])
	}
	if pdu[0] != byte(d.FunctionCode) || len(pdu) < 3 { // #nosec G115 -- function code is normalized to 1..4.
		return 0, fmt.Sprintf("%x", pdu), errors.New("unexpected modbus response")
	}
	data := pdu[2:]
	if d.FunctionCode == 1 || d.FunctionCode == 2 {
		value, raw := decodeCoilBits(data, d.Quantity)
		return value, raw, nil
	}
	value, err := decodeRegisters(data, d.DataType, d.ByteOrder, d.WordOrder)
	return value, fmt.Sprintf("%x", data), err
}

func readModbusRTUOverTCP(d Device) (float64, string, error) {
	if strings.TrimSpace(d.Host) == "" {
		return 0, "", errors.New("modbus RTU-over-TCP host is required")
	}
	if d.Port == 0 {
		d.Port = 502
	}
	switch d.FunctionCode {
	case 1, 2, 3, 4:
	default:
		return 0, "", fmt.Errorf("unsupported modbus function code %d", d.FunctionCode)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.Host, d.Port), 4*time.Second)
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	request := make([]byte, 8)
	request[0] = byte(d.UnitID)                                  // #nosec G115 -- normalized to Modbus range 1..247.
	request[1] = byte(d.FunctionCode)                            // #nosec G115 -- normalized to supported range 1..4.
	binary.BigEndian.PutUint16(request[2:4], uint16(d.Address))  // #nosec G115 -- normalized to 0..65535.
	binary.BigEndian.PutUint16(request[4:6], uint16(d.Quantity)) // #nosec G115 -- normalized to 1..125.
	crc := modbusCRC16(request[:6])
	binary.LittleEndian.PutUint16(request[6:8], crc)
	if _, err := conn.Write(request); err != nil {
		return 0, "", err
	}

	header := make([]byte, 3)
	if _, err := readFull(conn, header); err != nil {
		return 0, "", err
	}
	if header[0] != byte(d.UnitID) { // #nosec G115 -- unit ID is normalized to 1..247.
		return 0, fmt.Sprintf("%x", header), fmt.Errorf("unexpected RTU unit id %d", header[0])
	}
	if header[1]&0x80 != 0 {
		rest := make([]byte, 2)
		_, _ = readFull(conn, rest)
		return 0, fmt.Sprintf("%x%x", header, rest), fmt.Errorf("modbus exception code %d", header[2])
	}
	if header[1] != byte(d.FunctionCode) { // #nosec G115 -- function code is normalized to 1..4.
		return 0, fmt.Sprintf("%x", header), errors.New("unexpected RTU response function code")
	}

	byteCount := int(header[2])
	if byteCount > 252 {
		return 0, fmt.Sprintf("%x", header), fmt.Errorf("invalid RTU byte count %d", byteCount)
	}
	rest := make([]byte, byteCount+2)
	if _, err := readFull(conn, rest); err != nil {
		return 0, "", err
	}
	frame := append(append([]byte(nil), header...), rest...)
	got := binary.LittleEndian.Uint16(frame[len(frame)-2:])
	want := modbusCRC16(frame[:len(frame)-2])
	if got != want {
		return 0, fmt.Sprintf("%x", frame), fmt.Errorf("RTU CRC mismatch got 0x%04x want 0x%04x", got, want)
	}

	data := rest[:byteCount]
	if d.FunctionCode == 1 || d.FunctionCode == 2 {
		value, raw := decodeCoilBits(data, d.Quantity)
		return value, raw, nil
	}
	value, err := decodeRegisters(data, d.DataType, d.ByteOrder, d.WordOrder)
	return value, fmt.Sprintf("%x", data), err
}
func readModbusRTU(d Device) (float64, string, error) {
	port := strings.TrimSpace(d.SerialPort)
	if port == "" {
		return 0, "", errors.New("serial_port is required for Modbus RTU/RS485")
	}
	switch d.FunctionCode {
	case 1, 2, 3, 4:
	default:
		return 0, "", fmt.Errorf("unsupported modbus function code %d", d.FunctionCode)
	}

	baud := uint(d.BaudRate)
	if baud == 0 {
		baud = 9600
	}
	dataBits := uint(d.DataBits)
	if dataBits == 0 {
		dataBits = 8
	}
	stopBits := uint(d.StopBits)
	if stopBits == 0 {
		stopBits = 1
	}
	// simonvetter/modbus: PARITY_NONE=0, PARITY_EVEN=1, PARITY_ODD=2
	var parity uint
	switch strings.ToUpper(strings.TrimSpace(d.Parity)) {
	case "E":
		parity = modbusclient.PARITY_EVEN
	case "O":
		parity = modbusclient.PARITY_ODD
	default:
		parity = modbusclient.PARITY_NONE
	}

	// rs485 is a physical-layer variant; framing is identical to RTU.
	url := fmt.Sprintf("rtu://%s", port)

	client, err := modbusclient.NewClient(&modbusclient.ClientConfiguration{
		URL:      url,
		Speed:    baud,
		DataBits: dataBits,
		Parity:   parity,
		StopBits: stopBits,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		return 0, "", fmt.Errorf("modbus RTU client init: %w", err)
	}
	if err := client.Open(); err != nil {
		return 0, "", fmt.Errorf("open RTU %s: %w", port, err)
	}
	defer client.Close()

	if err := client.SetUnitId(uint8(d.UnitID)); err != nil { // #nosec G115 -- normalized to 1..247.
		return 0, "", fmt.Errorf("set unit id: %w", err)
	}

	qty := uint16(d.Quantity) // #nosec G115 -- normalized to 1..125.
	if qty == 0 {
		qty = 1
	}
	addr := uint16(d.Address) // #nosec G115 -- normalized to 0..65535.

	if d.FunctionCode == 1 || d.FunctionCode == 2 {
		var bools []bool
		if d.FunctionCode == 2 {
			bools, err = client.ReadDiscreteInputs(addr, qty)
		} else {
			bools, err = client.ReadCoils(addr, qty)
		}
		if err != nil {
			return 0, "", err
		}
		value, raw := decodeCoilBools(bools)
		return value, raw, nil
	}

	// ReadRawBytes quantity is in bytes, not registers — multiply by 2.
	byteCount := qty * 2
	var data []byte
	if d.FunctionCode == 4 {
		data, err = client.ReadRawBytes(addr, byteCount, modbusclient.INPUT_REGISTER)
	} else {
		data, err = client.ReadRawBytes(addr, byteCount, modbusclient.HOLDING_REGISTER)
	}
	if err != nil {
		return 0, "", err
	}

	value, decErr := decodeRegisters(data, d.DataType, d.ByteOrder, d.WordOrder)
	return value, fmt.Sprintf("%x", data), decErr
}

func modbusCRC16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			if total == len(buf) {
				return total, nil
			}
			return total, err
		}
	}
	return total, nil
}

// decodeCoilBits parses packed coil bits (FC1/FC2) and returns the first coil value.
func decodeCoilBits(data []byte, quantity int) (float64, string) {
	if len(data) == 0 || quantity == 0 {
		return 0, fmt.Sprintf("%x", data)
	}
	firstCoil := float64(data[0] & 1)
	return firstCoil, fmt.Sprintf("%x", data)
}

func decodeCoilBools(bools []bool) (float64, string) {
	if len(bools) == 0 {
		return 0, ""
	}
	byteCount := (len(bools) + 7) / 8
	packed := make([]byte, byteCount)
	for i, b := range bools {
		if b {
			packed[i/8] |= 1 << uint(i%8)
		}
	}
	var first float64
	if bools[0] {
		first = 1
	}
	return first, fmt.Sprintf("%x", packed)
}

type temperatureHumidityReading struct {
	Temperature float64
	Humidity    float64
}

func decodeTemperatureHumidityRaw(raw string, d Device) (temperatureHumidityReading, bool) {
	if strings.ToLower(strings.TrimSpace(d.SensorType)) != "temperature_humidity" || !is16BitDataType(d.DataType) {
		return temperatureHumidityReading{}, false
	}
	raw = strings.TrimSpace(raw)
	if len(raw) < 8 {
		return temperatureHumidityReading{}, false
	}
	data, err := hex.DecodeString(raw)
	if err != nil || len(data) < 4 {
		return temperatureHumidityReading{}, false
	}
	readRegister := func(offset int, signed bool) float64 {
		var value uint16
		if normalizeEndian(d.ByteOrder) == "little" {
			value = binary.BigEndian.Uint16([]byte{data[offset+1], data[offset]})
		} else {
			value = binary.BigEndian.Uint16(data[offset : offset+2])
		}
		if signed {
			return float64(int16(value)) // #nosec G115 -- intentional two's-complement Modbus int16 decode.
		}
		return float64(value)
	}
	scale := effectiveScale(d)
	temperatureOffset := 0
	humidityOffset := 2
	if normalizeEndian(d.WordOrder) == "little" {
		temperatureOffset = 2
		humidityOffset = 0
	}
	return temperatureHumidityReading{
		Temperature: readRegister(temperatureOffset, true)*scale + d.Offset,
		Humidity:    readRegister(humidityOffset, false) * scale,
	}, true
}

func decodeRegisters(data []byte, dataType, byteOrder, wordOrder string) (float64, error) {
	buf := reorderRegisterBytes(data, byteOrder, wordOrder)
	switch strings.ToLower(dataType) {
	case "uint16":
		if len(buf) < 2 {
			return 0, errors.New("not enough data for uint16")
		}
		return float64(binary.BigEndian.Uint16(buf[0:2])), nil
	case "int16":
		if len(buf) < 2 {
			return 0, errors.New("not enough data for int16")
		}
		return float64(int16(binary.BigEndian.Uint16(buf[0:2]))), nil // #nosec G115 -- intentional two's-complement decode.
	case "uint32":
		if len(buf) < 4 {
			return 0, errors.New("not enough data for uint32")
		}
		return float64(binary.BigEndian.Uint32(buf[0:4])), nil
	case "int32":
		if len(buf) < 4 {
			return 0, errors.New("not enough data for int32")
		}
		return float64(int32(binary.BigEndian.Uint32(buf[0:4]))), nil // #nosec G115 -- intentional two's-complement decode.
	case "float32":
		if len(buf) < 4 {
			return 0, errors.New("not enough data for float32")
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(buf[0:4]))), nil
	case "float64":
		if len(buf) < 8 {
			return 0, errors.New("not enough data for float64")
		}
		return math.Float64frombits(binary.BigEndian.Uint64(buf[0:8])), nil
	default:
		return 0, fmt.Errorf("unsupported data type %q", dataType)
	}
}

func reorderRegisterBytes(data []byte, byteOrder, wordOrder string) []byte {
	result := append([]byte(nil), data...)
	if normalizeEndian(byteOrder) == "little" {
		for i := 0; i+1 < len(result); i += 2 {
			result[i], result[i+1] = result[i+1], result[i]
		}
	}
	if normalizeEndian(wordOrder) == "little" && len(result) >= 4 {
		for start, end := 0, len(result)-2; start < end; start, end = start+2, end-2 {
			result[start], result[start+1], result[end], result[end+1] = result[end], result[end+1], result[start], result[start+1]
		}
	}
	return result
}

type deviceScanner interface {
	Scan(dest ...interface{}) error
}

func scanDevice(row deviceScanner) (Device, error) {
	var d Device
	var enabled bool
	var last sql.NullFloat64
	if err := row.Scan(
		&d.ID, &d.Name, &d.Protocol, &d.SensorType, &d.Host, &d.Port,
		&d.SerialPort, &d.BaudRate, &d.DataBits, &d.Parity, &d.StopBits,
		&d.UnitID, &d.Address, &d.Quantity, &d.FunctionCode,
		&d.DataType, &d.ByteOrder, &d.WordOrder, &d.Scale, &d.Offset,
		&d.Metric, &d.Topic, &enabled, &last, &d.LastRaw, &d.LastSeen, &d.LastPolledAt,
		&d.LastError, &d.CreatedAt, &d.UpdatedAt, &d.PollIntervalSeconds,
	); err != nil {
		return Device{}, err
	}
	d.Enabled = enabled
	d.ExternalID = d.Topic
	d.Protocol = normalizeProtocol(d.Protocol)
	switch d.Protocol {
	case "modbus_tcp", "modbus_rtu_tcp":
		d.SerialPort = ""
		d.BaudRate = 0
		d.DataBits = 0
		d.Parity = ""
		d.StopBits = 0
	case "modbus_rtu", "modbus_rs485":
		d.Host = ""
		d.Port = 0
	}
	if last.Valid {
		value := last.Float64
		d.LastValue = &value
	}
	populateLastReadings(&d)
	return d, nil
}

func populateLastReadings(d *Device) {
	if d == nil || strings.TrimSpace(d.LastRaw) == "" {
		return
	}
	if d.FunctionCode == 1 || d.FunctionCode == 2 {
		if readings, ok := decodeBitReadings(d.LastRaw, *d); ok {
			d.LastReadings = readings
			return
		}
	}
	if reading, ok := decodeTemperatureHumidityRaw(d.LastRaw, *d); ok {
		d.LastReadings = []SensorReading{
			{Metric: "temperature", Value: reading.Temperature, Unit: "C", DataType: "int16"},
			{Metric: "humidity", Value: reading.Humidity, Unit: "%", DataType: "uint16"},
		}
	}
}

func decodeBitReadings(raw string, d Device) ([]SensorReading, bool) {
	data, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil || len(data) == 0 || d.Quantity <= 1 {
		return nil, false
	}
	prefix := strings.TrimSpace(d.Metric)
	if prefix == "" || prefix == "value" {
		if d.FunctionCode == 2 {
			prefix = "di"
		} else {
			prefix = "coil"
		}
	}
	readings := make([]SensorReading, 0, d.Quantity)
	for i := 0; i < d.Quantity; i++ {
		var value float64
		if data[i/8]&(1<<uint(i%8)) != 0 {
			value = 1
		}
		readings = append(readings, SensorReading{
			Metric:   fmt.Sprintf("%s_%d", prefix, d.Address+i),
			Value:    value,
			DataType: "bool",
		})
	}
	return readings, true
}

type measurementScanner interface {
	Scan(dest ...interface{}) error
}

func scanMeasurement(row measurementScanner) (Measurement, error) {
	var item Measurement
	var deviceID sql.NullInt64
	if err := row.Scan(
		&item.ID, &item.EventID, &item.SampleID, &deviceID, &item.ExternalID, &item.Metric, &item.Value, &item.RawJSON,
		&item.ForwardStatus, &item.ForwardAttempts, &item.ForwardedAt, &item.DropAfter,
		&item.LastError, &item.CreatedAt,
	); err != nil {
		return Measurement{}, err
	}
	if deviceID.Valid {
		id := int(deviceID.Int64)
		item.DeviceID = &id
	}
	return item, nil
}

func (s *Service) getConfig(key string) string {
	var value string
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = ?`, key).Scan(&value)
	return strings.TrimSpace(value)
}

func (s *Service) setConfig(key, value, description string) error {
	_, err := s.db.Exec(`
		INSERT INTO system_config (config_key, config_value, description, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(config_key) DO UPDATE SET
			config_value = excluded.config_value,
			description = excluded.description,
			updated_at = CURRENT_TIMESTAMP
	`, key, value, description)
	return err
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 500 {
		return msg[:500]
	}
	return msg
}
