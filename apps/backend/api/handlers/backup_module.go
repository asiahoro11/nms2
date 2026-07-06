// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	backupmodule "management-server/modules/backup"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ExportBackup(c *gin.Context) {
	result, err := h.backup.ExportBackupArchive()
	if err != nil {
		message := "failed to export system backup"
		if backupmodule.Is(err, backupmodule.ErrBackupCheckpointIncomplete) {
			message = "backup export blocked because the WAL checkpoint is still incomplete"
		}
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: message})
		return
	}
	defer os.Remove(result.Path)

	readFile, err := os.Open(result.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to open generated backup archive"})
		return
	}
	defer readFile.Close()

	c.Header("Content-Disposition", "attachment; filename="+result.Filename)
	c.Header("Content-Type", result.ContentType)
	c.Header("Content-Transfer-Encoding", "binary")
	_, _ = io.Copy(c.Writer, readFile)
}

func (h *Handler) RestoreBackup(c *gin.Context) {
	file, err := c.FormFile("backup_file")
	if err != nil {
		h.WriteSystemLog("warning", "backup", "restore_backup_missing_file", "backup restore missing file", nil)
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module": "backup",
			"reason": "missing_file",
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "backup file is required"})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_backup_started", "backup restore started", map[string]interface{}{
		"file": file.Filename,
		"size": file.Size,
	})

	tempPath := filepath.Join("data", fmt.Sprintf("restore_upload_%d.zip", time.Now().UnixNano()))
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		h.WriteSystemLog("error", "backup", "restore_backup_save_failed", "backup restore upload save failed", map[string]interface{}{
			"file":  file.Filename,
			"size":  file.Size,
			"error": err.Error(),
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":        "backup",
			"reason":        "save_upload_failed",
			"resource_type": "system_backup",
			"resource_name": file.Filename,
			"error":         err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to store uploaded backup archive"})
		return
	}
	defer os.Remove(tempPath)

	summary, err := h.backup.PrepareRestore(tempPath)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to prepare backup restore"
		reason := "prepare_restore_failed"
		switch {
		case backupmodule.Is(err, backupmodule.ErrBackupInvalidArchive):
			status = http.StatusBadRequest
			message = "invalid backup archive"
			reason = "invalid_zip"
		case backupmodule.Is(err, backupmodule.ErrBackupMissingDatabase):
			status = http.StatusBadRequest
			message = "backup archive is missing nms.db"
			reason = "missing_database"
		}

		h.WriteSystemLog("warning", "backup", "restore_backup_prepare_failed", "backup restore preparation failed", map[string]interface{}{
			"file":             file.Filename,
			"size":             file.Size,
			"error":            err.Error(),
			"extracted_config": summary.ExtractedConfig,
			"uploads_count":    summary.ExtractedUploads,
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":           "backup",
			"reason":           reason,
			"resource_type":    "system_backup",
			"resource_name":    file.Filename,
			"error":            err.Error(),
			"extracted_config": summary.ExtractedConfig,
			"uploads_count":    summary.ExtractedUploads,
		})
		c.JSON(status, Response{Success: false, Error: message})
		return
	}

	if err := createRestoreScript(h.config.Database.Path); err != nil {
		h.WriteSystemLog("error", "backup", "restore_backup_prepare_failed", "backup restore restart script creation failed", map[string]interface{}{
			"file":             file.Filename,
			"error":            err.Error(),
			"extracted_config": summary.ExtractedConfig,
			"uploads_count":    summary.ExtractedUploads,
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":           "backup",
			"reason":           "create_restore_script_failed",
			"resource_type":    "system_backup",
			"resource_name":    file.Filename,
			"error":            err.Error(),
			"extracted_config": summary.ExtractedConfig,
			"uploads_count":    summary.ExtractedUploads,
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to prepare restore restart script"})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_backup_ready", "backup restore prepared; restart scheduled", map[string]interface{}{
		"file":             file.Filename,
		"extracted_config": summary.ExtractedConfig,
		"uploads_count":    summary.ExtractedUploads,
		"db_pending":       summary.HasDB,
	})
	h.WriteAuditFromContext(c, "restore_backup", "system_backup", "success", map[string]interface{}{
		"module":           "backup",
		"resource_type":    "system_backup",
		"resource_name":    file.Filename,
		"db_pending":       summary.HasDB,
		"extracted_config": summary.ExtractedConfig,
		"uploads_count":    summary.ExtractedUploads,
		"change_source":    "manual",
		"change_scope":     "restore_workflow",
	})

	c.JSON(http.StatusOK, Response{Success: true, Message: "restore prepared; the service will restart shortly"})

	go func() {
		time.Sleep(1 * time.Second)
		executeRestoreScript()
		os.Exit(0)
	}()
}

func (h *Handler) CheckRestoreReadiness(c *gin.Context) {
	readiness := h.backup.CheckRestoreReadiness()
	c.JSON(http.StatusOK, Response{
		Success: readiness.Ready,
		Data:    readiness,
	})
}

func (h *Handler) ExportEncryptedBackup(c *gin.Context) {
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

	result, err := h.backup.ExportEncryptedBackup(input.Password)
	if err != nil {
		username, _ := c.Get("username")
		h.WriteSystemLog("error", "backup", "export_encrypted_backup_failed", "encrypted backup export failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		status := http.StatusInternalServerError
		message := "failed to export encrypted backup"
		if errors.Is(err, backupmodule.ErrBackupPasswordTooShort) {
			status = http.StatusBadRequest
			message = "backup password must be at least 8 characters"
		}
		c.JSON(status, Response{Success: false, Error: message})
		return
	}
	defer os.Remove(result.Path)

	username, _ := c.Get("username")
	info, statErr := os.Stat(result.Path)
	if statErr == nil {
		h.WriteSystemLog("notice", "backup", "export_encrypted_backup_success", "encrypted backup exported", map[string]interface{}{
			"size": info.Size(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "export_encrypted_backup", "system", "success", map[string]interface{}{"size": info.Size()})
	}

	c.FileAttachment(result.Path, result.Filename)
}

func (h *Handler) RestoreEncryptedBackup(c *gin.Context) {
	password := c.PostForm("password")
	if strings.TrimSpace(password) == "" {
		h.WriteSystemLog("warning", "backup", "restore_encrypted_backup_missing_password", "encrypted backup restore missing password", nil)
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "backup password is required"})
		return
	}

	file, err := c.FormFile("backup_file")
	if err != nil {
		h.WriteSystemLog("warning", "backup", "restore_encrypted_backup_missing_file", "encrypted backup restore missing file", nil)
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "encrypted backup file is required"})
		return
	}

	username, _ := c.Get("username")
	tempEncrypted := filepath.Join(os.TempDir(), fmt.Sprintf("nms_restore_%d.enc", time.Now().UnixNano()))
	defer os.Remove(tempEncrypted)

	if err := c.SaveUploadedFile(file, tempEncrypted); err != nil {
		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_upload_failed", "encrypted backup upload failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": "upload_failed"})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "failed to store encrypted backup upload"})
		return
	}

	err = h.backup.RestoreEncryptedBackup(tempEncrypted, password)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to restore encrypted backup"
		switch {
		case errors.Is(err, backupmodule.ErrBackupPasswordRequired):
			status = http.StatusBadRequest
			message = "backup password is required"
		case errors.Is(err, backupmodule.ErrEncryptedBackupWrongPassword):
			status = http.StatusUnauthorized
			message = "backup password is incorrect"
		}

		h.WriteSystemLog("error", "backup", "restore_encrypted_backup_failed", "encrypted backup restore failed", map[string]interface{}{
			"file":  file.Filename,
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "failed", map[string]interface{}{"error": err.Error()})
		c.JSON(status, Response{Success: false, Error: message})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_encrypted_backup_success", "encrypted backup restored", map[string]interface{}{
		"file": file.Filename,
	})
	h.WriteAudit(username.(string), c.ClientIP(), "", "restore_encrypted_backup", "system", "success", map[string]interface{}{"file": file.Filename})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "encrypted backup restored; service will exit in 5 seconds",
	})

	go func() {
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}()
}

