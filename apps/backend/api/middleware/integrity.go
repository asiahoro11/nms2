// Made by YTSworks
// YTS工作室製作
package middleware

import (
	"net/http"
	"strings"

	"management-server/services/integrity"

	"github.com/gin-gonic/gin"
)

const integrityStatusPath = "/api/v1/system/integrity/status"

func IntegrityLock(guard *integrity.Guard) gin.HandlerFunc {
	return func(c *gin.Context) {
		if guard == nil {
			c.Next()
			return
		}
		if c.Request.URL.Path == integrityStatusPath {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"success": true,
				"data":    guard.Status(),
			})
			return
		}
		if guard.Locked() && strings.HasPrefix(c.Request.URL.Path, "/api/") {
			status := guard.Status()
			c.AbortWithStatusJSON(http.StatusLocked, gin.H{
				"success": false,
				"error":   "integrity_locked",
				"data": gin.H{
					"reason":    status.Reason,
					"locked_at": status.LockedAt,
				},
			})
			return
		}
		c.Next()
	}
}
