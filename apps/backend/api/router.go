package api

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"management-server/api/handlers"
	"management-server/api/middleware"
	"management-server/config"
	"management-server/services/snmp"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, db *sql.DB, collector *snmp.Collector, assets embed.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.MaxMultipartMemory = 256 << 20 // 256 MiB
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.Security.AllowedOrigins))
	r.Use(middleware.Logger())
	r.Use(middleware.Secure(cfg))

	// 建�??��???
	h := handlers.New(cfg, db, collector)
	h.StartCameraHealthLoop()
	h.StartLicenseHealthLoop()

	// --- ?��?檔�??��? ---
	// ?��?�?1: 檢查?��??��?下是?��??�實�?frontend 資�?�?(?�發??
	// ?��?�?2: 使用?��?資�? (?�產?��?)

	// ?��?上傳路�? - ?��?使用?��??��???data
	dataPath := "./data"
	if _, err := os.Stat("./data"); err != nil {
		// ?��??��?沒�? data，檢?�父?��?
		if _, err := os.Stat("../data"); err == nil {
			dataPath = "../data"
		}
	}
	r.Static("/uploads", filepath.Join(dataPath, "uploads"))

	log.Println("[Router] Operating in EMBEDDED mode (Production)")

	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// 使用 Sub FS 以簡?�路徑�?�?index.html 位於?�目??
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	// ?��?資�??��??�輯：�?使用 FileFromFS 以�??��? 301
	serveAsData := func(c *gin.Context, fsPath string, contentType string) {
		data, err := fs.ReadFile(staticFS, fsPath)
		if err != nil {
			// ?�找不到檔�?且�???index.html，�??�退??index.html (SPA)
			if fsPath != "index.html" {
				indexData, err := fs.ReadFile(staticFS, "index.html")
				if err == nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
					return
				}
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "Resource not found"})
			return
		}

		if contentType == "" {
			// ?��??�測 Content-Type
			switch {
			case strings.HasSuffix(fsPath, ".html"):
				contentType = "text/html; charset=utf-8"
			case strings.HasSuffix(fsPath, ".css"):
				contentType = "text/css"
			case strings.HasSuffix(fsPath, ".js"):
				contentType = "application/javascript"
			case strings.HasSuffix(fsPath, ".json"):
				contentType = "application/json"
			case strings.HasSuffix(fsPath, ".png"):
				contentType = "image/png"
			case strings.HasSuffix(fsPath, ".jpg") || strings.HasSuffix(fsPath, ".jpeg"):
				contentType = "image/jpeg"
			case strings.HasSuffix(fsPath, ".ico"):
				contentType = "image/x-icon"
			case strings.HasSuffix(fsPath, ".svg"):
				contentType = "image/svg+xml"
			default:
				contentType = "application/octet-stream"
			}
		}
		c.Data(http.StatusOK, contentType, data)
	}

	// ?�能路由：�??��??��?端�?源�? SPA 跳�?
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 請�?不�?走到?�裡
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
			return
		}

		// 清�?路�?並移??/static ?�綴
		fsPath := path
		fsPath = strings.TrimPrefix(path, "/static/")
		fsPath = strings.TrimPrefix(fsPath, "/")

		// ?��??�路徑�?空路�?
		if fsPath == "" {
			fsPath = "index.html"
		}

		// ?��??�副檔�???HTML (�?/login)
		if !strings.Contains(filepath.Base(fsPath), ".") {
			altPath := fsPath + ".html"
			if _, err := fs.Stat(staticFS, altPath); err == nil {
				fsPath = altPath
			}
		}

		serveAsData(c, fsPath, "")
	})

	// API 路由 (?�援 v1 ?�本?�管)
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", h.Login)
		v1.POST("/auth/verify-2fa", h.Verify2FA)
		v1.POST("/auth/forgot-password", h.ForgotPasswordRequest)
		v1.POST("/auth/reset-password", h.ResetPassword)
		v1.GET("/system/info", h.GetSystemInfo)
		v1.GET("/branding", h.GetBrandingSettings)
		v1.GET("/system/config", h.GetSystemConfig)
		v1.POST("/license/debug", h.DebugLicense)

		auth := v1.Group("")
		auth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		{
			auth.POST("/auth/logout", h.Logout)
			auth.GET("/auth/me", h.GetCurrentUser)
			auth.POST("/auth/change-password", h.ChangePassword)
			auth.GET("/auth/2fa/status", h.GetTwoFactorStatus)
			auth.POST("/auth/2fa/enroll", h.BeginTOTPEnrollment)
			auth.POST("/auth/2fa/confirm", h.ConfirmTOTPEnrollment)
			auth.POST("/auth/2fa/disable", h.DisableTwoFactor)
			auth.POST("/auth/2fa/recovery-codes/regenerate", h.RegenerateRecoveryCodes)
			auth.GET("/dashboard", h.GetDashboard)
			auth.GET("/dashboard/top-cpu", h.GetTopCPU)
			auth.GET("/dashboard/top-memory", h.GetTopMemory)
			auth.GET("/license/status", h.GetLicenseStatus)
			auth.GET("/license/features", h.GetLicenseFeatures)
			auth.GET("/alerts/settings", h.GetAlertSettings)
			auth.GET("/events", h.GetEvents)
			auth.GET("/logs", h.GetLogs)
			auth.GET("/system-logs", h.GetSystemLogs)
			auth.GET("/device-logs", h.GetDeviceLogs)
			auth.GET("/config-change-logs", h.GetConfigChangeLogs)
			auth.GET("/log-center/options", h.GetLogCenterOptions)
			auth.GET("/log-center/export", h.ExportLogCenter)
			auth.GET("/log-center/evidence-bundle", h.ExportLogEvidenceBundle)
			auth.PUT("/audit-logs/:id/review", h.ReviewAuditLog)
			auth.PUT("/system-logs/:id/review", h.ReviewSystemLog)
			auth.PUT("/config-change-logs/:id/review", h.ReviewConfigChangeLog)
			auth.PUT("/device-logs/:id/ack", h.AckDeviceLog)
			// In-app notifications
			auth.GET("/notifications", h.GetNotifications)
			auth.PUT("/notifications/read-all", h.MarkAllNotificationsRead)
			auth.PUT("/notifications/:id/read", h.MarkNotificationRead)
			auth.DELETE("/notifications/all", h.DeleteAllNotifications)
			auth.DELETE("/notifications/:id", h.DeleteNotification)
		}

		deviceMgmtRead := v1.Group("")
		deviceMgmtRead.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		deviceMgmtRead.Use(h.RequireDeviceManagement())
		{
			deviceMgmtRead.GET("/devices", h.GetDevices)
			deviceMgmtRead.GET("/devices/:id", h.GetDevice)
			deviceMgmtRead.GET("/devices/:id/metrics", h.GetDeviceMetrics)
			deviceMgmtRead.GET("/devices/:id/interfaces", h.GetDeviceInterfaces)
			deviceMgmtRead.GET("/devices/:id/events", h.GetDeviceEvents)
			deviceMgmtRead.GET("/topology", h.GetTopology)
			deviceMgmtRead.GET("/topology/device/:id/interfaces", h.GetDeviceInterfacesForLink)
		}

		editor := v1.Group("")
		editor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		editor.Use(middleware.RequireEditor())
		editor.Use(h.RequireDeviceManagement())
		{
			editor.POST("/devices", h.CreateDevice)
			editor.PUT("/devices/:id", h.UpdateDevice)
			editor.DELETE("/devices/:id", h.DeleteDevice)
			editor.POST("/devices/:id/image", h.UploadDeviceImage)
			editor.POST("/devices/:id/poll", h.PollDevice)
			editor.POST("/devices/bulk-scan", h.BulkScan)
			editor.POST("/devices/bulk-delete", h.BulkDeleteDevices)
			editor.POST("/devices/bulk-update", h.BulkUpdateDevices)
			editor.POST("/topology/links", h.CreateTopologyLink)
			editor.PUT("/topology/links/:id", h.UpdateTopologyLink)
			editor.DELETE("/topology/links/:id", h.DeleteTopologyLink)
			editor.PUT("/topology/positions", h.UpdateDevicePositions)
			editor.POST("/topology/discover", h.DiscoverTopology)
			editor.GET("/reports/devices", h.ExportDevicesReport)
			editor.GET("/reports/devices/pdf", h.ExportDevicesReportPDF)
			editor.GET("/reports/logs", h.ExportLogsReport)
			editor.GET("/reports/logs/pdf", h.ExportLogsReportPDF)
			editor.GET("/reports/traffic", h.ExportTopTrafficReport)
			editor.GET("/reports/health", h.ExportDeviceHealthReport)
			editor.GET("/reports/availability", h.ExportAvailabilityReport)
			editor.GET("/reports/inventory", h.ExportInventoryReport)
			editor.POST("/devices/:id/reboot", h.RebootDevice)
			editor.POST("/devices/:id/backup", h.SaveDeviceConfig)
			editor.GET("/devices/:id/backups", h.GetDeviceConfigBackups)
			editor.GET("/devices/:id/backups/:backupId/download", h.DownloadDeviceConfigBackup)
			editor.GET("/devices/:id/poe", h.GetDevicePoE)
			editor.GET("/devices/:id/ap", h.GetDeviceApStatus)
			editor.POST("/devices/:id/poe/:portIndex/action", h.ControlPoEPort)
			editor.POST("/devices/:id/interfaces/:ifIndex/status", h.ControlPortStatus)
		}

		admin := v1.Group("")
		admin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/users", h.GetUsers)
			admin.POST("/users", h.CreateUser)
			admin.PUT("/users/:id", h.UpdateUser)
			admin.DELETE("/users/:id", h.DeleteUser)
			// Audit logs
			admin.GET("/audit-logs", h.GetAuditLogs)
			admin.GET("/audit-logs/actions", h.GetAuditActionKeys)
			admin.GET("/license/machine-id", h.GetEncryptedMachineID)
			admin.POST("/license/activate", h.ActivateLicense)
			admin.GET("/licenses", h.GetLicenses)
			admin.POST("/licenses", h.CreateLicense)
			admin.POST("/license/reset", h.ResetLicenseIdentity)
			admin.POST("/license/reissue", h.ReissueLicense)
			admin.GET("/system/backup", h.ExportBackup)
			admin.GET("/system/restore-readiness", h.CheckRestoreReadiness)
			admin.POST("/system/restore", h.RestoreBackup)
			admin.POST("/alerts/settings", h.SaveAlertSettings)
			admin.PUT("/alerts/settings/:type", h.UpdateAlertSetting)
			admin.POST("/alerts/test/:type", h.TestAlert)
			admin.POST("/tools/ping", h.PingTool)
			admin.POST("/tools/traceroute", h.TracerouteTool)
			admin.PUT("/branding", h.UpdateBrandingSettings)
			admin.POST("/branding/logo", h.UploadBrandingLogo)
			admin.DELETE("/branding/logo", h.DeleteBrandingLogo)
			admin.GET("/system/host-status", h.GetHostStatus)
			admin.GET("/security/settings", h.GetSecuritySettings)
			admin.PUT("/security/settings", h.UpdateSecuritySettings)
			admin.PUT("/system/config/:key", h.UpdateSystemConfig)

			// Encrypted Backup Management
			admin.POST("/system/backup/encrypted", h.ExportEncryptedBackup)
			admin.POST("/system/restore/encrypted", h.RestoreEncryptedBackup)
			admin.POST("/system/encryption/password", h.SetEncryptionPassword)
			admin.GET("/system/encryption/status", h.GetEncryptionStatus)

			// Camera Management (admin only, requires camera license)
			// NOTE: literal paths (/cameras/discover) MUST come before parameterized (/cameras/:id)
			admin.POST("/cameras", h.CreateCamera)
			admin.POST("/cameras/onvif/probe", h.ProbeONVIFStream)
			admin.POST("/cameras/discover", h.DiscoverCameras)
			admin.POST("/cameras/bulk-scan", h.BulkScanCameras)
			admin.PUT("/cameras/batch/credentials", h.BatchUpdateCredentials)
			admin.PUT("/cameras/:id", h.UpdateCamera)
			admin.PUT("/cameras/:id/monitor-display", h.SetMonitorDisplay)
			admin.DELETE("/cameras/:id", h.DeleteCamera)
			admin.POST("/cameras/:id/health", h.CameraHealthCheck)
			admin.GET("/nvr/config", h.GetNVRConfig)
			admin.PUT("/nvr/config", h.SetNVRConfig)
		}

		// Camera read routes (editor+)
		camEditor := v1.Group("")
		camEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		camEditor.Use(middleware.RequireEditor())
		{
			camEditor.GET("/cameras", h.GetCameras)
			camEditor.GET("/cameras/monitor", h.GetMonitorCameras)
			camEditor.GET("/cameras/:id", h.GetCamera)

			// NVR Recording Routes ??literal paths before :id params
			camEditor.GET("/cameras/recording/status", h.GetRecordingStatus)
			camEditor.PUT("/cameras/recording/batch", h.BatchSetRecording)
			camEditor.PUT("/cameras/:id/recording", h.SetCameraRecording)

			// Recordings management ??literal paths before :id
			camEditor.GET("/recordings/stats", h.GetRecordingStats)
			camEditor.GET("/recordings/dates", h.ListRecordingDates)
			camEditor.GET("/recordings", h.ListRecordings)
			camEditor.DELETE("/recordings/batch", h.BatchDeleteRecordings)
			camEditor.PUT("/recordings/:id/label", h.UpdateRecordingLabel)
			camEditor.DELETE("/recordings/:id", h.DeleteRecording)
			camEditor.GET("/recordings/:id/play", h.PlayRecording)
			camEditor.GET("/recordings/:id/export", h.ExportRecording)
		}

		// Camera status (any authenticated user) + snapshot/stream with token support
		camAuth := v1.Group("")
		camAuth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		{
			camAuth.GET("/cameras/status", h.GetCameraModuleStatus)
			camAuth.GET("/cameras/:id/snapshot", h.GetCameraSnapshot)
			camAuth.GET("/cameras/:id/stream/mjpeg", h.GetCameraMJPEG)
		}

		// Access Control module ??status readable by any authenticated user
		acAuth := v1.Group("")
		acAuth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		{
			acAuth.GET("/access-control/status", h.GetACModuleStatus)
			acAuth.GET("/access-control/events", h.GetACEvents)
		}

		// Access Control ??read (editor+)
		acEditor := v1.Group("")
		acEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		acEditor.Use(middleware.RequireEditor())
		{
			acEditor.GET("/access-control/doors", h.GetDoors)
			acEditor.GET("/access-control/doors/:id", h.GetDoor)
			acEditor.GET("/access-control/cards", h.GetCards)
		}

		// Access Control ??write (admin only)
		acAdmin := v1.Group("")
		acAdmin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		acAdmin.Use(middleware.RequireAdmin())
		{
			acAdmin.POST("/access-control/doors", h.CreateDoor)
			acAdmin.PUT("/access-control/doors/:id", h.UpdateDoor)
			acAdmin.DELETE("/access-control/doors/:id", h.DeleteDoor)
			acAdmin.POST("/access-control/doors/:id/action", h.ControlDoor)
			acAdmin.POST("/access-control/cards", h.CreateCard)
			acAdmin.PUT("/access-control/cards/:id", h.UpdateCard)
			acAdmin.DELETE("/access-control/cards/:id", h.DeleteCard)
			acAdmin.POST("/access-control/events", h.AddACEvent)
			// Card schedules
			acAdmin.GET("/access-control/schedules", h.GetCardSchedules)
			acAdmin.POST("/access-control/schedules", h.CreateCardSchedule)
			acAdmin.PUT("/access-control/schedules/:id", h.UpdateCardSchedule)
			acAdmin.DELETE("/access-control/schedules/:id", h.DeleteCardSchedule)
			acAdmin.POST("/access-control/schedules/:id/approve", h.ApproveCardSchedule)
			acAdmin.GET("/access-control/schedules/check", h.CheckCardScheduleAccess)
		}

		// PDU/UPS module ??status readable by any authenticated user
		pduAuth := v1.Group("")
		pduAuth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		{
			pduAuth.GET("/pdu/status", h.GetPDUModuleStatus)
		}

		// PDU/UPS ??read (editor+)
		pduEditor := v1.Group("")
		pduEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		pduEditor.Use(middleware.RequireEditor())
		{
			pduEditor.GET("/pdu/devices", h.GetPDUDevices)
			pduEditor.GET("/pdu/devices/:id", h.GetPDUDevice)
			pduEditor.POST("/pdu/devices/:id/poll", h.PollPDUDevice)
		}

		// PDU/UPS ??write (admin only)
		pduAdmin := v1.Group("")
		pduAdmin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		pduAdmin.Use(middleware.RequireAdmin())
		{
			pduAdmin.POST("/pdu/devices", h.CreatePDUDevice)
			pduAdmin.PUT("/pdu/devices/:id", h.UpdatePDUDevice)
			pduAdmin.DELETE("/pdu/devices/:id", h.DeletePDUDevice)
		}
	}

	// WebSSH terminal (generic SSH relay over WebSocket)
	{
		wsTerm := v1.Group("")
		wsTerm.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		wsTerm.GET("/devices/:id/terminal", h.WebSSHTerminal)
	}

	// Start NVR auto-resume for any cameras that had recording enabled
	h.NVRStartAll()

	return r
}
