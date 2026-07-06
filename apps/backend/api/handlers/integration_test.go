// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"management-server/api/middleware"
	"management-server/config"
	dashboardmodule "management-server/modules/dashboard"
	devicesmodule "management-server/modules/devices"
	iotmodule "management-server/modules/iot"
	licensemodule "management-server/modules/license"
	topologymodule "management-server/modules/topology"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func setupIntegrationTestHandler(t *testing.T) *Handler {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE system_config (
			config_key TEXT PRIMARY KEY,
			config_value TEXT,
			description TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE licenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			is_active INTEGER DEFAULT 0,
			valid_until TEXT,
			license_type TEXT,
			device_count INTEGER DEFAULT 0,
			enabled_features TEXT
		);
		CREATE TABLE devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			sys_name TEXT,
			ip_address TEXT,
			mac_address TEXT,
			device_type TEXT,
			snmp_community TEXT,
			snmp_version INTEGER,
			vendor TEXT,
			model TEXT,
			firmware TEXT,
			is_online INTEGER DEFAULT 0,
			last_seen TEXT,
			image_path TEXT,
			pos_x REAL DEFAULT 0,
			pos_y REAL DEFAULT 0,
			is_name_custom INTEGER DEFAULT 0,
			sys_uptime TEXT,
			sys_location TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE device_interfaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			if_index INTEGER,
			if_name TEXT,
			if_desc TEXT,
			if_speed INTEGER,
			if_mac TEXT,
			if_status INTEGER,
			if_admin_status INTEGER,
			in_octets INTEGER,
			out_octets INTEGER,
			bandwidth_in INTEGER,
			bandwidth_out INTEGER,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE device_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			cpu_usage REAL,
			memory_usage REAL,
			disk_usage REAL,
			collected_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE topology_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_device_id INTEGER,
			target_device_id INTEGER,
			source_if_id INTEGER,
			target_if_id INTEGER,
			source_if_name TEXT,
			target_if_name TEXT,
			link_speed INTEGER,
			bandwidth_usage INTEGER,
			link_type TEXT,
			link_label TEXT,
			is_manual INTEGER DEFAULT 0
		);
		CREATE TABLE events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			event_type TEXT,
			severity TEXT,
			message TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO system_config (config_key, config_value) VALUES ('default_device_limit', '100');
		INSERT INTO devices (
			name, sys_name, ip_address, mac_address, device_type, snmp_community,
			snmp_version, vendor, model, firmware, is_online, last_seen, pos_x, pos_y
		) VALUES (
			'Core Switch', 'core-sw-01', '10.0.0.1', '00:11:22:33:44:55', 'switch', 'private',
			2, 'Edgecore', 'ECS', '1.0.0', 1, '2026-05-25 10:00:00', 10, 20
		);
	`); err != nil {
		t.Fatalf("seed test db: %v", err)
	}

	cfg := &config.Config{
		System: config.SystemConfig{
			Name:    "Test NMS",
			Version: "v1.2.4.7",
		},
		Security: config.SecurityConfig{
			JWTSecret:      "test-secret",
			FrameAncestors: []string{"'self'"},
		},
	}

	return &Handler{
		config:    cfg,
		db:        db,
		dashboard: dashboardmodule.NewService(db, cfg.System.Version, cfg.System.Name),
		devices:   devicesmodule.NewService(db),
		iot:       iotmodule.NewService(db),
		license:   licensemodule.NewService(db, cfg),
		topology:  topologymodule.NewService(db),
	}
}

func performIntegrationRequest(t *testing.T, h *Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/integrations/network-snapshot", h.GetIntegrationNetworkSnapshot)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestGetIntegrationNetworkSnapshotReturnsCombinedPayload(t *testing.T) {
	h := setupIntegrationTestHandler(t)

	recorder := performIntegrationRequest(t, h, "/integrations/network-snapshot")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, expected := range []string{
		`"schema_version":"nms.integration.network_snapshot.v1"`,
		`"version":"v1.2.4.7"`,
		`"dashboard":`,
		`"devices":`,
		`"topology":`,
		`"Core Switch"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s:\n%s", expected, body)
		}
	}
	if strings.Contains(body, "private") || strings.Contains(body, "snmp_community") {
		t.Fatalf("integration response leaked SNMP community:\n%s", body)
	}
}

