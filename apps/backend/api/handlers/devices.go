// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	devicesmodule "management-server/modules/devices"
	"management-server/services/alert"
	"management-server/services/snmp"

	"github.com/gin-gonic/gin"
)

type Device = devicesmodule.Device
type DeviceInput = devicesmodule.DeviceInput

func (h *Handler) GetDevices(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

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

	c.JSON(http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    devices,
		Total:   total,
		Page:    page,
		Limit:   resolvedLimit,
	})
}

func (h *Handler) GetDevice(c *gin.Context) {
	id := c.Param("id")

	device, err := h.devices.GetDevice(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}

func (h *Handler) GetDeviceInterfaces(c *gin.Context) {
	id := c.Param("id")

	interfaces, err := h.devices.ListInterfaces(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: interfaces})
}

func (h *Handler) GetDeviceTraffic(c *gin.Context) {
	id := c.Param("id")
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	traffic, err := h.devices.ListTrafficSamples(id, hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: traffic})
}

func (h *Handler) CreateDevice(c *gin.Context) {
	var input DeviceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if input.DeviceType == "" {
		input.DeviceType = "unknown"
	}
	// The form sends monitor_type explicitly. Keep Ping-only devices at
	// snmp_version=0; otherwise the UI and collector will treat them as SNMP.
	if strings.EqualFold(input.MonitorType, "ping") {
		input.SNMPCommunity = ""
		input.SNMPVersion = 0
	} else {
		if input.SNMPCommunity == "" {
			input.SNMPCommunity = h.config.SNMP.DefaultCommunity
		}
		if input.SNMPVersion == 0 {
			input.SNMPVersion = 2
		}
	}

	maxDevices := h.getMaxDeviceLimit()
	currentCount, err := h.devices.CountDevices()
	if err != nil {
		h.WriteSystemLog("error", "device", "create_device_failed", "device creation failed: count query error", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	if currentCount >= maxDevices {
		h.WriteSystemLog("warning", "device", "create_device_blocked", "device creation blocked by license limit", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"current":     currentCount,
			"limit":       maxDevices,
		})
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "device limit reached by current license"})
		return
	}

	id, err := h.devices.CreateDevice(input)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			h.WriteSystemLog("warning", "device", "create_device_failed", "device creation failed: duplicate ip", map[string]interface{}{
				"device_name": input.Name,
				"ip_address":  input.IPAddress,
				"mac_address": input.MACAddress,
			})
			h.WriteAuditFromContext(c, "create_device", "device", "failed", auditDetails(nil,
				"device_name", input.Name,
				"ip_address", input.IPAddress,
				"mac_address", input.MACAddress,
				"device_type", input.DeviceType,
				"reason", "duplicate_ip",
			))
			c.JSON(http.StatusConflict, Response{Success: false, Error: "Device with this IP already exists"})
			return
		}
		h.WriteSystemLog("error", "device", "create_device_failed", "device creation failed: insert error", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"mac_address": input.MACAddress,
			"device_type": input.DeviceType,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "create_device", "device", "failed", auditDetails(nil,
			"device_name", input.Name,
			"ip_address", input.IPAddress,
			"mac_address", input.MACAddress,
			"device_type", input.DeviceType,
			"reason", "insert_failed",
			"error", err.Error(),
		))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "create_device", "device created", map[string]interface{}{
		"device_id":   id,
		"device_name": input.Name,
		"ip_address":  input.IPAddress,
		"mac_address": input.MACAddress,
		"device_type": input.DeviceType,
	})
	h.WriteDeviceLog(int(id), "notice", "inventory", "device created", "Device created", map[string]interface{}{
		"device_name": input.Name,
		"ip_address":  input.IPAddress,
		"device_type": input.DeviceType,
	})
	h.WriteAuditFromContext(c, "create_device", auditResource("device", id), "success", auditDetails(nil,
		"device_id", id,
		"device_name", input.Name,
		"ip_address", input.IPAddress,
		"mac_address", input.MACAddress,
		"device_type", input.DeviceType,
		"snmp_version", input.SNMPVersion,
	))

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	method := "Ping"
	if input.SNMPCommunity != "" {
		method = "SNMP"
	}
	msg := fmt.Sprintf("?輯撒'%s'  %s ?謘踐澈??'%s'", username, method, input.Name)

	h.db.Exec(`
		INSERT INTO events (device_id, event_type, severity, message)
		VALUES (?, 'device_added', 'info', ?)
	`, id, msg)

	go func() {
		alert.DispatchToEnabledChannels(h.db, h.config, msg)
	}()

	if input.SNMPCommunity != "" && h.snmpCollector != nil {
		h.WriteSystemLog("notice", "discovery", "device_initial_poll_queued", "initial device poll queued", map[string]interface{}{
			"device_id":   id,
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"device_type": input.DeviceType,
		})
		go func() {
			time.Sleep(1 * time.Second)
			log.Printf("[Device] Triggering initial SNMP poll for %s", input.IPAddress)
			cfg := snmp.PollDeviceConfig{
				ID:         int(id),
				IPAddress:  input.IPAddress,
				Community:  input.SNMPCommunity,
				Version:    input.SNMPVersion,
				DeviceType: input.DeviceType,
			}
			h.snmpCollector.PollDevice(cfg, nil, nil)
		}()
	} else if input.SNMPCommunity != "" {
		h.WriteSystemLog("warning", "discovery", "device_initial_poll_skipped", "initial device poll skipped", map[string]interface{}{
			"device_id":   id,
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"device_type": input.DeviceType,
			"reason":      "snmp_collector_unavailable",
		})
	}

	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

