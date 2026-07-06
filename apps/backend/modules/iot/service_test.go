// Made by YTSworks
// YTS工作室製作
package iot

import (
	"database/sql"
	"encoding/binary"
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
