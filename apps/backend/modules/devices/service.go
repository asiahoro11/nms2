package devices

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CountDevices() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&count)
	return count, err
}

func (s *Service) ListDevices(query ListQuery) ([]Device, int, int, error) {
	page := query.Page
	limit := query.Limit
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}

	offset := (page - 1) * limit
	baseQuery := `
		SELECT id, name, sys_name, ip_address, mac_address, device_type,
		       snmp_community, snmp_version, vendor, model, firmware,
		       is_online, last_seen, image_path, pos_x, pos_y,
		       created_at, updated_at
		FROM devices WHERE 1=1`
	countQuery := "SELECT COUNT(*) FROM devices WHERE 1=1"
	var args []interface{}

	if query.Search != "" {
		baseQuery += " AND (name LIKE ? OR ip_address LIKE ?)"
		countQuery += " AND (name LIKE ? OR ip_address LIKE ?)"
		searchTerm := "%" + query.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}
	if query.DeviceType != "" {
		baseQuery += " AND device_type = ?"
		countQuery += " AND device_type = ?"
		args = append(args, query.DeviceType)
	}
	if query.Status == "online" {
		baseQuery += " AND is_online = 1"
		countQuery += " AND is_online = 1"
	} else if query.Status == "offline" {
		baseQuery += " AND is_online = 0"
		countQuery += " AND is_online = 0"
	}

	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, 0, err
	}

	maxDevices := query.MaxDevices
	if maxDevices > 0 && total > maxDevices {
		total = maxDevices
	}
	if maxDevices > 0 && offset >= maxDevices {
		return []Device{}, total, limit, nil
	}
	if maxDevices > 0 && offset+limit > maxDevices {
		limit = maxDevices - offset
	}

	baseQuery += " ORDER BY id ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.SysName, &d.IPAddress, &d.MACAddress, &d.DeviceType,
			&d.SNMPCommunity, &d.SNMPVersion, &d.Vendor, &d.Model, &d.Firmware,
			&d.IsOnline, &d.LastSeen, &d.ImagePath, &d.PosX, &d.PosY,
			&d.CreatedAt, &d.UpdatedAt); err != nil {
			continue
		}
		devices = append(devices, d)
	}

	return devices, total, limit, nil
}

