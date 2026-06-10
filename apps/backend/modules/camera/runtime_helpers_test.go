package camera

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRedactSensitiveText(t *testing.T) {
	input := `open rtsp://admin:Admin123@192.168.10.207:554/ch01?token=abc&password=secret&community=private&safe=1`

	got := RedactSensitiveText(input)

	for _, leaked := range []string{"admin:Admin123", "token=abc", "password=secret", "community=private"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("expected %q to be redacted from %q", leaked, got)
		}
	}
	if !strings.Contains(got, "rtsp://<credentials>@192.168.10.207:554/ch01") {
		t.Fatalf("expected URL credentials to be redacted, got %q", got)
	}
	if !strings.Contains(got, "safe=1") {
		t.Fatalf("expected non-sensitive query value to be preserved, got %q", got)
	}
}

func TestCameraResponsesRedactRTSPCredentials(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE cameras (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT,
			ip_address TEXT,
			port INTEGER DEFAULT 554,
			rtsp_url TEXT,
			preview_rtsp_url TEXT DEFAULT '',
			recording_rtsp_url TEXT DEFAULT '',
			rtsp_transport TEXT DEFAULT 'tcp',
			rtsp_udp_min_port INTEGER DEFAULT 0,
			rtsp_udp_max_port INTEGER DEFAULT 0,
			onvif_url TEXT,
			username TEXT,
			password_encrypted TEXT,
			manufacturer TEXT,
			model TEXT,
			firmware TEXT,
			supports_ptz BOOLEAN DEFAULT 0,
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			stream_type TEXT DEFAULT 'mjpeg',
			monitor_display INTEGER DEFAULT 0,
			monitor_order INTEGER DEFAULT 0,
			recording_source TEXT DEFAULT 'rtsp',
			recording_bitrate_kbps INTEGER DEFAULT 0,
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO cameras (
			name, location, ip_address, port, rtsp_url, preview_rtsp_url,
			recording_rtsp_url, username, monitor_display
		) VALUES (
			'Lobby', 'HQ', '192.0.2.10', 554,
			'rtsp://admin:Admin123@192.0.2.10/main?token=abc&safe=1',
			'rtsp://viewer:View123@192.0.2.10/sub?api_key=key',
			'rtsp://rec:Rec123@192.0.2.10/record?password=secret',
			'admin', 1
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(db, "", RuntimeHooks{})
	camera, err := service.GetCamera("1")
	if err != nil {
		t.Fatal(err)
	}
	cameras, err := service.ListCameras()
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := service.ListMonitorCameras()
	if err != nil {
		t.Fatal(err)
	}

	body := strings.Join([]string{
		camera.RTSPUrl,
		camera.PreviewRTSPUrl,
		camera.RecordingRTSPUrl,
		cameras[0].RTSPUrl,
		cameras[0].PreviewRTSPUrl,
		cameras[0].RecordingRTSPUrl,
		monitor[0].RTSPUrl,
		monitor[0].PreviewRTSPUrl,
	}, "\n")
	for _, leaked := range []string{
		"admin:Admin123",
		"viewer:View123",
		"rec:Rec123",
		"token=abc",
		"api_key=key",
		"password=secret",
	} {
		if strings.Contains(body, leaked) {
			t.Fatalf("camera response leaked %q in %s", leaked, body)
		}
	}
	if !strings.Contains(body, "safe=1") {
		t.Fatalf("expected non-sensitive RTSP query value to be preserved, got %s", body)
	}
}

func TestDisableHardwareDecodeFallsBackToCPU(t *testing.T) {
	hwAccel = hwAccelInfo{cuda: true, qsv: true, vaapi: true, d3d11: true}
	t.Cleanup(func() { hwAccel = hwAccelInfo{} })

	modes := hwDecodeModes()
	if len(modes) != 2 || modes[0].name != "cuda" || modes[1].name != "cpu" {
		t.Fatalf("expected cuda then cpu before disable, got %#v", modes)
	}

	disableHardwareDecodeMode(modes[0], "Hardware is lacking required capabilities")

	modes = hwDecodeModes()
	if len(modes) != 1 || modes[0].name != "cpu" {
		t.Fatalf("expected only cpu after hardware disable, got %#v", modes)
	}
	if hasActiveHardwareDecodeMode() {
		t.Fatal("expected hardware decode to be inactive after disable")
	}
}

func TestNormalizeCameraInputKeepsLegacyRTSPAsPreviewFallback(t *testing.T) {
	input := normalizeCameraInput(CameraInput{
		RTSPUrl: " rtsp://camera/main ",
	})

	if input.RTSPUrl != "rtsp://camera/main" {
		t.Fatalf("expected legacy rtsp_url to be trimmed, got %q", input.RTSPUrl)
	}
	if input.PreviewRTSPUrl != "rtsp://camera/main" {
		t.Fatalf("expected preview_rtsp_url to fall back to rtsp_url, got %q", input.PreviewRTSPUrl)
	}
	if input.RecordingRTSPUrl != "" {
		t.Fatalf("expected blank recording_rtsp_url to remain blank for runtime fallback, got %q", input.RecordingRTSPUrl)
	}
}

func TestNormalizeCameraInputKeepsSeparatePreviewAndRecordingStreams(t *testing.T) {
	input := normalizeCameraInput(CameraInput{
		PreviewRTSPUrl:   " rtsp://camera/substream ",
		RecordingRTSPUrl: " rtsp://camera/mainstream ",
	})

	if input.RTSPUrl != "rtsp://camera/substream" {
		t.Fatalf("expected legacy rtsp_url to mirror preview stream, got %q", input.RTSPUrl)
	}
	if input.PreviewRTSPUrl != "rtsp://camera/substream" {
		t.Fatalf("unexpected preview stream %q", input.PreviewRTSPUrl)
	}
	if input.RecordingRTSPUrl != "rtsp://camera/mainstream" {
		t.Fatalf("unexpected recording stream %q", input.RecordingRTSPUrl)
	}
}

func TestFFmpegArgsCanUseUDPTransportFallback(t *testing.T) {
	mode := ffmpegDecodeMode{name: "cpu"}

	streamArgs := strings.Join(BuildFFmpegStreamArgsForModeTransport("rtsp://encoder/ch2", "frame", 5, 5, 0, mode, "udp"), " ")
	if !strings.Contains(streamArgs, "-rtsp_transport udp") {
		t.Fatalf("expected UDP stream transport, got %q", streamArgs)
	}

	snapshotArgs := strings.Join(BuildFFmpegSnapshotArgsForModeTransport("rtsp://encoder/ch2", mode, "udp"), " ")
	if !strings.Contains(snapshotArgs, "-rtsp_transport udp") {
		t.Fatalf("expected UDP snapshot transport, got %q", snapshotArgs)
	}

	defaultArgs := strings.Join(BuildFFmpegStreamArgsForModeTransport("rtsp://encoder/ch2", "frame", 5, 5, 0, mode, "invalid"), " ")
	if !strings.Contains(defaultArgs, "-rtsp_transport tcp") {
		t.Fatalf("expected invalid transport to default to TCP, got %q", defaultArgs)
	}
}

func TestFFmpegArgsCanUseUDPPortRange(t *testing.T) {
	mode := ffmpegDecodeMode{name: "cpu"}
	options := rtspRuntimeOptions{Transport: "udp", UDPMinPort: 30000, UDPMaxPort: 30200}

	args := strings.Join(BuildFFmpegStreamArgsForModeTransportOptions("rtsp://encoder/stream1", "frame", 5, 5, 0, mode, "udp", options), " ")

	for _, want := range []string{"-rtsp_transport udp", "-min_port 30000", "-max_port 30200"} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in ffmpeg args, got %q", want, args)
		}
	}
}

