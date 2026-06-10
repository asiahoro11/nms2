package camera

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// hwAccelInfo caches which GPU decode backends ffmpeg supports.
type hwAccelInfo struct {
	once     sync.Once
	mu       sync.Mutex
	cuda     bool
	qsv      bool
	vaapi    bool
	d3d11    bool
	disabled map[string]bool
}

var hwAccel hwAccelInfo

type ffmpegDecodeMode struct {
	name      string
	inputArgs []string
	vfFilter  string
}

type rtspRuntimeOptions struct {
	Transport  string
	UDPMinPort int
	UDPMaxPort int
}

func rtspTransportFallbacks() []string {
	return []string{"tcp", "udp"}
}

func rtspTransportsForPreference(preference string) []string {
	switch normalizeRTSPTransportPreference(preference) {
	case "udp":
		return []string{"udp"}
	case "auto":
		return rtspTransportFallbacks()
	default:
		return []string{"tcp"}
	}
}

func normalizeRTSPTransportPreference(transport string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "auto":
		return "auto"
	case "tcp":
		return "tcp"
	case "udp":
		return "udp"
	default:
		return "tcp"
	}
}

func normalizeRTSPTransport(transport string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "udp":
		return "udp"
	default:
		return "tcp"
	}
}

func normalizeRTSPRuntimeOptions(options rtspRuntimeOptions) rtspRuntimeOptions {
	options.Transport = normalizeRTSPTransportPreference(options.Transport)
	if options.UDPMinPort < 0 {
		options.UDPMinPort = 0
	}
	if options.UDPMaxPort < 0 {
		options.UDPMaxPort = 0
	}
	if options.UDPMinPort > 65535 {
		options.UDPMinPort = 65535
	}
	if options.UDPMaxPort > 65535 {
		options.UDPMaxPort = 65535
	}
	if options.UDPMinPort > 0 && options.UDPMaxPort > 0 && options.UDPMinPort > options.UDPMaxPort {
		options.UDPMinPort, options.UDPMaxPort = options.UDPMaxPort, options.UDPMinPort
	}
	return options
}

func (o rtspRuntimeOptions) transports() []string {
	return rtspTransportsForPreference(o.Transport)
}

func (o rtspRuntimeOptions) inputArgs(transport string) []string {
	normalized := normalizeRTSPRuntimeOptions(o)
	transport = normalizeRTSPTransport(transport)
	args := []string{"-rtsp_transport", transport}
	if transport == "udp" && normalized.UDPMinPort > 0 && normalized.UDPMaxPort > 0 {
		args = append(args, "-min_port", fmt.Sprintf("%d", normalized.UDPMinPort), "-max_port", fmt.Sprintf("%d", normalized.UDPMaxPort))
	}
	return args
}

func (m ffmpegDecodeMode) isHardware() bool {
	return m.name != "" && m.name != "cpu"
}

