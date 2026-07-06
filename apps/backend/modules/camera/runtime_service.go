// Made by YTSworks
// YTS工作室製作
package camera

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
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
	lastFrameAt time.Time
}

type hlsStream struct {
	id         string
	poolKey    string
	dir        string
	cmd        *exec.Cmd
	startedAt  time.Time
	lastAccess time.Time
}

type nvrSession struct {
	cmd       *exec.Cmd
	startedAt time.Time
	filePath  string
	recID     int64
	done      chan struct{}
	cancel    chan struct{}
}

const (
	defaultPreviewBitrateKbps = 0
	defaultPreviewFPS         = 15
	defaultPreviewQuality     = 2
	debugBuildCameraLogging   = false
	maxCachedPreviewFrameAge  = 500 * time.Millisecond
	hlsStaleAfter             = 2 * time.Minute
)

func DebugEnabled() bool {
	value := strings.TrimSpace(os.Getenv("NMS_CAMERA_DEBUG"))
	if value == "" {
		return debugBuildCameraLogging
	}
	switch strings.ToLower(value) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

func Debugf(format string, args ...interface{}) {
	if DebugEnabled() {
		log.Printf("[CameraDebug] "+format, args...)
	}
}

func cameraPreviewFPS() int {
	value := strings.TrimSpace(os.Getenv("NMS_CAMERA_PREVIEW_FPS"))
	if value == "" {
		return defaultPreviewFPS
	}
	fps, err := strconv.Atoi(value)
	if err != nil {
		return defaultPreviewFPS
	}
	if fps < 1 {
		return 1
	}
	if fps > 30 {
		return 30
	}
	return fps
}

func cameraPreviewQuality() int {
	value := strings.TrimSpace(os.Getenv("NMS_CAMERA_PREVIEW_QUALITY"))
	if value == "" {
		return defaultPreviewQuality
	}
	quality, err := strconv.Atoi(value)
	if err != nil {
		return defaultPreviewQuality
	}
	if quality < 2 {
		return 2
	}
	if quality > 15 {
		return 15
	}
	return quality
}

func cameraPreviewSourceMode() string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("NMS_CAMERA_PREVIEW_SOURCE")))
	switch value {
	case "go2rtc", "fanout":
		return "go2rtc"
	case "auto":
		return "auto"
	default:
		return "direct"
	}
}

func cameraRecordingSourceMode() string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("NMS_CAMERA_RECORDING_SOURCE")))
	switch value {
	case "go2rtc", "fanout":
		return "go2rtc"
	case "auto":
		return "auto"
	default:
		return "direct"
	}
}

func cameraRecordingAudioEnabled() bool {
	value := strings.TrimSpace(os.Getenv("NMS_CAMERA_RECORD_AUDIO"))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
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
			log.Printf("[NVR ffmpeg cam%d] %s", w.cameraID, RedactSensitiveText(line))
		}
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
}

type cappedTextBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *cappedTextBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.max <= 0 {
		b.max = 32 * 1024
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = append([]byte(nil), b.buf[len(b.buf)-b.max:]...)
	}
	return len(p), nil
}

func (b *cappedTextBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return RedactSensitiveText(string(b.buf))
}

func (s *Service) setCameraStatus(id, status string) {
	if s.db != nil {
		_, _ = s.db.Exec("UPDATE cameras SET status=?, last_seen=? WHERE id=?", status, time.Now(), id)
	}
}

