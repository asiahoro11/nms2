package timesync

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"time"

	"github.com/beevik/ntp"
)

var stopCh chan struct{}

// Start initializes the time sync service
func Start() {
	stopCh = make(chan struct{})
	go func() {
		// Initial sync
		syncRoutine()

		// Periodic sync every hour
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				log.Println("[TimeSync] Service stopped")
				return
			case <-ticker.C:
				syncRoutine()
			}
		}
	}()
}

// Stop gracefully stops the time sync service
func Stop() {
	if stopCh != nil {
		close(stopCh)
	}
}

func syncRoutine() {
	log.Println("[TimeSync] Starting time synchronization check...")

	online := checkConnectivity()
	var err error

	if online {
		log.Println("[TimeSync] Internet is reachable. Syncing with tock.stdtime.gov.tw...")
		err = syncTime("tock.stdtime.gov.tw")
	} else {
		log.Println("[TimeSync] No internet connection. Skipping time sync.")
	}

	if err != nil {
		log.Printf("[TimeSync] Failed to sync time: %v", err)
	} else if online {
		log.Println("[TimeSync] Time synchronized successfully.")
	}
}

// checkConnectivity uses TCP connect to NTP port instead of ICMP (which may be blocked)
func checkConnectivity() bool {
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// syncTime gets time from NTP and updates system time
func syncTime(host string) error {
	options := ntp.QueryOptions{Timeout: 5 * time.Second}
	response, err := ntp.QueryWithOptions(host, options)
	if err != nil {
		return fmt.Errorf("NTP request failed: %v", err)
	}

	offset := response.ClockOffset
	log.Printf("[TimeSync] NTP Offset: %v", offset)

	targetTime := time.Now().Add(offset)
	return setSystemTime(targetTime)
}

func setSystemTime(t time.Time) error {
	// This usually requires Admin/Root privileges

	if runtime.GOOS == "linux" {
		timeStr := t.Format("2006-01-02 15:04:05")
		cmd := exec.Command("date", "-s", timeStr)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set time (linux): %v, output: %s", err, string(out))
		}
		// Also sync hardware clock if possible
		exec.Command("hwclock", "-w").Run()
	} else if runtime.GOOS == "windows" {
		timeStr := t.Format("2006-01-02 15:04:05")
		cmd := exec.Command("powershell", "-Command", fmt.Sprintf("Set-Date -Date '%s'", timeStr))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to set time (windows): %v, output: %s", err, string(out))
		}
	} else {
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return nil
}
