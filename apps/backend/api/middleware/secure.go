// Made by YTSworks
// YTS工作室製作
package middleware

import (
	"database/sql"
	"encoding/json"
	"management-server/config"
	"strings"

	"github.com/gin-gonic/gin"
)

func Secure(cfg *config.Config, db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// HSTS - Force HTTPS
		if cfg.Security.EnableTLS {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		frameAncestors := frameAncestorsPolicy(cfg, db)
		if frameAncestors == "'self'" {
			// X-Frame-Options is kept only for same-origin mode. External iframe
			// integrations are controlled by CSP frame-ancestors below.
			c.Header("X-Frame-Options", "SAMEORIGIN")
		}

		// X-Content-Type-Options - Prevent MIME sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// X-XSS-Protection - Enable XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")

		// Content-Security-Policy - Restrict resources
		// Relaxed policy to allow WebRTC streaming, common CDNs, and go2rtc assets
		c.Header("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://d3js.org https://cdn.jsdelivr.net https://static.cloudflareinsights.com; script-src-elem 'self' 'unsafe-inline' 'unsafe-eval' https://d3js.org https://cdn.jsdelivr.net https://static.cloudflareinsights.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; style-src-elem 'self' 'unsafe-inline' https://fonts.googleapis.com https://cdn.jsdelivr.net; img-src 'self' data: https: blob: *; font-src 'self' data: https://fonts.gstatic.com; connect-src 'self' ws: wss: http: https: *; frame-src 'self' http: https:; frame-ancestors "+frameAncestors+"; manifest-src 'self' https://go2rtc.org;")

		// Referrer-Policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

func frameAncestorsPolicy(cfg *config.Config, db *sql.DB) string {
	if db != nil {
		if fromDB := frameAncestorsFromDB(db); fromDB != "" {
			return fromDB
		}
	}
	configured := []string{}
	if cfg != nil {
		configured = cfg.Security.FrameAncestors
	}
	return normalizeFrameAncestors(configured)
}

func frameAncestorsFromDB(db *sql.DB) string {
	var raw string
	if err := db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'integration_frame_ancestors'`).Scan(&raw); err != nil {
		return ""
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return ""
	}
	return normalizeFrameAncestors(items)
}

func normalizeFrameAncestors(items []string) string {
	if len(items) == 0 {
		return "'self'"
	}
	allowed := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if validFrameAncestor(item) {
			allowed = append(allowed, item)
		}
	}
	if len(allowed) == 0 {
		return "'self'"
	}
	for _, item := range allowed {
		if item == "'none'" {
			return "'none'"
		}
	}
	hasSelf := false
	for _, item := range allowed {
		if item == "'self'" {
			hasSelf = true
			break
		}
	}
	if !hasSelf {
		allowed = append([]string{"'self'"}, allowed...)
	}
	return strings.Join(allowed, " ")
}

func validFrameAncestor(item string) bool {
	switch item {
	case "'self'", "'none'", "http:", "https:":
		return true
	}
	if strings.HasPrefix(item, "https://") || strings.HasPrefix(item, "http://") {
		return !strings.ContainsAny(item, " \t\r\n")
	}
	return false
}
