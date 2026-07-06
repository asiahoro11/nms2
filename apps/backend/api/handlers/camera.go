// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	cameramodule "management-server/modules/camera"
)

// ============================================================
// Camera Models
// ============================================================

type Camera struct {
	ID                   int        `json:"id"`
	Name                 string     `json:"name"`
	Location             string     `json:"location"`
	IPAddress            string     `json:"ip_address"`
	Port                 int        `json:"port"`
	Username             string     `json:"username"`
	RTSPUrl              string     `json:"rtsp_url"`
	PreviewRTSPUrl       string     `json:"preview_rtsp_url"`
	RecordingRTSPUrl     string     `json:"recording_rtsp_url"`
	RTSPTransport        string     `json:"rtsp_transport"`
	RTSPUDPMinPort       int        `json:"rtsp_udp_min_port"`
	RTSPUDPMaxPort       int        `json:"rtsp_udp_max_port"`
	ONVIFUrl             string     `json:"onvif_url"`
	Manufacturer         string     `json:"manufacturer"`
	Model                string     `json:"model"`
	Firmware             string     `json:"firmware"`
	SupportsPTZ          bool       `json:"supports_ptz"`
	IsEnabled            bool       `json:"is_enabled"`
	Status               string     `json:"status"`
	StreamType           string     `json:"stream_type"`
	MonitorDisplay       int        `json:"monitor_display"`
	MonitorOrder         int        `json:"monitor_order"`
	RecordingSource      string     `json:"recording_source"` // "rtsp" | "onvif"
	RecordingBitrateKbps int        `json:"recording_bitrate_kbps"`
	LastSeen             *time.Time `json:"last_seen"`
	CreatedAt            time.Time  `json:"created_at"`
}

