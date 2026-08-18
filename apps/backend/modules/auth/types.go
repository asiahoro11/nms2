// Made by YTSworks
// YTS工作室製作
package auth

import "errors"

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at,omitempty"`
}

type LoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResult struct {
	Token                 string `json:"token"`
	User                  User   `json:"user"`
	ExpiresAt             int64  `json:"expires_at"`
	RequirePasswordChange bool   `json:"require_password_change"`
	TwoFactorMethod       string `json:"two_factor_method,omitempty"`
	RequiresTwoFactor     bool   `json:"requires_two_factor,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
}

type VerifyTwoFactorInput struct {
	Username       string `json:"username,omitempty"`
	Code           string `json:"code" binding:"required"`
	Method         string `json:"method,omitempty"`
	ChallengeToken string `json:"challenge_token,omitempty"`
}

type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type SuperAdminLoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SuperAdminVerifyInput struct {
	ChallengeToken string `json:"challenge_token" binding:"required"`
	Code           string `json:"code" binding:"required"`
	Method         string `json:"method,omitempty"`
}

type SuperAdminRecoveryInput struct {
	Code        string `json:"code" binding:"required"`
	Method      string `json:"method,omitempty"`
	NewPassword string `json:"new_password" binding:"required,min=12"`
}

type SuperAdminLoginResult struct {
	Token             string `json:"token,omitempty"`
	ExpiresAt         int64  `json:"expires_at"`
	RequiresTwoFactor bool   `json:"requires_two_factor,omitempty"`
	TwoFactorMethod   string `json:"two_factor_method,omitempty"`
	ChallengeToken    string `json:"challenge_token,omitempty"`
}

type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordInput struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type BeginTOTPEnrollmentInput struct {
	Password string `json:"password" binding:"required"`
}

type BeginTOTPEnrollmentResult struct {
	Secret          string `json:"secret"`
	Issuer          string `json:"issuer"`
	AccountName     string `json:"account_name"`
	ProvisioningURI string `json:"provisioning_uri"`
}

type ConfirmTOTPEnrollmentInput struct {
	Code string `json:"code" binding:"required"`
}

type RecoveryCodesResult struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

type DisableTwoFactorInput struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Method   string `json:"method,omitempty"`
}

type RegenerateRecoveryCodesInput struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Method   string `json:"method,omitempty"`
}

type TwoFactorCapabilities struct {
	GlobalEnabled         bool   `json:"global_enabled"`
	PrimaryMethod         string `json:"primary_method"`
	SupportsTOTP          bool   `json:"supports_totp"`
	SupportsEmailOTP      bool   `json:"supports_email_otp"`
	SupportsRecoveryCodes bool   `json:"supports_recovery_codes"`
}

type TwoFactorStatus struct {
	GlobalEnabled            bool   `json:"global_enabled"`
	Enabled                  bool   `json:"enabled"`
	PendingEnrollment        bool   `json:"pending_enrollment"`
	PrimaryMethod            string `json:"primary_method"`
	SupportsTOTP             bool   `json:"supports_totp"`
	SupportsEmailOTP         bool   `json:"supports_email_otp"`
	SupportsRecoveryCodes    bool   `json:"supports_recovery_codes"`
	RecoveryCodesRemaining   int    `json:"recovery_codes_remaining"`
	RecoveryCodesGeneratedAt string `json:"recovery_codes_generated_at,omitempty"`
}

type ErrorCode string

const (
	ErrCodeInvalidInput       ErrorCode = "invalid_input"
	ErrCodeUnauthorized       ErrorCode = "unauthorized"
	ErrCodeUserNotFound       ErrorCode = "user_not_found"
	ErrCodeAccountDisabled    ErrorCode = "account_disabled"
	ErrCodeInvalidCredentials ErrorCode = "invalid_credentials" // #nosec G101 -- public error identifier, not a credential.
	ErrCodeTokenSignFailed    ErrorCode = "token_sign_failed"
	ErrCodeHashFailed         ErrorCode = "hash_failed"
	ErrCodeUpdateFailed       ErrorCode = "update_failed"
	ErrCodeMissingEmail       ErrorCode = "missing_email"
	ErrCodeMissingMailer      ErrorCode = "missing_mailer"
	// #nosec G101 -- public API error identifier, not an authentication token value.
	ErrCodeResetTokenInvalid ErrorCode = "reset_token_invalid"
	// #nosec G101 -- public API error identifier, not an authentication token value.
	ErrCodeResetTokenExpired ErrorCode = "reset_token_expired"
	// #nosec G101 -- public API error identifier, not a credential or secret.
	ErrCodeTwoFactorDisabled         ErrorCode = "two_factor_disabled"
	ErrCodeTwoFactorInvalid          ErrorCode = "two_factor_invalid"
	ErrCodeTwoFactorExpired          ErrorCode = "two_factor_expired"
	ErrCodeTwoFactorAlreadyEnabled   ErrorCode = "two_factor_already_enabled"
	ErrCodeTwoFactorNotConfigured    ErrorCode = "two_factor_not_configured"
	ErrCodeTwoFactorSecretFailed     ErrorCode = "two_factor_secret_failed" // #nosec G101 -- public error identifier, not a secret.
	ErrCodeTwoFactorChallengeInvalid ErrorCode = "two_factor_challenge_invalid"
	ErrCodeTwoFactorChallengeExpired ErrorCode = "two_factor_challenge_expired"
)

type Error struct {
	Code ErrorCode
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsErrorCode(err error, code ErrorCode) bool {
	var target *Error
	return errors.As(err, &target) && target.Code == code
}