func (h *Handler) UpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	before, updatedFields, err := h.devices.UpdateDevice(id, input)
	if err == sql.ErrNoRows {
		h.WriteSystemLog("warning", "device", "update_device_failed", "device update failed: device not found", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		h.WriteAuditFromContext(c, "update_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err == devicesmodule.ErrNoFieldsToUpdate {
		h.WriteSystemLog("warning", "device", "update_device_rejected", "device update rejected: no fields to update", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No fields to update"})
		return
	}
	if err != nil {
		h.WriteSystemLog("error", "device", "update_device_failed", "device update failed", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "update_device", auditResource("device", before.ID), "failed", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
			"mac_address": before.MACAddress,
			"reason":      "update_failed",
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "update_device", "device updated", map[string]interface{}{
		"device_id":   before.ID,
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"fields":      updatedFields,
	})
	h.WriteDeviceLog(before.ID, "notice", "inventory", "device updated", "Device updated", map[string]interface{}{
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"new_values":  input,
	})
	h.WriteAuditFromContext(c, "update_device", auditResource("device", before.ID), "success", map[string]interface{}{
		"device_id":   before.ID,
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"mac_address": before.MACAddress,
		"old_values": map[string]interface{}{
			"name":           before.Name,
			"ip_address":     before.IPAddress,
			"mac_address":    before.MACAddress,
			"device_type":    before.DeviceType,
			"snmp_community": before.SNMPCommunity,
			"snmp_version":   before.SNMPVersion,
			"vendor":         before.Vendor,
			"model":          before.Model,
			"firmware":       before.Firmware,
			"pos_x":          before.PosX,
			"pos_y":          before.PosY,
		},
		"new_values": input,
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "Device updated"})
}

func (h *Handler) DeleteDevice(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Panic] DeleteDevice: %v", r)
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: fmt.Sprintf("Internal Server Error (Panic): %v", r)})
		}
	}()

	id := c.Param("id")
	result, err := h.devices.DeleteDevice(id)
	if err == sql.ErrNoRows {
		h.WriteSystemLog("warning", "device", "delete_device_failed", "device deletion failed: device not found", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err != nil {
		h.WriteSystemLog("error", "device", "delete_device_failed", "device deletion failed", map[string]interface{}{
			"device_id":   id,
			"device_name": result.Name,
			"ip_address":  result.IPAddress,
			"mac_address": result.MACAddress,
			"device_type": result.DeviceType,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id":   id,
			"device_name": result.Name,
			"ip_address":  result.IPAddress,
			"mac_address": result.MACAddress,
			"device_type": result.DeviceType,
			"reason":      "delete_failed",
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to delete device: " + err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "delete_device", "device deleted", map[string]interface{}{
		"device_id":   id,
		"device_name": result.Name,
		"ip_address":  result.IPAddress,
		"mac_address": result.MACAddress,
		"device_type": result.DeviceType,
	})
	h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "success", map[string]interface{}{
		"device_id":   id,
		"device_name": result.Name,
		"ip_address":  result.IPAddress,
		"mac_address": result.MACAddress,
		"device_type": result.DeviceType,
	})

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	go func(n, u string) {
		msg := fmt.Sprintf("?輯撒'%s' ??畸??桀?? '%s'", u, n)
		h.db.Exec(`
			INSERT INTO events (event_type, severity, message) 
			VALUES ('device_removed', 'info', ?)
		`, msg)

		alert.DispatchToEnabledChannels(h.db, h.config, msg)
	}(result.Name, username)

	c.JSON(http.StatusOK, Response{Success: true, Message: "Device deleted"})
}

// GetDevices returns a paginated list of all devices
func (h *Handler) getDevicesLegacy(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	search := c.Query("search")
	deviceType := c.Query("type")
	status := c.Query("status")

	offset := (page - 1) * limit

	// 撱綽蕭??嚙質岷
	query := `
		SELECT id, name, sys_name, ip_address, mac_address, device_type,
		       snmp_community, snmp_version, vendor, model, firmware,
		       is_online, last_seen, image_path, pos_x, pos_y,
		       created_at, updated_at
		FROM devices WHERE 1=1`
	countQuery := "SELECT COUNT(*) FROM devices WHERE 1=1"
	var args []interface{}

	if search != "" {
		query += " AND (name LIKE ? OR ip_address LIKE ?)"
		countQuery += " AND (name LIKE ? OR ip_address LIKE ?)"
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if deviceType != "" {
		query += " AND device_type = ?"
		countQuery += " AND device_type = ?"
		args = append(args, deviceType)
	}

	if status == "online" {
		query += " AND is_online = 1"
		countQuery += " AND is_online = 1"
	} else if status == "offline" {
		query += " AND is_online = 0"
		countQuery += " AND is_online = 0"
	}

	// ?嚙踝蕭?蝮賣
	var total int
	err := h.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	// ?嚙踝蕭??嚙踝蕭??嚙賢
	maxDevices := h.getMaxDeviceLimit()
	if total > maxDevices {
		total = maxDevices
	}

	// 瑼Ｘ?嚙踝蕭?蝭蕭?
	if offset >= maxDevices {
		c.JSON(http.StatusOK, PaginatedResponse{
			Success: true,
			Data:    []Device{},
			Total:   total,
			Page:    page,
			Limit:   limit,
		})
		return
	}

	// 隤踵?嚙質岷?嚙賢嚗Ⅱ靽蕭?憿舐內頞蕭??嚙踝蕭??嚙踝蕭??嚙質身??
	if offset+limit > maxDevices {
		limit = maxDevices - offset
	}

	// ?嚙踝蕭??嚙質岷
	query += " ORDER BY id ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		err := rows.Scan(&d.ID, &d.Name, &d.SysName, &d.IPAddress, &d.MACAddress, &d.DeviceType,
			&d.SNMPCommunity, &d.SNMPVersion, &d.Vendor, &d.Model, &d.Firmware,
			&d.IsOnline, &d.LastSeen, &d.ImagePath, &d.PosX, &d.PosY,
			&d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			continue
		}
		devices = append(devices, d)
	}

	c.JSON(http.StatusOK, PaginatedResponse{
		Success: true,
		Data:    devices,
		Total:   total,
		Page:    page,
		Limit:   limit,
	})
}

