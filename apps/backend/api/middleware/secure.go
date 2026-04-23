package middleware

import (
	"management-server/config"

	"github.com/gin-gonic/gin"
)

func Secure(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// HSTS - Force HTTPS
		if cfg.Security.EnableTLS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// X-Frame-Options - Allow same-origin iframes (needed for go2rtc stream.html via /go2rtc/ proxy)
		c.Header("X-Frame-Options", "SAMEORIGIN")

		// X-Content-Type-Options - Prevent MIME sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection - Enable XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")

		// Content-Security-Policy - Restrict resources
		// Relaxed policy to allow WebRTC streaming, common CDNs, and go2rtc assets
		c.Header("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://d3js.org https://cdn.jsdelivr.net https://static.cloudflareinsights.com; script-src-elem 'self' 'unsafe-inline' 'unsafe-eval' https://d3js.org https://cdn.jsdelivr.net https://static.cloudflareinsights.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; style-src-elem 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; img-src 'self' data: https: blob: *; font-src 'self' data: https://fonts.gstatic.com; connect-src 'self' ws: wss: http: https: *; frame-src 'self' http: https:; frame-ancestors 'self'; manifest-src 'self' https://go2rtc.org;")

		// Referrer-Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}