func (h *Handler) SetEncryptionPassword(c *gin.Context) {
	var input struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
		OldPassword string `json:"old_password"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteSystemLog("warning", "backup", "set_encryption_password_invalid_request", "backup encryption password request invalid", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "new backup password must be at least 8 characters"})
		return
	}

	username, _ := c.Get("username")
	if err := h.backup.RotateEncryptionPassword(input.NewPassword); err != nil {
		h.WriteSystemLog("error", "backup", "set_encryption_password_failed", "backup encryption password apply failed", map[string]interface{}{
			"error": err.Error(),
		})
		h.WriteAudit(username.(string), c.ClientIP(), "", "set_encryption_password", "system", "failed", map[string]interface{}{"error": err.Error()})
		status := http.StatusInternalServerError
		message := "failed to update backup encryption password"
		if errors.Is(err, backupmodule.ErrBackupPasswordTooShort) {
			status = http.StatusBadRequest
			message = "new backup password must be at least 8 characters"
		}
		c.JSON(status, Response{Success: false, Error: message})
		return
	}

	h.WriteSystemLog("notice", "backup", "set_encryption_password_success", "backup encryption password updated", nil)
	h.WriteAudit(username.(string), c.ClientIP(), "", "set_encryption_password", "system", "success", nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "backup encryption password updated",
	})
}
