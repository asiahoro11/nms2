// Made by YTSworks
// YTS工作室製作
package snmp

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"management-server/services/edgecore"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// deviceSSHCreds loads SSH credentials for a device from DB.
// Returns ip, cliUsername, cliPassword, vendor. cliUsername/Password may be empty.
func deviceSSHCreds(db *sql.DB, deviceID int) (ip, cliUsername, cliPassword, vendor string, err error) {
	err = db.QueryRow(`
		SELECT ip_address, COALESCE(cli_username,''), COALESCE(cli_password,''), COALESCE(vendor,'')
		FROM devices WHERE id = ?`, deviceID).
		Scan(&ip, &cliUsername, &cliPassword, &vendor)
	return
}

// isEdgecore returns true if vendor string matches EdgeCore / Accton / ECS series
func isEdgecore(vendor string) bool {
	v := strings.ToLower(vendor)
	return strings.Contains(v, "edgecore") ||
		strings.Contains(v, "accton") ||
		strings.Contains(v, "ecs2100") ||
		strings.Contains(v, "ecs") && strings.Contains(v, "100")
}

// PoE Port OID (controls.go — OIDs shared with collector.go use the collector.go declarations)
var (
	// RFC 3621 - POWER-ETHERNET-MIB
	OIDPethPsePortAdminEnable     = ".1.3.6.1.2.1.105.1.1.1.3"     // 1:auto/on, 2:off
	OIDPethPsePortDetectionStatus = ".1.3.6.1.2.1.105.1.1.1.6"     // 1:disabled, 2:searching, 3:deliveringPower, ...
	OIDPethMainPsePower           = ".1.3.6.1.2.1.105.1.2.1.1.2.1" // Total Watts (budget)

	// Edgecore Specific (廠商專用)
	OIDEdgecoreReboot = ".1.3.6.1.4.1.259.10.1.1.1.1.1.10.0" // System Reset (1:reset)

	// Q-BRIDGE-MIB (VLAN)
	OIDDot1qPvid = ".1.3.6.1.2.1.17.7.1.4.5.1.1" // Port VLAN ID

	// Power Ethernet MIB Extensions
	OIDPethPsePortIfIndex           = ".1.3.6.1.2.1.105.1.1.1.1"  // Map PoE Port -> ifIndex
	OIDPethPsePortMeasuredPortPower = ".1.3.6.1.2.1.105.1.1.1.15" // Output power (mW) — not all vendors
	OIDPethPsePortPowerClass        = ".1.3.6.1.2.1.105.1.1.1.10" // pethPsePortPowerClassifications (0-4)

	// EdgeCore Private PoE OIDs
	OIDEdgecorePoePortPower = ".1.3.6.1.4.1.259.10.1.43.1.28.6.1.14.1" // Per-port real-time PoE power (mW), indexed by port number

	// Wireless / AP OIDs (Edgecore / Accton common)
	OIDApSsid        = ".1.3.6.1.4.1.259.10.1.45.1.4.1.1.2" // SSID Table
	OIDApClientCount = ".1.3.6.1.4.1.259.10.1.45.1.1.1.3"   // Total Associated Clients
)

// ApStatus represents wireless-specific information
type ApStatus struct {
	ClientCount int      `json:"client_count"`
	SSIDs       []string `json:"ssids"`
}

