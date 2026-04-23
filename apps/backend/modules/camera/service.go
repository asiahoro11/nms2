package camera

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Service struct {
	db          *sql.DB
	secret      string
	hooks       RuntimeHooks
	mjpegPoolMu sync.Mutex
	mjpegPool   map[string]*mjpegStream
	nvrMu       sync.Mutex
	nvrSessions map[int]*nvrSession
}

func NewService(db *sql.DB, secret string, hooks RuntimeHooks) *Service {
	return &Service{
		db:          db,
		secret:      secret,
		hooks:       hooks,
		mjpegPool:   make(map[string]*mjpegStream),
		nvrSessions: make(map[int]*nvrSession),
	}
}

func (s *Service) writeDeviceLog(deviceID int, severity, facility, eventCode, message string, detail map[string]interface{}) {
	if s.hooks.DeviceLog != nil {
		s.hooks.DeviceLog(deviceID, severity, facility, eventCode, message, detail)
	}
}

func (s *Service) writeSystemLog(severity, service, eventCode, message string, context map[string]interface{}) {
	if s.hooks.SystemLog != nil {
		s.hooks.SystemLog(severity, service, eventCode, message, context)
	}
}

func (s *Service) EnsureSchema() error {
	stmts := []string{
		"ALTER TABLE cameras ADD COLUMN onvif_url TEXT DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN password_encrypted TEXT DEFAULT ''",
		"ALTER TABLE cameras ADD COLUMN stream_type TEXT DEFAULT 'mjpeg'",
		"ALTER TABLE cameras ADD COLUMN recording_enabled INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN monitor_display INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN monitor_order INTEGER DEFAULT 0",
		"ALTER TABLE cameras ADD COLUMN recording_source TEXT DEFAULT 'rtsp'",
		"ALTER TABLE cameras ADD COLUMN recording_bitrate_kbps INTEGER DEFAULT 0",
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}

	var migrated string
	_ = s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key = 'cam_bitrate_migrated'`).Scan(&migrated)
	if migrated != "1" {
		_, _ = s.db.Exec(`
			UPDATE cameras
			SET recording_bitrate_kbps = bandwidth_limit_kbps
			WHERE COALESCE(recording_bitrate_kbps, 0) = 0
			  AND COALESCE(bandwidth_limit_kbps, 0) > 0
		`)
		_, _ = s.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value) VALUES ('cam_bitrate_migrated', '1')`)
	}

	return nil
}

func (s *Service) LicenseEnabled() bool {
	var val string
	err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&val)
	return err == nil && val == "1"
}

func (s *Service) MaxAllowed() int {
	var val string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_max_cameras'").Scan(&val); err == nil {
		if n, convErr := strconv.Atoi(strings.TrimSpace(val)); convErr == nil && n > 0 {
			return n
		}
	}
	return 4
}

func (s *Service) CountCameras() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM cameras").Scan(&count)
	return count, err
}

func (s *Service) ListCameras() ([]Camera, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cameras := make([]Camera, 0)
	for rows.Next() {
		var cam Camera
		if err := rows.Scan(
			&cam.ID, &cam.Name, &cam.Location, &cam.IPAddress, &cam.Port, &cam.Username,
			&cam.RTSPUrl, &cam.ONVIFUrl, &cam.Manufacturer, &cam.Model, &cam.Firmware,
			&cam.SupportsPTZ, &cam.IsEnabled, &cam.Status, &cam.StreamType,
			&cam.MonitorDisplay, &cam.MonitorOrder, &cam.RecordingSource,
			&cam.RecordingBitrateKbps, &cam.LastSeen, &cam.CreatedAt,
		); err != nil {
			continue
		}
		cameras = append(cameras, cam)
	}

	return cameras, nil
}

func (s *Service) GetCamera(id string) (Camera, error) {
	var cam Camera
	err := s.db.QueryRow(`
		SELECT id, COALESCE(name,''), COALESCE(location,''), COALESCE(ip_address,''),
		       COALESCE(port,554), COALESCE(username,''), COALESCE(rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(manufacturer,''), COALESCE(model,''),
		       COALESCE(firmware,''), COALESCE(supports_ptz,0), COALESCE(is_enabled,1),
		       COALESCE(status,'unknown'), COALESCE(stream_type,'mjpeg'),
		       COALESCE(monitor_display,0), COALESCE(monitor_order,0),
		       COALESCE(recording_source,'rtsp'), COALESCE(recording_bitrate_kbps,0),
		       last_seen, created_at
		FROM cameras WHERE id = ?
	`, id).Scan(
		&cam.ID, &cam.Name, &cam.Location, &cam.IPAddress, &cam.Port, &cam.Username,
		&cam.RTSPUrl, &cam.ONVIFUrl, &cam.Manufacturer, &cam.Model, &cam.Firmware,
		&cam.SupportsPTZ, &cam.IsEnabled, &cam.Status, &cam.StreamType,
		&cam.MonitorDisplay, &cam.MonitorOrder, &cam.RecordingSource,
		&cam.RecordingBitrateKbps, &cam.LastSeen, &cam.CreatedAt,
	)
	return cam, err
}

