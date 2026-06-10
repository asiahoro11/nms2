package iot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPollIntervalSeconds = 60
	defaultForwardBatchSize    = 50
	defaultSentRetentionMins   = 10
)

type Service struct {
	db *sql.DB

	loopOnce sync.Once
	stopCh   chan struct{}
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, stopCh: make(chan struct{})}
}

func (s *Service) EnsureTables() error {
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
		`ALTER TABLE iot_measurements ADD COLUMN forward_status TEXT DEFAULT 'pending'`,
		`ALTER TABLE iot_measurements ADD COLUMN forward_attempts INTEGER DEFAULT 0`,
		`ALTER TABLE iot_measurements ADD COLUMN forwarded_at DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN drop_after DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN next_attempt_at DATETIME`,
		`ALTER TABLE iot_measurements ADD COLUMN last_error TEXT DEFAULT ''`,
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

	configs := []struct {
		key, value, desc string
	}{
		{"iot_forward_enabled", "false", "Enable IoT HTTP forwarder"},
		{"iot_forward_url", "", "IoT HTTP forward webhook URL"},
		{"iot_forward_token", "", "IoT HTTP forward bearer token"},
		{"iot_forward_batch_size", strconv.Itoa(defaultForwardBatchSize), "IoT forward batch size"},
		{"iot_forward_sent_retention_minutes", strconv.Itoa(defaultSentRetentionMins), "IoT sent record retention in minutes"},
	}
	for _, cfg := range configs {
		if _, err := s.db.Exec(`
			INSERT OR IGNORE INTO system_config (config_key, config_value, description)
			VALUES (?, ?, ?)
		`, cfg.key, cfg.value, cfg.desc); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) StartBackgroundLoop() {
	s.loopOnce.Do(func() {
		go s.backgroundLoop()
	})
}

func (s *Service) backgroundLoop() {
	pollTicker := time.NewTicker(1 * time.Second)
	forwardTicker := time.NewTicker(10 * time.Second)
	cleanupTicker := time.NewTicker(1 * time.Minute)
	defer pollTicker.Stop()
	defer forwardTicker.Stop()
	defer cleanupTicker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-pollTicker.C:
			s.PollDueDevices()
		case <-forwardTicker.C:
			s.FlushForwardQueue()
		case <-cleanupTicker.C:
			s.CleanupForwardedMeasurements()
		}
	}
}

func (s *Service) Status() (Status, error) {
	if err := s.EnsureTables(); err != nil {
		return Status{}, err
	}
	settings, _ := s.ForwarderSettings()
	var status Status
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices`).Scan(&status.DeviceCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_devices WHERE enabled = 1`).Scan(&status.EnabledCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements`).Scan(&status.MeasurementCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, 'pending') = 'pending'`).Scan(&status.ForwardPendingCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'failed'`).Scan(&status.ForwardFailedCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM iot_measurements WHERE COALESCE(forward_status, '') = 'sent'`).Scan(&status.ForwardSentHoldCount)
	status.ForwardEnabled = settings.Enabled
	status.ForwardURLConfigured = strings.TrimSpace(settings.URL) != ""
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
			Description: "Background LAN polling for meters, PLCs, UPS, environmental sensors, and Modbus-capable equipment. Supports FC03 and FC04.",
			DataTypes:   []string{"uint16", "int16", "uint32", "int32", "float32"},
			Endpoint:    "/api/v1/iot/devices",
		},
		{
			Protocol:    "rest",
			Name:        "REST / Webhook",
			Mode:        "gateway_ingest",
			Status:      "ready",
			Description: "Generic push path for customer systems and IoT gateways. Measurements enter the same local forward queue.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "http_forward",
			Name:        "HTTP Forward Queue",
			Mode:        "store_and_forward",
			Status:      "ready",
			Description: "Durable local buffering, offline retry, and successful-record cleanup after 10 minutes.",
			Endpoint:    "/api/v1/iot/forwarder/settings",
		},
		{
			Protocol:    "mqtt",
			Name:        "MQTT Bridge",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Use an MQTT bridge to normalize topics into the REST ingest contract.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "opcua",
			Name:        "OPC-UA Gateway",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Use an OPC-UA gateway or edge script to push node values into the ingest API.",
			Endpoint:    "/api/v1/iot/ingest",
		},
		{
			Protocol:    "bacnet",
			Name:        "BACnet Gateway",
			Mode:        "gateway_ingest",
			Status:      "bridge_ready",
			Description: "Use a BACnet/IP gateway or collector to push building automation points into the ingest API.",
			Endpoint:    "/api/v1/iot/ingest",
		},
	}
}

