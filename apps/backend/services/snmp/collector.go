package snmp

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"management-server/config"
	"management-server/pkg/dbutils"
	"management-server/services/dbworker"
	"management-server/services/license"

	"github.com/gosnmp/gosnmp"
)

// 常用 OID
var (
	OIDSysDescr    = ".1.3.6.1.2.1.1.1.0"
	OIDSysName     = ".1.3.6.1.2.1.1.5.0"
	OIDSysUpTime   = ".1.3.6.1.2.1.1.3.0"
	OIDSysContact  = ".1.3.6.1.2.1.1.4.0"
	OIDSysLocation = ".1.3.6.1.2.1.1.6.0"
	OIDSysObjectID = ".1.3.6.1.2.1.1.2.0"

	// Interface OIDs
	OIDIfNumber      = ".1.3.6.1.2.1.2.1.0"
	OIDIfIndex       = ".1.3.6.1.2.1.2.2.1.1"
	OIDIfDescr       = ".1.3.6.1.2.1.2.2.1.2"
	OIDIfType        = ".1.3.6.1.2.1.2.2.1.3"
	OIDIfMtu         = ".1.3.6.1.2.1.2.2.1.4"
	OIDIfSpeed       = ".1.3.6.1.2.1.2.2.1.5"
	OIDIfPhysAddr    = ".1.3.6.1.2.1.2.2.1.6"
	OIDIfAdminStatus = ".1.3.6.1.2.1.2.2.1.7"
	OIDIfOperStatus  = ".1.3.6.1.2.1.2.2.1.8"
	OIDIfInOctets    = ".1.3.6.1.2.1.2.2.1.10"
	OIDIfInErrors    = ".1.3.6.1.2.1.2.2.1.14"
	OIDIfOutOctets   = ".1.3.6.1.2.1.2.2.1.16"
	OIDIfOutErrors   = ".1.3.6.1.2.1.2.2.1.20"
	OIDIfName        = ".1.3.6.1.2.1.31.1.1.1.1"  // ifXTable
	OIDIfAlias       = ".1.3.6.1.2.1.31.1.1.1.18" // ifAlias
	OIDIfHighSpeed   = ".1.3.6.1.2.1.31.1.1.1.15" // 高速�??�速度 (Mbps)
	OIDIfHCInOctets  = ".1.3.6.1.2.1.31.1.1.1.6"  // 64-bit counters
	OIDIfHCOutOctets = ".1.3.6.1.2.1.31.1.1.1.10" // 64-bit counters

	// CPU & Memory
	OIDHrProcessorLoad = ".1.3.6.1.2.1.25.3.3.1.2"
	OIDHrStorageUsed   = ".1.3.6.1.2.1.25.2.3.1.6"
	OIDHrStorageSize   = ".1.3.6.1.2.1.25.2.3.1.5"

	// LLDP MIB OIDs
	OIDLldpRemChassisId = ".1.0.8802.1.1.2.1.4.1.1.5" // lldpRemChassisId
	OIDLldpRemPortId    = ".1.0.8802.1.1.2.1.4.1.1.7" // lldpRemPortId
	OIDLldpRemSysName   = ".1.0.8802.1.1.2.1.4.1.1.9" // lldpRemSysName
	OIDLldpRemManAddr   = ".1.0.8802.1.1.2.1.4.2.1.4" // lldpRemManAddrIfSubtype (for IP address)
	OIDLldpLocPortId    = ".1.0.8802.1.1.2.1.3.7.1.3" // lldpLocPortId

	// Disk Storage OIDs (HOST-RESOURCES-MIB)
	OIDHrStorageType  = ".1.3.6.1.2.1.25.2.3.1.2"
	OIDHrStorageUnits = ".1.3.6.1.2.1.25.2.3.1.4"
	// OIDHrStorageSize and OIDHrStorageUsed are already defined above

	// Storage Types
	HrStorageFixedDisk = ".1.3.6.1.2.1.25.2.1.4"
	HrStorageRam       = ".1.3.6.1.2.1.25.2.1.2"
	// ARP & FDB OIDs
	OIDIpNetToMediaPhysAddress    = ".1.3.6.1.2.1.3.1.1.2"    // Old ARP Table MAC
	OIDIpNetToPhysicalPhysAddress = ".1.3.6.1.2.1.4.22.1.2"   // IPv4 ARP Table MAC
	OIDDot1dTpFdbAddress          = ".1.3.6.1.2.1.17.4.3.1.1" // Bridge FDB MAC
	OIDDot1dTpFdbPort             = ".1.3.6.1.2.1.17.4.3.1.2" // Bridge FDB Port Index
	OIDDot1dBasePortIfIndex       = ".1.3.6.1.2.1.17.1.4.1.2" // Mapping bridge port to ifIndex

	// ?�?� EnGenius (Senao International, Enterprise .1.3.6.1.4.1.14125) ?�?�?�?�?�?�?�?�?�?�
	// EnGenius AP/Switch firmware is Linux-based; CPU/MEM fall back to UCD-SNMP-MIB.
	// These private OIDs are used for supplemental info on supported models.
	OIDEngeniusModelName   = ".1.3.6.1.4.1.14125.2.1.1.5"   // AP/Switch model name
	OIDEngeniusSSID        = ".1.3.6.1.4.1.14125.3.2.1.1.2" // SSID name (AP)
	OIDEngeniusChannel     = ".1.3.6.1.4.1.14125.3.2.1.1.3" // Wireless channel (AP)
	OIDEngeniusTxPower     = ".1.3.6.1.4.1.14125.3.2.1.1.5" // Tx power dBm (AP)
	OIDEngeniusSignal      = ".1.3.6.1.4.1.14125.3.1.1.1.7" // Signal strength (AP client)

	// ?�?� IEEE 802.3af/at PoE MIB (POWER-ETHERNET-MIB, RFC 3621) ?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�
	// Supported by: Edgecore ECS series, Cisco, HP/Aruba, and most PoE switches.
	OIDPethPortDetectionStatus = ".1.3.6.1.2.1.105.1.1.1.6"  // pethPsePortDetectionStatus (1=off,3=deliv)
	OIDPethPortPowerConsumption = ".1.3.6.1.2.1.105.1.1.1.12" // pethPsePortPowerClassifications / actual mW
	OIDPethMainPseConsumption  = ".1.3.6.1.2.1.105.1.3.1.4"  // pethMainPseConsumptionPower (mW, total)
	OIDPethMainPseCapacity     = ".1.3.6.1.2.1.105.1.3.1.2"  // pethMainPsePower (mW, max budget)

	// ?�?� Hikvision (Enterprise .1.3.6.1.4.1.39165) ?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�
	OIDHikvisionCPU          = ".1.3.6.1.4.1.39165.1.7.0"  // CPU usage %
	OIDHikvisionMemTotal     = ".1.3.6.1.4.1.39165.1.10.0" // Total memory (KB)
	OIDHikvisionMemUsed      = ".1.3.6.1.4.1.39165.1.11.0" // Memory used (KB)
	OIDHikvisionDiskSize     = ".1.3.6.1.4.1.39165.1.8.0"  // Disk size (MB)
	OIDHikvisionDiskUsage    = ".1.3.6.1.4.1.39165.1.9.0"  // Disk usage %
	OIDHikvisionDeviceType   = ".1.3.6.1.4.1.39165.1.1.0"  // Device type string
	OIDHikvisionFirmware     = ".1.3.6.1.4.1.39165.1.3.0"  // Firmware version
	OIDHikvisionVideoInputs  = ".1.3.6.1.4.1.39165.1.20.0" // Video input channels
	OIDHikvisionEncodeStatus = ".1.3.6.1.4.1.39165.1.21.0" // Video encode status

	// ?�?� Dahua (Enterprise .1.3.6.1.4.1.1004849) ?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�?�
	OIDDahuaCPU          = ".1.3.6.1.4.1.1004849.2.1.3"       // CPU usage %
	OIDDahuaSoftVersion  = ".1.3.6.1.4.1.1004849.2.1.1.1"     // Software version
	OIDDahuaHardVersion  = ".1.3.6.1.4.1.1004849.2.1.1.2"     // Hardware version
	OIDDahuaDeviceStatus = ".1.3.6.1.4.1.1004849.2.1.2.8"     // Device status
	OIDDahuaChannelCount = ".1.3.6.1.4.1.1004849.2.1.2.1"     // Video channel count
	OIDDahuaSerialNo     = ".1.3.6.1.4.1.1004849.2.1.2.4"     // Serial number
	OIDDahuaStreamFPS    = ".1.3.6.1.4.1.1004849.2.3.1.1.1.1.3" // Main stream FPS
)

type PollDeviceConfig struct {
	ID              int
	IPAddress       string
	Community       string
	Version         int
	DeviceType      string
	V3SecurityName  string
	V3SecurityLevel string
	V3AuthProtocol  string
	V3AuthPassword  string
	V3PrivProtocol  string
	V3PrivPassword  string
	V3ContextName   string
}

// bwAlertKey = "deviceID-ifIndex"，記錄上次告警時間（in-memory 冷卻，5分鐘）
var (
	bwAlertMu      sync.Mutex
	bwAlertLastAt  = map[string]time.Time{}
)

