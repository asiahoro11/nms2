// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	cameramodule "management-server/modules/camera"
)

func (h *Handler) legacyExportBackup(c *gin.Context) {
	// TRUNCATE checkpoint flushes WAL into nms.db so the backup contains a
	// self-consistent database even without WAL sidecars.
	var busy, walPages, checkpointedPages int
	row := h.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)")
	err := row.Scan(&busy, &walPages, &checkpointedPages)
	if err != nil {
		log.Printf("WAL checkpoint failed: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "無法完成資料庫同步，請稍後再試或聯繫系統管理員",
		})
		return
	}
	if walPages > 0 {
		log.Printf("WAL checkpoint: %d/%d pages flushed (busy=%d)", checkpointedPages, walPages, busy)
	}

	dbPath := h.config.Database.Path
	walPath := dbPath + "-wal"
	if info, err := os.Stat(walPath); err == nil && info.Size() > 0 {
		log.Printf("WARNING: WAL file still has %d bytes after TRUNCATE checkpoint", info.Size())
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "資料庫同步異常，請重試或聯繫技術支援",
		})
		return
	}

	configPath := "data/config.yaml"
	uploadsPath := "data/uploads"

	tempFile, err := os.CreateTemp("", "nms_backup_*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to create temp backup file"})
		return
	}
	defer os.Remove(tempFile.Name()) // Clean up after send
	defer tempFile.Close()

	zipWriter := zip.NewWriter(tempFile)

	addFile := func(src, dst string) error {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			return nil
		}

		file, err := os.Open(src)
		if err != nil {
			return err
		}
		defer file.Close()

		w, err := zipWriter.Create(dst)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, file)
		return err
	}

	if err := addFile(dbPath, "nms.db"); err != nil {
		log.Printf("Failed to backup DB: %v", err)
	}
	addFile(dbPath+"-wal", "nms.db-wal")
	addFile(dbPath+"-shm", "nms.db-shm")

	if err := addFile(configPath, "config.yaml"); err != nil {
		addFile("config.yaml", "config.yaml")
	}

	filepath.Walk(uploadsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel("data", path)
		relPath = filepath.ToSlash(relPath)
		return addFile(path, relPath)
	})

	if err := zipWriter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to finalize backup zip"})
		return
	}

	readFile, err := os.Open(tempFile.Name())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to read backup file"})
		return
	}
	defer readFile.Close()

	timestamp := time.Now().Format("20060102_150405")
	prefix := "nms"
	if runtime.GOOS == "windows" {
		prefix = "win_nms"
	} else if runtime.GOOS == "linux" {
		prefix = "linux_nms"
	}

	filename := fmt.Sprintf("%s_backup_%s.zip", prefix, timestamp)

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Transfer-Encoding", "binary")

	io.Copy(c.Writer, readFile)
}

