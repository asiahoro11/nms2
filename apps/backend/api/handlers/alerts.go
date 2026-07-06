// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	notificationsmodule "management-server/modules/notifications"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) GetAlertSettings(c *gin.Context) {
	settings, err := h.notifications.GetAlertSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    settings,
	})
}

func (h *Handler) UpdateAlertSetting(c *gin.Context) {
	alertType := c.Param("type")

	var input notificationsmodule.UpdateAlertSettingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	err := h.notifications.UpdateAlertSetting(alertType, input)
	if err != nil {
		switch {
		case errors.Is(err, notificationsmodule.ErrAlertFeatureNotLicensed):
			c.JSON(http.StatusForbidden, Response{Success: false, Error: "此通知通道未授權，請先啟用對應授權"})
		case errors.Is(err, notificationsmodule.ErrAlertSettingNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "找不到指定的通知通道設定"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Message: "通知設定已更新"})
}

func (h *Handler) TestAlert(c *gin.Context) {
	alertType := c.Param("type")

	if err := h.notifications.TestAlert(alertType); err != nil {
		if errors.Is(err, notificationsmodule.ErrAlertSettingNotFound) {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "找不到通知通道設定"})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("測試通知發送失敗: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "測試通知已送出，請確認目標通道是否收到訊息",
	})
}

func (h *Handler) getLicenseFeatures() []string {
	featureFlags := h.notifications.FeatureFlags()
	result := make([]string, 0, len(featureFlags))
	for feature, enabled := range featureFlags {
		if enabled {
			result = append(result, feature)
		}
	}
	return result
}

func (h *Handler) GetDeviceEvents(c *gin.Context) {
	deviceID := c.Param("id")
	limit := c.DefaultQuery("limit", "50")

	rows, err := h.db.Query(`
		SELECT id, event_type, severity, message, created_at
		FROM events
		WHERE device_id = ?
		AND (event_type LIKE '%port%' OR event_type LIKE '%device%' OR event_type LIKE '%link%' OR event_type LIKE '%status%')
		ORDER BY created_at DESC
		LIMIT ?
	`, deviceID, limit)

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var events []map[string]interface{}
	for rows.Next() {
		var id int
		var eventType, severity, message, createdAt string

		if err := rows.Scan(&id, &eventType, &severity, &message, &createdAt); err != nil {
			continue
		}

		events = append(events, map[string]interface{}{
			"id":         id,
			"event_type": eventType,
			"severity":   severity,
			"message":    message,
			"created_at": createdAt,
		})
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    events,
	})
}

func (h *Handler) legacyGetBrandingSettings(c *gin.Context) {
	var logoPath, companyName string

	h.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_logo_path'`).Scan(&logoPath)
	h.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_name'`).Scan(&companyName)

	if companyName == "" {
		companyName = "Management Server"
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]string{
			"logo_path":    logoPath,
			"company_name": companyName,
		},
	})
}

func (h *Handler) legacyUpdateBrandingSettings(c *gin.Context) {
	var input struct {
		CompanyName string `json:"company_name"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_name', ?, CURRENT_TIMESTAMP)`, input.CompanyName)

	c.JSON(http.StatusOK, Response{Success: true, Message: "品牌設定已更新"})
}

func (h *Handler) legacyUploadBrandingLogo(c *gin.Context) {
	file, err := c.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供要上傳的 Logo 檔案"})
		return
	}

	filename := "company_logo_" + uuid.New().String() + filepath.Ext(file.Filename)

	dataPath := "../data"
	if _, err := os.Stat("./data"); err == nil {
		dataPath = "./data"
	}
	savePath := filepath.Join(dataPath, "uploads", filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	logoURL := "/uploads/" + filename
	h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_logo_path', ?, CURRENT_TIMESTAMP)`, logoURL)

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"logo_path": logoURL},
		Message: "Logo 已更新",
	})
}
