package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"management-server/config"

	"golang.org/x/crypto/bcrypt"
)

const (
	totpDigits           = 6
	totpPeriodSeconds    = 30
	totpAllowedWindow    = 1
	defaultRecoveryCount = 8
	defaultIssuerName    = "Management Server"
)

type TwoFactorService struct {
	db     *sql.DB
	config *config.Config
}

func NewTwoFactorService(db *sql.DB, cfg *config.Config) *TwoFactorService {
	return &TwoFactorService{db: db, config: cfg}
}

func (s *TwoFactorService) GlobalEnabled() bool {
	var value string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'global_2fa_enabled'").Scan(&value); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value), "true") || strings.TrimSpace(value) == "1"
}

func (s *TwoFactorService) Capabilities() TwoFactorCapabilities {
	return TwoFactorCapabilities{
		GlobalEnabled:         s.GlobalEnabled(),
		PrimaryMethod:         "totp",
		SupportsTOTP:          true,
		SupportsEmailOTP:      true,
		SupportsRecoveryCodes: true,
	}
}

func (s *TwoFactorService) Status(userID int) (TwoFactorStatus, error) {
	status := TwoFactorStatus{
		GlobalEnabled:         s.GlobalEnabled(),
		PrimaryMethod:         "totp",
		SupportsTOTP:          true,
		SupportsEmailOTP:      true,
		SupportsRecoveryCodes: true,
	}

	var enabled int
	var pending sql.NullString
	var generatedAt sql.NullString
	var preferred sql.NullString
	err := s.db.QueryRow(`
		SELECT
			COALESCE(totp_enabled, 0),
			COALESCE(pending_totp_secret_encrypted, ''),
			COALESCE(recovery_codes_generated_at, ''),
			COALESCE(preferred_method, 'totp')
		FROM user_twofactor_settings
		WHERE user_id = ?
	`, userID).Scan(&enabled, &pending, &generatedAt, &preferred)
	if err != nil {
		if err == sql.ErrNoRows {
			return status, nil
		}
		return status, err
	}

	status.Enabled = enabled != 0
	status.PendingEnrollment = strings.TrimSpace(pending.String) != ""
	if strings.TrimSpace(preferred.String) != "" {
		status.PrimaryMethod = strings.TrimSpace(preferred.String)
	}
	if generatedAt.Valid {
		status.RecoveryCodesGeneratedAt = strings.TrimSpace(generatedAt.String)
	}

	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM user_recovery_codes
		WHERE user_id = ? AND used_at IS NULL
	`, userID).Scan(&status.RecoveryCodesRemaining); err != nil {
		return status, err
	}

	return status, nil
}

func (s *TwoFactorService) BeginEnrollment(userID int, username string) (BeginTOTPEnrollmentResult, error) {
	var enabled int
	err := s.db.QueryRow(`
		SELECT COALESCE(totp_enabled, 0)
		FROM user_twofactor_settings
		WHERE user_id = ?
	`, userID).Scan(&enabled)
	if err != nil && err != sql.ErrNoRows {
		return BeginTOTPEnrollmentResult{}, err
	}
	if enabled != 0 {
		return BeginTOTPEnrollmentResult{}, &Error{Code: ErrCodeTwoFactorAlreadyEnabled}
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		return BeginTOTPEnrollmentResult{}, &Error{Code: ErrCodeTwoFactorSecretFailed, Err: err}
	}
	encrypted, err := s.encryptSecret(secret)
	if err != nil {
		return BeginTOTPEnrollmentResult{}, &Error{Code: ErrCodeTwoFactorSecretFailed, Err: err}
	}

	if _, err := s.db.Exec(`
		INSERT INTO user_twofactor_settings (
			user_id,
			pending_totp_secret_encrypted,
			totp_enabled,
			preferred_method,
			created_at,
			updated_at
		) VALUES (?, ?, 0, 'totp', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			pending_totp_secret_encrypted = excluded.pending_totp_secret_encrypted,
			updated_at = CURRENT_TIMESTAMP
	`, userID, encrypted); err != nil {
		return BeginTOTPEnrollmentResult{}, err
	}

	return BeginTOTPEnrollmentResult{
		Secret:          secret,
		Issuer:          defaultIssuerName,
		AccountName:     username,
		ProvisioningURI: buildProvisioningURI(defaultIssuerName, username, secret),
	}, nil
}

func (s *TwoFactorService) ConfirmEnrollment(userID int, code string) ([]string, error) {
	var pendingEncrypted sql.NullString
	err := s.db.QueryRow(`
		SELECT pending_totp_secret_encrypted
		FROM user_twofactor_settings
		WHERE user_id = ?
	`, userID).Scan(&pendingEncrypted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &Error{Code: ErrCodeTwoFactorNotConfigured}
		}
		return nil, err
	}
	if strings.TrimSpace(pendingEncrypted.String) == "" {
		return nil, &Error{Code: ErrCodeTwoFactorNotConfigured}
	}

	secret, err := s.decryptSecret(pendingEncrypted.String)
	if err != nil {
		return nil, &Error{Code: ErrCodeTwoFactorInvalid, Err: err}
	}
	if !verifyTOTP(secret, code, time.Now()) {
		return nil, &Error{Code: ErrCodeTwoFactorInvalid}
	}

	plainCodes, hashedCodes, err := generateRecoveryCodes(defaultRecoveryCount)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE user_twofactor_settings
		SET
			totp_secret_encrypted = pending_totp_secret_encrypted,
			pending_totp_secret_encrypted = '',
			totp_enabled = 1,
			preferred_method = 'totp',
			recovery_codes_generated_at = CURRENT_TIMESTAMP,
			last_verified_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, userID); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`DELETE FROM user_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	for _, hash := range hashedCodes {
		if _, err := tx.Exec(`
			INSERT INTO user_recovery_codes (user_id, code_hash, created_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
		`, userID, hash); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return plainCodes, nil
}

func (s *TwoFactorService) Disable(userID int, code string, method string) error {
	_, err := s.VerifyCurrentCode(userID, code, method)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_twofactor_settings WHERE user_id = ?`, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *TwoFactorService) RegenerateRecoveryCodes(userID int, code string, method string) ([]string, error) {
	_, err := s.VerifyCurrentCode(userID, code, method)
	if err != nil {
		return nil, err
	}

	plainCodes, hashedCodes, err := generateRecoveryCodes(defaultRecoveryCount)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_recovery_codes WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	for _, hash := range hashedCodes {
		if _, err := tx.Exec(`
			INSERT INTO user_recovery_codes (user_id, code_hash, created_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
		`, userID, hash); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(`
		UPDATE user_twofactor_settings
		SET recovery_codes_generated_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return plainCodes, nil
}

func (s *TwoFactorService) IsEnabledForUser(userID int) bool {
	var enabled int
	if err := s.db.QueryRow(`
		SELECT COALESCE(totp_enabled, 0)
		FROM user_twofactor_settings
		WHERE user_id = ?
	`, userID).Scan(&enabled); err != nil {
		return false
	}
	return enabled != 0
}

func (s *TwoFactorService) VerifyCurrentCode(userID int, code string, method string) (string, error) {
	normalizedMethod := strings.ToLower(strings.TrimSpace(method))
	if normalizedMethod == "" || normalizedMethod == "totp" {
		ok, err := s.verifyTOTPForUser(userID, code)
		if err != nil {
			return "", err
		}
		if ok {
			return "totp", nil
		}
		if normalizedMethod == "totp" {
			return "", &Error{Code: ErrCodeTwoFactorInvalid}
		}
	}

	if normalizedMethod == "" || normalizedMethod == "recovery_code" {
		ok, err := s.consumeRecoveryCode(userID, code)
		if err != nil {
			return "", err
		}
		if ok {
			return "recovery_code", nil
		}
	}

	return "", &Error{Code: ErrCodeTwoFactorInvalid}
}

func (s *TwoFactorService) verifyTOTPForUser(userID int, code string) (bool, error) {
	var encrypted sql.NullString
	err := s.db.QueryRow(`
		SELECT totp_secret_encrypted
		FROM user_twofactor_settings
		WHERE user_id = ? AND COALESCE(totp_enabled, 0) = 1
	`, userID).Scan(&encrypted)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, &Error{Code: ErrCodeTwoFactorNotConfigured}
		}
		return false, err
	}

	secret, err := s.decryptSecret(encrypted.String)
	if err != nil {
		return false, err
	}

	if !verifyTOTP(secret, code, time.Now()) {
		return false, nil
	}

	_, _ = s.db.Exec(`
		UPDATE user_twofactor_settings
		SET last_verified_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, userID)
	return true, nil
}

func (s *TwoFactorService) consumeRecoveryCode(userID int, code string) (bool, error) {
	rows, err := s.db.Query(`
		SELECT id, code_hash
		FROM user_recovery_codes
		WHERE user_id = ? AND used_at IS NULL
	`, userID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	normalized := normalizeRecoveryCode(code)
	var matchedID int64
	for rows.Next() {
		var id int64
		var hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return false, err
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(normalized)) == nil {
			matchedID = id
			break
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if matchedID == 0 {
		return false, nil
	}

	if _, err := s.db.Exec(`
		UPDATE user_recovery_codes
		SET used_at = CURRENT_TIMESTAMP
		WHERE id = ? AND used_at IS NULL
	`, matchedID); err != nil {
		return false, err
	}
	_, _ = s.db.Exec(`
		UPDATE user_twofactor_settings
		SET last_verified_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`, userID)
	return true, nil
}

func generateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

func buildProvisioningURI(issuer string, accountName string, secret string) string {
	label := url.PathEscape(issuer + ":" + accountName)
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", strconv.Itoa(totpDigits))
	params.Set("period", strconv.Itoa(totpPeriodSeconds))
	return "otpauth://totp/" + label + "?" + params.Encode()
}

func verifyTOTP(secret string, code string, now time.Time) bool {
	normalizedCode := normalizeOTPCode(code)
	for offset := -totpAllowedWindow; offset <= totpAllowedWindow; offset++ {
		counter := uint64((now.Unix() / totpPeriodSeconds) + int64(offset))
		candidate, err := generateTOTPCode(secret, counter)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(normalizedCode)) == 1 {
			return true
		}
	}
	return false
}

func generateTOTPCode(secret string, counter uint64) (string, error) {
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", err
	}
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)
	mac := hmac.New(sha1.New, decoded)
	if _, err := mac.Write(msg); err != nil {
		return "", err
	}
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	mod := 1
	for range totpDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, binaryCode%mod), nil
}

func generateRecoveryCodes(count int) ([]string, []string, error) {
	plain := make([]string, 0, count)
	hashes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		token, err := generateSecureToken(8)
		if err != nil {
			return nil, nil, err
		}
		code := formatRecoveryCode(token)
		hash, err := bcrypt.GenerateFromPassword([]byte(normalizeRecoveryCode(code)), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		plain = append(plain, code)
		hashes = append(hashes, string(hash))
	}
	return plain, hashes, nil
}

func formatRecoveryCode(token string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(token), "-", ""))
	if len(normalized) > 10 {
		normalized = normalized[:10]
	}
	if len(normalized) <= 5 {
		return normalized
	}
	return normalized[:5] + "-" + normalized[5:]
}

func normalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

func normalizeOTPCode(code string) string {
	replacer := strings.NewReplacer(" ", "", "-", "")
	return replacer.Replace(strings.TrimSpace(code))
}

func (s *TwoFactorService) encryptSecret(secret string) (string, error) {
	block, err := aes.NewCipher(s.secretKey())
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nonce, nonce, []byte(secret), nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func (s *TwoFactorService) decryptSecret(payload string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.secretKey())
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(decoded) < aead.NonceSize() {
		return "", fmt.Errorf("encrypted secret payload too short")
	}
	nonce := decoded[:aead.NonceSize()]
	ciphertext := decoded[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *TwoFactorService) secretKey() []byte {
	sum := sha256.Sum256([]byte(s.config.Security.JWTSecret))
	return sum[:]
}
