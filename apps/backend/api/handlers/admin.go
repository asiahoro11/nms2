// Made by YTSworks
// YTS工作室製作
package handlers

import (
	licensemodule "management-server/modules/license"
	licensesvc "management-server/services/license"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type CreateUserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"`
}

type UpdateUserInput struct {
	Password *string `json:"password"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

// legacyGetUsers remains only as a compatibility reference while active routes use module wrappers.
func (h *Handler) legacyGetUsers(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, username, role, is_active, created_at FROM users ORDER BY id
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.IsActive, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: users})
}

// legacyCreateUser remains only as a compatibility reference while active routes use module wrappers.
func (h *Handler) legacyCreateUser(c *gin.Context) {
	var input CreateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if input.Role == "" {
		input.Role = "viewer"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	result, err := h.db.Exec(`
		INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)
	`, input.Username, string(hash), input.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

// legacyUpdateUser remains only as a compatibility reference while active routes use module wrappers.
func (h *Handler) legacyUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var input UpdateUserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
			return
		}
		h.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), id)
	}

	if input.Role != nil {
		h.db.Exec("UPDATE users SET role = ? WHERE id = ?", *input.Role, id)
	}

	if input.IsActive != nil {
		h.db.Exec("UPDATE users SET is_active = ? WHERE id = ?", *input.IsActive, id)
	}

	c.JSON(http.StatusOK, Response{Success: true, Message: "User updated"})
}

// legacyDeleteUser remains only as a compatibility reference while active routes use module wrappers.
func (h *Handler) legacyDeleteUser(c *gin.Context) {
	id := c.Param("id")

	var adminCount int
	h.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND id != ?", id).Scan(&adminCount)
	if adminCount == 0 {
		var role string
		h.db.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&role)
		if role == "admin" {
			c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Cannot delete the last admin user"})
			return
		}
	}

	_, err := h.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Message: "User deleted"})
}

// GetLicenses returns the legacy admin license inventory view without changing its route.
func (h *Handler) GetLicenses(c *gin.Context) {
	data, err := h.license.ListAdminLicenses(getSystemUUID())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// CreateLicense stores a legacy admin license record without changing its route.
func (h *Handler) CreateLicense(c *gin.Context) {
	var input licensemodule.CreateLegacyLicenseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	id, err := h.license.CreateLegacyLicense(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": id}})
}

// GetSystemInfo returns runtime metadata without changing its route.
func (h *Handler) GetSystemInfo(c *gin.Context) {
	h.ensureLicenseRuntimeFresh(30 * time.Second)
	locked := h.license.ShouldLockSession()
	info := h.admin.GetSystemInfo(h.license.DisplayVersion(), h.config.System.Name, h.startTime, locked, h.license.LockReason())
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    info,
	})
}

// getSystemUUID returns the local machine identity used by legacy license flows.
func getSystemUUID() string {
	return licensesvc.SystemMachineID()
}
