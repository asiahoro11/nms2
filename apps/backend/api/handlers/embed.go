// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	devicesmodule "management-server/modules/devices"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type createEmbedTokenInput struct {
	Name             string   `json:"name"`
	Views            []string `json:"views"`
	ExpiresInMinutes int      `json:"expires_in_minutes"`
}

type integrationSettingsInput struct {
	FrameAncestors []string `json:"frame_ancestors"`
}

type embedTokenRecord struct {
	ID         int      `json:"id"`
	TokenID    string   `json:"token_id"`
	Name       string   `json:"name"`
	Views      []string `json:"views"`
	ExpiresAt  string   `json:"expires_at"`
	RevokedAt  string   `json:"revoked_at,omitempty"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

func (h *Handler) CreateEmbedToken(c *gin.Context) {
	var input createEmbedTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	views := normalizeEmbedViews(input.Views)
	if len(views) == 0 {
		views = []string{"dashboard", "topology", "devices", "alerts", "iot"}
	}
	minutes := input.ExpiresInMinutes
	if minutes <= 0 {
		minutes = 1440
	}
	if minutes > 43200 {
		minutes = 43200
	}
	expiresAt := time.Now().Add(time.Duration(minutes) * time.Minute)
	tokenID, err := randomTokenID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	claims := jwt.MapClaims{
		"user_id":  0,
		"username": "embed",
		"role":     "viewer",
		"embed":    true,
		"views":    strings.Join(views, ","),
		"jti":      tokenID,
		"iss":      "management-server",
		"iat":      time.Now().Unix(),
		"exp":      expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.config.Security.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.saveEmbedTokenRecord(tokenID, input.Name, views, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{
		"token_id":   tokenID,
		"token":      tokenString,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"url":        "/embed.html?view=" + views[0] + "&token=" + tokenString,
		"views":      views,
	}})
}

func (h *Handler) ListEmbedTokens(c *gin.Context) {
	records, err := h.listEmbedTokenRecords()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: records})
}

func (h *Handler) GetIntegrationSettings(c *gin.Context) {
	frameAncestors := h.integrationFrameAncestors()
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{
		"frame_ancestors":              frameAncestors,
		"effective_frame_ancestors":    strings.Join(frameAncestors, " "),
		"supported_embed_views":        []string{"dashboard", "topology", "devices", "alerts", "iot"},
		"max_embed_token_minutes":      43200,
		"default_embed_token_minutes":  1440,
		"supported_direct_protocols":   []string{"modbus_tcp"},
		"supported_gateway_protocols":  []string{"rest", "mqtt", "opcua", "bacnet"},
		"gateway_ingest_endpoint":      "/api/v1/iot/ingest",
		"embed_snapshot_endpoint":      "/api/v1/integrations/embed-snapshot",
		"network_snapshot_endpoint":    "/api/v1/integrations/network-snapshot",
		"frame_ancestors_config_key":   "integration_frame_ancestors",
		"requires_restart":             false,
		"security_header_applies_live": true,
	}})
}

func (h *Handler) UpdateIntegrationSettings(c *gin.Context) {
	var input integrationSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	frameAncestors := normalizeIntegrationFrameAncestors(input.FrameAncestors)
	if len(frameAncestors) == 0 {
		frameAncestors = []string{"'self'"}
	}
	value, _ := json.Marshal(frameAncestors)
	if _, err := h.db.Exec(`
		INSERT INTO system_config (config_key, config_value, description, updated_at)
		VALUES ('integration_frame_ancestors', ?, 'CSP frame-ancestors allowlist for iframe integrations', CURRENT_TIMESTAMP)
		ON CONFLICT(config_key) DO UPDATE SET
			config_value = excluded.config_value,
			description = excluded.description,
			updated_at = CURRENT_TIMESTAMP
	`, string(value)); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{
		"frame_ancestors":           frameAncestors,
		"effective_frame_ancestors": strings.Join(frameAncestors, " "),
	}})
}

func (h *Handler) RevokeEmbedToken(c *gin.Context) {
	if err := h.ensureEmbedTokenTable(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	res, err := h.db.Exec(`
		UPDATE integration_embed_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_id = ? AND revoked_at IS NULL
	`, strings.TrimSpace(c.Param("tokenId")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "embed token not found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"revoked": true}})
}

func (h *Handler) GetIntegrationEmbedSnapshot(c *gin.Context) {
	view := strings.ToLower(strings.TrimSpace(c.DefaultQuery("view", "dashboard")))
	if view == "" {
		view = "dashboard"
	}
	if !h.embedTokenActive(c) {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "embed token is revoked or expired"})
		return
	}
	if !h.embedViewAllowed(c, view) {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "embed view is not allowed"})
		return
	}

	payload := gin.H{
		"schema_version": "nms.integration.embed.v1",
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"view":           view,
		"system": gin.H{
			"name":    h.config.System.Name,
			"version": h.license.DisplayVersion(),
		},
	}

	switch view {
	case "dashboard":
		dashboard, err := h.dashboard.GetDashboard()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		dashboard.Version = h.license.DisplayVersion()
		payload["dashboard"] = dashboard
	case "topology":
		topology, err := h.topology.LoadGraph()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		payload["topology"] = topology
	case "devices":
		page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
		limit := parsePositiveInt(c.DefaultQuery("limit", "100"), 100)
		if limit > 500 {
			limit = 500
		}
		devices, total, resolvedLimit, err := h.devices.ListDevices(devicesmodule.ListQuery{
			Page:       page,
			Limit:      limit,
			Search:     c.Query("search"),
			DeviceType: c.Query("type"),
			Status:     c.Query("status"),
			MaxDevices: h.getMaxDeviceLimit(),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		payload["devices"] = integrationDevicePage{Items: sanitizeIntegrationDevices(devices), Total: total, Page: page, Limit: resolvedLimit}
	case "alerts":
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		events, err := h.listEmbedEvents(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		payload["events"] = events
	case "iot":
		if !h.iot.LicenseEnabled() {
			payload["iot"] = gin.H{
				"status":  gin.H{"enabled": false},
				"devices": []interface{}{},
			}
			break
		}
		status, err := h.iot.Status()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		devices, err := h.iot.ListDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		payload["iot"] = gin.H{"status": status, "devices": devices}
	default:
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "unsupported embed view"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: payload})
}

func (h *Handler) listEmbedEvents(limit int) ([]EventEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := h.db.Query(`
		SELECT id, device_id, event_type, severity, message, created_at
		FROM events
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]EventEntry, 0)
	for rows.Next() {
		var e EventEntry
		var deviceID sql.NullInt64
		if err := rows.Scan(&e.ID, &deviceID, &e.EventType, &e.Severity, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		if deviceID.Valid {
			id := int(deviceID.Int64)
			e.DeviceID = &id
		}
		e.CreatedAt = sqliteTimeToRFC3339(e.CreatedAt)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (h *Handler) embedViewAllowed(c *gin.Context, view string) bool {
	embed, _ := c.Get("embed")
	isEmbed, _ := embed.(bool)
	if !isEmbed {
		return true
	}
	raw, _ := c.Get("views")
	allowed := map[string]bool{}
	for _, item := range strings.Split(strings.TrimSpace(asString(raw)), ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			allowed[item] = true
		}
	}
	return allowed[view]
}

func (h *Handler) embedTokenActive(c *gin.Context) bool {
	embed, _ := c.Get("embed")
	isEmbed, _ := embed.(bool)
	if !isEmbed {
		return true
	}
	raw, _ := c.Get("jti")
	tokenID := strings.TrimSpace(asString(raw))
	if tokenID == "" {
		return false
	}
	if err := h.ensureEmbedTokenTable(); err != nil {
		return false
	}
	var revokedAt string
	var expired int
	err := h.db.QueryRow(`
		SELECT COALESCE(revoked_at, ''), CASE WHEN expires_at <= CURRENT_TIMESTAMP THEN 1 ELSE 0 END
		FROM integration_embed_tokens
		WHERE token_id = ?
	`, tokenID).Scan(&revokedAt, &expired)
	if err != nil {
		return false
	}
	if strings.TrimSpace(revokedAt) != "" || expired == 1 {
		return false
	}
	_, _ = h.db.Exec(`UPDATE integration_embed_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE token_id = ?`, tokenID)
	return true
}

func (h *Handler) ensureEmbedTokenTable() error {
	_, err := h.db.Exec(`
		CREATE TABLE IF NOT EXISTS integration_embed_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_id TEXT NOT NULL UNIQUE,
			name TEXT DEFAULT '',
			views_json TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			revoked_at DATETIME,
			last_used_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (h *Handler) saveEmbedTokenRecord(tokenID, name string, views []string, expiresAt time.Time) error {
	if err := h.ensureEmbedTokenTable(); err != nil {
		return err
	}
	viewsJSON, _ := json.Marshal(views)
	_, err := h.db.Exec(`
		INSERT INTO integration_embed_tokens (token_id, name, views_json, expires_at)
		VALUES (?, ?, ?, ?)
	`, tokenID, strings.TrimSpace(name), string(viewsJSON), expiresAt.UTC().Format("2006-01-02 15:04:05"))
	return err
}

func (h *Handler) listEmbedTokenRecords() ([]embedTokenRecord, error) {
	if err := h.ensureEmbedTokenTable(); err != nil {
		return nil, err
	}
	rows, err := h.db.Query(`
		SELECT id, token_id, COALESCE(name, ''), views_json, COALESCE(expires_at, ''),
		       COALESCE(revoked_at, ''), COALESCE(last_used_at, ''), COALESCE(created_at, '')
		FROM integration_embed_tokens
		ORDER BY id DESC
		LIMIT 200
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]embedTokenRecord, 0)
	for rows.Next() {
		var record embedTokenRecord
		var viewsJSON string
		if err := rows.Scan(&record.ID, &record.TokenID, &record.Name, &viewsJSON, &record.ExpiresAt, &record.RevokedAt, &record.LastUsedAt, &record.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(viewsJSON), &record.Views)
		records = append(records, record)
	}
	return records, rows.Err()
}