// GetDevice ?嚙踝蕭??嚙踝蕭?閮哨蕭?
func (h *Handler) getDeviceLegacy(c *gin.Context) {
	id := c.Param("id")

	var d Device
	err := h.db.QueryRow(`
		SELECT id, name, sys_name, ip_address, mac_address, device_type,
		       snmp_community, snmp_version, vendor, model, firmware,
		       is_online, last_seen, image_path, pos_x, pos_y,
		       created_at, updated_at
		FROM devices WHERE id = ?
	`, id).Scan(&d.ID, &d.Name, &d.SysName, &d.IPAddress, &d.MACAddress, &d.DeviceType,
		&d.SNMPCommunity, &d.SNMPVersion, &d.Vendor, &d.Model, &d.Firmware,
		&d.IsOnline, &d.LastSeen, &d.ImagePath, &d.PosX, &d.PosY,
		&d.CreatedAt, &d.UpdatedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: d})
}

// ... (CreateDevice, UpdateDevice etc skipped) ...

// GetDeviceInterfaces ?嚙踝蕭?閮哨蕭?隞鞈蕭?
func (h *Handler) getDeviceInterfacesLegacy(c *gin.Context) {
	id := c.Param("id")

	rows, err := h.db.Query(`
		SELECT id, device_id, if_index, if_name, if_desc, if_speed, if_mac, if_status, if_admin_status,
		       in_octets, out_octets, bandwidth_in, bandwidth_out, updated_at 
		FROM device_interfaces WHERE device_id = ? ORDER BY if_index
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var interfaces []map[string]interface{}
	for rows.Next() {
		var id, deviceID int
		var ifIndex sql.NullInt64
		var ifName, ifDesc, ifMac sql.NullString
		var ifSpeed, inOctets, outOctets, bandwidthIn, bandwidthOut sql.NullInt64
		var ifStatus, ifAdminStatus sql.NullInt64
		var updatedAt string

		if err := rows.Scan(&id, &deviceID, &ifIndex, &ifName, &ifDesc, &ifSpeed, &ifMac, &ifStatus, &ifAdminStatus,
			&inOctets, &outOctets, &bandwidthIn, &bandwidthOut, &updatedAt); err == nil {

			// ?嚙?嚙賭誨蝣潘蕭
			statusText := "unknown"
			if ifStatus.Valid {
				switch ifStatus.Int64 {
				case 1:
					statusText = "up"
				case 2:
					statusText = "down"
				case 3:
					statusText = "testing"
				}
			}

			interfaces = append(interfaces, map[string]interface{}{
				"id":              id,
				"device_id":       deviceID,
				"if_index":        ifIndex.Int64,
				"if_name":         ifName.String,
				"if_desc":         ifDesc.String,
				"if_speed":        ifSpeed.Int64,
				"if_mac":          ifMac.String,
				"if_status":       statusText,
				"if_admin_status": ifAdminStatus.Int64,
				"in_octets":       inOctets.Int64,
				"out_octets":      outOctets.Int64,
				"bandwidth_in":    bandwidthIn.Int64,
				"bandwidth_out":   bandwidthOut.Int64,
				"updated_at":      updatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: interfaces})
}

// CreateDevice ?嚙踝蕭?閮哨蕭?
func (h *Handler) createDeviceLegacy(c *gin.Context) {
	var input DeviceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	// ?嚙質身??
	if input.DeviceType == "" {
		input.DeviceType = "unknown"
	}
	if input.SNMPCommunity == "" {
		input.SNMPCommunity = h.config.SNMP.DefaultCommunity
	}
	if input.SNMPVersion == 0 {
		input.SNMPVersion = 2
	}

	// 瑼Ｘ閮哨蕭??嚙踝蕭??嚙賢
	maxDevices := h.getMaxDeviceLimit()
	var currentCount int
	if err := h.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&currentCount); err != nil {
		h.WriteSystemLog("error", "device", "create_device_failed", "device creation failed: count query error", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	if currentCount >= maxDevices {
		h.WriteSystemLog("warning", "device", "create_device_blocked", "device creation blocked by license limit", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"current":     currentCount,
			"limit":       maxDevices,
		})
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "device limit reached by current license"})
		return
	}

	result, err := h.db.Exec(`
		INSERT INTO devices (name, ip_address, mac_address, device_type, snmp_community, snmp_version) 
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.Name, input.IPAddress, input.MACAddress, input.DeviceType, input.SNMPCommunity, input.SNMPVersion)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			h.WriteSystemLog("warning", "device", "create_device_failed", "device creation failed: duplicate ip", map[string]interface{}{
				"device_name": input.Name,
				"ip_address":  input.IPAddress,
				"mac_address": input.MACAddress,
			})
			h.WriteAuditFromContext(c, "create_device", "device", "failed", auditDetails(nil,
				"device_name", input.Name,
				"ip_address", input.IPAddress,
				"mac_address", input.MACAddress,
				"device_type", input.DeviceType,
				"reason", "duplicate_ip",
			))
			c.JSON(http.StatusConflict, Response{Success: false, Error: "Device with this IP already exists"})
			return
		}
		h.WriteSystemLog("error", "device", "create_device_failed", "device creation failed: insert error", map[string]interface{}{
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"mac_address": input.MACAddress,
			"device_type": input.DeviceType,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "create_device", "device", "failed", auditDetails(nil,
			"device_name", input.Name,
			"ip_address", input.IPAddress,
			"mac_address", input.MACAddress,
			"device_type", input.DeviceType,
			"reason", "insert_failed",
			"error", err.Error(),
		))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	h.WriteSystemLog("notice", "device", "create_device", "device created", map[string]interface{}{
		"device_id":   id,
		"device_name": input.Name,
		"ip_address":  input.IPAddress,
		"mac_address": input.MACAddress,
		"device_type": input.DeviceType,
	})
	h.WriteDeviceLog(int(id), "notice", "inventory", "device created", "Device created", map[string]interface{}{
		"device_name": input.Name,
		"ip_address":  input.IPAddress,
		"device_type": input.DeviceType,
	})
	h.WriteAuditFromContext(c, "create_device", auditResource("device", id), "success", auditDetails(nil,
		"device_id", id,
		"device_name", input.Name,
		"ip_address", input.IPAddress,
		"mac_address", input.MACAddress,
		"device_type", input.DeviceType,
		"snmp_version", input.SNMPVersion,
	))

	// 閮蕭?鈭辣
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	method := "Ping"
	if input.SNMPCommunity != "" {
		method = "SNMP"
	}
	msg := fmt.Sprintf("雿輻??'%s' ?? %s ?啣???啗身??'%s'", username, method, input.Name)

	h.db.Exec(`
		INSERT INTO events (device_id, event_type, severity, message) 
		VALUES (?, 'device_added', 'info', ?)
	`, id, msg)

	// Dispatch Alert
	go func() {
		alert.DispatchToEnabledChannels(h.db, h.config, msg)
	}()

	// If SNMP device, trigger initial poll immediately (in background)
	if input.SNMPCommunity != "" && h.snmpCollector != nil {
		h.WriteSystemLog("notice", "discovery", "device_initial_poll_queued", "initial device poll queued", map[string]interface{}{
			"device_id":   id,
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"device_type": input.DeviceType,
		})
		go func() {
			time.Sleep(1 * time.Second)
			log.Printf("[Device] Triggering initial SNMP poll for %s", input.IPAddress)
			cfg := snmp.PollDeviceConfig{
				ID:         int(id),
				IPAddress:  input.IPAddress,
				Community:  input.SNMPCommunity,
				Version:    input.SNMPVersion,
				DeviceType: input.DeviceType,
			}
			h.snmpCollector.PollDevice(cfg, nil, nil)
		}()
	} else if input.SNMPCommunity != "" {
		h.WriteSystemLog("warning", "discovery", "device_initial_poll_skipped", "initial device poll skipped", map[string]interface{}{
			"device_id":   id,
			"device_name": input.Name,
			"ip_address":  input.IPAddress,
			"device_type": input.DeviceType,
			"reason":      "snmp_collector_unavailable",
		})
	}

	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

// UpdateDevice ?嚙賣閮哨蕭?
func (h *Handler) updateDeviceLegacy(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	var before Device
	err := h.db.QueryRow(`
		SELECT id, name, sys_name, ip_address, mac_address, device_type,
		       snmp_community, snmp_version, vendor, model, firmware,
		       is_online, last_seen, image_path, pos_x, pos_y,
		       created_at, updated_at
		FROM devices WHERE id = ?
	`, id).Scan(&before.ID, &before.Name, &before.SysName, &before.IPAddress, &before.MACAddress, &before.DeviceType,
		&before.SNMPCommunity, &before.SNMPVersion, &before.Vendor, &before.Model, &before.Firmware,
		&before.IsOnline, &before.LastSeen, &before.ImagePath, &before.PosX, &before.PosY,
		&before.CreatedAt, &before.UpdatedAt)
	if err == sql.ErrNoRows {
		h.WriteSystemLog("warning", "device", "update_device_failed", "device update failed: device not found", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		h.WriteAuditFromContext(c, "update_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err != nil {
		h.WriteSystemLog("error", "device", "update_device_failed", "device update failed: query error", map[string]interface{}{
			"device_id": id,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	// 撱綽蕭??嚙賣隤
	var updates []string
	var args []interface{}

	allowedFields := []string{"name", "ip_address", "mac_address", "device_type", "snmp_community", "snmp_version", "vendor", "model", "firmware", "pos_x", "pos_y"}
	for _, field := range allowedFields {
		if val, ok := input[field]; ok {
			updates = append(updates, field+" = ?")
			args = append(args, val)
		}
	}

	if len(updates) == 0 {
		h.WriteSystemLog("warning", "device", "update_device_rejected", "device update rejected: no fields to update", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now().Format("2006-01-02 15:04:05"))
	args = append(args, id)

	query := "UPDATE devices SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err = h.db.Exec(query, args...)
	if err != nil {
		h.WriteSystemLog("error", "device", "update_device_failed", "device update failed", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "update_device", auditResource("device", before.ID), "failed", map[string]interface{}{
			"device_id":   before.ID,
			"device_name": before.Name,
			"ip_address":  before.IPAddress,
			"mac_address": before.MACAddress,
			"reason":      "update_failed",
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "update_device", "device updated", map[string]interface{}{
		"device_id":   before.ID,
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"fields":      updates[:len(updates)-1],
	})
	h.WriteDeviceLog(before.ID, "notice", "inventory", "device updated", "Device updated", map[string]interface{}{
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"new_values":  input,
	})
	h.WriteAuditFromContext(c, "update_device", auditResource("device", before.ID), "success", map[string]interface{}{
		"device_id":   before.ID,
		"device_name": before.Name,
		"ip_address":  before.IPAddress,
		"mac_address": before.MACAddress,
		"old_values": map[string]interface{}{
			"name":           before.Name,
			"ip_address":     before.IPAddress,
			"mac_address":    before.MACAddress,
			"device_type":    before.DeviceType,
			"snmp_community": before.SNMPCommunity,
			"snmp_version":   before.SNMPVersion,
			"vendor":         before.Vendor,
			"model":          before.Model,
			"firmware":       before.Firmware,
			"pos_x":          before.PosX,
			"pos_y":          before.PosY,
		},
		"new_values": input,
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "Device updated"})
}

// DeleteDevice ?嚙賡閮哨蕭?
// DeleteDevice ?嚙賡閮哨蕭?
func (h *Handler) deleteDeviceLegacy(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Panic] DeleteDevice: %v", r)
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: fmt.Sprintf("Internal Server Error (Panic): %v", r)})
		}
	}()

	id := c.Param("id")

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		h.WriteSystemLog("error", "device", "delete_device_failed", "device deletion failed: transaction start error", map[string]interface{}{
			"device_id": id,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to start transaction: " + err.Error()})
		return
	}
	defer tx.Rollback()

	// ?嚙踝蕭?閮哨蕭??嚙賜迂
	var name, ipAddress, deviceType string
	var macAddress sql.NullString
	err = tx.QueryRow("SELECT name, ip_address, mac_address, device_type FROM devices WHERE id = ?", id).
		Scan(&name, &ipAddress, &macAddress, &deviceType)
	if err == sql.ErrNoRows {
		h.WriteSystemLog("warning", "device", "delete_device_failed", "device deletion failed: device not found", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	}
	if err != nil {
		h.WriteSystemLog("error", "device", "delete_device_failed", "device deletion failed: query error", map[string]interface{}{
			"device_id": id,
			"error":     err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to query device: " + err.Error()})
		return
	}

	// Manually delete dependencies to ensure clean deletion (and avoid potential lock/FK issues)
	// We ignore errors here as the tables might not have checks that prevent deletion,
	// or FK cascade might handle it, but we want to be proactive.
	_, _ = tx.Exec("DELETE FROM device_interfaces WHERE device_id = ?", id)
	_, _ = tx.Exec("DELETE FROM device_metrics WHERE device_id = ?", id)
	_, _ = tx.Exec("DELETE FROM topology_links WHERE source_device_id = ? OR target_device_id = ?", id, id)

	_, err = tx.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		h.WriteSystemLog("error", "device", "delete_device_failed", "device deletion failed", map[string]interface{}{
			"device_id":   id,
			"device_name": name,
			"ip_address":  ipAddress,
			"mac_address": macAddress.String,
			"device_type": deviceType,
			"error":       err.Error(),
		})
		h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "failed", map[string]interface{}{
			"device_id":   id,
			"device_name": name,
			"ip_address":  ipAddress,
			"mac_address": macAddress.String,
			"device_type": deviceType,
			"reason":      "delete_failed",
			"error":       err.Error(),
		})
		// If explicit delete fails, return error
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to delete device: " + err.Error()})
		return
	}

	// Commit
	if err := tx.Commit(); err != nil {
		h.WriteSystemLog("error", "device", "delete_device_failed", "device deletion failed: commit error", map[string]interface{}{
			"device_id":   id,
			"device_name": name,
			"ip_address":  ipAddress,
			"mac_address": macAddress.String,
			"device_type": deviceType,
			"error":       err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to commit transaction: " + err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "delete_device", "device deleted", map[string]interface{}{
		"device_id":   id,
		"device_name": name,
		"ip_address":  ipAddress,
		"mac_address": macAddress.String,
		"device_type": deviceType,
	})
	h.WriteAuditFromContext(c, "delete_device", auditResource("device", id), "success", map[string]interface{}{
		"device_id":   id,
		"device_name": name,
		"ip_address":  ipAddress,
		"mac_address": macAddress.String,
		"device_type": deviceType,
	})

	// 閮蕭?鈭辣 (Best effort, using main db connection outside tx)
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	go func(n, u string) {
		msg := fmt.Sprintf("雿輻??'%s' ?芷閮剖? '%s'", u, n)
		h.db.Exec(`
			INSERT INTO events (event_type, severity, message) 
			VALUES ('device_removed', 'info', ?)
		`, msg)

		alert.DispatchToEnabledChannels(h.db, h.config, msg)
	}(name, username)

	c.JSON(http.StatusOK, Response{Success: true, Message: "Device deleted"})
}

// UploadDeviceImage 銝閮哨蕭??嚙踝蕭?
func (h *Handler) UploadDeviceImage(c *gin.Context) {
	id := c.Param("id")

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No image file provided"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Invalid file type. Allowed: jpg, jpeg, png, gif, webp"})
		return
	}

	dataPath := "../data"
	if _, err := os.Stat("./data"); err == nil {
		dataPath = "./data"
	}

	imagePath, err := h.devices.SaveUploadedImage(id, file, header.Filename, filepath.Join(dataPath, "uploads"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]string{"image_path": imagePath}})
}

// GetDeviceMetrics ?嚙踝蕭?閮哨蕭??嚙質?嚙踝蕭?
func (h *Handler) GetDeviceMetrics(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	metrics, err := h.devices.ListMetrics(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: metrics})
}

// PollDevice ?嚙踝蕭?頛芾岷閮哨蕭?
func (h *Handler) PollDevice(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.devices.GetDevice(id); err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "Device not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "device", "poll_device", "device poll queued", map[string]interface{}{
		"device_id": id,
	})
	h.WriteAuditFromContext(c, "poll_device", auditResource("device", id), "success", map[string]interface{}{
		"device_id": id,
		"message":   "poll request queued",
	})
	if parsedID, err := strconv.Atoi(id); err == nil {
		h.WriteDeviceLog(parsedID, "notice", "polling", "device poll queued", "Device poll queued", nil)
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "Poll request queued"})
}

