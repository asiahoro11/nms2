// Made by YTSworks
// YTS工作室製作
package cloud

import (
	"log"
	"time"
)

// PublishHeartbeat collects site stats from the local database and publishes a heartbeat.
func (c *Connector) PublishHeartbeat() {
	msg := HeartbeatMessage{
		SiteID:    c.config.SiteID,
		SiteName:  c.config.SiteName,
		Timestamp: time.Now().UTC(),
	}

	// Collect device counts
	c.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&msg.DeviceCount)
	c.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE is_online = 1`).Scan(&msg.OnlineCount)

	// Collect camera count (if table exists)
	c.db.QueryRow(`SELECT COUNT(*) FROM cameras WHERE is_enabled = 1`).Scan(&msg.CameraCount)

	// Collect alert count (events in last 24h)
	c.db.QueryRow(`SELECT COUNT(*) FROM events WHERE created_at > datetime('now', '-1 day')`).Scan(&msg.AlertCount)

	// Agent version
	var version string
	if err := c.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'system_version'`).Scan(&version); err == nil {
		msg.Version = version
	}

	c.publish(TopicHeartbeat(c.config.SiteID), msg)
}

// HandleDeviceStatusChange is called by the pinger service when a device goes online/offline.
// This is the callback function wired into pinger.OnStatusChange.
func (c *Connector) HandleDeviceStatusChange(deviceID int, deviceIP, deviceName string, isOnline bool) {
	msg := DeviceStatusMessage{
		SiteID:     c.config.SiteID,
		DeviceName: deviceName,
		DeviceIP:   deviceIP,
		IsOnline:   isOnline,
		Timestamp:  time.Now().UTC(),
	}

	// Try to get device type
	var deviceType string
	if err := c.db.QueryRow(`SELECT COALESCE(device_type, 'unknown') FROM devices WHERE id = ?`, deviceID).Scan(&deviceType); err == nil {
		msg.DeviceType = deviceType
	}

	c.publish(TopicDeviceStatus(c.config.SiteID), msg)
	log.Printf("[Cloud] Published device status: %s (%s) → %v", deviceName, deviceIP, isOnline)
}

// HandleAlert is called by the alert manager when an alert dispatches.
// This is the callback function wired into alert.CloudAlertHook.
func (c *Connector) HandleAlert(message string) {
	msg := AlertMessage{
		SiteID:    c.config.SiteID,
		AlertType: "status_change",
		Severity:  "warning",
		Message:   message,
		Timestamp: time.Now().UTC(),
	}

	c.publish(TopicAlerts(c.config.SiteID), msg)
	log.Printf("[Cloud] Published alert: %s", message)
}

// PublishMetricBatch collects device metrics and publishes a batch.
func (c *Connector) PublishMetricBatch() {
	rows, err := c.db.Query(`
		SELECT
			COALESCE(d.sys_name, d.name, d.ip_address) as device_name,
			d.ip_address,
			d.is_online,
			COALESCE(m.cpu_usage, 0),
			COALESCE(m.memory_usage, 0),
			COALESCE(m.disk_usage, 0)
		FROM devices d
		LEFT JOIN device_metrics m ON d.id = m.device_id
		ORDER BY d.id ASC
		LIMIT 100
	`)
	if err != nil {
		log.Printf("[Cloud] Failed to query metrics: %v", err)
		return
	}
	defer rows.Close()

	var devices []DeviceMetric
	for rows.Next() {
		var dm DeviceMetric
		if err := rows.Scan(&dm.DeviceName, &dm.DeviceIP, &dm.IsOnline, &dm.CPUPct, &dm.MemoryPct, &dm.DiskPct); err != nil {
			continue
		}
		devices = append(devices, dm)
	}

	if len(devices) == 0 {
		return
	}

	msg := MetricBatchMessage{
		SiteID:    c.config.SiteID,
		Timestamp: time.Now().UTC(),
		Devices:   devices,
	}

	c.publish(TopicDeviceMetrics(c.config.SiteID), msg)
	log.Printf("[Cloud] Published metric batch: %d devices", len(devices))
}

// PublishCameraStatus publishes camera status to the cloud.
func (c *Connector) PublishCameraStatus(cameraName, cameraIP, status, streamType string) {
	msg := CameraStatusMessage{
		SiteID:     c.config.SiteID,
		CameraName: cameraName,
		CameraIP:   cameraIP,
		Status:     status,
		StreamType: streamType,
		Timestamp:  time.Now().UTC(),
	}

	c.publish(TopicCameraStatus(c.config.SiteID), msg)
}

// SendCommandResponse sends a response back to the cloud for a command.
func (c *Connector) SendCommandResponse(resp CommandResponseMessage) {
	resp.SiteID = c.config.SiteID
	resp.Timestamp = time.Now().UTC()
	c.publish(TopicCommandResponse(c.config.SiteID), resp)
}