func (s *Service) CreateCamera(input CameraInput) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO cameras (
			name, location, ip_address, port, rtsp_url, onvif_url, username, password_encrypted,
			manufacturer, model, supports_ptz, is_enabled, status, stream_type,
			monitor_display, monitor_order, recording_source, recording_bitrate_kbps
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'unknown', ?, ?, ?, ?, ?)
	`,
		input.Name, input.Location, input.IPAddress, input.Port, input.RTSPUrl, input.ONVIFUrl,
		input.Username, nullableString(input.PasswordEncrypted), input.Manufacturer, input.Model,
		input.SupportsPTZ, input.StreamType, input.MonitorDisplay, input.MonitorOrder,
		input.RecordingSource, input.RecordingBitrateKbps,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Service) UpdateCamera(id string, input CameraInput) (bool, error) {
	var (
		result sql.Result
		err    error
	)

	if input.PasswordEncrypted != nil {
		result, err = s.db.Exec(`
			UPDATE cameras SET name=?, location=?, ip_address=?,
				port=?, rtsp_url=?, onvif_url=?, username=?, password_encrypted=?,
				manufacturer=?, model=?, supports_ptz=?, stream_type=?,
				monitor_display=?, monitor_order=?, recording_source=?, recording_bitrate_kbps=? WHERE id=?`,
			input.Name, input.Location, input.IPAddress, input.Port, input.RTSPUrl, input.ONVIFUrl,
			input.Username, *input.PasswordEncrypted, input.Manufacturer, input.Model,
			input.SupportsPTZ, input.StreamType, input.MonitorDisplay, input.MonitorOrder,
			input.RecordingSource, input.RecordingBitrateKbps, id,
		)
	} else {
		result, err = s.db.Exec(`
			UPDATE cameras SET name=?, location=?, ip_address=?,
				port=?, rtsp_url=?, onvif_url=?, username=?, manufacturer=?, model=?,
				supports_ptz=?, stream_type=?, monitor_display=?, monitor_order=?, recording_source=?, recording_bitrate_kbps=? WHERE id=?`,
			input.Name, input.Location, input.IPAddress, input.Port, input.RTSPUrl, input.ONVIFUrl,
			input.Username, input.Manufacturer, input.Model, input.SupportsPTZ, input.StreamType,
			input.MonitorDisplay, input.MonitorOrder, input.RecordingSource, input.RecordingBitrateKbps, id,
		)
	}
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Service) DeleteCamera(id string) (bool, error) {
	result, err := s.db.Exec("DELETE FROM cameras WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Service) ListMonitorCameras() ([]MonitorCamera, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(name,''), COALESCE(rtsp_url,''), COALESCE(username,''),
		       COALESCE(stream_type,'mjpeg'), COALESCE(monitor_display,0),
		       COALESCE(monitor_order,0), COALESCE(status,'unknown')
		FROM cameras
		WHERE monitor_display = 1 AND is_enabled = 1
		ORDER BY monitor_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cams := make([]MonitorCamera, 0)
	for rows.Next() {
		var cam MonitorCamera
		if err := rows.Scan(
			&cam.ID,
			&cam.Name,
			&cam.RTSPUrl,
			&cam.Username,
			&cam.StreamType,
			&cam.MonitorDisplay,
			&cam.MonitorOrder,
			&cam.Status,
		); err != nil {
			return nil, err
		}
		cams = append(cams, cam)
	}

	return cams, rows.Err()
}

func (s *Service) SetMonitorDisplay(id string, monitorDisplay int, monitorOrder int) (bool, error) {
	result, err := s.db.Exec(
		"UPDATE cameras SET monitor_display = ?, monitor_order = ? WHERE id = ?",
		monitorDisplay, monitorOrder, id,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func nullableString(v *string) interface{} {
	if v == nil {
		return ""
	}
	return *v
}

func (s *Service) RecordingLicenseEnabled() bool {
	var val string
	if err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_recording_enabled'").Scan(&val); err == nil && val == "1" {
		return true
	}
	val = ""
	return s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&val) == nil && val == "1"
}

func (s *Service) ListRecordingStatusBase() ([]RecordingStatusItem, error) {
	rows, err := s.db.Query(`SELECT id, name, recording_enabled FROM cameras WHERE is_enabled=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]RecordingStatusItem, 0)
	for rows.Next() {
		var item RecordingStatusItem
		var enabled int
		if err := rows.Scan(&item.ID, &item.Name, &enabled); err != nil {
			return nil, err
		}
		item.RecordingEnabled = enabled == 1
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *Service) ListRecordings(cameraID, label, date string, page, limit int) (RecordingListResult, error) {
	offset := (page - 1) * limit
	query := `SELECT r.id, r.camera_id,
		COALESCE(NULLIF(r.camera_name, ''), c.name, '') AS camera_name,
		r.file_path, r.file_size, r.duration_sec, r.started_at, r.ended_at, r.label, r.status
		FROM camera_recordings r
		LEFT JOIN cameras c ON c.id=r.camera_id
		WHERE 1=1`
	args := []interface{}{}
	if cameraID != "" {
		query += " AND r.camera_id=?"
		args = append(args, cameraID)
	}
	if label != "" {
		query += " AND r.label LIKE ?"
		args = append(args, "%"+label+"%")
	}
	if date != "" {
		query += " AND date(r.started_at)=?"
		args = append(args, date)
	}
	query += " ORDER BY r.started_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return RecordingListResult{}, err
	}
	defer rows.Close()

	result := RecordingListResult{
		Recordings: make([]RecordingRecord, 0),
		Page:       page,
		Limit:      limit,
	}
	for rows.Next() {
		var record RecordingRecord
		if err := rows.Scan(
			&record.ID,
			&record.CameraID,
			&record.CameraName,
			&record.FilePath,
			&record.FileSize,
			&record.DurationSec,
			&record.StartedAt,
			&record.EndedAt,
			&record.Label,
			&record.Status,
		); err != nil {
			return RecordingListResult{}, err
		}
		result.Recordings = append(result.Recordings, record)
	}
	if err := rows.Err(); err != nil {
		return RecordingListResult{}, err
	}

	countQ := "SELECT COUNT(*) FROM camera_recordings WHERE 1=1"
	countArgs := []interface{}{}
	if cameraID != "" {
		countQ += " AND camera_id=?"
		countArgs = append(countArgs, cameraID)
	}
	if err := s.db.QueryRow(countQ, countArgs...).Scan(&result.Total); err != nil {
		return RecordingListResult{}, err
	}
	return result, nil
}

