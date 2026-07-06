// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	iotmodule "management-server/modules/iot"

	"github.com/gin-gonic/gin"
)

func (h *Handler) iotGuard(c *gin.Context) bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	if !h.iot.LicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "iot_not_licensed"})
		return false
	}
	return true
}

func (h *Handler) GetIoTStatus(c *gin.Context) {
	status, err := h.iot.Status()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) StartIoTLoop() {
	h.iot.StartBackgroundLoop()
}

func (h *Handler) GetIoTCapabilities(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: h.iot.Capabilities()})
}

func (h *Handler) ListIoTDevices(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	devices, err := h.iot.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

func (h *Handler) CreateIoTDevice(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	var input iotmodule.UpsertDeviceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.iot.CreateDevice(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"id": id}})
}

func (h *Handler) UpdateIoTDevice(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	var input iotmodule.UpsertDeviceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.iot.UpdateDevice(c.Param("id"), input); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "iot device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) DeleteIoTDevice(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	if err := h.iot.DeleteDevice(c.Param("id")); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "iot device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) PollIoTDevice(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	device, err := h.iot.PollDevice(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}

func (h *Handler) IngestIoTMeasurement(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	var input iotmodule.IngestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.iot.Ingest(input); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) GetIoTMeasurements(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.iot.RecentMeasurements(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: items})
}

func (h *Handler) GetIoTQueueStatus(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	status, err := h.iot.QueueStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) GetIoTForwarderSettings(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	settings, err := h.iot.ForwarderSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: settings})
}

func (h *Handler) UpdateIoTForwarderSettings(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	var input iotmodule.ForwarderSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.iot.UpdateForwarderSettings(input); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) FlushIoTForwardQueue(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	sent, err := h.iot.FlushForwardQueue()
	if err != nil {
		c.JSON(http.StatusBadGateway, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"sent": sent}})
}

func (h *Handler) CleanupIoTForwardQueue(c *gin.Context) {
	if !h.iotGuard(c) {
		return
	}
	deleted, err := h.iot.CleanupForwardedMeasurements()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"deleted": deleted}})
}
