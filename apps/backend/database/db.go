// Made by YTSworks
// YTS工作室製作
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Initialize(dbPath string, version string) (*sql.DB, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %v", err)
	}

	// Open database connection
	// _busy_timeout=30000 (30 seconds) to reduce lock errors
	// _journal_mode=WAL for better concurrency
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=30000")
	if err != nil {
		return nil, err
	}

	// Explicitly set PRAGMAs to ensure they take effect
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("Warning: Failed to set WAL mode: %v", err)
	}
	// Increase busy_timeout to 30 seconds to handle concurrency better
	if _, err := db.Exec("PRAGMA busy_timeout=30000;"); err != nil {
		log.Printf("Warning: Failed to set busy_timeout: %v", err)
	}
	// Synchronous=NORMAL is faster and safe enough for WAL
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		log.Printf("Warning: Failed to set synchronous mode: %v", err)
	}
	// Enable Foreign Keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Printf("Warning: Failed to enable foreign keys: %v", err)
	}
	// Treat schema objects as data, not trusted application code. This reduces
	// the impact of a locally modified SQLite schema.
	if _, err := db.Exec("PRAGMA trusted_schema = OFF;"); err != nil {
		log.Printf("Warning: Failed to disable trusted schema: %v", err)
	}
	// Ask SQLite to perform additional page-cell validation while reading.
	if _, err := db.Exec("PRAGMA cell_size_check = ON;"); err != nil {
		log.Printf("Warning: Failed to enable cell size checks: %v", err)
	}

	// busy_timeout is per-connection; single connection avoids SQLITE_BUSY under concurrent writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := createTables(db); err != nil {
		return nil, err
	}

	// 執行 Schema 升級
	if err := upgradeSchema(db, version); err != nil {
		log.Printf("Schema upgrade warning: %v", err)
	}

	log.Printf("Database initialized at %s", dbPath)
	return db, nil
}