func (s *Service) CaptureSnapshot(id string, timeout time.Duration) (SnapshotResult, error) {
	var rtspURL, username, passwordEncrypted string
	var transportPreference string
	var udpMinPort, udpMaxPort int
	err := s.db.QueryRow(`
		SELECT COALESCE(NULLIF(preview_rtsp_url,''), rtsp_url, ''), username, COALESCE(password_encrypted,''),
		       COALESCE(rtsp_transport,'auto'), COALESCE(rtsp_udp_min_port,0), COALESCE(rtsp_udp_max_port,0)
		FROM cameras WHERE id = ?`, id).
		Scan(&rtspURL, &username, &passwordEncrypted, &transportPreference, &udpMinPort, &udpMaxPort)
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
	log.Printf("[Snapshot] cam=%s rawURL=%q cleaned=%q userSet=%v passSet=%v passEncSet=%v finalURL=%q",
		id,
		RedactSensitiveText(rawURL),
		RedactSensitiveText(cleanedURL),
		username != "",
		password != "",
		passwordEncrypted != "",
		RedactSensitiveText(rtspURL))

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
	rtspOptions := normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: transportPreference, UDPMinPort: udpMinPort, UDPMaxPort: udpMaxPort})
	transports := rtspOptions.transports()
	var data []byte
	var lastErr error
	var lastStderr string
	for transportIdx, transport := range transports {
		modes := hwDecodeModes()
		for idx, mode := range modes {
			var stderr string
			attemptTimeout := timeout
			if mode.isHardware() && idx+1 < len(modes) && timeout > 3*time.Second {
				attemptTimeout = 3 * time.Second
			}
			data, stderr, lastErr = runSnapshotFFmpeg(ffmpegBin, rtspURL, mode, transport, rtspOptions, attemptTimeout)
			lastStderr = stderr
			if lastErr == nil {
				break
			}
			if mode.isHardware() && idx+1 < len(modes) {
				if isHardwareDecodeFailure(lastStderr) || errors.Is(lastErr, ErrSnapshotTimeout) {
					disableHardwareDecodeMode(mode, lastStderr)
				}
				log.Printf("[Camera] ffmpeg snapshot hwaccel failed id=%s mode=%s transport=%s; retrying CPU decode | %s",
					id, mode.name, transport, strings.TrimSpace(lastStderr))
				continue
			}
			break
		}
		if lastErr == nil {
			break
		}
		if transportIdx+1 < len(transports) {
			log.Printf("[Camera] ffmpeg snapshot transport failed id=%s transport=%s; retrying transport=%s | %s",
				id, transport, transports[transportIdx+1], strings.TrimSpace(lastStderr))
			continue
		}
		break
	}
	if lastErr != nil {
		s.setCameraStatus(id, "offline")
		switch {
		case errors.Is(lastErr, ErrSnapshotTimeout):
			log.Printf("[Camera] ffmpeg snapshot timeout id=%s | %s", id, strings.TrimSpace(lastStderr))
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "warning", "camera-stream", "camera snapshot timeout", "Camera snapshot timeout", map[string]interface{}{"reason": "snapshot_timeout"})
			}
			s.writeSystemLog("warning", "camera", "camera_snapshot_timeout", "camera snapshot timeout", map[string]interface{}{"camera_id": id, "reason": "snapshot_timeout"})
			return SnapshotResult{}, ErrSnapshotTimeout
		case errors.Is(lastErr, ErrSnapshotEmpty):
			log.Printf("[Camera] ffmpeg returned empty frame id=%s | %s", id, strings.TrimSpace(lastStderr))
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "warning", "camera-stream", "camera snapshot empty", "Camera snapshot empty", map[string]interface{}{"reason": "empty_frame"})
			}
			s.writeSystemLog("warning", "camera", "camera_snapshot_empty", "camera snapshot returned empty frame", map[string]interface{}{"camera_id": id, "reason": "empty_frame"})
			return SnapshotResult{}, ErrSnapshotEmpty
		default:
			log.Printf("[Camera] ffmpeg snapshot failed id=%s: %v | %s", id, lastErr, strings.TrimSpace(lastStderr))
			if cameraID > 0 {
				s.writeDeviceLog(cameraID, "error", "camera-stream", "camera snapshot failed", "Camera snapshot failed", map[string]interface{}{"reason": "ffmpeg_run_failed", "error": lastErr.Error()})
			}
			s.writeSystemLog("error", "camera", "camera_snapshot_failed", "camera snapshot failed", map[string]interface{}{"camera_id": id, "reason": "ffmpeg_run_failed", "error": lastErr.Error()})
			return SnapshotResult{}, lastErr
		}
	}

	s.setCameraStatus(id, "online")
	if cameraID > 0 {
		s.writeDeviceLog(cameraID, "info", "camera-stream", "camera snapshot success", "Camera snapshot success", map[string]interface{}{"bytes": len(data)})
	}
	return SnapshotResult{CameraID: cameraID, Data: data}, nil
}

func runSnapshotFFmpeg(ffmpegBin, rtspURL string, mode ffmpegDecodeMode, transport string, options rtspRuntimeOptions, timeout time.Duration) ([]byte, string, error) {
	cmd := exec.Command(ffmpegBin, BuildFFmpegSnapshotArgsForModeTransportOptions(rtspURL, mode, transport, options)...)
	var out bytes.Buffer
	var stderr cappedTextBuffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case runErr := <-done:
		if runErr != nil {
			return nil, stderr.String(), fmt.Errorf("%w: %v", ErrSnapshotRunFailed, runErr)
		}
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, stderr.String(), ErrSnapshotTimeout
	}

	data := out.Bytes()
	if len(data) == 0 {
		return nil, stderr.String(), ErrSnapshotEmpty
	}
	return data, "", nil
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
	ch := make(chan []byte, 1)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	var cached []byte
	cachedAge := time.Duration(0)
	if !s.lastFrameAt.IsZero() {
		cachedAge = time.Since(s.lastFrameAt)
	}
	if len(s.lastFrame) > 0 && cachedAge <= maxCachedPreviewFrameAge {
		cached = append([]byte(nil), s.lastFrame...)
	}
	count := len(s.clients)
	s.mu.Unlock()
	if len(cached) > 0 {
		select {
		case ch <- cached:
		default:
		}
	}
	Debugf("mjpeg_client_add cam=%s clients=%d cached=%v cached_age_ms=%d", s.id, count, len(cached) > 0, cachedAge.Milliseconds())
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
	Debugf("mjpeg_client_remove cam=%s clients=%d", s.id, count)
	if fn != nil {
		fn()
	}
}

