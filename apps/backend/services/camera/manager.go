package camera

import (
	"database/sql"
	"log"
	"management-server/config"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Manager struct {
	config *config.Config
	db     *sql.DB
	cmd    *exec.Cmd
	mu     sync.Mutex
	stopCh chan struct{}
}

func New(cfg *config.Config, db *sql.DB) *Manager {
	return &Manager{
		config: cfg,
		db:     db,
		stopCh: make(chan struct{}),
	}
}

func (m *Manager) Start() {
	log.Println("[Camera] Starting go2rtc manager loop...")
	go m.mainLoop()
}

func (m *Manager) Stop() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		log.Println("[Camera] Stopping go2rtc process...")
		m.cmd.Process.Kill()
	}
}

func (m *Manager) isEnabled() bool {
	var val string
	err := m.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'camera_viewer_enabled'").Scan(&val)
	return err == nil && (val == "1" || val == "true")
}

func (m *Manager) mainLoop() {
	// Initial check
	if m.isEnabled() {
		m.ensureRunning()
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			if m.isEnabled() {
				m.ensureRunning()
			} else {
				m.ensureStopped()
			}
		}
	}
}

func (m *Manager) ensureRunning() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		if m.isProcessAlive() {
			return
		}
		log.Println("[Camera] go2rtc process died, restarting...")
		m.cmd = nil
	}

	log.Println("[Camera] Starting go2rtc service...")

	// Find binary
	exeName := "go2rtc"
	if runtime.GOOS == "windows" {
		exeName = "go2rtc.exe"
	}

	// Try multiple paths: ./bin, ./backend/bin, or system PATH
	binPath := ""
	cwd, _ := os.Getwd()
	searchPaths := []string{
		filepath.Join(cwd, "bin", exeName),
		filepath.Join(cwd, "backend", "bin", exeName),
		filepath.Join(cwd, "..", "bin", exeName),
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			binPath = p
			break
		}
	}

	if binPath == "" {
		// Fallback to system PATH
		var err error
		binPath, err = exec.LookPath(exeName)
		if err != nil {
			log.Printf("[Camera] ERROR: go2rtc binary not found in ./bin or PATH")
			return
		}
	}

	// Ensure bin folder is in PATH so go2rtc can find ffmpeg if it's there
	absBinDir, _ := filepath.Abs(filepath.Dir(binPath))
	pathEnv := os.Getenv("PATH")
	if !contains(pathEnv, absBinDir) {
		os.Setenv("PATH", absBinDir+string(os.PathListSeparator)+pathEnv)
	}

	m.cmd = exec.Command(binPath)
	m.cmd.Dir = absBinDir // Important for relative config search

	err := m.cmd.Start()
	if err != nil {
		log.Printf("[Camera] Failed to start go2rtc: %v", err)
		return
	}

	log.Printf("[Camera] go2rtc started with PID %d (bin: %s)", m.cmd.Process.Pid, binPath)
	
	// Start a goroutine to wait for the process to exit
	go func(c *exec.Cmd) {
		err := c.Wait()
		log.Printf("[Camera] go2rtc process exited: %v", err)
	}(m.cmd)
}

func (m *Manager) ensureStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.isProcessAlive() {
		log.Println("[Camera] Module disabled, stopping go2rtc...")
		m.cmd.Process.Kill()
		m.cmd = nil
	}
}

func (m *Manager) isProcessAlive() bool {
	if m.cmd == nil || m.cmd.Process == nil {
		return false
	}
	// On Windows, checking if ProcessState is set is the most reliable way 
	// after a Start() call has returned and we are using Wait() in background.
	if m.cmd.ProcessState != nil {
		return false
	}
	// For Unix/Linux, we might send signal 0
	// But let's stick to ProcessState check since we use go c.Wait()
	return true
}

func contains(s string, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
