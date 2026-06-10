package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	iotmodule "management-server/modules/iot"

	"github.com/gin-gonic/gin"
)

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
	c.JSON(http.StatusOK, Response{Success: true, Data: h.iot.Capabilities()})
}

func (h *Handler) ListIoTDevices(c *gin.Context) {
	devices, err := h.iot.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

func (h *Handler) CreateIoTDevice(c *gin.Context) {
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
	device, err := h.iot.PollDevice(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}

func (h *Handler) IngestIoTMeasurement(c *gin.Context) {
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.iot.RecentMeasurements(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: items})
}

func (h *Handler) GetIoTQueueStatus(c *gin.Context) {
	status, err := h.iot.QueueStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) GetIoTForwarderSettings(c *gin.Context) {
	settings, err := h.iot.ForwarderSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: settings})
}

func (h *Handler) UpdateIoTForwarderSettings(c *gin.Context) {
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
	sent, err := h.iot.FlushForwardQueue()
	if err != nil {
		c.JSON(http.StatusBadGateway, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"sent": sent}})
}

func (h *Handler) CleanupIoTForwardQueue(c *gin.Context) {
	deleted, err := h.iot.CleanupForwardedMeasurements()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"deleted": deleted}})
}
