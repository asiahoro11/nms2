// Made by YTSworks
// YTS工作室製作
package notifications

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"management-server/config"
	licensemodule "management-server/modules/license"
	"management-server/services/alert"
)

type Service struct {
	db                   *sql.DB
	config               *config.Config
	license              *licensemodule.Service
	alertSettingsMu      sync.Mutex
	alertSettingsEnsured bool
	notificationsMu      sync.Mutex
	notificationsEnsured bool
}

func NewService(db *sql.DB, cfg *config.Config) *Service {
	return &Service{
		db:      db,
		config:  cfg,
		license: licensemodule.NewService(db, cfg),
	}
}

func (s *Service) EnsureAlertSettingsTable() error {
	s.alertSettingsMu.Lock()
	defer s.alertSettingsMu.Unlock()

	if s.alertSettingsEnsured {
		return nil
	}

	_, err := s.db.Exec(`
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

	defaults := []string{"email", "line", "telegram", "discord", "slack", "whatsapp"}
	for _, alertType := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO alert_settings (alert_type, is_enabled, config_json) VALUES (?, 0, '{}')`, alertType); err != nil {
			return err
		}
	}

	s.alertSettingsEnsured = true
	return nil
}

func (s *Service) EnsureNotificationsTable() error {
	s.notificationsMu.Lock()
	defer s.notificationsMu.Unlock()

	if s.notificationsEnsured {
		return nil
	}

	_, err := s.db.Exec(`
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

	s.notificationsEnsured = true
	return nil
}

func (s *Service) FeatureFlags() map[string]bool {
	return s.license.FeatureFlags()
}

func (s *Service) GetAlertSettings() ([]AlertSettingView, error) {
	if err := s.EnsureAlertSettingsTable(); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, alert_type, is_enabled, config_json
		FROM alert_settings
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}

	settings := make([]AlertSettingView, 0)
	for rows.Next() {
		var item AlertSettingView
		if err := rows.Scan(&item.ID, &item.AlertType, &item.IsEnabled, &item.Config); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.NeedsLicense = item.AlertType != "email"
		settings = append(settings, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	features := s.FeatureFlags()
	for i := range settings {
		settings[i].HasLicense = !settings[i].NeedsLicense || features[settings[i].AlertType]
	}

	return settings, nil
}

func (s *Service) UpdateAlertSetting(alertType string, input UpdateAlertSettingInput) error {
	if err := s.EnsureAlertSettingsTable(); err != nil {
		return err
	}

	if alertType != "email" && !s.FeatureFlags()[alertType] {
		return ErrAlertFeatureNotLicensed
	}

	result, err := s.db.Exec(`
		UPDATE alert_settings
		SET is_enabled = ?, config_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE alert_type = ?
	`, input.IsEnabled, input.Config, alertType)
	if err != nil {
		return err
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrAlertSettingNotFound
	}
	return nil
}

func (s *Service) TestAlert(alertType string) error {
	if err := s.EnsureAlertSettingsTable(); err != nil {
		return err
	}

	var configJSON string
	var isEnabled bool
	err := s.db.QueryRow(`SELECT config_json, is_enabled FROM alert_settings WHERE alert_type = ?`, alertType).Scan(&configJSON, &isEnabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrAlertSettingNotFound
		}
		return err
	}

	testMessage := fmt.Sprintf("[%s] 這是一則測試通知", time.Now().Format("2006-01-02 15:04:05"))
	return alert.SendAlert(alertType, configJSON, testMessage)
}

func (s *Service) LoadEmailChannelConfig() (string, error) {
	if err := s.EnsureAlertSettingsTable(); err != nil {
		return "", err
	}

	var configJSON string
	err := s.db.QueryRow(`
		SELECT config_json
		FROM alert_settings
		WHERE alert_type = 'email'
	`).Scan(&configJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrAlertSettingNotFound
		}
		return "", err
	}

	return configJSON, nil
}

func (s *Service) GetNotifications(unreadOnly bool) ([]Notification, int, error) {
	if err := s.EnsureNotificationsTable(); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, severity, title, message, is_read, created_at FROM notifications`
	if unreadOnly {
		query += ` WHERE is_read = 0`
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		if err := rows.Scan(&item.ID, &item.Severity, &item.Title, &item.Message, &item.IsRead, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var unreadCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE is_read = 0`).Scan(&unreadCount); err != nil {
		return nil, 0, err
	}

	return list, unreadCount, nil
}

func (s *Service) MarkNotificationRead(id int) error {
	if err := s.EnsureNotificationsTable(); err != nil {
		return err
	}

	result, err := s.db.Exec(`UPDATE notifications SET is_read = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *Service) MarkAllNotificationsRead() error {
	if err := s.EnsureNotificationsTable(); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE notifications SET is_read = 1 WHERE is_read = 0`)
	return err
}

func (s *Service) DeleteNotification(id int) error {
	if err := s.EnsureNotificationsTable(); err != nil {
		return err
	}

	result, err := s.db.Exec(`DELETE FROM notifications WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *Service) DeleteAllNotifications() error {
	if err := s.EnsureNotificationsTable(); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM notifications`)
	return err
}