var (
	urlCredentialsPattern = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)([^@\s/]+@)`)
	urlSecretQueryPattern = regexp.MustCompile(`(?i)([?&](?:user|username|pass|password|pwd|token|access_token|refresh_token|auth|authorization|key|api_key|client_secret|secret|community|snmp_community)=)[^&\s]+`)
)

// FindFFmpegBin returns the path to the ffmpeg binary.
func FindFFmpegBin() string {
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}

	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), "bin", exe)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		candidate = filepath.Join(filepath.Dir(self), exe)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if path, err := exec.LookPath(exe); err == nil {
		return path
	}

	log.Println("[Camera] ffmpeg not found, attempting auto-download...")
	if downloaded := autoDownloadFFmpeg(); downloaded != "" {
		log.Printf("[Camera] ffmpeg auto-download succeeded: %s", downloaded)
		return downloaded
	}
	return ""
}

func autoDownloadFFmpeg() string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	binDir := filepath.Join(filepath.Dir(self), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return ""
	}

	var downloadURL, archiveName, exeInArchive string
	switch runtime.GOOS {
	case "windows":
		downloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
		archiveName = "ffmpeg-win64.zip"
		exeInArchive = "ffmpeg.exe"
	case "linux":
		switch runtime.GOARCH {
		case "arm64":
			downloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linuxarm64-gpl.tar.xz"
		default:
			downloadURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
		}
		archiveName = "ffmpeg-linux.tar.xz"
		exeInArchive = "ffmpeg"
	default:
		log.Printf("[Camera] auto-download: unsupported OS %s", runtime.GOOS)
		return ""
	}

	destExe := filepath.Join(binDir, filepath.Base(exeInArchive))
	if runtime.GOOS == "windows" {
		destExe = filepath.Join(binDir, "ffmpeg.exe")
	} else {
		destExe = filepath.Join(binDir, "ffmpeg")
	}

	archivePath := filepath.Join(binDir, archiveName)
	log.Printf("[Camera] downloading ffmpeg from %s ...", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		log.Printf("[Camera] download failed: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Camera] download HTTP %d", resp.StatusCode)
		return ""
	}

	f, err := os.Create(archivePath)
	if err != nil {
		return ""
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return ""
	}
	f.Close()

	var extracted string
	if runtime.GOOS == "windows" {
		extracted = extractFFmpegFromZip(archivePath, binDir, exeInArchive)
	} else {
		extracted = extractFFmpegFromTarXz(archivePath, binDir, exeInArchive)
	}
	_ = os.Remove(archivePath)
	if extracted == "" {
		return ""
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(extracted, 0o755)
	}
	_ = destExe
	return extracted
}

func extractFFmpegFromZip(archivePath, destDir, targetName string) string {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		log.Printf("[Camera] zip open failed: %v", err)
		return ""
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.EqualFold(filepath.Base(f.Name), targetName) && !f.FileInfo().IsDir() {
			destPath := filepath.Join(destDir, targetName)
			rc, err := f.Open()
			if err != nil {
				continue
			}
			out, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				continue
			}
			_, _ = io.Copy(out, rc)
			out.Close()
			rc.Close()
			return destPath
		}
	}
	log.Printf("[Camera] %s not found in zip", targetName)
	return ""
}

func extractFFmpegFromTarXz(archivePath, destDir, targetName string) string {
	cmd := exec.Command("tar", "-xJf", archivePath, "--strip-components=2",
		"--wildcards", "*/bin/"+targetName, "-C", destDir)
	if err := cmd.Run(); err != nil {
		cmd2 := exec.Command("sh", "-c",
			fmt.Sprintf("tar -xJf %q -C %q --wildcards '*/bin/%s' --strip-components=2 2>/dev/null || "+
				"tar -xJf %q -C %q --wildcards '*/%s' --strip-components=1 2>/dev/null",
				archivePath, destDir, targetName,
				archivePath, destDir, targetName))
		_ = cmd2.Run()
	}
	candidate := filepath.Join(destDir, targetName)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// ProbeGPU runs "ffmpeg -hwaccels" once and caches results.
func ProbeGPU(ffmpegBin string) {
	hwAccel.once.Do(func() {
		out, err := exec.Command(ffmpegBin, "-hwaccels").Output()
		if err != nil {
			log.Printf("[Camera] GPU probe failed: %v, CPU decode will be used", err)
			return
		}
		s := string(out)
		has := func(sub string) bool {
			return strings.Contains(s, sub)
		}
		hwAccel.cuda = has("cuda")
		hwAccel.qsv = has("qsv")
		hwAccel.vaapi = has("vaapi")
		hwAccel.d3d11 = has("d3d11va")
		log.Printf("[Camera] GPU hwaccel probe: cuda=%v qsv=%v vaapi=%v d3d11va=%v",
			hwAccel.cuda, hwAccel.qsv, hwAccel.vaapi, hwAccel.d3d11)
	})
}

func HasHardwareDecode(ffmpegBin string) bool {
	ProbeGPU(ffmpegBin)
	return hwAccel.cuda || hwAccel.qsv || hwAccel.vaapi || hwAccel.d3d11
}

func GetHardwareAccelState(ffmpegBin string) HardwareAccelState {
	ProbeGPU(ffmpegBin)
	return HardwareAccelState{
		CUDA:   hwAccel.cuda && !isHardwareDecodeModeDisabled("cuda"),
		QSV:    hwAccel.qsv && !isHardwareDecodeModeDisabled("qsv"),
		VAAPI:  hwAccel.vaapi && !isHardwareDecodeModeDisabled("vaapi"),
		D3D11:  hwAccel.d3d11 && !isHardwareDecodeModeDisabled("d3d11va"),
		Active: hasActiveHardwareDecodeMode(),
	}
}

func isHardwareDecodeModeDisabled(name string) bool {
	hwAccel.mu.Lock()
	defer hwAccel.mu.Unlock()
	return hwAccel.disabled != nil && hwAccel.disabled[name]
}

func hasActiveHardwareDecodeMode() bool {
	return (hwAccel.cuda && !isHardwareDecodeModeDisabled("cuda")) ||
		(hwAccel.qsv && !isHardwareDecodeModeDisabled("qsv")) ||
		(hwAccel.vaapi && !isHardwareDecodeModeDisabled("vaapi")) ||
		(hwAccel.d3d11 && !isHardwareDecodeModeDisabled("d3d11va"))
}

func disableHardwareDecodeMode(mode ffmpegDecodeMode, reason string) {
	if !mode.isHardware() {
		return
	}
	hwAccel.mu.Lock()
	defer hwAccel.mu.Unlock()
	if hwAccel.disabled == nil {
		hwAccel.disabled = make(map[string]bool)
	}
	if hwAccel.disabled["cuda"] && hwAccel.disabled["qsv"] && hwAccel.disabled["vaapi"] && hwAccel.disabled["d3d11va"] {
		return
	}
	for _, name := range []string{"cuda", "qsv", "vaapi", "d3d11va"} {
		hwAccel.disabled[name] = true
	}
	if strings.TrimSpace(reason) == "" {
		reason = "no first frame from hardware decoder"
	}
	log.Printf("[Camera] disabling ffmpeg hwaccel after mode=%s decode failure; CPU decode will be used: %s",
		mode.name, strings.TrimSpace(RedactSensitiveText(reason)))
}

func isHardwareDecodeFailure(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "hardware is lacking") ||
		strings.Contains(lower, "hwaccel") ||
		strings.Contains(lower, "failed setup for format") ||
		strings.Contains(lower, "device creation failed") ||
		strings.Contains(lower, "no device available") ||
		strings.Contains(lower, "cuvid") ||
		strings.Contains(lower, "cuda") && strings.Contains(lower, "failed") ||
		strings.Contains(lower, "qsv") && strings.Contains(lower, "failed") ||
		strings.Contains(lower, "vaapi") && strings.Contains(lower, "failed") ||
		strings.Contains(lower, "d3d11") && strings.Contains(lower, "failed")
}

func hwDecodeModes() []ffmpegDecodeMode {
	modes := make([]ffmpegDecodeMode, 0, 2)
	switch {
	case hwAccel.cuda && !isHardwareDecodeModeDisabled("cuda"):
		modes = append(modes, ffmpegDecodeMode{
			name:      "cuda",
			inputArgs: []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"},
			vfFilter:  "scale_cuda=iw:ih,hwdownload,format=nv12",
		})
	case hwAccel.qsv && !isHardwareDecodeModeDisabled("qsv"):
		modes = append(modes, ffmpegDecodeMode{
			name:      "qsv",
			inputArgs: []string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"},
			vfFilter:  "vpp_qsv=iw:ih,hwdownload,format=nv12",
		})
	case hwAccel.vaapi && !isHardwareDecodeModeDisabled("vaapi"):
		modes = append(modes, ffmpegDecodeMode{
			name:      "vaapi",
			inputArgs: []string{"-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi", "-vaapi_device", "/dev/dri/renderD128"},
			vfFilter:  "scale_vaapi=iw:ih,hwdownload,format=nv12",
		})
	case hwAccel.d3d11 && !isHardwareDecodeModeDisabled("d3d11va"):
		modes = append(modes, ffmpegDecodeMode{
			name:      "d3d11va",
			inputArgs: []string{"-hwaccel", "d3d11va"},
		})
	}
	modes = append(modes, ffmpegDecodeMode{name: "cpu"})
	return modes
}

func hwDecodeArgs() (inputArgs []string, vfFilter string) {
	modes := hwDecodeModes()
	if len(modes) == 0 {
		return nil, ""
	}
	return modes[0].inputArgs, modes[0].vfFilter
}

func BuildFFmpegStreamArgs(rtspURL, boundary string, fps, quality, bitrateKbps int) []string {
	modes := hwDecodeModes()
	return BuildFFmpegStreamArgsForMode(rtspURL, boundary, fps, quality, bitrateKbps, modes[0])
}

func BuildFFmpegStreamArgsForMode(rtspURL, boundary string, fps, quality, bitrateKbps int, mode ffmpegDecodeMode) []string {
	return BuildFFmpegStreamArgsForModeTransport(rtspURL, boundary, fps, quality, bitrateKbps, mode, "tcp")
}

func BuildFFmpegStreamArgsForModeTransport(rtspURL, boundary string, fps, quality, bitrateKbps int, mode ffmpegDecodeMode, transport string) []string {
	return BuildFFmpegStreamArgsForModeTransportOptions(rtspURL, boundary, fps, quality, bitrateKbps, mode, transport, rtspRuntimeOptions{})
}

func BuildFFmpegStreamArgsForModeTransportOptions(rtspURL, boundary string, fps, quality, bitrateKbps int, mode ffmpegDecodeMode, transport string, options rtspRuntimeOptions) []string {
	args := options.inputArgs(transport)
	args = append(args, mode.inputArgs...)
	args = append(args,
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-avioflags", "direct",
		"-analyzeduration", "0",
		"-probesize", "32768",
		"-rtbufsize", "256k",
		"-max_delay", "0",
		"-reorder_queue_size", "0",
		"-use_wallclock_as_timestamps", "1",
		"-i", rtspURL,
		"-an",
	)

	vfFilters := make([]string, 0, 2)
	if mode.vfFilter != "" {
		vfFilters = append(vfFilters, mode.vfFilter)
	}
	if fps > 0 {
		vfFilters = append(vfFilters, fmt.Sprintf("fps=%d", fps))
	}

	if bitrateKbps > 0 {
		var qv int
		var scaleFilter string
		switch {
		case bitrateKbps >= 3000:
			qv = 2
		case bitrateKbps >= 1500:
			qv = 4
		case bitrateKbps >= 800:
			qv = 5
			scaleFilter = "scale='min(iw,1280)':'min(ih,720)':force_original_aspect_ratio=decrease"
		case bitrateKbps >= 400:
			qv = 9
			scaleFilter = "scale='min(iw,854)':'min(ih,480)':force_original_aspect_ratio=decrease"
		default:
			qv = 15
			scaleFilter = "scale='min(iw,640)':'min(ih,360)':force_original_aspect_ratio=decrease"
		}
		if scaleFilter != "" {
			vfFilters = append(vfFilters, scaleFilter)
		}
		args = append(args, "-q:v", fmt.Sprintf("%d", qv))
	} else {
		args = append(args, "-q:v", fmt.Sprintf("%d", quality))
	}

	if len(vfFilters) > 0 {
		args = append(args, "-vf", strings.Join(vfFilters, ","))
	}

	args = append(args,
		"-vsync", "0",
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "mpjpeg",
		"-boundary_tag", boundary,
		"-",
	)
	return args
}

func BuildFFmpegHLSArgsForModeTransportOptions(rtspURL, playlistPath, segmentPattern string, mode ffmpegDecodeMode, transport string, options rtspRuntimeOptions) []string {
	args := options.inputArgs(transport)
	args = append(args, mode.inputArgs...)
	args = append(args,
		"-fflags", "nobuffer",
		"-flags", "low_delay",
		"-avioflags", "direct",
		"-analyzeduration", "0",
		"-probesize", "32768",
		"-rtbufsize", "512k",
		"-max_delay", "0",
		"-reorder_queue_size", "0",
		"-use_wallclock_as_timestamps", "1",
		"-i", rtspURL,
		"-an",
	)
	if mode.vfFilter != "" {
		args = append(args, "-vf", mode.vfFilter)
	}
	args = append(args,
		"-c:v", "copy",
		"-flush_packets", "1",
		"-muxdelay", "0",
		"-muxpreload", "0",
		"-f", "hls",
		"-hls_time", "1",
		"-hls_list_size", "3",
		"-hls_flags", "delete_segments+omit_endlist+program_date_time",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)
	return args
}

func BuildFFmpegSnapshotArgs(rtspURL string) []string {
	modes := hwDecodeModes()
	return BuildFFmpegSnapshotArgsForMode(rtspURL, modes[0])
}

func BuildFFmpegSnapshotArgsForMode(rtspURL string, mode ffmpegDecodeMode) []string {
	return BuildFFmpegSnapshotArgsForModeTransport(rtspURL, mode, "tcp")
}

func BuildFFmpegSnapshotArgsForModeTransport(rtspURL string, mode ffmpegDecodeMode, transport string) []string {
	return BuildFFmpegSnapshotArgsForModeTransportOptions(rtspURL, mode, transport, rtspRuntimeOptions{})
}

func BuildFFmpegSnapshotArgsForModeTransportOptions(rtspURL string, mode ffmpegDecodeMode, transport string, options rtspRuntimeOptions) []string {
	args := options.inputArgs(transport)
	args = append(args, mode.inputArgs...)
	args = append(args, "-i", rtspURL, "-frames:v", "1", "-f", "image2pipe")
	if mode.vfFilter != "" {
		args = append(args, "-vf", mode.vfFilter)
	}
	args = append(args, "-vcodec", "mjpeg", "-")
	return args
}

func RedactSensitiveText(text string) string {
	if text == "" {
		return text
	}
	text = urlCredentialsPattern.ReplaceAllString(text, "${1}<credentials>@")
	text = urlSecretQueryPattern.ReplaceAllString(text, "${1}<redacted>")
	return text
}

func IndexBytes(data, sep []byte) int {
	for i := 0; i <= len(data)-len(sep); i++ {
		if string(data[i:i+len(sep)]) == string(sep) {
			return i
		}
	}
	return -1
}

func ExtractJPEG(chunk []byte) []byte {
	for i := 0; i < len(chunk)-1; i++ {
		if chunk[i] == 0xFF && chunk[i+1] == 0xD8 {
			return chunk[i:]
		}
	}
	return nil
}

func StripMultipartHeader(data, boundary []byte) []byte {
	if idx := IndexBytes(data, boundary); idx >= 0 {
		return data[idx+len(boundary):]
	}
	return data
}

func RTSPCleanURL(rawURL string) string {
	parsed, err := url.Parse(strings.ReplaceAll(rawURL, "rtsp://", "http://"))
	if err != nil {
		return rawURL
	}
	parsed.User = nil
	q := parsed.Query()
	q.Del("username")
	q.Del("password")
	parsed.RawQuery = q.Encode()
	return strings.ReplaceAll(parsed.String(), "http://", "rtsp://")
}

func BuildAudioArgs(ffmpegBin, rtspURL string) []string {
	return BuildAudioArgsForTransport(ffmpegBin, rtspURL, "tcp", rtspRuntimeOptions{})
}

func BuildAudioArgsForTransport(ffmpegBin, rtspURL, transport string, options rtspRuntimeOptions) []string {
	ffprobeBin := strings.TrimSuffix(ffmpegBin, "ffmpeg") + "ffprobe"
	if strings.HasSuffix(ffmpegBin, ".exe") {
		ffprobeBin = strings.TrimSuffix(ffmpegBin, "ffmpeg.exe") + "ffprobe.exe"
	}

	probeArgs := []string{
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
	}
	probeArgs = append(probeArgs, options.inputArgs(transport)...)
	probeArgs = append(probeArgs,
		"-timeout", "5000000",
		rtspURL,
	)
	probeCmd := exec.Command(ffprobeBin, probeArgs...)
	out, err := probeCmd.Output()
	codec := strings.TrimSpace(strings.ToLower(string(out)))
	if err != nil || codec == "" {
		log.Printf("[NVR] audio probe failed/no audio (codec=%q err=%v), -an", codec, err)
		return []string{"-an"}
	}

	mp4Safe := map[string]bool{
		"aac": true, "mp3": true, "ac3": true, "eac3": true,
		"opus": true, "alac": true, "mp2": true,
	}
	if mp4Safe[codec] {
		log.Printf("[NVR] audio codec %q is MP4-safe, copy", codec)
		return []string{"-map", "0:a:0", "-c:a", "copy"}
	}

	log.Printf("[NVR] audio codec %q not MP4-safe, transcode to AAC 64k", codec)
	return []string{"-map", "0:a:0", "-c:a", "aac", "-b:a", "64k", "-ar", "8000", "-ac", "1"}
}

func BuildAuthURL(rtspURL, username, password string) string {
	if username == "" {
		return rtspURL
	}
	if strings.Contains(rtspURL, "@") {
		schemeEnd := strings.Index(rtspURL, "://")
		if schemeEnd != -1 {
			afterScheme := rtspURL[schemeEnd+3:]
			if atIdx := strings.Index(afterScheme, "@"); atIdx != -1 {
				scheme := rtspURL[:schemeEnd+3]
				rtspURL = scheme + afterScheme[atIdx+1:]
			}
		}
	}
	encUser := strings.NewReplacer("@", "%40", ":", "%3A", "/", "%2F").Replace(username)
	encPass := strings.NewReplacer("@", "%40", ":", "%3A", "/", "%2F").Replace(password)
	if strings.HasPrefix(rtspURL, "rtsp://") {
		return "rtsp://" + encUser + ":" + encPass + "@" + rtspURL[7:]
	}
	return rtspURL
}

func CheckRTSP(ip string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 1500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func ProbeONVIFPort(ip string, preferred int) (bool, int) {
	ports := []int{preferred}
	for _, p := range []int{80, 8080, 8000} {
		if p != preferred {
			ports = append(ports, p)
		}
	}

	const soapBody = `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <s:Body>
    <tds:GetDeviceInformation/>
  </s:Body>
