// Made by YTSworks
// YTS工作室製作
package iot

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE system_config (
		config_key TEXT PRIMARY KEY,
		config_value TEXT,
		description TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create system_config: %v", err)
	}
	svc := NewService(db)
	if err := svc.EnsureTables(); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}
	return svc, db
}

func startModbusTCPServer(t *testing.T, registerValue uint16) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen modbus tcp: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				req := make([]byte, 12)
				if _, err := readFull(c, req); err != nil {
					return
				}
				resp := make([]byte, 11)
				copy(resp[0:4], req[0:4])
				binary.BigEndian.PutUint16(resp[4:6], 5)
				resp[6] = req[6]
				resp[7] = req[7]
				resp[8] = 2
				binary.BigEndian.PutUint16(resp[9:11], registerValue)
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func startModbusTCPServerRaw(t *testing.T, data []byte) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen modbus tcp: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				req := make([]byte, 12)
				if _, err := readFull(c, req); err != nil {
					return
				}
				resp := make([]byte, 9+len(data))
				copy(resp[0:4], req[0:4])
				binary.BigEndian.PutUint16(resp[4:6], uint16(3+len(data)))
				resp[6] = req[6]
				resp[7] = req[7]
				resp[8] = byte(len(data))
				copy(resp[9:], data)
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port
}