func randomTokenID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func normalizeEmbedViews(input []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	for _, view := range input {
		view = strings.ToLower(strings.TrimSpace(view))
		switch view {
		case "dashboard", "topology", "devices", "alerts", "iot":
			if !seen[view] {
				seen[view] = true
				result = append(result, view)
			}
		}
	}
	return result
}

func (h *Handler) integrationFrameAncestors() []string {
	var raw string
	if err := h.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'integration_frame_ancestors'`).Scan(&raw); err == nil {
		var items []string
		if json.Unmarshal([]byte(raw), &items) == nil {
			if normalized := normalizeIntegrationFrameAncestors(items); len(normalized) > 0 {
				return normalized
			}
		}
	}
	if h.config != nil {
		if normalized := normalizeIntegrationFrameAncestors(h.config.Security.FrameAncestors); len(normalized) > 0 {
			return normalized
		}
	}
	return []string{"'self'"}
}

func normalizeIntegrationFrameAncestors(input []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if !validIntegrationFrameAncestor(item) || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	if len(result) > 1 && seen["'none'"] {
		return []string{"'none'"}
	}
	return result
}

func validIntegrationFrameAncestor(item string) bool {
	switch item {
	case "'self'", "'none'", "http:", "https:":
		return true
	}
	if strings.HasPrefix(item, "https://") || strings.HasPrefix(item, "http://") {
		return !strings.ContainsAny(strings.TrimPrefix(strings.TrimPrefix(item, "https://"), "http://"), " \t\r\n")
	}
	return false
}

func asString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return ""
	}
}