func TestFFmpegStreamArgsUseLowLatencyPreviewOptions(t *testing.T) {
	mode := ffmpegDecodeMode{name: "cpu"}

	args := strings.Join(BuildFFmpegStreamArgsForModeTransportOptions("rtsp://encoder/stream1", "frame", 5, 5, 1200, mode, "tcp", rtspRuntimeOptions{}), " ")

	for _, want := range []string{
		"-fflags nobuffer",
		"-flags low_delay",
		"-avioflags direct",
		"-analyzeduration 0",
		"-probesize 32768",
		"-rtbufsize 256k",
		"-max_delay 0",
		"-reorder_queue_size 0",
		"-use_wallclock_as_timestamps 1",
		"-an",
		"-vf fps=5,scale='min(iw,1280)':'min(ih,720)':force_original_aspect_ratio=decrease",
		"-vsync 0",
		"-flush_packets 1",
		"-muxdelay 0",
		"-muxpreload 0",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in ffmpeg args, got %q", want, args)
		}
	}
}

func TestFFmpegStreamArgsUseTunedQualityWithoutBitrateCap(t *testing.T) {
	mode := ffmpegDecodeMode{name: "cpu"}

	args := strings.Join(BuildFFmpegStreamArgsForModeTransportOptions("rtsp://encoder/stream1", "frame", 15, 3, 0, mode, "tcp", rtspRuntimeOptions{}), " ")

	if !strings.Contains(args, "-q:v 3") {
		t.Fatalf("expected tuned MJPEG quality in ffmpeg args, got %q", args)
	}
	if strings.Contains(args, "scale='min") {
		t.Fatalf("expected uncapped preview to avoid downscale, got %q", args)
	}
}