type CameraRequest struct {
	Name                 string `json:"name" binding:"required"`
	Location             string `json:"location"`
	IPAddress            string `json:"ip_address" binding:"required"`
	Port                 int    `json:"port"`
	RTSPUrl              string `json:"rtsp_url"`
	PreviewRTSPUrl       string `json:"preview_rtsp_url"`
	RecordingRTSPUrl     string `json:"recording_rtsp_url"`
	RTSPTransport        string `json:"rtsp_transport"`
	RTSPUDPMinPort       int    `json:"rtsp_udp_min_port"`
	RTSPUDPMaxPort       int    `json:"rtsp_udp_max_port"`
	ONVIFUrl             string `json:"onvif_url"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	Manufacturer         string `json:"manufacturer"`
	Model                string `json:"model"`
	SupportsPTZ          bool   `json:"supports_ptz"`
	StreamType           string `json:"stream_type"`
	MonitorDisplay       int    `json:"monitor_display"`
	MonitorOrder         int    `json:"monitor_order"`
	RecordingSource      string `json:"recording_source"` // "rtsp" | "onvif"
	RecordingBitrateKbps int    `json:"recording_bitrate_kbps"`
}

func normalizeCameraRequest(req *CameraRequest) {
	req.RTSPUrl = strings.TrimSpace(req.RTSPUrl)
	req.PreviewRTSPUrl = strings.TrimSpace(req.PreviewRTSPUrl)
	req.RecordingRTSPUrl = strings.TrimSpace(req.RecordingRTSPUrl)
	if req.PreviewRTSPUrl == "" {
		req.PreviewRTSPUrl = req.RTSPUrl
	}
	if req.RTSPUrl == "" {
		req.RTSPUrl = req.PreviewRTSPUrl
	}
	switch strings.ToLower(strings.TrimSpace(req.RTSPTransport)) {
	case "tcp", "udp":
		req.RTSPTransport = strings.ToLower(strings.TrimSpace(req.RTSPTransport))
	default:
		req.RTSPTransport = "auto"
	}
	if req.RTSPUDPMinPort < 0 {
		req.RTSPUDPMinPort = 0
	}
	if req.RTSPUDPMaxPort < 0 {
		req.RTSPUDPMaxPort = 0
	}
	if req.RTSPUDPMinPort > 65535 {
		req.RTSPUDPMinPort = 65535
	}
	if req.RTSPUDPMaxPort > 65535 {
		req.RTSPUDPMaxPort = 65535
	}
	if req.RTSPUDPMinPort > 0 && req.RTSPUDPMaxPort > 0 && req.RTSPUDPMinPort > req.RTSPUDPMaxPort {
		req.RTSPUDPMinPort, req.RTSPUDPMaxPort = req.RTSPUDPMaxPort, req.RTSPUDPMinPort
	}
}

// ============================================================
// License check helper
// ============================================================

// cameraLicenseEnabled returns true if the camera module is unlocked via license.
// The tab is always visible in the UI; this gate controls actual usage (add/view).
func (h *Handler) cameraLicenseEnabled() bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.camera.LicenseEnabled()
}

func (h *Handler) cameraMaxAllowed() int {
	return h.camera.MaxAllowed()
}

func (h *Handler) ensureCameraSchema() error {
	return h.camera.EnsureSchema()
}

// ============================================================
// API Handlers — Camera CRUD
// ============================================================

// GetCameraModuleStatus GET /api/v1/cameras/status
// Returns whether the camera module is licensed, and current/max counts.
// Always succeeds so the frontend can render the "locked" state without auth errors.
func (h *Handler) GetCameraModuleStatus(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	enabled := h.cameraLicenseEnabled()
	maxCams := h.cameraMaxAllowed()

	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM cameras").Scan(&count)

	// Report GPU availability so frontend can unlock 32ch grid option
	ffmpegBin := cameramodule.FindFFmpegBin()
	gpuAvailable := false
	if ffmpegBin != "" {
		gpuAvailable = cameramodule.HasHardwareDecode(ffmpegBin)
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"licensed":      enabled,
			"count":         count,
			"max":           maxCams,
			"locked_ui":     !enabled,
			"gpu_available": gpuAvailable,
		},
	})
}

// GetMonitorCameras GET /api/v1/cameras/monitor
// Returns cameras that are set to appear in the monitor split-view, ordered by monitor_order.
func (h *Handler) GetMonitorCameras(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	cams, err := h.camera.ListMonitorCameras()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: cams})
}

// SetMonitorDisplay PUT /api/v1/cameras/:id/monitor-display
// Sets monitor_display and monitor_order for a camera.
func (h *Handler) SetMonitorDisplay(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")
	var before Camera
	err := h.db.QueryRow(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras WHERE id = ?
	`, id).Scan(
		&before.ID, &before.Name, &before.Location, &before.IPAddress, &before.Port, &before.Username,
		&before.RTSPUrl, &before.ONVIFUrl, &before.Manufacturer, &before.Model, &before.Firmware,
		&before.SupportsPTZ, &before.IsEnabled, &before.Status, &before.StreamType,
		&before.MonitorDisplay, &before.MonitorOrder, &before.RecordingSource,
		&before.RecordingBitrateKbps, &before.LastSeen, &before.CreatedAt,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	var req struct {
		MonitorDisplay int `json:"monitor_display"`
		MonitorOrder   int `json:"monitor_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	updated, err := h.camera.SetMonitorDisplay(id, req.MonitorDisplay, req.MonitorOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

// GetCameras GET /api/v1/cameras
func (h *Handler) GetCameras(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	cameras, err := h.camera.ListCameras()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"cameras": cameras,
			"total":   len(cameras),
			"max":     h.cameraMaxAllowed(),
		},
	})
}

// CreateCamera POST /api/v1/cameras
func (h *Handler) CreateCamera(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	var req CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	normalizeCameraRequest(&req)

	count, err := h.camera.CountCameras()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if count >= h.cameraMaxAllowed() {
		c.JSON(http.StatusPaymentRequired, Response{
			Success: false,
			Error:   "camera_limit_reached",
			Message: "camera_limit_reached",
		})
		return
	}

	var encPwd *string
	if req.Password != "" && strings.TrimSpace(req.Username) != "" {
		encrypted, encErr := cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password)
		if encErr != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "password encryption failed"})
			return
		}
		encPwd = &encrypted
	}

	if req.Port == 0 {
		req.Port = 554
	}
	if req.StreamType == "" {
		req.StreamType = "mjpeg"
	}
	if req.RecordingSource == "" {
		req.RecordingSource = "rtsp"
	}

	id, err := h.camera.CreateCamera(cameramodule.CameraInput{
		Name:                 req.Name,
		Location:             req.Location,
		IPAddress:            req.IPAddress,
		Port:                 req.Port,
		RTSPUrl:              req.RTSPUrl,
		PreviewRTSPUrl:       req.PreviewRTSPUrl,
		RecordingRTSPUrl:     req.RecordingRTSPUrl,
		RTSPTransport:        req.RTSPTransport,
		RTSPUDPMinPort:       req.RTSPUDPMinPort,
		RTSPUDPMaxPort:       req.RTSPUDPMaxPort,
		ONVIFUrl:             req.ONVIFUrl,
		Username:             req.Username,
		PasswordEncrypted:    encPwd,
		Manufacturer:         req.Manufacturer,
		Model:                req.Model,
		SupportsPTZ:          req.SupportsPTZ,
		StreamType:           req.StreamType,
		MonitorDisplay:       req.MonitorDisplay,
		MonitorOrder:         req.MonitorOrder,
		RecordingSource:      req.RecordingSource,
		RecordingBitrateKbps: req.RecordingBitrateKbps,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to create camera: " + err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "create_camera", auditResource("camera", id), "success", map[string]interface{}{
		"camera_id":         id,
		"camera_name":       req.Name,
		"ip_address":        req.IPAddress,
		"location":          req.Location,
		"stream_type":       req.StreamType,
		"monitor_display":   req.MonitorDisplay,
		"monitor_order":     req.MonitorOrder,
		"recording_source":  req.RecordingSource,
		"recording_bitrate": req.RecordingBitrateKbps,
		"rtsp_transport":    req.RTSPTransport,
		"rtsp_udp_min_port": req.RTSPUDPMinPort,
		"rtsp_udp_max_port": req.RTSPUDPMaxPort,
	})
	c.JSON(http.StatusCreated, Response{Success: true, Message: "camera created", Data: map[string]int64{"id": id}})
}

// GetCamera GET /api/v1/cameras/:id
func (h *Handler) GetCamera(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	cameraID := c.Param("id")
	camera, err := h.camera.GetCamera(cameraID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: camera})
}

// UpdateCamera PUT /api/v1/cameras/:id
func (h *Handler) UpdateCamera(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	id := c.Param("id")
	var req CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	normalizeCameraRequest(&req)

	before, err := h.camera.GetCamera(id)
	if err == sql.ErrNoRows {
		h.WriteAuditFromContext(c, "update_camera", auditResource("camera", id), "failed", map[string]interface{}{
			"camera_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	var encPwd *string
	if req.Password != "" && strings.TrimSpace(req.Username) != "" {
		encrypted, encErr := cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password)
		if encErr != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "password encryption failed"})
			return
		}
		encPwd = &encrypted
	} else if strings.TrimSpace(req.Username) == "" {
		empty := ""
		encPwd = &empty
	}
	if req.RecordingSource == "" {
		req.RecordingSource = "rtsp"
	}

	updated, err := h.camera.UpdateCamera(id, cameramodule.CameraInput{
		Name:                 req.Name,
		Location:             req.Location,
		IPAddress:            req.IPAddress,
		Port:                 req.Port,
		RTSPUrl:              req.RTSPUrl,
		PreviewRTSPUrl:       req.PreviewRTSPUrl,
		RecordingRTSPUrl:     req.RecordingRTSPUrl,
		RTSPTransport:        req.RTSPTransport,
		RTSPUDPMinPort:       req.RTSPUDPMinPort,
		RTSPUDPMaxPort:       req.RTSPUDPMaxPort,
		ONVIFUrl:             req.ONVIFUrl,
		Username:             req.Username,
		PasswordEncrypted:    encPwd,
		Manufacturer:         req.Manufacturer,
		Model:                req.Model,
		SupportsPTZ:          req.SupportsPTZ,
		StreamType:           req.StreamType,
		MonitorDisplay:       req.MonitorDisplay,
		MonitorOrder:         req.MonitorOrder,
		RecordingSource:      req.RecordingSource,
		RecordingBitrateKbps: req.RecordingBitrateKbps,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !updated {
		h.WriteAuditFromContext(c, "update_camera", auditResource("camera", id), "failed", map[string]interface{}{
			"camera_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}

	h.WriteAuditFromContext(c, "update_camera", auditResource("camera", before.ID), "success", map[string]interface{}{
		"camera_id":   before.ID,
		"camera_name": before.Name,
		"ip_address":  before.IPAddress,
		"old_values": map[string]interface{}{
			"name":               before.Name,
			"location":           before.Location,
			"ip_address":         before.IPAddress,
			"port":               before.Port,
			"username":           before.Username,
			"rtsp_url":           cameramodule.RedactSensitiveText(before.RTSPUrl),
			"preview_rtsp_url":   cameramodule.RedactSensitiveText(before.PreviewRTSPUrl),
			"recording_rtsp_url": cameramodule.RedactSensitiveText(before.RecordingRTSPUrl),
			"onvif_url":          before.ONVIFUrl,
			"manufacturer":       before.Manufacturer,
			"model":              before.Model,
			"supports_ptz":       before.SupportsPTZ,
			"stream_type":        before.StreamType,
			"monitor_display":    before.MonitorDisplay,
			"monitor_order":      before.MonitorOrder,
			"recording_source":   before.RecordingSource,
			"recording_bitrate":  before.RecordingBitrateKbps,
			"rtsp_transport":     before.RTSPTransport,
			"rtsp_udp_min_port":  before.RTSPUDPMinPort,
			"rtsp_udp_max_port":  before.RTSPUDPMaxPort,
		},
		"new_values": map[string]interface{}{
			"name":               req.Name,
			"location":           req.Location,
			"ip_address":         req.IPAddress,
			"port":               req.Port,
			"username":           req.Username,
			"rtsp_url":           cameramodule.RedactSensitiveText(req.RTSPUrl),
			"preview_rtsp_url":   cameramodule.RedactSensitiveText(req.PreviewRTSPUrl),
			"recording_rtsp_url": cameramodule.RedactSensitiveText(req.RecordingRTSPUrl),
			"onvif_url":          req.ONVIFUrl,
			"manufacturer":       req.Manufacturer,
			"model":              req.Model,
			"supports_ptz":       req.SupportsPTZ,
			"stream_type":        req.StreamType,
			"monitor_display":    req.MonitorDisplay,
			"monitor_order":      req.MonitorOrder,
			"recording_source":   req.RecordingSource,
			"recording_bitrate":  req.RecordingBitrateKbps,
			"rtsp_transport":     req.RTSPTransport,
			"rtsp_udp_min_port":  req.RTSPUDPMinPort,
			"rtsp_udp_max_port":  req.RTSPUDPMaxPort,
			"password_updated":   req.Password != "",
			"password_cleared":   strings.TrimSpace(req.Username) == "",
		},
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "camera updated"})
}

// DeleteCamera DELETE /api/v1/cameras/:id
func (h *Handler) DeleteCamera(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	id := c.Param("id")
	before, err := h.camera.GetCamera(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	deleted, err := h.camera.DeleteCamera(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}

	h.WriteAuditFromContext(c, "delete_camera", auditResource("camera", before.ID), "success", map[string]interface{}{
		"camera_id":   before.ID,
		"camera_name": before.Name,
		"ip_address":  before.IPAddress,
		"location":    before.Location,
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "camera deleted"})
}

func (h *Handler) getCamerasLegacy(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	rows, err := h.db.Query(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras ORDER BY id ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	cameras := []Camera{}
	for rows.Next() {
		var cam Camera
		if err := rows.Scan(
			&cam.ID, &cam.Name, &cam.Location, &cam.IPAddress, &cam.Port, &cam.Username,
			&cam.RTSPUrl, &cam.ONVIFUrl, &cam.Manufacturer, &cam.Model, &cam.Firmware,
			&cam.SupportsPTZ, &cam.IsEnabled, &cam.Status, &cam.StreamType,
			&cam.MonitorDisplay, &cam.MonitorOrder, &cam.RecordingSource,
			&cam.RecordingBitrateKbps, &cam.LastSeen, &cam.CreatedAt,
		); err != nil {
			continue
		}
		cameras = append(cameras, cam)
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"cameras": cameras,
			"total":   len(cameras),
			"max":     h.cameraMaxAllowed(),
		},
	})
}

// CreateCamera POST /api/v1/cameras
func (h *Handler) createCameraLegacy(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	var req CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	var count int
	h.db.QueryRow("SELECT COUNT(*) FROM cameras").Scan(&count)
	if count >= h.cameraMaxAllowed() {
		c.JSON(http.StatusPaymentRequired, Response{
			Success: false,
			Error:   "camera_limit_reached",
			Message: "camera_limit_reached",
		})
		return
	}

	encPwd := ""
	if req.Password != "" {
		var err error
		encPwd, err = cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "?ïîº??­æ??"})
			return
		}
	}

	if req.Port == 0 {
		req.Port = 554
	}
	if req.StreamType == "" {
		req.StreamType = "mjpeg"
	}

	if req.RecordingSource == "" {
		req.RecordingSource = "rtsp"
	}
	result, err := h.db.Exec(`
		INSERT INTO cameras (name, location, ip_address, port, rtsp_url, onvif_url, username, password_encrypted,
		                     manufacturer, model, supports_ptz, is_enabled, status, stream_type,
		                     monitor_display, monitor_order, recording_source, recording_bitrate_kbps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'unknown', ?, ?, ?, ?, ?)
	`, req.Name, req.Location, req.IPAddress, req.Port, req.RTSPUrl, req.ONVIFUrl, req.Username, encPwd,
		req.Manufacturer, req.Model, req.SupportsPTZ, req.StreamType, req.MonitorDisplay, req.MonitorOrder,
		req.RecordingSource, req.RecordingBitrateKbps)

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "å»ºç??å½±æ©å¤±?? " + err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	h.WriteAuditFromContext(c, "create_camera", auditResource("camera", id), "success", map[string]interface{}{
		"camera_id":         id,
		"camera_name":       req.Name,
		"ip_address":        req.IPAddress,
		"location":          req.Location,
		"stream_type":       req.StreamType,
		"monitor_display":   req.MonitorDisplay,
		"monitor_order":     req.MonitorOrder,
		"recording_source":  req.RecordingSource,
		"recording_bitrate": req.RecordingBitrateKbps,
	})
	c.JSON(http.StatusCreated, Response{Success: true, Message: "?î³è£?î?æ­", Data: map[string]int64{"id": id}})
}

// GetCamera GET /api/v1/cameras/:id
func (h *Handler) getCameraLegacy(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")
	var cam Camera
	err := h.db.QueryRow(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras WHERE id = ?
	`, id).Scan(
		&cam.ID, &cam.Name, &cam.Location, &cam.IPAddress, &cam.Port, &cam.Username,
		&cam.RTSPUrl, &cam.ONVIFUrl, &cam.Manufacturer, &cam.Model, &cam.Firmware,
		&cam.SupportsPTZ, &cam.IsEnabled, &cam.Status, &cam.StreamType,
		&cam.MonitorDisplay, &cam.MonitorOrder, &cam.RecordingSource,
		&cam.RecordingBitrateKbps, &cam.LastSeen, &cam.CreatedAt,
	)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: cam})
}

