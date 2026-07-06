// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	notificationsmodule "management-server/modules/notifications"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetNotifications(c *gin.Context) {
	unreadOnly := c.Query("unread") == "1"

	list, unreadCount, err := h.notifications.GetNotifications(unreadOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"data":         list,
		"unread_count": unreadCount,
	})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid id"})
		return
	}

	if err := h.notifications.MarkNotificationRead(id); err != nil {
		if errors.Is(err, notificationsmodule.ErrNotificationNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "找不到指定通知"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	if err := h.notifications.MarkAllNotificationsRead(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) DeleteNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid id"})
		return
	}

	if err := h.notifications.DeleteNotification(id); err != nil {
		if errors.Is(err, notificationsmodule.ErrNotificationNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "找不到指定通知"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) DeleteAllNotifications(c *gin.Context) {
	if err := h.notifications.DeleteAllNotifications(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}
