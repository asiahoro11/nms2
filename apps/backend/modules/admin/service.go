// Made by YTSworks
// YTS工作室製作
package admin

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrCannotDeleteLastAdmin = errors.New("cannot_delete_last_admin")
	ErrUserNotFound          = errors.New("user_not_found")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, is_active, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.IsActive, &user.CreatedAt); err == nil {
			users = append(users, user)
		}
	}

	return users, rows.Err()
}

func (s *Service) CreateUser(input CreateUserInput) (int64, error) {
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "viewer"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	result, err := s.db.Exec(`INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, input.Username, string(hash), role)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Service) UpdateUser(id string, input UpdateUserInput) error {
	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), id); err != nil {
			return err
		}
	}

	if input.Role != nil {
		if _, err := s.db.Exec("UPDATE users SET role = ? WHERE id = ?", *input.Role, id); err != nil {
			return err
		}
	}

	if input.IsActive != nil {
		if _, err := s.db.Exec("UPDATE users SET is_active = ? WHERE id = ?", *input.IsActive, id); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) DeleteUser(id string) error {
	var adminCount int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND id != ?", id).Scan(&adminCount)
	if adminCount == 0 {
		var role string
		_ = s.db.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&role)
		if role == "admin" {
			return ErrCannotDeleteLastAdmin
		}
	}

	result, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) GetBrandingSettings() (BrandingSettings, error) {
	result := BrandingSettings{
		CompanyName:  "Management Server",
		NamePosition: "right",
	}

	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_logo_path'`).Scan(&result.LogoPath)
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_name'`).Scan(&result.CompanyName)
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_name_font_size'`).Scan(&result.FontSize)
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_name_font_color'`).Scan(&result.FontColor)
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'company_name_position'`).Scan(&result.NamePosition)

	if strings.TrimSpace(result.NamePosition) == "" {
		result.NamePosition = "right"
	}

	return result, nil
}

func (s *Service) UpdateBrandingSettings(input UpdateBrandingInput) (BrandingSettings, error) {
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_name', ?, CURRENT_TIMESTAMP)`, input.CompanyName); err != nil {
		return BrandingSettings{}, err
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_name_font_size', ?, CURRENT_TIMESTAMP)`, input.FontSize); err != nil {
		return BrandingSettings{}, err
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_name_font_color', ?, CURRENT_TIMESTAMP)`, input.FontColor); err != nil {
		return BrandingSettings{}, err
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_name_position', ?, CURRENT_TIMESTAMP)`, input.NamePosition); err != nil {
		return BrandingSettings{}, err
	}

	return s.GetBrandingSettings()
}

