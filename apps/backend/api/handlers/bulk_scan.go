// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"encoding/binary"
	"fmt"
	"log"
	"management-server/services/alert"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gosnmp/gosnmp"
)

const (
	bulkScanWorkerCount    = 20
	bulkScanLaunchInterval = 20 * time.Millisecond
)

// BulkScanRequest is the input for a bulk scan request
type BulkScanRequest struct {
	ScanType            string `json:"scan_type"` // "range" or "subnet"
	StartIP             string `json:"start_ip"`  // for range scan
	EndIP               string `json:"end_ip"`    // for range scan
	Subnet              string `json:"subnet"`    // for subnet scan (CIDR notation)
	NamePrefix          string `json:"name_prefix"`
	DeviceType          string `json:"device_type"`
	MonitorType         string `json:"monitor_type"` // "ping" or "snmp"
	SNMPCommunity       string `json:"snmp_community"`
	SNMPVersion         int    `json:"snmp_version"`
	IncludePingOnly     bool   `json:"include_ping_only"`
	SNMPV3SecurityName  string `json:"snmpv3_security_name"`
	SNMPV3SecurityLevel string `json:"snmpv3_security_level"`
	SNMPV3AuthProtocol  string `json:"snmpv3_auth_protocol"`
	SNMPV3AuthPassword  string `json:"snmpv3_auth_password"`
	SNMPV3PrivProtocol  string `json:"snmpv3_priv_protocol"`
	SNMPV3PrivPassword  string `json:"snmpv3_priv_password"`
	SNMPV3ContextName   string `json:"snmpv3_context_name"`
}

// BulkScanResult holds the result for a single scanned host
type BulkScanResult struct {
	Found int `json:"found"`
	Added int `json:"added"`
}

