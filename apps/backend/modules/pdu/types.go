package pdu

type Device struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Location      string  `json:"location"`
	IPAddress     string  `json:"ip_address"`
	Port          int     `json:"port"`
	SNMPCommunity string  `json:"-"`
	SNMPVersion   int     `json:"snmp_version"`
	DeviceType    string  `json:"device_type"`
	Manufacturer  string  `json:"manufacturer"`
	Model         string  `json:"model"`
	IsEnabled     bool    `json:"is_enabled"`
	Status        string  `json:"status"`
	LastPolledAt  *string `json:"last_polled_at"`
	CreatedAt     string  `json:"created_at"`

	BatteryStatus      *string  `json:"battery_status,omitempty"`
	BatteryCapacityPct *int     `json:"battery_capacity_pct,omitempty"`
	BatteryRuntimeMin  *int     `json:"battery_runtime_min,omitempty"`
	BatteryTempC       *float64 `json:"battery_temp_c,omitempty"`
	InputVoltage       *float64 `json:"input_voltage,omitempty"`
	OutputVoltage      *float64 `json:"output_voltage,omitempty"`
	OutputLoadPct      *int     `json:"output_load_pct,omitempty"`
	OutputSource       *string  `json:"output_source,omitempty"`
	AlarmsPresent      *int     `json:"alarms_present,omitempty"`
}

type DeviceRequest struct {
	Name          string `json:"name"`
	Location      string `json:"location"`
	IPAddress     string `json:"ip_address"`
	Port          int    `json:"port"`
	SNMPCommunity string `json:"snmp_community"`
	SNMPVersion   int    `json:"snmp_version"`
	DeviceType    string `json:"device_type"`
	Manufacturer  string `json:"manufacturer"`
	Model         string `json:"model"`
	IsEnabled     bool   `json:"is_enabled"`
}

type ModuleStatus struct {
	Enabled bool `json:"enabled"`
}