func (s *mjpegStream) broadcast(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFrame = append(s.lastFrame[:0], frame...)
	s.lastFrameAt = time.Now()
	s.bytesSent += int64(len(frame))
	for ch := range s.clients {
		select {
		case ch <- frame:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- frame:
			default:
			}
		}
	}
}

func mjpegPoolKey(id, rtspURL string, bitrateKbps int, options rtspRuntimeOptions) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(rtspURL))
	normalized := normalizeRTSPRuntimeOptions(options)
	return fmt.Sprintf("%s@%d@%s@%d-%d@%08x", id, bitrateKbps, normalized.Transport, normalized.UDPMinPort, normalized.UDPMaxPort, h.Sum32())
}

func (s *Service) getMjpegStream(id, rtspURL, ffmpegBin string, bitrateKbps int, options rtspRuntimeOptions) *mjpegStream {
	ProbeGPU(ffmpegBin)
	if bitrateKbps <= 0 {
		bitrateKbps = defaultPreviewBitrateKbps
	}
	options = normalizeRTSPRuntimeOptions(options)
	poolKey := mjpegPoolKey(id, rtspURL, bitrateKbps, options)

	s.mjpegPoolMu.Lock()
	stream, ok := s.mjpegPool[poolKey]
	if ok {
		stream.mu.Lock()
		stale := len(stream.clients) == 0 && stream.cancelFn == nil
		stream.mu.Unlock()
		if !stale {
			Debugf("mjpeg_pool_reuse cam=%s pool=%s bitrate=%d transport=%s source=%s", id, poolKey, bitrateKbps, options.Transport, RedactSensitiveText(rtspURL))
			s.mjpegPoolMu.Unlock()
			return stream
		}
		delete(s.mjpegPool, poolKey)
	}
	stream = newMjpegStream(s, id, poolKey, bitrateKbps)
	s.mjpegPool[poolKey] = stream
	s.mjpegPoolMu.Unlock()

	Debugf("mjpeg_pool_new cam=%s pool=%s bitrate=%d transport=%s source=%s", id, poolKey, bitrateKbps, options.Transport, RedactSensitiveText(rtspURL))
	go s.runMjpegFFmpeg(rtspURL, ffmpegBin, stream, options)
	return stream
}

func hlsBaseDir() string {
	if self, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(self), "data", "hls")
	}
	return filepath.Join("data", "hls")
}

func hlsPoolKey(id, rtspURL string, options rtspRuntimeOptions) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(rtspURL))
	normalized := normalizeRTSPRuntimeOptions(options)
	return fmt.Sprintf("%s@%s@%d-%d@%08x", id, normalized.Transport, normalized.UDPMinPort, normalized.UDPMaxPort, h.Sum32())
}

func safeHLSStreamDir(poolKey string) (string, error) {
	base, err := filepath.Abs(hlsBaseDir())
	if err != nil {
		return "", err
	}
	dir, err := filepath.Abs(filepath.Join(base, poolKey))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid hls stream directory")
	}
	return dir, nil
}

func (s *Service) cleanupStaleHLSLocked(now time.Time) {
	for key, stream := range s.hlsPool {
		if now.Sub(stream.lastAccess) <= hlsStaleAfter {
			continue
		}
		if stream.cmd != nil && stream.cmd.Process != nil {
			_ = stream.cmd.Process.Kill()
		}
		delete(s.hlsPool, key)
		_ = os.RemoveAll(stream.dir)
		Debugf("hls_pool_cleanup cam=%s pool=%s idle_ms=%d", stream.id, key, now.Sub(stream.lastAccess).Milliseconds())
	}
}

