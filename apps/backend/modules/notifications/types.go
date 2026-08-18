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
	ID             int    `json:"id"`
	Severity       string `json:"severity"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	IsRead         bool   `json:"is_read"`
	CreatedAt      string `json:"created_at"`
	Status         string `json:"status"`
	AssignedTo     string `json:"assigned_to"`
	AcknowledgedBy string `json:"acknowledged_by"`
	AcknowledgedAt string `json:"acknowledged_at"`
	ResolvedAt     string `json:"resolved_at"`
	ResolutionNote string `json:"resolution_note"`
	DeviceID       int    `json:"device_id"`
	Category       string `json:"category"`
}

type UpdateWorkflowInput struct {
	Status         string `json:"status"`
	AssignedTo     string `json:"assigned_to"`
	ResolutionNote string `json:"resolution_note"`
}

type WorkflowQuery struct {
	Status   string
	Severity string
	Search   string
	Page     int
	Limit    int
}

var (
	ErrAlertFeatureNotLicensed = errors.New("alert feature not licensed")
	ErrAlertSettingNotFound    = errors.New("alert setting not found")
	ErrNotificationNotFound    = errors.New("notification not found")
)
