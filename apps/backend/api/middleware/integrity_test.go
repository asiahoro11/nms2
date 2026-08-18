// Made by YTSworks
// YTS工作室製作
package middleware

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"management-server/services/integrity"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func lockedIntegrityGuard(t *testing.T) *integrity.Guard {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "nms.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	executable := filepath.Join(t.TempDir(), "server.bin")
	if err := os.WriteFile(executable, []byte("modified"), 0600); err != nil {
		t.Fatalf("write executable: %v", err)
	}
	expected := sha256.Sum256([]byte("trusted"))
	guard := integrity.New(db, integrity.Options{
		DatabasePath:             dbPath,
		ExecutablePath:           executable,
		ExpectedExecutableSHA256: hex.EncodeToString(expected[:]),
	})
	if err := guard.Check(); err != nil {
		t.Fatalf("lock guard: %v", err)
	}
	return guard
}

func TestIntegrityLockBlocksAPIAndLeavesStaticFilesAlone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(IntegrityLock(lockedIntegrityGuard(t)))
	router.GET("/api/v1/devices", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "frontend") })

	apiResponse := httptest.NewRecorder()
	router.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil))
	if apiResponse.Code != http.StatusLocked {
		t.Fatalf("API status = %d, want %d", apiResponse.Code, http.StatusLocked)
	}

	staticResponse := httptest.NewRecorder()
	router.ServeHTTP(staticResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if staticResponse.Code != http.StatusOK || staticResponse.Body.String() != "frontend" {
		t.Fatalf("static response = %d %q", staticResponse.Code, staticResponse.Body.String())
	}
}

func TestIntegrityStatusRemainsAvailableWhileLocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(IntegrityLock(lockedIntegrityGuard(t)))

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, integrityStatusPath, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, want 200", response.Code)
	}
	if body := response.Body.String(); body == "" || body == "{}" {
		t.Fatalf("empty status body: %q", body)
	}
}
