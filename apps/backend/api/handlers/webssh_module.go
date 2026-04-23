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