func (s *Service) ReplaceBrandingLogo(originalFilename string, srcPath string) (BrandingSettings, error) {
	filename := "company_logo_" + strings.ReplaceAll(filepath.Base(originalFilename), " ", "_")
	filename = strings.TrimSuffix(filename, filepath.Ext(filename)) + filepath.Ext(originalFilename)
	filename = strings.ReplaceAll(filename, "..", "")

	dataPath := "../data"
	if _, err := os.Stat("./data"); err == nil {
		dataPath = "./data"
	}
	uploadsPath := filepath.Join(dataPath, "uploads")
	if err := os.MkdirAll(uploadsPath, 0755); err != nil {
		return BrandingSettings{}, err
	}

	current, _ := s.GetBrandingSettings()
	if current.LogoPath != "" {
		_ = s.removeLogoFile(current.LogoPath)
	}

	targetPath := filepath.Join(uploadsPath, filename)
	in, err := os.Open(srcPath)
	if err != nil {
		return BrandingSettings{}, err
	}
	defer in.Close()

	out, err := os.Create(targetPath)
	if err != nil {
		return BrandingSettings{}, err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return BrandingSettings{}, err
	}
	_ = out.Close()

	logoURL := "/uploads/" + filename
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('company_logo_path', ?, CURRENT_TIMESTAMP)`, logoURL); err != nil {
		return BrandingSettings{}, err
	}

	return s.GetBrandingSettings()
}

func (s *Service) DeleteBrandingLogo() error {
	current, err := s.GetBrandingSettings()
	if err != nil {
		return err
	}
	if current.LogoPath != "" {
		_ = s.removeLogoFile(current.LogoPath)
	}

	_, err = s.db.Exec(`DELETE FROM system_config WHERE config_key = 'company_logo_path'`)
	return err
}

func (s *Service) GetSecuritySettings() (SecuritySettings, error) {
	settings := SecuritySettings{
		GlobalTwoFactorEnabled: "false",
		PasswordExpiryDays:     "90",
	}

	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'global_2fa_enabled'`).Scan(&settings.GlobalTwoFactorEnabled)
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'password_expiry_days'`).Scan(&settings.PasswordExpiryDays)

	if strings.TrimSpace(settings.GlobalTwoFactorEnabled) == "" {
		settings.GlobalTwoFactorEnabled = "false"
	}
	if strings.TrimSpace(settings.PasswordExpiryDays) == "" {
		settings.PasswordExpiryDays = "90"
	}
	return settings, nil
}

func (s *Service) UpdateSecuritySettings(input UpdateSecuritySettingsInput) (SecuritySettings, error) {
	if input.GlobalTwoFactorEnabled != nil {
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('global_2fa_enabled', ?, CURRENT_TIMESTAMP)`, *input.GlobalTwoFactorEnabled); err != nil {
			return SecuritySettings{}, err
		}
	}
	if input.PasswordExpiryDays != nil {
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, updated_at) VALUES ('password_expiry_days', ?, CURRENT_TIMESTAMP)`, *input.PasswordExpiryDays); err != nil {
			return SecuritySettings{}, err
		}
	}
	return s.GetSecuritySettings()
}

func (s *Service) ListSystemConfig() ([]SystemConfigEntry, error) {
	rows, err := s.db.Query("SELECT config_key, config_value, COALESCE(description, '') FROM system_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []SystemConfigEntry{}
	for rows.Next() {
		var entry SystemConfigEntry
		if err := rows.Scan(&entry.ConfigKey, &entry.ConfigValue, &entry.Description); err == nil {
			list = append(list, entry)
		}
	}
	return list, rows.Err()
}

func (s *Service) UpdateSystemConfig(key string, input UpdateSystemConfigInput) (SystemConfigUpdateResult, error) {
	key = strings.TrimSpace(key)
	result := SystemConfigUpdateResult{ConfigKey: key}

	newValue := strings.TrimSpace(fmt.Sprint(input.Value))
	if newValue == "<nil>" || newValue == "" {
		newValue = strings.TrimSpace(fmt.Sprint(input.ConfigValue))
	}
	if newValue == "<nil>" {
		newValue = ""
	}

	var oldValue, oldDescription string
	var existed bool
	if err := s.db.QueryRow("SELECT config_value, COALESCE(description, '') FROM system_config WHERE config_key = ?", key).Scan(&oldValue, &oldDescription); err == nil {
		existed = true
	}

	description := input.Description
	if description == "" {
		description = oldDescription
	}

	_, err := s.db.Exec(`
		INSERT INTO system_config (config_key, config_value, description)
		VALUES (?, ?, ?)
		ON CONFLICT(config_key) DO UPDATE SET config_value = excluded.config_value, description = excluded.description
	`, key, newValue, description)
	if err != nil {
		return result, err
	}

	result.OldValue = oldValue
	result.OldDescription = oldDescription
	result.Existed = existed
	result.NewValue = newValue
	result.Description = description
	return result, nil
}

func (s *Service) GetSystemInfo(version, name string, startTime time.Time, licenseLocked bool, lockReason string) SystemInfo {
	return SystemInfo{
		Version:           strings.TrimSpace(version),
		Name:              strings.TrimSpace(name),
		StartTime:         startTime.Format(time.RFC3339),
		LicenseLocked:     licenseLocked,
		LicenseLockReason: strings.TrimSpace(lockReason),
	}
}

func (s *Service) removeLogoFile(logoURL string) error {
	trimmed := strings.TrimSpace(logoURL)
	if trimmed == "" {
		return nil
	}
	relative := strings.TrimPrefix(trimmed, "/")

	candidates := []string{
		filepath.Join(".", relative),
		filepath.Join("..", relative),
		filepath.Join("./data", strings.TrimPrefix(relative, "uploads/")),
		filepath.Join("../data", strings.TrimPrefix(relative, "uploads/")),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return os.Remove(candidate)
		}
	}
	return nil
}