func TestGetIntegrationNetworkSnapshotSupportsIncludeFilter(t *testing.T) {
	h := setupIntegrationTestHandler(t)

	recorder := performIntegrationRequest(t, h, "/integrations/network-snapshot?include=devices&limit=1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"devices":`) {
		t.Fatalf("response missing devices:\n%s", body)
	}
	if strings.Contains(body, `"dashboard":{`) || strings.Contains(body, `"topology":{"`) {
		t.Fatalf("include filter returned extra sections:\n%s", body)
	}
}

func TestEmbedTokenCanBeListedAndRevoked(t *testing.T) {
	h := setupIntegrationTestHandler(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/integrations/embed-tokens", h.CreateEmbedToken)
	router.GET("/integrations/embed-tokens", h.ListEmbedTokens)
	router.DELETE("/integrations/embed-tokens/:tokenId", h.RevokeEmbedToken)
	router.GET("/integrations/embed-snapshot", middleware.AuthRequired([]byte(h.config.Security.JWTSecret)), h.GetIntegrationEmbedSnapshot)

	createReq := httptest.NewRequest(http.MethodPost, "/integrations/embed-tokens", bytes.NewBufferString(`{"name":"portal","views":["dashboard"],"expires_in_minutes":60}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body: %s", createRec.Code, createRec.Body.String())
	}
	body := createRec.Body.String()
	tokenID := extractJSONValue(body, "token_id")
	token := extractJSONValue(body, "token")
	if tokenID == "" || token == "" {
		t.Fatalf("missing token fields: %s", body)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/integrations/embed-tokens", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), tokenID) {
		t.Fatalf("list failed status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	beforeRevoke := httptest.NewRequest(http.MethodGet, "/integrations/embed-snapshot?view=dashboard", nil)
	beforeRevoke.Header.Set("Authorization", "Bearer "+token)
	beforeRec := httptest.NewRecorder()
	router.ServeHTTP(beforeRec, beforeRevoke)
	if beforeRec.Code != http.StatusOK {
		t.Fatalf("snapshot before revoke status = %d, body: %s", beforeRec.Code, beforeRec.Body.String())
	}

	revokeReq := httptest.NewRequest(http.MethodDelete, "/integrations/embed-tokens/"+tokenID, nil)
	revokeRec := httptest.NewRecorder()
	router.ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body: %s", revokeRec.Code, revokeRec.Body.String())
	}

	afterRevoke := httptest.NewRequest(http.MethodGet, "/integrations/embed-snapshot?view=dashboard", nil)
	afterRevoke.Header.Set("Authorization", "Bearer "+token)
	afterRec := httptest.NewRecorder()
	router.ServeHTTP(afterRec, afterRevoke)
	if afterRec.Code != http.StatusForbidden {
		t.Fatalf("snapshot after revoke status = %d, want %d, body: %s", afterRec.Code, http.StatusForbidden, afterRec.Body.String())
	}
}

func TestIntegrationSettingsUpdateFrameAncestors(t *testing.T) {
	h := setupIntegrationTestHandler(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/integrations/settings", h.GetIntegrationSettings)
	router.PUT("/integrations/settings", h.UpdateIntegrationSettings)

	req := httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewBufferString(`{"frame_ancestors":["'self'","https://customer.example.com","javascript:bad"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "https://customer.example.com") || strings.Contains(body, "javascript:bad") {
		t.Fatalf("unexpected settings body: %s", body)
	}
}

func extractJSONValue(body, key string) string {
	needle := `"` + key + `":"`
	start := strings.Index(body, needle)
	if start < 0 {
		return ""
	}
	start += len(needle)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		return ""
	}
	return body[start : start+end]
}
