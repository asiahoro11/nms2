package main

import (
	"database/sql"
	"embed"
	"log"
	"management-server/api"          // Assuming api package is at the root or correctly imported based on go.mod
	"management-server/config"
	"management-server/database"
	"management-server/services/alert"
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

//go:embed static
var assets embed.FS

func init() {
	setupConsole()
}

func main() {
	// 載入設�?
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// ?��??�日�?
	logging.Init(cfg.Logging)
	log.Println("Logging system initialized")

	// ?��??��??�庫
	dbPath := cfg.Database.Path
	log.Printf("Initializing local SQLite database at %s", dbPath)
	db, err := database.Initialize(dbPath, "v1")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// ?��??�系統�?設�?�?
	initSystemDefaults(db)

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
		log.Printf("NMS Server starting on %s", addr)
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
	sch.Stop()
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

func initSystemDefaults(db *sql.DB) {
	// ?�裡?�確保�??�庫中�?必�??��?設值�??��??�端?�找不到 key ?�顯示錯誤�??�??
	// ?�設??false，�?要使?�者�??��???
	defaults := map[string]string{
		"alerts_global_enabled": "false",
		"camera_viewer_enabled": "false",
	}

	for key, val := range defaults {
		db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES (?, ?)`, key, val)
	}

	// Camera Viewer: Respect admin settings, only set default on first install
	var existingValue string
	err := db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&existingValue)
	if err != nil {
		// First install: set default to 'false'
		db.Exec(`INSERT INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_enabled', 'false', 'Enable Camera Viewer Module (requires license + manual enable)')`)
		log.Println("[Main] Camera Viewer config initialized to 'false' (default, requires manual enable)")
	} else {
		// Existing config: preserve admin choice
		log.Printf("[Main] Camera Viewer config preserved: '%s' (admin choice respected)", existingValue)
	}
}

// Stub setupConsole if it's missing from the main package scope or was in a separate file not imported
func setupConsole() {
	// Basic console setup if needed, or rely on internal/platform specific init
	// Originally this might have been in another file in the main package
	if runtime.GOOS == "windows" {
		// Windows console setup for UTF-8 is handled in backend/utf8_windows.go typically
		// but since we moved main, we might need to ensure that init runs or is present.
	}
}
