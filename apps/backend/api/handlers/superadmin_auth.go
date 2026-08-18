package handlers

import (
	"net/http"
	"time"

	authmodule "management-server/modules/auth"

	"github.com/gin-gonic/gin"
)

func parentSession(c *gin.Context) (int, string, time.Time, bool) {
	var userID int
	rawUserID, exists := c.Get("user_id")
	if !exists {
		return 0, "", time.Time{}, false
	}
	switch value := rawUserID.(type) {
	case float64:
		userID = int(value)
	case int:
		userID = value
	}
	jti := c.GetString("jti")
	var expiry time.Time
	switch value, _ := c.Get("expires_at"); typed := value.(type) {
	case float64:
		expiry = time.Unix(int64(typed), 0)
	case int64:
		expiry = time.Unix(typed, 0)
	}
	return userID, jti, expiry, userID > 0 && jti != "" && expiry.After(time.Now())
}

func (h *Handler) GetSuperAdminStatus(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Success: true, Data: gin.H{"initialized": h.auth.SuperAdminInitialized()}})
}

func (h *Handler) BeginSuperAdminLogin(c *gin.Context) {
	var input authmodule.SuperAdminLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid request"})
		return
	}
	if !h.checkLoginRateLimit(c, "superadmin|"+input.Username) {
		return
	}
	parentID, parentJTI, parentExpiry, ok := parentSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "fresh Admin login required"})
		return
	}
	result, err := h.auth.BeginSuperAdminLogin(input, parentID, parentJTI, parentExpiry)
	if err != nil {
		h.recordLoginFailure(c, "superadmin|"+input.Username)
		h.WriteAudit(c.GetString("username"), c.ClientIP(), h.auditSourceMAC(c), "superadmin_login", "hidden_admin", "failed", map[string]interface{}{"parent_user_id": parentID})
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "invalid SuperAdmin credentials"})
		return
	}
	h.resetLoginFailures(c, "superadmin|"+input.Username)
	status := "success"
	if result.RequiresTwoFactor {
		status = "pending_totp"
	}
	h.WriteAudit("SuperAdmin", c.ClientIP(), h.auditSourceMAC(c), "superadmin_login", "hidden_admin", status, map[string]interface{}{"parent_user_id": parentID, "parent_username": c.GetString("username"), "totp_required": result.RequiresTwoFactor})
	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) VerifySuperAdminLogin(c *gin.Context) {
	var input authmodule.SuperAdminVerifyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid request"})
		return
	}
	parentID, parentJTI, parentExpiry, ok := parentSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "fresh Admin login required"})
		return
	}
	result, err := h.auth.VerifySuperAdminLogin(input, parentID, parentJTI, parentExpiry)
	if err != nil {
		h.recordLoginFailure(c, "superadmin|totp")
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "invalid or expired verification code"})
		return
	}
	h.resetLoginFailures(c, "superadmin|totp")
	h.WriteAudit("SuperAdmin", c.ClientIP(), h.auditSourceMAC(c), "superadmin_2fa", "hidden_admin", "success", map[string]interface{}{"parent_user_id": parentID, "parent_username": c.GetString("username")})
	c.JSON(http.StatusOK, Response{Success: true, Data: result})
}

func (h *Handler) EndSuperAdminSession(c *gin.Context) {
	_ = h.auth.RevokeSuperAdminSessions(c.GetString("jti"))
	c.JSON(http.StatusOK, Response{Success: true})
}

func (h *Handler) RecoverSuperAdminPassword(c *gin.Context) {
	var input authmodule.SuperAdminRecoveryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid recovery request"})
		return
	}
	if !h.checkLoginRateLimit(c, "superadmin|recovery") {
		return
	}
	parentID, parentJTI, _, ok := parentSession(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "fresh Admin login required"})
		return
	}
	if err := h.auth.RecoverSuperAdminPassword(input, parentID, parentJTI); err != nil {
		h.recordLoginFailure(c, "superadmin|recovery")
		h.WriteAudit("SuperAdmin", c.ClientIP(), h.auditSourceMAC(c), "superadmin_totp_recovery", "hidden_admin", "failed", map[string]interface{}{"parent_user_id": parentID})
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "recovery failed"})
		return
	}
	h.resetLoginFailures(c, "superadmin|recovery")
	h.WriteAudit("SuperAdmin", c.ClientIP(), h.auditSourceMAC(c), "superadmin_totp_recovery", "hidden_admin", "success", map[string]interface{}{"parent_user_id": parentID, "parent_username": c.GetString("username")})
	c.JSON(http.StatusOK, Response{Success: true, Message: "SuperAdmin password reset; all hidden sessions were revoked"})
}

func (h *Handler) ChangeSuperAdminPassword(c *gin.Context) {
	var input authmodule.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "invalid password request"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "unauthorized"})
		return
	}
	if _, err := h.auth.ChangePassword(userID, input); err != nil {
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "password change failed"})
		return
	}
	_ = h.auth.RevokeSuperAdminSessions(c.GetString("parent_jti"))
	h.WriteAudit("SuperAdmin", c.ClientIP(), h.auditSourceMAC(c), "superadmin_password_change", "hidden_admin", "success", hiddenSessionAudit(c))
	c.JSON(http.StatusOK, Response{Success: true, Message: "SuperAdmin password changed; re-authentication required"})
}

func hiddenSessionAudit(c *gin.Context) map[string]interface{} {
	parentJTI := c.GetString("parent_jti")
	if len(parentJTI) > 8 {
		parentJTI = parentJTI[:8]
	}
	parentUserID, _ := c.Get("parent_user_id")
	return map[string]interface{}{
		"actor": "SuperAdmin", "scope": "hidden_admin",
		"parent_user_id": parentUserID, "parent_session_prefix": parentJTI,
	}
}