// UpdateCamera PUT /api/v1/cameras/:id
func (h *Handler) updateCameraLegacy(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")
	var req CameraRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	var before Camera
	err := h.db.QueryRow(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras WHERE id = ?
	`, id).Scan(
		&before.ID, &before.Name, &before.Location, &before.IPAddress, &before.Port, &before.Username,
		&before.RTSPUrl, &before.ONVIFUrl, &before.Manufacturer, &before.Model, &before.Firmware,
		&before.SupportsPTZ, &before.IsEnabled, &before.Status, &before.StreamType,
		&before.MonitorDisplay, &before.MonitorOrder, &before.RecordingSource,
		&before.RecordingBitrateKbps, &before.LastSeen, &before.CreatedAt,
	)
	if err == sql.ErrNoRows {
		h.WriteAuditFromContext(c, "update_camera", auditResource("camera", id), "failed", map[string]interface{}{
			"camera_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	var result sql.Result

	if req.RecordingSource == "" {
		req.RecordingSource = "rtsp"
	}
	if req.Password != "" {
		encPwd, encErr := cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password)
		if encErr != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "?ïîº??­æ??"})
			return
		}
		result, err = h.db.Exec(`UPDATE cameras SET name=?, location=?, ip_address=?,
			port=?, rtsp_url=?, onvif_url=?, username=?, password_encrypted=?,
			manufacturer=?, model=?, supports_ptz=?, stream_type=?,
			monitor_display=?, monitor_order=?, recording_source=?, recording_bitrate_kbps=? WHERE id=?`,
			req.Name, req.Location, req.IPAddress, req.Port, req.RTSPUrl, req.ONVIFUrl, req.Username,
			encPwd, req.Manufacturer, req.Model, req.SupportsPTZ, req.StreamType,
			req.MonitorDisplay, req.MonitorOrder, req.RecordingSource, req.RecordingBitrateKbps, id)
	} else {
		result, err = h.db.Exec(`UPDATE cameras SET name=?, location=?, ip_address=?,
			port=?, rtsp_url=?, onvif_url=?, username=?, manufacturer=?, model=?,
			supports_ptz=?, stream_type=?, monitor_display=?, monitor_order=?, recording_source=?, recording_bitrate_kbps=? WHERE id=?`,
			req.Name, req.Location, req.IPAddress, req.Port, req.RTSPUrl, req.ONVIFUrl, req.Username,
			req.Manufacturer, req.Model, req.SupportsPTZ, req.StreamType,
			req.MonitorDisplay, req.MonitorOrder, req.RecordingSource, req.RecordingBitrateKbps, id)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		h.WriteAuditFromContext(c, "update_camera", auditResource("camera", id), "failed", map[string]interface{}{
			"camera_id": id,
			"reason":    "not_found",
		})
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	h.WriteAuditFromContext(c, "update_camera", auditResource("camera", before.ID), "success", map[string]interface{}{
		"camera_id":   before.ID,
		"camera_name": before.Name,
		"ip_address":  before.IPAddress,
		"old_values": map[string]interface{}{
			"name":              before.Name,
			"location":          before.Location,
			"ip_address":        before.IPAddress,
			"port":              before.Port,
			"username":          before.Username,
			"rtsp_url":          before.RTSPUrl,
			"onvif_url":         before.ONVIFUrl,
			"manufacturer":      before.Manufacturer,
			"model":             before.Model,
			"supports_ptz":      before.SupportsPTZ,
			"stream_type":       before.StreamType,
			"monitor_display":   before.MonitorDisplay,
			"monitor_order":     before.MonitorOrder,
			"recording_source":  before.RecordingSource,
			"recording_bitrate": before.RecordingBitrateKbps,
		},
		"new_values": map[string]interface{}{
			"name":              req.Name,
			"location":          req.Location,
			"ip_address":        req.IPAddress,
			"port":              req.Port,
			"username":          req.Username,
			"rtsp_url":          req.RTSPUrl,
			"onvif_url":         req.ONVIFUrl,
			"manufacturer":      req.Manufacturer,
			"model":             req.Model,
			"supports_ptz":      req.SupportsPTZ,
			"stream_type":       req.StreamType,
			"monitor_display":   req.MonitorDisplay,
			"monitor_order":     req.MonitorOrder,
			"recording_source":  req.RecordingSource,
			"recording_bitrate": req.RecordingBitrateKbps,
			"password_updated":  req.Password != "",
		},
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "?î³è£?î?æ­?æ¹î?"})
}

// DeleteCamera DELETE /api/v1/cameras/:id
func (h *Handler) deleteCameraLegacy(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")
	result, err := h.db.Exec("DELETE FROM cameras WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "?î³è£?î?æ­??·î?"})
}

// DiscoverCameras POST /api/v1/cameras/discover
// Scans device list for IPs with RTSP port 554 open.
func (h *Handler) DiscoverCameras(c *gin.Context) {
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	discovered, err := h.camera.DiscoverCandidates(time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "discover_failed"})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"total": len(discovered),
			"data":  discovered,
		},
	})
}

// GetCameraSnapshot GET /api/v1/cameras/:id/snapshot
// Returns a JPEG snapshot captured via ffmpeg from the RTSP stream.
func (h *Handler) GetCameraSnapshot(c *gin.Context) {
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	result, err := h.camera.CaptureSnapshot(c.Param("id"), 15*time.Second)
	if err != nil {
		switch {
		case errors.Is(err, cameramodule.ErrCameraNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		case errors.Is(err, cameramodule.ErrFFmpegNotFound):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "ffmpeg_not_found"})
		case errors.Is(err, cameramodule.ErrSnapshotTimeout):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_snapshot_timeout"})
		case errors.Is(err, cameramodule.ErrSnapshotEmpty):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_snapshot_empty"})
		case errors.Is(err, cameramodule.ErrSnapshotRunFailed):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_snapshot_failed"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}
	c.Data(http.StatusOK, "image/jpeg", result.Data)
}

// CameraHealthCheck POST /api/v1/cameras/:id/health
func (h *Handler) CameraHealthCheck(c *gin.Context) {
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")

	result, err := h.camera.ProbeHealth(id, 3*time.Second)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		return
	}

	log.Printf("[Camera] Health check id=%s status=%s latency=%dms", id, result.Status, result.LatencyMS)
	if cameraID, convErr := strconv.Atoi(id); convErr == nil {
		h.WriteDeviceLog(cameraID, "info", "camera-health", "camera health check", "Camera health check", map[string]interface{}{"status": result.Status, "latency_ms": result.LatencyMS})
	}
	h.WriteSystemLog("info", "camera", "camera_health_check", "camera health check executed", map[string]interface{}{"camera_id": id, "status": result.Status, "latency_ms": result.LatencyMS, "ip_address": result.IPAddress, "camera_port": result.Port})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"status":     result.Status,
			"latency_ms": result.LatencyMS,
		},
	})
}

// ============================================================
// MJPEG Streaming
// ============================================================
// GetCameraMJPEG GET /api/v1/cameras/:id/stream/mjpeg
// Streams MJPEG (multipart/x-mixed-replace) via ffmpeg. One ffmpeg process
// per camera, shared across all viewers. Stops when last viewer disconnects.
func (h *Handler) GetCameraMJPEG(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}
	id := c.Param("id")
	requestStartedAt := time.Now()
	sub, err := h.camera.SubscribeMJPEG(id)
	if err != nil {
		switch {
		case errors.Is(err, cameramodule.ErrCameraNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		case errors.Is(err, cameramodule.ErrFFmpegNotFound):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "ffmpeg_not_found"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}
	cameramodule.Debugf("mjpeg_http_subscribed cam=%s client=%s subscribe_ms=%d", id, c.ClientIP(), time.Since(requestStartedAt).Milliseconds())
	defer sub.Cleanup()
	firstFrame, err := h.camera.WaitForMJPEGFirstFrame(id, sub, 12*time.Second)
	if err != nil {
		switch {
		case errors.Is(err, cameramodule.ErrStreamUnavailable):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_stream_unavailable"})
		case errors.Is(err, cameramodule.ErrStreamTimeout):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_stream_timeout"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}
	cameramodule.Debugf("mjpeg_http_ready cam=%s client=%s wait_first_frame_ms=%d first_bytes=%d", id, c.ClientIP(), time.Since(requestStartedAt).Milliseconds(), len(firstFrame))
	boundary := "mjpegboundary"
	c.Writer.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	c.Writer.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Writer.Header().Set("Pragma", "no-cache")
	c.Writer.Header().Set("Expires", "0")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	frameSeq := 1
	flushStartedAt := time.Now()
	fmt.Fprintf(c.Writer, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(firstFrame))
	_, _ = c.Writer.Write(firstFrame)
	fmt.Fprintf(c.Writer, "\r\n")
	c.Writer.Flush()
	cameramodule.Debugf("mjpeg_http_flush cam=%s client=%s seq=%d bytes=%d flush_ms=%d since_req_ms=%d", id, c.ClientIP(), frameSeq, len(firstFrame), time.Since(flushStartedAt).Milliseconds(), time.Since(requestStartedAt).Milliseconds())
	clientGone := c.Request.Context().Done()
	heartbeat := time.NewTimer(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-clientGone:
			return
		case frame, ok := <-sub.Channel:
			if !ok {
				return
			}
			if !heartbeat.Stop() {
				select {
				case <-heartbeat.C:
				default:
				}
			}
			heartbeat.Reset(10 * time.Second)
			frameSeq++
			flushStartedAt = time.Now()
			fmt.Fprintf(c.Writer, "--%s\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", boundary, len(frame))
			_, _ = c.Writer.Write(frame)
			fmt.Fprintf(c.Writer, "\r\n")
			c.Writer.Flush()
			if frameSeq <= 5 || frameSeq%30 == 0 {
				cameramodule.Debugf("mjpeg_http_flush cam=%s client=%s seq=%d bytes=%d flush_ms=%d since_req_ms=%d", id, c.ClientIP(), frameSeq, len(frame), time.Since(flushStartedAt).Milliseconds(), time.Since(requestStartedAt).Milliseconds())
			}
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, "--%s\r\n\r\n", boundary)
			c.Writer.Flush()
			heartbeat.Reset(10 * time.Second)
		}
	}
}

func waitForReadableFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return os.ErrNotExist
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func rewriteHLSPlaylist(content []byte, token string) []byte {
	if strings.TrimSpace(token) == "" {
		return content
	}
	escaped := url.QueryEscape(token)
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, ".ts") && !strings.Contains(trimmed, "?") {
			suffix := ""
			if strings.HasSuffix(line, "\r") {
				line = strings.TrimSuffix(line, "\r")
				suffix = "\r"
			}
			lines[i] = line + "?token=" + escaped + suffix
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// GetCameraHLS GET /api/v1/cameras/:id/stream/hls/:file
// Optional stable preview path. MJPEG remains the default low-latency grid path.
func (h *Handler) GetCameraHLS(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	file := c.Param("file")
	if file == "" || filepath.Base(file) != file || !(file == "index.m3u8" || strings.HasSuffix(file, ".ts")) {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid_hls_file"})
		return
	}

	id := c.Param("id")
	dir, err := h.camera.EnsureHLSStream(id)
	if err != nil {
		switch {
		case errors.Is(err, cameramodule.ErrCameraNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		case errors.Is(err, cameramodule.ErrFFmpegNotFound):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "ffmpeg_not_found"})
		case errors.Is(err, cameramodule.ErrStreamUnavailable):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_stream_unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}

	path := filepath.Join(dir, file)
	if err := waitForReadableFile(path, 6*time.Second); err != nil {
		c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_hls_not_ready"})
		return
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	if file == "index.m3u8" {
		data, err := os.ReadFile(path)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "camera_hls_not_ready"})
			return
		}
		c.Data(http.StatusOK, "application/vnd.apple.mpegurl; charset=utf-8", rewriteHLSPlaylist(data, c.Query("token")))
		return
	}
	c.File(path)
}

// GetCameraWebRTCWS GET /api/v1/cameras/:id/stream/webrtc/ws
// Proxies go2rtc WebRTC signaling while keeping the go2rtc API bound to localhost.
func (h *Handler) GetCameraWebRTCWS(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	id := c.Param("id")
	streamName, err := h.camera.EnsureWebRTCStream(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, cameramodule.ErrCameraNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "camera_not_found"})
		case errors.Is(err, cameramodule.ErrGo2RTCNotFound):
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "go2rtc_not_found"})
		case cameramodule.IsGo2RTCError(err):
			log.Printf("[Camera] go2rtc stream prepare failed camera_id=%s: %v", id, err)
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "go2rtc_unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}

	if err := h.camera.ProxyGo2RTCWebSocket(c.Writer, c.Request, streamName); err != nil {
		log.Printf("[Camera] WebRTC signaling proxy failed camera_id=%s: %v", id, err)
	}
}

// ============================================================
// NVR / Recording state management
// ============================================================

// nvrRecordingLicenseEnabled returns true if the NVR recording feature is unlocked.
// camera_recording_enabled is set by a camera_recording feature license.
// camera_viewer_enabled is set by a camera feature license and also implies recording rights.
func (h *Handler) nvrRecordingLicenseEnabled() bool {
	return h.camera.RecordingLicenseEnabled()
}

// SetCameraRecording enables or disables recording for a camera.
// PUT /cameras/:id/recording
func (h *Handler) SetCameraRecording(c *gin.Context) {
	idStr := c.Param("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	var cameraID int
	if _, err := fmt.Sscanf(idStr, "%d", &cameraID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	if req.Enabled && !h.nvrRecordingLicenseEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "recording_not_licensed", "message": "recording_not_licensed"})
		return
	}
	if err := h.camera.SetRecording(cameraID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "camera_id": cameraID, "recording": req.Enabled})
}

// BatchSetRecording toggles recording for multiple cameras.
// PUT /cameras/recording/batch
func (h *Handler) BatchSetRecording(c *gin.Context) {
	var req struct {
		CameraIDs []int `json:"camera_ids"`
		Enabled   bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.Enabled && !h.nvrRecordingLicenseEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "recording_not_licensed", "message": "recording_not_licensed"})
		return
	}
	results := h.camera.BatchSetRecording(req.CameraIDs, req.Enabled)
	c.JSON(http.StatusOK, gin.H{"success": true, "results": results})
}

// GetRecordingStatus returns current runtime recording state.
// GET /cameras/recording/status
func (h *Handler) GetRecordingStatus(c *gin.Context) {
	list, active, err := h.camera.ListRecordingStatusRuntime()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "cameras": list, "active_sessions": active})
}

// ListRecordings GET /recordings
func (h *Handler) ListRecordings(c *gin.Context) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "20")))
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	result, err := h.camera.ListRecordings(
		strings.TrimSpace(c.Query("camera_id")),
		strings.TrimSpace(c.Query("label")),
		strings.TrimSpace(c.Query("date")),
		page,
		limit,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"recordings": result.Recordings,
		"total":      result.Total,
		"page":       result.Page,
		"limit":      result.Limit,
	})
}

// ListRecordingDates GET /recordings/dates
func (h *Handler) ListRecordingDates(c *gin.Context) {
	dates, err := h.camera.ListRecordingDates(strings.TrimSpace(c.Query("camera_id")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "dates": dates})
}

// UpdateRecordingLabel PUT /recordings/:id/label
func (h *Handler) UpdateRecordingLabel(c *gin.Context) {
	var req struct {
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	updated, err := h.camera.UpdateRecordingLabel(c.Param("id"), strings.TrimSpace(req.Label))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteRecording DELETE /recordings/:id
func (h *Handler) DeleteRecording(c *gin.Context) {
	filePath, deleted, err := h.camera.DeleteRecording(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "not found"})
		return
	}
	if filePath != "" {
		_ = os.Remove(filePath)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BatchDeleteRecordings DELETE /recordings/batch
func (h *Handler) BatchDeleteRecordings(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	paths, deleted, err := h.camera.BatchDeleteRecordings(req.IDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	for _, p := range paths {
		if p != "" {
			_ = os.Remove(p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "deleted": deleted})
}

func recordingContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mpegts", ".mts":
		return "video/mp2t"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	default:
		return "video/mp4"
	}
}

// PlayRecording GET /recordings/:id/play
func (h *Handler) PlayRecording(c *gin.Context) {
	id := c.Param("id")
	fp, err := h.camera.GetRecordingFile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	f, err := os.Open(fp)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found on disk"})
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(fp)))
	c.Header("Content-Type", recordingContentType(fp))
	http.ServeContent(c.Writer, c.Request, filepath.Base(fp), fi.ModTime(), f)
}

// ExportRecording GET /recordings/:id/export
func (h *Handler) ExportRecording(c *gin.Context) {
	id := c.Param("id")
	recording, err := h.camera.GetRecordingExport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if _, err := os.Stat(recording.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found on disk"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, recording.FileName))
	c.Header("Content-Type", recordingContentType(recording.FilePath))
	http.ServeFile(c.Writer, c.Request, recording.FilePath)
}

// GetRecordingStats GET /recordings/stats
func (h *Handler) GetRecordingStats(c *gin.Context) {
	stats, err := h.camera.GetRecordingStats(h.camera.StorageDir(), h.camera.ActiveRecordingCount())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"total_recordings":   stats.TotalRecordings,
		"total_size_bytes":   stats.TotalSizeBytes,
		"total_duration_sec": stats.TotalDurationSec,
		"active_recordings":  stats.ActiveRecordings,
		"recording_dir":      stats.RecordingDir,
	})
}

// GetNVRConfig GET /nvr/config
func (h *Handler) GetNVRConfig(c *gin.Context) {
	cfg, err := h.camera.GetNVRConfig(h.camera.StorageDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "storage_dir": cfg.StorageDir, "used_bytes": cfg.UsedBytes})
}

// SetNVRConfig PUT /nvr/config
func (h *Handler) SetNVRConfig(c *gin.Context) {
	var req struct {
		StorageDir string `json:"storage_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	dir, err := h.camera.SetNVRStorageDir(req.StorageDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "storage_dir": dir})
}

// ProbeONVIFStream POST /cameras/onvif/probe
func (h *Handler) ProbeONVIFStream(c *gin.Context) {
	var req struct {
		ONVIFURL string `json:"onvif_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	req.ONVIFURL = strings.TrimSpace(req.ONVIFURL)
	if req.ONVIFURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing_onvif_url"})
		return
	}
	rtspURL, err := cameramodule.GetONVIFStreamURI(req.ONVIFURL, strings.TrimSpace(req.Username), req.Password)
	if err != nil || strings.TrimSpace(rtspURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "onvif_probe_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"rtsp_url": rtspURL}})
}

// BatchUpdateCredentials PUT /cameras/batch/credentials
func (h *Handler) BatchUpdateCredentials(c *gin.Context) {
	var req struct {
		CameraIDs  []int  `json:"camera_ids"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		RTSPPort   int    `json:"rtsp_port"`
		UpdateRTSP bool   `json:"update_rtsp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	input := cameramodule.BatchCredentialUpdateInput{
		CameraIDs:  req.CameraIDs,
		Username:   strings.TrimSpace(req.Username),
		Password:   req.Password,
		RTSPPort:   req.RTSPPort,
		UpdateRTSP: req.UpdateRTSP,
	}
	if req.Password != "" {
		encPwd, err := cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "encrypt_password_failed"})
			return
		}
		input.PasswordEncrypted = &encPwd
	}
	result, err := h.camera.BatchUpdateCredentials(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "updated": result.Updated, "failed": result.Failed, "total": result.Total})
}

// BulkScanCameras POST /cameras/bulk-scan
func (h *Handler) BulkScanCameras(c *gin.Context) {
	if err := h.ensureCameraSchema(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !h.cameraLicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "camera_not_licensed"})
		return
	}

	var req struct {
		ScanType   string `json:"scan_type"`
		ScanMode   string `json:"scan_mode"`
		RTSPPort   int    `json:"rtsp_port"`
		ONVIFPort  int    `json:"onvif_port"`
		Username   string `json:"username"`
		Password   string `json:"password"`
		AutoAdd    bool   `json:"auto_add"`
		NamePrefix string `json:"name_prefix"`
		StartIP    string `json:"start_ip"`
		EndIP      string `json:"end_ip"`
		Subnet     string `json:"subnet"`
		SingleIP   string `json:"single_ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	if req.RTSPPort <= 0 {
		req.RTSPPort = 554
	}
	if req.ONVIFPort <= 0 {
		req.ONVIFPort = 80
	}
	if strings.TrimSpace(req.NamePrefix) == "" {
		req.NamePrefix = "Camera-"
	}
	mode := strings.ToLower(strings.TrimSpace(req.ScanMode))
	if mode == "" {
		mode = "both"
	}

	ips := make([]string, 0)
	switch strings.ToLower(strings.TrimSpace(req.ScanType)) {
	case "single":
		if ip := net.ParseIP(strings.TrimSpace(req.SingleIP)); ip != nil {
			ips = append(ips, ip.String())
		}
	case "subnet":
		expanded, err := cameramodule.ExpandSubnet(strings.TrimSpace(req.Subnet))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_subnet"})
			return
		}
		ips = expanded
	default:
		expanded, err := cameramodule.ExpandRange(strings.TrimSpace(req.StartIP), strings.TrimSpace(req.EndIP))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_range"})
			return
		}
		ips = expanded
	}
	if len(ips) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "no_scan_targets"})
		return
	}
	if len(ips) > 512 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "too_many_targets"})
		return
	}

	type scanResult struct {
		IPAddress  string `json:"ip_address"`
		RTSPFound  bool   `json:"rtsp_found"`
		ONVIFFound bool   `json:"onvif_found"`
		AlreadyIn  bool   `json:"already_in"`
		Added      bool   `json:"added"`
	}
	results := make([]scanResult, 0, len(ips))
	addedCount := 0
	currentCount, _ := h.camera.CountCameras()
	maxAllowed := h.cameraMaxAllowed()
	var encPwd *string
	if req.Password != "" {
		if encrypted, err := cameramodule.EncryptPassword(h.config.Security.JWTSecret, req.Password); err == nil {
			encPwd = &encrypted
		}
	}
	for _, ip := range ips {
		res := scanResult{IPAddress: ip}
		if mode == "rtsp" || mode == "both" {
			res.RTSPFound = cameramodule.CheckRTSP(ip, req.RTSPPort)
		}
		if mode == "onvif" || mode == "both" {
			found, _ := cameramodule.ProbeONVIFPort(ip, req.ONVIFPort)
			res.ONVIFFound = found
		}
		var existingID int
		if err := h.db.QueryRow(`SELECT id FROM cameras WHERE ip_address=? LIMIT 1`, ip).Scan(&existingID); err == nil {
			res.AlreadyIn = true
		}
		if req.AutoAdd && !res.AlreadyIn && currentCount < maxAllowed && (res.RTSPFound || res.ONVIFFound) {
			rtspURL := ""
			onvifURL := ""
			if res.RTSPFound {
				if strings.TrimSpace(req.Username) != "" {
					rtspURL = fmt.Sprintf("rtsp://%s:%s@%s:%d/stream1", strings.ReplaceAll(req.Username, "@", "%40"), strings.ReplaceAll(req.Password, "@", "%40"), ip, req.RTSPPort)
				} else {
					rtspURL = fmt.Sprintf("rtsp://%s:%d/stream1", ip, req.RTSPPort)
				}
			}
			if res.ONVIFFound {
				onvifURL = fmt.Sprintf("http://%s:%d/onvif/device_service", ip, req.ONVIFPort)
				if rtspURL == "" && strings.TrimSpace(req.Username) != "" {
					if fetched, err := cameramodule.GetONVIFStreamURI(onvifURL, strings.TrimSpace(req.Username), req.Password); err == nil {
						rtspURL = fetched
					}
				}
			}
			name := req.NamePrefix + strings.ReplaceAll(ip, ".", "-")
			if _, err := h.camera.CreateCamera(cameramodule.CameraInput{
				Name:              name,
				IPAddress:         ip,
				Port:              req.RTSPPort,
				RTSPUrl:           rtspURL,
				ONVIFUrl:          onvifURL,
				Username:          strings.TrimSpace(req.Username),
				PasswordEncrypted: encPwd,
				StreamType:        "mjpeg",
				RecordingSource:   "rtsp",
			}); err == nil {
				res.Added = true
				addedCount++
				currentCount++
			}
		}
		results = append(results, res)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "results": results, "scanned": len(ips), "added": addedCount})
}

// NVRStartAll starts all enabled recording sessions on boot.
func (h *Handler) NVRStartAll() {
	if h.shouldLockSession() {
		log.Printf("[Camera] skip auto-start while PoC session is locked")
		return
	}
	h.camera.StartAllRecordings()
}

// StartCameraHealthLoop retries only offline cameras so recovery is detected.
func (h *Handler) StartCameraHealthLoop() {
	go func() {
		ticker := time.NewTicker(90 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if h.shouldLockSession() {
				continue
			}
			results, err := h.camera.RecoverOfflineCameras(3 * time.Second)
			if err != nil {
				log.Printf("[Camera] health loop failed: %v", err)
				continue
			}
			for _, result := range results {
				h.WriteSystemLog("info", "camera", "camera_port_reachable", "camera port reachable; waiting for frame", map[string]interface{}{
					"camera_id":  result.CameraID,
					"ip_address": result.IPAddress,
					"latency_ms": result.LatencyMS,
				})
			}
		}
	}()
}
