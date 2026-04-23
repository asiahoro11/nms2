package topology

import "database/sql"

func (s *Service) LoadDeviceInterfaces(deviceID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT id, if_index, COALESCE(if_name, if_desc, ''),
		       COALESCE(if_speed, 0), if_status, COALESCE(if_mac, '')
		FROM device_interfaces
		WHERE device_id = ?
		ORDER BY if_index
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	interfaces := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, ifIndex int
		var ifName, ifMAC string
		var ifSpeed int64
		var ifStatus sql.NullInt64

		if err := rows.Scan(&id, &ifIndex, &ifName, &ifSpeed, &ifStatus, &ifMAC); err != nil {
			continue
		}

		statusText := "unknown"
		if ifStatus.Valid {
			switch ifStatus.Int64 {
			case 1:
				statusText = "up"
			case 2:
				statusText = "down"
			case 3:
				statusText = "testing"
			}
		}

		interfaces = append(interfaces, map[string]interface{}{
			"id":            id,
			"if_index":      ifIndex,
			"if_name":       ifName,
			"if_speed":      ifSpeed,
			"if_speed_text": formatSpeed(ifSpeed),
			"if_status":     statusText,
			"if_mac":        ifMAC,
		})
	}

	return interfaces, nil
}
