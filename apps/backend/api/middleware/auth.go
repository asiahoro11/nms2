package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthRequired(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			// Try to get from query parameter (for downloads)
			tokenString = c.Query("token")
			if tokenString == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
				c.Abort()
				return
			}
			// Query param token doesn't have "Bearer " prefix usually
		} else {
			if !strings.HasPrefix(tokenString, "Bearer ") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
				c.Abort()
				return
			}
			tokenString = strings.TrimPrefix(tokenString, "Bearer ")
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
			c.Set("role", claims["role"])
			c.Set("embed", claims["embed"])
			c.Set("views", claims["views"])
			c.Set("jti", claims["jti"])
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid claims"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check defaults
		if len(allowedOrigins) == 0 {
			allowedOrigins = []string{"*"}
		}

		for _, o := range allowedOrigins {
			if o == "*" {
				// Allow * (but careful with Credentials)
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
				break
			}
			if o == origin {

				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		// If no match found but we have an origin, and list didn't contain *, we don't set header
		// But for legacy support if list is empty we defaulted to * above.

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Pragma")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Logger middleware (structured with slog)
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		// Get user info if available in context
		username, _ := c.Get("username")
		role, _ := c.Get("role")

		logMsg := "Request"
		if len(c.Errors) > 0 {
			logMsg = "Request Error: " + c.Errors.String()
		}

		if raw != "" {
			path = path + "?" + redactRawQuery(raw)
		}

		slog.Info(logMsg,
			slog.Int("status", statusCode),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("ip", clientIP),
			slog.Duration("latency", latency),
			slog.Any("username", username),
			slog.Any("role", role),
		)
	}
}

func redactRawQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return raw
	}
	for key := range values {
		switch strings.ToLower(key) {
		case "token", "auth", "key", "password", "pass", "username", "user":
			values.Set(key, "<redacted>")
		}
	}
	return values.Encode()
}

func RequireEditor() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || (role != "editor" && role != "admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Editor access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