// BulkScan scans a subnet or IP range and registers discovered devices
func (h *Handler) BulkScan(c *gin.Context) {
	var req BulkScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.WriteSystemLog("warning", "discovery", "bulk_scan_invalid_request", "bulk scan request invalid", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	// 預設值
	if req.NamePrefix == "" {
		req.NamePrefix = "Device-"
	}
	if req.DeviceType == "" {
		req.DeviceType = "other"
	}
	if req.MonitorType == "" {
		req.MonitorType = "ping"
	}

	// 解析要掃描的 IP 列表
	var ips []string
	var err error

	if req.ScanType == "subnet" {
		subnets := strings.Split(req.Subnet, ",")
		for _, s := range subnets {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			subIPs, err := expandSubnet(s)
			if err != nil {
				h.WriteSystemLog("warning", "discovery", "bulk_scan_invalid_subnet", "bulk scan rejected: invalid subnet", map[string]interface{}{
					"subnet": s,
					"error":  err.Error(),
				})
				c.JSON(http.StatusBadRequest, Response{Success: false, Error: fmt.Sprintf("Invalid subnet %s: %v", s, err)})
				return
			}
			ips = append(ips, subIPs...)
		}
	} else {
		ips, err = expandIPRange(req.StartIP, req.EndIP)
	}

	if err != nil {
		h.WriteSystemLog("warning", "discovery", "bulk_scan_invalid_range", "bulk scan rejected: invalid ip range", map[string]interface{}{
			"start_ip": req.StartIP,
			"end_ip":   req.EndIP,
			"error":    err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	// 限制掃描範圍避免過大
	if len(ips) > 1024 {
		h.WriteSystemLog("warning", "discovery", "bulk_scan_rejected", "bulk scan rejected: too many ips", map[string]interface{}{
			"count": len(ips),
			"max":   1024,
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: "Too many IPs to scan (max 1024)"})
		return
	}

	// limit batch size
	maxDevices := h.getMaxDeviceLimit()
	var initialCount int
	if err := h.db.QueryRow("SELECT COUNT(*) FROM devices").Scan(&initialCount); err != nil {
		h.WriteSystemLog("error", "discovery", "bulk_scan_failed", "bulk scan failed: device count query error", map[string]interface{}{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteSystemLog("notice", "discovery", "bulk_scan_started", "bulk scan started", map[string]interface{}{
		"scan_type":     req.ScanType,
		"monitor_type":  req.MonitorType,
		"candidate_ips": len(ips),
		"initial_count": initialCount,
		"max_devices":   maxDevices,
		"include_ping":  req.IncludePingOnly,
		"device_type":   req.DeviceType,
		"name_prefix":   req.NamePrefix,
	})

	// concurrent scan
	var found, added int
	limitReached := false
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 使用 worker pool
	workerCount := bulkScanWorkerCount
	jobs := make(chan string, len(ips))

	// 第一階段掃描（SNMP 或 Ping）
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range jobs {
				isOnline := false
				isSnmp := false

				if req.MonitorType == "snmp" {
					// 1. Try SNMP
					if checkSNMP(ip, req) {
						isOnline = true
						isSnmp = true
					} else if req.IncludePingOnly {
						// 2. If SNMP failed but Ping Only allowed, Try Ping
						if checkPing(ip) {
							isOnline = true
							isSnmp = false // It's a Ping device
						}
					}
				} else {
					// Ping Only 模式
					if checkPing(ip) {
						isOnline = true
						isSnmp = false
					}
				}

				if isOnline {
					mu.Lock()
					found++

					// 檢查數量限制
					if initialCount+added >= maxDevices {
						limitReached = true
						mu.Unlock()
						continue
					}

					// 檢查是否已存在
					var exists int
					h.db.QueryRow("SELECT COUNT(*) FROM devices WHERE ip_address = ?", ip).Scan(&exists)

					if exists == 0 {
						name := fmt.Sprintf("%s%s", req.NamePrefix, strings.ReplaceAll(ip, ".", "-"))
						snmpCommunity := ""
						snmpVersion := 0

						// Only set SNMP details if it was actually detected via SNMP
						if isSnmp {
							snmpCommunity = req.SNMPCommunity
							if snmpCommunity == "" && req.SNMPVersion != 3 {
								snmpCommunity = "public"
							}
							snmpVersion = req.SNMPVersion
							if snmpVersion == 0 {
								snmpVersion = 2
							}
						}

						_, err := h.db.Exec(`
							INSERT INTO devices (name, ip_address, device_type, snmp_community, snmp_version, is_online, last_seen, is_name_custom,
							                    snmpv3_security_name, snmpv3_security_level, snmpv3_auth_protocol, snmpv3_auth_password,
							                    snmpv3_priv_protocol, snmpv3_priv_password, snmpv3_context_name)
							VALUES (?, ?, ?, ?, ?, 1, datetime('now'), 0, ?, ?, ?, ?, ?, ?, ?)
						`, name, ip, req.DeviceType, snmpCommunity, snmpVersion,
							req.SNMPV3SecurityName, req.SNMPV3SecurityLevel, req.SNMPV3AuthProtocol, req.SNMPV3AuthPassword,
							req.SNMPV3PrivProtocol, req.SNMPV3PrivPassword, req.SNMPV3ContextName)

						if err == nil {
							added++
						}
					}
					mu.Unlock()
				}
			}
		}()
	}

	// 派送任務
	launchTicker := time.NewTicker(bulkScanLaunchInterval)
	defer launchTicker.Stop()
	for _, ip := range ips {
		select {
		case <-c.Request.Context().Done():
			close(jobs)
			wg.Wait()
			c.JSON(http.StatusRequestTimeout, Response{Success: false, Error: "bulk scan cancelled"})
			return
		case <-launchTicker.C:
		}
		jobs <- ip
	}
	close(jobs)
	wg.Wait()

	// 記錄事件
	username := c.GetString("username")
	if username == "" {
		username = "unknown"
	}

	msg := fmt.Sprintf("使用者 '%s' 透過 %s 完成批次掃描，新增 %d 台設備", username, req.MonitorType, added)

	h.db.Exec(`
		INSERT INTO events (event_type, severity, message)
		VALUES ('bulk_scan', 'info', ?)
	`, msg)

	if added > 0 {
		go alert.DispatchToEnabledChannels(h.db, h.config, msg)
	}

	// 如果有新增設備且使用 SNMP 監控, 立即觸發一次 SNMP 輪詢
	if added > 0 && req.MonitorType == "snmp" && h.snmpCollector != nil {
		go func() {
			time.Sleep(1 * time.Second) // 稍微等待以確保資料已提交
			h.snmpCollector.PollAllDevices()
		}()
	}

	scanned := len(ips)
	alreadyExists := found - added

	responseMsg := ""
	if req.MonitorType == "snmp" {
		responseMsg = fmt.Sprintf("\u63b4\u63cf\u5b8c\u6210\uff01\u6383\u63cf\u4e86 %d \u500b IP\uff0c\u627e\u5230 %d \u53f0\u652f\u63f4 SNMP \u7684\u8a2d\u5099\uff0c\u65b0\u589e\u4e86 %d \u53f0\u65b0\u8a2d\u5099", scanned, found, added)
		if alreadyExists > 0 {
			responseMsg += fmt.Sprintf("\uff0c%d \u53f0\u8a2d\u5099\u5df2\u5b58\u5728", alreadyExists)
		}
		notFound := scanned - found
		if notFound > 0 {
			responseMsg += fmt.Sprintf("\uff0c%d \u500b IP \u7121\u56de\u61c9\u6216\u4e0d\u652f\u63f4 SNMP", notFound)
		}
	} else {
		responseMsg = fmt.Sprintf("\u63b4\u63cf\u5b8c\u6210\uff01\u6383\u63cf\u4e86 %d \u500b IP\uff0c\u627e\u5230 %d \u53f0\u5728\u7dda\u8a2d\u5099\uff0c\u65b0\u589e\u4e86 %d \u53f0\u65b0\u8a2d\u5099", scanned, found, added)
		if alreadyExists > 0 {
			responseMsg += fmt.Sprintf("\uff0c%d \u53f0\u8a2d\u5099\u5df2\u5b58\u5728", alreadyExists)
		}
	}

	if limitReached {
		h.WriteSystemLog("warning", "discovery", "bulk_scan_license_limit", "bulk scan hit device license limit", map[string]interface{}{
			"added":       added,
			"found":       found,
			"max_devices": maxDevices,
		})
		responseMsg += "\uff0c\u5df2\u9054\u8a2d\u5099\u6578\u91cf\u4e0a\u9650\uff0c\u90e8\u5206\u8a2d\u5099\u672a\u65b0\u589e\u3002\u8acb\u8cfc\u8cb7\u66f4\u591a\u7ba1\u7406\u8a2d\u5099\u6388\u6b0a"
		log.Printf("[BulkScan] Limit reached! Added: %d, Max: %d", added, maxDevices)

		limitMsg := fmt.Sprintf("已達授權數量上限: %d 台，無法新增更多管理設備，請購買授權", maxDevices)
		h.db.Exec(`
			INSERT INTO events (event_type, severity, message)
			VALUES ('license_limit', 'warning', ?)
		`, limitMsg)
	} else {
		log.Printf("[BulkScan] Finished. Scanned: %d, Found: %d, Added: %d", scanned, found, added)
	}

	h.WriteSystemLog("notice", "discovery", "bulk_scan_completed", "bulk scan completed", map[string]interface{}{
		"scan_type":      req.ScanType,
		"monitor_type":   req.MonitorType,
		"scanned":        scanned,
		"found":          found,
		"added":          added,
		"already_exists": alreadyExists,
		"limit_reached":  limitReached,
	})
	h.WriteAuditFromContext(c, "bulk_scan_devices", "discovery", "success", map[string]interface{}{
		"scan_type":      req.ScanType,
		"monitor_type":   req.MonitorType,
		"scanned":        scanned,
		"found":          found,
		"added":          added,
		"already_exists": alreadyExists,
		"limit_reached":  limitReached,
	})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    BulkScanResult{Found: found, Added: added},
		Message: responseMsg,
	})
}

// expandIPRange 展開 IP 範圍
func expandIPRange(startIP, endIP string) ([]string, error) {
	start := net.ParseIP(startIP)
	end := net.ParseIP(endIP)

	if start == nil || end == nil {
		return nil, fmt.Errorf("invalid IP address format")
	}

	start = start.To4()
	end = end.To4()

	if start == nil || end == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}

	startUint := ip2uint(start)
	endUint := ip2uint(end)

	if startUint > endUint {
		return nil, fmt.Errorf("start IP must be less than or equal to end IP")
	}

	var ips []string
	for i := startUint; i <= endUint; i++ {
		ips = append(ips, uint2ip(i).String())
	}

	return ips, nil
}

// expandSubnet 展開子網段
func expandSubnet(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %s", cidr)
	}

	var ips []string
	for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		// 跳過網段位址與廣播位址
		if !isNetworkOrBroadcast(ip, ipnet) {
			ips = append(ips, ip.String())
		}
	}

	return ips, nil
}

