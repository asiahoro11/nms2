package camera

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mjpegStream struct {
	service     *Service
	mu          sync.Mutex
	clients     map[chan []byte]struct{}
	cmd         *exec.Cmd
	cancelFn    func()
	id          string
	poolKey     string
	bitrateKbps int
	bytesSent   int64
	lastAlertAt time.Time
	lastFrame   []byte
}

type nvrSession struct {
	cmd       *exec.Cmd
	startedAt time.Time
	filePath  string
	recID     int64
	done      chan struct{}
	cancel    chan struct{}
}

type ffmpegLogWriter struct {
	cameraID int
	buf      []byte
}

func (w *ffmpegLogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:idx]), "\r")
		if line != "" {
			log.Printf("[NVR ffmpeg cam%d] %s", w.cameraID, line)
		}
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
}

func (s *Service) setCameraStatus(id, status string) {
	if s.db != nil {
		_, _ = s.db.Exec("UPDATE cameras SET status=?, last_seen=? WHERE id=?", status, time.Now(), id)
	}
}

func (s *Service) CaptureSnapshot(id string, timeout time.Duration) (SnapshotResult, error) {
	var rtspURL, username, passwordEncrypted string
	err := s.db.QueryRow(`SELECT rtsp_url, username, COALESCE(password_encrypted,'') FROM cameras WHERE id = ?`, id).
		Scan(&rtspURL, &username, &passwordEncrypted)
	if err == sql.ErrNoRows {
		return SnapshotResult{}, ErrCameraNotFound
	}
	if err != nil {
		return SnapshotResult{}, err
	}

	password := ResolvePassword(s.secret, passwordEncrypted, rtspURL)
	rawURL := rtspURL
	cleanedURL := RTSPCleanURL(rtspURL)
	rtspURL = BuildAuthURL(cleanedURL, username, password)
	log.Printf("[Snapshot] cam=%s rawURL=%q cleaned=%q user=%q passEmpty=%v passEncEmpty=%v finalURL=%q",
		id, rawURL, cleanedURL, username, password == "", passwordEncrypted == "", rtspURL)

	ffmpegBin := FindFFmpegBin()
	cameraID, _ := strconv.Atoi(id)
	if ffmpegBin == "" {
		s.setCameraStatus(id, "offline")
		if cameraID > 0 {
			s.writeDeviceLog(cameraID, "error", "camera-stream", "camera snapshot failed", "Camera snapshot failed", map[string]interface{}{"reason": "ffmpeg_not_found"})
		}
		s.writeSystemLog("error", "camera", "camera_snapshot_failed", "camera snapshot failed: ffmpeg not found", map[string]interface{}{"camera_id": id, "reason": "ffmpeg_not_found"})
		return SnapshotResult{}, ErrFFmpegNotFound
	}

	ProbeGPU(ffmpegBin)
	cmd := exec.Command(ffmpegBin, BuildFFmpegSnapshotArgs(rtspURL)...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case runErr := <-done:
		if runErr != nil {
			log.Printf("[Camera] ffmpeg snapshot failed id=%s: %v | %s", id, runErr, stderr.String())
			s.setCameraStatus(id, "offline")
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "error", "camera-stream", "camera snapshot failed", "Camera snapshot failed", map[string]interface{}{"reason": "ffmpeg_run_failed", "error": runErr.Error()})
			}
			s.writeSystemLog("error", "camera", "camera_snapshot_failed", "camera snapshot failed", map[string]interface{}{"camera_id": id, "reason": "ffmpeg_run_failed", "error": runErr.Error()})
			return SnapshotResult{}, fmt.Errorf("%w: %v", ErrSnapshotRunFailed, runErr)
		}
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		log.Printf("[Camera] ffmpeg snapshot timeout id=%s", id)
		s.setCameraStatus(id, "offline")
		if cameraID > 0 {
			s.writeDeviceLog(cameraID, "warning", "camera-stream", "camera snapshot timeout", "Camera snapshot timeout", map[string]interface{}{"reason": "snapshot_timeout"})
		}
		s.writeSystemLog("warning", "camera", "camera_snapshot_timeout", "camera snapshot timeout", map[string]interface{}{"camera_id": id, "reason": "snapshot_timeout"})
		return SnapshotResult{}, ErrSnapshotTimeout
	}

	data := out.Bytes()
	if len(data) == 0 {
		log.Printf("[Camera] ffmpeg returned empty frame id=%s | %s", id, stderr.String())
		s.setCameraStatus(id, "offline")
		if cameraID > 0 {
			s.writeDeviceLog(cameraID, "warning", "camera-stream", "camera snapshot empty", "Camera snapshot empty", map[string]interface{}{"reason": "empty_frame"})
		}
		s.writeSystemLog("warning", "camera", "camera_snapshot_empty", "camera snapshot returned empty frame", map[string]interface{}{"camera_id": id, "reason": "empty_frame"})
		return SnapshotResult{}, ErrSnapshotEmpty
	}

	s.setCameraStatus(id, "online")
	if cameraID > 0 {
		s.writeDeviceLog(cameraID, "info", "camera-stream", "camera snapshot success", "Camera snapshot success", map[string]interface{}{"bytes": len(data)})
	}
	return SnapshotResult{CameraID: cameraID, Data: data}, nil
}

