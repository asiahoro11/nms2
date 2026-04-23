package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	acmodule "management-server/modules/accesscontrol"

	"github.com/gin-gonic/gin"
)

func (h *Handler) acGuard(c *gin.Context) bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	if !h.accessControl.LicenseEnabled() {
		c.JSON(http.StatusForbidden, Response{Success: false, Error: "access_control_not_licensed"})
		return false
	}
	return true
}

func (h *Handler) GetACModuleStatus(c *gin.Context) {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	status, err := h.accessControl.ModuleStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) GetDoors(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	doors, err := h.accessControl.ListDoors()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: doors})
}

func (h *Handler) GetDoor(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	door, err := h.accessControl.GetDoor(c.Param("id"))
	if err != nil {
		if errors.Is(err, acmodule.ErrDoorNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "door_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: door})
}

func (h *Handler) CreateDoor(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.DoorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.accessControl.CreateDoor(req, h.config.Security.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "門禁控制器已新增",
		Data:    map[string]int64{"id": id},
	})
}

func (h *Handler) UpdateDoor(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.DoorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	ok, err := h.accessControl.UpdateDoor(c.Param("id"), req, h.config.Security.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "door_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "門禁控制器已更新"})
}

func (h *Handler) DeleteDoor(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	ok, err := h.accessControl.DeleteDoor(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "door_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "門禁控制器已刪除"})
}

func (h *Handler) ControlDoor(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var body struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	result, err := h.accessControl.ControlDoor(c.Param("id"), body.Action)
	if err != nil {
		if errors.Is(err, acmodule.ErrDoorNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "door_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "門禁操作已送出", Data: result})
}

func (h *Handler) GetCards(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	cards, err := h.accessControl.ListCards(c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: cards})
}

func (h *Handler) CreateCard(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.CardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.accessControl.CreateCard(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "卡片已新增",
		Data:    map[string]int64{"id": id},
	})
}

func (h *Handler) UpdateCard(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.CardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	ok, err := h.accessControl.UpdateCard(c.Param("id"), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "card_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "卡片已更新"})
}

func (h *Handler) DeleteCard(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	ok, err := h.accessControl.DeleteCard(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, Response{Success: false, Error: "card_not_found"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "卡片已刪除"})
}

func (h *Handler) GetACEvents(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	events, err := h.accessControl.ListEvents(acmodule.EventsQuery{
		DoorID:     c.Query("door_id"),
		CardNumber: c.Query("card_number"),
		EventType:  c.Query("event_type"),
		Limit:      limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: events})
}

func (h *Handler) AddACEvent(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var body acmodule.EventCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.accessControl.AddEvent(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

func (h *Handler) GetCardSchedules(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	schedules, err := h.accessControl.ListSchedules(c.Query("status"), c.Query("card_number"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: schedules})
}

func (h *Handler) CreateCardSchedule(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	id, err := h.accessControl.CreateSchedule(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

func (h *Handler) UpdateCardSchedule(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var req acmodule.ScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}
	if err := h.accessControl.UpdateSchedule(c.Param("id"), req); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) ApproveCardSchedule(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	_ = c.ShouldBindJSON(&body)
	status, err := h.accessControl.ApproveSchedule(c.Param("id"), c.GetString("username"), body.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	message := "排程已審核"
	if status == "rejected" {
		message = "排程已駁回"
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: message})
}

func (h *Handler) DeleteCardSchedule(c *gin.Context) {
	if !h.acGuard(c) {
		return
	}
	if err := h.accessControl.DeleteSchedule(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) CheckCardScheduleAccess(c *gin.Context) {
	allowed, err := h.accessControl.CheckScheduleAccess(c.Query("card_number"), c.Query("door_id"))
	if err != nil {
		if errors.Is(err, acmodule.ErrScheduleDenied) {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "card_number required"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]bool{"allowed": allowed}})
}
