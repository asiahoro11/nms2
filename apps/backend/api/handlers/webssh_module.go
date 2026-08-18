// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"net/http"

	websshmodule "management-server/modules/webssh"

	"github.com/gin-gonic/gin"
)

func (h *Handler) WebSSHTerminal(c *gin.Context) {
	if err := h.webssh.Terminal(c); err != nil {
		if errors.Is(err, websshmodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
	}
}

func (h *Handler) CreateWebSSHTicket(c *gin.Context) {
	ticket, expiresAt, err := h.webssh.IssueTicket(c.Param("id"))
	if err != nil {
		if errors.Is(err, websshmodule.ErrDeviceNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to issue terminal ticket"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"ticket": ticket, "expires_at": expiresAt.Unix()}})
}
