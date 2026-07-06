// Made by YTSworks
// YTS工作室製作
package main

import (
	"context"
	"database/sql"
	"log"
	"management-server/api"
	"management-server/config"
	"management-server/database"
	"management-server/services/alert"
	"management-server/services/cloud"
	"management-server/services/dbworker"
	"management-server/services/license"
	"management-server/services/logging"
	"management-server/services/pinger"
	"management-server/services/scheduler"
	"management-server/services/syslog"
	"management-server/services/timesync"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func init() {
	setupConsole()
}

func main() {
	enforceLauncher()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logging
	logging.Init(cfg.Logging)
	log.Println("Logging system initialized")

	// 初始化資料庫
	db, err := database.Initialize(cfg.Database.Path, cfg.System.Version)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize system defaults
	initSystemDefaults(db, cfg)

	// Initialize DB worker (serialized writes)
	dbWorker := dbworker.New(db)
	dbWorker.Start()
	defer dbWorker.Stop()

	// Startup health checks
	license.CheckCompliance(db, cfg, func(msg string) {
		alert.DispatchToEnabledChannels(db, cfg, msg)
	})

	// Start Syslog receiver
	syslogReceiver := syslog.NewReceiver(cfg.Syslog.Port, db)
	go syslogReceiver.Start()

	// 啟動 Pinger 心跳檢測服務
	// Apply license limit to pinger
	pingSvc := pinger.New(cfg, db, dbWorker)
	pingSvc.Start()
	defer pingSvc.Stop()

	// Start cloud edge connector
	var cloudConn *cloud.Connector
	if cfg.Cloud.Enabled {
		cloudConn = cloud.New(cfg.Cloud, db)
		if err := cloudConn.Start(); err != nil {
			log.Printf("WARNING: Cloud connector failed to start: %v", err)
		} else {
			defer cloudConn.Stop()
			// Wire up callbacks
			pingSvc.OnStatusChange = cloudConn.HandleDeviceStatusChange
			alert.CloudAlertHook = cloudConn.HandleAlert
			log.Println("Cloud connector enabled and callbacks wired")
		}
	}

	// 啟動排程器
	sch := scheduler.New(cfg, db, dbWorker)
	go sch.Start()

	// 啟動時間同步服務
	timesync.Start()

	// 建立 API 路由 (注入 SNMP collector 與嵌入資源)
	router := api.SetupRouter(cfg, db, sch.GetCollector(), assets)

	// 等待結束訊號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ReadHeaderTimeout/IdleTimeout only: blanket Read/Write timeouts would
	// kill long-lived WebSSH websockets and camera streams.
	srv := &http.Server{
		Addr:              cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Management Server starting on %s", srv.Addr)
		log.Printf("System Version: %s", cfg.System.Version)

		var err error
		if cfg.Security.EnableTLS {
			log.Printf("[TLS] Enabled. Using cert: %s", cfg.Security.CertFile)
			err = srv.ListenAndServeTLS(cfg.Security.CertFile, cfg.Security.KeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Auto-open browser
	scheme := "http"
	if cfg.Security.EnableTLS {
		scheme = "https"
	}
	go openBrowser(cfg.Server.Host, cfg.Server.Port, scheme)

	<-quit
	log.Println("Shutting down server...")

	// Stop accepting new requests and let in-flight ones finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown: %v", err)
	}

	if cloudConn != nil {
		cloudConn.Stop()
	}
	sch.Stop()
	timesync.Stop()
	syslogReceiver.Stop()
}

func openBrowser(host string, port string, scheme string) {
	// 0.0.0.0 / :: / empty bind addresses are not browsable; use localhost.
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	fullURL := scheme + "://" + host
	if port != "" {
		fullURL += ":" + port
	}

	// Wait a bit for server to start
	time.Sleep(1 * time.Second)

	var err error
	switch runtime.GOOS {
	case "linux":
		// On Linux, only try to open browser if a display server is detected (Desktop env)
		if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
			err = exec.Command("xdg-open", fullURL).Start()
		} else {
			log.Println("Server environment detected (headless), skipping auto-open browser.")
		}
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", fullURL).Start()
	case "darwin":
		err = exec.Command("open", fullURL).Start()
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

func initSystemDefaults(db *sql.DB, cfg *config.Config) {
	// 確保資料庫中有必要的預設值，避免前端因找不到 key 而顯示錯誤
	// 預設為 false，需要使用者手動開啟
	defaults := map[string]string{
		"alerts_global_enabled":     "false",
		"camera_viewer_enabled":     "false",
		"camera_viewer_max_cameras": "4",
		"default_device_limit":      "10",
	}

	for key, val := range defaults {
		db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES (?, ?)`, key, val)
	}

	// 確保設定項目都有描述資料 (如果不存在)
	db.Exec(`UPDATE system_config SET description = 'Enable Camera Viewer Module' WHERE config_key = 'camera_viewer_enabled' AND description IS NULL`)

}
