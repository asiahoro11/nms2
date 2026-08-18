package middleware

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	_ "modernc.org/sqlite"
)

func scopedToken(t *testing.T, secret, role, scope, jti, parentJTI string) string {
	t.Helper()
	claims := jwt.MapClaims{"user_id": 2, "username": "SuperAdmin", "role": role, "scope": scope, "jti": jti, "parent_jti": parentJTI, "iss": "management-server", "exp": time.Now().Add(time.Hour).Unix()}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRequireSuperAdminRejectsAdminAndAcceptsActiveScopedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, role TEXT, is_active INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE superadmin_sessions (jti TEXT PRIMARY KEY, parent_jti TEXT, parent_user_id INTEGER, revoked_at TEXT, expires_at TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users VALUES (1,'admin',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO superadmin_sessions VALUES ('super-jti','parent-jti',1,NULL,datetime('now','+10 minutes'))`); err != nil {
		t.Fatal(err)
	}
	secret := "test-secret-at-least-32-bytes-long"
	router := gin.New()
	router.Use(AuthRequired([]byte(secret)), RequireSuperAdmin(db))
	router.GET("/hidden", func(c *gin.Context) { c.Status(http.StatusOK) })

	admin := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hidden", nil)
	req.Header.Set("Authorization", "Bearer "+scopedToken(t, secret, "admin", "nms", "admin-jti", ""))
	router.ServeHTTP(admin, req)
	if admin.Code != http.StatusForbidden {
		t.Fatalf("admin status=%d", admin.Code)
	}

	super := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/hidden", nil)
	req.Header.Set("Authorization", "Bearer "+scopedToken(t, secret, "super_admin", "hidden_admin", "super-jti", "parent-jti"))
	router.ServeHTTP(super, req)
	if super.Code != http.StatusOK {
		t.Fatalf("SuperAdmin status=%d body=%s", super.Code, super.Body.String())
	}
}