func (h *Handler) legacyRestoreBackup(c *gin.Context) {
	file, err := c.FormFile("backup_file")
	if err != nil {
		h.WriteSystemLog("warning", "backup", "restore_backup_missing_file", "backup restore missing file", nil)
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module": "backup",
			"reason": "missing_file",
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "No file uploaded"})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_backup_started", "backup restore started", map[string]interface{}{
		"file": file.Filename,
		"size": file.Size,
	})

	// Use data/ dir to avoid cross-device rename errors when /tmp is on a different partition.
	tempPath := filepath.Join("data", "restore_upload.zip")
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		h.WriteSystemLog("error", "backup", "restore_backup_save_failed", "backup restore upload save failed", map[string]interface{}{
			"file":  file.Filename,
			"size":  file.Size,
			"error": err.Error(),
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":        "backup",
			"reason":        "save_upload_failed",
			"resource_type": "system_backup",
			"resource_name": file.Filename,
			"error":         err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to save upload" + err.Error()})
		return
	}
	defer os.Remove(tempPath)

	r, err := zip.OpenReader(tempPath)
	if err != nil {
		h.WriteSystemLog("warning", "backup", "restore_backup_invalid_zip", "backup restore rejected: invalid zip", map[string]interface{}{
			"file":  file.Filename,
			"size":  file.Size,
			"error": err.Error(),
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":        "backup",
			"reason":        "invalid_zip",
			"resource_type": "system_backup",
			"resource_name": file.Filename,
			"error":         err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Invalid zip file"})
		return
	}
	defer r.Close()

	os.RemoveAll("data/uploads_pending")
	os.MkdirAll("data/uploads_pending", 0755)

	pendingFiles, _ := filepath.Glob("data/*.pending")
	for _, f := range pendingFiles {
		os.Remove(f)
	}

	var hasDB bool
	extractedUploads := 0
	extractedConfig := false

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") { // zip-slip guard
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		if f.Name == "nms.db" {
			os.MkdirAll("data", 0755)
			outFile, err := os.Create("data/nms.db.pending")
			if err == nil {
				io.Copy(outFile, rc)
				outFile.Close()
				hasDB = true
			}
		} else if f.Name == "nms.db-wal" {
			outFile, err := os.Create("data/nms.db-wal.pending")
			if err == nil {
				io.Copy(outFile, rc)
				outFile.Close()
			}
		} else if f.Name == "nms.db-shm" {
			outFile, err := os.Create("data/nms.db-shm.pending")
			if err == nil {
				io.Copy(outFile, rc)
				outFile.Close()
			}
		} else if f.Name == "config.yaml" {
			os.MkdirAll("config", 0755)
			outFile, err := os.Create("config/config.yaml.pending")
			if err == nil {
				io.Copy(outFile, rc)
				outFile.Close()
				extractedConfig = true
			}
		} else if strings.HasPrefix(f.Name, "uploads/") || strings.HasPrefix(f.Name, "data/uploads/") {
			relPath := f.Name
			if strings.HasPrefix(relPath, "data/") {
				relPath = relPath[5:]
			}
			innerName := relPath[8:] // strip "uploads/" prefix
			if innerName == "" {
				rc.Close()
				continue
			}

			targetPath := filepath.Join("data/uploads_pending", innerName)
			if f.FileInfo().IsDir() {
				os.MkdirAll(targetPath, f.Mode())
			} else {
				os.MkdirAll(filepath.Dir(targetPath), 0755)
				outFile, err := os.Create(targetPath)
				if err == nil {
					io.Copy(outFile, rc)
					outFile.Close()
					extractedUploads++
				}
			}
		}

		rc.Close()
	}

	if !hasDB {
		h.WriteSystemLog("warning", "backup", "restore_backup_missing_db", "backup restore rejected: database file missing", map[string]interface{}{
			"file":             file.Filename,
			"extracted_config": extractedConfig,
			"uploads_count":    extractedUploads,
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":           "backup",
			"reason":           "missing_database",
			"resource_type":    "system_backup",
			"resource_name":    file.Filename,
			"extracted_config": extractedConfig,
			"uploads_count":    extractedUploads,
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Invalid backup: nms.db not found"})
		return
	}

	// Create restart script
	if err := createRestoreScript(h.config.Database.Path); err != nil {
		h.WriteSystemLog("error", "backup", "restore_backup_prepare_failed", "backup restore restart script creation failed", map[string]interface{}{
			"file":             file.Filename,
			"error":            err.Error(),
			"extracted_config": extractedConfig,
			"uploads_count":    extractedUploads,
		})
		h.WriteAuditFromContext(c, "restore_backup", "system_backup", "failed", map[string]interface{}{
			"module":           "backup",
			"reason":           "create_restore_script_failed",
			"resource_type":    "system_backup",
			"resource_name":    file.Filename,
			"error":            err.Error(),
			"extracted_config": extractedConfig,
			"uploads_count":    extractedUploads,
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: "Failed to create restore script: " + err.Error()})
		return
	}

	h.WriteSystemLog("notice", "backup", "restore_backup_ready", "backup restore prepared; restart scheduled", map[string]interface{}{
		"file":             file.Filename,
		"extracted_config": extractedConfig,
		"uploads_count":    extractedUploads,
		"db_pending":       hasDB,
	})
	h.WriteAuditFromContext(c, "restore_backup", "system_backup", "success", map[string]interface{}{
		"module":           "backup",
		"resource_type":    "system_backup",
		"resource_name":    file.Filename,
		"db_pending":       hasDB,
		"extracted_config": extractedConfig,
		"uploads_count":    extractedUploads,
		"change_source":    "manual",
		"change_scope":     "restore_workflow",
	})

	// Respond OK before restarting
	c.JSON(http.StatusOK, Response{Success: true, Message: "Restore initiated. Server will restart."})

	// Trigger restart in goroutine to allow response to complete
	go func() {
		time.Sleep(1 * time.Second)
		executeRestoreScript()
		os.Exit(0)
	}()
}

func (h *Handler) legacyCheckRestoreReadiness(c *gin.Context) {
	issues := []string{}

	// Check sqlite3 availability (Linux only)
	if runtime.GOOS == "linux" {
		cmd := exec.Command("which", "sqlite3")
		if err := cmd.Run(); err != nil {
			issues = append(issues, "sqlite3 工具未安裝（Linux 還原建議安裝以確保最佳完整性）")
		}
	}

	// Check WAL mode
	var walMode string
	h.db.QueryRow("PRAGMA journal_mode").Scan(&walMode)

	// Check if checkpoint works (non-destructive passive checkpoint)
	if _, err := h.db.Exec("PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		issues = append(issues, fmt.Sprintf("WAL checkpoint 測試失敗: %v", err))
	}

	c.JSON(http.StatusOK, Response{
		Success: len(issues) == 0,
		Data: map[string]interface{}{
			"ready":    len(issues) == 0,
			"issues":   issues,
			"wal_mode": walMode,
			"os":       runtime.GOOS,
		},
	})
}

// gpuInfo holds detected GPU name and active hwaccel method.
type gpuInfo struct {
	Available bool   `json:"available"`
	Name      string `json:"name"`     // e.g. "NVIDIA RTX 5050", "Intel Arc A770"
	Method    string `json:"method"`   // "cuda", "qsv", "vaapi", "d3d11va", "none"
	MaxCams   int    `json:"max_cams"` // recommended max concurrent camera streams
	IsVM      bool   `json:"is_vm"`    // true if running inside a VM
	VMType    string `json:"vm_type"`  // "kvm", "vmware", "lxc", "hyperv", "none", etc.
}

var (
	cachedGPUOnce sync.Once
	cachedGPU     gpuInfo
)

func runWithTimeout(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func lspciGPUName() string {
	out := runWithTimeout("lspci")
	if out == "" {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d controller") || strings.Contains(lower, "display controller") {
			// "00:02.0 VGA compatible controller: Intel Corporation ..."
			// Strip leading PCI address
			if idx := strings.Index(line, ": "); idx >= 0 {
				return strings.TrimSpace(line[idx+2:])
			}
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func wmicGPUName() string {
	parse := func(out string) []string {
		var names []string
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(strings.TrimRight(l, "\r"))
			if l != "" && !strings.EqualFold(l, "Name") {
				names = append(names, l)
			}
		}
		return names
	}
	if out := runWithTimeout("wmic", "path", "win32_VideoController", "get", "name"); out != "" {
		if names := parse(out); len(names) > 0 {
			return strings.Join(names, " / ")
		}
	}
	// PowerShell fallback for Windows 11 where wmic is deprecated
	out := runWithTimeout("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_VideoController).Name -join ' / '")
	return strings.TrimSpace(out)
}

func detectVMType() (bool, string) {
	if runtime.GOOS == "linux" {
		out := runWithTimeout("systemd-detect-virt")
		if out != "" && out != "none" {
			return true, out
		}
		dmi := runWithTimeout("sh", "-c", "cat /sys/class/dmi/id/product_name 2>/dev/null")
		lower := strings.ToLower(dmi)
		switch {
		case strings.Contains(lower, "vmware"):
			return true, "vmware"
		case strings.Contains(lower, "virtualbox"):
			return true, "virtualbox"
		case strings.Contains(lower, "kvm") || strings.Contains(lower, "qemu"):
			return true, "kvm"
		case strings.Contains(lower, "proxmox"):
			return true, "proxmox"
		}
		return false, "none"
	}
	if runtime.GOOS == "windows" {
		out := runWithTimeout("wmic", "computersystem", "get", "model")
		lower := strings.ToLower(out)
		switch {
		case strings.Contains(lower, "vmware"):
			return true, "vmware"
		case strings.Contains(lower, "virtual machine"):
			return true, "hyperv"
		case strings.Contains(lower, "virtualbox"):
			return true, "virtualbox"
		}
	}
	return false, "none"
}

func detectGPUInfo(ffmpegBin string) gpuInfo {
	cachedGPUOnce.Do(func() {
		cachedGPU = probeGPUInfo(ffmpegBin)
		log.Printf("[GPU] method=%s name=%q maxCams=%d isVM=%v vmType=%s",
			cachedGPU.Method, cachedGPU.Name, cachedGPU.MaxCams, cachedGPU.IsVM, cachedGPU.VMType)
	})
	return cachedGPU
}

func findLocalFFmpegBin() string {
	exe := "ffmpeg"
	if runtime.GOOS == "windows" {
		exe = "ffmpeg.exe"
	}

	if self, err := os.Executable(); err == nil {
		for _, candidate := range []string{
			filepath.Join(filepath.Dir(self), "bin", exe),
			filepath.Join(filepath.Dir(self), exe),
		} {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	if path, err := exec.LookPath(exe); err == nil {
		return path
	}
	return ""
}

func probeGPUInfo(ffmpegBin string) gpuInfo {
	isVM, vmType := detectVMType()
	info := gpuInfo{IsVM: isVM, VMType: vmType}

	if ffmpegBin == "" {
		info.Method = "none"
		info.Name = "ffmpeg 未找到"
		info.MaxCams = 16
		return info
	}

	hw := cameramodule.GetHardwareAccelState(ffmpegBin)

	if runtime.GOOS == "linux" {
		gpuName := lspciGPUName()
		lower := strings.ToLower(gpuName)

		switch {
		case strings.Contains(lower, "nvidia"):
			info.Method = "cuda"
			info.Available = hw.CUDA
			info.MaxCams = 32
			if n := runWithTimeout("nvidia-smi", "--query-gpu=name", "--format=csv,noheader"); n != "" {
				info.Name = n
			} else if gpuName != "" {
				info.Name = gpuName
			} else {
				info.Name = "NVIDIA GPU"
			}
		case strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") || strings.Contains(lower, "advanced micro"):
			info.Method = "vaapi"
			info.Available = hw.VAAPI
			info.MaxCams = 24
			info.Name = gpuName
			if info.Name == "" {
				info.Name = "AMD GPU (VAAPI)"
			}
		case strings.Contains(lower, "intel"):
			info.Method = "vaapi"
			info.Available = hw.VAAPI
			info.MaxCams = 24
			info.Name = gpuName
			if info.Name == "" {
				info.Name = "Intel GPU (VAAPI)"
			}
		case hw.CUDA:
			info.Method = "cuda"
			info.Available = true
			info.MaxCams = 32
			info.Name = gpuName
			if info.Name == "" {
				info.Name = "GPU (CUDA)"
			}
		case hw.VAAPI:
			info.Method = "vaapi"
			info.Available = true
			info.MaxCams = 24
			info.Name = gpuName
			if info.Name == "" {
				info.Name = "GPU (VAAPI)"
			}
		default:
			info.Method = "none"
			info.Available = false
			info.MaxCams = 16
			if gpuName != "" {
				info.Name = gpuName + " (無支援的 hwaccel)"
			} else {
				info.Name = "無 GPU 加速 (CPU 解碼)"
			}
		}
	} else {
		// Prefer name-based detection; ffmpeg hwaccel probes give false positives on mixed-vendor hardware.
		gpuName := wmicGPUName()
		lower := strings.ToLower(gpuName)

		switch {
		case strings.Contains(lower, "nvidia"):
			info.Method = "cuda"
			info.Available = true
			info.MaxCams = 32
			if n := runWithTimeout("nvidia-smi", "--query-gpu=name", "--format=csv,noheader"); n != "" {
				info.Name = strings.TrimSpace(n)
			} else {
				info.Name = gpuName
			}
		case strings.Contains(lower, "intel") || strings.Contains(lower, "arc"):
			info.Method = "qsv"
			info.Available = true
			info.MaxCams = 24
			info.Name = gpuName
		case strings.Contains(lower, "amd") || strings.Contains(lower, "radeon") || strings.Contains(lower, "advanced micro"):
			info.Method = "d3d11va"
			info.Available = true
			info.MaxCams = 16
			info.Name = gpuName
		case hw.CUDA:
			info.Method = "cuda"
			info.Available = true
			info.MaxCams = 32
			info.Name = gpuName
		case hw.QSV:
			info.Method = "qsv"
			info.Available = true
			info.MaxCams = 24
			info.Name = gpuName
		case hw.D3D11:
			info.Method = "d3d11va"
			info.Available = true
			info.MaxCams = 16
			info.Name = gpuName
		default:
			info.Method = "none"
			info.Available = false
			info.MaxCams = 16
			info.Name = gpuName
			if info.Name == "" {
				info.Name = "無 GPU 加速 (CPU 解碼)"
			}
		}
	}
	return info
}

type hostUsageMetrics struct {
	CPUUsage         float64
	MemTotal         uint64
	MemUsed          uint64
	MemUsagePercent  float64
	DiskTotal        uint64
	DiskUsed         uint64
	DiskUsagePercent float64
}

func collectHostUsageMetrics(timeout time.Duration) hostUsageMetrics {
	type result struct {
		metrics hostUsageMetrics
	}

	ch := make(chan result, 1)
	go func() {
		var metrics hostUsageMetrics
		if values, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(values) > 0 {
			metrics.CPUUsage = values[0]
		}

		if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
			metrics.MemTotal = vm.Total
			metrics.MemUsed = vm.Used
			metrics.MemUsagePercent = vm.UsedPercent
		}

		diskPath := "."
		if wd, err := os.Getwd(); err == nil && wd != "" {
			diskPath = wd
		}
		if usage, err := disk.Usage(diskPath); err == nil && usage != nil {
			metrics.DiskTotal = usage.Total
			metrics.DiskUsed = usage.Used
			metrics.DiskUsagePercent = usage.UsedPercent
		}

		ch <- result{metrics: metrics}
	}()

	select {
	case res := <-ch:
		return res.metrics
	case <-time.After(timeout):
		log.Printf("[HostStatus] usage metrics collection timed out after %s", timeout)
		return hostUsageMetrics{}
	}
}

func (h *Handler) GetHostStatus(c *gin.Context) {
	hostname, _ := os.Hostname()
	metrics := collectHostUsageMetrics(2 * time.Second)

	gpu := gpuInfo{
		Available: false,
		Name:      "Host GPU detection skipped for admin page stability",
		Method:    "none",
		MaxCams:   16,
		IsVM:      false,
		VMType:    "unknown",
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"os":                 runtime.GOOS,
			"platform":           runtime.GOOS,
			"platform_version":   runtime.GOARCH,
			"hostname":           hostname,
			"uptime":             uint64(time.Since(h.startTime).Seconds()),
			"cpu_usage":          metrics.CPUUsage,
			"mem_total":          metrics.MemTotal,
			"mem_used":           metrics.MemUsed,
			"mem_usage_percent":  metrics.MemUsagePercent,
			"disk_total":         metrics.DiskTotal,
			"disk_used":          metrics.DiskUsed,
			"disk_usage_percent": metrics.DiskUsagePercent,
			"gpu":                gpu,
		},
	})
}

func (h *Handler) legacyGetSystemConfig(c *gin.Context) {
	rows, err := h.db.Query("SELECT config_key, config_value, COALESCE(description, '') FROM system_config")
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	configs := []map[string]string{}
	for rows.Next() {
		var key, val, desc string
		if err := rows.Scan(&key, &val, &desc); err == nil {
			configs = append(configs, map[string]string{
				"config_key":   key,
				"config_value": val,
				"description":  desc,
			})
		}
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    configs,
	})
}

// repairCameraConfigFromLicense 根據當前授權狀態修復相機模組配置
// Update 2025-02-15: User requested to keep this manual. Auto-enabling removed.
/*
func (h *Handler) repairCameraConfigFromLicense() {
	var cameraEnabled string
	err := h.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&cameraEnabled)

	// 如果配置已存在（不論是 true 或 false），則不自動覆寫 (尊重管理員手動選擇)
	if err == nil {
		return
	}

	// 檢查是否有包含相機功能的有效授權
	var count int
	err = h.db.QueryRow(`
		SELECT COUNT(*) FROM licenses
		WHERE is_active = 1
		AND (valid_until IS NULL OR valid_until = '' OR valid_until >= date('now'))
		AND (enabled_features LIKE '%camera_viewer%' OR device_count > 0)
	`).Scan(&count)

	if err == nil && count > 0 {
		log.Println("[ConfigRepair] Valid camera license detected, auto-enabling camera_viewer_enabled")
		h.db.Exec(`INSERT OR REPLACE INTO system_config (config_key, config_value, description)
			VALUES ('camera_viewer_enabled', '1', 'Auto-enabled from license')`)
	}
}
*/
