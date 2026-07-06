// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	devicesmodule "management-server/modules/devices"

	"github.com/gin-gonic/gin"
)

const integrationSnapshotSchemaVersion = "nms.integration.network_snapshot.v1"

type integrationSnapshotResponse struct {
	SchemaVersion string                   `json:"schema_version"`
	GeneratedAt   string                   `json:"generated_at"`
	APIVersion    string                   `json:"api_version"`
	System        integrationSystemInfo    `json:"system"`
	Dashboard     interface{}              `json:"dashboard,omitempty"`
	Devices       *integrationDevicePage   `json:"devices,omitempty"`
	Topology      interface{}              `json:"topology,omitempty"`
	Links         integrationSnapshotLinks `json:"links"`
}

type integrationSystemInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type integrationDevicePage struct {
	Items []integrationDevice `json:"items"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}

type integrationDevice struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	SysName     *string `json:"sys_name,omitempty"`
	IPAddress   string  `json:"ip_address"`
	MACAddress  *string `json:"mac_address,omitempty"`
	DeviceType  string  `json:"device_type"`
	SNMPVersion int     `json:"snmp_version"`
	Vendor      *string `json:"vendor,omitempty"`
	Model       *string `json:"model,omitempty"`
	Firmware    *string `json:"firmware,omitempty"`
	IsOnline    bool    `json:"is_online"`
	LastSeen    *string `json:"last_seen,omitempty"`
	ImagePath   *string `json:"image_path,omitempty"`
	PosX        float64 `json:"pos_x"`
	PosY        float64 `json:"pos_y"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type integrationSnapshotLinks struct {
	Self      string `json:"self"`
	Dashboard string `json:"dashboard"`
	Devices   string `json:"devices"`
	Topology  string `json:"topology"`
}

// GetIntegrationNetworkSnapshot gives external clients one stable read endpoint
// for the common dashboard + inventory + topology bootstrap flow.
func (h *Handler) GetIntegrationNetworkSnapshot(c *gin.Context) {
	includes := parseIntegrationIncludes(c.DefaultQuery("include", "dashboard,devices,topology"))
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	limit := parsePositiveInt(c.DefaultQuery("limit", "500"), 500)
	if limit > 2000 {
		limit = 2000
	}

	payload := integrationSnapshotResponse{
		SchemaVersion: integrationSnapshotSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		APIVersion:    "v1",
		System: integrationSystemInfo{
			Name:    h.config.System.Name,
			Version: h.license.DisplayVersion(),
		},
		Links: integrationSnapshotLinks{
			Self:      "/api/v1/integrations/network-snapshot",
			Dashboard: "/api/v1/dashboard",
			Devices:   "/api/v1/devices",
			Topology:  "/api/v1/topology",
		},
	}

	if includes["dashboard"] {
		dashboard, err := h.dashboard.GetDashboard()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		dashboard.Version = h.license.DisplayVersion()
		payload.Dashboard = dashboard
	}

	if includes["devices"] {
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
		payload.Devices = &integrationDevicePage{
			Items: sanitizeIntegrationDevices(devices),
			Total: total,
			Page:  page,
			Limit: resolvedLimit,
		}
	}

	if includes["topology"] {
		topology, err := h.topology.LoadGraph()
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		payload.Topology = topology
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: payload})
}

func parseIntegrationIncludes(raw string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(item))
		switch key {
		case "dashboard", "devices", "topology":
			result[key] = true
		}
	}
	if len(result) == 0 {
		result["dashboard"] = true
		result["devices"] = true
		result["topology"] = true
	}
	return result
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func sanitizeIntegrationDevices(devices []devicesmodule.Device) []integrationDevice {
	items := make([]integrationDevice, 0, len(devices))
	for _, d := range devices {
		items = append(items, integrationDevice{
			ID:          d.ID,
			Name:        d.Name,
			SysName:     d.SysName,
			IPAddress:   d.IPAddress,
			MACAddress:  d.MACAddress,
			DeviceType:  d.DeviceType,
			SNMPVersion: d.SNMPVersion,
			Vendor:      d.Vendor,
			Model:       d.Model,
			Firmware:    d.Firmware,
			IsOnline:    d.IsOnline,
			LastSeen:    d.LastSeen,
			ImagePath:   d.ImagePath,
			PosX:        d.PosX,
			PosY:        d.PosY,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}
	return items
}
