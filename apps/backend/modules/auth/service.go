package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"management-server/config"
	"management-server/services/alert"

	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db        *sql.DB
	config    *config.Config
	twoFactor *TwoFactorService
}

func NewService(db *sql.DB, cfg *config.Config) *Service {
	return &Service{
		db:        db,
		config:    cfg,
		twoFactor: NewTwoFactorService(db, cfg),
	}
}

func (s *Service) Login(input LoginInput) (LoginResult, error) {
	user, passwordHash, forceChangePassword, err := s.lookupLoginUserByUsername(input.Username)
	if err != nil {
		return LoginResult{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return LoginResult{}, &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}

	if s.twoFactor.IsEnabledForUser(user.ID) {
		challengeToken, expiresAt, err := s.createLoginChallenge(user.ID, "totp")
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{
			User:                  user,
			ExpiresAt:             expiresAt.Unix(),
			RequirePasswordChange: forceChangePassword,
			RequiresTwoFactor:     true,
			TwoFactorMethod:       "totp",
			ChallengeToken:        challengeToken,
		}, nil
	}

	if s.twoFactor.GlobalEnabled() {
		challengeToken, expiresAt, err := s.prepareEmailOTPChallenge(user)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{
			User:                  user,
			ExpiresAt:             expiresAt.Unix(),
			RequirePasswordChange: forceChangePassword,
			RequiresTwoFactor:     true,
			TwoFactorMethod:       "email_otp",
			ChallengeToken:        challengeToken,
		}, nil
	}

	tokenString, expiresAt, err := s.issueJWT(user)
	if err != nil {
		return LoginResult{}, &Error{Code: ErrCodeTokenSignFailed, Err: err}
	}

	return LoginResult{
		Token:                 tokenString,
		User:                  user,
		ExpiresAt:             expiresAt.Unix(),
		RequirePasswordChange: forceChangePassword,
	}, nil
}

func (s *Service) VerifyTwoFactor(input VerifyTwoFactorInput) (LoginResult, error) {
	if strings.TrimSpace(input.ChallengeToken) != "" {
		return s.verifyLoginChallenge(input)
	}

	user, _, forceChangePassword, err := s.lookupLoginUserByUsername(input.Username)
	if err != nil {
		return LoginResult{}, err
	}

	if !s.twoFactor.GlobalEnabled() {
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorDisabled}
	}

	var code sql.NullString
	var expiresAt sql.NullString
	if err := s.db.QueryRow(`
		SELECT two_fa_code, two_fa_expires_at
		FROM users
		WHERE id = ?
	`, user.ID).Scan(&code, &expiresAt); err != nil {
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorInvalid, Err: err}
	}

	if !code.Valid || strings.TrimSpace(code.String) == "" {
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorInvalid}
	}
	if strings.TrimSpace(code.String) != strings.TrimSpace(input.Code) {
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorInvalid}
	}

	if expiresAt.Valid && strings.TrimSpace(expiresAt.String) != "" {
		expiry, err := parseDateTime(expiresAt.String)
		if err == nil && time.Now().After(expiry) {
			return LoginResult{}, &Error{Code: ErrCodeTwoFactorExpired}
		}
	}

	_, _ = s.db.Exec("UPDATE users SET two_fa_code = NULL, two_fa_expires_at = NULL WHERE id = ?", user.ID)

	tokenString, sessionExpiry, err := s.issueJWT(user)
	if err != nil {
		return LoginResult{}, &Error{Code: ErrCodeTokenSignFailed, Err: err}
	}

	return LoginResult{
		Token:                 tokenString,
		User:                  user,
		ExpiresAt:             sessionExpiry.Unix(),
		RequirePasswordChange: forceChangePassword,
		TwoFactorMethod:       "email_otp",
	}, nil
}

func (s *Service) GetCurrentUser(userID any) (User, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return User{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	var user User
	err = s.db.QueryRow(`
		SELECT id, username, role, is_active, COALESCE(created_at, '')
		FROM users
		WHERE id = ?
	`, normalizedID).Scan(&user.ID, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, &Error{Code: ErrCodeUserNotFound, Err: err}
		}
		return User{}, err
	}

	return user, nil
}