func (s *Service) ListRecordingDates(cameraID string) ([]string, error) {
	query := `SELECT DISTINCT date(started_at) as d FROM camera_recordings WHERE status='done'`
	args := []interface{}{}
	if cameraID != "" {
		query += " AND camera_id=?"
		args = append(args, cameraID)
	}
	query += " ORDER BY d DESC LIMIT 365"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dates := make([]string, 0)
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

func (s *Service) UpdateRecordingLabel(id, label string) (bool, error) {
	result, err := s.db.Exec(`UPDATE camera_recordings SET label=? WHERE id=?`, label, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Service) DeleteRecording(id string) (string, bool, error) {
	var filePath string
	if err := s.db.QueryRow(`SELECT file_path FROM camera_recordings WHERE id=?`, id).Scan(&filePath); err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	if _, err := s.db.Exec(`DELETE FROM camera_recordings WHERE id=?`, id); err != nil {
		return "", false, err
	}
	return filePath, true, nil
}

func (s *Service) BatchDeleteRecordings(ids []int64) ([]string, int, error) {
	filePaths := make([]string, 0, len(ids))
	deleted := 0
	for _, id := range ids {
		var filePath string
		if err := s.db.QueryRow(`SELECT file_path FROM camera_recordings WHERE id=?`, id).Scan(&filePath); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return filePaths, deleted, err
		}
		if _, err := s.db.Exec(`DELETE FROM camera_recordings WHERE id=?`, id); err != nil {
			return filePaths, deleted, err
		}
		filePaths = append(filePaths, filePath)
		deleted++
	}
	return filePaths, deleted, nil
}

func (s *Service) GetRecordingFile(id string) (string, error) {
	var filePath string
	if err := s.db.QueryRow(`SELECT file_path FROM camera_recordings WHERE id=?`, id).Scan(&filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

func (s *Service) GetRecordingExport(id string) (RecordingExport, error) {
	var (
		filePath   string
		label      string
		cameraName string
		startedAt  time.Time
		cameraID   int
	)
	err := s.db.QueryRow(`SELECT r.file_path, r.label, r.started_at, r.camera_id,
		COALESCE(NULLIF(r.camera_name, ''), c.name, '')
		FROM camera_recordings r
		LEFT JOIN cameras c ON c.id = r.camera_id
		WHERE r.id=?`, id).
		Scan(&filePath, &label, &startedAt, &cameraID, &cameraName)
	if err != nil {
		return RecordingExport{}, err
	}

	timestamp := startedAt.Format("20060102_150405")
	baseName := strings.TrimSpace(cameraName)
	if baseName == "" {
		baseName = fmt.Sprintf("camera_%d", cameraID)
	}
	safeBase := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, baseName)
	fileName := fmt.Sprintf("%s_%s.mp4", safeBase, timestamp)
	if label != "" {
		safeLabel := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
				return r
			}
			return '_'
		}, label)
		fileName = fmt.Sprintf("%s_%s_%s.mp4", safeBase, timestamp, safeLabel)
	}
	return RecordingExport{FilePath: filePath, FileName: fileName}, nil
}

