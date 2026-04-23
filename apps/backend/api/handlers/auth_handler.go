package handlers

import (
	"net/http"

	authmodule "management-server/modules/auth"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Verify2FA(c *gin.Context) {
	var input authmodule.VerifyTwoFactorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供有效的驗證碼"})
		return
	}

	result, err := h.auth.VerifyTwoFactor(input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorDisabled):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "二階段驗證尚未啟用"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorChallengeExpired):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "登入驗證已過期，請重新登入"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorChallengeInvalid):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "登入驗證無效，請重新登入"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorExpired):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "驗證碼已過期"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorInvalid), authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorNotConfigured):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "驗證碼錯誤"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "二階段驗證失敗"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "login_2fa", "user login success via 2fa", map[string]interface{}{
		"user_id":           result.User.ID,
		"username":          result.User.Username,
		"role":              result.User.Role,
		"two_factor_method": result.TwoFactorMethod,
	})
	h.WriteAudit(result.User.Username, c.ClientIP(), h.auditSourceMAC(c), "verify_2fa", "auth", "success", map[string]interface{}{
		"user_id":           result.User.ID,
		"username":          result.User.Username,
		"role":              result.User.Role,
		"two_factor_method": result.TwoFactorMethod,
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "二階段驗證成功", Data: result})
}

func (h *Handler) GetTwoFactorStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	status, err := h.auth.GetTwoFactorStatus(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法取得二階段驗證狀態"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: status})
}

func (h *Handler) BeginTOTPEnrollment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	var input authmodule.BeginTOTPEnrollmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供目前密碼"})
		return
	}

	result, err := h.auth.BeginTOTPEnrollment(userID, input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeInvalidCredentials):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "目前密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorAlreadyEnabled):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "二階段驗證已啟用"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法開始設定二階段驗證"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "2fa_enrollment_started", "2fa enrollment started", map[string]interface{}{
		"username": h.auditUsername(c),
		"method":   "totp",
	})
	h.WriteAuditFromContext(c, "begin_2fa_enrollment", "auth", "success", map[string]interface{}{
		"username": h.auditUsername(c),
		"method":   "totp",
	})

	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) ConfirmTOTPEnrollment(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	var input authmodule.ConfirmTOTPEnrollmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供驗證碼"})
		return
	}

	result, err := h.auth.ConfirmTOTPEnrollment(userID, input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorNotConfigured):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "尚未開始設定二階段驗證"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorInvalid):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "驗證碼錯誤"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法完成二階段驗證設定"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "2fa_enrollment_confirmed", "2fa enrollment confirmed", map[string]interface{}{
		"username":            h.auditUsername(c),
		"method":              "totp",
		"recovery_code_count": len(result.RecoveryCodes),
	})
	h.WriteAuditFromContext(c, "confirm_2fa_enrollment", "auth", "success", map[string]interface{}{
		"username":            h.auditUsername(c),
		"method":              "totp",
		"recovery_code_count": len(result.RecoveryCodes),
	})

	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) DisableTwoFactor(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	var input authmodule.DisableTwoFactorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供目前密碼與驗證碼"})
		return
	}

	if err := h.auth.DisableTwoFactor(userID, input); err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeInvalidCredentials):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "目前密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorNotConfigured):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "二階段驗證尚未啟用"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorInvalid):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "驗證碼錯誤"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法停用二階段驗證"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "2fa_disabled", "2fa disabled", map[string]interface{}{
		"username": h.auditUsername(c),
	})
	h.WriteAuditFromContext(c, "disable_2fa", "auth", "success", map[string]interface{}{
		"username": h.auditUsername(c),
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "二階段驗證已停用"})
}

