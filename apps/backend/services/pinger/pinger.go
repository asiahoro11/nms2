package pinger

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"management-server/config"
	"management-server/pkg/dbutils"
	"management-server/services/alert"
	"management-server/services/dbworker"
	"management-server/services/license"
)

type Pinger struct {
	config        *config.Config
	db            *sql.DB
	dbWorker      *dbworker.Worker
	stopCh        chan struct{}
	failCounts    map[int]int  // Track consecutive failures
	successCounts map[int]int  // Track consecutive successes for recovery
	statusCache   map[int]bool // Memory cache for device status to avoid DB race conditions
	cacheMutex    sync.RWMutex

	// OnStatusChange is called when a device transitions online/offline.
	// Used by the cloud edge connector to publish status changes.
	OnStatusChange func(deviceID int, deviceIP, deviceName string, isOnline bool)
}

type pingResult struct {
	deviceID   int
	deviceIP   string
	deviceName string
	isOnline   bool
	prevStatus bool // status snapshot before ping (fallback)
}

func New(cfg *config.Config, db *sql.DB, worker *dbworker.Worker) *Pinger {
	return &Pinger{
		config:        cfg, // Store config
		db:            db,
		dbWorker:      worker,
		stopCh:        make(chan struct{}),
		failCounts:    make(map[int]int),
		successCounts: make(map[int]int),
		statusCache:   make(map[int]bool),
	}
}

func (p *Pinger) Start() {
	// 3 seconds interval (3s * 3 retries = ~9s detection)
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		log.Println("[Pinger] Service started")
		for {
			select {
			case <-p.stopCh:
				ticker.Stop()
				return
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[Pinger] Panic in cycle: %v", r)
						}
					}()
					p.pingAll()
				}()
			}
		}
	}()
}

func (p *Pinger) Stop() {
	close(p.stopCh)
}

func (p *Pinger) pingAll() {
	if license.ShouldRuntimeLockdown(p.db, p.config.System.Version) {
		return
	}

	// 1. Fetch authorized devices only
	maxDevices := license.GetMaxDevices(p.db, p.config)

	// Fetch display name to avoid extra queries, enforced by license limit
	rows, err := p.db.Query(`
		SELECT id, ip_address, is_online, COALESCE(sys_name, name, ip_address) as display_name
		FROM devices
		ORDER BY id ASC
		LIMIT ?
	`, maxDevices)
	if err != nil {
		log.Printf("[Pinger] Failed to fetch devices: %v", err)
		return
	}

	var devices []struct {
		ID         int
		IP         string
		IsOnline   bool
		DeviceName string
	}

	for rows.Next() {
		var d struct {
			ID         int
			IP         string
			IsOnline   bool
			DeviceName string
		}
		if err := rows.Scan(&d.ID, &d.IP, &d.IsOnline, &d.DeviceName); err == nil {
			devices = append(devices, d)
		}
	}
	rows.Close()

	if len(devices) == 0 {
		return
	}

	// 2. Setup channels and synchronization
	resultsCh := make(chan pingResult, len(devices))
	var workerWg sync.WaitGroup
	var writerWg sync.WaitGroup

	// 3. Start Single Writer Goroutine (Serial DB Access)
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Pinger] Panic in result processor: %v", r)
			}
		}()
		p.processResults(resultsCh)
	}()

	// 4. Start Concurrent Workers
	// Limit concurrency to 50
	sem := make(chan struct{}, 50)

	for _, d := range devices {
		workerWg.Add(1)
		sem <- struct{}{}
		go func(id int, ip string, dbStatus bool, devName string) {
			defer workerWg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Pinger] Panic processing %s: %v", ip, r)
				}
			}()

			online := checkPing(ip)
			resultsCh <- pingResult{
				deviceID:   id,
				deviceIP:   ip,
				deviceName: devName,
				isOnline:   online,
				prevStatus: dbStatus, // Still pass DB status, but processResults will prefer cache if available
			}
		}(d.ID, d.IP, d.IsOnline, d.DeviceName)
	}

	// 5. Wait for workers to finish, then close channel to stop writer
	workerWg.Wait()
	close(resultsCh)
	writerWg.Wait()
}

// processResults handles all DB updates sequentially to avoid locks