func TestPollDueDevicesRecordsPendingMeasurement(t *testing.T) {
	svc, _ := setupTestService(t)
	host, port := startModbusTCPServer(t, 253)

	enabled := true
	id, err := svc.CreateDevice(UpsertDeviceInput{
		Name:                "temperature",
		Protocol:            "modbus_tcp",
		Host:                host,
		Port:                port,
		UnitID:              1,
		Address:             0,
		FunctionCode:        3,
		DataType:            "uint16",
		Scale:               0.1,
		Metric:              "temperature",
		PollIntervalSeconds: 5,
		Enabled:             &enabled,
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	svc.PollDueDevices()

	device, err := svc.getDevice(stringID(id))
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.LastValue == nil || *device.LastValue != 25.3 {
		t.Fatalf("last value = %v, want 25.3", device.LastValue)
	}
	items, err := svc.RecentMeasurements(10)
	if err != nil {
		t.Fatalf("recent measurements: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("measurements len = %d, want 1", len(items))
	}
	if items[0].Metric != "temperature" || items[0].ForwardStatus != "pending" || items[0].EventID == "" {
		t.Fatalf("unexpected measurement: %+v", items[0])
	}
}

func TestPollTemperatureHumidityRecordsBothMeasurements(t *testing.T) {
	svc, _ := setupTestService(t)
	host, port := startModbusTCPServerRaw(t, []byte{0x01, 0x27, 0x02, 0x63})

	enabled := true
	id, err := svc.CreateDevice(UpsertDeviceInput{
		Name:                "temp-humidity",
		Protocol:            "modbus_tcp",
		SensorType:          "temperature_humidity",
		Host:                host,
		Port:                port,
		UnitID:              1,
		Address:             0,
		FunctionCode:        4,
		DataType:            "int16",
		ByteOrder:           "big",
		WordOrder:           "big",
		Quantity:            1,
		Scale:               1,
		PollIntervalSeconds: 5,
		Enabled:             &enabled,
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	svc.PollDueDevices()

	device, err := svc.getDevice(stringID(id))
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.LastValue == nil || math.Abs(*device.LastValue-29.5) > 0.000001 {
		t.Fatalf("last value = %v, want 29.5", device.LastValue)
	}
	if device.LastRaw != "01270263" {
		t.Fatalf("last raw = %q, want 01270263", device.LastRaw)
	}
	if len(device.LastReadings) != 2 {
		t.Fatalf("last readings len = %d, want 2: %+v", len(device.LastReadings), device.LastReadings)
	}
	if device.LastReadings[0].Metric != "temperature" || device.LastReadings[0].DataType != "int16" || math.Abs(device.LastReadings[0].Value-29.5) > 0.000001 {
		t.Fatalf("temperature reading = %+v, want int16 29.5", device.LastReadings[0])
	}
	if device.LastReadings[1].Metric != "humidity" || device.LastReadings[1].DataType != "uint16" || math.Abs(device.LastReadings[1].Value-61.1) > 0.000001 {
		t.Fatalf("humidity reading = %+v, want uint16 61.1", device.LastReadings[1])
	}

	items, err := svc.RecentMeasurements(10)
	if err != nil {
		t.Fatalf("recent measurements: %v", err)
	}
	values := map[string]float64{}
	for _, item := range items {
		values[item.Metric] = item.Value
	}
	if math.Abs(values["temperature"]-29.5) > 0.000001 {
		t.Fatalf("temperature measurement = %v, want 29.5", values["temperature"])
	}
	if math.Abs(values["humidity"]-61.1) > 0.000001 {
		t.Fatalf("humidity measurement = %v, want 61.1", values["humidity"])
	}
}

func TestForwardQueueRetrySuccessAndCleanup(t *testing.T) {
	svc, db := setupTestService(t)
	if err := svc.Ingest(IngestInput{
		ExternalID: "sensor-1",
		Name:       "Sensor 1",
		Protocol:   "rest",
		Metric:     "humidity",
		Value:      61.2,
		Raw:        map[string]interface{}{"source": "test"},
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	var fail atomic.Bool
	fail.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Fatalf("missing bearer token")
		}
		if fail.Load() {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		var payload struct {
			DeviceID string             `json:"deviceId"`
			SendTime int64              `json:"sendTime"`
			TagData  map[string]float64 `json:"tagData"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode forward payload: %v", err)
		}
		if payload.DeviceID != "sensor-1" || payload.SendTime <= 0 || payload.TagData["humidity"] != 61.2 {
			t.Errorf("unexpected forward payload: %+v", payload)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	enabled := true
	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{
		Enabled:   &enabled,
		URL:       server.URL,
		Token:     "secret-token",
		BatchSize: 10,
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	if sent, err := svc.FlushForwardQueue(); err == nil || sent != 0 {
		t.Fatalf("flush offline sent=%d err=%v, want err", sent, err)
	}
	status, err := svc.QueueStatus()
	if err != nil {
		t.Fatalf("queue status: %v", err)
	}
	if status.Failed != 1 {
		t.Fatalf("failed count = %d, want 1", status.Failed)
	}

	_, _ = db.Exec(`UPDATE iot_measurements SET next_attempt_at = datetime(CURRENT_TIMESTAMP, '-1 second')`)
	fail.Store(false)
	sent, err := svc.FlushForwardQueue()
	if err != nil {
		t.Fatalf("flush online: %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent = %d, want 1", sent)
	}

	settings, err := svc.ForwarderSettings()
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if !settings.TokenConfigured || settings.URL != server.URL {
		t.Fatalf("settings leaked or lost token state: %+v", settings)
	}

	_, _ = db.Exec(`UPDATE iot_measurements SET drop_after = datetime(CURRENT_TIMESTAMP, '-1 second')`)
	deleted, err := svc.CleanupForwardedMeasurements()
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}

func TestForwardQueuePartialSuccessDoesNotResendCompletedSample(t *testing.T) {
	svc, db := setupTestService(t)
	for _, input := range []IngestInput{
		{ExternalID: "DEV001", Name: "Chiller", Protocol: "rest", Metric: "status", Value: 1},
		{ExternalID: "DEV002", Name: "AC", Protocol: "rest", Metric: "temperature", Value: 24.5},
	} {
		if err := svc.Ingest(input); err != nil {
			t.Fatalf("ingest %s: %v", input.ExternalID, err)
		}
	}

	var allowSecond atomic.Bool
	var dev1Requests atomic.Int32
	var dev2Requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			DeviceID string `json:"deviceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode forward payload: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		switch payload.DeviceID {
		case "DEV001":
			dev1Requests.Add(1)
			w.WriteHeader(http.StatusAccepted)
		case "DEV002":
			dev2Requests.Add(1)
			if !allowSecond.Load() {
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Errorf("unexpected device id %q", payload.DeviceID)
			http.Error(w, "unknown device", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	enabled := true
	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{
		Enabled:   &enabled,
		URL:       server.URL,
		BatchSize: 10,
	}); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	sent, err := svc.FlushForwardQueue()
	if err == nil {
		t.Fatal("first flush error = nil, want partial failure")
	}
	if sent != 1 {
		t.Fatalf("first flush sent = %d, want 1", sent)
	}
	status, err := svc.QueueStatus()
	if err != nil {
		t.Fatalf("queue status after partial failure: %v", err)
	}
	if status.SentHold != 1 || status.Failed != 1 {
		t.Fatalf("queue after partial failure = %+v, want sent_hold=1 failed=1", status)
	}

	allowSecond.Store(true)
	if _, err := db.Exec(`UPDATE iot_measurements SET next_attempt_at = datetime(CURRENT_TIMESTAMP, '-1 second') WHERE forward_status = 'failed'`); err != nil {
		t.Fatalf("make failed sample retryable: %v", err)
	}
	sent, err = svc.FlushForwardQueue()
	if err != nil {
		t.Fatalf("second flush: %v", err)
	}
	if sent != 1 {
		t.Fatalf("second flush sent = %d, want 1", sent)
	}
	if got := dev1Requests.Load(); got != 1 {
		t.Fatalf("DEV001 requests = %d, want 1 (must not be resent)", got)
	}
	if got := dev2Requests.Load(); got != 2 {
		t.Fatalf("DEV002 requests = %d, want 2", got)
	}
}

func TestDeviceSupportsMultipleRegisterSignals(t *testing.T) {
	svc, db := setupTestService(t)
	host, port := startModbusTCPServer(t, 125)
	id, err := svc.CreateDevice(UpsertDeviceInput{
		Name:       "Chiller",
		ExternalID: "DEV001",
		Protocol:   "modbus_tcp",
		Host:       host,
		Port:       port,
		UnitID:     1,
		Signals: []DeviceSignalInput{
			{Name: "Supply temperature", Metric: "chwSupplyTempC", Address: 40001, FunctionCode: 3, DataType: "uint16", Scale: 0.1, Unit: "C"},
			{Name: "Return temperature", Metric: "chwReturnTempC", Address: 40002, FunctionCode: 3, DataType: "uint16", Scale: 0.1, Unit: "C"},
		},
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	device, err := svc.getDevice(strconv.Itoa(id))
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.ExternalID != "DEV001" || len(device.Signals) != 2 {
		t.Fatalf("unexpected device signals: %+v", device)
	}
	if err := svc.pollDevice(device); err != nil {
		t.Fatalf("poll signals: %v", err)
	}
	var count int
	var distinctSamples int
	if err := db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT sample_id)
		FROM iot_measurements
		WHERE device_id = ?
	`, id).Scan(&count, &distinctSamples); err != nil {
		t.Fatalf("count measurements: %v", err)
	}
	if count != 2 || distinctSamples != 1 {
		t.Fatalf("measurement count=%d sample groups=%d, want 2/1", count, distinctSamples)
	}

	var requests atomic.Int32
	var received struct {
		DeviceID string             `json:"deviceId"`
		SendTime int64              `json:"sendTime"`
		TagData  map[string]float64 `json:"tagData"`
	}
	var idempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		idempotencyKey = r.Header.Get("X-NMS-Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode multi-signal payload: %v", err)
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	enabled := true
	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{
		Enabled:   &enabled,
		URL:       server.URL,
		BatchSize: 10,
	}); err != nil {
		t.Fatalf("update forward settings: %v", err)
	}
	sent, err := svc.FlushForwardQueue()
	if err != nil {
		t.Fatalf("forward multi-signal sample: %v", err)
	}
	if sent != 2 || requests.Load() != 1 {
		t.Fatalf("forwarded measurements=%d requests=%d, want 2/1", sent, requests.Load())
	}
	if received.DeviceID != "DEV001" || received.SendTime <= 0 || idempotencyKey == "" {
		t.Fatalf("unexpected multi-signal envelope: payload=%+v idempotency=%q", received, idempotencyKey)
	}
	if len(received.TagData) != 2 || received.TagData["chwSupplyTempC"] != 12.5 || received.TagData["chwReturnTempC"] != 12.5 {
		t.Fatalf("unexpected tagData: %+v", received.TagData)
	}
}

func TestForwarderIntervalBounds(t *testing.T) {
	svc, _ := setupTestService(t)
	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{IntervalMilliseconds: 20}); err != nil {
		t.Fatalf("set short interval: %v", err)
	}
	settings, err := svc.ForwarderSettings()
	if err != nil {
		t.Fatalf("get short interval: %v", err)
	}
	if settings.IntervalMilliseconds != 100 {
		t.Fatalf("short interval = %d ms, want 100 ms", settings.IntervalMilliseconds)
	}

	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{IntervalMilliseconds: 90000000}); err != nil {
		t.Fatalf("set long interval: %v", err)
	}
	settings, err = svc.ForwarderSettings()
	if err != nil {
		t.Fatalf("get long interval: %v", err)
	}
	if settings.IntervalMilliseconds != 86400000 {
		t.Fatalf("long interval = %d ms, want 86400000 ms", settings.IntervalMilliseconds)
	}

	if err := svc.UpdateForwarderSettings(ForwarderSettingsInput{IntervalSeconds: 7}); err != nil {
		t.Fatalf("set legacy seconds interval: %v", err)
	}
	settings, err = svc.ForwarderSettings()
	if err != nil {
		t.Fatalf("get legacy seconds interval: %v", err)
	}
	if settings.IntervalMilliseconds != 7000 {
		t.Fatalf("legacy interval = %d ms, want 7000 ms", settings.IntervalMilliseconds)
	}
}

func TestFormatForwardTimeReturnsUnixMilliseconds(t *testing.T) {
	if got := formatForwardTime("1970-01-01 00:00:01"); got != 1000 {
		t.Fatalf("formatForwardTime = %d, want 1000", got)
	}
	if got := formatForwardTime("2026-07-23T10:30:00.123Z"); got != 1784802600123 {
		t.Fatalf("formatForwardTime with milliseconds = %d, want 1784802600123", got)
	}
}

func TestEnsureTablesUpgradesLegacyIoTSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE system_config (
			config_key TEXT PRIMARY KEY,
			config_value TEXT,
			description TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE iot_devices (
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
		)`,
		`CREATE TABLE iot_measurements (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			external_id TEXT DEFAULT '',
			metric TEXT NOT NULL DEFAULT 'value',
			value REAL NOT NULL,
			raw_json TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO system_config (config_key, config_value, description)
		VALUES ('iot_forward_interval_seconds', '7', 'legacy interval')
	`); err != nil {
		t.Fatalf("insert legacy forwarding interval: %v", err)
	}

	svc := NewService(db)
	if err := svc.EnsureTables(); err != nil {
		t.Fatalf("upgrade legacy schema: %v", err)
	}
	var sampleIDColumn int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('iot_measurements')
		WHERE name = 'sample_id'
	`).Scan(&sampleIDColumn); err != nil {
		t.Fatalf("inspect upgraded measurements: %v", err)
	}
	if sampleIDColumn != 1 {
		t.Fatalf("sample_id columns = %d, want 1", sampleIDColumn)
	}
	var signalTable int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'iot_device_signals'
	`).Scan(&signalTable); err != nil {
		t.Fatalf("inspect signal table: %v", err)
	}
	if signalTable != 1 {
		t.Fatalf("iot_device_signals tables = %d, want 1", signalTable)
	}
	var intervalMilliseconds string
	if err := db.QueryRow(`
		SELECT config_value
		FROM system_config
		WHERE config_key = 'iot_forward_interval_ms'
	`).Scan(&intervalMilliseconds); err != nil {
		t.Fatalf("read migrated forwarding interval: %v", err)
	}
	if intervalMilliseconds != "7000" {
		t.Fatalf("migrated forwarding interval = %q, want 7000", intervalMilliseconds)
	}
}

func TestModbusWireAddressSupportsReferenceNotation(t *testing.T) {
	tests := []struct {
		name         string
		address      int
		functionCode int
		want         int
	}{
		{name: "holding register 40001", address: 40001, functionCode: 3, want: 0},
		{name: "holding register 40010", address: 40010, functionCode: 3, want: 9},
		{name: "input register 30001", address: 30001, functionCode: 4, want: 0},
		{name: "discrete input 10001", address: 10001, functionCode: 2, want: 0},
		{name: "already zero based", address: 12, functionCode: 3, want: 12},
		{name: "reference does not match function", address: 40001, functionCode: 4, want: 40001},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modbusWireAddress(tt.address, tt.functionCode); got != tt.want {
				t.Fatalf("modbusWireAddress(%d, %d) = %d, want %d", tt.address, tt.functionCode, got, tt.want)
			}
		})
	}
}

func TestTemperatureHumidityDefaultsUseManualRegisterShape(t *testing.T) {
	enabled := true
	input := normalizeInput(UpsertDeviceInput{
		Name:       "temp-humidity",
		Protocol:   "modbus_rs485",
		SensorType: "temperature_humidity",
		DataType:   "int16",
		Scale:      1,
		Enabled:    &enabled,
	})

	if input.FunctionCode != 3 {
		t.Fatalf("function code = %d, want 3 before UI override", input.FunctionCode)
	}
	if input.Quantity != 2 {
		t.Fatalf("quantity = %d, want 2", input.Quantity)
	}
	if input.Scale != 0.1 {
		t.Fatalf("scale = %v, want 0.1", input.Scale)
	}
}

func TestNormalizeInputKeepsProtocolConnectionFieldsExclusive(t *testing.T) {
	t.Run("tcp clears serial settings", func(t *testing.T) {
		input := normalizeInput(UpsertDeviceInput{
			Protocol:   "modbus_tcp",
			Host:       "192.168.1.20",
			Port:       502,
			SerialPort: "COM9",
			BaudRate:   19200,
			DataBits:   8,
			Parity:     "E",
			StopBits:   1,
		})
		if input.Host != "192.168.1.20" || input.Port != 502 {
			t.Fatalf("tcp endpoint changed: host=%q port=%d", input.Host, input.Port)
		}
		if input.SerialPort != "" || input.BaudRate != 0 || input.DataBits != 0 || input.Parity != "" || input.StopBits != 0 {
			t.Fatalf("tcp retained serial fields: %+v", input)
		}
	})

	t.Run("rtu over tcp clears serial settings", func(t *testing.T) {
		input := normalizeInput(UpsertDeviceInput{
			Protocol:   "modbus_rtu_tcp",
			Host:       "192.168.1.21",
			Port:       1502,
			SerialPort: "COM8",
			BaudRate:   9600,
		})
		if input.Host != "192.168.1.21" || input.Port != 1502 || input.SerialPort != "" || input.BaudRate != 0 {
			t.Fatalf("rtu over tcp fields not normalized: %+v", input)
		}
	})

	t.Run("rs485 clears tcp endpoint", func(t *testing.T) {
		input := normalizeInput(UpsertDeviceInput{
			Protocol:   "rs485",
			Host:       "192.168.1.22",
			Port:       502,
			SerialPort: "COM3",
			BaudRate:   38400,
			DataBits:   8,
			Parity:     "N",
			StopBits:   1,
		})
		if input.Protocol != "modbus_rs485" {
			t.Fatalf("protocol = %q, want modbus_rs485", input.Protocol)
		}
		if input.Host != "" || input.Port != 0 {
			t.Fatalf("rs485 retained tcp endpoint: host=%q port=%d", input.Host, input.Port)
		}
		if input.SerialPort != "COM3" || input.BaudRate != 38400 {
			t.Fatalf("rs485 serial settings changed: %+v", input)
		}
	})
}

func TestListDevicesHidesLegacyFieldsFromOtherTransport(t *testing.T) {
	svc, db := setupTestService(t)
	if _, err := db.Exec(`
		INSERT INTO iot_devices (
			name, protocol, host, port, serial_port, baud_rate, data_bits, parity, stop_bits,
			unit_id, address, quantity, data_type, scale, topic, enabled
		) VALUES
			('Legacy TCP', 'modbus_tcp', '192.168.1.30', 502, 'COM8', 19200, 8, 'E', 1, 1, 0, 1, 'uint16', 1, 'TCP001', 1),
			('Legacy RS485', 'rs485', '192.168.1.31', 1502, 'COM3', 38400, 8, 'N', 1, 1, 0, 1, 'uint16', 1, 'RTU001', 1)
	`); err != nil {
		t.Fatalf("insert legacy mixed transports: %v", err)
	}

	devices, err := svc.ListDevices()
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("devices len = %d, want 2", len(devices))
	}
	byProtocol := make(map[string]Device, len(devices))
	for _, device := range devices {
		byProtocol[device.Protocol] = device
	}
	tcp := byProtocol["modbus_tcp"]
	if tcp.Host != "192.168.1.30" || tcp.Port != 502 || tcp.SerialPort != "" || tcp.BaudRate != 0 {
		t.Fatalf("tcp response mixed transport fields: %+v", tcp)
	}
	rs485 := byProtocol["modbus_rs485"]
	if rs485.Host != "" || rs485.Port != 0 || rs485.SerialPort != "COM3" || rs485.BaudRate != 38400 {
		t.Fatalf("rs485 response mixed transport fields: %+v", rs485)
	}
}

func TestDecodeSignedTemperatureRegister(t *testing.T) {
	value, err := decodeRegisters([]byte{0xFF, 0x9B}, "int16", "big", "big")
	if err != nil {
		t.Fatalf("decode int16: %v", err)
	}
	actual := value * effectiveScale(Device{SensorType: "temperature", DataType: "int16", Scale: 1})
	if math.Abs(actual-(-10.1)) > 0.000001 {
		t.Fatalf("actual = %v, want -10.1", actual)
	}
}

func TestDecodeManualTemperatureHumidityRegisterOrder(t *testing.T) {
	value, err := decodeRegisters([]byte{0x01, 0x27, 0x02, 0x63}, "int16", "big", "big")
	if err != nil {
		t.Fatalf("decode manual int16 pair: %v", err)
	}
	actual := value * effectiveScale(Device{SensorType: "temperature_humidity", DataType: "int16", Scale: 1})
	if math.Abs(actual-29.5) > 0.000001 {
		t.Fatalf("actual = %v, want 29.5", actual)
	}

	humidityRaw := float64(binary.BigEndian.Uint16([]byte{0x02, 0x63}))
	humidity := humidityRaw * effectiveScale(Device{SensorType: "temperature_humidity", DataType: "int16", Scale: 1})
	if math.Abs(humidity-61.1) > 0.000001 {
		t.Fatalf("humidity = %v, want 61.1", humidity)
	}
}

func startModbusRTUOverTCPServerRaw(t *testing.T, data []byte) (host string, port int, gotRequest *atomic.Value) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen modbus rtu over tcp: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	var requestValue atomic.Value
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				req := make([]byte, 8)
				if _, err := readFull(c, req); err != nil {
					return
				}
				requestValue.Store(append([]byte(nil), req...))
				resp := make([]byte, 3+len(data)+2)
				resp[0] = req[0]
				resp[1] = req[1]
				resp[2] = byte(len(data))
				copy(resp[3:], data)
				crc := modbusCRC16(resp[:len(resp)-2])
				binary.LittleEndian.PutUint16(resp[len(resp)-2:], crc)
				_, _ = c.Write(resp)
			}(conn)
		}
	}()

	addr := ln.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, &requestValue
}

func TestPollRTUOverTCPUsesRTUFrameWithCRC(t *testing.T) {
	svc, _ := setupTestService(t)
	host, port, gotRequest := startModbusRTUOverTCPServerRaw(t, []byte{0x01, 0x27, 0x02, 0x63})

	enabled := true
	id, err := svc.CreateDevice(UpsertDeviceInput{
		Name:                "jy-dam-through-gateway",
		Protocol:            "modbus_rtu_tcp",
		SensorType:          "temperature_humidity",
		Host:                host,
		Port:                port,
		UnitID:              1,
		Address:             0,
		FunctionCode:        4,
		DataType:            "int16",
		ByteOrder:           "big",
		WordOrder:           "big",
		Quantity:            2,
		Scale:               0.1,
		PollIntervalSeconds: 5,
		Enabled:             &enabled,
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	device, err := svc.PollDevice(stringID(id))
	if err != nil {
		t.Fatalf("poll rtu over tcp: %v", err)
	}
	request, _ := gotRequest.Load().([]byte)
	wantRequest := []byte{0x01, 0x04, 0x00, 0x00, 0x00, 0x02, 0x71, 0xCB}
	if string(request) != string(wantRequest) {
		t.Fatalf("request = % x, want % x", request, wantRequest)
	}
	if device.LastRaw != "01270263" {
		t.Fatalf("last raw = %q, want 01270263", device.LastRaw)
	}
	if len(device.LastReadings) != 2 {
		t.Fatalf("last readings len = %d, want 2", len(device.LastReadings))
	}
}
func TestDecodeBitReadingsFromPackedRaw(t *testing.T) {
	readings, ok := decodeBitReadings("05", Device{FunctionCode: 2, Address: 0, Quantity: 8, Metric: "di"})
	if !ok {
		t.Fatal("decode bit readings failed")
	}
	if len(readings) != 8 {
		t.Fatalf("readings len = %d, want 8", len(readings))
	}
	want := []float64{1, 0, 1, 0, 0, 0, 0, 0}
	for i := range want {
		if readings[i].Value != want[i] {
			t.Fatalf("reading %d = %v, want %v", i, readings[i].Value, want[i])
		}
	}
	if readings[2].Metric != "di_2" || readings[2].DataType != "bool" {
		t.Fatalf("unexpected reading metadata: %+v", readings[2])
	}
}
func stringID(id int) string {
	return strconv.Itoa(id)
}
