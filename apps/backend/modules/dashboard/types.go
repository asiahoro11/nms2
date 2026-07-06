// Made by YTSworks
// YTS工作室製作
package dashboard

type Data struct {
	Version      string         `json:"version"`
	SystemName   string         `json:"system_name"`
	TotalDevices int            `json:"total_devices"`
	OnlineCount  int            `json:"online_count"`
	OfflineCount int            `json:"offline_count"`
	DeviceTypes  map[string]int `json:"device_types"`
	RecentEvents []Event        `json:"recent_events"`
	TopDevices   []TopDevice    `json:"top_devices"`
	SystemStats  SystemStats    `json:"system_stats"`
}

type TopDevice struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	IPAddress    string  `json:"ip_address"`
	TotalTraffic int64   `json:"total_traffic"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
}

type TopMetricDevice struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	IPAddress    string  `json:"ip_address"`
	Value        float64 `json:"value"`
	TotalTraffic int64   `json:"total_traffic"`
	CPUUsage     float64 `json:"cpu_usage"`
	MemoryUsage  float64 `json:"memory_usage"`
}

type SystemStats struct {
	Uptime      string  `json:"uptime"`
	MemoryUsage float64 `json:"memory_usage"`
	GoRoutines  int     `json:"go_routines"`
}

type Event struct {
	ID        int    `json:"id"`
	DeviceID  *int   `json:"device_id"`
	EventType string `json:"event_type"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}
