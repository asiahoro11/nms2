// Made by YTSworks
// YTS工作室製作
package dashboard

import (
	"database/sql"
	"runtime"
)

type Service struct {
	db         *sql.DB
	version    string
	systemName string
}

func NewService(db *sql.DB, version string, systemName string) *Service {
	return &Service{db: db, version: version, systemName: systemName}
}

func (s *Service) GetDashboard() (Data, error) {
	var totalDevices, onlineCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&totalDevices); err != nil {
		return Data{}, err
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM devices WHERE is_online = 1").Scan(&onlineCount); err != nil {
		return Data{}, err
	}

	deviceTypes := make(map[string]int)
	rows, err := s.db.Query("SELECT device_type, COUNT(*) as cnt FROM devices GROUP BY device_type")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var deviceType string
			var count int
			if scanErr := rows.Scan(&deviceType, &count); scanErr == nil {
				deviceTypes[deviceType] = count
			}
		}
	}

	topDevices := []TopDevice{}
	topRows, err := s.db.Query(`
		SELECT d.id,
		       CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as name,
		       d.ip_address,
		       COALESCE(SUM(di.bandwidth_in + di.bandwidth_out), 0) as total_traffic,
		       COALESCE((SELECT cpu_usage FROM device_metrics WHERE device_id = d.id ORDER BY collected_at DESC LIMIT 1), 0) as cpu,
		       COALESCE((SELECT memory_usage FROM device_metrics WHERE device_id = d.id ORDER BY collected_at DESC LIMIT 1), 0) as mem
		FROM devices d
		LEFT JOIN device_interfaces di ON d.id = di.device_id
		GROUP BY d.id
		ORDER BY total_traffic DESC
		LIMIT 5`)
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var td TopDevice
			if scanErr := topRows.Scan(&td.ID, &td.Name, &td.IPAddress, &td.TotalTraffic, &td.CPUUsage, &td.MemoryUsage); scanErr == nil {
				topDevices = append(topDevices, td)
			}
		}
	}

	events := []Event{}
	eventRows, err := s.db.Query(`
		SELECT id, device_id, event_type, severity, message, created_at
		FROM events ORDER BY created_at DESC LIMIT 10`)
	if err == nil {
		defer eventRows.Close()
		for eventRows.Next() {
			var e Event
			if scanErr := eventRows.Scan(&e.ID, &e.DeviceID, &e.EventType, &e.Severity, &e.Message, &e.CreatedAt); scanErr == nil {
				events = append(events, e)
			}
		}
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return Data{
		Version:      s.version,
		SystemName:   s.systemName,
		TotalDevices: totalDevices,
		OnlineCount:  onlineCount,
		OfflineCount: totalDevices - onlineCount,
		DeviceTypes:  deviceTypes,
		RecentEvents: events,
		TopDevices:   topDevices,
		SystemStats: SystemStats{
			MemoryUsage: float64(m.Alloc) / 1024 / 1024,
			GoRoutines:  runtime.NumGoroutine(),
		},
	}, nil
}

func (s *Service) GetTopCPU() ([]TopMetricDevice, error) {
	return s.getTopMetric("cpu_usage")
}

func (s *Service) GetTopMemory() ([]TopMetricDevice, error) {
	return s.getTopMetric("memory_usage")
}

func (s *Service) getTopMetric(metric string) ([]TopMetricDevice, error) {
	orderExpr := "cpu_usage"
	if metric == "memory_usage" {
		orderExpr = "memory_usage"
	}

	rows, err := s.db.Query(`
		SELECT d.id,
		       CASE WHEN d.is_name_custom = 1 THEN d.name ELSE COALESCE(NULLIF(d.sys_name, ''), d.name) END as name,
		       d.ip_address,
		       COALESCE((SELECT ` + orderExpr + ` FROM device_metrics WHERE device_id = d.id ORDER BY collected_at DESC LIMIT 1), 0) as primary_val,
		       COALESCE((SELECT SUM(bandwidth_in + bandwidth_out) FROM device_interfaces WHERE device_id = d.id), 0) as traffic,
		       COALESCE((SELECT cpu_usage FROM device_metrics WHERE device_id = d.id ORDER BY collected_at DESC LIMIT 1), 0) as cpu,
		       COALESCE((SELECT memory_usage FROM device_metrics WHERE device_id = d.id ORDER BY collected_at DESC LIMIT 1), 0) as mem
		FROM devices d
		WHERE d.is_online = 1
		ORDER BY primary_val DESC
		LIMIT 5`)
	if err != nil {
		return []TopMetricDevice{}, nil
	}
	defer rows.Close()

	list := []TopMetricDevice{}
	for rows.Next() {
		var td TopMetricDevice
		if scanErr := rows.Scan(&td.ID, &td.Name, &td.IPAddress, &td.Value, &td.TotalTraffic, &td.CPUUsage, &td.MemoryUsage); scanErr == nil {
			list = append(list, td)
		}
	}
	return list, rows.Err()
}
