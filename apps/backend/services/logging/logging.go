// Made by YTSworks
// YTS工作室製作
package logging

import (
	"log/slog"
	"management-server/config"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var mainLogger *lumberjack.Logger

func Init(cfg config.LoggingConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Setup file logger with rotation
	mainLogger = &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// Create JSON handler
	jsonHandler := slog.NewJSONHandler(mainLogger, &slog.HandlerOptions{
		Level: level,
	})

	// Set as default logger
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)

	// Start background rotation check (Daily at midnight)
	go startPeriodicRotation()
}

func startPeriodicRotation() {
	for {
		now := time.Now()
		// Calculate time until next midnight
		next := now.Add(24 * time.Hour)
		next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		t := time.NewTimer(next.Sub(now))

		<-t.C
		if mainLogger != nil {
			slog.Info("Performing scheduled log rotation")
			_ = mainLogger.Rotate()
		}
	}
}

// AuditLog logs user actions specifically for compliance
func AuditLog(username string, ip string, action string, resource string, status string, details map[string]interface{}) {
	slog.Info("AUDIT_EVENT",
		slog.String("type", "audit"),
		slog.String("ts", time.Now().Format(time.RFC3339)),
		slog.String("user", username),
		slog.String("ip", ip),
		slog.String("action", action),
		slog.String("resource", resource),
		slog.String("status", status),
		slog.Any("details", details),
	)
}

func GetMainLogger() *lumberjack.Logger {
	return mainLogger
}
