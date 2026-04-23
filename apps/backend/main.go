package main

import (
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
	if err := enforceLauncher(); err != nil {
		log.Fatal(err)
	}

	// 載入設�?
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ?��??�日�?
	logging.Init(cfg.Logging)
	log.Println("Logging system initialized")

	// ?��??��??�庫
	db, err := database.Initialize(cfg.Database.Path, cfg.System.Version)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// ?��??�系統�?設�?�?
	initSystemDefaults(db, cfg)

	// ?��? DB Worker (序�??�寫??
	dbWorker := dbworker.New(db)
	dbWorker.Start()
	defer dbWorker.Stop()

	// 檢查?��??��???(Startup Check)
	license.CheckCompliance(db, cfg, func(msg string) {
		alert.DispatchToEnabledChannels(db, cfg, msg)
	})

	// ?��? Syslog ?�收??
	syslogReceiver := syslog.NewReceiver(cfg.Syslog.Port, db)
	go syslogReceiver.Start()

	// ?��? Pinger ?��? (�?-5�?Ping)
	// Apply license limit to pinger
	pingSvc := pinger.New(cfg, db, dbWorker)
	pingSvc.Start()
	defer pingSvc.Stop()

	// ?��??�端??��??(Cloud Edge Connector)
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

	// ?��??��???
	sch := scheduler.New(cfg, db, dbWorker)
	go sch.Start()

	// ?��??��??�步?��?
	timesync.Start()

	// ?��? API ?��? (?�入 SNMP collector ???��?資�?)
	router := api.SetupRouter(cfg, db, sch.GetCollector(), assets)

	// ?��??��?
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := cfg.Server.Host + ":" + cfg.Server.Port
		log.Printf("Management Server starting on %s", addr)
		log.Printf("System Version: %s", cfg.System.Version)

		if cfg.Security.EnableTLS {
			log.Printf("[TLS] Enabled. Using cert: %s", cfg.Security.CertFile)
			if err := router.RunTLS(addr, cfg.Security.CertFile, cfg.Security.KeyFile); err != nil {
				log.Fatalf("Failed to start server (TLS): %v", err)
			}
		} else {
			if err := router.Run(addr); err != nil {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	// Auto-open browser
	scheme := "http"
	if cfg.Security.EnableTLS {
		scheme = "https"
	}
	go openBrowser(cfg.Server.Host+":"+cfg.Server.Port, scheme)

	<-quit
	log.Println("Shutting down server...")
	if cloudConn != nil {
		cloudConn.Stop()
	}
	sch.Stop()
	timesync.Stop()
	syslogReceiver.Stop()
}

func openBrowser(url string, scheme string) {
	// Add scheme if missing
	fullURL := scheme + "://" + url
	// Handle 0.0.0.0 or empty host
	if url[0] == ':' || url[:7] == "0.0.0.0" {
		port := url
		if url[:7] == "0.0.0.0" {
			port = url[7:]
		}
		fullURL = scheme + "://localhost" + port
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

// isCameraViewerEnabled 檢查 Camera Viewer 模�??�否?�用
func isCameraViewerEnabled(db *sql.DB) bool {
	var enabled string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&enabled)
	if err != nil {
		return false // ?�設?��?
	}
	return enabled == "true" || enabled == "1"
}

func initSystemDefaults(db *sql.DB, cfg *config.Config) {
	// ?�裡?�確保�??�庫中�?必�??��?設值�??��??�端?�找不到 key ?�顯示錯誤�??�??
	// ?�設??false，�?要使?�者�??��???
	defaults := map[string]string{
		"alerts_global_enabled":     "false",
		"camera_viewer_enabled":     "false",
		"camera_viewer_max_cameras": "4",
		"default_device_limit":      "10",
	}

	for key, val := range defaults {
		db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES (?, ?)`, key, val)
	}

	// 確�??�?��??�都?��?要�??�述資�? (如�?不�???
	db.Exec(`UPDATE system_config SET description = 'Enable Camera Viewer Module' WHERE config_key = 'camera_viewer_enabled' AND description IS NULL`)

}