</s:Envelope>`

	for _, p := range ports {
		target := fmt.Sprintf("http://%s:%d/onvif/device_service", ip, p)
		client := &http.Client{Timeout: 2 * time.Second}
		req, err := http.NewRequest("POST", target, strings.NewReader(soapBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8`)
		req.Header.Set("SOAPAction", `"http://www.onvif.org/ver10/device/wsdl/GetDeviceInformation"`)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		bodyStr := string(body)
		if (strings.Contains(bodyStr, "Envelope") || strings.Contains(bodyStr, "envelope")) &&
			(strings.Contains(bodyStr, "Manufacturer") ||
				strings.Contains(bodyStr, "Model") ||
				strings.Contains(bodyStr, "FirmwareVersion") ||
				strings.Contains(bodyStr, "GetDeviceInformationResponse")) {
			return true, p
		}
	}
	return false, 0
}

func ExpandRange(startIP, endIP string) ([]string, error) {
	s := net.ParseIP(startIP).To4()
	e := net.ParseIP(endIP).To4()
	if s == nil || e == nil {
		return nil, fmt.Errorf("invalid IP address")
	}
	su := binary.BigEndian.Uint32(s)
	eu := binary.BigEndian.Uint32(e)
	if su > eu {
		return nil, fmt.Errorf("start IP must be <= end IP")
	}
	var ips []string
	for i := su; i <= eu; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		ips = append(ips, ip.String())
	}
	return ips, nil
}

