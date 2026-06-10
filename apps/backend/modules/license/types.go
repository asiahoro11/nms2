package license

import licensesvc "management-server/services/license"

type Status struct {
	CurrentDevices int    `json:"current_devices"`
	MaxDevices     int    `json:"max_devices"`
	MaxCameras     int    `json:"max_cameras"`
	DefaultLimit   int    `json:"default_limit"`
	TrialAvailable bool   `json:"trial_available"`
	Licenses       []Info `json:"licenses"`
}

type Info struct {
	ID          int      `json:"id"`
	LicenseKey  string   `json:"license_key"`
	LicenseType string   `json:"license_type"`
	DeviceCount int      `json:"device_count"`
	CameraCount int      `json:"camera_count"`
	Features    []string `json:"features"`
	ValidFrom   string   `json:"valid_from"`
	ValidUntil  string   `json:"valid_until"`
	IsPermanent bool     `json:"is_permanent"`
	IsActive    bool     `json:"is_active"`
	Status      string   `json:"status"`
}

type EncryptedMachineIDResult struct {
	EncryptedID string `json:"encrypted_id"`
	RawID       string `json:"raw_id"`
}

type ActivateResult struct {
	ID          int64
	Message     string
	Payload     *licensesvc.LicensePayload
	LicenseType string
	Reactivated bool
}

type TrialActivateResult struct {
	Message string
	Payload *licensesvc.LicensePayload
}

type GenerateInput struct {
	MachineID    string
	LicenseMode  string
	LicenseType  string
	DeviceCount  int
	CameraCount  int
	Years        int
	DurationDays int
	ValidUntil   string
	Features     []string
	AlertOnly    bool
}

type GenerateResult struct {
	LicenseKey   string
	LicenseMode  string
	LicenseType  string
	TypeDisplay  string
	DeviceCount  int
	CameraCount  int
	Years        int
	DurationDays int
	Features     []string
	ValidUntil   string
	IsPermanent  bool
}

type ReissueInput struct {
	OldMachineID  string
	OldLicenseKey string
}

type ReissueResult struct {
	NewLicenseKey string `json:"new_license_key"`
}

type AdminLicenseRecord struct {
	ID          int    `json:"id"`
	LicenseKey  string `json:"license_key"`
	LicenseType string `json:"license_type"`
	MaxDevices  int    `json:"max_devices"`
	ValidUntil  string `json:"valid_until"`
	IsActive    bool   `json:"is_active"`
}

type AdminLicenseList struct {
	DeviceCount int                  `json:"device_count"`
	SystemUUID  string               `json:"system_uuid"`
	Licenses    []AdminLicenseRecord `json:"licenses"`
}

type CreateLegacyLicenseInput struct {
	LicenseKey  string `json:"license_key" binding:"required"`
	LicenseType string `json:"license_type"`
	MaxDevices  int    `json:"max_devices"`
	ValidFrom   string `json:"valid_from"`
	ValidUntil  string `json:"valid_until"`
}
