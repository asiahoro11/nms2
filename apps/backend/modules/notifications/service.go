// Made by YTSworks
// YTS工作室製作
package notifications

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
			status TEXT NOT NULL DEFAULT 'open',
			assigned_to TEXT DEFAULT '',
			acknowledged_by TEXT DEFAULT '',
			acknowledged_at DATETIME,
			resolved_at DATETIME,
			resolution_note TEXT DEFAULT '',
			device_id INTEGER DEFAULT 0,
			category TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	for _, statement := range []string{
		"ALTER TABLE notifications ADD COLUMN status TEXT NOT NULL DEFAULT 'open'",
		"ALTER TABLE notifications ADD COLUMN assigned_to TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN acknowledged_by TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN acknowledged_at DATETIME",
		"ALTER TABLE notifications ADD COLUMN resolved_at DATETIME",
		"ALTER TABLE notifications ADD COLUMN resolution_note TEXT DEFAULT ''",
		"ALTER TABLE notifications ADD COLUMN device_id INTEGER DEFAULT 0",
		"ALTER TABLE notifications ADD COLUMN category TEXT DEFAULT ''",
	} {
		_, _ = s.db.Exec(statement)
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

	query := `SELECT id, severity, title, message, is_read, created_at, status, assigned_to, acknowledged_by, COALESCE(acknowledged_at, ''), COALESCE(resolved_at, ''), resolution_note, COALESCE(device_id, 0), COALESCE(category, '') FROM notifications`
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
		if err := rows.Scan(&item.ID, &item.Severity, &item.Title, &item.Message, &item.IsRead, &item.CreatedAt, &item.Status, &item.AssignedTo, &item.AcknowledgedBy, &item.AcknowledgedAt, &item.ResolvedAt, &item.ResolutionNote, &item.DeviceID, &item.Category); err != nil {
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

func (s *Service) UpdateWorkflow(id int, input UpdateWorkflowInput, actor string) error {
	if err := s.EnsureNotificationsTable(); err != nil {
		return err
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "open" && status != "acknowledged" && status != "resolved" {
		return errors.New("invalid workflow status")
	}
	result, err := s.db.Exec(`UPDATE notifications SET status=?, assigned_to=?, resolution_note=?, is_read=1,
		acknowledged_by=CASE WHEN ?='acknowledged' THEN ? ELSE acknowledged_by END,
		acknowledged_at=CASE WHEN ?='acknowledged' THEN CURRENT_TIMESTAMP ELSE acknowledged_at END,
		resolved_at=CASE WHEN ?='resolved' THEN CURRENT_TIMESTAMP ELSE NULL END WHERE id=?`,
		status, strings.TrimSpace(input.AssignedTo), strings.TrimSpace(input.ResolutionNote), status, actor, status, status, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrNotificationNotFound
	}
	return nil
}

func (s *Service) ListWorkflow(query WorkflowQuery) ([]Notification, int, map[string]int, error) {
	if err := s.EnsureNotificationsTable(); err != nil {
		return nil, 0, nil, err
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 25
	}
	where, args := " WHERE 1=1", []interface{}{}
	if status := strings.TrimSpace(query.Status); status != "" && status != "all" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if severity := strings.TrimSpace(query.Severity); severity != "" {
		where += " AND severity = ?"
		args = append(args, severity)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		where += " AND (title LIKE ? OR message LIKE ? OR assigned_to LIKE ?)"
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM notifications"+where, args...).Scan(&total); err != nil {
		return nil, 0, nil, err
	}
	rows, err := s.db.Query(`SELECT id, severity, title, message, is_read, created_at, status, assigned_to, acknowledged_by, COALESCE(acknowledged_at, ''), COALESCE(resolved_at, ''), resolution_note, COALESCE(device_id, 0), COALESCE(category, '') FROM notifications`+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, append(args, query.Limit, (query.Page-1)*query.Limit)...)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()
	items := make([]Notification, 0)
	for rows.Next() {
		var item Notification
		if err := rows.Scan(&item.ID, &item.Severity, &item.Title, &item.Message, &item.IsRead, &item.CreatedAt, &item.Status, &item.AssignedTo, &item.AcknowledgedBy, &item.AcknowledgedAt, &item.ResolvedAt, &item.ResolutionNote, &item.DeviceID, &item.Category); err != nil {
			return nil, 0, nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}
	summary := map[string]int{"open": 0, "acknowledged": 0, "resolved": 0}
	summaryRows, err := s.db.Query(`SELECT status, COUNT(*) FROM notifications GROUP BY status`)
	if err != nil {
		return nil, 0, nil, err
	}
	defer summaryRows.Close()
	for summaryRows.Next() {
		var status string
		var count int
		if err := summaryRows.Scan(&status, &count); err != nil {
			return nil, 0, nil, err
		}
		summary[status] = count
	}
	return items, total, summary, nil
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