type Collector struct {
	config      *config.Config
	db          *sql.DB
	community   string
	timeout     int
	retries     int
	workerCount int
	dbWorker    *dbworker.Worker
}

func NewCollector(cfg *config.Config, db *sql.DB, worker *dbworker.Worker, community string, timeout, retries int) *Collector {
	return &Collector{
		config:      cfg,
		db:          db,
		dbWorker:    worker,
		community:   community,
		timeout:     timeout,
		retries:     retries,
		workerCount: 5,
	}
}

// PollAllDevices 輪詢?�?�設??
func (c *Collector) PollAllDevices() {
	maxDevices := license.GetMaxDevices(c.db, c.config)

	// ?�輪詢�? SNMP community ?�設?? 且�??�在?��?範�???(id LIMIT ?)
	query := fmt.Sprintf(`
		SELECT id, ip_address, snmp_community, snmp_version, device_type,
		       snmpv3_security_name, snmpv3_security_level, snmpv3_auth_protocol, snmpv3_auth_password,
		       snmpv3_priv_protocol, snmpv3_priv_password, snmpv3_context_name
		FROM devices 
		WHERE (snmp_community != '' OR snmp_version = 3) AND id IN (SELECT id FROM devices ORDER BY id ASC LIMIT %d)
	`, maxDevices)

	rows, err := c.db.Query(query)
	if err != nil {
		log.Printf("Error fetching devices: %v", err)
		return
	}
	defer rows.Close()

	var devices []PollDeviceConfig
	for rows.Next() {
		var d PollDeviceConfig
		if err := rows.Scan(&d.ID, &d.IPAddress, &d.Community, &d.Version, &d.DeviceType,
			&d.V3SecurityName, &d.V3SecurityLevel, &d.V3AuthProtocol, &d.V3AuthPassword,
			&d.V3PrivProtocol, &d.V3PrivPassword, &d.V3ContextName); err == nil {
			devices = append(devices, d)
		}
	}

	// ?�於?��? IP -> MAC ?��? (�?Switch/Router ?��?)
	ipToMacMap := &sync.Map{}

	// ?�於?��? SwitchID -> [MAC -> ifIndex] ?��?
	switchFdbMap := &sync.Map{}

	// 使用 worker pool
	jobs := make(chan PollDeviceConfig, len(devices))
	var wg sync.WaitGroup

	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range jobs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("SNMP Worker panic polling %s: %v", d.IPAddress, r)
						}
					}()
					c.PollDevice(d, ipToMacMap, switchFdbMap)
				}()
			}
		}()
	}

	for _, d := range devices {
		jobs <- d
	}
	close(jobs)
	wg.Wait()

	// 1. ?�試??ping-only 設�??�步 MAC ?��?
	c.SyncPingOnlyMacs(ipToMacMap)

	// 2. ?�試?�步 FDB ?�樸 (Switch 下�?設�?)
	c.SyncFdbTopology(switchFdbMap)

	log.Printf("SNMP poll completed for %d devices", len(devices))
}

// PollDevice 輪詢?��?設�?
func (c *Collector) PollDevice(d PollDeviceConfig, ipToMacMap *sync.Map, switchFdbMap *sync.Map) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[SNMP] Panic polling %s: %v", d.IPAddress, r)
		}
	}()
	snmpVersion := gosnmp.Version2c
	if d.Version == 1 {
		snmpVersion = gosnmp.Version1
	} else if d.Version == 3 {
		snmpVersion = gosnmp.Version3
	}

	params := &gosnmp.GoSNMP{
		Target:    d.IPAddress,
		Port:      161,
		Community: d.Community,
		Version:   snmpVersion,
		Timeout:   time.Duration(c.timeout) * time.Second,
		Retries:   c.retries,
	}

	if d.Version == 3 {
		params.SecurityModel = gosnmp.UserSecurityModel
		params.MsgFlags = gosnmp.NoAuthNoPriv
		if d.V3SecurityLevel == "authNoPriv" {
			params.MsgFlags = gosnmp.AuthNoPriv
		} else if d.V3SecurityLevel == "authPriv" {
			params.MsgFlags = gosnmp.AuthPriv
		}

		params.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: d.V3SecurityName,
		}

		if d.V3SecurityLevel != "noAuthNoPriv" {
			params.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationPassphrase = d.V3AuthPassword
			if strings.ToUpper(d.V3AuthProtocol) == "SHA" {
				params.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationProtocol = gosnmp.SHA
			} else {
				params.SecurityParameters.(*gosnmp.UsmSecurityParameters).AuthenticationProtocol = gosnmp.MD5
			}

			if d.V3SecurityLevel == "authPriv" {
				params.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyPassphrase = d.V3PrivPassword
				if strings.ToUpper(d.V3PrivProtocol) == "AES" {
					params.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyProtocol = gosnmp.AES
				} else {
					params.SecurityParameters.(*gosnmp.UsmSecurityParameters).PrivacyProtocol = gosnmp.DES
				}
			}
		}
		params.ContextName = d.V3ContextName
	}

	err := params.Connect()
	if err != nil {
		// Pinger handles availability, don't mark offline here
		// c.updateDeviceStatus(d.ID, false)
		c.logEvent(d.ID, "snmp_error", "warning", "Failed to connect: "+err.Error())
		return
	}
	defer params.Conn.Close()

	// ?��?系統資�? (?��? OIDIfPhysAddr.1 以獲?�基�?MAC)
	// 常�??��? MAC OID: .1.3.6.1.2.1.2.2.1.6.1 (介面 1 MAC)
	result, err := params.Get([]string{OIDSysDescr, OIDSysName, OIDSysUpTime, OIDSysLocation, OIDIfPhysAddr + ".1"})
	if err != nil {
		// c.updateDeviceStatus(deviceID, false)
		return
	}

	// ?�新設�??�?�為上�? (交由 Pinger ?��?，此?�忽??
	// c.updateDeviceStatus(deviceID, true)

	// �??系統資�?
	var sysDescr, sysName, sysLocation, baseMac string
	var sysUpTime int64
	for _, variable := range result.Variables {
		switch variable.Name {
		case OIDSysDescr:
			if bytes, ok := variable.Value.([]byte); ok {
				sysDescr = string(bytes)
			}
		case OIDSysName:
			if bytes, ok := variable.Value.([]byte); ok {
				sysName = string(bytes)
			}
		case OIDSysUpTime:
			sysUpTime = gosnmp.ToBigInt(variable.Value).Int64()
		case OIDSysLocation:
			if bytes, ok := variable.Value.([]byte); ok {
				sysLocation = string(bytes)
			}
		case OIDIfPhysAddr + ".1":
			if bytes, ok := variable.Value.([]byte); ok && len(bytes) == 6 {
				// 驗�??�否?�全 0 (?��? MAC)
				isZero := true
				for _, b := range bytes {
					if b != 0 {
						isZero = false
						break
					}
				}
				if !isZero {
					baseMac = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
						bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])
				}
			}
		}
	}

	// 如�? .1 沒�??��??��? MAC，�??�試 Walk ?�第一?��??��?
	if baseMac == "" {
		params.Walk(OIDIfPhysAddr, func(pdu gosnmp.SnmpPDU) error {
			if bytes, ok := pdu.Value.([]byte); ok && len(bytes) == 6 {
				isZero := true
				for _, b := range bytes {
					if b != 0 {
						isZero = false
						break
					}
				}
				if !isZero {
					baseMac = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
						bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])
					return fmt.Errorf("found") // Stop walking
				}
			}
			return nil
		})
	}

	// ?�新設�?資�? (?�含 sysName)
	vendor, model := parseVendorModel(sysDescr)
	uptimeStr := formatUptime(sysUpTime)

	// 檢測設�?類�?
	detectedType := detectDeviceType(sysDescr, vendor)

	// Update device info (Lock required -> No, serialized by dbWorker)
	c.dbWorker.Push(func(db *sql.DB) error {
		_, err := dbutils.ExecWithRetry(db, `
			UPDATE devices SET 
				vendor = COALESCE(NULLIF(?, ''), vendor),
				model = COALESCE(NULLIF(?, ''), model),
				firmware = COALESCE(NULLIF(?, ''), firmware),
				sys_name = ?,
				mac_address = COALESCE(NULLIF(?, ''), mac_address),
				sys_uptime = ?,
				sys_location = ?,
				device_type = CASE WHEN device_type = 'unknown' OR device_type = 'other' THEN ? ELSE device_type END
			WHERE id = ?
		`, vendor, model, sysDescr, sysName, baseMac, uptimeStr, sysLocation, detectedType, d.ID)
		return err
	})

	// ?��??�?��??��?�?
	c.collectAllInterfaces(params, d.ID)

	// ?��??�能?��?
	c.collectMetrics(params, d.ID)

	// 如�??�交?�器?�路?�器，收??ARP 表以建聯 IP->MAC ?��?
	if d.DeviceType == "switch" || d.DeviceType == "router" || d.DeviceType == "firewall" {
		c.collectArpTable(params, ipToMacMap)
	}

	// 如�??�交?�器，收??FDB 表以建聯 MAC->Port ?��?
	if d.DeviceType == "switch" {
		c.collectFdbTable(params, d.ID, switchFdbMap)
	}
}