func (s *Service) EnsureHLSStream(id string) (string, error) {
	var rtspURL, username, passwordEnc string
	var transportPreference string
	var udpMinPort, udpMaxPort int
	err := s.db.QueryRow(`
		SELECT COALESCE(NULLIF(preview_rtsp_url,''), rtsp_url, ''), username,
		       COALESCE(password_encrypted,''), COALESCE(rtsp_transport,'auto'),
		       COALESCE(rtsp_udp_min_port,0), COALESCE(rtsp_udp_max_port,0)
		FROM cameras WHERE id=?`, id).
		Scan(&rtspURL, &username, &passwordEnc, &transportPreference, &udpMinPort, &udpMaxPort)
	if err == sql.ErrNoRows {
		return "", ErrCameraNotFound
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(rtspURL) == "" {
		return "", ErrStreamUnavailable
	}

	password := ResolvePassword(s.secret, passwordEnc, rtspURL)
	rtspURL = BuildAuthURL(RTSPCleanURL(rtspURL), username, password)
	ffmpegBin := FindFFmpegBin()
	if ffmpegBin == "" {
		return "", ErrFFmpegNotFound
	}
	rtspOptions := normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: transportPreference, UDPMinPort: udpMinPort, UDPMaxPort: udpMaxPort})
	poolKey := hlsPoolKey(id, rtspURL, rtspOptions)
	now := time.Now()

	s.hlsPoolMu.Lock()
	s.cleanupStaleHLSLocked(now)
	if stream, ok := s.hlsPool[poolKey]; ok {
		stream.lastAccess = now
		dir := stream.dir
		s.hlsPoolMu.Unlock()
		Debugf("hls_pool_reuse cam=%s pool=%s source=%s", id, poolKey, RedactSensitiveText(rtspURL))
		return dir, nil
	}
	s.hlsPoolMu.Unlock()

	dir, err := safeHLSStreamDir(poolKey)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	transports := rtspOptions.transports()
	transport := "tcp"
	if len(transports) > 0 {
		transport = transports[0]
	}
	playlistPath := filepath.Join(dir, "index.m3u8")
	segmentPattern := filepath.Join(dir, "seg_%06d.ts")
	args := BuildFFmpegHLSArgsForModeTransportOptions(rtspURL, playlistPath, segmentPattern, ffmpegDecodeMode{name: "copy"}, transport, rtspOptions)
	stderr := &cappedTextBuffer{}
	cmd := exec.Command(ffmpegBin, args...)
	cmd.Stderr = stderr
	Debugf("hls_ffmpeg_start cam=%s transport=%s args=%s", id, transport, RedactSensitiveText(strings.Join(args, " ")))
	if err := cmd.Start(); err != nil {
		return "", err
	}

	stream := &hlsStream{id: id, poolKey: poolKey, dir: dir, cmd: cmd, startedAt: now, lastAccess: now}
	s.hlsPoolMu.Lock()
	s.hlsPool[poolKey] = stream
	s.hlsPoolMu.Unlock()

	go func() {
		err := cmd.Wait()
		s.hlsPoolMu.Lock()
		if s.hlsPool[poolKey] == stream {
			delete(s.hlsPool, poolKey)
		}
		s.hlsPoolMu.Unlock()
		if err != nil {
			log.Printf("[Camera] HLS worker exited cam=%s: %v | %s", id, err, strings.TrimSpace(stderr.String()))
		}
	}()
	return dir, nil
}

