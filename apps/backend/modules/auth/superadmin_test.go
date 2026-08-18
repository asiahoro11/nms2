package auth

import (
	"database/sql"
	"testing"
	"time"

	"management-server/config"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func superAdminTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT UNIQUE, password_hash TEXT, role TEXT, is_active INTEGER, force_change_password INTEGER, created_at TEXT)`,
		`CREATE TABLE user_twofactor_settings (user_id INTEGER PRIMARY KEY, totp_enabled INTEGER DEFAULT 0)`,
		`CREATE TABLE superadmin_auth_challenges (challenge_token TEXT PRIMARY KEY, superadmin_user_id INTEGER, parent_user_id INTEGER, parent_jti TEXT, expires_at TEXT, consumed_at TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE superadmin_sessions (jti TEXT PRIMARY KEY, superadmin_user_id INTEGER, parent_user_id INTEGER, parent_jti TEXT, expires_at TEXT, revoked_at TEXT, created_at TEXT DEFAULT CURRENT_TIMESTAMP)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("Admin-Test-Password!1"), bcrypt.MinCost)
	superHash, _ := bcrypt.GenerateFromPassword([]byte("Super-Test-Password!1"), bcrypt.MinCost)
	if _, err := db.Exec(`INSERT INTO users VALUES (1,'admin',?,'admin',1,0,''),(2,'SuperAdmin',?,'super_admin',1,0,'')`, string(adminHash), string(superHash)); err != nil {
		t.Fatal(err)
	}
	return NewService(db, &config.Config{Security: config.SecurityConfig{JWTSecret: "test-secret-at-least-32-bytes-long"}}), db
}

func TestNormalLoginRejectsSuperAdmin(t *testing.T) {
	service, _ := superAdminTestService(t)
	if _, err := service.Login(LoginInput{Username: "SuperAdmin", Password: "Super-Test-Password!1"}); !IsErrorCode(err, ErrCodeInvalidCredentials) {
		t.Fatalf("normal login accepted SuperAdmin: %v", err)
	}
}

func TestSuperAdminSessionBoundToAdminSession(t *testing.T) {
	service, db := superAdminTestService(t)
	result, err := service.BeginSuperAdminLogin(SuperAdminLoginInput{Username: "SuperAdmin", Password: "Super-Test-Password!1"}, 1, "parent-jti", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.RequiresTwoFactor {
		t.Fatalf("unexpected result: %+v", result)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM superadmin_sessions WHERE parent_jti='parent-jti' AND revoked_at IS NULL`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("session not persisted: count=%d err=%v", count, err)
	}
	if err := service.RevokeSuperAdminSessions("parent-jti"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM superadmin_sessions WHERE parent_jti='parent-jti' AND revoked_at IS NULL`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("session not revoked: count=%d err=%v", count, err)
	}
}
