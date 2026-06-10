package camera

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultGo2RTCAPI  = "http://127.0.0.1:1984"
	defaultGo2RTCRTSP = "rtsp://127.0.0.1:8554"
)

var go2RTCWSUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	CheckOrigin:      func(r *http.Request) bool { return true },
}

type Go2RTCState struct {
	Available bool   `json:"available"`
	Running   bool   `json:"running"`
	API       string `json:"api"`
	Binary    string `json:"binary,omitempty"`
	Error     string `json:"error,omitempty"`
}

type go2RTCLogWriter struct {
	prefix string
	buf    []byte
}

func (w *go2RTCLogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := IndexBytes(w.buf, []byte("\n"))
		if idx < 0 {
			break
		}
		line := strings.TrimRight(string(w.buf[:idx]), "\r")
		if strings.TrimSpace(line) != "" {
			log.Printf("[%s] %s", w.prefix, RedactSensitiveText(line))
		}
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
}

func go2RTCAPIBase() string {
	if value := strings.TrimSpace(os.Getenv("NMS_GO2RTC_API")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return defaultGo2RTCAPI
}

func go2RTCRTSPBase() string {
	if value := strings.TrimSpace(os.Getenv("NMS_GO2RTC_RTSP")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return defaultGo2RTCRTSP
}

func go2RTCBinaryNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"go2rtc.exe", "go2rtc_windows_amd64.exe", "go2rtc"}
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			return []string{"go2rtc", "go2rtc_linux_arm64"}
		default:
			return []string{"go2rtc", "go2rtc_linux_amd64"}
		}
	default:
		return []string{"go2rtc"}
	}
}

func FindGo2RTCBin() string {
	if value := strings.TrimSpace(os.Getenv("NMS_GO2RTC_BIN")); value != "" {
		if _, err := os.Stat(value); err == nil {
			return value
		}
	}

	names := go2RTCBinaryNames()
	if self, err := os.Executable(); err == nil {
		base := filepath.Dir(self)
		for _, name := range names {
			for _, dir := range []string{filepath.Join(base, "bin"), base} {
				candidate := filepath.Join(dir, name)
				if _, err := os.Stat(candidate); err == nil {
					if runtime.GOOS != "windows" {
						_ = os.Chmod(candidate, 0o755)
					}
					return candidate
				}
			}
		}
	}

	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func go2RTCWorkDir() string {
	if self, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(self), "data", "go2rtc")
	}
	return filepath.Join("data", "go2rtc")
}

func go2RTCWebRTCCandidates() []string {
	seen := make(map[string]bool)
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		candidates = append(candidates, value)
	}

	for _, value := range strings.FieldsFunc(os.Getenv("NMS_GO2RTC_WEBRTC_CANDIDATES"), func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		add(value)
	}
	if len(candidates) > 0 {
		return candidates
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				add(v4.String() + ":8555")
			}
		}
	}
	sort.Strings(candidates)
	return candidates
}

func writeGo2RTCConfig(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	configPath := filepath.Join(dir, "go2rtc.yaml")
	lines := []string{
		"api:",
		"  listen: \"127.0.0.1:1984\"",
		"rtsp:",
		"  listen: \"127.0.0.1:8554\"",
		"webrtc:",
		"  listen: \":8555\"",
	}
	if candidates := go2RTCWebRTCCandidates(); len(candidates) > 0 {
		lines = append(lines, "  candidates:")
		for _, candidate := range candidates {
			lines = append(lines, "    - \""+strings.ReplaceAll(candidate, "\"", "")+"\"")
		}
	}
	lines = append(lines,
		"log:",
		"  level: \"warn\"",
		"  output: \"stdout\"",
		"",
	)
	content := strings.Join(lines, "\n")
	return configPath, os.WriteFile(configPath, []byte(content), 0o644)
}