func (s *Service) runMjpegFFmpeg(rtspURL, ffmpegBin string, stream *mjpegStream, options rtspRuntimeOptions) {
	boundary := "mjpegframe"
	camID := stream.id
	cameraID, _ := strconv.Atoi(camID)
	ProbeGPU(ffmpegBin)
	modes := hwDecodeModes()
	transports := options.transports()
	modeIndex := 0
	transportIndex := 0

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

		mode := modes[modeIndex]
		transport := transports[transportIndex]
		stderr := &cappedTextBuffer{}
		fps := cameraPreviewFPS()
		quality := cameraPreviewQuality()
		args := BuildFFmpegStreamArgsForModeTransportOptions(rtspURL, boundary, fps, quality, stream.bitrateKbps, mode, transport, options)
		cmd := exec.Command(ffmpegBin, args...)
		cmd.Stdout = pw
		cmd.Stderr = stderr

		cancelCh := make(chan struct{})
		var cancelOnce sync.Once
		stream.mu.Lock()
		stream.cmd = cmd
		stream.cancelFn = func() { cancelOnce.Do(func() { close(cancelCh) }) }
		stream.mu.Unlock()

		startedAt := time.Now()
		Debugf("mjpeg_ffmpeg_start cam=%s mode=%s transport=%s fps=%d quality=%d bitrate=%d clients=%d args=%s",
			camID, mode.name, transport, fps, quality, stream.bitrateKbps, count, RedactSensitiveText(strings.Join(args, " ")))
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
		frameSeenCh := make(chan struct{})
		var frameSeenOnce sync.Once
		firstFrameTimeoutCh := make(chan struct{})
		go func(cmd *exec.Cmd) {
			timeout := 8 * time.Second
			if mode.isHardware() {
				timeout = 3 * time.Second
			}
			timer := time.NewTimer(timeout)
			defer timer.Stop()
			select {
			case <-frameSeenCh:
			case <-done:
			case <-cancelCh:
			case <-timer.C:
				close(firstFrameTimeoutCh)
				log.Printf("[Camera] ffmpeg stream first-frame timeout id=%s mode=%s transport=%s; trying fallback",
					camID, mode.name, transport)
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			}
		}(cmd)

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
			frameSeq := 0
			var lastFrameAt time.Time

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
						tail := accumulated[idx+len(sep):]
						accumulated = append(accumulated[:0], tail...)
						if frame := ExtractJPEG(chunk); len(frame) > 0 {
							frameSeq++
							now := time.Now()
							intervalMs := int64(0)
							if !lastFrameAt.IsZero() {
								intervalMs = now.Sub(lastFrameAt).Milliseconds()
							}
							lastFrameAt = now
							if firstFrame {
								firstFrame = false
								frameSeenOnce.Do(func() { close(frameSeenCh) })
								s.setCameraStatus(camID, "online")
								Debugf("mjpeg_first_frame cam=%s mode=%s transport=%s first_frame_ms=%d bytes=%d", camID, mode.name, transport, now.Sub(startedAt).Milliseconds(), len(frame))
							}
							if frameSeq <= 5 || frameSeq%30 == 0 {
								stream.mu.Lock()
								clients := len(stream.clients)
								stream.mu.Unlock()
								Debugf("mjpeg_frame cam=%s seq=%d bytes=%d interval_ms=%d clients=%d", camID, frameSeq, len(frame), intervalMs, clients)
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
				frameSeq++
				now := time.Now()
				if firstFrame {
					firstFrame = false
					frameSeenOnce.Do(func() { close(frameSeenCh) })
					s.setCameraStatus(camID, "online")
					Debugf("mjpeg_first_frame cam=%s mode=%s transport=%s first_frame_ms=%d bytes=%d", camID, mode.name, transport, now.Sub(startedAt).Milliseconds(), len(frame))
				}
				if frameSeq <= 5 || frameSeq%30 == 0 {
					stream.mu.Lock()
					clients := len(stream.clients)
					stream.mu.Unlock()
					Debugf("mjpeg_frame cam=%s seq=%d bytes=%d interval_ms=final clients=%d", camID, frameSeq, len(frame), clients)
				}
				stream.broadcast(frame)
			}
		}()

		select {
		case <-done:
			frameSeen := false
			select {
			case <-frameSeenCh:
				frameSeen = true
			default:
			}
			firstFrameTimedOut := false
			select {
			case <-firstFrameTimeoutCh:
				firstFrameTimedOut = true
			default:
			}
			if !frameSeen && mode.isHardware() && modeIndex+1 < len(modes) {
				timedOut := false
				if firstFrameTimedOut {
					timedOut = true
				}
				if timedOut || isHardwareDecodeFailure(stderr.String()) {
					disableHardwareDecodeMode(mode, stderr.String())
				}
				log.Printf("[Camera] ffmpeg stream hwaccel failed id=%s mode=%s transport=%s; retrying CPU decode | %s",
					camID, mode.name, transport, strings.TrimSpace(stderr.String()))
				modeIndex = len(modes) - 1
				_ = pr.Close()
				continue
			}
			if !frameSeen && transportIndex+1 < len(transports) {
				transportIndex++
				modeIndex = 0
				modes = hwDecodeModes()
				log.Printf("[Camera] ffmpeg stream transport failed id=%s mode=%s transport=%s; retrying transport=%s | %s",
					camID, mode.name, transport, transports[transportIndex], strings.TrimSpace(stderr.String()))
				_ = pr.Close()
				continue
			}
			if text := strings.TrimSpace(stderr.String()); text != "" {
				log.Printf("[Camera] ffmpeg stream worker exited id=%s mode=%s transport=%s | %s", camID, mode.name, transport, text)
			}
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
	var transportPreference string
	var udpMinPort, udpMaxPort int
	err := s.db.QueryRow(`
		SELECT COALESCE(NULLIF(preview_rtsp_url,''), rtsp_url, ''), username,
		       COALESCE(password_encrypted,''), COALESCE(bandwidth_limit_kbps,0),
		       COALESCE(rtsp_transport,'auto'), COALESCE(rtsp_udp_min_port,0),
		       COALESCE(rtsp_udp_max_port,0)
		FROM cameras WHERE id=?`, id).
		Scan(&rtspURL, &username, &passwordEnc, &bitrateKbps, &transportPreference, &udpMinPort, &udpMaxPort)
	if err == sql.ErrNoRows {
		return nil, ErrCameraNotFound
	}
	if err != nil {
		return nil, err
	}

	password := ResolvePassword(s.secret, passwordEnc, rtspURL)
	rawRTSPURL := rtspURL
	rtspURL = BuildAuthURL(RTSPCleanURL(rawRTSPURL), username, password)

	ffmpegBin := FindFFmpegBin()
	cameraID, _ := strconv.Atoi(id)
	if ffmpegBin == "" {
		if cameraID > 0 {
			s.writeDeviceLog(cameraID, "error", "camera-stream", "camera live preview failed", "Camera live preview failed", map[string]interface{}{"reason": "ffmpeg_not_found"})
		}
		s.writeSystemLog("error", "camera", "camera_stream_failed", "camera live preview failed: ffmpeg not found", map[string]interface{}{"camera_id": id, "reason": "ffmpeg_not_found"})
		return nil, ErrFFmpegNotFound
	}

	rtspOptions := normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: transportPreference, UDPMinPort: udpMinPort, UDPMaxPort: udpMaxPort})
	sourceMode := cameraPreviewSourceMode()
	if sourceMode == "go2rtc" || sourceMode == "auto" {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		if localRTSPURL, err := s.EnsureLocalRTSPStream(ctx, id, rawRTSPURL, username, password); err == nil {
			rtspURL = localRTSPURL
			rtspOptions = normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: "tcp"})
			log.Printf("[Camera] MJPEG preview cam=%s via go2rtc local RTSP %s", id, RedactSensitiveText(localRTSPURL))
		} else {
			log.Printf("[Camera] MJPEG preview cam=%s go2rtc local RTSP unavailable, fallback direct RTSP: %v", id, err)
		}
		cancel()
	} else {
		Debugf("mjpeg_preview_direct cam=%s transport=%s source=%s", id, rtspOptions.Transport, RedactSensitiveText(rtspURL))
	}
	stream := s.getMjpegStream(id, rtspURL, ffmpegBin, bitrateKbps, rtspOptions)
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

	var recordingRTSPURL, rtspURL, previewRTSPURL, onvifURL, username, passEnc, recSource string
	var recBitrate int
	var transportPreference string
	var udpMinPort, udpMaxPort int
	if err := s.db.QueryRow(`
		SELECT COALESCE(recording_rtsp_url,''), COALESCE(rtsp_url,''), COALESCE(preview_rtsp_url,''),
		       COALESCE(onvif_url,''), COALESCE(username,''),
		       COALESCE(password_encrypted,''), COALESCE(recording_source,'rtsp'),
		       COALESCE(recording_bitrate_kbps,0), COALESCE(rtsp_transport,'auto'),
		       COALESCE(rtsp_udp_min_port,0), COALESCE(rtsp_udp_max_port,0)
		FROM cameras WHERE id=?`, cameraID).
		Scan(&recordingRTSPURL, &rtspURL, &previewRTSPURL, &onvifURL, &username, &passEnc, &recSource, &recBitrate, &transportPreference, &udpMinPort, &udpMaxPort); err != nil {
		return fmt.Errorf("camera not found: %v", err)
	}

	rtspURL = firstNonEmpty(recordingRTSPURL, rtspURL, previewRTSPURL)
	pass := ResolvePassword(s.secret, passEnc, firstNonEmpty(rtspURL, previewRTSPURL, recordingRTSPURL))
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
				_, _ = s.db.Exec(`UPDATE cameras SET rtsp_url=?, preview_rtsp_url=? WHERE id=?`, rtspURL, rtspURL, cameraID)
			}
		}
		if strings.TrimSpace(rtspURL) == "" {
			return fmt.Errorf("cam %d: recording RTSP URL is empty", cameraID)
		}
	}

	_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=1 WHERE id=?`, cameraID)
	rtspOptions := normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: transportPreference, UDPMinPort: udpMinPort, UDPMaxPort: udpMaxPort})
	recordRTSPURL := rtspURL
	recordUsername := username
	recordPassword := pass
	recordOptions := rtspOptions
	sourceMode := cameraRecordingSourceMode()
	if sourceMode == "go2rtc" || sourceMode == "auto" {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		if localRTSPURL, err := s.EnsureRecordingRTSPStream(ctx, strconv.Itoa(cameraID), rtspURL, username, pass); err == nil {
			recordRTSPURL = localRTSPURL
			recordUsername = ""
			recordPassword = ""
			recordOptions = normalizeRTSPRuntimeOptions(rtspRuntimeOptions{Transport: "tcp"})
			log.Printf("[NVR] cam %d recording via go2rtc local RTSP %s", cameraID, RedactSensitiveText(localRTSPURL))
		} else {
			log.Printf("[NVR] cam %d go2rtc recording source unavailable, fallback direct RTSP: %v", cameraID, err)
			s.writeSystemLog("warning", "camera", "camera_recording_go2rtc_fallback", "camera recording fallback to direct RTSP", map[string]interface{}{"camera_id": cameraID, "reason": "go2rtc_unavailable", "error": err.Error()})
		}
		cancel()
	} else {
		log.Printf("[NVR] cam %d recording via direct RTSP %s", cameraID, RedactSensitiveText(recordRTSPURL))
	}
	go s.nvrRunSession(cameraID, recordRTSPURL, recordUsername, recordPassword, recBitrate, recordOptions)
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

func buildNVRFFmpegArgs(ffmpegBin, authURL, tmpFilePath string, bitrateKbps int, transport string, options rtspRuntimeOptions) []string {
	args := options.inputArgs(transport)
	args = append(args,
		"-timeout", "30000000",
		"-fflags", "+genpts",
		"-use_wallclock_as_timestamps", "1",
		"-i", authURL,
		"-map", "0:v:0",
	)
	if cameraRecordingAudioEnabled() {
		args = append(args, BuildAudioArgsForTransport(ffmpegBin, authURL, transport, options)...)
	} else {
		args = append(args, "-an")
	}
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
	args = append(args,
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "mpegts",
		"-y", tmpFilePath,
	)
	return args
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil || fi == nil {
		return 0
	}
	return fi.Size()
}

func (s *Service) nvrRunSession(cameraID int, rtspURL, username, password string, bitrateKbps int, options rtspRuntimeOptions) {
	authURL := BuildAuthURL(RTSPCleanURL(rtspURL), username, password)
	options = normalizeRTSPRuntimeOptions(options)
	cameraName := fmt.Sprintf("camera_%d", cameraID)
	var dbCameraName string
	if err := s.db.QueryRow(`SELECT name FROM cameras WHERE id=?`, cameraID).Scan(&dbCameraName); err == nil && strings.TrimSpace(dbCameraName) != "" {
		cameraName = strings.TrimSpace(dbCameraName)
	}

	baseDir := s.StorageDir()
	now := time.Now()
	dir := filepath.Join(baseDir, fmt.Sprintf("cam%d", cameraID), now.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)
		log.Printf("[NVR] storage directory create failed cam %d dir=%s: %v", cameraID, dir, err)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording failed", "Camera recording failed", map[string]interface{}{"camera_name": cameraName, "reason": "storage_create_failed", "storage_dir": dir, "error": err.Error()})
		s.writeSystemLog("error", "camera", "camera_recording_failed", "camera recording failed: storage directory error", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "storage_create_failed", "storage_dir": dir, "error": err.Error()})
		return
	}
	ts := now.Format("150405")
	filePath := filepath.Join(dir, fmt.Sprintf("rec_%s.mp4", ts))

	ffmpegBin := FindFFmpegBin()
	if ffmpegBin == "" {
		_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)
		log.Printf("[NVR] ffmpeg not found, cannot record cam %d", cameraID)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording failed", "Camera recording failed", map[string]interface{}{"camera_name": cameraName, "reason": "ffmpeg_not_found"})
		s.writeSystemLog("error", "camera", "camera_recording_failed", "camera recording failed: ffmpeg not found", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "ffmpeg_not_found"})
		return
	}

	tmpFilePath := filePath + ".tmp.ts"
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

	transports := options.transports()
	var cmd *exec.Cmd
	var cmdDone chan error
	activeTransport := ""
	var lastErr error
	var lastStderr string
	for idx, transport := range transports {
		_ = os.Remove(tmpFilePath)
		args := buildNVRFFmpegArgs(ffmpegBin, authURL, tmpFilePath, bitrateKbps, transport, options)
		candidate := exec.Command(ffmpegBin, args...)
		stderr := &cappedTextBuffer{}
		candidate.Stderr = io.MultiWriter(&ffmpegLogWriter{cameraID: cameraID}, stderr)

		log.Printf("[NVR] ffmpeg start cam %d transport=%s output=%s args=%s", cameraID, transport, tmpFilePath, RedactSensitiveText(strings.Join(args, " ")))
		if err := candidate.Start(); err != nil {
			lastErr = err
			lastStderr = stderr.String()
			log.Printf("[NVR] ffmpeg start failed cam %d transport=%s: %v", cameraID, transport, err)
			if idx+1 < len(transports) {
				continue
			}
			break
		}

		done := make(chan error, 1)
		go func(c *exec.Cmd) { done <- c.Wait() }(candidate)
		startDeadline := time.NewTimer(12 * time.Second)
		startTicker := time.NewTicker(250 * time.Millisecond)
		started := false
		processExited := false
		startTimedOut := false
		for !started && !processExited && !startTimedOut {
			select {
			case err := <-done:
				processExited = true
				lastErr = err
				lastStderr = stderr.String()
			case <-startDeadline.C:
				startTimedOut = true
				lastErr = fmt.Errorf("recording_start_timeout_no_data")
				lastStderr = stderr.String()
			case <-startTicker.C:
				if fileSize(tmpFilePath) > 0 {
					started = true
				}
			case <-sess.cancel:
				startTicker.Stop()
				if !startDeadline.Stop() {
					select {
					case <-startDeadline.C:
					default:
					}
				}
				if candidate.Process != nil {
					_ = candidate.Process.Kill()
				}
				_ = <-done
				s.nvrMu.Lock()
				if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
					delete(s.nvrSessions, cameraID)
				}
				s.nvrMu.Unlock()
				close(sess.done)
				return
			}
		}
		startTicker.Stop()
		if !startDeadline.Stop() {
			select {
			case <-startDeadline.C:
			default:
			}
		}
		if started {
			cmd = candidate
			cmdDone = done
			activeTransport = transport
			lastErr = nil
			lastStderr = ""
		}
		if !started {
			if !processExited && candidate.Process != nil {
				_ = candidate.Process.Kill()
				select {
				case err := <-done:
					if lastErr == nil {
						lastErr = err
					}
				case <-time.After(2 * time.Second):
				}
			}
			if lastStderr == "" {
				lastStderr = stderr.String()
			}
			log.Printf("[NVR] ffmpeg did not produce recording data cam %d transport=%s size=%d err=%v | %s",
				cameraID, transport, fileSize(tmpFilePath), lastErr, strings.TrimSpace(lastStderr))
			if idx+1 < len(transports) {
				log.Printf("[NVR] retrying recording cam %d with transport=%s", cameraID, transports[idx+1])
				_ = os.Remove(tmpFilePath)
				continue
			}
		}
		if cmd != nil {
			break
		}
	}

	if cmd == nil {
		s.nvrMu.Lock()
		if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
			delete(s.nvrSessions, cameraID)
		}
		s.nvrMu.Unlock()
		close(sess.done)
		_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)
		errText := "ffmpeg_start_failed"
		if lastErr != nil {
			errText = lastErr.Error()
		}
		if stderrText := strings.TrimSpace(lastStderr); stderrText != "" {
			errText += " | " + stderrText
		}
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording start failed", "Camera recording start failed", map[string]interface{}{"camera_name": cameraName, "reason": "ffmpeg_start_failed", "error": errText, "rtsp_transport": options.Transport})
		s.writeSystemLog("error", "camera", "camera_recording_start_failed", "camera recording start failed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "ffmpeg_start_failed", "error": errText, "rtsp_transport": options.Transport})
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
		_ = <-cmdDone
		s.nvrMu.Lock()
		if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
			delete(s.nvrSessions, cameraID)
		}
		s.nvrMu.Unlock()
		close(sess.done)
		return
	default:
	}

	res, err := s.db.Exec(`INSERT INTO camera_recordings (camera_id, camera_name, file_path, started_at, status)
		VALUES (?, ?, ?, datetime('now'), 'recording')`, cameraID, cameraName, filePath)
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-cmdDone:
		case <-time.After(2 * time.Second):
		}
		_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)
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

	log.Printf("[NVR] recording cam %d transport=%s -> %s (id=%d)", cameraID, activeTransport, tmpFilePath, recID)
	s.writeDeviceLog(cameraID, "notice", "camera-recording", "camera recording started", "Camera recording started", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "file_path": filePath, "recording_bitrate": bitrateKbps, "rtsp_transport": activeTransport})
	s.writeSystemLog("notice", "camera", "camera_recording_started", "camera recording started", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "file_path": filePath, "recording_bitrate": bitrateKbps, "rtsp_transport": activeTransport})
	_ = <-cmdDone

	actualFilePath := filePath
	if fi, err := os.Stat(tmpFilePath); err != nil || fi == nil || fi.Size() == 0 {
		_ = os.Remove(tmpFilePath)
		dur := int(time.Since(sess.startedAt).Seconds())
		_, _ = s.db.Exec(`UPDATE camera_recordings SET ended_at=datetime('now'), duration_sec=?, file_size=0, status='failed' WHERE id=?`, dur, recID)
		_, _ = s.db.Exec(`UPDATE cameras SET recording_enabled=0 WHERE id=?`, cameraID)

		s.nvrMu.Lock()
		if cur, ok := s.nvrSessions[cameraID]; ok && cur == sess {
			delete(s.nvrSessions, cameraID)
		}
		s.nvrMu.Unlock()
		close(sess.done)

		log.Printf("[NVR] recording failed cam %d: no output data produced", cameraID)
		s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording failed", "Camera recording failed", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "duration_sec": dur, "reason": "no_output_data", "rtsp_transport": activeTransport})
		s.writeSystemLog("error", "camera", "camera_recording_failed", "camera recording failed: no output data", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "duration_sec": dur, "reason": "no_output_data", "rtsp_transport": activeTransport})
		return
	}

	remuxStderr := &cappedTextBuffer{}
	remuxCmd := exec.Command(ffmpegBin,
		"-i", tmpFilePath,
		"-c", "copy",
		"-movflags", "+faststart",
		"-f", "mp4",
		"-y", filePath,
	)
	remuxCmd.Stderr = remuxStderr
	if remuxErr := remuxCmd.Run(); remuxErr != nil {
		fallbackTSPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".ts"
		_ = os.Remove(fallbackTSPath)
		if renameErr := os.Rename(tmpFilePath, fallbackTSPath); renameErr != nil {
			actualFilePath = tmpFilePath
			log.Printf("[NVR] remux failed cam %d and TS rename failed: remux=%v rename=%v", cameraID, remuxErr, renameErr)
		} else {
			actualFilePath = fallbackTSPath
		}
		_, _ = s.db.Exec(`UPDATE camera_recordings SET file_path=? WHERE id=?`, actualFilePath, recID)
		errText := remuxErr.Error()
		if stderrText := strings.TrimSpace(remuxStderr.String()); stderrText != "" {
			errText += " | " + stderrText
		}
		log.Printf("[NVR] remux failed cam %d: %v, keeping TS file %s", cameraID, remuxErr, actualFilePath)
		s.writeDeviceLog(cameraID, "warning", "camera-recording", "camera recording remux failed", "Camera recording remux failed", map[string]interface{}{"camera_name": cameraName, "record_id": recID, "file_path": actualFilePath, "reason": "remux_failed", "error": errText})
		s.writeSystemLog("warning", "camera", "camera_recording_remux_failed", "camera recording remux failed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "record_id": recID, "file_path": actualFilePath, "reason": "remux_failed", "error": errText})
	} else {
		_ = os.Remove(tmpFilePath)
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
			go func() {
				if err := s.nvrStart(cameraID); err != nil {
					log.Printf("[NVR] restart failed cam %d: %v", cameraID, err)
					s.writeDeviceLog(cameraID, "error", "camera-recording", "camera recording restart failed", "Camera recording restart failed", map[string]interface{}{"camera_name": cameraName, "reason": "restart_failed", "error": err.Error()})
					s.writeSystemLog("error", "camera", "camera_recording_restart_failed", "camera recording restart failed", map[string]interface{}{"camera_id": cameraID, "camera_name": cameraName, "reason": "restart_failed", "error": err.Error()})
				}
			}()
		}
	}
}