func (s *Service) ListDevices() ([]Device, error) {
	if err := s.EnsureTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
		SELECT id, name, protocol, host, port, unit_id, address, quantity, COALESCE(function_code, 3),
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
	return devices, rows.Err()
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
	res, err := s.db.Exec(`
		INSERT INTO iot_devices (
			name, protocol, host, port, unit_id, address, quantity, function_code, data_type,
			byte_order, word_order, scale, offset, metric, topic, poll_interval_seconds, enabled
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, normalized.Name, normalized.Protocol, normalized.Host, normalized.Port, normalized.UnitID,
		normalized.Address, normalized.Quantity, normalized.FunctionCode, normalized.DataType,
		normalized.ByteOrder, normalized.WordOrder, normalized.Scale, normalized.Offset,
		normalized.Metric, normalized.Topic, normalized.PollIntervalSeconds, enabled)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Service) UpdateDevice(id string, input UpsertDeviceInput) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	normalized := normalizeInput(input)
	enabled := true
	if normalized.Enabled != nil {
		enabled = *normalized.Enabled
	}
	res, err := s.db.Exec(`
		UPDATE iot_devices
		SET name = ?, protocol = ?, host = ?, port = ?, unit_id = ?, address = ?,
		    quantity = ?, function_code = ?, data_type = ?, byte_order = ?, word_order = ?,
		    scale = ?, offset = ?, metric = ?, topic = ?, poll_interval_seconds = ?,
		    enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, normalized.Name, normalized.Protocol, normalized.Host, normalized.Port, normalized.UnitID,
		normalized.Address, normalized.Quantity, normalized.FunctionCode, normalized.DataType,
		normalized.ByteOrder, normalized.WordOrder, normalized.Scale, normalized.Offset,
		normalized.Metric, normalized.Topic, normalized.PollIntervalSeconds, enabled, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Service) DeleteDevice(id string) error {
	if err := s.EnsureTables(); err != nil {
		return err
	}
	res, err := s.db.Exec(`DELETE FROM iot_devices WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
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
		SELECT id, name, protocol, host, port, unit_id, address, quantity, COALESCE(function_code, 3),
		       data_type, COALESCE(byte_order, 'big'), COALESCE(word_order, 'big'), scale, COALESCE(offset, 0),
		       COALESCE(metric, 'value'), topic, enabled, last_value, COALESCE(last_raw, ''),
		       COALESCE(last_seen, ''), COALESCE(last_polled_at, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, ''), COALESCE(updated_at, ''),
		       COALESCE(poll_interval_seconds, ?)
		FROM iot_devices
		WHERE enabled = 1
		  AND protocol = 'modbus_tcp'
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
	defer rows.Close()

	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			continue
		}
		_ = s.pollDevice(d)
	}
}

func (s *Service) pollDevice(d Device) error {
	if !d.Enabled {
		return errors.New("iot device is disabled")
	}
	var value float64
	var raw string
	var err error
	switch d.Protocol {
	case "modbus_tcp":
		value, raw, err = readModbusTCP(d)
	default:
		err = fmt.Errorf("protocol %q does not support active polling", d.Protocol)
	}
	if err != nil {
		_, _ = s.db.Exec(`
			UPDATE iot_devices
			SET last_error = ?, last_polled_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, err.Error(), d.ID)
		return err
	}
	value = (value * d.Scale) + d.Offset
	metric := strings.TrimSpace(d.Metric)
	if metric == "" {
		metric = "value"
	}
	if _, err := s.recordMeasurement(&d.ID, "", metric, value, raw); err != nil {
		return err
	}
	_, err = s.db.Exec(`
		UPDATE iot_devices
		SET last_value = ?, last_raw = ?, last_seen = CURRENT_TIMESTAMP,
		    last_polled_at = CURRENT_TIMESTAMP, last_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, value, raw, d.ID)
	return err
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
		SELECT id, COALESCE(event_id, ''), device_id, external_id, metric, value, COALESCE(raw_json, ''),
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
	if batch, err := strconv.Atoi(s.getConfig("iot_forward_batch_size")); err == nil && batch > 0 && batch <= 500 {
		settings.BatchSize = batch
	}
	if mins, err := strconv.Atoi(s.getConfig("iot_forward_sent_retention_minutes")); err == nil && mins > 0 {
		settings.RetentionMinutes = mins
	}
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
	return nil
}

func (s *Service) FlushForwardQueue() (int, error) {
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
	rows, err := s.db.Query(`
		SELECT id, COALESCE(event_id, ''), device_id, external_id, metric, value, COALESCE(raw_json, ''),
		       COALESCE(forward_status, 'pending'), COALESCE(forward_attempts, 0),
		       COALESCE(forwarded_at, ''), COALESCE(drop_after, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, '')
		FROM iot_measurements
		WHERE COALESCE(forward_status, 'pending') IN ('pending', 'failed')
		  AND (next_attempt_at IS NULL OR next_attempt_at = '' OR next_attempt_at <= CURRENT_TIMESTAMP)
		ORDER BY id ASC
		LIMIT ?
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
	if err := s.forwardMeasurements(settings, items); err != nil {
		msg := truncateError(err)
		for _, item := range items {
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
		return 0, err
	}
	ids := make([]interface{}, 0, len(items))
	placeholders := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
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
	if _, err := s.db.Exec(query, ids...); err != nil {
		return 0, err
	}
	return len(items), nil
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

func (s *Service) forwardMeasurements(settings ForwarderSettings, items []Measurement) error {
	payload := map[string]interface{}{
		"schema_version": "nms.iot.forward.v1",
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"records":        items,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-NMS-Idempotency-Key", items[0].EventID)
	if token := s.getConfig("iot_forward_token"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("forward target returned %d %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
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
		SELECT id, name, protocol, host, port, unit_id, address, quantity, COALESCE(function_code, 3),
		       data_type, COALESCE(byte_order, 'big'), COALESCE(word_order, 'big'), scale, COALESCE(offset, 0),
		       COALESCE(metric, 'value'), topic, enabled, last_value, COALESCE(last_raw, ''),
		       COALESCE(last_seen, ''), COALESCE(last_polled_at, ''), COALESCE(last_error, ''),
		       COALESCE(created_at, ''), COALESCE(updated_at, ''),
		       COALESCE(poll_interval_seconds, ?)
		FROM iot_devices
		WHERE id = ?
	`, defaultPollIntervalSeconds, id)
	return scanDevice(row)
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
	var id interface{}
	if deviceID != nil {
		id = *deviceID
	}
	eventID := uuid.NewString()
	_, err := s.db.Exec(`
		INSERT INTO iot_measurements (
			event_id, device_id, external_id, metric, value, raw_json, forward_status, forward_attempts
		)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', 0)
	`, eventID, id, externalID, metric, value, raw)
	return eventID, err
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
	if input.Port == 0 && input.Protocol == "modbus_tcp" {
		input.Port = 502
	}
	if input.UnitID <= 0 {
		input.UnitID = 1
	}
	if input.FunctionCode == 0 {
		input.FunctionCode = 3
	}
	if input.FunctionCode != 3 && input.FunctionCode != 4 {
		input.FunctionCode = 3
	}
	if input.Quantity <= 0 {
		input.Quantity = quantityForType(input.DataType)
	}
	if input.Scale == 0 {
		input.Scale = 1
	}
	input.DataType = strings.ToLower(strings.TrimSpace(input.DataType))
	if input.DataType == "" {
		input.DataType = "uint16"
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
	return input
}

func normalizeProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "modbus", "modbus-tcp", "modbus_tcp":
		return "modbus_tcp"
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
	default:
		return 1
	}
}

func readModbusTCP(d Device) (float64, string, error) {
	if strings.TrimSpace(d.Host) == "" {
		return 0, "", errors.New("modbus host is required")
	}
	if d.FunctionCode != 3 && d.FunctionCode != 4 {
		return 0, "", fmt.Errorf("unsupported modbus function code %d", d.FunctionCode)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.Host, d.Port), 4*time.Second)
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	request := make([]byte, 12)
	binary.BigEndian.PutUint16(request[0:2], uint16(time.Now().UnixNano()))
	binary.BigEndian.PutUint16(request[2:4], 0)
	binary.BigEndian.PutUint16(request[4:6], 6)
	request[6] = byte(d.UnitID)
	request[7] = byte(d.FunctionCode)
	binary.BigEndian.PutUint16(request[8:10], uint16(d.Address))
	binary.BigEndian.PutUint16(request[10:12], uint16(d.Quantity))
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
	if pdu[0] != byte(d.FunctionCode) || len(pdu) < 3 {
		return 0, fmt.Sprintf("%x", pdu), errors.New("unexpected modbus response")
	}
	data := pdu[2:]
	value, err := decodeRegisters(data, d.DataType, d.ByteOrder, d.WordOrder)
	return value, fmt.Sprintf("%x", data), err
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func decodeRegisters(data []byte, dataType, byteOrder, wordOrder string) (float64, error) {
	bytes := reorderRegisterBytes(data, byteOrder, wordOrder)
	switch strings.ToLower(dataType) {
	case "uint16":
		if len(bytes) < 2 {
			return 0, errors.New("not enough data for uint16")
		}
		return float64(binary.BigEndian.Uint16(bytes[0:2])), nil
	case "int16":
		if len(bytes) < 2 {
			return 0, errors.New("not enough data for int16")
		}
		return float64(int16(binary.BigEndian.Uint16(bytes[0:2]))), nil
	case "uint32":
		if len(bytes) < 4 {
			return 0, errors.New("not enough data for uint32")
		}
		return float64(binary.BigEndian.Uint32(bytes[0:4])), nil
	case "int32":
		if len(bytes) < 4 {
			return 0, errors.New("not enough data for int32")
		}
		return float64(int32(binary.BigEndian.Uint32(bytes[0:4]))), nil
	case "float32":
		if len(bytes) < 4 {
			return 0, errors.New("not enough data for float32")
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(bytes[0:4]))), nil
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
		&d.ID, &d.Name, &d.Protocol, &d.Host, &d.Port, &d.UnitID, &d.Address, &d.Quantity,
		&d.FunctionCode, &d.DataType, &d.ByteOrder, &d.WordOrder, &d.Scale, &d.Offset,
		&d.Metric, &d.Topic, &enabled, &last, &d.LastRaw, &d.LastSeen, &d.LastPolledAt,
		&d.LastError, &d.CreatedAt, &d.UpdatedAt, &d.PollIntervalSeconds,
	); err != nil {
		return Device{}, err
	}
	d.Enabled = enabled
	if last.Valid {
		value := last.Float64
		d.LastValue = &value
	}
	return d, nil
}

type measurementScanner interface {
	Scan(dest ...interface{}) error
}

func scanMeasurement(row measurementScanner) (Measurement, error) {
	var item Measurement
	var deviceID sql.NullInt64
	if err := row.Scan(
		&item.ID, &item.EventID, &deviceID, &item.ExternalID, &item.Metric, &item.Value, &item.RawJSON,
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