func (s *Service) Go2RTCState(ctx context.Context) Go2RTCState {
	bin := FindGo2RTCBin()
	state := Go2RTCState{
		Available: bin != "",
		API:       go2RTCAPIBase(),
		Binary:    bin,
	}
	if err := s.go2RTCHealth(ctx); err == nil {
		state.Running = true
	} else {
		state.Error = err.Error()
	}
	return state
}

func (s *Service) ensureGo2RTC(ctx context.Context) error {
	if err := s.go2RTCHealth(ctx); err == nil {
		return nil
	}

	s.go2rtcMu.Lock()
	defer s.go2rtcMu.Unlock()

	if err := s.go2RTCHealth(ctx); err == nil {
		return nil
	}
	if s.go2rtcCmd != nil && s.go2rtcCmd.Process != nil {
		if err := s.waitForGo2RTCHealth(ctx, 18*time.Second); err == nil {
			return nil
		}
		return ErrGo2RTCUnavailable
	}

	bin := FindGo2RTCBin()
	if bin == "" {
		return ErrGo2RTCNotFound
	}

	workDir := go2RTCWorkDir()
	configPath, err := writeGo2RTCConfig(workDir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGo2RTCUnavailable, err)
	}

	cmd := exec.Command(bin, "-c", configPath)
	cmd.Dir = workDir
	cmd.Stdout = &go2RTCLogWriter{prefix: "go2rtc"}
	cmd.Stderr = &go2RTCLogWriter{prefix: "go2rtc"}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", ErrGo2RTCUnavailable, err)
	}
	s.go2rtcCmd = cmd
	go func() {
		err := cmd.Wait()
		s.go2rtcMu.Lock()
		if s.go2rtcCmd == cmd {
			s.go2rtcCmd = nil
		}
		s.go2rtcMu.Unlock()
		if err != nil {
			log.Printf("[go2rtc] exited: %v", err)
		}
	}()

	if err := s.waitForGo2RTCHealth(ctx, 18*time.Second); err == nil {
		log.Printf("[go2rtc] started with API %s", go2RTCAPIBase())
		return nil
	}
	return ErrGo2RTCUnavailable
}

func (s *Service) waitForGo2RTCHealth(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := s.go2RTCHealth(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return ErrGo2RTCUnavailable
}

func (s *Service) go2RTCHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, go2RTCAPIBase()+"/api/streams", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}
	return fmt.Errorf("go2rtc health HTTP %d", resp.StatusCode)
}

func go2RTCStreamName(id, source string) string {
	h := fnvHash(source)
	return fmt.Sprintf("nms_cam_%s_%08x", id, h)
}

func fnvHash(value string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(value); i++ {
		h ^= uint32(value[i])
		h *= 16777619
	}
	return h
}

func appendGo2RTCFragmentParams(raw string, params ...string) string {
	out := strings.TrimSpace(raw)
	if out == "" {
		return out
	}
	for _, param := range params {
		param = strings.Trim(strings.TrimSpace(param), "#")
		if param == "" {
			continue
		}
		out += "#" + param
	}
	return out
}

func appendGo2RTCSourceParams(raw string) string {
	return appendGo2RTCFragmentParams(raw, "backchannel=0")
}

func buildGo2RTCSource(raw string) string {
	return appendGo2RTCSourceParams(raw)
}

func buildGo2RTCRecordingSource(raw string) string {
	return buildGo2RTCSource(raw)
}

func (s *Service) EnsureWebRTCStream(ctx context.Context, id string) (string, error) {
	var rtspURL, username, passwordEnc string
	err := s.db.QueryRow(`
		SELECT COALESCE(NULLIF(preview_rtsp_url,''), rtsp_url, ''), COALESCE(username,''),
		       COALESCE(password_encrypted,'')
		FROM cameras WHERE id=?`, id).Scan(&rtspURL, &username, &passwordEnc)
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
	source := BuildAuthURL(RTSPCleanURL(rtspURL), username, password)
	source = buildGo2RTCSource(source)
	streamName := go2RTCStreamName(id, source)

	if err := s.ensureGo2RTC(ctx); err != nil {
		return "", err
	}
	if err := s.registerGo2RTCStream(ctx, streamName, source); err != nil {
		return "", err
	}
	return streamName, nil
}

