package handlers

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"management-server/config"
	authmodule "management-server/modules/auth"
	"management-server/pkg/loginlimiter"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func setupLoginRateLimitTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE,
			password_hash TEXT,
			role TEXT,
			is_active BOOLEAN DEFAULT 1,
			force_change_password BOOLEAN DEFAULT 0,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("seed test db: %v", err)
	}

	cfg := &config.Config{
		Security: config.SecurityConfig{JWTSecret: "test-secret"},
	}
	h := &Handler{
		config:              cfg,
		db:                  db,
		auth:                authmodule.NewService(db, cfg),
		loginAccountLimiter: loginlimiter.New(3, time.Minute, time.Minute),
		loginIPLimiter:      loginlimiter.New(30, time.Minute, time.Minute),
	}

	router := gin.New()
	router.POST("/auth/login", h.Login)
	return router
}

func postLogin(router *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:50000"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestLoginLockedOutAfterRepeatedFailures(t *testing.T) {
	router := setupLoginRateLimitTest(t)

	for i := 0; i < 3; i++ {
		w := postLogin(router, `{"username":"ghost","password":"wrong"}`)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d (%s)", i+1, w.Code, w.Body.String())
		}
	}

	w := postLogin(router, `{"username":"ghost","password":"wrong"}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after lockout, got %d (%s)", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 response must include Retry-After header")
	}
}

func TestLoginLockoutIsScopedToUsername(t *testing.T) {
	router := setupLoginRateLimitTest(t)

	for i := 0; i < 3; i++ {
		postLogin(router, `{"username":"ghost","password":"wrong"}`)
	}

	// Another username from the same IP is still allowed (IP cap is higher).
	w := postLogin(router, `{"username":"someone-else","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for other username, got %d (%s)", w.Code, w.Body.String())
	}
}