func ip2uint(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip)
}

func uint2ip(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func isNetworkOrBroadcast(ip net.IP, ipnet *net.IPNet) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	// 網段位址 (結尾為 0)
	network := ipnet.IP.To4()
	isBroadcast := true
	isNetwork := true

	mask := ipnet.Mask
	for i := 0; i < 4; i++ {
		hostBits := ip4[i] &^ mask[i]
		inverseMask := ^mask[i]

		if hostBits != 0 {
			isNetwork = false
		}
		if hostBits != inverseMask {
			isBroadcast = false
		}
	}

	// 對於 /32 網段，不跳過任何位址
	ones, bits := ipnet.Mask.Size()
	if ones == bits {
		return false
	}

	return isNetwork || isBroadcast || ip4.Equal(network)
}

// checkPing 檢查 Ping 可達性
func checkPing(ip string) bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: -n 1 (次數), -w 1000 (逾時 ms)
		cmd = exec.Command("ping", "-n", "1", "-w", "1000", ip)
	} else {
		// Linux/Unix: -c 1 (次數), -W 1 (逾時 sec)
		cmd = exec.Command("ping", "-c", "1", "-W", "1", ip)
	}

	// Windows ping might return 0 even for 'Destination host unreachable'
	// We need to check output for "TTL=" which confirms a real response from the target
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	res := string(output)
	if runtime.GOOS == "windows" {
		return strings.Contains(res, "TTL=") || strings.Contains(res, "ttl=")
	}
	return true // Linux ping exit code is reliable
}