func (s *Service) EnsureLocalRTSPStream(ctx context.Context, id, rtspURL, username, password string) (string, error) {
	if strings.TrimSpace(rtspURL) == "" {
		return "", ErrStreamUnavailable
	}

	source := BuildAuthURL(RTSPCleanURL(rtspURL), username, password)
	source = buildGo2RTCRecordingSource(source)
	streamName := go2RTCStreamName(id, source)

	if err := s.ensureGo2RTC(ctx); err != nil {
		return "", err
	}
	if err := s.registerGo2RTCStream(ctx, streamName, source); err != nil {
		return "", err
	}
	return go2RTCRTSPBase() + "/" + url.PathEscape(streamName), nil
}

func (s *Service) EnsureRecordingRTSPStream(ctx context.Context, id, rtspURL, username, password string) (string, error) {
	return s.EnsureLocalRTSPStream(ctx, id, rtspURL, username, password)
}

func (s *Service) registerGo2RTCStream(ctx context.Context, streamName, source string) error {
	endpoint := go2RTCRegisterEndpoint(streamName, source)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGo2RTCUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("%w: register stream HTTP %d: %s", ErrGo2RTCUnavailable, resp.StatusCode, RedactSensitiveText(string(body)))
}

func go2RTCRegisterEndpoint(streamName, source string) string {
	return go2RTCAPIBase() + "/api/streams?name=" + url.QueryEscape(streamName) + "&src=" + url.QueryEscape(source)
}

func (s *Service) ProxyGo2RTCWebSocket(w http.ResponseWriter, r *http.Request, streamName string) error {
	wsURL := strings.TrimPrefix(go2RTCAPIBase(), "http")
	if strings.HasPrefix(wsURL, "s://") {
		wsURL = "ws" + wsURL
	} else {
		wsURL = "ws" + wsURL
	}
	wsURL += "/api/ws?src=" + url.QueryEscape(streamName)

	upstream, _, err := websocket.DefaultDialer.DialContext(r.Context(), wsURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGo2RTCUnavailable, err)
	}
	defer upstream.Close()

	client, err := go2RTCWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	defer client.Close()

	done := make(chan struct{}, 2)
	clientModeLogged := false
	copyWS := func(label string, dst, src *websocket.Conn) {
		defer func() { done <- struct{}{} }()
		for {
			msgType, payload, err := src.ReadMessage()
			if err != nil {
				return
			}
			if label == "client" && !clientModeLogged && msgType == websocket.TextMessage {
				text := strings.TrimSpace(string(payload))
				mode := ""
				switch {
				case strings.Contains(text, `"type":"webrtc/offer"`) || strings.Contains(text, `"type": "webrtc/offer"`):
					mode = "webrtc"
				case strings.Contains(text, `"type":"mse"`) || strings.Contains(text, `"type": "mse"`):
					mode = "mse"
				}
				if mode != "" {
					clientModeLogged = true
					log.Printf("[go2rtc] preview client mode src=%s mode=%s", streamName, mode)
				}
			}
			if label == "upstream" && msgType == websocket.TextMessage {
				text := strings.TrimSpace(string(payload))
				if strings.Contains(text, `"type":"error"`) || strings.Contains(text, `"type": "error"`) {
					log.Printf("[go2rtc] websocket upstream error src=%s msg=%s", streamName, RedactSensitiveText(text))
				}
			}
			if err := dst.WriteMessage(msgType, payload); err != nil {
				return
			}
		}
	}

	go copyWS("client", upstream, client)
	go copyWS("upstream", client, upstream)
	<-done
	return nil
}

func IsGo2RTCError(err error) bool {
	return errors.Is(err, ErrGo2RTCNotFound) || errors.Is(err, ErrGo2RTCUnavailable)
}
