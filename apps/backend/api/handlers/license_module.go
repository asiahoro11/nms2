package handlers

import (
	"net/http"
	"strings"

	licensemodule "management-server/modules/license"
	licensesvc "management-server/services/license"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetEncryptedMachineID(c *gin.Context) {
	result, err := h.license.EncryptedMachineID(getSystemUUID(), h.getSecretKey())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to encrypt machine id"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) ActivateLicense(c *gin.Context) {
	var input ActivateLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "license", "activate_license_failed", "license activation rejected", map[string]interface{}{
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_license", "license", "failed", map[string]interface{}{
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid license request"})
		return
	}

	key := strings.TrimSpace(input.LicenseKey)
	if licensesvc.IsTrialKey(key) {
		result, err := h.license.ActivateTrial(getSystemUUID())
		if err != nil {
			reason := "database_write_failed"
			if strings.Contains(err.Error(), "trial_already_used") {
				reason = "trial_already_used"
			}
			h.WriteSystemLog("warning", "license", "activate_trial_license_failed", "trial activation failed", map[string]interface{}{
				"reason": reason,
				"error":  err.Error(),
			})
			h.WriteAuditFromContext(c, "activate_trial_license", "license:trial", "failed", map[string]interface{}{
				"reason": reason,
				"error":  err.Error(),
			})
			status := http.StatusInternalServerError
			if reason == "trial_already_used" {
				status = http.StatusBadRequest
			}
			c.JSON(status, Response{Success: false, Error: err.Error()})
			return
		}
		h.WriteSystemLog("notice", "license", "activate_trial_license", "trial license activated", map[string]interface{}{
			"device_count": result.Payload.DeviceCount,
			"features":     result.Payload.Features,
			"valid_until":  result.Payload.ValidUntil,
		})
		h.WriteAuditFromContext(c, "activate_trial_license", "license:trial", "success", map[string]interface{}{
			"device_count": result.Payload.DeviceCount,
			"features":     result.Payload.Features,
			"valid_until":  result.Payload.ValidUntil,
		})
		c.JSON(http.StatusOK, Response{Success: true, Message: result.Message})
		return
	}

	result, err := h.license.ActivateLicense(key, getSystemUUID(), h.getFormalSecretKey(), h.getPOCSecretKey(), h.initializeCameraModule)
	if err != nil {
		h.WriteSystemLog("warning", "license", "activate_license_failed", "license validation failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"machine_id":         getSystemUUID(),
			"reason":             "activation_failed",
			"error":              err.Error(),
		})
		h.WriteAuditFromContext(c, "activate_license", auditResource("license", key), "failed", map[string]interface{}{
			"license_key_prefix": truncateAuditLicenseKey(key),
			"machine_id":         getSystemUUID(),
			"reason":             "activation_failed",
			"error":              err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "license", "activate_license", "license activated", map[string]interface{}{
		"license_key_prefix": truncateAuditLicenseKey(key),
		"license_mode":       result.Payload.LicenseMode,
		"license_type":       result.LicenseType,
		"device_count":       result.Payload.DeviceCount,
		"camera_count":       result.Payload.CameraCount,
		"features":           result.Payload.Features,
		"valid_until":        result.Payload.ValidUntil,
		"is_permanent":       licensesvc.IsPermanentValidity(result.Payload.IssuedAt, result.Payload.ValidUntil),
		"reactivated":        result.Reactivated,
	})
	h.WriteAuditFromContext(c, "activate_license", auditResource("license", key), "success", map[string]interface{}{
		"license_key_prefix": truncateAuditLicenseKey(key),
		"license_mode":       result.Payload.LicenseMode,
		"license_type":       result.LicenseType,
		"device_count":       result.Payload.DeviceCount,
		"camera_count":       result.Payload.CameraCount,
		"features":           result.Payload.Features,
		"valid_until":        result.Payload.ValidUntil,
		"is_permanent":       licensesvc.IsPermanentValidity(result.Payload.IssuedAt, result.Payload.ValidUntil),
		"reactivated":        result.Reactivated,
	})
	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: result.Message,
		Data:    map[string]int64{"id": result.ID},
	})
}

func (h *Handler) GenerateLicenseKey(c *gin.Context) {
	var input GenerateLicenseKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid license request"})
		return
	}

	result, err := h.license.GenerateLicenseKey(licensemodule.GenerateInput{
		MachineID:   input.MachineID,
		LicenseMode: input.LicenseMode,
		LicenseType: input.LicenseType,
		DeviceCount: input.DeviceCount,
		CameraCount: input.CameraCount,
		Years:       input.Years,
		ValidUntil:  input.ValidUntil,
		Features:    input.Features,
		AlertOnly:   input.AlertOnly,
	}, h.getFormalSecretKey(), h.getPOCSecretKey())
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "license generated",
		Data:    result,
	})
}

func (h *Handler) ResetLicenseIdentity(c *gin.Context) {
	if err := h.license.ResetIdentity(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to reset license identity"})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Message: "license identity reset"})
}

func (h *Handler) ReissueLicense(c *gin.Context) {
	var input ReissueLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid reissue request"})
		return
	}

	result, err := h.license.ReissueLicense(licensemodule.ReissueInput{
		OldMachineID:  input.OldMachineID,
		OldLicenseKey: input.OldLicenseKey,
	}, getSystemUUID(), h.getSecretKey(), h.getPOCSecretKey())
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "license_expired") {
			status = http.StatusBadRequest
		}
		c.JSON(status, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "license reissued",
		Data:    result,
	})
}
