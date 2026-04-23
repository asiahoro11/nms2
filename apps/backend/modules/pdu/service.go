package pdu

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	oidUpsBatteryStatus             = ".1.3.6.1.2.1.33.1.2.1.0"
	oidUpsEstimatedMinutesRemaining = ".1.3.6.1.2.1.33.1.2.3.0"
	oidUpsEstimatedChargeRemaining  = ".1.3.6.1.2.1.33.1.2.4.0"
	oidUpsBatteryTemperature        = ".1.3.6.1.2.1.33.1.2.7.0"
	oidUpsInputVoltage              = ".1.3.6.1.2.1.33.1.3.3.1.3"
	oidUpsOutputSource              = ".1.3.6.1.2.1.33.1.4.1.0"
	oidUpsOutputVoltage             = ".1.3.6.1.2.1.33.1.4.4.1.2"
	oidUpsOutputLoad                = ".1.3.6.1.2.1.33.1.4.4.1.5"
	oidUpsAlarmsPresent             = ".1.3.6.1.2.1.33.1.6.1.0"
	oidApcBatteryCapacity           = ".1.3.6.1.4.1.318.1.1.1.2.2.1.0"
	oidApcBatteryStatus             = ".1.3.6.1.4.1.318.1.1.1.2.2.4.0"
	oidApcBatteryTempC              = ".1.3.6.1.4.1.318.1.1.1.2.2.2.0"
	oidApcBatteryRuntime            = ".1.3.6.1.4.1.318.1.1.1.2.2.3.0"
	oidApcInputVoltage              = ".1.3.6.1.4.1.318.1.1.1.3.2.1.0"
	oidApcOutputLoad                = ".1.3.6.1.4.1.318.1.1.1.4.2.3.0"
	oidApcOutputVoltage             = ".1.3.6.1.4.1.318.1.1.1.4.2.1.0"
)

var ErrDeviceNotFound = errors.New("pdu_device_not_found")

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) LicenseEnabled() bool {
	var val string
	err := s.db.QueryRow(`SELECT config_value FROM system_config WHERE config_key='pdu_enabled'`).Scan(&val)
	return err == nil && val == "1"
}

func (s *Service) ModuleStatus() ModuleStatus {
	return ModuleStatus{Enabled: s.LicenseEnabled()}
}

func (s *Service) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`
		SELECT id, name, location, ip_address, port, snmp_community, snmp_version,
		       device_type, manufacturer, model, is_enabled, status,
		       strftime('%Y-%m-%dT%H:%M:%SZ', last_polled_at),
		       strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
		FROM pdu_devices ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var d Device
		var lastPolled *string
		if err := rows.Scan(&d.ID, &d.Name, &d.Location, &d.IPAddress, &d.Port,
			&d.SNMPCommunity, &d.SNMPVersion, &d.DeviceType, &d.Manufacturer, &d.Model,
			&d.IsEnabled, &d.Status, &lastPolled, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.LastPolledAt = lastPolled
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

func (s *Service) GetDevice(id string, withLiveData bool) (Device, error) {
	device, err := s.loadDevice(id)
	if err != nil {
		return Device{}, err
	}
	if withLiveData && device.IsEnabled {
		s.fetchLiveData(&device)
	}
	return device, nil
}

func (s *Service) CreateDevice(req DeviceRequest) (int64, error) {
	normalizeDeviceRequest(&req)
	res, err := s.db.Exec(`
		INSERT INTO pdu_devices (name, location, ip_address, port, snmp_community, snmp_version,
		    device_type, manufacturer, model, is_enabled, status)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		req.Name, req.Location, req.IPAddress, req.Port, req.SNMPCommunity, req.SNMPVersion,
		req.DeviceType, req.Manufacturer, req.Model, req.IsEnabled, "unknown")
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) UpdateDevice(id string, req DeviceRequest) error {
	normalizeDeviceRequest(&req)
	res, err := s.db.Exec(`
		UPDATE pdu_devices SET name=?, location=?, ip_address=?, port=?, snmp_community=?,
		    snmp_version=?, device_type=?, manufacturer=?, model=?, is_enabled=?
		WHERE id=?`,
		req.Name, req.Location, req.IPAddress, req.Port, req.SNMPCommunity,
		req.SNMPVersion, req.DeviceType, req.Manufacturer, req.Model, req.IsEnabled, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDeviceNotFound
	}
	return nil
}

