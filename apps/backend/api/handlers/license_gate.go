// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"net/http"
	"strings"
	"time"

	"management-server/services/license"

	"github.com/gin-gonic/gin"
)

const activeLicenseWindowSQL = "(valid_until IS NULL OR valid_until = '' OR CASE WHEN length(valid_until) <= 10 THEN datetime(valid_until || ' 23:59:59') ELSE datetime(valid_until) END >= datetime('now'))"

func hasFeature(features []string, target string) bool {
	for _, feature := range features {
		if strings.EqualFold(strings.TrimSpace(feature), target) {
			return true
		}
	}
	return false
}

func (h *Handler) getFormalSecretKey() []byte {
	return h.getSecretKey()
}

func (h *Handler) getLicensePublicKey() []byte {
	return license.RuntimePublicKey()
}

func (h *Handler) getPOCSecretKey() []byte {
	return license.DeriveKey("NMS-POC-LICENSE-v1.2.1-PoC")
}

func (h *Handler) isPoCEdition() bool {
	return h.license.IsPoCEdition()
}

func (h *Handler) getDefaultDeviceLimit() int {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.DefaultDeviceLimit()
}

func (h *Handler) activeLicensedDeviceCount() int {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.ActiveLicensedDeviceCount()
}

func (h *Handler) licenseFeatureEnabled(feature string) bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.LicenseFeatureEnabled(feature)
}

func (h *Handler) deviceManagementEnabled() bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.DeviceManagementEnabled()
}

func (h *Handler) shouldLockSession() bool {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.ShouldLockSession()
}

func (h *Handler) sessionLockReason() string {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	return h.license.LockReason()
}

func isLicenseLockAllowedPath(path string) bool {
	switch path {
	case "/api/v1/auth/logout",
		"/api/v1/auth/me",
		"/api/v1/auth/change-password",
		"/api/v1/license/status",
		"/api/v1/license/features",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/license/machine-id",
		"/api/v1/license/activate":
		return true
	default:
		return false
	}
}

func (h *Handler) EnforceLicenseLock() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.shouldLockSession() {
			c.Next()
			return
		}

		fullPath := c.FullPath()
		if fullPath == "" {
			fullPath = c.Request.URL.Path
		}
		if isLicenseLockAllowedPath(fullPath) {
			c.Next()
			return
		}

		reason := h.sessionLockReason()
		if reason == "" {
			reason = "license_required"
		}

		c.AbortWithStatusJSON(http.StatusLocked, Response{
			Success: false,
			Error:   "license_locked",
			Data: gin.H{
				"reason": reason,
			},
		})
	}
}

func (h *Handler) RequireDeviceManagement() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.deviceManagementEnabled() {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "device management license required",
		})
		c.Abort()
	}
}