func TestPreviewSourceDefaultsToDirectRTSP(t *testing.T) {
	t.Setenv("NMS_CAMERA_PREVIEW_SOURCE", "")
	if got := cameraPreviewSourceMode(); got != "direct" {
		t.Fatalf("expected default preview source to be direct, got %q", got)
	}

	t.Setenv("NMS_CAMERA_PREVIEW_SOURCE", "go2rtc")
	if got := cameraPreviewSourceMode(); got != "go2rtc" {
		t.Fatalf("expected go2rtc preview source, got %q", got)
	}

	t.Setenv("NMS_CAMERA_PREVIEW_SOURCE", "auto")
	if got := cameraPreviewSourceMode(); got != "auto" {
		t.Fatalf("expected auto preview source, got %q", got)
	}
}

func TestRecordingSourceDefaultsToDirectRTSP(t *testing.T) {
	t.Setenv("NMS_CAMERA_RECORDING_SOURCE", "")
	if got := cameraRecordingSourceMode(); got != "direct" {
		t.Fatalf("expected default recording source to be direct, got %q", got)
	}

	t.Setenv("NMS_CAMERA_RECORDING_SOURCE", "go2rtc")
	if got := cameraRecordingSourceMode(); got != "go2rtc" {
		t.Fatalf("expected go2rtc recording source, got %q", got)
	}

	t.Setenv("NMS_CAMERA_RECORDING_SOURCE", "auto")
	if got := cameraRecordingSourceMode(); got != "auto" {
		t.Fatalf("expected auto recording source, got %q", got)
	}
}

func TestRecordingArgsDisableAudioProbeByDefault(t *testing.T) {
	t.Setenv("NMS_CAMERA_RECORD_AUDIO", "")
	args := strings.Join(buildNVRFFmpegArgs("ffmpeg", "rtsp://encoder/stream1", "out.tmp.ts", 0, "tcp", rtspRuntimeOptions{}), " ")

	if !strings.Contains(args, "-map 0:v:0 -an") {
		t.Fatalf("expected recording args to map video only by default, got %q", args)
	}
	if strings.Contains(args, "0:a:0") {
		t.Fatalf("expected recording args not to map audio by default, got %q", args)
	}
	if strings.Contains(args, "rw_timeout") {
		t.Fatalf("expected recording args not to use unsupported rw_timeout option, got %q", args)
	}
	if !strings.Contains(args, "-f mpegts -y out.tmp.ts") {
		t.Fatalf("expected recording args to write a temporary MPEG-TS file, got %q", args)
	}
}

func TestPreviewQualityDefaultsToHighQualityAndCanBeTuned(t *testing.T) {
	t.Setenv("NMS_CAMERA_PREVIEW_QUALITY", "")
	if got := cameraPreviewQuality(); got != 2 {
		t.Fatalf("expected default preview quality 2, got %d", got)
	}

	t.Setenv("NMS_CAMERA_PREVIEW_QUALITY", "2")
	if got := cameraPreviewQuality(); got != 2 {
		t.Fatalf("expected preview quality 2, got %d", got)
	}

	t.Setenv("NMS_CAMERA_PREVIEW_QUALITY", "99")
	if got := cameraPreviewQuality(); got != 15 {
		t.Fatalf("expected preview quality to clamp at 15, got %d", got)
	}
}

func TestFFmpegHLSArgsUseStreamCopyWithShortPlaylist(t *testing.T) {
	mode := ffmpegDecodeMode{name: "copy"}
	args := strings.Join(BuildFFmpegHLSArgsForModeTransportOptions(
		"rtsp://encoder/stream1",
		"C:/nms/hls/index.m3u8",
		"C:/nms/hls/seg_%06d.ts",
		mode,
		"tcp",
		rtspRuntimeOptions{},
	), " ")

	for _, want := range []string{
		"-fflags nobuffer",
		"-flags low_delay",
		"-c:v copy",
		"-f hls",
		"-hls_time 1",
		"-hls_list_size 3",
		"-hls_flags delete_segments+omit_endlist+program_date_time",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("expected %q in HLS args, got %q", want, args)
		}
	}
}

func TestMjpegBroadcastKeepsLatestFrameForSlowClient(t *testing.T) {
	stream := newMjpegStream(nil, "1", "pool", 0)
	ch := stream.addClient()

	stream.broadcast([]byte("first"))
	stream.broadcast([]byte("second"))
	stream.broadcast([]byte("third"))

	got := string(<-ch)
	if got != "third" {
		t.Fatalf("expected slow client to receive latest frame, got %q", got)
	}

	select {
	case extra := <-ch:
		t.Fatalf("expected old frames to be dropped, got extra frame %q", string(extra))
	default:
	}
}