func OpenReadOnly(dbPath string) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve read-only database path: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(absolutePath) + "?mode=ro&_busy_timeout=30000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open read-only database: %w", err)
	}
	if _, err := db.Exec(`PRAGMA query_only = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable read-only database mode: %w", err)
	}
	log.Printf("Database opened in integrity-lock read-only mode at %s", dbPath)
	return db, nil
}

// OpenIntegrityFallback provides a query-only in-memory connection when a
// corrupted SQLite file cannot be opened. Locked middleware blocks application
// APIs before handlers can use it; the connection only keeps static diagnostics
// and the integrity status endpoint available without touching the damaged file.
func OpenIntegrityFallback() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec(`PRAGMA query_only = ON`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable integrity fallback query-only mode: %w", err)
	}
	log.Println("Database integrity fallback is active; the persisted database was not opened")
	return db, nil
}

func createTables(db *sql.DB) error {
	// Devices 表 (含新的 SNMP 欄位)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			ip_address TEXT UNIQUE NOT NULL,
			mac_address TEXT,
			device_type TEXT DEFAULT 'unknown',
			snmp_community TEXT DEFAULT '',
			snmp_version INTEGER DEFAULT 2,
			vendor TEXT,
			model TEXT,
			firmware TEXT,
			sys_name TEXT,
			sys_uptime TEXT,
			sys_location TEXT,
			is_online BOOLEAN DEFAULT 0,
			last_seen DATETIME,
			image_path TEXT,
			pos_x REAL DEFAULT 0,
			pos_y REAL DEFAULT 0,
			is_name_custom BOOLEAN DEFAULT 0,
			snmpv3_security_name TEXT DEFAULT '',
			snmpv3_security_level TEXT DEFAULT 'noAuthNoPriv',
			snmpv3_auth_protocol TEXT DEFAULT 'MD5',
			snmpv3_auth_password TEXT DEFAULT '',
			snmpv3_priv_protocol TEXT DEFAULT 'DES',
			snmpv3_priv_password TEXT DEFAULT '',
			snmpv3_context_name TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Device Interfaces 表 (增強版)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS device_interfaces (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			if_index INTEGER,
			if_name TEXT,
			if_desc TEXT,
			if_speed BIGINT,
			if_mac TEXT,
			if_status INTEGER,
			if_admin_status INTEGER,
			in_octets BIGINT DEFAULT 0,
			out_octets BIGINT DEFAULT 0,
			in_errors BIGINT DEFAULT 0,
			out_errors BIGINT DEFAULT 0,
			prev_in_octets BIGINT DEFAULT 0,
			prev_out_octets BIGINT DEFAULT 0,
			bandwidth_in BIGINT DEFAULT 0,
			bandwidth_out BIGINT DEFAULT 0,
			poe_enabled INTEGER DEFAULT 0,
			poe_power_mw INTEGER DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Device Metrics 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS device_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			cpu_usage REAL,
			memory_usage REAL,
			disk_usage REAL,
			mem_total BIGINT DEFAULT 0,
			mem_used BIGINT DEFAULT 0,
			disk_total BIGINT DEFAULT 0,
			disk_used BIGINT DEFAULT 0,
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Topology Links 表 (增強版，含頻寬資訊)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_links (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_device_id INTEGER NOT NULL,
			target_device_id INTEGER NOT NULL,
			source_if_id INTEGER,
			target_if_id INTEGER,
			source_if_name TEXT,
			target_if_name TEXT,
			link_speed BIGINT DEFAULT 0,
			bandwidth_usage BIGINT DEFAULT 0,
			link_type TEXT DEFAULT 'auto',
			link_label TEXT,
			is_manual BOOLEAN DEFAULT 0,
			discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (source_device_id) REFERENCES devices(id) ON DELETE CASCADE,
			FOREIGN KEY (target_device_id) REFERENCES devices(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Syslogs 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS syslogs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			source_ip TEXT,
			severity TEXT,
			facility TEXT,
			message TEXT,
			received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return err
	}

	// Events 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER,
			event_type TEXT,
			severity TEXT,
			message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return err
	}

	// Users 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT,
			password_hash TEXT NOT NULL,
			role TEXT DEFAULT 'viewer',
			is_active BOOLEAN DEFAULT 1,
			force_change_password BOOLEAN DEFAULT 0,
			login_attempts INTEGER DEFAULT 0,
			locked_until DATETIME,
			last_password_change DATETIME DEFAULT CURRENT_TIMESTAMP,
			two_fa_code TEXT,
			two_fa_expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Password Reset Tokens 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS password_reset_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			token TEXT UNIQUE NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// User Two-Factor Settings 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_twofactor_settings (
			user_id INTEGER PRIMARY KEY,
			totp_secret_encrypted TEXT DEFAULT '',
			pending_totp_secret_encrypted TEXT DEFAULT '',
			totp_enabled BOOLEAN DEFAULT 0,
			email_otp_enabled BOOLEAN DEFAULT 0,
			preferred_method TEXT DEFAULT 'totp',
			recovery_codes_generated_at DATETIME,
			last_verified_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// User Recovery Codes 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_recovery_codes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			code_hash TEXT NOT NULL,
			used_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_user_id ON user_recovery_codes(user_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_user_recovery_codes_used_at ON user_recovery_codes(used_at)`)

	// Auth Login Challenges 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auth_login_challenges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			challenge_token TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			method TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			consumed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_auth_login_challenges_user_id ON auth_login_challenges(user_id)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_auth_login_challenges_expires_at ON auth_login_challenges(expires_at)`)

	// SuperAdmin authentication is isolated from normal login sessions. The
	// short-lived session is bound to the Admin session that opened the hidden UI.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS superadmin_auth_challenges (
			challenge_token TEXT PRIMARY KEY,
			superadmin_user_id INTEGER NOT NULL,
			parent_user_id INTEGER NOT NULL,
			parent_jti TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			consumed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (superadmin_user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS superadmin_sessions (
			jti TEXT PRIMARY KEY,
			superadmin_user_id INTEGER NOT NULL,
			parent_user_id INTEGER NOT NULL,
			parent_jti TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			revoked_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (superadmin_user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_superadmin_sessions_parent_jti ON superadmin_sessions(parent_jti)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_superadmin_sessions_expiry ON superadmin_sessions(expires_at)`)

	// Enhanced Licenses 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS licenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			license_key TEXT UNIQUE NOT NULL,
			license_type TEXT DEFAULT 'standard',
			device_count INTEGER DEFAULT 1,
			enabled_features TEXT DEFAULT '["email"]',
			valid_from DATETIME DEFAULT CURRENT_TIMESTAMP,
			valid_until DATETIME,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// System Configuration 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			config_key TEXT UNIQUE NOT NULL,
			config_value TEXT NOT NULL,
			description TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Alert Settings 表 (Corrected Schema)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS alert_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_type TEXT UNIQUE NOT NULL,
			is_enabled BOOLEAN DEFAULT 0,
			config_json TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('email', 0, '{}')`)
	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('line', 0, '{}')`)
	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('telegram', 0, '{}')`)
	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('discord', 0, '{}')`)
	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('slack', 0, '{}')`)
	db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('whatsapp', 0, '{}')`)

	// In-app notifications 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			severity TEXT NOT NULL DEFAULT 'warning',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			is_read BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// Audit logs 表（稽核日誌）
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			username TEXT NOT NULL,
			source_ip TEXT,
			source_mac TEXT,
			module TEXT DEFAULT '',
			action TEXT NOT NULL,
			resource TEXT,
			resource_type TEXT DEFAULT '',
			resource_id TEXT DEFAULT '',
			resource_name TEXT DEFAULT '',
			resource_ip TEXT DEFAULT '',
			resource_mac TEXT DEFAULT '',
			status TEXT DEFAULT 'success',
			detail TEXT,
			detail_json TEXT DEFAULT '',
			recordset_id TEXT DEFAULT '',
			correlation_id TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT DEFAULT 'info',
			service TEXT DEFAULT '',
			event_code TEXT DEFAULT '',
			message TEXT NOT NULL,
			context_json TEXT DEFAULT '',
			node_name TEXT DEFAULT '',
			build_version TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS device_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			device_id INTEGER,
			device_name TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			mac_address TEXT DEFAULT '',
			facility TEXT DEFAULT '',
			severity TEXT DEFAULT '',
			raw_message TEXT NOT NULL,
			normalized_message TEXT DEFAULT '',
			matched_rule TEXT DEFAULT '',
			ack_status TEXT DEFAULT 'unacked',
			context_json TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS config_change_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			username TEXT NOT NULL,
			source_ip TEXT,
			module TEXT DEFAULT '',
			target_type TEXT DEFAULT '',
			target_id TEXT DEFAULT '',
			target_name TEXT DEFAULT '',
			target_ip TEXT DEFAULT '',
			target_mac TEXT DEFAULT '',
			action TEXT NOT NULL,
			change_scope TEXT DEFAULT '',
			status TEXT DEFAULT 'success',
			change_source TEXT DEFAULT 'manual',
			recordset_id TEXT DEFAULT '',
			correlation_id TEXT DEFAULT '',
			old_values_json TEXT DEFAULT '',
			new_values_json TEXT DEFAULT '',
			detail_json TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_change_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			actor TEXT DEFAULT '',
			source_ip TEXT DEFAULT '',
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT DEFAULT '',
			graph_id TEXT DEFAULT 'default',
			detail_json TEXT DEFAULT '',
			old_values_json TEXT DEFAULT '',
			new_values_json TEXT DEFAULT '',
			correlation_id TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS topology_layout_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			actor TEXT DEFAULT '',
			source_ip TEXT DEFAULT '',
			graph_id TEXT DEFAULT 'default',
			layout_id TEXT DEFAULT 'default-layout',
			layout_type TEXT DEFAULT 'manual',
			reason TEXT DEFAULT '',
			snapshot_json TEXT NOT NULL,
			correlation_id TEXT DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	if _, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_topology_change_logs_occurred_at ON topology_change_logs(occurred_at)"); err != nil {
		return err
	}
	if _, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_topology_change_logs_entity ON topology_change_logs(entity_type, entity_id)"); err != nil {
		return err
	}
	if _, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_topology_layout_snapshots_created_at ON topology_layout_snapshots(created_at)"); err != nil {
		return err
	}
	if _, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_topology_layout_snapshots_layout ON topology_layout_snapshots(graph_id, layout_id)"); err != nil {
		return err
	}

	// Force password change for default admin
	_, _ = db.Exec(`
		INSERT OR IGNORE INTO users (username, password_hash, role, force_change_password) 
		VALUES ('admin', '$2a$10$Ec/YhIGuHjX6/g2bE8dsq.to2oo.oDO9i/zylvPFuqZx3KmtOC3yO', 'admin', 1)
	`)

	// Device Config Backups 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS device_config_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Cameras 表
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cameras (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT,
			ip_address TEXT,
			port INTEGER DEFAULT 554,
			username TEXT,
			password_encrypted TEXT,
			rtsp_url TEXT,
			preview_rtsp_url TEXT DEFAULT '',
			recording_rtsp_url TEXT DEFAULT '',
			rtsp_transport TEXT DEFAULT 'tcp',
			rtsp_udp_min_port INTEGER DEFAULT 0,
			rtsp_udp_max_port INTEGER DEFAULT 0,
			onvif_url TEXT,
			manufacturer TEXT,
			model TEXT,
			firmware TEXT,
			supports_ptz BOOLEAN DEFAULT 0,
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			stream_type TEXT DEFAULT 'mjpeg'
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func upgradeSchema(db *sql.DB, version string) error {
	// Migrate alert_settings if it uses the old schema (has 'key' column, not 'alert_type').
	var hasAlertTypeColumn int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('alert_settings') WHERE name='alert_type'").Scan(&hasAlertTypeColumn)
	if err == nil && hasAlertTypeColumn == 0 {
		var hasKeyColumn int
		db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('alert_settings') WHERE name='key'").Scan(&hasKeyColumn)

		if hasKeyColumn > 0 {
			log.Println("Migrating alert_settings from old schema...")
			if _, err := db.Exec("DROP TABLE alert_settings"); err != nil {
				log.Printf("Failed to drop old alert_settings table: %v", err)
			} else {
				_, err = db.Exec(`
					CREATE TABLE IF NOT EXISTS alert_settings (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						alert_type TEXT UNIQUE NOT NULL,
						is_enabled BOOLEAN DEFAULT 0,
						config_json TEXT DEFAULT '{}',
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
					)
				`)
				if err != nil {
					log.Printf("Failed to recreate alert_settings table: %v", err)
				} else {
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('email', 0, '{}')`)
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('line', 0, '{}')`)
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('telegram', 0, '{}')`)
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('discord', 0, '{}')`)
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('slack', 0, '{}')`)
					db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES ('whatsapp', 0, '{}')`)
					log.Println("alert_settings migrated.")
				}
			}
		}
	}

	alterStatements := []string{
		`CREATE TABLE IF NOT EXISTS interface_traffic_samples (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id INTEGER NOT NULL,
			if_index INTEGER NOT NULL,
			bandwidth_in BIGINT DEFAULT 0,
			bandwidth_out BIGINT DEFAULT 0,
			in_errors BIGINT DEFAULT 0,
			out_errors BIGINT DEFAULT 0,
			collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
		)`,
		"CREATE INDEX IF NOT EXISTS idx_interface_traffic_samples_device_time ON interface_traffic_samples(device_id, collected_at)",
		"ALTER TABLE devices ADD COLUMN sys_name TEXT",
		"ALTER TABLE notifications ADD COLUMN status TEXT NOT NULL DEFAULT 'open'",
		"ALTER TABLE notifications ADD COLUMN assigned_to TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN acknowledged_by TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN acknowledged_at DATETIME",
		"ALTER TABLE notifications ADD COLUMN resolved_at DATETIME",
		"ALTER TABLE notifications ADD COLUMN resolution_note TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN device_id INTEGER DEFAULT 0",
		"ALTER TABLE notifications ADD COLUMN category TEXT DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN sys_uptime TEXT",
		"ALTER TABLE devices ADD COLUMN sys_location TEXT",
		"ALTER TABLE device_interfaces ADD COLUMN if_admin_status INTEGER",
		"ALTER TABLE device_interfaces ADD COLUMN in_errors BIGINT DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN out_errors BIGINT DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN prev_in_octets BIGINT DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN prev_out_octets BIGINT DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN bandwidth_in BIGINT DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN bandwidth_out BIGINT DEFAULT 0",
		"ALTER TABLE topology_links ADD COLUMN source_if_name TEXT",
		"ALTER TABLE topology_links ADD COLUMN target_if_name TEXT",
		"ALTER TABLE topology_links ADD COLUMN link_speed BIGINT DEFAULT 0",
		"ALTER TABLE topology_links ADD COLUMN bandwidth_usage BIGINT DEFAULT 0",
		"ALTER TABLE topology_links ADD COLUMN link_label TEXT",
		// License table enhancements
		"ALTER TABLE licenses ADD COLUMN device_count INTEGER DEFAULT 1",
		"ALTER TABLE licenses ADD COLUMN enabled_features TEXT DEFAULT '[\"email\"]'",
		// User table enhancements
		"ALTER TABLE users ADD COLUMN force_change_password BOOLEAN DEFAULT 0",
		"ALTER TABLE devices ADD COLUMN cli_username TEXT",
		"ALTER TABLE devices ADD COLUMN cli_password TEXT",
		"ALTER TABLE devices ADD COLUMN snmp_rw_community TEXT",
		"ALTER TABLE devices ADD COLUMN is_name_custom BOOLEAN DEFAULT 0",
		"ALTER TABLE device_metrics ADD COLUMN mem_total BIGINT DEFAULT 0",
		"ALTER TABLE device_metrics ADD COLUMN mem_used BIGINT DEFAULT 0",
		"ALTER TABLE device_metrics ADD COLUMN disk_total BIGINT DEFAULT 0",
		"ALTER TABLE device_metrics ADD COLUMN disk_used BIGINT DEFAULT 0",
		"ALTER TABLE devices ADD COLUMN snmpv3_security_name TEXT DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN snmpv3_security_level TEXT DEFAULT 'noAuthNoPriv'",
		"ALTER TABLE devices ADD COLUMN snmpv3_auth_protocol TEXT DEFAULT 'MD5'",
		"ALTER TABLE devices ADD COLUMN snmpv3_auth_password TEXT DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN snmpv3_priv_protocol TEXT DEFAULT 'DES'",
		"ALTER TABLE devices ADD COLUMN snmpv3_priv_password TEXT DEFAULT ''",
		"ALTER TABLE devices ADD COLUMN snmpv3_context_name TEXT DEFAULT ''",
		"ALTER TABLE users ADD COLUMN email TEXT",
		"ALTER TABLE users ADD COLUMN login_attempts INTEGER DEFAULT 0",
		"ALTER TABLE users ADD COLUMN locked_until DATETIME",
		"ALTER TABLE users ADD COLUMN last_password_change DATETIME DEFAULT CURRENT_TIMESTAMP",
		"ALTER TABLE users ADD COLUMN two_fa_code TEXT",
		"ALTER TABLE users ADD COLUMN two_fa_expires_at DATETIME",
		"ALTER TABLE system_config ADD COLUMN description TEXT",
		"ALTER TABLE cameras ADD COLUMN stream_type TEXT DEFAULT 'mjpeg'",
		"ALTER TABLE licenses ADD COLUMN camera_count INTEGER DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN poe_enabled INTEGER DEFAULT 0",
		"ALTER TABLE device_interfaces ADD COLUMN poe_power_mw INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN recording_enabled INTEGER DEFAULT 0",
		`CREATE TABLE IF NOT EXISTS camera_recordings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			camera_id INTEGER NOT NULL,
			camera_name TEXT DEFAULT '',
			file_path TEXT NOT NULL,
			file_size INTEGER DEFAULT 0,
			duration_sec INTEGER DEFAULT 0,
			started_at DATETIME NOT NULL,
			ended_at DATETIME,
			label TEXT DEFAULT '',
			status TEXT DEFAULT 'recording' CHECK(status IN ('recording','done','error')),
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (camera_id) REFERENCES cameras(id) ON DELETE CASCADE
		)`,
		"ALTER TABLE camera_recordings ADD COLUMN camera_name TEXT DEFAULT ''",
		`UPDATE camera_recordings
			SET camera_name = COALESCE(
				NULLIF(camera_name, ''),
				(SELECT name FROM cameras WHERE cameras.id = camera_recordings.camera_id),
				''
			)
			WHERE COALESCE(camera_name, '') = ''`,
		"CREATE INDEX IF NOT EXISTS idx_recordings_camera ON camera_recordings(camera_id)",
		"CREATE INDEX IF NOT EXISTS idx_recordings_started ON camera_recordings(started_at)",
		// v1.2.0sp1
		"ALTER TABLE cameras ADD COLUMN onvif_url TEXT DEFAULT ''",
		// v1.2.0sp1 — Access Control module
		`CREATE TABLE IF NOT EXISTS ac_doors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			port INTEGER DEFAULT 80,
			username TEXT DEFAULT '',
			password_encrypted TEXT DEFAULT '',
			manufacturer TEXT DEFAULT '',
			model TEXT DEFAULT '',
			protocol TEXT DEFAULT 'http',
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ac_cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_number TEXT NOT NULL UNIQUE,
			holder_name TEXT NOT NULL,
			department TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 1,
			valid_from DATE,
			valid_until DATE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ac_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			door_id INTEGER,
			card_id INTEGER,
			card_number TEXT DEFAULT '',
			holder_name TEXT DEFAULT '',
			event_type TEXT DEFAULT 'access' CHECK(event_type IN ('access','denied','alarm','open','close','tamper')),
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (door_id) REFERENCES ac_doors(id) ON DELETE SET NULL,
			FOREIGN KEY (card_id) REFERENCES ac_cards(id) ON DELETE SET NULL
		)`,
		"CREATE INDEX IF NOT EXISTS idx_ac_events_door ON ac_events(door_id)",
		"CREATE INDEX IF NOT EXISTS idx_ac_events_occurred ON ac_events(occurred_at)",

		// v1.2.1 — Card schedule (timed access)
		`CREATE TABLE IF NOT EXISTS ac_card_schedules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER,
			card_number TEXT NOT NULL,
			door_id INTEGER,
			holder_name TEXT DEFAULT '',
			department TEXT DEFAULT '',
			allow_days TEXT DEFAULT '1,2,3,4,5,6,7',
			time_from TEXT DEFAULT '00:00',
			time_until TEXT DEFAULT '23:59',
			valid_from DATE NOT NULL,
			valid_until DATE NOT NULL,
			status TEXT DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected','expired')),
			note TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			approved_at DATETIME,
			approved_by TEXT DEFAULT '',
			FOREIGN KEY (card_id) REFERENCES ac_cards(id) ON DELETE SET NULL,
			FOREIGN KEY (door_id) REFERENCES ac_doors(id) ON DELETE SET NULL
		)`,
		"CREATE INDEX IF NOT EXISTS idx_ac_schedules_card ON ac_card_schedules(card_id)",
		"CREATE INDEX IF NOT EXISTS idx_ac_schedules_status ON ac_card_schedules(status)",

		// v1.2.1 — Camera monitor display preference
		"ALTER TABLE cameras ADD COLUMN monitor_display INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN monitor_order INTEGER DEFAULT 0",

		// v1.2.2 — Per-camera bandwidth limit (kbps, 0 = unlimited)
		"ALTER TABLE cameras ADD COLUMN bandwidth_limit_kbps INTEGER DEFAULT 0",

		// v1.2.2 — Recording source: 'rtsp' (use rtsp_url directly) | 'onvif' (fetch via ONVIF GetStreamUri each time)
		"ALTER TABLE cameras ADD COLUMN recording_source TEXT DEFAULT 'rtsp'",

		// v1.2.2 — IPCAM recording bitrate (kbps, 0 = copy/unlimited)
		"ALTER TABLE cameras ADD COLUMN recording_bitrate_kbps INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN preview_rtsp_url TEXT DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN recording_rtsp_url TEXT DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN rtsp_transport TEXT DEFAULT 'tcp'",
		"ALTER TABLE cameras ADD COLUMN rtsp_udp_min_port INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN rtsp_udp_max_port INTEGER DEFAULT 0",
		`UPDATE cameras
			SET preview_rtsp_url = COALESCE(NULLIF(preview_rtsp_url, ''), rtsp_url, '')
			WHERE COALESCE(preview_rtsp_url, '') = ''
			  AND COALESCE(rtsp_url, '') <> ''`,

		// v1.2.2 — In-app notifications table
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			severity TEXT NOT NULL DEFAULT 'warning',
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			is_read BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// v1.2.2 — Audit logs table
		"ALTER TABLE audit_logs ADD COLUMN module TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN resource_type TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN resource_id TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN resource_name TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN resource_ip TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN resource_mac TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN detail_json TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN recordset_id TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN correlation_id TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN review_status TEXT DEFAULT 'pending'",
		"ALTER TABLE audit_logs ADD COLUMN reviewed_by TEXT DEFAULT ''",
		"ALTER TABLE audit_logs ADD COLUMN reviewed_at DATETIME",
		"ALTER TABLE audit_logs ADD COLUMN review_note TEXT DEFAULT ''",
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			username TEXT NOT NULL,
			source_ip TEXT,
			source_mac TEXT,
			module TEXT DEFAULT '',
			action TEXT NOT NULL,
			resource TEXT,
			resource_type TEXT DEFAULT '',
			resource_id TEXT DEFAULT '',
			resource_name TEXT DEFAULT '',
			resource_ip TEXT DEFAULT '',
			resource_mac TEXT DEFAULT '',
			status TEXT DEFAULT 'success',
			detail TEXT,
			detail_json TEXT DEFAULT '',
			recordset_id TEXT DEFAULT '',
			correlation_id TEXT DEFAULT '',
			review_status TEXT DEFAULT 'pending',
			reviewed_by TEXT DEFAULT '',
			reviewed_at DATETIME,
			review_note TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT DEFAULT 'info',
			service TEXT DEFAULT '',
			event_code TEXT DEFAULT '',
			message TEXT NOT NULL,
			context_json TEXT DEFAULT '',
			node_name TEXT DEFAULT '',
			build_version TEXT DEFAULT '',
			review_status TEXT DEFAULT 'pending',
			reviewed_by TEXT DEFAULT '',
			reviewed_at DATETIME,
			review_note TEXT DEFAULT ''
		)`,
		"ALTER TABLE system_logs ADD COLUMN review_status TEXT DEFAULT 'pending'",
		"ALTER TABLE system_logs ADD COLUMN reviewed_by TEXT DEFAULT ''",
		"ALTER TABLE system_logs ADD COLUMN reviewed_at DATETIME",
		"ALTER TABLE system_logs ADD COLUMN review_note TEXT DEFAULT ''",
		`CREATE TABLE IF NOT EXISTS device_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			device_id INTEGER,
			device_name TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			mac_address TEXT DEFAULT '',
			facility TEXT DEFAULT '',
			severity TEXT DEFAULT '',
			raw_message TEXT NOT NULL,
			normalized_message TEXT DEFAULT '',
			matched_rule TEXT DEFAULT '',
			ack_status TEXT DEFAULT 'unacked',
			context_json TEXT DEFAULT '',
			review_status TEXT DEFAULT 'pending',
			reviewed_by TEXT DEFAULT '',
			reviewed_at DATETIME,
			review_note TEXT DEFAULT ''
		)`,
		"ALTER TABLE device_logs ADD COLUMN review_status TEXT DEFAULT 'pending'",
		"ALTER TABLE device_logs ADD COLUMN reviewed_by TEXT DEFAULT ''",
		"ALTER TABLE device_logs ADD COLUMN reviewed_at DATETIME",
		"ALTER TABLE device_logs ADD COLUMN review_note TEXT DEFAULT ''",
		`CREATE TABLE IF NOT EXISTS config_change_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			username TEXT NOT NULL,
			source_ip TEXT,
			module TEXT DEFAULT '',
			target_type TEXT DEFAULT '',
			target_id TEXT DEFAULT '',
			target_name TEXT DEFAULT '',
			target_ip TEXT DEFAULT '',
			target_mac TEXT DEFAULT '',
			action TEXT NOT NULL,
			change_scope TEXT DEFAULT '',
			status TEXT DEFAULT 'success',
			change_source TEXT DEFAULT 'manual',
			recordset_id TEXT DEFAULT '',
			correlation_id TEXT DEFAULT '',
			old_values_json TEXT DEFAULT '',
			new_values_json TEXT DEFAULT '',
			detail_json TEXT DEFAULT '',
			review_status TEXT DEFAULT 'pending',
			reviewed_by TEXT DEFAULT '',
			reviewed_at DATETIME,
			review_note TEXT DEFAULT ''
		)`,
		"ALTER TABLE config_change_logs ADD COLUMN review_status TEXT DEFAULT 'pending'",
		"ALTER TABLE config_change_logs ADD COLUMN reviewed_by TEXT DEFAULT ''",
		"ALTER TABLE config_change_logs ADD COLUMN reviewed_at DATETIME",
		"ALTER TABLE config_change_logs ADD COLUMN review_note TEXT DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_occurred_at ON audit_logs(occurred_at)",
		"CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)",
		"CREATE INDEX IF NOT EXISTS idx_system_logs_occurred_at ON system_logs(occurred_at)",
		"CREATE INDEX IF NOT EXISTS idx_device_logs_occurred_at ON device_logs(occurred_at)",
		"CREATE INDEX IF NOT EXISTS idx_device_logs_device_id ON device_logs(device_id)",
		"CREATE INDEX IF NOT EXISTS idx_config_change_logs_occurred_at ON config_change_logs(occurred_at)",
		"CREATE INDEX IF NOT EXISTS idx_config_change_logs_target ON config_change_logs(target_type, target_id)",

		// v1.2.1 — PDU/UPS SNMP module
		`CREATE TABLE IF NOT EXISTS pdu_devices (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT DEFAULT '',
			ip_address TEXT DEFAULT '',
			port INTEGER DEFAULT 161,
			snmp_community TEXT DEFAULT 'public',
			snmp_version INTEGER DEFAULT 2,
			device_type TEXT DEFAULT 'ups',
			manufacturer TEXT DEFAULT '',
			model TEXT DEFAULT '',
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			last_polled_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range alterStatements {
		_, _ = db.Exec(stmt)
	}

	// Reset admin password to default if the stored hash is not a valid bcrypt hash (length != 60).
	_, err = db.Exec(`
		UPDATE users
		SET password_hash = '$2a$10$Ec/YhIGuHjX6/g2bE8dsq.to2oo.oDO9i/zylvPFuqZx3KmtOC3yO',
			force_change_password = 1
		WHERE username = 'admin' AND LENGTH(password_hash) != 60
	`)
	if err != nil {
		log.Printf("Failed to auto-fix admin password: %v", err)
	}
	trimmedVersion := strings.TrimSpace(version)
	baseVersion := trimmedVersion
	if strings.HasSuffix(strings.ToLower(baseVersion), "-poc") {
		baseVersion = strings.TrimSpace(baseVersion[:len(baseVersion)-len("-poc")])
	}

	isPoC := strings.Contains(strings.ToLower(trimmedVersion), "poc")
	defaultDeviceLimit := "0"
	deviceManagementEnabled := "1"
	nmsEdition := "standard-" + baseVersion
	if isPoC {
		defaultDeviceLimit = "0"
		deviceManagementEnabled = "0"
		nmsEdition = "poc-" + baseVersion
	}

	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('default_device_limit', ?)`, defaultDeviceLimit)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('device_management_enabled', ?)`, deviceManagementEnabled)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('nms_edition', ?)`, nmsEdition)

	if isPoC {
		_, _ = db.Exec(`UPDATE system_config SET config_value = ? WHERE config_key = 'nms_edition' AND (TRIM(config_value) = '' OR LOWER(config_value) LIKE 'standard-%')`, nmsEdition)
	} else {
		_, _ = db.Exec(`UPDATE system_config SET config_value = ? WHERE config_key = 'device_management_enabled' AND TRIM(config_value) = '0'`, deviceManagementEnabled)
		_, _ = db.Exec(`UPDATE system_config SET config_value = ? WHERE config_key = 'nms_edition' AND (TRIM(config_value) = '' OR LOWER(config_value) LIKE 'poc-%')`, nmsEdition)
	}
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('trial_activated', 'false')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('global_2fa_enabled', 'false')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value) VALUES ('password_expiry_days', '90')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value, description) VALUES ('audit_log_retention_days', '1095', 'Audit log retention in days')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value, description) VALUES ('system_log_retention_days', '365', 'System log retention in days')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value, description) VALUES ('device_log_retention_days', '180', 'Device log retention in days')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value, description) VALUES ('config_change_log_retention_days', '1095', 'Config change log retention in days')`)
	_, _ = db.Exec(`INSERT OR IGNORE INTO system_config (config_key, config_value, description) VALUES ('log_retention_enabled', 'false', 'Enable log retention policy')`)

	return nil
}