// checkSNMP 檢查 SNMP 可達性
func checkSNMP(ip string, req BulkScanRequest) bool {
	community := req.SNMPCommunity
	if community == "" && req.SNMPVersion != 3 {
		community = "public"
	}

	snmpVersion := gosnmp.Version2c
	if req.SNMPVersion == 1 {
		snmpVersion = gosnmp.Version1
	} else if req.SNMPVersion == 3 {
		snmpVersion = gosnmp.Version3
	}

	g := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   2 * time.Second,
		Retries:   1,
	}

	if req.SNMPVersion == 3 {
		g.SecurityModel = gosnmp.UserSecurityModel
		g.MsgFlags = gosnmp.NoAuthNoPriv
		if req.SNMPV3SecurityLevel == "authNoPriv" {
			g.MsgFlags = gosnmp.AuthNoPriv
		} else if req.SNMPV3SecurityLevel == "authPriv" {
			g.MsgFlags = gosnmp.AuthPriv
		}

		g.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: req.SNMPV3SecurityName,
		}

		if req.SNMPV3SecurityLevel != "noAuthNoPriv" {
			g.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationPassphrase = req.SNMPV3AuthPassword
			if strings.ToUpper(req.SNMPV3AuthProtocol) == "SHA" {
				g.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationProtocol = gosnmp.SHA
			} else {
				g.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationProtocol = gosnmp.MD5
			}

			if req.SNMPV3SecurityLevel == "authPriv" {
				g.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyPassphrase = req.SNMPV3PrivPassword
				if strings.ToUpper(req.SNMPV3PrivProtocol) == "AES" {
					g.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyProtocol = gosnmp.AES
				} else {
					g.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyProtocol = gosnmp.DES
				}
			}
		}
		g.ContextName = req.SNMPV3ContextName
	}

	err := g.Connect()
	if err != nil {
		return false
	}
	defer g.Conn.Close()

	// 嘗試讀取 sysDescr
	oids := []string{"1.3.6.1.2.1.1.1.0"}
	result, err := g.Get(oids)
	if err != nil || len(result.Variables) == 0 {
		return false
	}

	return true
}
