package iot

import (
	"database/sql"
	"encoding/binary"
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

func stringID(id int) string {
	return strconv.Itoa(id)
}
