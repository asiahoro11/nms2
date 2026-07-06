// Made by YTSworks
// YTS工作室製作
package notifications

import "errors"

type AlertSettingView struct {
	ID           int    `json:"id"`
	AlertType    string `json:"alert_type"`
	IsEnabled    bool   `json:"is_enabled"`
	Config       string `json:"config"`
	NeedsLicense bool   `json:"needs_license"`
	HasLicense   bool   `json:"has_license"`
}

type UpdateAlertSettingInput struct {
	IsEnabled bool   `json:"is_enabled"`
	Config    string `json:"config"`
}

type Notification struct {
	ID        int    `json:"id"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	IsRead    bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
}

var (
	ErrAlertFeatureNotLicensed = errors.New("alert feature not licensed")
	ErrAlertSettingNotFound    = errors.New("alert setting not found")
	ErrNotificationNotFound    = errors.New("notification not found")
)
