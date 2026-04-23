package admin

type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type CreateUserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"`
}

type UpdateUserInput struct {
	Password *string `json:"password"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

type BrandingSettings struct {
	LogoPath     string `json:"logo_path"`
	CompanyName  string `json:"company_name"`
	FontSize     string `json:"font_size,omitempty"`
	FontColor    string `json:"font_color,omitempty"`
	NamePosition string `json:"name_position,omitempty"`
}

type UpdateBrandingInput struct {
	CompanyName  string `json:"company_name"`
	FontSize     string `json:"font_size"`
	FontColor    string `json:"font_color"`
	NamePosition string `json:"name_position"`
}

type SecuritySettings struct {
	GlobalTwoFactorEnabled string `json:"global_2fa_enabled"`
	PasswordExpiryDays     string `json:"password_expiry_days"`
}

type UpdateSecuritySettingsInput struct {
	GlobalTwoFactorEnabled *string `json:"global_2fa_enabled"`
	PasswordExpiryDays     *string `json:"password_expiry_days"`
}

type SystemConfigEntry struct {
	ConfigKey   string `json:"config_key"`
	ConfigValue string `json:"config_value"`
	Description string `json:"description"`
}

type UpdateSystemConfigInput struct {
	Value       interface{} `json:"value"`
	ConfigValue interface{} `json:"config_value"`
	Description string      `json:"description"`
}

type SystemConfigUpdateResult struct {
	ConfigKey      string
	OldValue       string
	OldDescription string
	Existed        bool
	NewValue       string
	Description    string
}

type SystemInfo struct {
	Version   string `json:"version"`
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
}