// processResults handles all DB updates sequentially to avoid locks
func (p *Pinger) processResults(resultsCh <-chan pingResult) {
	for res := range resultsCh {
		id := res.deviceID
		online := res.isOnline

		// Hysteresis Logic
		currentFail := p.failCounts[id]
		currentSuccess := p.successCounts[id]

		if online {
			p.failCounts[id] = 0
			p.successCounts[id] = currentSuccess + 1
		} else {
			p.successCounts[id] = 0
			p.failCounts[id] = currentFail + 1
		}

		newFail := p.failCounts[id]
		newSuccess := p.successCounts[id]

		// Determine previous status from Cache (Primary) or DB (Fallback)
		p.cacheMutex.Lock()
		prevStatus, inCache := p.statusCache[id]
		if !inCache {
			prevStatus = res.prevStatus
			p.statusCache[id] = prevStatus
		}
		p.cacheMutex.Unlock()

		// Determine new status
		newStatus := prevStatus // Default to no change

		if prevStatus {
			// Currently Online: needs 2 fails to go Offline (User requested faster detection ~6s)
			if newFail >= 2 {
				newStatus = false
			}
		} else {
			// Currently Offline: needs 2 successes to go Online
			if newSuccess >= 2 {
				newStatus = true
			}
		}

		shouldUpdateDB := false
		updateQuery := ""
		args := []interface{}{}

		// CASE 1: Status Changed
		if newStatus != prevStatus {
			shouldUpdateDB = true
			statusInt := 0
			if newStatus {
				statusInt = 1
			}
			updateQuery = "UPDATE devices SET is_online = ?, last_seen = datetime('now') WHERE id = ?"
			args = []interface{}{statusInt, id}

			// Update Cache IMMEDIATELY
			p.cacheMutex.Lock()
			p.statusCache[id] = newStatus
			p.cacheMutex.Unlock()

		} else if newStatus && online {
			// CASE 2: No change, but valid ping. Update last_seen.
			shouldUpdateDB = true
			updateQuery = "UPDATE devices SET last_seen = datetime('now') WHERE id = ?"
			args = []interface{}{id}
		}

		if shouldUpdateDB {
			newStatusForSync := newStatus
			deviceIPForSync := res.deviceIP
			// Push to global DB worker (serialized)
			p.dbWorker.Push(func(db *sql.DB) error {
				// Use shared dbutils for retry (double safety)
				// Capture variables by value: updateQuery, args... (args is slice, careful)
				// updateQuery is string (safe). args is []interface{} (slice is ref, but created fresh each loop, so safe)
				_, err := dbutils.ExecWithRetry(db, updateQuery, args...)
				if err != nil {
					log.Printf("[Pinger] DB Update failed for %s: %v", res.deviceIP, err)
					return err
				}
				p.syncCameraStatusByIP(db, deviceIPForSync, newStatusForSync)
				return err
			})

			if newStatus != prevStatus {
				// Log event AND Dispatch Alert
				p.logEventAndAlert(res.deviceID, res.deviceIP, res.deviceName, newStatus)
			}
		}
	}
}

func (p *Pinger) syncCameraStatusByIP(db *sql.DB, ip string, isOnline bool) {
	status := "offline"
	if isOnline {
		status = "online"
	}

	result, err := dbutils.ExecWithRetry(
		db,
		"UPDATE cameras SET status = ?, last_seen = datetime('now') WHERE ip_address = ?",
		status, ip,
	)
	if err != nil {
		log.Printf("[Pinger] Failed to sync camera status for %s: %v", ip, err)
		return
	}

	if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
		log.Printf("[Pinger] Synced %d camera row(s) for %s -> %s", rows, ip, status)
	}
}

func (p *Pinger) logEventAndAlert(id int, ip, name string, newStatus bool) {
	eventType := "status_change"
	severity := "warning"
	msg := fmt.Sprintf("設備 '%s'（%s）已離線", name, ip)

	if newStatus {
		severity = "info"
		msg = fmt.Sprintf("設備 '%s'（%s）已上線", name, ip)
	}

	// 1. Log to Database (Events table + in-app Notifications) via Worker
	notifTitle := fmt.Sprintf("設備離線：%s", name)
	if newStatus {
		notifTitle = fmt.Sprintf("設備上線：%s", name)
	}
	capturedTitle := notifTitle
	capturedSeverity := severity
	capturedMsg := msg

	p.dbWorker.Push(func(db *sql.DB) error {
		_, err := dbutils.ExecWithRetry(db, `
			INSERT INTO events (device_id, event_type, severity, message, created_at)
			VALUES (?, ?, ?, ?, datetime('now'))
		`, id, eventType, capturedSeverity, capturedMsg)

		if err != nil {
			log.Printf("[Pinger] Failed to insert event: %v", err)
		} else {
			log.Printf("[Pinger] Device %s (%s) changed state to %s", ip, name, capturedSeverity)
		}

		// 寫入 in-app notification
		_, _ = db.Exec(`
			INSERT INTO notifications (severity, title, message, is_read, created_at)
			VALUES (?, ?, ?, 0, datetime('now'))
		`, capturedSeverity, capturedTitle, capturedMsg)

		return err
	})

	// 2. Dispatch Alerts using shared manager
	go alert.DispatchToEnabledChannels(p.db, p.config, msg)

	// 3. Notify cloud connector (if configured)
	if p.OnStatusChange != nil {
		go p.OnStatusChange(id, ip, name, newStatus)
	}
}

func checkPing(ip string) bool {
	// 1. Try ICMP Ping with Strict checking
	if runtime.GOOS == "windows" {
		// Windows: Check output for "TTL=" to avoid "Destination host unreachable" (which has exit code 0)
		// Reduced timeout to 1000ms (1s)
		cmd := exec.Command("ping", "-n", "1", "-w", "1000", ip)
		output, err := cmd.Output()

		if err == nil {
			// Must contain TTL= to be a real success
			if strings.Contains(strings.ToUpper(string(output)), "TTL=") {
				return true
			}
		}
	} else {
		// Linux/Mac: Exit code is usually reliable
		// Reduced timeout to 1s
		cmd := exec.Command("ping", "-c", "1", "-W", "1", ip)
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	// 2. Fallback to TCP Probe if ICMP failed
	// Optimized ports list as requested
	ports := []string{"554", "80", "8080", "443", "22", "161"}
	for _, port := range ports {
		// Reduced timeout to 500ms
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
	}

	return false
}
