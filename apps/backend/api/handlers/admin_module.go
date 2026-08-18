// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	adminmodule "management-server/modules/admin"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetUsers(c *gin.Context) {
	users, err := h.admin.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: users})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var input adminmodule.CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	id, err := h.admin.CreateUser(input, c.GetString("role"))
	if err != nil {
		if errors.Is(err, adminmodule.ErrSuperAdminCreation) {
			c.JSON(http.StatusForbidden, Response{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var input adminmodule.UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if err := h.admin.UpdateUser(c.Param("id"), input, c.GetString("role")); err != nil {
		if errors.Is(err, adminmodule.ErrProtectedUser) {
			c.JSON(http.StatusForbidden, Response{Success: false, Error: err.Error()})
			return
		}
		if errors.Is(err, adminmodule.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "User updated"})
}

func (h *Handler) DeleteUser(c *gin.Context) {
	err := h.admin.DeleteUser(c.Param("id"), c.GetString("role"))
	if err != nil {
		switch {
		case errors.Is(err, adminmodule.ErrCannotDeleteLastAdmin):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Cannot delete the last admin user"})
		case errors.Is(err, adminmodule.ErrUserNotFound):
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "User not found"})
		case errors.Is(err, adminmodule.ErrProtectedUser):
			c.JSON(http.StatusForbidden, Response{Success: false, Error: "Protected account cannot be deleted"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "User deleted"})
}

func (h *Handler) GetBrandingSettings(c *gin.Context) {
	settings, err := h.admin.GetBrandingSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: settings})
}

func (h *Handler) UpdateBrandingSettings(c *gin.Context) {
	var input adminmodule.UpdateBrandingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	settings, err := h.admin.UpdateBrandingSettings(input)
	if err != nil {
		h.WriteAuditFromContext(c, "update_branding", "branding", "failed", hiddenSessionAudit(c))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	h.WriteAuditFromContext(c, "update_branding", "branding", "success", hiddenSessionAudit(c))

	c.JSON(http.StatusOK, Response{Success: true, Data: settings, Message: "Branding settings updated"})
}

func (h *Handler) UploadBrandingLogo(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 3<<20)
	file, err := c.FormFile("logo")
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "logo upload is required"})
		return
	}
	if file.Size <= 0 || file.Size > 2<<20 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "logo must be between 1 byte and 2 MiB"})
		return
	}
	source, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid logo"})
		return
	}
	header := make([]byte, 512)
	n, readErr := source.Read(header)
	_ = source.Close()
	if readErr != nil && n == 0 {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid logo"})
		return
	}
	extensions := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}
	ext, allowed := extensions[http.DetectContentType(header[:n])]
	if !allowed {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "only PNG, JPEG, or WebP logos are allowed"})
		return
	}

	tempPath := filepath.Join(os.TempDir(), "branding_logo_"+time.Now().Format("20060102150405")+ext)
	defer os.Remove(tempPath)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	settings, err := h.admin.ReplaceBrandingLogo("logo"+ext, tempPath)
	if err != nil {
		h.WriteAuditFromContext(c, "upload_branding_logo", "branding:logo", "failed", hiddenSessionAudit(c))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	h.WriteAuditFromContext(c, "upload_branding_logo", "branding:logo", "success", hiddenSessionAudit(c))

	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]string{"logo_path": settings.LogoPath}, Message: "Logo updated"})
}

func (h *Handler) DeleteBrandingLogo(c *gin.Context) {
	if err := h.admin.DeleteBrandingLogo(); err != nil {
		h.WriteAuditFromContext(c, "delete_branding_logo", "branding:logo", "failed", hiddenSessionAudit(c))
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	h.WriteAuditFromContext(c, "delete_branding_logo", "branding:logo", "success", hiddenSessionAudit(c))
	c.JSON(http.StatusOK, Response{Success: true, Message: "Logo deleted"})
}

func (h *Handler) GetSecuritySettings(c *gin.Context) {
	settings, err := h.admin.GetSecuritySettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: settings})
}

func (h *Handler) UpdateSecuritySettings(c *gin.Context) {
	var input adminmodule.UpdateSecuritySettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	settings, err := h.admin.UpdateSecuritySettings(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: settings, Message: "Security settings updated"})
}

func (h *Handler) GetSystemConfig(c *gin.Context) {
	configs, err := h.admin.ListSystemConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: configs})
}

func (h *Handler) UpdateSystemConfig(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		h.WriteSystemLog("warning", "system-config", "update_system_config_failed", "system config update rejected", map[string]interface{}{"reason": "missing_config_key"})
		h.WriteAuditFromContext(c, "update_system_config", "system_config", "failed", map[string]interface{}{"module": "system", "reason": "missing_config_key"})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "missing config key"})
		return
	}

	var input adminmodule.UpdateSystemConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "system-config", "update_system_config_failed", "system config update rejected", map[string]interface{}{
			"config_key": key,
			"reason":     "invalid_request",
			"error":      err.Error(),
		})
		h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "failed", map[string]interface{}{
			"module": "system",
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	result, err := h.admin.UpdateSystemConfig(key, input)
	if err != nil {
		h.WriteSystemLog("error", "system-config", "update_system_config_failed", "system config update failed", map[string]interface{}{
			"config_key": key,
			"error":      err.Error(),
		})
		h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "failed", map[string]interface{}{
			"module":     "system",
			"reason":     "db_upsert_failed",
			"error":      err.Error(),
			"config_key": key,
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "update_system_config", auditResource("system_config", key), "success", map[string]interface{}{
		"module":        "system",
		"resource_type": "system_config",
		"resource_id":   key,
		"resource_name": key,
		"config_key":    key,
		"old_values": map[string]interface{}{
			"config_value": result.OldValue,
			"description":  result.OldDescription,
			"existed":      result.Existed,
		},
		"new_values": map[string]interface{}{
			"config_value": result.NewValue,
			"description":  result.Description,
		},
		"config_change": true,
		"change_scope":  "system_config",
	})
	h.WriteSystemLog("notice", "system-config", "update_system_config", "system config updated", map[string]interface{}{
		"config_key":   key,
		"config_value": result.NewValue,
		"description":  result.Description,
	})

	c.JSON(http.StatusOK, Response{Success: true, Data: map[string]string{
		"config_key":   result.ConfigKey,
		"config_value": result.NewValue,
		"description":  result.Description,
	}})
}