func (s *Service) DeleteDevice(id string) error {
	res, err := s.db.Exec(`DELETE FROM pdu_devices WHERE id=?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDeviceNotFound
	}
	return nil
}

func (s *Service) PollDevice(id string) (Device, error) {
	device, err := s.loadDevice(id)
	if err != nil {
		return Device{}, err
	}

	s.fetchLiveData(&device)
	device.Status = determineStatus(&device)
	if _, err := s.db.Exec(`UPDATE pdu_devices SET status=?, last_polled_at=datetime('now') WHERE id=?`, device.Status, id); err != nil {
		return Device{}, err
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	device.LastPolledAt = &now
	return device, nil
}

func (s *Service) loadDevice(id string) (Device, error) {
	var d Device
	var lastPolled *string
	err := s.db.QueryRow(`
		SELECT id, name, location, ip_address, port, snmp_community, snmp_version,
		       device_type, manufacturer, model, is_enabled, status,
		       strftime('%Y-%m-%dT%H:%M:%SZ', last_polled_at),
		       strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
		FROM pdu_devices WHERE id=?`, id).
		Scan(&d.ID, &d.Name, &d.Location, &d.IPAddress, &d.Port, &d.SNMPCommunity,
			&d.SNMPVersion, &d.DeviceType, &d.Manufacturer, &d.Model, &d.IsEnabled,
			&d.Status, &lastPolled, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return Device{}, ErrDeviceNotFound
	}
	if err != nil {
		return Device{}, err
	}
	d.LastPolledAt = lastPolled
	return d, nil
}

func normalizeDeviceRequest(req *DeviceRequest) {
	if req.Port == 0 {
		req.Port = 161
	}
	if req.SNMPCommunity == "" {
		req.SNMPCommunity = "public"
	}
	if req.SNMPVersion == 0 {
		req.SNMPVersion = 2
	}
	if req.DeviceType == "" {
		req.DeviceType = "ups"
	}
}

func (s *Service) fetchLiveData(d *Device) {
	client, err := connect(d.IPAddress, d.Port, d.SNMPCommunity, d.SNMPVersion)
	if err != nil {
		log.Printf("[PDU] connect failed %s: %v", d.IPAddress, err)
		return
	}
	defer client.Conn.Close()

	if d.DeviceType == "ups" {
		fetchUPS(client, d)
	}
}

func connect(ip string, port int, community string, version int) (*gosnmp.GoSNMP, error) {
	v := gosnmp.Version2c
	if version == 1 {
		v = gosnmp.Version1
	}
	g := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      uint16(port),
		Community: community,
		Version:   v,
		Timeout:   5 * time.Second,
		Retries:   1,
	}
	if err := g.Connect(); err != nil {
		return nil, err
	}
	return g, nil
}

func snmpGetInt(g *gosnmp.GoSNMP, oid string) (int, error) {
	result, err := g.Get([]string{oid})
	if err != nil || len(result.Variables) == 0 {
		return 0, fmt.Errorf("snmp get failed")
	}
	v := result.Variables[0]
	switch v.Type {
	case gosnmp.Integer, gosnmp.Gauge32, gosnmp.Counter32, gosnmp.TimeTicks:
		return int(gosnmp.ToBigInt(v.Value).Int64()), nil
	default:
		return 0, fmt.Errorf("unexpected type %v", v.Type)
	}
}

func snmpGetStr(g *gosnmp.GoSNMP, oid string) (string, error) {
	result, err := g.Get([]string{oid})
	if err != nil || len(result.Variables) == 0 {
		return "", fmt.Errorf("snmp get failed")
	}
	v := result.Variables[0]
	if v.Type == gosnmp.OctetString {
		return strings.TrimSpace(string(v.Value.([]byte))), nil
	}
	return fmt.Sprintf("%v", v.Value), nil
}

func fetchUPS(g *gosnmp.GoSNMP, d *Device) {
	if v, err := snmpGetInt(g, oidUpsEstimatedChargeRemaining); err == nil {
		d.BatteryCapacityPct = &v
	} else if v2, err2 := snmpGetInt(g, oidApcBatteryCapacity); err2 == nil {
		d.BatteryCapacityPct = &v2
	}

	if v, err := snmpGetInt(g, oidUpsEstimatedMinutesRemaining); err == nil {
		d.BatteryRuntimeMin = &v
	} else if v2, err2 := snmpGetInt(g, oidApcBatteryRuntime); err2 == nil {
		d.BatteryRuntimeMin = &v2
	}

	if v, err := snmpGetInt(g, oidUpsBatteryTemperature); err == nil {
		f := float64(v)
		d.BatteryTempC = &f
	} else if v2, err2 := snmpGetInt(g, oidApcBatteryTempC); err2 == nil {
		f := float64(v2)
		d.BatteryTempC = &f
	}

	statusCode := 0
	if v, err := snmpGetInt(g, oidUpsBatteryStatus); err == nil {
		statusCode = v
	} else if v2, err2 := snmpGetInt(g, oidApcBatteryStatus); err2 == nil {
		statusCode = v2
	}
	if statusCode > 0 {
		s := batteryStatusText(statusCode)
		d.BatteryStatus = &s
	}

	if v, err := snmpGetInt(g, oidUpsInputVoltage+".1"); err == nil {
		f := float64(v)
		d.InputVoltage = &f
	} else if v2, err2 := snmpGetInt(g, oidApcInputVoltage); err2 == nil {
		f := float64(v2)
		d.InputVoltage = &f
	}

	if v, err := snmpGetInt(g, oidUpsOutputLoad+".1"); err == nil {
		d.OutputLoadPct = &v
	} else if v2, err2 := snmpGetInt(g, oidApcOutputLoad); err2 == nil {
		d.OutputLoadPct = &v2
	}

	if v, err := snmpGetInt(g, oidUpsOutputVoltage+".1"); err == nil {
		f := float64(v)
		d.OutputVoltage = &f
	} else if v2, err2 := snmpGetInt(g, oidApcOutputVoltage); err2 == nil {
		f := float64(v2)
		d.OutputVoltage = &f
	}

	if v, err := snmpGetInt(g, oidUpsOutputSource); err == nil {
		s := outputSourceText(v)
		d.OutputSource = &s
	}

	if v, err := snmpGetInt(g, oidUpsAlarmsPresent); err == nil {
		d.AlarmsPresent = &v
	}

	if d.Manufacturer == "" {
		if manufacturer, err := snmpGetStr(g, ".1.3.6.1.2.1.33.1.1.1.0"); err == nil {
			d.Manufacturer = manufacturer
		}
	}
	if d.Model == "" {
		if model, err := snmpGetStr(g, ".1.3.6.1.2.1.33.1.1.2.0"); err == nil {
			d.Model = model
		}
	}
}

func batteryStatusText(code int) string {
	switch code {
	case 2:
		return "normal"
	case 3:
		return "low"
	case 4:
		return "depleted"
	default:
		return "unknown"
	}
}

func outputSourceText(code int) string {
	switch code {
	case 3:
		return "normal"
	case 4:
		return "bypass"
	case 5:
		return "battery"
	case 6:
		return "booster"
	case 7:
		return "reducer"
	default:
		return "unknown"
	}
}

func determineStatus(d *Device) string {
	if d.BatteryStatus != nil {
		switch *d.BatteryStatus {
		case "depleted":
			return "critical"
		case "low":
			return "warning"
		}
	}
	if d.BatteryCapacityPct != nil {
		if *d.BatteryCapacityPct < 20 {
			return "critical"
		}
		if *d.BatteryCapacityPct < 50 {
			return "warning"
		}
	}
	if d.OutputSource != nil && *d.OutputSource == "battery" {
		return "warning"
	}
	if d.AlarmsPresent != nil && *d.AlarmsPresent > 0 {
		return "warning"
	}
	if d.BatteryCapacityPct != nil || d.InputVoltage != nil {
		return "normal"
	}
	return "offline"
}

func parseID(id string) (int, error) {
	return strconv.Atoi(id)
}
