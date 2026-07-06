// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"net/http"
	"time"

	pdumodule "management-server/modules/pdu"

	"github.com/gin-gonic/gin"
)

func (h *Handler) pduGuard(c *gin.Context) bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	if !h.pdu.LicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "pdu_not_licensed"})
		return false
	}
	return true
}

func (h *Handler) GetPDUModuleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Success: true, Data: h.pdu.ModuleStatus()})
}

func (h *Handler) GetPDUDevices(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	devices, err := h.pdu.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: devices})
}

func (h *Handler) GetPDUDevice(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	device, err := h.pdu.GetDevice(c.Param("id"), true)
	if err != nil {
		if errors.Is(err, pdumodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "pdu_device_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}

func (h *Handler) CreatePDUDevice(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	var req pdumodule.DeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.IPAddress == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "name and ip_address are required"})
		return
	}
	id, err := h.pdu.CreateDevice(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]interface{}{"id": id}})
}

func (h *Handler) UpdatePDUDevice(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	var req pdumodule.DeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.pdu.UpdateDevice(c.Param("id"), req); err != nil {
		if errors.Is(err, pdumodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "pdu_device_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) DeletePDUDevice(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	if err := h.pdu.DeleteDevice(c.Param("id")); err != nil {
		if errors.Is(err, pdumodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "pdu_device_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) PollPDUDevice(c *gin.Context) {
	if !h.pduGuard(c) {
		return
	}
	device, err := h.pdu.PollDevice(c.Param("id"))
	if err != nil {
		if errors.Is(err, pdumodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "pdu_device_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: device})
}