func (s *Service) GetRecordingStats(recordingDir string, activeRecordings int) (RecordingStats, error) {
	stats := RecordingStats{
		ActiveRecordings: activeRecordings,
		RecordingDir:     recordingDir,
	}
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(file_size),0), COALESCE(SUM(duration_sec),0)
		FROM camera_recordings WHERE status='done'`).Scan(
		&stats.TotalRecordings,
		&stats.TotalSizeBytes,
		&stats.TotalDurationSec,
	)
	return stats, err
}

func (s *Service) GetNVRConfig(storageDir string) (NVRConfig, error) {
	cfg := NVRConfig{StorageDir: storageDir}
	err := s.db.QueryRow(`SELECT COALESCE(SUM(file_size),0) FROM camera_recordings WHERE status='done'`).Scan(&cfg.UsedBytes)
	return cfg, err
}

func (s *Service) SetNVRStorageDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "recordings"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	_, err := s.db.Exec(`INSERT INTO system_config (config_key, config_value, description)
			VALUES ('nvr_storage_dir', ?, '錄影儲存路徑')
		ON CONFLICT(config_key) DO UPDATE SET config_value=excluded.config_value`, dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (s *Service) BatchUpdateCredentials(input BatchCredentialUpdateInput) (BatchCredentialUpdateResult, error) {
	result := BatchCredentialUpdateResult{}

	ids := make([]int, 0)
	if len(input.CameraIDs) > 0 {
		ids = append(ids, input.CameraIDs...)
	} else {
		rows, err := s.db.Query(`SELECT id FROM cameras WHERE is_enabled=1`)
		if err != nil {
			return result, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				return result, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
	}

	result.Total = len(ids)
	for _, id := range ids {
		var oldRTSP, oldUser string
		var oldPort int
		if err := s.db.QueryRow(`SELECT rtsp_url, username, COALESCE(port,554) FROM cameras WHERE id=?`, id).
			Scan(&oldRTSP, &oldUser, &oldPort); err != nil {
			result.Failed++
			continue
		}

		newUser := oldUser
		if input.Username != "" {
			newUser = input.Username
		}
		port := oldPort
		if input.RTSPPort > 0 {
			port = input.RTSPPort
		}

		newRTSP := oldRTSP
		if input.UpdateRTSP {
			newRTSP = rebuildRTSPURL(oldRTSP, newUser, input.Password, port)
		}

		var err error
		if input.PasswordEncrypted != nil {
			_, err = s.db.Exec(`UPDATE cameras SET username=?, password_encrypted=?, port=?, rtsp_url=? WHERE id=?`,
				newUser, *input.PasswordEncrypted, port, newRTSP, id)
		} else {
			_, err = s.db.Exec(`UPDATE cameras SET username=?, port=?, rtsp_url=? WHERE id=?`,
				newUser, port, newRTSP, id)
		}
		if err != nil {
			result.Failed++
			continue
		}
		result.Updated++
	}

	return result, nil
}

func rebuildRTSPURL(original, username, password string, port int) string {
	if original == "" || !strings.HasPrefix(original, "rtsp://") {
		return original
	}
	inner := original[7:]
	if idx := strings.Index(inner, "@"); idx != -1 {
		inner = inner[idx+1:]
	}
	slash := strings.Index(inner, "/")
	host := inner
	path := ""
	if slash != -1 {
		host = inner[:slash]
		path = inner[slash:]
	}
	if colon := strings.LastIndex(host, ":"); colon != -1 {
		host = host[:colon]
	}
	if port > 0 {
		host = fmt.Sprintf("%s:%d", host, port)
	}
	if username != "" && password != "" {
		return fmt.Sprintf("rtsp://%s:%s@%s%s", url.QueryEscape(username), url.QueryEscape(password), host, path)
	}
	if username != "" {
		return fmt.Sprintf("rtsp://%s@%s%s", url.QueryEscape(username), host, path)
	}
	return "rtsp://" + host + path
}

func (s *Service) BuildRecordingPath(baseDir string, cameraID int, now time.Time) string {
	return filepath.Join(baseDir, fmt.Sprintf("cam%d", cameraID), now.Format("2006-01-02"))
}

func (s *Service) DiscoverCandidates(timeout time.Duration) ([]DiscoveredCameraCandidate, error) {
	rows, err := s.db.Query("SELECT id, name, ip_address FROM devices")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]DiscoveredCameraCandidate, 0)
	type deviceInfo struct {
		id        int
		name      string
		ipAddress string
	}
	devices := make([]deviceInfo, 0)
	for rows.Next() {
		var d deviceInfo
		if err := rows.Scan(&d.id, &d.name, &d.ipAddress); err == nil {
			devices = append(devices, d)
		}
	}

	existingRows, _ := s.db.Query("SELECT ip_address FROM cameras")
	existing := map[string]bool{}
	if existingRows != nil {
		defer existingRows.Close()
		for existingRows.Next() {
			var ip string
			if existingRows.Scan(&ip) == nil {
				existing[ip] = true
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 50)
	for _, d := range devices {
		if existing[d.ipAddress] || strings.TrimSpace(d.ipAddress) == "" {
			continue
		}
		wg.Add(1)
		go func(dev deviceInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(dev.ipAddress, "554"), timeout)
			if err == nil {
				conn.Close()
				mu.Lock()
				candidates = append(candidates, DiscoveredCameraCandidate{
					DeviceID:  dev.id,
					Name:      dev.name,
					IPAddress: dev.ipAddress,
					Port:      554,
				})
				mu.Unlock()
			}
		}(d)
	}
	wg.Wait()
	return candidates, nil
}

func (s *Service) ProbeHealth(id string, timeout time.Duration) (CameraHealthResult, error) {
	var result CameraHealthResult
	result.Status = "offline"
	if err := s.db.QueryRow("SELECT id, ip_address, COALESCE(port,554) FROM cameras WHERE id=?", id).
		Scan(&result.CameraID, &result.IPAddress, &result.Port); err != nil {
		return result, err
	}
	start := time.Now()
	conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(result.IPAddress, fmt.Sprintf("%d", result.Port)), timeout)
	result.LatencyMS = time.Since(start).Milliseconds()
	if dialErr == nil {
		result.Status = "online"
		conn.Close()
	}
	_, err := s.db.Exec("UPDATE cameras SET status=?, last_seen=? WHERE id=?", result.Status, time.Now(), id)
	if err != nil {
		return result, err
	}
	s.syncDevicesByCameraIP(result.IPAddress, result.Status == "online")
	return result, nil
}

func (s *Service) RecoverOfflineCameras(timeout time.Duration) ([]CameraHealthResult, error) {
	rows, err := s.db.Query(`
		SELECT id, ip_address, COALESCE(port,554)
		FROM cameras
		WHERE COALESCE(is_enabled,1)=1
		  AND COALESCE(status, 'offline')='offline'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]CameraHealthResult, 0)
	for rows.Next() {
		var result CameraHealthResult
		if err := rows.Scan(&result.CameraID, &result.IPAddress, &result.Port); err != nil {
			return nil, err
		}
		start := time.Now()
		addr := net.JoinHostPort(result.IPAddress, fmt.Sprintf("%d", result.Port))
		conn, dialErr := net.DialTimeout("tcp", addr, timeout)
		if dialErr != nil {
			continue
		}
		conn.Close()

		result.Status = "online"
		result.LatencyMS = time.Since(start).Milliseconds()
		if _, err := s.db.Exec("UPDATE cameras SET status='online', last_seen=? WHERE id=?", time.Now(), result.CameraID); err != nil {
			return nil, err
		}
		s.syncDevicesByCameraIP(result.IPAddress, true)
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *Service) syncDevicesByCameraIP(ip string, isOnline bool) {
	statusInt := 0
	if isOnline {
		statusInt = 1
	}
	if _, err := s.db.Exec(
		"UPDATE devices SET is_online=?, last_seen=datetime('now') WHERE ip_address=?",
		statusInt, ip,
	); err != nil {
		return
	}
}