func (h *Handler) RegenerateRecoveryCodes(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	var input authmodule.RegenerateRecoveryCodesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供目前密碼與驗證碼"})
		return
	}

	result, err := h.auth.RegenerateRecoveryCodes(userID, input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeInvalidCredentials):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "目前密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorNotConfigured):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "二階段驗證尚未啟用"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTwoFactorInvalid):
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "驗證碼錯誤"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法重產備援碼"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "recovery_codes_regenerated", "2fa recovery codes regenerated", map[string]interface{}{
		"username":            h.auditUsername(c),
		"recovery_code_count": len(result.RecoveryCodes),
	})
	h.WriteAuditFromContext(c, "regenerate_recovery_codes", "auth", "success", map[string]interface{}{
		"username":            h.auditUsername(c),
		"recovery_code_count": len(result.RecoveryCodes),
	})

	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) Login(c *gin.Context) {
	var input authmodule.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供帳號與密碼"})
		return
	}

	result, err := h.auth.Login(input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeUserNotFound):
			h.WriteSystemLog("warning", "auth", "login_failed", "login failed: user not found", map[string]interface{}{
				"username": input.Username,
				"reason":   "user_not_found",
			})
			h.WriteAudit(input.Username, c.ClientIP(), h.auditSourceMAC(c), "login_failed", "auth", "failed", map[string]interface{}{
				"username": input.Username,
				"reason":   "user_not_found",
			})
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "帳號或密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeAccountDisabled):
			h.WriteSystemLog("warning", "auth", "login_failed", "login failed: account disabled", map[string]interface{}{
				"username": input.Username,
				"reason":   "account_disabled",
			})
			h.WriteAudit(input.Username, c.ClientIP(), h.auditSourceMAC(c), "login_failed", "auth", "failed", map[string]interface{}{
				"username": input.Username,
				"reason":   "account_disabled",
			})
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "帳號已停用"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeInvalidCredentials):
			h.WriteSystemLog("warning", "auth", "login_failed", "login failed: wrong password", map[string]interface{}{
				"username": input.Username,
				"reason":   "wrong_password",
			})
			h.WriteAudit(input.Username, c.ClientIP(), h.auditSourceMAC(c), "login_failed", "auth", "failed", map[string]interface{}{
				"username": input.Username,
				"reason":   "wrong_password",
			})
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "帳號或密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeTokenSignFailed):
			h.WriteSystemLog("error", "auth", "login_failed", "login failed: token signing error", map[string]interface{}{
				"username": input.Username,
				"reason":   "token_sign_failed",
			})
			h.WriteAudit(input.Username, c.ClientIP(), h.auditSourceMAC(c), "login_failed", "auth", "failed", map[string]interface{}{
				"username": input.Username,
				"reason":   "token_sign_failed",
			})
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "登入失敗，系統無法簽發憑證"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "登入失敗"})
		}
		return
	}

	if result.RequiresTwoFactor {
		h.WriteSystemLog("notice", "auth", "login_2fa_challenge", "user login requires second factor", map[string]interface{}{
			"user_id":           result.User.ID,
			"username":          result.User.Username,
			"role":              result.User.Role,
			"two_factor_method": result.TwoFactorMethod,
		})
		h.WriteAudit(result.User.Username, c.ClientIP(), h.auditSourceMAC(c), "login_2fa_challenge", "auth", "pending", map[string]interface{}{
			"user_id":           result.User.ID,
			"username":          result.User.Username,
			"role":              result.User.Role,
			"two_factor_method": result.TwoFactorMethod,
		})
		c.JSON(http.StatusOK, Response{
			Success: true,
			Message: "需要二階段驗證",
			Data:    result,
		})
		return
	}

	h.WriteSystemLog("notice", "auth", "login", "user login success", map[string]interface{}{
		"user_id":                 result.User.ID,
		"username":                result.User.Username,
		"role":                    result.User.Role,
		"require_password_change": result.RequirePasswordChange,
	})
	h.WriteAudit(result.User.Username, c.ClientIP(), h.auditSourceMAC(c), "login", "auth", "success", map[string]interface{}{
		"user_id":                 result.User.ID,
		"username":                result.User.Username,
		"role":                    result.User.Role,
		"require_password_change": result.RequirePasswordChange,
	})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "登入成功",
		Data:    result,
	})
}