func TestRTSPTransportPreferenceControlsFallbacks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "auto", in: "auto", want: "tcp,udp"},
		{name: "blank", in: "", want: "tcp"},
		{name: "tcp", in: "tcp", want: "tcp"},
		{name: "udp", in: "udp", want: "udp"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Join(rtspTransportsForPreference(tt.in), ",")
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestGo2RTCSourceParamsDisableBackchannelOnly(t *testing.T) {
	got := appendGo2RTCSourceParams("rtsp://user:pass@192.0.2.10/stream1")

	for _, want := range []string{"rtsp://user:pass@192.0.2.10/stream1", "#backchannel=0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in go2rtc source, got %q", want, got)
		}
	}
	if strings.Contains(got, "#media=video") {
		t.Fatalf("expected preview and recording to share one go2rtc source, got %q", got)
	}
	if strings.Contains(got, "#transport=") {
		t.Fatalf("expected go2rtc direct source to avoid fixed transport, got %q", got)
	}
}

func TestGo2RTCPreviewSourceDoesNotUseFFmpegTranscode(t *testing.T) {
	got := buildGo2RTCSource("rtsp://user:pass@192.0.2.10/stream1")

	for _, want := range []string{"rtsp://user:pass@192.0.2.10/stream1", "#backchannel=0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in go2rtc preview source, got %q", want, got)
		}
	}
	if strings.Contains(got, "ffmpeg:") || strings.Contains(got, "#media=video") || strings.Contains(got, "#video=h264") || strings.Contains(got, "#input=rtsp/udp") {
		t.Fatalf("expected MSE preview source to stay direct RTSP without FFmpeg transcode, got %q", got)
	}
}

func TestGo2RTCConfigDoesNotAttachFFmpegForPreview(t *testing.T) {
	path, err := writeGo2RTCConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "ffmpeg:") {
		t.Fatalf("expected go2rtc preview config not to include ffmpeg block, got %q", content)
	}
}

func TestGo2RTCRegisterEndpointUsesStreamConfigAPI(t *testing.T) {
	t.Setenv("NMS_GO2RTC_API", "http://127.0.0.1:1984")

	got := go2RTCRegisterEndpoint("camera 8", "rtsp://example.test/stream1#media=video#backchannel=0")

	if !strings.Contains(got, "/api/streams?") {
		t.Fatalf("expected go2rtc streams API endpoint, got %q", got)
	}
	if !strings.Contains(got, "name=camera+8") {
		t.Fatalf("expected stream name query parameter, got %q", got)
	}
	if strings.Contains(got, "dst=") {
		t.Fatalf("expected registration endpoint to avoid stream-to-camera dst API, got %q", got)
	}
}

func TestGo2RTCStreamNameChangesWithSource(t *testing.T) {
	a := go2RTCStreamName("7", "rtsp://example.test/stream1#media=video")
	b := go2RTCStreamName("7", "rtsp://example.test/stream2#media=video")

	if a == b {
		t.Fatalf("expected stream name to change with source, got %q", a)
	}
	if !strings.HasPrefix(a, "nms_cam_7_") {
		t.Fatalf("unexpected stream name %q", a)
	}
}

func TestCreateCameraAllowsSameIPAddressWithDifferentRTSP(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE cameras (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			location TEXT,
			ip_address TEXT,
			port INTEGER DEFAULT 554,
			rtsp_url TEXT,
			preview_rtsp_url TEXT DEFAULT '',
			recording_rtsp_url TEXT DEFAULT '',
			rtsp_transport TEXT DEFAULT 'tcp',
			rtsp_udp_min_port INTEGER DEFAULT 0,
			rtsp_udp_max_port INTEGER DEFAULT 0,
			onvif_url TEXT,
			username TEXT,
			password_encrypted TEXT,
			manufacturer TEXT,
			model TEXT,
			supports_ptz BOOLEAN DEFAULT 0,
			is_enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'unknown',
			stream_type TEXT DEFAULT 'mjpeg',
			monitor_display INTEGER DEFAULT 0,
			monitor_order INTEGER DEFAULT 0,
			recording_source TEXT DEFAULT 'rtsp',
			recording_bitrate_kbps INTEGER DEFAULT 0,
			last_seen DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(db, "", RuntimeHooks{})
	for _, input := range []CameraInput{
		{Name: "encoder ch1", IPAddress: "192.168.10.50", Port: 554, PreviewRTSPUrl: "rtsp://192.168.10.50/ch1"},
		{Name: "encoder ch2", IPAddress: "192.168.10.50", Port: 554, PreviewRTSPUrl: "rtsp://192.168.10.50/ch2"},
	} {
		if _, err := service.CreateCamera(input); err != nil {
			t.Fatalf("expected same IP with different RTSP URL to be accepted: %v", err)
		}
	}

	count, err := service.CountCameras()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected two camera rows for one encoder IP, got %d", count)
	}
}