// collectAllInterfaces 完整?��??�?��???
func (c *Collector) collectAllInterfaces(params *gosnmp.GoSNMP, deviceID int) {
	interfaces := make(map[int]map[string]interface{})

	// 1. Walk ifDescr (?�本資�?, RFC1213 規�?必�?)
	params.Walk(OIDIfDescr, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfDescr)
		if ifIndex == 0 {
			return nil
		}
		if _, ok := interfaces[ifIndex]; !ok {
			interfaces[ifIndex] = make(map[string]interface{})
			interfaces[ifIndex]["if_index"] = ifIndex
		}
		if bytes, ok := pdu.Value.([]byte); ok {
			val := string(bytes)
			interfaces[ifIndex]["if_desc"] = val
			interfaces[ifIndex]["if_name"] = val // ?�設 if_name 也�???if_desc
		}
		return nil
	})

	// 2. Walk ifName (ifXTable, �?ifDescr ?�簡�? �? Gi1/0/1)
	params.Walk(OIDIfName, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfName)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		if bytes, ok := pdu.Value.([]byte); ok {
			interfaces[ifIndex]["if_name"] = string(bytes)
		}
		return nil
	})

	// ?�收??ifSpeed (32-bit, ?��?.3Gbps) 作為?��???
	params.Walk(OIDIfSpeed, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfSpeed)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		speedBps := gosnmp.ToBigInt(pdu.Value).Int64()
		if speedBps > 0 {
			interfaces[ifIndex]["if_speed"] = speedBps
		}
		return nil
	})

	// ?�用 ifHighSpeed (Mbps) 覆�?,?�援 >4Gbps ?��???
	params.Walk(OIDIfHighSpeed, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfHighSpeed)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		speedMbps := gosnmp.ToBigInt(pdu.Value).Int64()
		// ?��???ifHighSpeed ?�值�?不為0?��?覆�?
		if speedMbps > 0 {
			interfaces[ifIndex]["if_speed"] = speedMbps * 1000000 // Mbps ??bps
		}
		return nil
	})

	// 修正?�擬介面?�速度 - �?2-bit上�?(4.3Gbps)?�為2.5Gbps
	for ifIndex, ifData := range interfaces {
		if speed, ok := ifData["if_speed"].(int64); ok && speed == 4294967295 {
			// 檢查?�否?��??��???(vmbr, tap, veth, docker, br-, lo�?
			ifName := ""
			if name, ok := ifData["if_name"].(string); ok {
				ifName = name
			} else if desc, ok := ifData["if_desc"].(string); ok {
				ifName = desc
			}

			if isVirtualInterface(ifName) {
				interfaces[ifIndex]["if_speed"] = 2500000000 // 2.5 Gbps
			}
		}
	}

	// Walk ifAdminStatus
	params.Walk(OIDIfAdminStatus, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfAdminStatus)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["if_admin_status"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	// Walk ifOperStatus
	params.Walk(OIDIfOperStatus, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfOperStatus)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["if_status"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	// Walk ifPhysAddr (MAC)
	params.Walk(OIDIfPhysAddr, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfPhysAddr)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		if bytes, ok := pdu.Value.([]byte); ok && len(bytes) == 6 {
			mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])
			interfaces[ifIndex]["if_mac"] = mac
		}
		return nil
	})

	// Walk 64-bit counters (ifHCInOctets, ifHCOutOctets)
	params.Walk(OIDIfHCInOctets, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfHCInOctets)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["in_octets"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	params.Walk(OIDIfHCOutOctets, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfHCOutOctets)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["out_octets"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	// 如�? HC counters 不支?��?使用 32-bit counters
	params.Walk(OIDIfInOctets, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfInOctets)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		if _, ok := interfaces[ifIndex]["in_octets"]; !ok {
			interfaces[ifIndex]["in_octets"] = gosnmp.ToBigInt(pdu.Value).Int64()
		}
		return nil
	})

	params.Walk(OIDIfOutOctets, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfOutOctets)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		if _, ok := interfaces[ifIndex]["out_octets"]; !ok {
			interfaces[ifIndex]["out_octets"] = gosnmp.ToBigInt(pdu.Value).Int64()
		}
		return nil
	})

	// Walk ifInErrors, ifOutErrors
	params.Walk(OIDIfInErrors, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfInErrors)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["in_errors"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	params.Walk(OIDIfOutErrors, func(pdu gosnmp.SnmpPDU) error {
		ifIndex := extractIndex(pdu.Name, OIDIfOutErrors)
		if ifIndex == 0 || interfaces[ifIndex] == nil {
			return nil
		}
		interfaces[ifIndex]["out_errors"] = gosnmp.ToBigInt(pdu.Value).Int64()
		return nil
	})

	// ?��?介面資�??��??�庫
	// 使用 dbWorker ?��?序�??�寫??
	c.dbWorker.Push(func(db *sql.DB) error {
		return dbutils.TxWithRetry(db, func(tx *sql.Tx) error {
			for ifIndex, ifData := range interfaces {
				var existingID int
				var prevInOctets, prevOutOctets int64
				var prevUpdatedAt time.Time

				// ?�試?�詢?��??��?以�?算帶�?
				// Use QueryRow on tx, not c.db
				err := tx.QueryRow(`
			SELECT id, in_octets, out_octets, updated_at
			FROM device_interfaces
			WHERE device_id = ? AND if_index = ?
		`, deviceID, ifIndex).Scan(&existingID, &prevInOctets, &prevOutOctets, &prevUpdatedAt)

				ifName := getStringValue(ifData, "if_name")
				ifDesc := getStringValue(ifData, "if_desc")
				ifSpeed := getInt64Value(ifData, "if_speed")
				ifMac := getStringValue(ifData, "if_mac")
				ifStatus := getInt64Value(ifData, "if_status")
				ifAdminStatus := getInt64Value(ifData, "if_admin_status")
				inOctets := getInt64Value(ifData, "in_octets")
				outOctets := getInt64Value(ifData, "out_octets")
				inErrors := getInt64Value(ifData, "in_errors")
				outErrors := getInt64Value(ifData, "out_errors")

				// 修正?�擬介面?�度: ??G/10G?�都設為2.5Gbps
				if ifSpeed > 1000000000 && ifSpeed != 10000000000 {
					ifSpeed = 2500000000 // 2.5 Gbps
				}

				// 計�??��?帶寬 (Bytes/sec)
				var bandwidthIn, bandwidthOut int64 = 0, 0
				if err == nil && !prevUpdatedAt.IsZero() {
					// Ensure we compare apples to apples (UTC vs UTC or derived)
					// SQLite CURRENT_TIMESTAMP is UTC. prevUpdatedAt scanned should be UTC.
					// time.Since uses time.Now() (Monotonic).
					// We should force time.Now().UTC() to align with DB if needed, but time.Sub handles logic.
					// Safest: parsed time vs time.Now().UTC()
					now := time.Now().UTC()
					// If prevUpdatedAt is not in UTC, force it (assuming it was stored as UTC)
					if prevUpdatedAt.Location() != time.UTC {
						prevUpdatedAt = prevUpdatedAt.In(time.UTC)
					}

					deltaTime := now.Sub(prevUpdatedAt).Seconds()

					if deltaTime > 0 && deltaTime < 600 {
						deltaIn := inOctets - prevInOctets
						deltaOut := outOctets - prevOutOctets

						if deltaIn < 0 {
							deltaIn += 4294967296
						}
						if deltaOut < 0 {
							deltaOut += 4294967296
						}

						if deltaIn >= 0 && deltaOut >= 0 {
							bandwidthIn = int64(float64(deltaIn) / deltaTime)
							bandwidthOut = int64(float64(deltaOut) / deltaTime)
						}
					}
				}

				// 檢查?�否已�???(使用 device_id + if_index)
				// 注�?: 上面??SELECT ?�於計�?，這裡?�們�?次檢?�主要為了確定是 INSERT ?�是 UPDATE (?�然?�輯?��??��?，�??��??�中?��??��?)
				// ?��?: 上面??err ?�為 nil ?�表示�??��?sql.ErrNoRows ?��?存在
				if err == sql.ErrNoRows {
					// ?��?
					_, execErr := tx.Exec(`
				INSERT INTO device_interfaces (device_id, if_index, if_name, if_desc, if_speed, if_mac, if_status, if_admin_status, in_octets, out_octets, in_errors, out_errors, bandwidth_in, bandwidth_out)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, deviceID, ifIndex, ifName, ifDesc, ifSpeed, ifMac, ifStatus, ifAdminStatus, inOctets, outOctets, inErrors, outErrors, bandwidthIn, bandwidthOut)
					if execErr != nil {
						log.Printf("Error inserting interface %d for device %d: %v", ifIndex, deviceID, execErr)
					}
				} else if err == nil {
					// ?�新
					_, execErr := tx.Exec(`
				UPDATE device_interfaces
				SET if_name = ?, if_desc = ?, if_speed = ?, if_mac = ?, if_status = ?, if_admin_status = ?,
					in_octets = ?, out_octets = ?, in_errors = ?, out_errors = ?, bandwidth_in = ?, bandwidth_out = ?, updated_at = CURRENT_TIMESTAMP
				WHERE id = ?
			`, ifName, ifDesc, ifSpeed, ifMac, ifStatus, ifAdminStatus, inOctets, outOctets, inErrors, outErrors, bandwidthIn, bandwidthOut, existingID)
					if execErr != nil {
						log.Printf("Error updating interface %d for device %d: %v", ifIndex, deviceID, execErr)
					}
				} else {
					log.Printf("Error checking existing interface %d for device %d: %v", ifIndex, deviceID, err)
				}

				// 頻寬超限告警：若 bandwidth_in 或 bandwidth_out（bytes/s）超過 if_speed（bps）的 80%，寫 notifications
				if ifSpeed > 0 && (bandwidthIn > 0 || bandwidthOut > 0) {
					thresholdBps := ifSpeed * 8 / 10 // 80% of link speed in bps
					actualInBps := bandwidthIn * 8
					actualOutBps := bandwidthOut * 8
					if actualInBps > thresholdBps || actualOutBps > thresholdBps {
						alertKey := fmt.Sprintf("%d-%d", deviceID, ifIndex)
						bwAlertMu.Lock()
						lastAlert := bwAlertLastAt[alertKey]
						bwAlertMu.Unlock()
						if time.Since(lastAlert) > 5*time.Minute {
							bwAlertMu.Lock()
							bwAlertLastAt[alertKey] = time.Now()
							bwAlertMu.Unlock()

							// 取設備名稱
							var devName string
							c.db.QueryRow(`SELECT COALESCE(name, ip_address) FROM devices WHERE id = ?`, deviceID).Scan(&devName)
							usageMbps := (actualInBps + actualOutBps) / 1000000
							limitMbps := ifSpeed / 1000000
							title := fmt.Sprintf("設備頻寬超限：%s", devName)
							msg := fmt.Sprintf("設備 %s 介面 %s (if_index=%d) 流量 %d Mbps 超過連線速度 %d Mbps 的 80%%",
								devName, ifName, ifIndex, usageMbps, limitMbps)
							c.db.Exec(`INSERT INTO notifications (severity, title, message) VALUES (?, ?, ?)`,
								"warning", title, msg)
							log.Printf("[BW-ALERT] %s", msg)
						}
					}
				}
			}

			// ?��?修正?��?準速度??.5Gbps
			_, err := tx.Exec(`
		UPDATE device_interfaces 
		SET if_speed = 2500000000 
		WHERE device_id = ? 
		  AND if_speed NOT IN (10000000, 1000000000, 10000000000) 
		  AND if_speed > 1000000000
	`, deviceID)
			if err != nil {
				log.Printf("Error auto-correcting interface speeds for device %d: %v", deviceID, err)
			}

			return nil
		})
	})
}

func (c *Collector) collectMetrics(params *gosnmp.GoSNMP, deviceID int) {
	var cpuUsage float64
	var memUsage float64

	// ?�試?��? CPU 使用??
	// 1. HOST-RESOURCES-MIB (hrProcessorLoad)
	params.Walk(OIDHrProcessorLoad, func(pdu gosnmp.SnmpPDU) error {
		if val := gosnmp.ToBigInt(pdu.Value).Int64(); val > 0 {
			if cpuUsage == 0 || float64(val) > cpuUsage {
				cpuUsage = float64(val)
			}
		}
		return nil
	})

	// 2. ?��??��??��??��?
	if cpuUsage == 0 {
		cpuOids := []string{
			".1.3.6.1.4.1.9.9.109.1.1.1.1.7.1",      // Cisco 5min
			".1.3.6.1.4.1.2011.5.25.31.1.1.1.1.5.0", // Huawei
			".1.3.6.1.4.1.11863.6.1.1.2.1.1.1.0",    // TP-Link
			".1.3.6.1.4.1.2021.11.11.0",              // Net-SNMP / EnGenius (Linux, Idle) - ?��?100-val
			".1.3.6.1.4.1.25506.2.6.1.1.1.1.6.1",    // H3C
			OIDHikvisionCPU,                           // Hikvision IP Camera
			OIDDahuaCPU,                               // Dahua IP Camera
		}
		for _, oid := range cpuOids {
			res, err := params.Get([]string{oid})
			if err == nil && len(res.Variables) > 0 && res.Variables[0].Type != gosnmp.NoSuchObject {
				val := gosnmp.ToBigInt(res.Variables[0].Value).Int64()
				if oid == ".1.3.6.1.4.1.2021.11.11.0" {
					if val > 0 {
						cpuUsage = 100 - float64(val)
					}
				} else {
					cpuUsage = float64(val)
				}
				if cpuUsage > 0 {
					break
				}
			}
		}
	}

	// 3. ?��?記憶體使?��? - 修正 HOST-RESOURCES-MIB ?�輯 (?��?�?RAM 類�?)
	// hrStorageRam OID: .1.3.6.1.2.1.25.2.1.2
	var memTotal, memUsed int64
	var ramSize, ramUsed int64
	params.Walk(".1.3.6.1.2.1.25.2.3.1.2", func(pdu gosnmp.SnmpPDU) error {
		storageType := pdu.Value.(string)                           // OID string
		if strings.Contains(storageType, ".1.3.6.1.2.1.25.2.1.2") { // hrStorageRam
			index := extractIndex(pdu.Name, ".1.3.6.1.2.1.25.2.3.1.2")

			// ?��?�?index ??Units, Size, Used
			unitsRes, _ := params.Get([]string{fmt.Sprintf(".1.3.6.1.2.1.25.2.3.1.4.%d", index)})
			sizeRes, _ := params.Get([]string{fmt.Sprintf(".1.3.6.1.2.1.25.2.3.1.5.%d", index)})
			usedRes, _ := params.Get([]string{fmt.Sprintf(".1.3.6.1.2.1.25.2.3.1.6.%d", index)})

			var units int64
			if len(unitsRes.Variables) > 0 {
				units = gosnmp.ToBigInt(unitsRes.Variables[0].Value).Int64()
			}
			if len(sizeRes.Variables) > 0 {
				ramSize = gosnmp.ToBigInt(sizeRes.Variables[0].Value).Int64()
			}
			if len(usedRes.Variables) > 0 {
				ramUsed = gosnmp.ToBigInt(usedRes.Variables[0].Value).Int64()
			}

			if ramSize > 0 && units > 0 {
				memTotal = ramSize * units
				memUsed = ramUsed * units
				memUsage = (float64(ramUsed) / float64(ramSize)) * 100
			}
		}
		return nil
	})

	// 4. ?��?記憶體�??�優??(??Hikvision / EnGenius Linux-based)
	if memUsage == 0 {
		// Hikvision: direct total/used in KB
		hikvisionMemTotalOID := OIDHikvisionMemTotal
		hikvisionMemUsedOID := OIDHikvisionMemUsed
		hikTotalRes, hikTotalErr := params.Get([]string{hikvisionMemTotalOID})
		hikUsedRes, hikUsedErr := params.Get([]string{hikvisionMemUsedOID})
		if hikTotalErr == nil && hikUsedErr == nil &&
			len(hikTotalRes.Variables) > 0 && len(hikUsedRes.Variables) > 0 &&
			hikTotalRes.Variables[0].Type != gosnmp.NoSuchObject &&
			hikUsedRes.Variables[0].Type != gosnmp.NoSuchObject {
			total := gosnmp.ToBigInt(hikTotalRes.Variables[0].Value).Int64()
			used := gosnmp.ToBigInt(hikUsedRes.Variables[0].Value).Int64()
			if total > 0 {
				memTotal = total * 1024 // KB ??bytes
				memUsed = used * 1024
				memUsage = float64(used) / float64(total) * 100
			}
		}
	}
	if memUsage == 0 {
		memOids := []string{
			".1.3.6.1.4.1.11863.6.1.1.2.1.1.2.0", // TP-Link (Percentage)
			".1.3.6.1.4.1.9.9.48.1.1.1.5.1",      // Cisco Used Memory Pool
			".1.3.6.1.4.1.2021.4.11.0",            // Net-SNMP / EnGenius MemFree (requires MemTotal)
		}
		for _, oid := range memOids {
			res, err := params.Get([]string{oid})
			if err == nil && len(res.Variables) > 0 && res.Variables[0].Type != gosnmp.NoSuchObject {
				val := gosnmp.ToBigInt(res.Variables[0].Value).Int64()
				if oid == ".1.3.6.1.4.1.9.9.48.1.1.1.5.1" {
					// Cisco logic: Used / (Used + Free)
					resFree, errFree := params.Get([]string{".1.3.6.1.4.1.9.9.48.1.1.1.6.1"})
					if errFree == nil && len(resFree.Variables) > 0 {
						free := gosnmp.ToBigInt(resFree.Variables[0].Value).Int64()
						if val+free > 0 {
							memUsage = (float64(val) / float64(val+free)) * 100
						}
					}
				} else if oid == ".1.3.6.1.4.1.2021.4.11.0" {
					// Net-SNMP: (Total - Free - Buffers - Cached) / Total
					// val �?MemFree (.1.3.6.1.4.1.2021.4.11.0)
					resTotal, _ := params.Get([]string{".1.3.6.1.4.1.2021.4.5.0"})
					resBuffers, _ := params.Get([]string{".1.3.6.1.4.1.2021.4.14.0"})
					resCached, _ := params.Get([]string{".1.3.6.1.4.1.2021.4.15.0"})

					var total, buffers, cached int64
					if len(resTotal.Variables) > 0 {
						total = gosnmp.ToBigInt(resTotal.Variables[0].Value).Int64()
					}
					if len(resBuffers.Variables) > 0 {
						buffers = gosnmp.ToBigInt(resBuffers.Variables[0].Value).Int64()
					}
					if len(resCached.Variables) > 0 {
						cached = gosnmp.ToBigInt(resCached.Variables[0].Value).Int64()
					}

					if total > 0 {
						// ??�� Free, Buffers, Cached ?�是?�正??Used
						actualUsed := total - val - buffers - cached
						if actualUsed < 0 {
							actualUsed = 0
						}
						memUsage = (float64(actualUsed) / float64(total)) * 100
					}
				} else {
					memUsage = float64(val)
				}
				if memUsage > 0 {
					break
				}
			}
		}
	}

	// 5. ?��?磁�?使用??(HOST-RESOURCES-MIB)
	var diskTotal, diskUsed uint64
	params.Walk(OIDHrStorageType, func(pdu gosnmp.SnmpPDU) error {
		storageType := pdu.Value.(string)
		if strings.Contains(storageType, HrStorageFixedDisk) {
			index := extractIndex(pdu.Name, OIDHrStorageType)

			resUnits, _ := params.Get([]string{fmt.Sprintf("%s.%d", OIDHrStorageUnits, index)})
			resSize, _ := params.Get([]string{fmt.Sprintf("%s.%d", OIDHrStorageSize, index)})
			resUsed, _ := params.Get([]string{fmt.Sprintf("%s.%d", OIDHrStorageUsed, index)})

			var units, size, used int64
			if len(resUnits.Variables) > 0 {
				units = gosnmp.ToBigInt(resUnits.Variables[0].Value).Int64()
			}
			if len(resSize.Variables) > 0 {
				size = gosnmp.ToBigInt(resSize.Variables[0].Value).Int64()
			}
			if len(resUsed.Variables) > 0 {
				used = gosnmp.ToBigInt(resUsed.Variables[0].Value).Int64()
			}

			if units > 0 && size > 0 {
				diskTotal += uint64(size * units)
				diskUsed += uint64(used * units)
			}
		}
		return nil
	})

	var diskUsage float64
	if diskTotal > 0 {
		diskUsage = (float64(diskUsed) / float64(diskTotal)) * 100
		if diskUsage > 100 {
			diskUsage = 100
		}
	}

	// ?�在?�數?��??�入
	if (cpuUsage > 0 && cpuUsage <= 100) || (memUsage > 0 && memUsage <= 100) || diskUsage > 0 {
		// 修正極端?��?
		if cpuUsage > 100 {
			cpuUsage = 100
		}
		if memUsage > 100 {
			memUsage = 100
		}

		c.dbWorker.Push(func(db *sql.DB) error {
			_, err := dbutils.ExecWithRetry(db, `
			INSERT INTO device_metrics (device_id, cpu_usage, memory_usage, disk_usage, mem_total, mem_used, disk_total, disk_used) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, deviceID, cpuUsage, memUsage, diskUsage, memTotal, memUsed, int64(diskTotal), int64(diskUsed))
			return err
		})
	}
}

// updateDeviceStatus ?�新設�??�??
func (c *Collector) updateDeviceStatus(deviceID int, isOnline bool) {
	c.dbWorker.Push(func(db *sql.DB) error {
		var wasOnline bool
		err := db.QueryRow("SELECT is_online FROM devices WHERE id = ?", deviceID).Scan(&wasOnline)
		// Ignore error (e.g. no rows)

		_, err = dbutils.ExecWithRetry(db, `
		UPDATE devices SET is_online = ?, last_seen = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?
	`, isOnline, deviceID)
		if err != nil {
			return err
		}

		if wasOnline != isOnline {
			status := "offline"
			severity := "warning"
			if isOnline {
				status = "online"
				severity = "info"
			}

			var evtMsg string
			if status == "online" {
				evtMsg = "設備已上線（SNMP 偵測）"
			} else {
				evtMsg = "設備已離線（SNMP 偵測）"
			}
			_, err = dbutils.ExecWithRetry(db, `
			INSERT INTO events (device_id, event_type, severity, message, created_at) VALUES (?, ?, ?, ?, datetime('now'))
		`, deviceID, "status_change", severity, evtMsg)
			return err
		}
		return nil
	})
}

// logEvent 記�?事件
func (c *Collector) logEvent(deviceID int, eventType, severity, message string) {
	c.dbWorker.Push(func(db *sql.DB) error {
		_, err := dbutils.ExecWithRetry(db, `
		INSERT INTO events (device_id, event_type, severity, message, created_at) VALUES (?, ?, ?, ?, datetime('now'))
	`, deviceID, eventType, severity, message)
		return err
	})
}

// Helper functions
func getStringValue(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt64Value(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		if i, ok := v.(int64); ok {
			return i
		}
	}
	return 0
}

// isVirtualInterface ?�斷?�否?��??�網路�???
func isVirtualInterface(ifName string) bool {
	if ifName == "" {
		return false
	}

	lowerName := strings.ToLower(ifName)

	// 常�??�擬介面?�綴
	virtualPrefixes := []string{
		"vmbr",   // Proxmox virtual bridge
		"tap",    // TAP device
		"veth",   // Virtual Ethernet
		"docker", // Docker
		"br-",    // Linux bridge
		"virbr",  // libvirt bridge
		"vnet",   // Virtual network
		"tun",    // TUN device
		"fwbr",   // Firewall bridge
		"fwpr",   // Firewall port
		"fwln",   // Firewall link
		"vlan",   // VLAN interface
	}

	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(lowerName, prefix) {
			return true
		}
	}

	// lo (loopback)
	if lowerName == "lo" || strings.HasPrefix(lowerName, "lo:") {
		return true
	}

	return false
}

func formatUptime(ticks int64) string {
	seconds := ticks / 100
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
}

func parseVendorModel(sysDescr string) (vendor, model string) {
	// Ordered list ??first match wins. More specific keywords must come before generic ones.
	// E.g. "ECS2100" before "ECS" before "EnGenius", "Accton" before generic "Linux".
	type entry struct {
		keyword string
		vendor  string
	}
	vendors := []entry{
		// Edgecore / Accton ??must be before EnGenius because "ECS" prefix is shared
		{"Edgecore", "Edgecore"},
		{"Accton", "Edgecore"},
		{"ECS2100", "Edgecore"},
		{"ECS4100", "Edgecore"},
		{"ECS4200", "Edgecore"},
		{"ECS4500", "Edgecore"},
		{"ECS", "Edgecore"}, // generic Edgecore switch prefix ??after specific models
		// Other network vendors
		{"Cisco", "Cisco"},
		{"Juniper", "Juniper"},
		{"Huawei", "Huawei"},
		{"Aruba", "HP/Aruba"},
		{"HP", "HP/Aruba"},
		{"Dell", "Dell"},
		{"Fortinet", "Fortinet"},
		{"Palo Alto", "Palo Alto"},
		{"Sophos", "Sophos"},
		{"MikroTik", "MikroTik"},
		{"Ubiquiti", "Ubiquiti"},
		{"TP-Link", "TP-Link"},
		{"Omada", "TP-Link"},
		{"JetStream", "TP-Link"},
		{"T1500", "TP-Link"},
		{"T1600", "TP-Link"},
		{"T2600", "TP-Link"},
		{"TL-SG", "TP-Link"},
		{"D-Link", "D-Link"},
		{"Netgear", "Netgear"},
		// EnGenius (Senao, enterprise OID 1.3.6.1.4.1.14125) ??AP/switch brand
		{"EnGenius", "EnGenius"},
		{"ECB", "EnGenius"},
		{"EWS", "EnGenius"},
		{"ENS", "EnGenius"},
		{"EAP", "EnGenius"},
		// OS / platform
		{"Windows", "Microsoft"},
		{"Linux", "Linux"},
		// IP Camera vendors
		{"Axis", "Axis"},
		{"Vivotek", "Vivotek"},
		{"Vport", "Vivotek"},
		{"QCTek", "QCTek"},
		{"QCTEK", "QCTek"},
		{"Dahua", "Dahua"},
		{"HiLookVision", "Hikvision"},
		{"Hikvision", "Hikvision"},
		{"Hanwha", "Hanwha"},
		{"Wisenet", "Hanwha"},
		{"Bosch", "Bosch"},
		{"Pelco", "Pelco"},
		{"Genetec", "Genetec"},
		{"Milestone", "Milestone"},
		{"Uniview", "Uniview"},
		{"UNV", "Uniview"},
		{"Reolink", "Reolink"},
		{"Amcrest", "Amcrest"},
	}

	descLower := strings.ToLower(sysDescr)
	for _, e := range vendors {
		if strings.Contains(descLower, strings.ToLower(e.keyword)) {
			model = extractModelFromDescr(sysDescr, e.keyword)
			return e.vendor, model
		}
	}
	return "Other", ""
}

// extractModelFromDescr extracts a model hint from sysDescr by finding the word
// immediately following the vendor keyword (best-effort).
func extractModelFromDescr(sysDescr, keyword string) string {
	lower := strings.ToLower(sysDescr)
	idx := strings.Index(lower, strings.ToLower(keyword))
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(sysDescr[idx+len(keyword):])
	fields := strings.Fields(rest)
	if len(fields) > 0 {
		// Return up to 2 tokens as model hint
		if len(fields) >= 2 {
			return fields[0] + " " + fields[1]
		}
		return fields[0]
	}
	return ""
}

// detectDeviceType ?��? sysDescr ??vendor ?�斷設�?類�?
func detectDeviceType(sysDescr, vendor string) string {
	descLower := strings.ToLower(sysDescr)
	vendorLower := strings.ToLower(vendor)

	// 路由?��???
	if strings.Contains(descLower, "router") ||
		strings.Contains(descLower, "routing") ||
		(strings.Contains(vendorLower, "cisco") && strings.Contains(descLower, "ios")) ||
		strings.Contains(descLower, "juniper") ||
		strings.Contains(descLower, "mikrotik") {
		return "router"
	}

	// 交�??��???
	if strings.Contains(descLower, "switch") ||
		strings.Contains(descLower, "switching") ||
		strings.Contains(descLower, "ethernet switch") ||
		strings.Contains(vendorLower, "tp-link") ||
		strings.Contains(vendorLower, "d-link") ||
		strings.Contains(vendorLower, "netgear") ||
		strings.Contains(vendorLower, "edgecore") ||
		strings.Contains(vendorLower, "aruba") ||
		strings.Contains(vendorLower, "juniper") ||
		strings.Contains(vendorLower, "dell") ||
		strings.Contains(vendorLower, "h3c") ||
		strings.Contains(vendorLower, "huawei") ||
		strings.Contains(vendorLower, "extreme") ||
		strings.Contains(vendorLower, "fortiswitch") ||
		strings.Contains(descLower, "catalyst") ||
		strings.Contains(descLower, "procurve") ||
		strings.Contains(descLower, "l2 switch") ||
		strings.Contains(descLower, "l3 switch") {
		return "switch"
	}

	// ?�火?��???
	if strings.Contains(descLower, "firewall") ||
		strings.Contains(descLower, "fortigate") ||
		strings.Contains(descLower, "palo alto") ||
		strings.Contains(descLower, "pfsense") ||
		strings.Contains(descLower, "sophos") ||
		strings.Contains(vendorLower, "fortinet") {
		return "firewall"
	}

	// 伺�??��???
	if strings.Contains(descLower, "linux") ||
		strings.Contains(descLower, "ubuntu") ||
		strings.Contains(descLower, "centos") ||
		strings.Contains(descLower, "debian") ||
		strings.Contains(descLower, "windows server") ||
		strings.Contains(descLower, "freebsd") ||
		strings.Contains(descLower, "red hat") {
		return "server"
	}

	// ?��?存�?�?(AP) 識別 ???�含 EnGenius AP 系�?
	if strings.Contains(descLower, "access point") ||
		strings.Contains(descLower, "wireless ap") ||
		strings.Contains(descLower, "wap") ||
		(strings.Contains(vendorLower, "edgecore") && strings.Contains(descLower, "ecw")) ||
		(strings.Contains(vendorLower, "tp-link") && strings.Contains(descLower, "eap")) ||
		strings.Contains(descLower, "aironet") ||
		strings.Contains(descLower, "unifi") ||
		// EnGenius AP 系�?: ECB (Enterprise Ceiling/Bridge), ENS (Outdoor), EAP
		strings.Contains(vendorLower, "engenius") ||
		strings.Contains(descLower, "engenius") ||
		(strings.Contains(descLower, "ecb") && strings.Contains(descLower, "wireless")) ||
		(strings.Contains(descLower, "ens") && strings.Contains(descLower, "wireless")) {
		// EnGenius ECS = switch series, ENS/ECB = AP series
		// If the descr also mentions "switch", classify as switch instead
		if strings.Contains(vendorLower, "engenius") || strings.Contains(descLower, "engenius") {
			if strings.Contains(descLower, "switch") || strings.Contains(descLower, "ecs") {
				return "switch"
			}
		}
		return "access_point"
	}

	// ?�影�?(IP Camera / CCTV) 識別 ???��?廠�?清單
	// QCTek platform detection: sysDescr usually contains "QCTek", "QCTEK", "QCT" or "Camera"
	if strings.Contains(descLower, "camera") ||
		strings.Contains(descLower, "ipcam") ||
		strings.Contains(descLower, "ip camera") ||
		strings.Contains(descLower, "network camera") ||
		strings.Contains(descLower, "video server") ||
		strings.Contains(descLower, "onvif") ||
		strings.Contains(descLower, "vport") ||
		strings.Contains(descLower, "dahua") ||
		strings.Contains(descLower, "hikvision") ||
		strings.Contains(descLower, "hilookvision") ||
		strings.Contains(descLower, "hanwha") ||
		strings.Contains(descLower, "wisenet") ||
		strings.Contains(descLower, "uniview") ||
		strings.Contains(descLower, "unv") ||
		strings.Contains(descLower, "reolink") ||
		strings.Contains(descLower, "amcrest") ||
		strings.Contains(vendorLower, "axis") ||
		strings.Contains(vendorLower, "vivotek") ||
		strings.Contains(vendorLower, "qctek") ||
		strings.Contains(descLower, "qctek") ||
		strings.Contains(descLower, "qcam") ||    // QCTek camera model prefix
		strings.Contains(vendorLower, "dahua") ||
		strings.Contains(vendorLower, "hikvision") ||
		strings.Contains(vendorLower, "hanwha") ||
		strings.Contains(vendorLower, "bosch") ||
		strings.Contains(vendorLower, "pelco") ||
		strings.Contains(vendorLower, "uniview") ||
		strings.Contains(vendorLower, "reolink") ||
		strings.Contains(vendorLower, "amcrest") {
		return "ipcam"
	}

	// ?��??�控?�器 / 影�???(Video Wall)
	if strings.Contains(descLower, "video wall") ||
		strings.Contains(descLower, "tv wall") ||
		strings.Contains(descLower, "wall controller") ||
		strings.Contains(descLower, "multiviewer") {
		return "video_wall"
	}

	return "other"
}

func extractIndex(fullOID, baseOID string) int {
	if len(fullOID) <= len(baseOID)+1 {
		return 0
	}
	suffix := fullOID[len(baseOID)+1:]
	var idx int
	for _, c := range suffix {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		} else {
			break
		}
	}
	return idx
}

// DiscoverLLDPTopology ?��? LLDP ?�索?�樸???
func (c *Collector) DiscoverLLDPTopology() {
	log.Println("Starting LLDP topology discovery...")

	// ?��??�?��? SNMP community ?�設??
	maxDevices := license.GetMaxDevices(c.db, c.config)
	query := fmt.Sprintf(`
		SELECT id, ip_address, snmp_community, snmp_version 
		FROM devices 
		WHERE snmp_community != '' AND snmp_community IS NOT NULL
		AND id IN (SELECT id FROM devices ORDER BY id ASC LIMIT %d)
	`, maxDevices)

	rows, err := c.db.Query(query)
	if err != nil {
		log.Printf("Error fetching devices for LLDP discovery: %v", err)
		return
	}
	defer rows.Close()

	type device struct {
		ID        int
		IPAddress string
		Community string
		Version   int
	}

	var devices []device
	for rows.Next() {
		var d device
		if err := rows.Scan(&d.ID, &d.IPAddress, &d.Community, &d.Version); err == nil {
			devices = append(devices, d)
		}
	}

	// 建�? IP ?�設??ID ?��?�?
	ipToDeviceID := make(map[string]int)
	for _, d := range devices {
		ipToDeviceID[d.IPAddress] = d.ID
	}

	// 建�? sysName ?�設??ID ?��?�?(?�含完整?�稱?�短?�稱)
	sysNameToDeviceID := make(map[string]int)
	sysNameRows, _ := c.db.Query("SELECT id, COALESCE(sys_name, '') FROM devices")
	if sysNameRows != nil {
		defer sysNameRows.Close()
		for sysNameRows.Next() {
			var id int
			var sysName string
			if err := sysNameRows.Scan(&id, &sysName); err == nil && sysName != "" {
				lowerName := strings.ToLower(sysName)
				sysNameToDeviceID[lowerName] = id

				// ?��?索�??��?�?(移除網�?後綴)
				parts := strings.Split(lowerName, ".")
				if len(parts) > 1 {
					sysNameToDeviceID[parts[0]] = id
				}
				log.Printf("LLDP Index: Device %d -> %s (and %s)", id, lowerName, parts[0])
			}
		}
	}

	discoveredLinks := make(map[string]bool) // ?�於?��?

	for _, d := range devices {
		c.discoverDeviceLLDP(d.ID, d.IPAddress, d.Community, d.Version, ipToDeviceID, sysNameToDeviceID, discoveredLinks)
	}

	log.Printf("LLDP discovery completed for %d devices. Created %d new links.", len(devices), len(discoveredLinks))
}

// discoverDeviceLLDP ?�索?��?設�???LLDP ?��?
func (c *Collector) discoverDeviceLLDP(deviceID int, ip, community string, version int, ipToDeviceID map[string]int, sysNameToDeviceID map[string]int, discoveredLinks map[string]bool) {
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
		log.Printf("LLDP: Failed to connect to %s: %v", ip, err)
		return
	}
	defer params.Conn.Close()

	// ?��? LLDP ?��?資�?
	type neighborInfo struct {
		sysName   string
		chassisID string
		portID    string
		manAddrs  []string
	}
	neighbors := make(map[int]*neighborInfo) // localIfIndex -> neighbor info

	// Walk lldpRemSysName
	err := params.Walk(OIDLldpRemSysName, func(pdu gosnmp.SnmpPDU) error {
		indices := extractLLDPIndices(pdu.Name, OIDLldpRemSysName)
		if len(indices) >= 2 {
			localPortNum := indices[1]
			if neighbors[localPortNum] == nil {
				neighbors[localPortNum] = &neighborInfo{}
			}
			if bytes, ok := pdu.Value.([]byte); ok {
				neighbors[localPortNum].sysName = string(bytes)
				log.Printf("LLDP DEBUG [%s]: Port %d -> sysName: %s", ip, localPortNum, string(bytes))
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("LLDP: Walk lldpRemSysName failed for %s: %v", ip, err)
	}

	// Walk lldpRemChassisId for devices that don't report sysName
	err = params.Walk(OIDLldpRemChassisId, func(pdu gosnmp.SnmpPDU) error {
		indices := extractLLDPIndices(pdu.Name, OIDLldpRemChassisId)
		if len(indices) >= 2 {
			localPortNum := indices[1]
			if neighbors[localPortNum] == nil {
				neighbors[localPortNum] = &neighborInfo{}
			}
			if bytes, ok := pdu.Value.([]byte); ok {
				// Format as MAC if 6 bytes
				if len(bytes) == 6 {
					neighbors[localPortNum].chassisID = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
						bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])
				} else {
					neighbors[localPortNum].chassisID = string(bytes)
				}
				log.Printf("LLDP DEBUG [%s]: Port %d -> chassisID: %s", ip, localPortNum, neighbors[localPortNum].chassisID)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("LLDP: Walk lldpRemChassisId failed for %s: %v", ip, err)
	}

	// Walk lldpRemPortId
	err = params.Walk(OIDLldpRemPortId, func(pdu gosnmp.SnmpPDU) error {
		indices := extractLLDPIndices(pdu.Name, OIDLldpRemPortId)
		if len(indices) >= 2 {
			localPortNum := indices[1]
			if neighbors[localPortNum] == nil {
				neighbors[localPortNum] = &neighborInfo{}
			}
			if bytes, ok := pdu.Value.([]byte); ok {
				neighbors[localPortNum].portID = string(bytes)
				log.Printf("LLDP DEBUG [%s]: Port %d -> portID: %s", ip, localPortNum, string(bytes))
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("LLDP: Walk lldpRemPortId failed for %s: %v", ip, err)
	}

	// Walk lldpRemManAddr to get management IP addresses
	// OID format: .1.0.8802.1.1.2.1.4.2.1.4.<timeMark>.<localPortNum>.<addrSubtype>.<addrLen>.<addr bytes>
	err = params.Walk(OIDLldpRemManAddr, func(pdu gosnmp.SnmpPDU) error {
		fullOID := pdu.Name
		// Parse management address from OID suffix
		// After base OID, format is: timeMark.localPortNum.remIndex.addrSubtype.addrLen.addr...
		suffix := fullOID[len(OIDLldpRemManAddr)+1:]
		parts := strings.Split(suffix, ".")
		if len(parts) >= 6 {
			localPortNum := 0
			fmt.Sscanf(parts[1], "%d", &localPortNum)

			// Check if this is IPv4 (addrSubtype=1, len=4)
			addrSubtype := 0
			addrLen := 0
			fmt.Sscanf(parts[3], "%d", &addrSubtype)
			fmt.Sscanf(parts[4], "%d", &addrLen)

			if addrSubtype == 1 && addrLen == 4 && len(parts) >= 9 {
				// IPv4 address
				ipAddr := fmt.Sprintf("%s.%s.%s.%s", parts[5], parts[6], parts[7], parts[8])
				if neighbors[localPortNum] == nil {
					neighbors[localPortNum] = &neighborInfo{}
				}
				neighbors[localPortNum].manAddrs = append(neighbors[localPortNum].manAddrs, ipAddr)
				log.Printf("LLDP DEBUG [%s]: Port %d -> managementIP: %s", ip, localPortNum, ipAddr)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("LLDP: Walk lldpRemManAddr failed for %s: %v", ip, err)
	}

	log.Printf("LLDP: Device %s (ID:%d) found %d LLDP neighbors", ip, deviceID, len(neighbors))

	// ?��??��?資�?建�??�樸???
	for localIfIndex, neighbor := range neighbors {
		if neighbor == nil {
			continue
		}

		var remoteDeviceID int
		var found bool

		// ?��? 1: ?��? sysName ?��?
		if neighbor.sysName != "" {
			lowerName := strings.ToLower(neighbor.sysName)
			remoteDeviceID, found = sysNameToDeviceID[lowerName]

			// 如�?完整?�稱沒找?��??�試?��?�?
			if !found {
				parts := strings.Split(lowerName, ".")
				if len(parts) > 1 {
					remoteDeviceID, found = sysNameToDeviceID[parts[0]]
					if found {
						log.Printf("LLDP: Matched neighbor by short sysName: %s -> deviceID %d", parts[0], remoteDeviceID)
					}
				}
			} else {
				log.Printf("LLDP: Matched neighbor by sysName: %s -> deviceID %d", neighbor.sysName, remoteDeviceID)
			}
		}

		// ?��? 2: ?��?管�? IP ?��?
		if !found && len(neighbor.manAddrs) > 0 {
			for _, maddr := range neighbor.manAddrs {
				if id, ok := ipToDeviceID[maddr]; ok {
					remoteDeviceID = id
					found = true
					log.Printf("LLDP: Matched neighbor by managementIP: %s -> deviceID %d", maddr, remoteDeviceID)
					break
				}
			}
		}

		if !found {
			log.Printf("LLDP: Unknown neighbor on device %s port %d (sysName=%s, chassisID=%s, manAddrs=%v)",
				ip, localIfIndex, neighbor.sysName, neighbor.chassisID, neighbor.manAddrs)
			continue
		}

		// ?��??�己??���?
		if remoteDeviceID == deviceID {
			continue
		}

		// 建�???? key ?�於?��? (確�? A-B ??B-A ?��??��?�?
		linkKey := fmt.Sprintf("%d-%d", min(deviceID, remoteDeviceID), max(deviceID, remoteDeviceID))
		if discoveredLinks[linkKey] {
			continue
		}

		// 檢查????�否已�???
		var existingID int
		err := c.db.QueryRow(`
			SELECT id FROM topology_links 
			WHERE (source_device_id = ? AND target_device_id = ?) 
			   OR (source_device_id = ? AND target_device_id = ?)
		`, deviceID, remoteDeviceID, remoteDeviceID, deviceID).Scan(&existingID)

		// ?��?來�?介面資�?
		var sourceIfID sql.NullInt64
		var sourceSpeed sql.NullInt64
		c.db.QueryRow("SELECT id, if_speed FROM device_interfaces WHERE device_id = ? AND if_index = ?", deviceID, localIfIndex).Scan(&sourceIfID, &sourceSpeed)

		if err == sql.ErrNoRows {
			// 建�??��??
			_, insertErr := c.db.Exec(`
				INSERT INTO topology_links (source_device_id, target_device_id, source_if_id, link_speed, link_type, is_manual) 
				VALUES (?, ?, ?, ?, 'lldp', 0)
			`, deviceID, remoteDeviceID, sourceIfID, sourceSpeed.Int64)

			if insertErr == nil {
				log.Printf("LLDP: Created link between device %d and %d (Speed: %d)", deviceID, remoteDeviceID, sourceSpeed.Int64)
				discoveredLinks[linkKey] = true
			} else {
				log.Printf("LLDP: Failed to create link: %v", insertErr)
			}
		} else if err == nil {
			// ???已�??��??�試?�新?�度資�? (如�???0 ??NULL)
			if sourceSpeed.Int64 > 0 {
				c.db.Exec("UPDATE topology_links SET link_speed = ?, source_if_id = ? WHERE id = ? AND (link_speed IS NULL OR link_speed = 0)", sourceSpeed.Int64, sourceIfID, existingID)
			}
			discoveredLinks[linkKey] = true
			log.Printf("LLDP: Link already exists between device %d and %d (id=%d)", deviceID, remoteDeviceID, existingID)
		}
	}
}

// extractLLDPIndices �?LLDP OID 中�??�索�?
func extractLLDPIndices(fullOID, baseOID string) []int {
	if len(fullOID) <= len(baseOID)+1 {
		return nil
	}
	suffix := fullOID[len(baseOID)+1:]
	parts := strings.Split(suffix, ".")
	indices := make([]int, 0, len(parts))
	for _, p := range parts {
		var idx int
		for _, c := range p {
			if c >= '0' && c <= '9' {
				idx = idx*10 + int(c-'0')
			} else {
				break
			}
		}
		indices = append(indices, idx)
	}
	return indices
}

// collectFdbTable ?��?交�??��? FDB �?(MAC -> Port)
func (c *Collector) collectFdbTable(params *gosnmp.GoSNMP, deviceID int, switchFdbMap *sync.Map) {
	if switchFdbMap == nil {
		return
	}
	// 1. ?��? Bridge Port ??ifIndex ?��???
	portToIfIndex := make(map[int]int)
	params.Walk(OIDDot1dBasePortIfIndex, func(pdu gosnmp.SnmpPDU) error {
		bridgePort := extractIndex(pdu.Name, OIDDot1dBasePortIfIndex)
		ifIndex := int(gosnmp.ToBigInt(pdu.Value).Int64())
		if bridgePort > 0 && ifIndex > 0 {
			portToIfIndex[bridgePort] = ifIndex
		}
		return nil
	})

	// 2. ?��? MAC ??Bridge Port ?��???
	macToPort := make(map[string]int)
	params.Walk(OIDDot1dTpFdbPort, func(pdu gosnmp.SnmpPDU) error {
		bridgePort := int(gosnmp.ToBigInt(pdu.Value).Int64())
		if bridgePort <= 0 {
			return nil
		}

		// �?OID ?��? MAC ?��?
		// .1.3.6.1.2.1.17.4.3.1.2.m1.m2.m3.m4.m5.m6
		parts := strings.Split(pdu.Name, ".")
		if len(parts) >= 6 {
			m1, _ := strconv.Atoi(parts[len(parts)-6])
			m2, _ := strconv.Atoi(parts[len(parts)-5])
			m3, _ := strconv.Atoi(parts[len(parts)-4])
			m4, _ := strconv.Atoi(parts[len(parts)-3])
			m5, _ := strconv.Atoi(parts[len(parts)-2])
			m6, _ := strconv.Atoi(parts[len(parts)-1])
			mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", m1, m2, m3, m4, m5, m6)

			if ifIndex, ok := portToIfIndex[bridgePort]; ok {
				macToPort[mac] = ifIndex
			}
		}
		return nil
	})

	if len(macToPort) > 0 {
		switchFdbMap.Store(deviceID, macToPort)
		log.Printf("[SNMP] Collected %d FDB entries from switch %d", len(macToPort), deviceID)
	}
}

// SyncFdbTopology ?��? FDB 資�??�步?�樸???
func (c *Collector) SyncFdbTopology(switchFdbMap *sync.Map) {
	// 1. 建�? MAC ??DeviceID ?��?�?
	macToDeviceID := make(map[string]int)
	rows, err := c.db.Query("SELECT id, UPPER(mac_address), device_type FROM devices WHERE mac_address IS NOT NULL AND mac_address != ''")
	if err != nil {
		return
	}
	defer rows.Close()

	deviceTypes := make(map[int]string)
	for rows.Next() {
		var id int
		var mac, dType string
		if err := rows.Scan(&id, &mac, &dType); err == nil {
			macToDeviceID[mac] = id
			deviceTypes[id] = dType
		}
	}

	// 2. ?�歷每�?Switch ??FDB �?
	switchFdbMap.Range(func(key, value interface{}) bool {
		switchID := key.(int)
		macToPort := value.(map[string]int)

		// 統�?每�?port ?��?少�?MAC (如�?一??port 太�? MAC，通常??Uplink/Trunk，�?建�??��????)
		portMacCount := make(map[int]int)
		for _, ifIndex := range macToPort {
			portMacCount[ifIndex]++
		}

		for mac, ifIndex := range macToPort {
			targetDeviceID, found := macToDeviceID[mac]
			if !found || targetDeviceID == switchID {
				continue
			}

			// 如�?�?Port ?��???2 ??MAC，跳??(?��???Uplink ?��??�接???)
			if portMacCount[ifIndex] > 2 {
				continue
			}

			// 如�??��???Switch/Router/Firewall，優?�信�?LLDP，此?�跳??
			if tType := deviceTypes[targetDeviceID]; tType == "switch" || tType == "router" || tType == "firewall" {
				continue
			}

			// 建�????
			c.createFdbLink(switchID, targetDeviceID, ifIndex)
		}
		return true
	})
}

// createFdbLink 建�??�更??FDB ???
func (c *Collector) createFdbLink(switchID, targetID, ifIndex int) {
	// ?��?來�?介面 ID
	var sourceIfID sql.NullInt64
	var sourceSpeed sql.NullInt64
	var ifName sql.NullString
	c.db.QueryRow("SELECT id, if_speed, if_name FROM device_interfaces WHERE device_id = ? AND if_index = ?", switchID, ifIndex).Scan(&sourceIfID, &sourceSpeed, &ifName)

	if !sourceIfID.Valid {
		return
	}

	// 檢查????�否已�???
	var existingID int
	err := c.db.QueryRow(`
		SELECT id FROM topology_links 
		WHERE (source_device_id = ? AND target_device_id = ?) 
		   OR (source_device_id = ? AND target_device_id = ?)
	`, switchID, targetID, targetID, switchID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// 建�??��??
		_, err := c.db.Exec(`
			INSERT INTO topology_links (source_device_id, target_device_id, source_if_id, source_if_name, link_speed, link_type, is_manual) 
			VALUES (?, ?, ?, ?, ?, 'fdb', 0)
		`, switchID, targetID, sourceIfID, ifName, sourceSpeed.Int64)
		if err == nil {
			log.Printf("[Topology] Created FDB link: Switch %d (Port %d) -> Device %d", switchID, ifIndex, targetID)
		}
	} else if err == nil {
		// 已�??��??，�??�是?��??�測?��??�新介面資�?
		c.db.Exec(`
			UPDATE topology_links 
			SET source_if_id = ?, source_if_name = ?, link_speed = ?, link_type = 'fdb'
			WHERE id = ? AND is_manual = 0 AND (link_type = 'auto' OR link_type = 'fdb')
		`, sourceIfID, ifName, sourceSpeed.Int64, existingID)
	}
}

// collectArpTable ?��? ARP 表�?�?
func (c *Collector) collectArpTable(params *gosnmp.GoSNMP, ipToMacMap *sync.Map) {
	if ipToMacMap == nil {
		return
	}
	// ?�試 IPv4 ARP Table
	params.Walk(OIDIpNetToPhysicalPhysAddress, func(pdu gosnmp.SnmpPDU) error {
		if bytes, ok := pdu.Value.([]byte); ok && len(bytes) == 6 {
			mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])

			// �?OID ?��? IP (?��?4 �?
			parts := strings.Split(pdu.Name, ".")
			if len(parts) >= 4 {
				ip := fmt.Sprintf("%s.%s.%s.%s", parts[len(parts)-4], parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1])
				ipToMacMap.Store(ip, mac)
			}
		}
		return nil
	})

	// ?�試?��? ARP Table (RFC1213)
	params.Walk(OIDIpNetToMediaPhysAddress, func(pdu gosnmp.SnmpPDU) error {
		if bytes, ok := pdu.Value.([]byte); ok && len(bytes) == 6 {
			mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5])

			parts := strings.Split(pdu.Name, ".")
			if len(parts) >= 4 {
				ip := fmt.Sprintf("%s.%s.%s.%s", parts[len(parts)-4], parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1])
				ipToMacMap.Store(ip, mac)
			}
		}
		return nil
	})
}

// SyncPingOnlyMacs ?�步?��??��? SNMP ?��? MAC ?�設??
func (c *Collector) SyncPingOnlyMacs(ipToMacMap *sync.Map) {
	rows, err := c.db.Query("SELECT id, ip_address FROM devices WHERE (mac_address = '' OR mac_address IS NULL)")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var ip string
		if err := rows.Scan(&id, &ip); err == nil {
			if mac, ok := ipToMacMap.Load(ip); ok {
				macStr := mac.(string)
				c.dbWorker.Push(func(db *sql.DB) error {
					_, err := dbutils.ExecWithRetry(db, "UPDATE devices SET mac_address = ? WHERE id = ?", macStr, id)
					return err
				})
				log.Printf("[SNMP] Mapped MAC %s to IP %s (from switch table)", macStr, ip)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
