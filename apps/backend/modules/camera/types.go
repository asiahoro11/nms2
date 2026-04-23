package camera

import (
	"errors"
	"time"
)

var (
	ErrCameraNotFound      = errors.New("camera_not_found")
	ErrFFmpegNotFound      = errors.New("ffmpeg_not_found")
	ErrSnapshotRunFailed   = errors.New("camera_snapshot_failed")
	ErrSnapshotTimeout     = errors.New("camera_snapshot_timeout")
	ErrSnapshotEmpty       = errors.New("camera_snapshot_empty")
	ErrStreamUnavailable   = errors.New("camera_stream_unavailable")
	ErrStreamTimeout       = errors.New("camera_stream_timeout")
	ErrRecordingNotFound   = errors.New("recording_not_found")
	ErrRecordingNotAllowed = errors.New("recording_not_licensed")
)

type RuntimeHooks struct {
	DeviceLog func(deviceID int, severity, facility, eventCode, message string, detail map[string]interface{})
	SystemLog func(severity, service, eventCode, message string, context map[string]interface{})
}

type Camera struct {
	ID                   int        `json:"id"`
	Name                 string     `json:"name"`
	Location             string     `json:"location"`
	IPAddress            string     `json:"ip_address"`
	Port                 int        `json:"port"`
	Username             string     `json:"username"`
	RTSPUrl              string     `json:"rtsp_url"`
	ONVIFUrl             string     `json:"onvif_url"`
	Manufacturer         string     `json:"manufacturer"`
	Model                string     `json:"model"`
	Firmware             string     `json:"firmware"`
	SupportsPTZ          bool       `json:"supports_ptz"`
	IsEnabled            bool       `json:"is_enabled"`
	Status               string     `json:"status"`
	StreamType           string     `json:"stream_type"`
	MonitorDisplay       int        `json:"monitor_display"`
	MonitorOrder         int        `json:"monitor_order"`
	RecordingSource      string     `json:"recording_source"`
	RecordingBitrateKbps int        `json:"recording_bitrate_kbps"`
	LastSeen             *time.Time `json:"last_seen"`
	CreatedAt            time.Time  `json:"created_at"`
}

type CameraInput struct {
	Name                 string
	Location             string
	IPAddress            string
	Port                 int
	RTSPUrl              string
	ONVIFUrl             string
	Username             string
	PasswordEncrypted    *string
	Manufacturer         string
	Model                string
	SupportsPTZ          bool
	StreamType           string
	MonitorDisplay       int
	MonitorOrder         int
	RecordingSource      string
	RecordingBitrateKbps int
}

type MonitorCamera struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	RTSPUrl        string `json:"rtsp_url"`
	Username       string `json:"username"`
	StreamType     string `json:"stream_type"`
	MonitorDisplay int    `json:"monitor_display"`
	MonitorOrder   int    `json:"monitor_order"`
	Status         string `json:"status"`
}

type RecordingStatusItem struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	RecordingEnabled bool   `json:"recording_enabled"`
}

type RecordingRecord struct {
	ID          int64      `json:"id"`
	CameraID    int        `json:"camera_id"`
	CameraName  string     `json:"camera_name"`
	FilePath    string     `json:"file_path"`
	FileSize    int64      `json:"file_size"`
	DurationSec int        `json:"duration_sec"`
	StartedAt   time.Time  `json:"started_at"`
	EndedAt     *time.Time `json:"ended_at"`
	Label       string     `json:"label"`
	Status      string     `json:"status"`
}

type RecordingListResult struct {
	Recordings []RecordingRecord `json:"recordings"`
	Total      int               `json:"total"`
	Page       int               `json:"page"`
	Limit      int               `json:"limit"`
}

type RecordingExport struct {
	FilePath string
	FileName string
}

type RecordingStats struct {
	TotalRecordings  int    `json:"total_recordings"`
	TotalSizeBytes   int64  `json:"total_size_bytes"`
	TotalDurationSec int64  `json:"total_duration_sec"`
	ActiveRecordings int    `json:"active_recordings"`
	RecordingDir     string `json:"recording_dir"`
}

type NVRConfig struct {
	StorageDir string `json:"storage_dir"`
	UsedBytes  int64  `json:"used_bytes"`
}

type BatchCredentialUpdateInput struct {
	CameraIDs         []int
	Username          string
	Password          string
	PasswordEncrypted *string
	RTSPPort          int
	UpdateRTSP        bool
}

type BatchCredentialUpdateResult struct {
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
	Total   int `json:"total"`
}

type DiscoveredCameraCandidate struct {
	DeviceID  int    `json:"device_id"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
}

type CameraHealthResult struct {
	CameraID  int    `json:"camera_id"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"camera_port"`
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
}

type HardwareAccelState struct {
	CUDA   bool `json:"cuda"`
	QSV    bool `json:"qsv"`
	VAAPI  bool `json:"vaapi"`
	D3D11  bool `json:"d3d11va"`
	Active bool `json:"active"`
}

type SnapshotResult struct {
	CameraID int
	Data     []byte
}

type MJPEGSubscription struct {
	CameraID int
	Channel  <-chan []byte
	Cleanup  func()
}

type RecordingRuntimeStatus struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	RecordingEnabled bool   `json:"recording_enabled"`
	IsRecording      bool   `json:"is_recording"`
}

type BatchRecordingResult struct {
	CameraID int    `json:"camera_id"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}
