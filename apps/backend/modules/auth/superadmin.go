package auth

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const superAdminSessionTTL = 10 * time.Minute

func (s *Service) SuperAdminInitialized() bool {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE lower(username) = 'superadmin' AND role = 'super_admin' AND is_active = 1`).Scan(&count)
	return count == 1
}

func (s *Service) BeginSuperAdminLogin(input SuperAdminLoginInput, parentUserID int, parentJTI string, parentExpiry time.Time) (SuperAdminLoginResult, error) {
	if strings.TrimSpace(parentJTI) == "" || parentExpiry.Before(time.Now()) {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeUnauthorized}
	}
	var parentRole string
	if err := s.db.QueryRow(`SELECT role FROM users WHERE id = ? AND is_active = 1`, parentUserID).Scan(&parentRole); err != nil || parentRole != "admin" {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}
	user, passwordHash, _, err := s.lookupLoginUserByUsername(input.Username)
	if err != nil || !strings.EqualFold(user.Username, "SuperAdmin") || user.Role != "super_admin" {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeInvalidCredentials}
	}
	if s.twoFactor.IsEnabledForUser(user.ID) {
		challenge, err := generateSecureToken(32)
		if err != nil {
			return SuperAdminLoginResult{}, err
		}
		expires := minTime(time.Now().Add(5*time.Minute), parentExpiry)
		if _, err := s.db.Exec(`DELETE FROM superadmin_auth_challenges WHERE parent_jti = ? OR expires_at <= CURRENT_TIMESTAMP`, parentJTI); err != nil {
			return SuperAdminLoginResult{}, err
		}
		if _, err := s.db.Exec(`INSERT INTO superadmin_auth_challenges (challenge_token, superadmin_user_id, parent_user_id, parent_jti, expires_at) VALUES (?, ?, ?, ?, ?)`, challenge, user.ID, parentUserID, parentJTI, expires.UTC().Format("2006-01-02 15:04:05")); err != nil {
			return SuperAdminLoginResult{}, err
		}
		return SuperAdminLoginResult{RequiresTwoFactor: true, TwoFactorMethod: "totp", ChallengeToken: challenge, ExpiresAt: expires.Unix()}, nil
	}
	return s.issueSuperAdminToken(user.ID, parentUserID, parentJTI, parentExpiry)
}

func (s *Service) VerifySuperAdminLogin(input SuperAdminVerifyInput, parentUserID int, parentJTI string, parentExpiry time.Time) (SuperAdminLoginResult, error) {
	var superID, storedParentID int
	var storedParentJTI, expiresRaw string
	err := s.db.QueryRow(`SELECT superadmin_user_id, parent_user_id, parent_jti, expires_at FROM superadmin_auth_challenges WHERE challenge_token = ? AND consumed_at IS NULL`, strings.TrimSpace(input.ChallengeToken)).Scan(&superID, &storedParentID, &storedParentJTI, &expiresRaw)
	if err != nil {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeInvalid, Err: err}
	}
	if storedParentID != parentUserID || storedParentJTI != parentJTI {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeInvalid}
	}
	expires, err := parseDateTime(expiresRaw)
	if err != nil || time.Now().After(expires) {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeExpired, Err: err}
	}
	if _, err := s.twoFactor.VerifyCurrentCode(superID, input.Code, input.Method); err != nil {
		return SuperAdminLoginResult{}, err
	}
	result, err := s.db.Exec(`UPDATE superadmin_auth_challenges SET consumed_at = CURRENT_TIMESTAMP WHERE challenge_token = ? AND consumed_at IS NULL`, input.ChallengeToken)
	if err != nil {
		return SuperAdminLoginResult{}, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeInvalid}
	}
	return s.issueSuperAdminToken(superID, parentUserID, parentJTI, parentExpiry)
}

func (s *Service) issueSuperAdminToken(superID, parentUserID int, parentJTI string, parentExpiry time.Time) (SuperAdminLoginResult, error) {
	now := time.Now()
	expires := minTime(now.Add(superAdminSessionTTL), parentExpiry)
	jti, err := generateSecureToken(32)
	if err != nil {
		return SuperAdminLoginResult{}, err
	}
	claims := jwt.MapClaims{"user_id": superID, "username": "SuperAdmin", "role": "super_admin", "scope": "hidden_admin", "session_type": "superadmin", "parent_user_id": parentUserID, "parent_jti": parentJTI, "jti": jti, "iss": "management-server", "iat": now.Unix(), "exp": expires.Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.config.Security.JWTSecret))
	if err != nil {
		return SuperAdminLoginResult{}, &Error{Code: ErrCodeTokenSignFailed, Err: err}
	}
	if _, err := s.db.Exec(`INSERT INTO superadmin_sessions (jti, superadmin_user_id, parent_user_id, parent_jti, expires_at) VALUES (?, ?, ?, ?, ?)`, jti, superID, parentUserID, parentJTI, expires.UTC().Format("2006-01-02 15:04:05")); err != nil {
		return SuperAdminLoginResult{}, err
	}
	return SuperAdminLoginResult{Token: token, ExpiresAt: expires.Unix()}, nil
}

func (s *Service) RevokeSuperAdminSessions(parentJTI string) error {
	if strings.TrimSpace(parentJTI) == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE superadmin_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE parent_jti = ? AND revoked_at IS NULL`, parentJTI)
	return err
}

func (s *Service) RecoverSuperAdminPassword(input SuperAdminRecoveryInput, parentUserID int, parentJTI string) error {
	var parentRole string
	if strings.TrimSpace(parentJTI) == "" || s.db.QueryRow(`SELECT role FROM users WHERE id = ? AND is_active = 1`, parentUserID).Scan(&parentRole) != nil || parentRole != "admin" {
		return &Error{Code: ErrCodeUnauthorized}
	}
	var superID int
	if err := s.db.QueryRow(`SELECT id FROM users WHERE lower(username) = 'superadmin' AND role = 'super_admin' AND is_active = 1`).Scan(&superID); err != nil {
		return &Error{Code: ErrCodeUserNotFound, Err: err}
	}
	if _, err := s.twoFactor.VerifyCurrentCode(superID, input.Code, input.Method); err != nil {
		return err
	}
	if err := validateSuperAdminPassword(input.NewPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 12)
	if err != nil {
		return &Error{Code: ErrCodeHashFailed, Err: err}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE users SET password_hash = ?, last_password_change = CURRENT_TIMESTAMP, force_change_password = 0 WHERE id = ?`, string(hash), superID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE superadmin_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE revoked_at IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM superadmin_auth_challenges`); err != nil {
		return err
	}
	return tx.Commit()
}

func validateSuperAdminPassword(password string) error {
	if len([]rune(password)) < 12 {
		return fmt.Errorf("password must contain at least 12 characters")
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
		digit = digit || unicode.IsDigit(r)
		symbol = symbol || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r))
	}
	if !upper || !lower || !digit || !symbol {
		return fmt.Errorf("password complexity requirements not met")
	}
	if strings.Contains(strings.ToLower(password), "superadmin") {
		return fmt.Errorf("password must not contain account name")
	}
	return nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
