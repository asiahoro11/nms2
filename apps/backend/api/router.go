// Made by YTSworks
// YTS工作室製作
package api

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"management-server/api/handlers"
	"management-server/api/middleware"
	"management-server/config"
	"management-server/services/integrity"
	"management-server/services/snmp"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, db *sql.DB, collector *snmp.Collector, assets embed.FS, integrityGuard *integrity.Guard) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.MaxMultipartMemory = 256 << 20 // 256 MiB
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.Security.AllowedOrigins))
	r.Use(middleware.Logger())
	r.Use(middleware.IntegrityLock(integrityGuard))
	r.Use(middleware.Secure(cfg, db))

	// Create handlers and start background loops
	h := handlers.New(cfg, db, collector)
	if integrityGuard == nil || !integrityGuard.Locked() {
		h.StartCameraHealthLoop()
		h.StartLicenseHealthLoop()
		h.StartIoTLoop()
	}

	// --- Static file serving ---
	// Strategy 1: check if a real frontend/ dir exists (dev mode)
	// Strategy 2: use embedded assets (production)

	// Upload path — prefer ./data in current dir
	dataPath := "./data"
	if _, err := os.Stat("./data"); err != nil {
		// No ./data found, try ../data
		if _, err := os.Stat("../data"); err == nil {
			dataPath = "../data"
		}
	}
	r.Static("/uploads", filepath.Join(dataPath, "uploads"))

	log.Println("[Router] Operating in EMBEDDED mode (Production)")

	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false

	// Sub FS so index.html is at root (not /static/index.html)
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatalf("Failed to create sub-filesystem: %v", err)
	}

	// Serve via ReadFile instead of FileFromFS to avoid 301 redirects
	serveAsData := func(c *gin.Context, fsPath string, contentType string) {
		data, err := fs.ReadFile(staticFS, fsPath)
		if err != nil {
			// File not found and not index.html → fall back to SPA root
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
			// Auto-detect Content-Type from extension
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

	// Catch-all: serve SPA for non-API frontend routes
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API requests should not reach NoRoute
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
			return
		}

		// Strip /static/ prefix and leading slash
		fsPath := path
		fsPath = strings.TrimPrefix(path, "/static/")
		fsPath = strings.TrimPrefix(fsPath, "/")

		// Empty path → serve index.html
		if fsPath == "" {
			fsPath = "index.html"
		}

		// No extension (e.g. /login) → try .html variant
		if !strings.Contains(filepath.Base(fsPath), ".") {
			altPath := fsPath + ".html"
			if _, err := fs.Stat(staticFS, altPath); err == nil {
				fsPath = altPath
			}
		}

		serveAsData(c, fsPath, "")
	})

	// API routes (v1)
	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", h.Login)
		v1.POST("/auth/verify-2fa", h.Verify2FA)
		v1.POST("/auth/forgot-password", h.ForgotPasswordRequest)
		v1.POST("/auth/reset-password", h.ResetPassword)
		v1.GET("/system/info", h.GetSystemInfo)
		v1.GET("/branding", h.GetBrandingSettings)

		auth := v1.Group("")
		auth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		auth.Use(h.EnforceLicenseLock())
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
			auth.GET("/system/config", h.GetSystemConfig)
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
		deviceMgmtRead.Use(h.EnforceLicenseLock())
		deviceMgmtRead.Use(h.RequireDeviceManagement())
		{
			deviceMgmtRead.GET("/devices", h.GetDevices)
			deviceMgmtRead.GET("/devices/:id", h.GetDevice)
			deviceMgmtRead.GET("/devices/:id/metrics", h.GetDeviceMetrics)
			deviceMgmtRead.GET("/devices/:id/interfaces", h.GetDeviceInterfaces)
			deviceMgmtRead.GET("/devices/:id/traffic", h.GetDeviceTraffic)
			deviceMgmtRead.GET("/devices/:id/events", h.GetDeviceEvents)
			deviceMgmtRead.GET("/topology", h.GetTopology)
			deviceMgmtRead.GET("/topology/device/:id/interfaces", h.GetDeviceInterfacesForLink)
			deviceMgmtRead.GET("/integrations/network-snapshot", h.GetIntegrationNetworkSnapshot)
			deviceMgmtRead.GET("/integrations/embed-snapshot", h.GetIntegrationEmbedSnapshot)
			deviceMgmtRead.GET("/iot/status", h.GetIoTStatus)
			deviceMgmtRead.GET("/iot/capabilities", h.GetIoTCapabilities)
			deviceMgmtRead.GET("/iot/devices", h.ListIoTDevices)
			deviceMgmtRead.GET("/iot/measurements", h.GetIoTMeasurements)
			deviceMgmtRead.GET("/iot/queue/status", h.GetIoTQueueStatus)
		}

		editor := v1.Group("")
		editor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		editor.Use(h.EnforceLicenseLock())
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
			editor.GET("/reports/interfaces", h.ExportInterfaceReport)
			editor.GET("/reports/health-trend", h.ExportHealthTrendReport)
			editor.GET("/reports/sla", h.ExportSLAReport)
			editor.GET("/reports/audit", h.ExportAuditReport)
			editor.GET("/reports/license-capacity", h.ExportLicenseCapacityReport)
			editor.GET("/reports/cameras", h.ExportCameraReport)
			editor.GET("/reports/pdu", h.ExportPDUReport)
			editor.GET("/reports/access-control", h.ExportAccessControlReport)
			editor.GET("/reports/topology", h.ExportTopologyReport)
			editor.GET("/reports/events", h.ExportEventReport)
			editor.GET("/reports/syslog", h.ExportSyslogReport)
			editor.GET("/reports/notifications", h.ExportNotificationReport)
			editor.GET("/reports/iot-devices", h.ExportIoTDeviceReport)
			editor.GET("/reports/iot-measurements", h.ExportIoTMeasurementReport)
			editor.GET("/reports/camera-recordings", h.ExportCameraRecordingReport)
			editor.GET("/reports/access-events", h.ExportAccessEventReport)
			editor.GET("/reports/access-cards", h.ExportAccessCardReport)
			editor.GET("/reports/access-schedules", h.ExportAccessScheduleReport)
			editor.GET("/reports/config-backups", h.ExportConfigBackupReport)
			editor.POST("/devices/:id/reboot", h.RebootDevice)
			editor.POST("/devices/:id/backup", h.SaveDeviceConfig)
			editor.GET("/devices/:id/backups", h.GetDeviceConfigBackups)
			editor.GET("/devices/:id/backups/:backupId/download", h.DownloadDeviceConfigBackup)
			editor.GET("/devices/:id/poe", h.GetDevicePoE)
			editor.GET("/devices/:id/ap", h.GetDeviceApStatus)
			editor.POST("/devices/:id/poe/:portIndex/action", h.ControlPoEPort)
			editor.POST("/devices/:id/interfaces/:ifIndex/status", h.ControlPortStatus)
			editor.POST("/iot/ingest", h.IngestIoTMeasurement)
			editor.POST("/iot/devices/:id/poll", h.PollIoTDevice)
			editor.POST("/iot/queue/flush", h.FlushIoTForwardQueue)
		}

		admin := v1.Group("")
		admin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		admin.Use(h.EnforceLicenseLock())
		admin.Use(middleware.RequireAdmin())
		{
			admin.GET("/users", h.GetUsers)
			admin.GET("/auth/superadmin/status", h.GetSuperAdminStatus)
			admin.POST("/auth/superadmin/login", h.BeginSuperAdminLogin)
			admin.POST("/auth/superadmin/verify", h.VerifySuperAdminLogin)
			admin.POST("/auth/superadmin/logout", h.EndSuperAdminSession)
			admin.POST("/auth/superadmin/recover", h.RecoverSuperAdminPassword)
			admin.POST("/users", h.CreateUser)
			admin.PUT("/users/:id", h.UpdateUser)
			admin.DELETE("/users/:id", h.DeleteUser)
			// Audit logs
			admin.GET("/audit-logs", h.GetAuditLogs)
			admin.GET("/audit-logs/actions", h.GetAuditActionKeys)
			admin.GET("/license/machine-id", h.GetEncryptedMachineID)
			admin.POST("/license/activate", h.ActivateLicense)
			admin.GET("/licenses", h.GetLicenses)
			admin.GET("/system/backup", h.ExportBackup)
			admin.GET("/system/restore-readiness", h.CheckRestoreReadiness)
			admin.POST("/system/restore", h.RestoreBackup)
			admin.POST("/alerts/settings", h.SaveAlertSettings)
			admin.PUT("/alerts/settings/:type", h.UpdateAlertSetting)
			admin.POST("/alerts/test/:type", h.TestAlert)
			admin.GET("/system/host-status", h.GetHostStatus)
			admin.GET("/security/settings", h.GetSecuritySettings)
			admin.PUT("/security/settings", h.UpdateSecuritySettings)
			admin.PUT("/system/config/:key", h.UpdateSystemConfig)
			admin.GET("/integrations/settings", h.GetIntegrationSettings)
			admin.PUT("/integrations/settings", h.UpdateIntegrationSettings)
			admin.GET("/integrations/embed-tokens", h.ListEmbedTokens)
			admin.POST("/integrations/embed-tokens", h.CreateEmbedToken)
			admin.DELETE("/integrations/embed-tokens/:tokenId", h.RevokeEmbedToken)
			admin.POST("/iot/devices", h.CreateIoTDevice)
			admin.PUT("/iot/devices/:id", h.UpdateIoTDevice)
			admin.DELETE("/iot/devices/:id", h.DeleteIoTDevice)
			admin.GET("/iot/forwarder/settings", h.GetIoTForwarderSettings)
			admin.PUT("/iot/forwarder/settings", h.UpdateIoTForwarderSettings)
			admin.POST("/iot/queue/cleanup", h.CleanupIoTForwardQueue)

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

		superAdmin := v1.Group("")
		superAdmin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		superAdmin.Use(h.EnforceLicenseLock())
		superAdmin.Use(middleware.RequireSuperAdmin(db))
		{
			superAdmin.POST("/auth/superadmin/change-password", h.ChangeSuperAdminPassword)
			superAdmin.POST("/license/reset", h.ResetLicenseIdentity)
			superAdmin.POST("/tools/ping", h.PingTool)
			superAdmin.POST("/tools/traceroute", h.TracerouteTool)
			superAdmin.PUT("/branding", h.UpdateBrandingSettings)
			superAdmin.POST("/branding/logo", h.UploadBrandingLogo)
			superAdmin.DELETE("/branding/logo", h.DeleteBrandingLogo)
		}

		// Camera read routes (editor+)
		camEditor := v1.Group("")
		camEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		camEditor.Use(h.EnforceLicenseLock())
		camEditor.Use(middleware.RequireEditor())
		{
			camEditor.GET("/cameras", h.GetCameras)
			camEditor.GET("/cameras/monitor", h.GetMonitorCameras)
			camEditor.GET("/cameras/:id", h.GetCamera)

			// NVR Recording Routes — literal paths before :id params
			camEditor.GET("/cameras/recording/status", h.GetRecordingStatus)
			camEditor.PUT("/cameras/recording/batch", h.BatchSetRecording)
			camEditor.PUT("/cameras/:id/recording", h.SetCameraRecording)

			// Recordings management — literal paths before :id
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
		camAuth.Use(h.EnforceLicenseLock())
		{
			camAuth.GET("/cameras/status", h.GetCameraModuleStatus)
			camAuth.GET("/cameras/:id/snapshot", h.GetCameraSnapshot)
			camAuth.GET("/cameras/:id/stream/mjpeg", h.GetCameraMJPEG)
		}

		// Access Control module — status readable by any authenticated user
		acAuth := v1.Group("")
		acAuth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		acAuth.Use(h.EnforceLicenseLock())
		{
			acAuth.GET("/access-control/status", h.GetACModuleStatus)
			acAuth.GET("/access-control/events", h.GetACEvents)
		}

		// Access Control — read (editor+)
		acEditor := v1.Group("")
		acEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		acEditor.Use(h.EnforceLicenseLock())
		acEditor.Use(middleware.RequireEditor())
		{
			acEditor.GET("/access-control/doors", h.GetDoors)
			acEditor.GET("/access-control/doors/:id", h.GetDoor)
			acEditor.GET("/access-control/cards", h.GetCards)
		}

		// Access Control — write (admin only)
		acAdmin := v1.Group("")
		acAdmin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		acAdmin.Use(h.EnforceLicenseLock())
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

		// PDU/UPS module — status readable by any authenticated user
		pduAuth := v1.Group("")
		pduAuth.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		pduAuth.Use(h.EnforceLicenseLock())
		{
			pduAuth.GET("/pdu/status", h.GetPDUModuleStatus)
		}

		// PDU/UPS — read (editor+)
		pduEditor := v1.Group("")
		pduEditor.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		pduEditor.Use(h.EnforceLicenseLock())
		pduEditor.Use(middleware.RequireEditor())
		{
			pduEditor.GET("/pdu/devices", h.GetPDUDevices)
			pduEditor.GET("/pdu/devices/:id", h.GetPDUDevice)
			pduEditor.POST("/pdu/devices/:id/poll", h.PollPDUDevice)
		}

		// PDU/UPS — write (admin only)
		pduAdmin := v1.Group("")
		pduAdmin.Use(middleware.AuthRequired([]byte(cfg.Security.JWTSecret)))
		pduAdmin.Use(h.EnforceLicenseLock())
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
		wsTerm.Use(h.EnforceLicenseLock())
		wsTerm.POST("/devices/:id/terminal-ticket", h.CreateWebSSHTicket)
	}
	v1.GET("/devices/:id/terminal", h.WebSSHTerminal)

	// Start NVR auto-resume for any cameras that had recording enabled
	if integrityGuard == nil || !integrityGuard.Locked() {
		h.NVRStartAll()
	}

	return r
}
