package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetDashboard(c *gin.Context) {
	data, err := h.dashboard.GetDashboard()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	data.Version = h.license.DisplayVersion()
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

func (h *Handler) GetTopCPU(c *gin.Context) {
	data, err := h.dashboard.GetTopCPU()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

func (h *Handler) GetTopMemory(c *gin.Context) {
	data, err := h.dashboard.GetTopMemory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}