func ExpandSubnet(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		if !isNetOrBcast(ip, ipnet) {
			ips = append(ips, ip.String())
		}
	}
	return ips, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func isNetOrBcast(ip net.IP, ipnet *net.IPNet) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	ones, bits := ipnet.Mask.Size()
	if ones == bits {
		return false
	}
	mask := ipnet.Mask
	isNet, isBcast := true, true
	for i := 0; i < 4; i++ {
		h := ip4[i] &^ mask[i]
		if h != 0 {
			isNet = false
		}
		if h != ^mask[i] {
			isBcast = false
		}
	}
	return isNet || isBcast
}

func GetONVIFStreamURI(deviceURL, user, pass string) (string, error) {
	profilesBody := `<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:trt="http://www.onvif.org/ver10/media/wsdl">
  <s:Body><trt:GetProfiles/></s:Body>
</s:Envelope>`

	resp1, err := onvifSOAP(deviceURL, profilesBody, user, pass, `"http://www.onvif.org/ver10/media/wsdl/GetProfiles"`)
	if err != nil {
		return "", fmt.Errorf("GetProfiles failed: %v", err)
	}

	token := onvifExtractAttr(resp1, "token")
	if token == "" {
		return "", fmt.Errorf("no profile token found in GetProfiles response")
	}

	mediaURL := strings.Replace(deviceURL, "/device_service", "/media_service", 1)
	streamBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope"
            xmlns:trt="http://www.onvif.org/ver10/media/wsdl"
            xmlns:tt="http://www.onvif.org/ver10/schema">
  <s:Body>
    <trt:GetStreamUri>
      <trt:StreamSetup>
        <tt:Stream>RTP-Unicast</tt:Stream>
        <tt:Transport><tt:Protocol>RTSP</tt:Protocol></tt:Transport>
      </trt:StreamSetup>
      <trt:ProfileToken>%s</trt:ProfileToken>
    </trt:GetStreamUri>
  </s:Body>
</s:Envelope>`, token)

	resp2, err := onvifSOAP(mediaURL, streamBody, user, pass, `"http://www.onvif.org/ver10/media/wsdl/GetStreamUri"`)
	if err != nil {
		resp2, err = onvifSOAP(deviceURL, streamBody, user, pass, `"http://www.onvif.org/ver10/media/wsdl/GetStreamUri"`)
		if err != nil {
			return "", fmt.Errorf("GetStreamUri failed: %v", err)
		}
	}

	rtsp := onvifExtractTag(resp2, "Uri")
	if rtsp == "" || !strings.HasPrefix(rtsp, "rtsp://") {
		return "", fmt.Errorf("no RTSP URI in GetStreamUri response")
	}
	return rtsp, nil
}

func onvifSOAP(url_, body, user, pass, action string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("POST", url_, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8`)
	if action != "" {
		req.Header.Set("SOAPAction", action)
	}
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	return string(b), nil
}

func onvifExtractAttr(s, attr string) string {
	needle := attr + `="`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(s[start:], `"`)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

func onvifExtractTag(s, tagName string) string {
	open := ":" + tagName + ">"
	idx := strings.Index(s, open)
	if idx < 0 {
		open = "<" + tagName + ">"
		idx = strings.Index(s, open)
		if idx < 0 {
			return ""
		}
	}
	start := idx + len(open)
	end := strings.Index(s[start:], "</")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(s[start : start+end])
}
