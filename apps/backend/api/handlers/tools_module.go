// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"net/http"

	toolsmodule "management-server/modules/tools"

	"github.com/gin-gonic/gin"
)

func (h *Handler) PingTool(c *gin.Context) {
	var req toolsmodule.PingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if len(req.Targets) == 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No targets provided"})
		return
	}
	if len(req.Targets) > 10 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Too many targets (max 10)"})
		return
	}

	resp, err := h.tools.Ping(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) TracerouteTool(c *gin.Context) {
	var req toolsmodule.TracerouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if req.Target == "" {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Target is required"})
		return
	}

	resp, err := h.tools.Traceroute(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