func (s *Service) GetDevice(id string) (Device, error) {
	var d Device
	err := s.db.QueryRow(`
		SELECT id, name, sys_name, ip_address, mac_address, device_type,
		       snmp_community, snmp_version, vendor, model, firmware,
		       is_online, last_seen, image_path, pos_x, pos_y,
		       created_at, updated_at
		FROM devices WHERE id = ?
	`, id).Scan(&d.ID, &d.Name, &d.SysName, &d.IPAddress, &d.MACAddress, &d.DeviceType,
		&d.SNMPCommunity, &d.SNMPVersion, &d.Vendor, &d.Model, &d.Firmware,
		&d.IsOnline, &d.LastSeen, &d.ImagePath, &d.PosX, &d.PosY,
		&d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (s *Service) ListInterfaces(id string) ([]DeviceInterface, error) {
	rows, err := s.db.Query(`
		SELECT id, device_id, if_index, if_name, if_desc, if_speed, if_mac, if_status, if_admin_status,
		       in_octets, out_octets, bandwidth_in, bandwidth_out, updated_at
		FROM device_interfaces WHERE device_id = ? ORDER BY if_index
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	interfaces := make([]DeviceInterface, 0)
	for rows.Next() {
		var (
			item                      DeviceInterface
			ifIndex                   sql.NullInt64
			ifName, ifDesc, ifMac     sql.NullString
			ifSpeed                   sql.NullInt64
			ifStatus, ifAdminStatus   sql.NullInt64
			inOctets, outOctets       sql.NullInt64
			bandwidthIn, bandwidthOut sql.NullInt64
		)

		if err := rows.Scan(&item.ID, &item.DeviceID, &ifIndex, &ifName, &ifDesc, &ifSpeed, &ifMac, &ifStatus, &ifAdminStatus,
			&inOctets, &outOctets, &bandwidthIn, &bandwidthOut, &item.UpdatedAt); err != nil {
			continue
		}

		item.IfIndex = ifIndex.Int64
		item.IfName = ifName.String
		item.IfDesc = ifDesc.String
		item.IfSpeed = ifSpeed.Int64
		item.IfMAC = ifMac.String
		item.IfAdminStatus = ifAdminStatus.Int64
		item.InOctets = inOctets.Int64
		item.OutOctets = outOctets.Int64
		item.BandwidthIn = bandwidthIn.Int64
		item.BandwidthOut = bandwidthOut.Int64
		item.IfStatus = interfaceStatusText(ifStatus)

		interfaces = append(interfaces, item)
	}

	return interfaces, nil
}

func (s *Service) CreateDevice(input DeviceInput) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO devices (name, ip_address, mac_address, device_type, snmp_community, snmp_version)
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.Name, input.IPAddress, input.MACAddress, input.DeviceType, input.SNMPCommunity, input.SNMPVersion)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Service) UpdateDevice(id string, input map[string]interface{}) (Device, []string, error) {
	before, err := s.GetDevice(id)
	if err != nil {
		return Device{}, nil, err
	}

	var updates []string
	var args []interface{}
	allowedFields := []string{"name", "ip_address", "mac_address", "device_type", "snmp_community", "snmp_version", "vendor", "model", "firmware", "pos_x", "pos_y"}
	for _, field := range allowedFields {
		if val, ok := input[field]; ok {
			updates = append(updates, field+" = ?")
			args = append(args, val)
		}
	}
	if len(updates) == 0 {
		return before, nil, ErrNoFieldsToUpdate
	}

	updates = append(updates, "updated_at = ?")
	args = append(args, time.Now().Format("2006-01-02 15:04:05"), id)

	query := "UPDATE devices SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	if _, err := s.db.Exec(query, args...); err != nil {
		return before, nil, err
	}

	return before, updates[:len(updates)-1], nil
}

func (s *Service) DeleteDevice(id string) (DeleteResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return DeleteResult{}, err
	}
	defer tx.Rollback()

	var result DeleteResult
	var macAddress sql.NullString
	err = tx.QueryRow("SELECT name, ip_address, mac_address, device_type FROM devices WHERE id = ?", id).
		Scan(&result.Name, &result.IPAddress, &macAddress, &result.DeviceType)
	if err != nil {
		return DeleteResult{}, err
	}
	result.ID = id
	result.MACAddress = macAddress.String

	_, _ = tx.Exec("DELETE FROM device_interfaces WHERE device_id = ?", id)
	_, _ = tx.Exec("DELETE FROM device_metrics WHERE device_id = ?", id)
	_, _ = tx.Exec("DELETE FROM topology_links WHERE source_device_id = ? OR target_device_id = ?", id, id)

	if _, err := tx.Exec("DELETE FROM devices WHERE id = ?", id); err != nil {
		return DeleteResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return DeleteResult{}, err
	}

	return result, nil
}

func (s *Service) SaveUploadedImage(id string, src io.Reader, originalFilename string, uploadRoot string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	filename := uuid.New().String() + ext
	uploadPath := filepath.Join(uploadRoot, filename)

	if err := os.MkdirAll(filepath.Dir(uploadPath), 0o755); err != nil {
		return "", err
	}

	dst, err := os.Create(uploadPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	imagePath := "/uploads/" + filename
	if _, err := s.db.Exec(
		"UPDATE devices SET image_path = ?, updated_at = ? WHERE id = ?",
		imagePath,
		time.Now().Format("2006-01-02 15:04:05"),
		id,
	); err != nil {
		return "", err
	}

	return imagePath, nil
}

func (s *Service) ListMetrics(id string, limit int) ([]DeviceMetric, error) {
	rows, err := s.db.Query(`
		SELECT id, device_id, cpu_usage, memory_usage, disk_usage, collected_at
		FROM device_metrics
		WHERE device_id = ?
		ORDER BY collected_at DESC
		LIMIT ?
	`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics := make([]DeviceMetric, 0)
	for rows.Next() {
		var metric DeviceMetric
		var cpu, memory, disk sql.NullFloat64
		if err := rows.Scan(&metric.ID, &metric.DeviceID, &cpu, &memory, &disk, &metric.CollectedAt); err != nil {
			return nil, err
		}
		if cpu.Valid {
			metric.CPUUsage = &cpu.Float64
		}
		if memory.Valid {
			metric.MemoryUsage = &memory.Float64
		}
		if disk.Valid {
			metric.DiskUsage = &disk.Float64
		}
		metrics = append(metrics, metric)
	}

	return metrics, rows.Err()
}

func (s *Service) BulkDelete(ids []int) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := "DELETE FROM devices WHERE id IN ("
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	result, err := tx.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return rowsAffected, nil
}

func (s *Service) BulkUpdate(input BulkUpdateInput) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	totalUpdated := 0
	for _, id := range input.IDs {
		var updates []string
		var args []interface{}

		if input.Vendor != "" {
			updates = append(updates, "vendor = ?")
			args = append(args, input.Vendor)
		}
		if input.Model != "" {
			updates = append(updates, "model = ?")
			args = append(args, input.Model)
		}
		if input.ImagePath != "" {
			updates = append(updates, "image_path = ?")
			args = append(args, input.ImagePath)
		}
		updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
		if len(updates) == 1 {
			continue
		}

		query := fmt.Sprintf("UPDATE devices SET %s WHERE id = ?", strings.Join(updates, ", "))
		args = append(args, id)
		res, err := tx.Exec(query, args...)
		if err != nil {
			continue
		}
		ra, _ := res.RowsAffected()
		totalUpdated += int(ra)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return totalUpdated, nil
}

func interfaceStatusText(status sql.NullInt64) string {
	if !status.Valid {
		return "unknown"
	}
	switch status.Int64 {
	case 1:
		return "up"
	case 2:
		return "down"
	case 3:
		return "testing"
	default:
		return "unknown"
	}
}