// BulkDeleteDevices ?嚙踝蕭??嚙賡閮哨蕭?
func (h *Handler) BulkDeleteDevices(c *gin.Context) {
	var input struct {
		IDs []int `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if len(input.IDs) == 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No device IDs provided"})
		return
	}

	rowsAffected, err := h.devices.BulkDelete(input.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "bulk_delete_devices", "device", "success", map[string]interface{}{
		"target_ids":     input.IDs,
		"affected_count": rowsAffected,
	})

	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	msg := fmt.Sprintf("使用者 '%s' 批次刪除了 %d 台設備", username, rowsAffected)

	h.db.Exec(`
		INSERT INTO events (event_type, severity, message) 
		VALUES ('bulk_device_removed', 'info', ?)
	`, msg)

	go alert.DispatchToEnabledChannels(h.db, h.config, msg)

	c.JSON(http.StatusOK, Response{Success: true, Message: fmt.Sprintf("Successfully deleted %d devices", rowsAffected)})
}

// BulkUpdateDevices ?嚙踝蕭??嚙賣閮哨蕭? (?嚙賣 ?嚙踝蕭?/?嚙踝蕭?/?嚙踝蕭?)
func (h *Handler) BulkUpdateDevices(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "File too large or invalid format"})
		return
	}

	idsStr := c.Request.FormValue("ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No device IDs provided"})
		return
	}
	idList := strings.Split(idsStr, ",")

	vendor := c.Request.FormValue("vendor")
	model := c.Request.FormValue("model")

	var imagePath string
	file, header, err := c.Request.FormFile("image")
	if err == nil {
		defer file.Close()
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Only image files allowed"})
			return
		}

		newFilename := fmt.Sprintf("%d_bulk%s", time.Now().UnixNano(), ext)
		saveDir := filepath.Join("data", "uploads", "devices")
		if err := os.MkdirAll(saveDir, 0o755); err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to create upload directory"})
			return
		}
		dst := filepath.Join(saveDir, newFilename)

		out, err := os.Create(dst)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to save file"})
			return
		}
		defer out.Close()
		if _, err = io.Copy(out, file); err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to write file"})
			return
		}
		imagePath = "/uploads/devices/" + newFilename
	}

	ids := make([]int, 0, len(idList))
	for _, idStr := range idList {
		if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil {
			ids = append(ids, id)
		}
	}

	totalUpdated, err := h.devices.BulkUpdate(devicesmodule.BulkUpdateInput{
		IDs:       ids,
		Vendor:    vendor,
		Model:     model,
		ImagePath: imagePath,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "bulk_update_devices", "device", "success", map[string]interface{}{
		"target_ids":     ids,
		"affected_count": totalUpdated,
		"vendor":         vendor,
		"model":          model,
		"image_path":     imagePath,
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: fmt.Sprintf("Updated %d devices", totalUpdated), Data: imagePath})
}