func (s *Service) ForgotPassword(input ForgotPasswordInput, baseURL string) error {
	var userID int
	var username string
	var email sql.NullString

	err := s.db.QueryRow(`
		SELECT id, username, email
		FROM users
		WHERE lower(email) = lower(?) AND is_active = 1
	`, strings.TrimSpace(input.Email)).Scan(&userID, &username, &email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if !email.Valid || strings.TrimSpace(email.String) == "" {
		return &Error{Code: ErrCodeMissingEmail}
	}

	mailConfig, err := s.loadAuthMailConfig()
	if err != nil {
		return err
	}

	resetToken, err := generateSecureToken(32)
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(30 * time.Minute).UTC().Format("2006-01-02 15:04:05")
	if _, err := s.db.Exec("DELETE FROM password_reset_tokens WHERE user_id = ?", userID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		INSERT INTO password_reset_tokens (user_id, token, expires_at)
		VALUES (?, ?, ?)
	`, userID, resetToken, expiresAt); err != nil {
		return err
	}

	resetURL := strings.TrimRight(baseURL, "/") + "/reset-password.html?token=" + resetToken
	subject := "管理系統密碼重設通知"
	body := fmt.Sprintf("您好 %s，\n\n請使用以下連結重設密碼。此連結將在 30 分鐘後失效：\n%s\n\n如果這不是您的操作，請忽略此信件。\n", username, resetURL)

	if err := alert.SendDirectEmail(mailConfig, email.String, subject, body); err != nil {
		return err
	}

	return nil
}

func (s *Service) ResetPassword(input ResetPasswordInput) error {
	var userID int
	var expiresAt string

	err := s.db.QueryRow(`
		SELECT user_id, expires_at
		FROM password_reset_tokens
		WHERE token = ?
	`, strings.TrimSpace(input.Token)).Scan(&userID, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return &Error{Code: ErrCodeResetTokenInvalid, Err: err}
		}
		return err
	}

	expiry, err := parseDateTime(expiresAt)
	if err == nil && time.Now().After(expiry) {
		return &Error{Code: ErrCodeResetTokenExpired}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return &Error{Code: ErrCodeHashFailed, Err: err}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE users
		SET password_hash = ?, force_change_password = 0, last_password_change = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(newHash), userID); err != nil {
		return &Error{Code: ErrCodeUpdateFailed, Err: err}
	}

	if _, err := tx.Exec("DELETE FROM password_reset_tokens WHERE user_id = ?", userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Service) ChangePassword(userID any, input ChangePasswordInput) (User, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return User{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	user, passwordHash, err := s.lookupUserCredentialsByID(normalizedID)
	if err != nil {
		return User{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.OldPassword)); err != nil {
		return User{}, &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return User{}, &Error{Code: ErrCodeHashFailed, Err: err}
	}

	if _, err := s.db.Exec(`
		UPDATE users
		SET password_hash = ?, force_change_password = 0, last_password_change = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(newHash), normalizedID); err != nil {
		return User{}, &Error{Code: ErrCodeUpdateFailed, Err: err}
	}

	return user, nil
}

func (s *Service) GetTwoFactorStatus(userID any) (TwoFactorStatus, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return TwoFactorStatus{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}
	return s.twoFactor.Status(normalizedID)
}

func (s *Service) BeginTOTPEnrollment(userID any, input BeginTOTPEnrollmentInput) (BeginTOTPEnrollmentResult, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return BeginTOTPEnrollmentResult{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	user, passwordHash, err := s.lookupUserCredentialsByID(normalizedID)
	if err != nil {
		return BeginTOTPEnrollmentResult{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return BeginTOTPEnrollmentResult{}, &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}

	return s.twoFactor.BeginEnrollment(user.ID, user.Username)
}

func (s *Service) ConfirmTOTPEnrollment(userID any, input ConfirmTOTPEnrollmentInput) (RecoveryCodesResult, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return RecoveryCodesResult{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	codes, err := s.twoFactor.ConfirmEnrollment(normalizedID, input.Code)
	if err != nil {
		return RecoveryCodesResult{}, err
	}
	return RecoveryCodesResult{RecoveryCodes: codes}, nil
}

func (s *Service) DisableTwoFactor(userID any, input DisableTwoFactorInput) error {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	_, passwordHash, err := s.lookupUserCredentialsByID(normalizedID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}

	return s.twoFactor.Disable(normalizedID, input.Code, input.Method)
}

func (s *Service) RegenerateRecoveryCodes(userID any, input RegenerateRecoveryCodesInput) (RecoveryCodesResult, error) {
	normalizedID, err := normalizeUserID(userID)
	if err != nil {
		return RecoveryCodesResult{}, &Error{Code: ErrCodeUnauthorized, Err: err}
	}

	_, passwordHash, err := s.lookupUserCredentialsByID(normalizedID)
	if err != nil {
		return RecoveryCodesResult{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		return RecoveryCodesResult{}, &Error{Code: ErrCodeInvalidCredentials, Err: err}
	}

	codes, err := s.twoFactor.RegenerateRecoveryCodes(normalizedID, input.Code, input.Method)
	if err != nil {
		return RecoveryCodesResult{}, err
	}
	return RecoveryCodesResult{RecoveryCodes: codes}, nil
}

func (s *Service) TwoFactorCapabilities() TwoFactorCapabilities {
	return s.twoFactor.Capabilities()
}

func (s *Service) verifyLoginChallenge(input VerifyTwoFactorInput) (LoginResult, error) {
	var userID int
	var method string
	var expiresAt string
	err := s.db.QueryRow(`
		SELECT user_id, method, expires_at
		FROM auth_login_challenges
		WHERE challenge_token = ? AND consumed_at IS NULL
	`, strings.TrimSpace(input.ChallengeToken)).Scan(&userID, &method, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return LoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeInvalid, Err: err}
		}
		return LoginResult{}, err
	}

	expiry, err := parseDateTime(expiresAt)
	if err == nil && time.Now().After(expiry) {
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeExpired}
	}

	user, _, err := s.lookupUserCredentialsByID(userID)
	if err != nil {
		return LoginResult{}, err
	}
	forceChangePassword, err := s.lookupForcePasswordChange(userID)
	if err != nil {
		return LoginResult{}, err
	}

	actualMethod := strings.TrimSpace(method)
	switch actualMethod {
	case "totp":
		actualMethod, err = s.twoFactor.VerifyCurrentCode(userID, input.Code, input.Method)
		if err != nil {
			return LoginResult{}, err
		}
	case "email_otp":
		if err := s.verifyLegacyEmailOTP(userID, input.Code); err != nil {
			return LoginResult{}, err
		}
	default:
		return LoginResult{}, &Error{Code: ErrCodeTwoFactorChallengeInvalid}
	}

	if _, err := s.db.Exec(`
		UPDATE auth_login_challenges
		SET consumed_at = CURRENT_TIMESTAMP
		WHERE challenge_token = ? AND consumed_at IS NULL
	`, strings.TrimSpace(input.ChallengeToken)); err != nil {
		return LoginResult{}, err
	}

	tokenString, sessionExpiry, err := s.issueJWT(user)
	if err != nil {
		return LoginResult{}, &Error{Code: ErrCodeTokenSignFailed, Err: err}
	}

	return LoginResult{
		Token:                 tokenString,
		User:                  user,
		ExpiresAt:             sessionExpiry.Unix(),
		RequirePasswordChange: forceChangePassword,
		TwoFactorMethod:       actualMethod,
	}, nil
}

func (s *Service) issueJWT(user User) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"iss":      "management-server",
		"iat":      time.Now().Unix(),
		"exp":      expiresAt.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.Security.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

func (s *Service) lookupLoginUserByUsername(username string) (User, string, bool, error) {
	var user User
	var passwordHash string
	var forceChangePassword bool

	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, is_active, COALESCE(force_change_password, 0), COALESCE(created_at, '')
		FROM users
		WHERE username = ?
	`, username).Scan(
		&user.ID,
		&user.Username,
		&passwordHash,
		&user.Role,
		&user.IsActive,
		&forceChangePassword,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, "", false, &Error{Code: ErrCodeUserNotFound, Err: err}
		}
		return User{}, "", false, &Error{Code: ErrCodeUnauthorized, Err: err}
	}
	if !user.IsActive {
		return User{}, "", false, &Error{Code: ErrCodeAccountDisabled}
	}
	return user, passwordHash, forceChangePassword, nil
}

func (s *Service) lookupUserCredentialsByID(userID int) (User, string, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRow(`
		SELECT id, username, role, is_active, COALESCE(created_at, ''), password_hash
		FROM users
		WHERE id = ?
	`, userID).Scan(&user.ID, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return User{}, "", &Error{Code: ErrCodeUserNotFound, Err: err}
		}
		return User{}, "", &Error{Code: ErrCodeUnauthorized, Err: err}
	}
	if !user.IsActive {
		return User{}, "", &Error{Code: ErrCodeAccountDisabled}
	}
	return user, passwordHash, nil
}

func (s *Service) lookupForcePasswordChange(userID int) (bool, error) {
	var force int
	if err := s.db.QueryRow(`
		SELECT COALESCE(force_change_password, 0)
		FROM users
		WHERE id = ?
	`, userID).Scan(&force); err != nil {
		if err == sql.ErrNoRows {
			return false, &Error{Code: ErrCodeUserNotFound, Err: err}
		}
		return false, err
	}
	return force != 0, nil
}

func (s *Service) loadAuthMailConfig() (string, error) {
	var configJSON string
	if err := s.db.QueryRow(`
		SELECT config_json
		FROM alert_settings
		WHERE alert_type = 'email'
	`).Scan(&configJSON); err != nil {
		if err == sql.ErrNoRows {
			return "", &Error{Code: ErrCodeMissingMailer, Err: err}
		}
		return "", err
	}
	if strings.TrimSpace(configJSON) == "" || strings.TrimSpace(configJSON) == "{}" {
		return "", &Error{Code: ErrCodeMissingMailer}
	}
	return configJSON, nil
}

func (s *Service) createLoginChallenge(userID int, method string) (string, time.Time, error) {
	challengeToken, err := generateSecureToken(24)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(5 * time.Minute).UTC()

	if _, err := s.db.Exec(`DELETE FROM auth_login_challenges WHERE user_id = ? AND consumed_at IS NULL`, userID); err != nil {
		return "", time.Time{}, err
	}

	if _, err := s.db.Exec(`
		INSERT INTO auth_login_challenges (challenge_token, user_id, method, expires_at)
		VALUES (?, ?, ?, ?)
	`, challengeToken, userID, method, expiresAt.Format("2006-01-02 15:04:05")); err != nil {
		return "", time.Time{}, err
	}
	return challengeToken, expiresAt, nil
}

func (s *Service) prepareEmailOTPChallenge(user User) (string, time.Time, error) {
	var email sql.NullString
	if err := s.db.QueryRow(`SELECT email FROM users WHERE id = ?`, user.ID).Scan(&email); err != nil {
		if err == sql.ErrNoRows {
			return "", time.Time{}, &Error{Code: ErrCodeUserNotFound, Err: err}
		}
		return "", time.Time{}, err
	}
	if !email.Valid || strings.TrimSpace(email.String) == "" {
		return "", time.Time{}, &Error{Code: ErrCodeMissingEmail}
	}

	mailConfig, err := s.loadAuthMailConfig()
	if err != nil {
		return "", time.Time{}, err
	}

	code, err := generateNumericOTP(6)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().Add(5 * time.Minute).UTC()
	if _, err := s.db.Exec(`
		UPDATE users
		SET two_fa_code = ?, two_fa_expires_at = ?
		WHERE id = ?
	`, code, expiresAt.Format("2006-01-02 15:04:05"), user.ID); err != nil {
		return "", time.Time{}, err
	}

	subject := "管理系統登入驗證碼"
	body := fmt.Sprintf("您好 %s，\n\n您的登入驗證碼為：%s\n\n此驗證碼將在 5 分鐘後失效。\n", user.Username, code)
	if err := alert.SendDirectEmail(mailConfig, email.String, subject, body); err != nil {
		return "", time.Time{}, err
	}

	challengeToken, _, err := s.createLoginChallenge(user.ID, "email_otp")
	if err != nil {
		return "", time.Time{}, err
	}
	return challengeToken, expiresAt, nil
}

func (s *Service) verifyLegacyEmailOTP(userID int, code string) error {
	var stored sql.NullString
	var expiresAt sql.NullString
	if err := s.db.QueryRow(`
		SELECT two_fa_code, two_fa_expires_at
		FROM users
		WHERE id = ?
	`, userID).Scan(&stored, &expiresAt); err != nil {
		return &Error{Code: ErrCodeTwoFactorInvalid, Err: err}
	}
	if !stored.Valid || strings.TrimSpace(stored.String) == "" {
		return &Error{Code: ErrCodeTwoFactorInvalid}
	}
	if strings.TrimSpace(stored.String) != normalizeOTPCode(code) {
		return &Error{Code: ErrCodeTwoFactorInvalid}
	}
	if expiresAt.Valid && strings.TrimSpace(expiresAt.String) != "" {
		expiry, err := parseDateTime(expiresAt.String)
		if err == nil && time.Now().After(expiry) {
			return &Error{Code: ErrCodeTwoFactorExpired}
		}
	}
	_, _ = s.db.Exec("UPDATE users SET two_fa_code = NULL, two_fa_expires_at = NULL WHERE id = ?", userID)
	return nil
}

func generateSecureToken(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateNumericOTP(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid otp length")
	}
	const digits = "0123456789"
	buf := make([]byte, length)
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = digits[int(random[i])%len(digits)]
	}
	return string(buf), nil
}

func parseDateTime(v string) (time.Time, error) {
	value := strings.TrimSpace(v)
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339,
		time.RFC3339Nano,
	}
	normalized := strings.ReplaceAll(value, "T", " ")
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		}
		if ts, err := time.Parse(layout, normalized); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime format: %s", v)
}

func normalizeUserID(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		id, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unsupported user id type %T", value)
	}
}
