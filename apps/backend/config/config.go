package config

import (
	"fmt"
	"os"
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
var Version = "v1.2.4.8"

func Load() (*Config, error) {

	// ??澈?桀??
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
			JWTSecret:      "system-secret-key-2026-CHANGE-ME", // Default secret
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

	// ?謅疵???config.yaml
	data, err := os.ReadFile("config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config.yaml: %v", err)
		}
	}

	// ????謅???蟡???⊿豲??瞏秧???(?頦config.yaml ?謘餉爸)
	// Version is injected via ldflags, fallback to default
	cfg.System.Version = Version

	return cfg, nil
}
