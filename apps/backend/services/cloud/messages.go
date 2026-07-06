// Made by YTSworks
// YTS工作室製作
package cloud

import (
	"encoding/json"
	"fmt"
	"time"
)

// --- Agent → Cloud Messages ---

// HeartbeatMessage is sent by the agent to the cloud every heartbeat interval.
type HeartbeatMessage struct {
	SiteID      string    `json:"site_id"`
	SiteName    string    `json:"site_name"`
	Version     string    `json:"version"`
	DeviceCount int       `json:"device_count"`
	OnlineCount int       `json:"online_count"`
	AlertCount  int       `json:"alert_count"`
	CameraCount int       `json:"camera_count,omitempty"`
	CPUAvg      float64   `json:"cpu_avg"`
	MemoryAvg   float64   `json:"memory_avg"`
	UptimeSec   int64     `json:"uptime_seconds"`
	Timestamp   time.Time `json:"timestamp"`
}

// DeviceStatusMessage is sent when a device goes online or offline.
type DeviceStatusMessage struct {
	SiteID     string    `json:"site_id"`
	DeviceName string    `json:"device_name"`
	DeviceIP   string    `json:"device_ip"`
	DeviceType string    `json:"device_type"`
	IsOnline   bool      `json:"is_online"`
	CPUPct     float64   `json:"cpu_pct,omitempty"`
	MemoryPct  float64   `json:"memory_pct,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// AlertMessage is sent when an alert triggers on the agent.
type AlertMessage struct {
	SiteID     string    `json:"site_id"`
	DeviceName string    `json:"device_name"`
	DeviceIP   string    `json:"device_ip"`
	AlertType  string    `json:"alert_type"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

// DeviceMetric holds metrics for a single device within a batch.
type DeviceMetric struct {
	DeviceName string  `json:"device_name"`
	DeviceIP   string  `json:"device_ip"`
	IsOnline   bool    `json:"is_online"`
	CPUPct     float64 `json:"cpu_pct"`
	MemoryPct  float64 `json:"memory_pct"`
	DiskPct    float64 `json:"disk_pct"`
}

// MetricBatchMessage is sent periodically with aggregated metrics.
type MetricBatchMessage struct {
	SiteID    string         `json:"site_id"`
	Timestamp time.Time      `json:"timestamp"`
	Devices   []DeviceMetric `json:"devices"`
}

// CameraStatusMessage reports camera state changes.
type CameraStatusMessage struct {
	SiteID     string    `json:"site_id"`
	CameraName string    `json:"camera_name"`
	CameraIP   string    `json:"camera_ip"`
	Status     string    `json:"status"`
	StreamType string    `json:"stream_type"`
	Timestamp  time.Time `json:"timestamp"`
}

// --- Cloud → Agent Messages ---

// CommandMessage is a command sent from the cloud to an agent.
type CommandMessage struct {
	CommandID   string          `json:"command_id"`
	CommandType string          `json:"command_type"`
	Payload     json.RawMessage `json:"payload"`
	Timestamp   time.Time       `json:"timestamp"`
}

// CommandResponseMessage is the agent's response to a cloud command.
type CommandResponseMessage struct {
	CommandID string          `json:"command_id"`
	SiteID    string          `json:"site_id"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// --- MQTT Topic Helpers ---

const topicPrefix = "pronms"

func TopicHeartbeat(siteID string) string {
	return fmt.Sprintf("%s/%s/heartbeat", topicPrefix, siteID)
}

func TopicDeviceStatus(siteID string) string {
	return fmt.Sprintf("%s/%s/devices/status", topicPrefix, siteID)
}

func TopicDeviceMetrics(siteID string) string {
	return fmt.Sprintf("%s/%s/devices/metrics", topicPrefix, siteID)
}

func TopicAlerts(siteID string) string {
	return fmt.Sprintf("%s/%s/alerts", topicPrefix, siteID)
}

func TopicCameraStatus(siteID string) string {
	return fmt.Sprintf("%s/%s/cameras/status", topicPrefix, siteID)
}

func TopicCommands(siteID string) string {
	return fmt.Sprintf("%s/%s/commands", topicPrefix, siteID)
}

func TopicCommandResponse(siteID string) string {
	return fmt.Sprintf("%s/%s/commands/response", topicPrefix, siteID)
}

func TopicConfig(siteID string) string {
	return fmt.Sprintf("%s/%s/config", topicPrefix, siteID)
}