func (h *Handler) ForgotPasswordRequest(c *gin.Context) {
	var input authmodule.ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請輸入有效的 Email"})
		return
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := scheme + "://" + c.Request.Host

	err := h.auth.ForgotPassword(input, baseURL)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeMissingMailer):
			h.WriteSystemLog("warning", "auth", "forgot_password_failed", "forgot password requested without configured mail transport", map[string]interface{}{
				"email":  input.Email,
				"reason": "missing_mailer",
			})
			c.JSON(http.StatusServiceUnavailable, Response{Success: false, Error: "尚未設定郵件服務"})
			return
		case authmodule.IsErrorCode(err, authmodule.ErrCodeMissingEmail):
			h.WriteSystemLog("warning", "auth", "forgot_password_failed", "forgot password requested for account without email", map[string]interface{}{
				"email":  input.Email,
				"reason": "missing_email",
			})
			c.JSON(http.StatusOK, Response{Success: true, Message: "若帳號存在，系統將寄送重設郵件"})
			return
		default:
			h.WriteSystemLog("error", "auth", "forgot_password_failed", "forgot password request failed", map[string]interface{}{
				"email": input.Email,
				"error": err.Error(),
			})
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法寄送重設密碼郵件"})
			return
		}
	}

	h.WriteSystemLog("notice", "auth", "forgot_password_requested", "password reset email requested", map[string]interface{}{
		"email": input.Email,
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "若帳號存在，系統將寄送重設郵件"})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var input authmodule.ResetPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供有效的重設 Token 與新密碼"})
		return
	}

	err := h.auth.ResetPassword(input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeResetTokenInvalid):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "重設連結無效"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeResetTokenExpired):
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "重設連結已過期"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeHashFailed), authmodule.IsErrorCode(err, authmodule.ErrCodeUpdateFailed):
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法重設密碼"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法重設密碼"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "password_reset", "password reset success", map[string]interface{}{
		"token_present": true,
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "密碼已重設，請重新登入"})
}

func (h *Handler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	user, err := h.auth.GetCurrentUser(userID)
	if err != nil {
		if authmodule.IsErrorCode(err, authmodule.ErrCodeUserNotFound) {
			c.JSON(http.StatusNotFound, Response{Success: false, Error: "找不到使用者"})
			return
		}
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: user})
}

func (h *Handler) Logout(c *gin.Context) {
	h.WriteSystemLog("notice", "auth", "logout", "user logout", map[string]interface{}{
		"username": h.auditUsername(c),
	})
	h.WriteAuditFromContext(c, "logout", "auth", "success", map[string]interface{}{
		"username": h.auditUsername(c),
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "已登出"})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		return
	}

	var input authmodule.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "請提供目前密碼與新密碼"})
		return
	}

	user, err := h.auth.ChangePassword(userID, input)
	if err != nil {
		switch {
		case authmodule.IsErrorCode(err, authmodule.ErrCodeUserNotFound):
			h.WriteSystemLog("warning", "auth", "change_password_failed", "change password failed: user not found", map[string]interface{}{
				"reason": "user_not_found",
			})
			h.WriteAuditFromContext(c, "change_password", "auth", "failed", map[string]interface{}{
				"reason": "user_not_found",
			})
			c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "尚未登入"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeInvalidCredentials):
			h.WriteSystemLog("warning", "auth", "change_password_failed", "change password failed: wrong old password", map[string]interface{}{
				"reason": "wrong_old_password",
			})
			h.WriteAuditFromContext(c, "change_password", "auth", "failed", map[string]interface{}{
				"reason": "wrong_old_password",
			})
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "目前密碼錯誤"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeHashFailed):
			h.WriteSystemLog("error", "auth", "change_password_failed", "change password failed: hash error", map[string]interface{}{
				"reason": "hash_failed",
			})
			h.WriteAuditFromContext(c, "change_password", "auth", "failed", map[string]interface{}{
				"reason": "hash_failed",
			})
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "系統無法處理新密碼"})
		case authmodule.IsErrorCode(err, authmodule.ErrCodeUpdateFailed):
			h.WriteSystemLog("error", "auth", "change_password_failed", "change password failed: update error", map[string]interface{}{
				"reason": "update_failed",
			})
			h.WriteAuditFromContext(c, "change_password", "auth", "failed", map[string]interface{}{
				"reason": "update_failed",
			})
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法更新密碼"})
		default:
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "無法更新密碼"})
		}
		return
	}

	h.WriteSystemLog("notice", "auth", "change_password", "password changed", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
	h.WriteAuditFromContext(c, "change_password", "auth", "success", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "密碼已更新"})
}