func newMjpegStream(service *Service, id, poolKey string, bitrateKbps int) *mjpegStream {
	return &mjpegStream{
		service:     service,
		clients:     make(map[chan []byte]struct{}),
		id:          id,
		poolKey:     poolKey,
		bitrateKbps: bitrateKbps,
	}
}

func (s *mjpegStream) addClient() chan []byte {
	ch := make(chan []byte, 8)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	var cached []byte
	if len(s.lastFrame) > 0 {
		cached = append([]byte(nil), s.lastFrame...)
	}
	s.mu.Unlock()
	if len(cached) > 0 {
		select {
		case ch <- cached:
		default:
		}
	}
	return ch
}

func (s *mjpegStream) removeClient(ch chan []byte) {
	s.mu.Lock()
	delete(s.clients, ch)
	count := len(s.clients)
	var fn func()
	if count == 0 {
		fn = s.cancelFn
		s.cancelFn = nil
	}
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (s *mjpegStream) broadcast(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFrame = append(s.lastFrame[:0], frame...)
	s.bytesSent += int64(len(frame))
	for ch := range s.clients {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (s *Service) getMjpegStream(id, rtspURL, ffmpegBin string, bitrateKbps int) *mjpegStream {
	ProbeGPU(ffmpegBin)
	poolKey := fmt.Sprintf("%s@%d", id, bitrateKbps)

	s.mjpegPoolMu.Lock()
	stream, ok := s.mjpegPool[poolKey]
	if ok {
		stream.mu.Lock()
		stale := len(stream.clients) == 0 && stream.cancelFn == nil
		stream.mu.Unlock()
		if !stale {
			s.mjpegPoolMu.Unlock()
			return stream
		}
		delete(s.mjpegPool, poolKey)
	}
	stream = newMjpegStream(s, id, poolKey, bitrateKbps)
	s.mjpegPool[poolKey] = stream
	s.mjpegPoolMu.Unlock()

	go s.runMjpegFFmpeg(rtspURL, ffmpegBin, stream)
	return stream
}

func (s *Service) runMjpegFFmpeg(rtspURL, ffmpegBin string, stream *mjpegStream) {
	boundary := "mjpegframe"
	camID := stream.id
	cameraID, _ := strconv.Atoi(camID)

	for {
		stream.mu.Lock()
		count := len(stream.clients)
		stream.mu.Unlock()
		if count == 0 {
			s.mjpegPoolMu.Lock()
			delete(s.mjpegPool, stream.poolKey)
			s.mjpegPoolMu.Unlock()
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "info", "camera-stream", "camera stream worker stopped", "Camera stream worker stopped", map[string]interface{}{"pool_key": stream.poolKey, "reason": "no_viewers"})
			}
			s.writeSystemLog("info", "camera", "camera_stream_worker_stopped", "camera stream worker stopped", map[string]interface{}{"camera_id": camID, "pool_key": stream.poolKey, "reason": "no_viewers"})
			return
		}

		pr, pw, err := os.Pipe()
		if err != nil {
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "error", "camera-stream", "camera stream worker failed", "Camera stream worker failed", map[string]interface{}{"pool_key": stream.poolKey, "reason": "pipe_create_failed", "error": err.Error()})
			}
			s.writeSystemLog("error", "camera", "camera_stream_worker_failed", "camera stream worker failed", map[string]interface{}{"camera_id": camID, "pool_key": stream.poolKey, "reason": "pipe_create_failed", "error": err.Error()})
			time.Sleep(2 * time.Second)
			continue
		}

		cmd := exec.Command(ffmpegBin, BuildFFmpegStreamArgs(rtspURL, boundary, 5, 5, stream.bitrateKbps)...)
		cmd.Stdout = pw
		cmd.Stderr = nil

		cancelCh := make(chan struct{})
		var cancelOnce sync.Once
		stream.mu.Lock()
		stream.cmd = cmd
		stream.cancelFn = func() { cancelOnce.Do(func() { close(cancelCh) }) }
		stream.mu.Unlock()

		if startErr := cmd.Start(); startErr != nil {
			_ = pw.Close()
			_ = pr.Close()
			s.setCameraStatus(camID, "offline")
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "error", "camera-stream", "camera stream worker start failed", "Camera stream worker start failed", map[string]interface{}{"pool_key": stream.poolKey, "reason": "ffmpeg_start_failed", "error": startErr.Error()})
			}
			s.writeSystemLog("error", "camera", "camera_stream_worker_failed", "camera stream worker start failed", map[string]interface{}{"camera_id": camID, "pool_key": stream.poolKey, "reason": "ffmpeg_start_failed", "error": startErr.Error()})
			time.Sleep(2 * time.Second)
			continue
		}
		_ = pw.Close()

		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()

		if stream.bitrateKbps > 0 {
			go func() {
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()
				var lastBytes int64
				for {
					select {
					case <-cancelCh:
						return
					case <-done:
						return
					case <-ticker.C:
						stream.mu.Lock()
						cur := stream.bytesSent
						alertAt := stream.lastAlertAt
						stream.mu.Unlock()

						delta := cur - lastBytes
						lastBytes = cur
						actualKbps := int(delta * 8 / 10 / 1024)

						if actualKbps > stream.bitrateKbps && time.Since(alertAt) > 5*time.Minute {
							stream.mu.Lock()
							stream.lastAlertAt = time.Now()
							stream.mu.Unlock()

							if s.db != nil {
								title := fmt.Sprintf("Camera bitrate warning: ID %s", camID)
								message := fmt.Sprintf("Camera ID %s MJPEG bitrate exceeded limit (%d kbps > %d kbps)", camID, actualKbps, stream.bitrateKbps)
								_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS notifications (
									id INTEGER PRIMARY KEY AUTOINCREMENT,
									severity TEXT NOT NULL DEFAULT 'warning',
									title TEXT NOT NULL,
									message TEXT NOT NULL,
									is_read BOOLEAN DEFAULT 0,
									created_at DATETIME DEFAULT CURRENT_TIMESTAMP
								)`)
								_, _ = s.db.Exec(`INSERT INTO notifications (severity, title, message) VALUES (?, ?, ?)`, "warning", title, message)
							}
						}
					}
				}
			}()
		}

		firstFrame := true
		go func() {
			defer pr.Close()
			buf := make([]byte, 512*1024)
			var accumulated []byte
			sep := []byte("\r\n--" + boundary + "\r\n")
			sepStart := []byte("--" + boundary + "\r\n")

			for {
				n, readErr := pr.Read(buf)
				if n > 0 {
					accumulated = append(accumulated, buf[:n]...)
					for {
						idx := -1
						if len(accumulated) > 0 {
							idx = IndexBytes(accumulated, sep)
						}
						if idx < 0 {
							break
						}
						chunk := accumulated[:idx]
						accumulated = accumulated[idx+len(sep):]
						if frame := ExtractJPEG(chunk); len(frame) > 0 {
							if firstFrame {
								firstFrame = false
								s.setCameraStatus(camID, "online")
							}
							stream.broadcast(frame)
						}
					}
				}
				if readErr != nil {
					break
				}
			}
			if frame := ExtractJPEG(StripMultipartHeader(accumulated, sepStart)); len(frame) > 0 {
				if firstFrame {
					firstFrame = false
					s.setCameraStatus(camID, "online")
				}
				stream.broadcast(frame)
			}
		}()

		select {
		case <-done:
			s.setCameraStatus(camID, "offline")
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "warning", "camera-stream", "camera stream worker recycled", "Camera stream worker recycled", map[string]interface{}{"pool_key": stream.poolKey, "reason": "ffmpeg_exit", "restart_in_s": 2})
			}
			s.writeSystemLog("warning", "camera", "camera_stream_worker_recycled", "camera stream worker recycled", map[string]interface{}{"camera_id": camID, "pool_key": stream.poolKey, "reason": "ffmpeg_exit", "restart_in_s": 2})
			time.Sleep(2 * time.Second)
		case <-cancelCh:
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = pr.Close()
			s.mjpegPoolMu.Lock()
			delete(s.mjpegPool, stream.poolKey)
			s.mjpegPoolMu.Unlock()
			return
		}
	}
}

func (s *Service) SubscribeMJPEG(id string) (*MJPEGSubscription, error) {
	var rtspURL, username, passwordEnc string
	var bitrateKbps int
	err := s.db.QueryRow(`SELECT rtsp_url, username, COALESCE(password_encrypted,''), COALESCE(recording_bitrate_kbps,0) FROM cameras WHERE id=?`, id).
		Scan(&rtspURL, &username, &passwordEnc, &bitrateKbps)
	if err == sql.ErrNoRows {
		return nil, ErrCameraNotFound
	}
	if err != nil {
		return nil, err
	}

	password := ResolvePassword(s.secret, passwordEnc, rtspURL)
	rtspURL = BuildAuthURL(RTSPCleanURL(rtspURL), username, password)

	ffmpegBin := FindFFmpegBin()
	cameraID, _ := strconv.Atoi(id)
	if ffmpegBin == "" {
		if cameraID > 0 {
			s.writeDeviceLog(cameraID, "error", "camera-stream", "camera live preview failed", "Camera live preview failed", map[string]interface{}{"reason": "ffmpeg_not_found"})
		}
		s.writeSystemLog("error", "camera", "camera_stream_failed", "camera live preview failed: ffmpeg not found", map[string]interface{}{"camera_id": id, "reason": "ffmpeg_not_found"})
		return nil, ErrFFmpegNotFound
	}

	stream := s.getMjpegStream(id, rtspURL, ffmpegBin, bitrateKbps)
	ch := stream.addClient()
	return &MJPEGSubscription{
		CameraID: cameraID,
		Channel:  ch,
		Cleanup:  func() { stream.removeClient(ch) },
	}, nil
}

func (s *Service) WaitForMJPEGFirstFrame(id string, sub *MJPEGSubscription, timeout time.Duration) ([]byte, error) {
	select {
	case frame, ok := <-sub.Channel:
		if !ok || len(frame) == 0 {
			if sub.CameraID > 0 {
				s.writeDeviceLog(sub.CameraID, "error", "camera-stream", "camera live preview unavailable", "Camera live preview unavailable", map[string]interface{}{"reason": "stream_unavailable"})
			}
			s.writeSystemLog("error", "camera", "camera_stream_unavailable", "camera live preview unavailable", map[string]interface{}{"camera_id": id, "reason": "stream_unavailable"})
			return nil, ErrStreamUnavailable
		}
		if sub.CameraID > 0 {
			s.writeDeviceLog(sub.CameraID, "info", "camera-stream", "camera live preview started", "Camera live preview started", map[string]interface{}{"bytes": len(frame)})
		}
		s.writeSystemLog("info", "camera", "camera_stream_started", "camera live preview started", map[string]interface{}{"camera_id": id, "bytes": len(frame)})
		return frame, nil
	case <-time.After(timeout):
		s.setCameraStatus(id, "offline")
		if sub.CameraID > 0 {
			s.writeDeviceLog(sub.CameraID, "warning", "camera-stream", "camera live preview timeout", "Camera live preview timeout", map[string]interface{}{"reason": "stream_timeout"})
		}
		s.writeSystemLog("warning", "camera", "camera_stream_timeout", "camera live preview timeout", map[string]interface{}{"camera_id": id, "reason": "stream_timeout"})
		return nil, ErrStreamTimeout
	}
}

func (s *Service) StorageDir() string {
	var val string
	if err := s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key='nvr_storage_dir'`).Scan(&val); err == nil && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return "recordings"
}

func (s *Service) SetRecording(cameraID int, enabled bool) error {
	if enabled {
		return s.nvrStart(cameraID)
	}
	s.nvrStop(cameraID)
	return nil
}

func (s *Service) BatchSetRecording(cameraIDs []int, enabled bool) []BatchRecordingResult {
	results := make([]BatchRecordingResult, 0, len(cameraIDs))
	for _, id := range cameraIDs {
		err := s.SetRecording(id, enabled)
		item := BatchRecordingResult{CameraID: id, OK: err == nil}
		if err != nil {
			item.Error = err.Error()
		}
		results = append(results, item)
	}
	return results
}

func (s *Service) ListRecordingStatusRuntime() ([]RecordingRuntimeStatus, int, error) {
	baseItems, err := s.ListRecordingStatusBase()
	if err != nil {
		return nil, 0, err
	}
	list := make([]RecordingRuntimeStatus, 0, len(baseItems))
	for _, base := range baseItems {
		item := RecordingRuntimeStatus{
			ID:               base.ID,
			Name:             base.Name,
			RecordingEnabled: base.RecordingEnabled,
		}
		s.nvrMu.Lock()
		_, item.IsRecording = s.nvrSessions[item.ID]
		s.nvrMu.Unlock()
		list = append(list, item)
	}
	s.nvrMu.Lock()
	active := len(s.nvrSessions)
	s.nvrMu.Unlock()
	return list, active, nil
}

func (s *Service) ActiveRecordingCount() int {
	s.nvrMu.Lock()
	defer s.nvrMu.Unlock()
	return len(s.nvrSessions)
}

func (s *Service) StartAllRecordings() {
	rows, err := s.db.Query(`SELECT id FROM cameras WHERE recording_enabled=1 AND is_enabled=1`)
	if err != nil {
		return
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		if err := s.nvrStart(id); err != nil {
			log.Printf("[NVR] Auto-start cam %d failed: %v", id, err)
		}
	}
	log.Println("[NVR] Auto-started recording sessions on boot")
}

func (s *Service) nvrStart(cameraID int) error {
	s.nvrMu.Lock()
	if _, ok := s.nvrSessions[cameraID]; ok {
		s.nvrMu.Unlock()
		return nil
	}
	s.nvrMu.Unlock()

	var rtspURL, onvifURL, username, passEnc, recSource string
	var recBitrate int
	if err := s.db.QueryRow(`
		SELECT COALESCE(rtsp_url,''), COALESCE(onvif_url,''), username,
		       COALESCE(password_encrypted,''), COALESCE(recording_source,'rtsp'),
		       COALESCE(recording_bitrate_kbps,0)
		FROM cameras WHERE id=?`, cameraID).
		Scan(&rtspURL, &onvifURL, &username, &passEnc, &recSource, &recBitrate); err != nil {
		return fmt.Errorf("camera not found: %v", err)
	}

	pass := ResolvePassword(s.secret, passEnc, rtspURL)
	switch recSource {
	case "onvif":
		if strings.TrimSpace(onvifURL) == "" {
			return fmt.Errorf("cam %d: recording_source=onvif missing onvif_url", cameraID)
		}
		fetched, err := GetONVIFStreamURI(onvifURL, username, pass)
		if err != nil || fetched == "" {
			return fmt.Errorf("cam %d: ONVIF GetStreamUri failed: %v", cameraID, err)
		}
		rtspURL = fetched
	default:
		if strings.TrimSpace(rtspURL) == "" && strings.TrimSpace(onvifURL) != "" {
			if fetched, err := GetONVIFStreamURI(onvifURL, username, pass); err == nil && fetched != "" {
				rtspURL = fetched
				_, _ = s.db.Exec(`UPDATE cameras SET rtsp_url=? WHERE id=?`, rtspURL, cameraID)
			}
		}
		if strings.TrimSpace(rtspURL) == "" {
			return fmt.Errorf("cam %d: rtsp_url is empty", cameraID)
		}
	}

	_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=1 WHERE id=?`, cameraID)
	go s.nvrRunSession(cameraID, rtspURL, username, pass, recBitrate)
	return nil
}

func (s *Service) nvrStop(cameraID int) {
	_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)

	s.nvrMu.Lock()
	sess, ok := s.nvrSessions[cameraID]
	if !ok {
		s.nvrMu.Unlock()
		return
	}
	select {
	case <-sess.cancel:
	default:
		close(sess.cancel)
	}
	if sess.cmd != nil && sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}
	done := sess.done
	delete(s.nvrSessions, cameraID)
	s.nvrMu.Unlock()

	if done != nil {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}

func (s *Service) nvrRunSession(cameraID int, rtspURL, username, password string, bitrateKbps int) {
	authURL := BuildAuthURL(RTSPCleanURL(rtspURL), username, password)
	cameraName := fmt.Sprintf("camera_%d", cameraID)
	var dbCameraName string
	if err := s.db.QueryRow(`SELECT name FROM cameras WHERE id=?`, cameraID).Scan(&dbCameraName); err == nil && strings.TrimSpace(dbCameraName) != "" {
		cameraName = strings.TrimSpace(dbCameraName)
	}

	baseDir := s.StorageDir()
	now := time.Now()
	dir := filepath.Join(baseDir, fmt.Sprintf("cam%d", cameraID), now.Format("2006-01-02"))
	_ = os.MkdirAll(dir, 0o755)
	ts := now.Format("150405")
	filePath := filepath.Join(dir, fmt.Sprintf("rec_%s.mp4", ts))

	ffmpegBin := FindFFmpegBin()
	if ffmpegBin == "" {
		log.Printf("[NVR] ffmpeg not found, cannot record cam %d", cameraID)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording failed", "Camera recording failed", map[string]interface{}{"camera_name": cameraName, "reason": "ffmpeg_not_found"})
		s.writeSystemLog("error", "camera", "camera_recording_failed", "camera recording failed: ffmpeg not found", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "ffmpeg_not_found"})
		return
	}

	audioArgs := BuildAudioArgs(ffmpegBin, authURL)
	tmpFilePath := filePath + ".tmp.mp4"
	args := []string{
		"-rtsp_transport", "tcp",
		"-timeout", "10000000",
		"-fflags", "+genpts",
		"-use_wallclock_as_timestamps", "1",
		"-i", authURL,
		"-map", "0:v:0",
	}
	args = append(args, audioArgs...)
	if bitrateKbps > 0 {
		bps := fmt.Sprintf("%dk", bitrateKbps)
		bufSize := fmt.Sprintf("%dk", bitrateKbps*2)
		args = append(args,
			"-c:v", "libx264",
			"-b:v", bps,
			"-maxrate", bps,
			"-bufsize", bufSize,
			"-preset", "veryfast",
			"-profile:v", "baseline",
			"-pix_fmt", "yuv420p",
		)
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "-y", tmpFilePath)
	cmd := exec.Command(ffmpegBin, args...)
	cmd.Stderr = &ffmpegLogWriter{cameraID: cameraID}

	sess := &nvrSession{
		startedAt: now,
		filePath:  filePath,
		done:      make(chan struct{}),
		cancel:    make(chan struct{}),
	}
	s.nvrMu.Lock()
	s.nvrSessions[cameraID] = sess
	s.nvrMu.Unlock()

	select {
	case <-sess.cancel:
		s.nvrMu.Lock()
		if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
			delete(s.nvrSessions, cameraID)
		}
		s.nvrMu.Unlock()
		close(sess.done)
		return
	default:
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[NVR] ffmpeg start failed cam %d: %v", cameraID, err)
		s.nvrMu.Lock()
		if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
			delete(s.nvrSessions, cameraID)
		}
		s.nvrMu.Unlock()
		close(sess.done)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording start failed", "Camera recording start failed", map[string]interface{}{"camera_name": cameraName, "reason": "ffmpeg_start_failed", "error": err.Error()})
		s.writeSystemLog("error", "camera", "camera_recording_start_failed", "camera recording start failed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "ffmpeg_start_failed", "error": err.Error()})
		return
	}

	s.nvrMu.Lock()
	sess.cmd = cmd
	s.nvrMu.Unlock()

	select {
	case <-sess.cancel:
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	default:
	}

	res, err := s.db.Exec(`INSERT INTO camera_recordings (camera_id, camera_name, file_path, started_at, status)
		VALUES (?, ?, ?, datetime('now'), 'recording')`, cameraID, cameraName, filePath)
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		s.nvrMu.Lock()
		delete(s.nvrSessions, cameraID)
		s.nvrMu.Unlock()
		close(sess.done)
		log.Printf("[NVR] DB insert failed cam %d: %v", cameraID, err)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording failed", "Camera recording failed", map[string]interface{}{"camera_name": cameraName, "reason": "record_insert_failed", "error": err.Error()})
		s.writeSystemLog("error", "camera", "camera_recording_failed", "camera recording failed: database insert error", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "record_insert_failed", "error": err.Error()})
		return
	}
	recID, _ := res.LastInsertId()
	sess.recID = recID

	log.Printf("[NVR] recording cam %d -> %s (id=%d)", cameraID, tmpFilePath, recID)
	s.writeDeviceLog(cameraID, "notice", "camera-recording", "camera recording started", "Camera recording started", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "file_path": filePath, "recording_bitrate": bitrateKbps})
	s.writeSystemLog("notice", "camera", "camera_recording_started", "camera recording started", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "file_path": filePath, "recording_bitrate": bitrateKbps})
	_ = cmd.Wait()

	actualFilePath := filePath
	if fi, err := os.Stat(tmpFilePath); err == nil && fi.Size() > 0 {
		remuxCmd := exec.Command(ffmpegBin,
			"-i", tmpFilePath,
			"-c", "copy",
			"-movflags", "+faststart",
			"-f", "mp4",
			"-y", filePath,
		)
		if remuxErr := remuxCmd.Run(); remuxErr != nil {
			log.Printf("[NVR] remux failed cam %d: %v, using raw file", cameraID, remuxErr)
			_ = os.Rename(tmpFilePath, filePath)
			s.writeDeviceLog(cameraID, "warning", "camera-recording", "camera recording remux failed", "Camera recording remux failed", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "file_path": filePath, "reason": "remux_failed", "error": remuxErr.Error()})
			s.writeSystemLog("warning", "camera", "camera_recording_remux_failed", "camera recording remux failed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "file_path": filePath, "reason": "remux_failed", "error": remuxErr.Error()})
		} else {
			_ = os.Remove(tmpFilePath)
		}
	} else {
		_ = os.Rename(tmpFilePath, filePath)
	}

	s.nvrMu.Lock()
	if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
		delete(s.nvrSessions, cameraID)
	}
	s.nvrMu.Unlock()
	close(sess.done)

	fi, _ := os.Stat(actualFilePath)
	sz := int64(0)
	if fi != nil {
		sz = fi.Size()
	}
	dur := int(time.Since(sess.startedAt).Seconds())
	_, _ = s.db.Exec(`UPDATE camera_recordings SET ended_at=datetime('now'), duration_sec=?, file_size=?, status='done' WHERE id=?`, dur, sz, recID)
	s.writeDeviceLog(cameraID, "notice", "camera-recording", "camera recording completed", "Camera recording completed", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "duration_sec": dur, "file_size": sz, "file_path": actualFilePath})
	s.writeSystemLog("notice", "camera", "camera_recording_completed", "camera recording completed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "duration_sec": dur, "file_size": sz, "file_path": actualFilePath})

	var enabled int
	_ = s.db.QueryRow(`SELECT recording_enabled FROM cameras WHERE id=?`, cameraID).Scan(&enabled)
	if enabled == 1 {
		delay := 3 * time.Second
		if dur < 3 {
			delay = 10 * time.Second
			s.writeDeviceLog(cameraID, "warning", "camera-recording", "camera recording recycled", "Camera recording recycled", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "duration_sec": dur, "restart_in_s": int(delay.Seconds()), "reason": "fast_exit"})
			s.writeSystemLog("warning", "camera", "camera_recording_recycled", "camera recording recycled after fast exit", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "duration_sec": dur, "restart_in_s": int(delay.Seconds()), "reason": "fast_exit"})
		}
		time.Sleep(delay)
		s.nvrMu.Lock()
		_, still := s.nvrSessions[cameraID]
		s.nvrMu.Unlock()
		if !still {
			var rtsp, user, pe string
			var bitr int
			if s.db.QueryRow(`SELECT rtsp_url, username, password_encrypted, COALESCE(recording_bitrate_kbps,0) FROM cameras WHERE id=?`, cameraID).
				Scan(&rtsp, &user, &pe, &bitr) == nil {
				pw := ResolvePassword(s.secret, pe, rtsp)
				go s.nvrRunSession(cameraID, rtsp, user, pw, bitr)
			}
		}
	}
}
