// Made by YTSworks
// YTS工作室製作
package backup

import "errors"

var (
	ErrBackupPasswordTooShort       = errors.New("backup_password_too_short")
	ErrBackupPasswordRequired       = errors.New("backup_password_required")
	ErrBackupInvalidArchive         = errors.New("backup_invalid_archive")
	ErrBackupMissingDatabase        = errors.New("backup_missing_database")
	ErrBackupCheckpointIncomplete   = errors.New("backup_checkpoint_incomplete")
	ErrEncryptedBackupWrongPassword = errors.New("encrypted_backup_wrong_password")
	ErrBackupCollectorUnavailable   = errors.New("backup_collector_unavailable")
	ErrDeviceConfigBackupNotFound   = errors.New("device_config_backup_not_found")
)

type ArchiveResult struct {
	Path        string
	Filename    string
	ContentType string
}

type RestorePreparation struct {
	HasDB            bool
	ExtractedUploads int
	ExtractedConfig  bool
}

type RestoreReadiness struct {
	Ready   bool     `json:"ready"`
	Issues  []string `json:"issues"`
	WALMode string   `json:"wal_mode"`
	OS      string   `json:"os"`
}

type DeviceConfigBackup struct {
	ID        int    `json:"id"`
	DeviceID  int    `json:"device_id"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
	Size      int    `json:"size"`
}

type DeviceConfigBackupContent struct {
	ID        int
	DeviceID  int
	CreatedAt string
	Note      string
	Content   string
}
