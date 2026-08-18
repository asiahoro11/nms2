// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"fmt"
	"management-server/services/dbencryption"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// ExportEncryptedBackup exports the current database as an encrypted backup file.
func (h *Handler) legacyExportEncryptedBackup(c *gin.Context) {
	var input struct {
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "backup", "export_encrypted_backup_invalid_request", "encrypted backup export request invalid", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "backup password must be at least 8 characters"})
		return
	}

	username, _ := c.Get("username")
	encService, err := dbencryption.NewService(input.Password)
	if err != nil {
		h.WriteSystemLog("error", "backup", "export_encrypted_backup_service_failed", "encrypted backup service init failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to initialize encrypted backup service"})
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("nms_backup_encrypted_%s.enc", timestamp))
	defer os.Remove(tempPath)

	dbPath := h.config.Database.Path
	if err := encService.CreateEncryptedBackup(dbPath, tempPath); err != nil {
		h.WriteSystemLog("error", "backup", "export_encrypted_backup_failed", "encrypted backup export failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to create encrypted backup: " + err.Error()})
		return
	}

	encryptedData, err := os.ReadFile(tempPath)
	if err != nil {
		h.WriteSystemLog("error", "backup", "export_encrypted_backup_read_failed", "encrypted backup file read failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to read encrypted backup file"})
		return
	}

	h.WriteSystemLog("notice", "backup", "export_encrypted_backup_success", "encrypted backup exported", map[string]interface{}{
		"size": len(encryptedData),
	})
	h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "success", map[string]interface{}{"size": len(encryptedData)})

	filename := fmt.Sprintf("nms_backup_encrypted_%s.enc", timestamp)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", len(encryptedData)))
	c.Data(http.StatusOK, "application/octet-stream", encryptedData)
}

// RestoreEncryptedBackup restores the database from an encrypted backup file.
func (h *Handler) legacyRestoreEncryptedBackup(c *gin.Context) {
	password := c.PostForm("password")
	if password == "" {
		h.WriteSystemLog("warning", "backup", "restore_encrypted_backup_missing_password", "encrypted backup restore missing password", nil)
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "password is required"})
		return
	}

	file, err := c.FormFile("backup_file")
	if err != nil {
		h.WriteSystemLog("warning", "backup", "restore_encrypted_backup_missing_file", "encrypted backup restore missing file", nil)
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "backup file is required"})
		return
	}

	username, _ := c.Get("username")
	tempEncrypted := filepath.Join(os.TempDir(), fmt.Sprintf("nms_restore_%d.enc", time.Now().Unix()))
	defer os.Remove(tempEncrypted)

	if err := c.SaveUploadedFile(file, tempEncrypted); err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_upload_failed", "encrypted backup upload failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": "upload_failed"})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to save uploaded backup file"})
		return
	}

	decService, err := dbencryption.NewService(password)
	if err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_service_failed", "encrypted backup restore service init failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": "decryption_service_init_failed"})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to initialize restore service"})
		return
	}

	if err := decService.ValidatePassword(tempEncrypted); err != nil {
		h.WriteSystemLog("warning", "backup", "restore_encrypted_backup_wrong_password", "encrypted backup restore password validation failed", map[string]interface{}{
			"file": file.Filename,
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": "wrong_password"})
		c.JSON(http.StatusUnauthorized, Response{Success: false, Error: "backup password is incorrect"})
		return
	}

	tempDecrypted := filepath.Join(os.TempDir(), fmt.Sprintf("nms_restore_%d.db", time.Now().Unix()))
	defer os.Remove(tempDecrypted)

	if err := decService.DecryptBackup(tempEncrypted, tempDecrypted); err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_decrypt_failed", "encrypted backup decrypt failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to decrypt backup: " + err.Error()})
		return
	}

	decryptedData, err := os.ReadFile(tempDecrypted)
	if err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_read_failed", "decrypted backup file read failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to read decrypted backup"})
		return
	}

	dbPath := h.config.Database.Path
	// #nosec G703 -- dbPath is the operator-controlled configured database path, never request input.
	if err := os.WriteFile(dbPath, decryptedData, 0600); err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_write_failed", "restored backup write failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to replace database with restored backup"})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_encrypted_backup_success", "encrypted backup restored", map[string]interface{}{
		"file": file.Filename,
	})
	h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "success", map[string]interface{}{"file": file.Filename})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "backup restored successfully, system will restart in 5 seconds",
	})

	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}()
}

// SetEncryptionPassword sets or rotates the backup encryption password.
func (h *Handler) legacySetEncryptionPassword(c *gin.Context) {
	var input struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
		OldPassword string `json:"old_password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "backup", "set_encryption_password_invalid_request", "backup encryption password request invalid", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "new password must be at least 8 characters"})
		return
	}

	username, _ := c.Get("username")
	dbPath := h.config.Database.Path

	encService, err := dbencryption.NewService(input.NewPassword)
	if err != nil {
		h.WriteSystemLog("error", "backup", "set_encryption_password_service_failed", "backup encryption password service init failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "set_encryption_password", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to initialize encryption service"})
		return
	}

	if input.OldPassword == "" {
		_ = encService
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(os.TempDir(), fmt.Sprintf("nms_pre_encrypt_%s.enc", timestamp))
	defer os.Remove(backupPath)

	if err := encService.CreateEncryptedBackup(dbPath, backupPath); err != nil {
		h.WriteSystemLog("error", "backup", "set_encryption_password_backup_failed", "backup encryption password apply failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "set_encryption_password", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to create encrypted backup with new password"})
		return
	}

	h.WriteSystemLog("notice", "backup", "set_encryption_password_success", "backup encryption password updated", nil)
	h.WriteAudit(username.(string), c.ClientIP(), "", "set_encryption_password", "system", "success", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "backup encryption password updated successfully",
	})
}