// GetDevicePortDetails gets comprehensive port information (Status, VLAN, PoE)
func (c *Collector) GetDevicePortDetails(deviceID int) ([]map[string]interface{}, error) {
	var ip, community, rwCommunity, vendor string
	var version int
	err := c.db.QueryRow("SELECT ip_address, snmp_community, COALESCE(snmp_rw_community, ''), snmp_version, COALESCE(vendor,'') FROM devices WHERE id = ?", deviceID).Scan(&ip, &community, &rwCommunity, &version, &vendor)
	if err != nil {
		return nil, err
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}

	if err := params.Connect(); err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	// Map: IfIndex -> PortData
	ports := make(map[int]map[string]interface{})

	// helper to ensure map entry exists
	ensurePort := func(idx int) {
		if ports[idx] == nil {
			ports[idx] = make(map[string]interface{})
			ports[idx]["index"] = idx
			ports[idx]["poe"] = false // default
		}
	}

	// 1. Walk Interfaces
	// Descr (basic device info)
	params.Walk(OIDIfDescr, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		ensurePort(idx)
		if bytes, ok := pdu.Value.([]byte); ok {
			val := string(bytes)
			ports[idx]["descr"] = val
			ports[idx]["name"] = val // 預設 name 為 descr
		}
		return nil
	})
	// Name (如果支援則覆寫 name, 通常較簡短)
	params.Walk(OIDIfName, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		if ports[idx] != nil {
			if bytes, ok := pdu.Value.([]byte); ok {
				ports[idx]["name"] = string(bytes)
			}
		}
		return nil
	})
	// Type
	params.Walk(OIDIfType, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		ensurePort(idx)
		ports[idx]["type"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})
	// OperStatus
	params.Walk(OIDIfOperStatus, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		ensurePort(idx)
		ports[idx]["oper_status"] = gosnmp.ToBigInt(pdu.Value).Int64() // 1:up, 2:down
		return nil
	})
	// Speed
	params.Walk(OIDIfSpeed, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		ensurePort(idx)
		ports[idx]["speed"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	// 2. Walk VLAN (PVID)
	params.Walk(OIDDot1qPvid, func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndexFromOID(pdu.Name)
		// PVID is indexed by ifIndex usually
		if ports[idx] != nil {
			ports[idx]["vlan"] = gosnmp.ToBigInt(pdu.Value).Int64()
		}
		return nil
	})

	// 3. PoE Mapping: pethPsePortIfIndex maps PoE port index -> ifIndex
	// Some switches (e.g. ECS2100) do NOT implement this optional OID.
	// Fallback: assume PoE portIndex == ifIndex (1-based, common on access switches).
	poeMap := make(map[int]int) // poePortIndex -> ifIndex
	params.Walk(OIDPethPsePortIfIndex, func(pdu gosnmp.SnmpPDU) error {
		// OID suffix: grpIndex.poeIndex ??take last component as poeIndex
		poeIdx := extractIndexFromOID(pdu.Name)
		ifIdx := int(gosnmp.ToBigInt(pdu.Value).Int64())
		if ifIdx > 0 {
			poeMap[poeIdx] = ifIdx
		}
		return nil
	})
	// poeMapLookup resolves poePortIndex -> ifIndex with identity fallback
	poeMapLookup := func(poeIdx int) (int, bool) {
		if ifIdx, ok := poeMap[poeIdx]; ok {
			return ifIdx, true
		}
		// Fallback: treat poePortIndex as ifIndex directly
		// (valid for ECS2100 and similar single-group PoE switches)
		if ports[poeIdx] != nil {
			return poeIdx, true
		}
		return 0, false
	}

	// 4. Walk PoE Data
	// Admin Status (1=enabled/auto, 2=disabled)
	params.Walk(OIDPethPsePortAdminEnable, func(pdu gosnmp.SnmpPDU) error {
		poeIdx := extractIndexFromOID(pdu.Name)
		ifIdx, ok := poeMapLookup(poeIdx)
		if !ok {
			return nil
		}
		ensurePort(ifIdx)
		ports[ifIdx]["poe"] = true
		ports[ifIdx]["poe_index"] = poeIdx
		ports[ifIdx]["poe_admin"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})
	// Detection Status
	params.Walk(OIDPethPsePortDetectionStatus, func(pdu gosnmp.SnmpPDU) error {
		poeIdx := extractIndexFromOID(pdu.Name)
		ifIdx, ok := poeMapLookup(poeIdx)
		if !ok {
			return nil
		}
		ensurePort(ifIdx)
		val := gosnmp.ToBigInt(pdu.Value).Int64()
		statusText := "unknown"
		switch val {
		case 1:
			statusText = "disabled"
		case 2:
			statusText = "searching"
		case 3:
			statusText = "delivering"
		case 4:
			statusText = "fault"
		case 5:
			statusText = "test"
		case 6:
			statusText = "otherFault"
		}
		ports[ifIdx]["poe_status_text"] = statusText
		ports[ifIdx]["poe_status_code"] = val
		ports[ifIdx]["poe"] = true
		return nil
	})
	// Per-port measured power (mW) ??RFC3621 .15; NOT supported on ECS2100 but works on others
	params.Walk(OIDPethPsePortMeasuredPortPower, func(pdu gosnmp.SnmpPDU) error {
		poeIdx := extractIndexFromOID(pdu.Name)
		raw := gosnmp.ToBigInt(pdu.Value).Int64()
		ifIdx, ok := poeMapLookup(poeIdx)
		if !ok {
			return nil
		}
		ensurePort(ifIdx)
		ports[ifIdx]["poe_power_mw"] = raw
		return nil
	})

	// Per-port PoE class (0=class0 ~15.4W, 1=class1 ~4W, 2=class2 ~7W, 3=class3 ~15.4W, 4=class4 ~30W)
	// Used as fallback when measured power is unavailable (e.g. ECS2100)
	params.Walk(OIDPethPsePortPowerClass, func(pdu gosnmp.SnmpPDU) error {
		poeIdx := extractIndexFromOID(pdu.Name)
		raw := gosnmp.ToBigInt(pdu.Value).Int64()
		ifIdx, ok := poeMapLookup(poeIdx)
		if !ok {
			return nil
		}
		ensurePort(ifIdx)
		ports[ifIdx]["poe_class"] = raw
		// If no measured power yet, estimate from class
		if _, hasPower := ports[ifIdx]["poe_power_mw"]; !hasPower {
			classMaxMw := map[int64]int64{0: 15400, 1: 4000, 2: 7000, 3: 15400, 4: 30000}
			if maxMw, ok := classMaxMw[raw]; ok {
				ports[ifIdx]["poe_class_max_mw"] = maxMw
			}
		}
		return nil
	})

	// EdgeCore private per-port PoE power (mW) ??confirmed on ECS2100/ECS4150/ECS2220
	// OID: 1.3.6.1.4.1.259.10.1.43.1.28.6.1.14.1.(portIndex)
	// Walk only for EdgeCore devices to avoid unnecessary SNMP timeouts on other vendors
	if isEdgecore(vendor) {
		params.Walk(OIDEdgecorePoePortPower, func(pdu gosnmp.SnmpPDU) error {
			poeIdx := extractIndexFromOID(pdu.Name)
			raw := gosnmp.ToBigInt(pdu.Value).Int64()
			ifIdx, ok := poeMapLookup(poeIdx)
			if !ok {
				return nil
			}
			ensurePort(ifIdx)
			// Override class estimate with real measured value (even if 0 = no device)
			ports[ifIdx]["poe_power_mw"] = raw
			return nil
		})
	}

	// Total PoE budget and consumption from MainPse table (supported on ECS2100)
	var poeBudgetMw, poeUsedMw int64
	if result, err := params.Get([]string{
		OIDPethMainPseCapacity + ".1",
		OIDPethMainPseConsumption + ".1",
	}); err == nil {
		for _, pdu := range result.Variables {
			switch {
			case strings.HasSuffix(pdu.Name, OIDPethMainPseCapacity+".1") ||
				strings.Contains(pdu.Name, "105.1.3.1.1.2"):
				poeBudgetMw = gosnmp.ToBigInt(pdu.Value).Int64()
			case strings.HasSuffix(pdu.Name, OIDPethMainPseConsumption+".1") ||
				strings.Contains(pdu.Name, "105.1.3.1.1.4"):
				poeUsedMw = gosnmp.ToBigInt(pdu.Value).Int64()
			}
		}
	}

	// Convert map to slice
	var result []map[string]interface{}
	for _, v := range ports {
		result = append(result, v)
	}

	// Inject total PoE stats as a synthetic entry so frontend can display it
	if poeBudgetMw > 0 || poeUsedMw > 0 {
		for i := range result {
			result[i]["poe_budget_mw"] = poeBudgetMw
			result[i]["poe_used_mw"] = poeUsedMw
		}
	}

	return result, nil
}

// GetApStatus gets wireless-specific information
func (c *Collector) GetApStatus(deviceID int) (*ApStatus, error) {
	var ip, community string
	var version int
	err := c.db.QueryRow("SELECT ip_address, snmp_community, snmp_version FROM devices WHERE id = ?", deviceID).Scan(&ip, &community, &version)
	if err != nil {
		return nil, err
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}

	if err := params.Connect(); err != nil {
		return nil, err
	}
	defer params.Conn.Close()

	status := &ApStatus{
		SSIDs: []string{},
	}

	// 1. Get Client Count
	result, err := params.Get([]string{OIDApClientCount + ".0"})
	if err == nil && len(result.Variables) > 0 {
		status.ClientCount = int(gosnmp.ToBigInt(result.Variables[0].Value).Int64())
	}

	// 2. Walk SSIDs
	params.Walk(OIDApSsid, func(pdu gosnmp.SnmpPDU) error {
		if bytes, ok := pdu.Value.([]byte); ok {
			ssid := string(bytes)
			if ssid != "" {
				status.SSIDs = append(status.SSIDs, ssid)
			}
		}
		return nil
	})

	return status, nil
}

// ControlPoEPort controls the PoE port power state
// EdgeCore: SSH CLI preferred. Other vendors or no SSH creds: SNMP SET.
func (c *Collector) ControlPoEPort(deviceID int, portIndex int, action string) error {
	ip, cliUser, cliPass, vendor, err := deviceSSHCreds(c.db, deviceID)
	if err != nil {
		return err
	}

	log.Printf("[PoE] device=%d ip=%s vendor=%q cliUser=%q action=%s port=%d isEdgecore=%v hasCreds=%v",
		deviceID, ip, vendor, cliUser, action, portIndex, isEdgecore(vendor), cliUser != "" && cliPass != "")

	if action != "on" && action != "off" && action != "recycle" {
		return errors.New("invalid action: must be on/off/recycle")
	}

	// EdgeCore via SSH
	if isEdgecore(vendor) && cliUser != "" && cliPass != "" {
		sshClient, sshErr := edgecore.NewSSHClient(ip, cliUser, cliPass)
		if sshErr != nil {
			log.Printf("EdgeCore SSH PoE control failed for device %d, falling back to SNMP: %v", deviceID, sshErr)
		} else {
			defer sshClient.Close()
			var opErr error
			switch action {
			case "on":
				opErr = sshClient.PoEPortEnable(portIndex)
			case "off":
				opErr = sshClient.PoEPortDisable(portIndex)
			case "recycle":
				opErr = sshClient.PoEPortRecycle(portIndex, 3)
			}
			if opErr != nil {
				log.Printf("EdgeCore SSH PoE %s port %d failed for device %d: %v", action, portIndex, deviceID, opErr)
				// fall through to SNMP
			} else {
				log.Printf("EdgeCore device %d PoE port %d action=%s via SSH", deviceID, portIndex, action)
				return nil
			}
		}
	}

	// SNMP SET fallback
	var community, rwCommunity string
	var version int
	err = c.db.QueryRow("SELECT snmp_community, COALESCE(snmp_rw_community, ''), snmp_version FROM devices WHERE id = ?", deviceID).
		Scan(&community, &rwCommunity, &version)
	if err != nil {
		return err
	}
	if rwCommunity != "" {
		community = rwCommunity
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}
	if err := params.Connect(); err != nil {
		return err
	}
	defer params.Conn.Close()

	// grpIndex.portIndex ??ECS2100 uses group 1
	targetOID := fmt.Sprintf("%s.1.%d", OIDPethPsePortAdminEnable, portIndex)

	switch action {
	case "on":
		_, err = params.Set([]gosnmp.SnmpPDU{{Name: targetOID, Type: gosnmp.Integer, Value: 1}})
		return err
	case "off":
		_, err = params.Set([]gosnmp.SnmpPDU{{Name: targetOID, Type: gosnmp.Integer, Value: 2}})
		return err
	case "recycle":
		params.Set([]gosnmp.SnmpPDU{{Name: targetOID, Type: gosnmp.Integer, Value: 2}})
		time.Sleep(3 * time.Second)
		_, err = params.Set([]gosnmp.SnmpPDU{{Name: targetOID, Type: gosnmp.Integer, Value: 1}})
		return err
	default:
		return errors.New("invalid action")
	}
}

// RebootDevice reboots a network device
// EdgeCore/Accton: uses SSH CLI "reload". Other vendors: SNMP SET.
func (c *Collector) RebootDevice(deviceID int) error {
	ip, cliUser, cliPass, vendor, err := deviceSSHCreds(c.db, deviceID)
	if err != nil {
		return err
	}

	log.Printf("[Reboot] device=%d ip=%s vendor=%q cliUser=%q isEdgecore=%v hasCreds=%v",
		deviceID, ip, vendor, cliUser, isEdgecore(vendor), cliUser != "" && cliPass != "")

	// EdgeCore via SSH (preferred ??SNMP RW write is often disabled)
	if isEdgecore(vendor) && cliUser != "" && cliPass != "" {
		sshClient, err := edgecore.NewSSHClient(ip, cliUser, cliPass)
		if err != nil {
			log.Printf("EdgeCore SSH reboot failed for device %d, falling back to SNMP: %v", deviceID, err)
			// fall through to SNMP
		} else {
			defer sshClient.Close()
			if err := sshClient.Reboot(); err != nil {
				log.Printf("EdgeCore SSH reboot command failed for device %d: %v", deviceID, err)
				// fall through to SNMP
			} else {
				log.Printf("EdgeCore device %d reboot triggered via SSH", deviceID)
				return nil
			}
		}
	}

	// SNMP SET fallback (or non-EdgeCore vendors)
	var community, rwCommunity string
	var version int
	err = c.db.QueryRow("SELECT snmp_community, COALESCE(snmp_rw_community, ''), snmp_version FROM devices WHERE id = ?", deviceID).
		Scan(&community, &rwCommunity, &version)
	if err != nil {
		return err
	}
	if rwCommunity != "" {
		community = rwCommunity
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}
	if err := params.Connect(); err != nil {
		return err
	}
	defer params.Conn.Close()

	v := strings.ToLower(vendor)
	var oids []string
	val := 1

	switch {
	case strings.Contains(v, "edgecore") || strings.Contains(v, "accton"):
		oids = []string{".1.3.6.1.4.1.259.10.1.1.1.1.1.10.0", ".1.3.6.1.4.1.259.10.1.1.1.1.1.10"}
	case strings.Contains(v, "tp-link") || strings.Contains(v, "tplink"):
		oids = []string{
			".1.3.6.1.4.1.11863.6.1.1.5.0",
			".1.3.6.1.4.1.11863.1.1.101.1.0",
			".1.3.6.1.4.1.11863.1.1.1.1.3.0",
		}
	case strings.Contains(v, "d-link"):
		oids = []string{".1.3.6.1.4.1.171.12.1.2.3.0"}
		val = 3
	case strings.Contains(v, "cisco"):
		oids = []string{".1.3.6.1.4.1.9.2.1.55.0"}
	default:
		oids = []string{
			".1.3.6.1.4.1.259.10.1.1.1.1.1.10.0",
			".1.3.6.1.4.1.11863.6.1.1.5.0",
			".1.3.6.1.4.1.171.12.1.2.3.0",
		}
	}

	var lastErr error
	for _, oid := range oids {
		pdu := gosnmp.SnmpPDU{Name: oid, Type: gosnmp.Integer, Value: val}
		_, lastErr = params.Set([]gosnmp.SnmpPDU{pdu})
		if lastErr == nil {
			return nil
		}
		log.Printf("Reboot SNMP attempt failed for OID %s: %v", oid, lastErr)
	}
	return lastErr
}

// SaveDeviceConfig backs up device configuration
// EdgeCore: SSH "show running-config" ??store in device_config_backups.
// Other vendors: SNMP write-memory OID.
func (c *Collector) SaveDeviceConfig(deviceID int) error {
	ip, cliUser, cliPass, vendor, err := deviceSSHCreds(c.db, deviceID)
	if err != nil {
		return err
	}

	// EdgeCore via SSH ??full running-config backup
	if isEdgecore(vendor) && cliUser != "" && cliPass != "" {
		sshClient, err := edgecore.NewSSHClient(ip, cliUser, cliPass)
		if err != nil {
			return fmt.Errorf("SSH connect for config backup: %w", err)
		}
		defer sshClient.Close()

		configText, err := sshClient.ShowRunningConfig()
		if err != nil {
			return fmt.Errorf("show running-config: %w", err)
		}

		_, err = c.db.Exec(`
			INSERT INTO device_config_backups (device_id, content, note, created_at)
			VALUES (?, ?, 'manual', datetime('now'))
		`, deviceID, configText)
		if err != nil {
			return fmt.Errorf("store config backup: %w", err)
		}

		log.Printf("EdgeCore device %d config backup stored (%d bytes)", deviceID, len(configText))
		return nil
	}

	// SNMP write-memory for other vendors
	var community, rwCommunity string
	var version int
	err = c.db.QueryRow("SELECT snmp_community, COALESCE(snmp_rw_community, ''), snmp_version FROM devices WHERE id = ?", deviceID).
		Scan(&community, &rwCommunity, &version)
	if err != nil {
		return err
	}
	if rwCommunity != "" {
		community = rwCommunity
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}
	if err := params.Connect(); err != nil {
		return err
	}
	defer params.Conn.Close()

	v := strings.ToLower(vendor)
	var oids []string
	val := 1

	switch {
	case strings.Contains(v, "tp-link") || strings.Contains(v, "tplink"):
		oids = []string{
			".1.3.6.1.4.1.11863.6.1.1.6.0",
			".1.3.6.1.4.1.11863.1.1.101.2.0",
			".1.3.6.1.4.1.11863.1.1.1.1.2.0",
		}
	case strings.Contains(v, "d-link"):
		oids = []string{".1.3.6.1.4.1.171.12.1.2.6.0"}
		val = 2
	default:
		oids = []string{
			".1.3.6.1.4.1.259.10.1.1.1.1.1.11.0",
			".1.3.6.1.4.1.11863.6.1.1.6.0",
			".1.3.6.1.4.1.171.12.1.2.6.0",
		}
	}

	var lastErr error
	for _, oid := range oids {
		pdu := gosnmp.SnmpPDU{Name: oid, Type: gosnmp.Integer, Value: val}
		_, lastErr = params.Set([]gosnmp.SnmpPDU{pdu})
		if lastErr == nil {
			return nil
		}
		log.Printf("Save config SNMP attempt failed for OID %s: %v", oid, lastErr)
	}
	return lastErr
}

// ControlPortStatus controls the interface Admin Status (shutdown/no-shutdown)
func (c *Collector) ControlPortStatus(deviceID int, portIndex int, status string) error {
	var ip, community, rwCommunity string
	var version int
	err := c.db.QueryRow("SELECT ip_address, snmp_community, COALESCE(snmp_rw_community, ''), snmp_version FROM devices WHERE id = ?", deviceID).Scan(&ip, &community, &rwCommunity, &version)
	if err != nil {
		return err
	}

	if rwCommunity != "" {
		community = rwCommunity
	}

	snmpVersion := gosnmp.Version2c
	if version == 1 {
		snmpVersion = gosnmp.Version1
	}

	params := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      161,
		Community: community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}

	if err := params.Connect(); err != nil {
		return err
	}
	defer params.Conn.Close()

	// OIDIfAdminStatus = ".1.3.6.1.2.1.2.2.1.7"
	targetOID := fmt.Sprintf(".1.3.6.1.2.1.2.2.1.7.%d", portIndex)

	var val int
	if status == "up" {
		val = 1 // up
	} else if status == "down" {
		val = 2 // down
	} else {
		return errors.New("invalid status")
	}

	pdu := gosnmp.SnmpPDU{
		Name:  targetOID,
		Type:  gosnmp.Integer,
		Value: val,
	}

	_, err = params.Set([]gosnmp.SnmpPDU{pdu})
	return err
}

func extractIndexFromOID(oid string) int {
	parts := strings.Split(oid, ".")
	if len(parts) == 0 {
		return 0
	}
	val := 0
	fmt.Sscanf(parts[len(parts)-1], "%d", &val)
	return val
}
