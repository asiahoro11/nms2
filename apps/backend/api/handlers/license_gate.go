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
