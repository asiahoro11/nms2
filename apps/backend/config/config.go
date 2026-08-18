// Made by YTSworks
// YTS工作室製作
package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Syslog   SyslogConfig   `yaml:"syslog"`
	SNMP     SNMPConfig     `yaml:"snmp"`
	System   SystemConfig   `yaml:"system"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
	Cloud    CloudConfig    `yaml:"cloud"`
}

type CloudConfig struct {
	Enabled           bool   `yaml:"enabled"`
	BrokerURL         string `yaml:"broker_url"`
	SiteID            string `yaml:"site_id"`
	SiteName          string `yaml:"site_name"`
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	CACert            string `yaml:"ca_cert"`
	HeartbeatInterval int    `yaml:"heartbeat_interval"`
	MetricsInterval   int    `yaml:"metrics_interval"`
	BufferMaxSize     int    `yaml:"buffer_max_size"`
}

type ServerConfig struct {
	Port    string `yaml:"port"`
	Host    string `yaml:"host"`
	BaseURL string `yaml:"base_url"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Path   string `yaml:"path"`
}

type SyslogConfig struct {
	Port          int `yaml:"port"`
	RetentionDays int `yaml:"retention_days"`
}

type SNMPConfig struct {
	Community        string `yaml:"community"`
	DefaultCommunity string `yaml:"default_community"`
	Timeout          int    `yaml:"timeout"`
	Retries          int    `yaml:"retries"`
}

type SystemConfig struct {
	Name      string `yaml:"name"`
	Version   string `yaml:"version"`
	StartTime string `json:"start_time"`
}

type SecurityConfig struct {
	JWTSecret      string   `yaml:"jwt_secret"`
	EnableTLS      bool     `yaml:"enable_tls"`
	CertFile       string   `yaml:"cert_file"`
	KeyFile        string   `yaml:"key_file"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	FrameAncestors []string `yaml:"frame_ancestors"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`    // debug, info, warn, error
	Filename   string `yaml:"filename"` // server.log
	MaxSize    int    `yaml:"max_size"` // megabytes
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`  // days
	Compress   bool   `yaml:"compress"` // disabled by default
}

// Version variable can be overridden by ldflags
var Version = "v1.2.4.11"

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: "8080",
			Host: "0.0.0.0",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "./data/nms.db",
		},
		Syslog: SyslogConfig{
			Port:          514,
			RetentionDays: 90,
		},
		SNMP: SNMPConfig{
			Community:        "public",
			DefaultCommunity: "public",
			Timeout:          2,
			Retries:          3,
		},
		System: SystemConfig{
			Name:      "System Server",
			Version:   Version,
			StartTime: fmt.Sprintf("%d", time.Now().Unix()),
		},
		Security: SecurityConfig{
			JWTSecret:      "",
			EnableTLS:      false,
			AllowedOrigins: []string{"*"},
			FrameAncestors: []string{"'self'", "http:", "https:"},
		},
		Logging: LoggingConfig{
			Level:      "info",
			Filename:   "server.log",
			MaxSize:    10,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
		Cloud: CloudConfig{
			Enabled:           false,
			BrokerURL:         "ssl://localhost:8883",
			HeartbeatInterval: 60,
			MetricsInterval:   300,
			BufferMaxSize:     10000,
		},
	}

	data, err := os.ReadFile("config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %v", err)
		}
	}

	cfg.System.Version = Version

	if err := ensureJWTSecret(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// legacyDefaultJWTSecret is the secret that older builds shipped as a hardcoded
// default. It is publicly known, so it must never be used to sign tokens on a
// new installation.
const legacyDefaultJWTSecret = "system-secret-key-2026-CHANGE-ME"

// jwtSecretFilename stores the generated secret next to the database so it
// survives restarts. The JWT secret also derives the encryption key for stored
// camera/door credentials and 2FA secrets, so it must stay stable once data
// has been written with it.
const jwtSecretFilename = "jwt.secret"

func ensureJWTSecret(cfg *Config) error {
	if cfg.Security.JWTSecret != "" && cfg.Security.JWTSecret != legacyDefaultJWTSecret {
		return nil
	}

	dataDir := filepath.Dir(cfg.Database.Path)
	secretPath := filepath.Join(dataDir, jwtSecretFilename)

	if raw, err := os.ReadFile(secretPath); err == nil {
		if secret := strings.TrimSpace(string(raw)); secret != "" {
			cfg.Security.JWTSecret = secret
			return nil
		}
	}

	// No stored secret. If a database already exists, this is an upgrade of a
	// deployment that ran with the legacy default; rotating the secret here
	// would make its encrypted camera/door passwords and 2FA secrets
	// unreadable, so keep the legacy value and warn loudly instead.
	if _, err := os.Stat(cfg.Database.Path); err == nil {
		cfg.Security.JWTSecret = legacyDefaultJWTSecret
		log.Printf("SECURITY WARNING: running with the publicly known default JWT secret. " +
			"Set security.jwt_secret in config.yaml to a random value. " +
			"Note: rotating the secret invalidates sessions and requires re-entering camera/door passwords and re-enrolling 2FA.")
		return nil
	}

	// Fresh installation: generate a random secret and persist it.
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("failed to generate JWT secret: %v", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(buf)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory for JWT secret: %v", err)
	}
	if err := os.WriteFile(secretPath, []byte(secret+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to persist JWT secret: %v", err)
	}

	cfg.Security.JWTSecret = secret
	log.Printf("Generated new JWT secret at %s", secretPath)
	return nil
}
