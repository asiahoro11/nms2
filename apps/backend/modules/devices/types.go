// Made by YTSworks
// YTS工作室製作
package devices

import "errors"

var ErrNoFieldsToUpdate = errors.New("no fields to update")

type Device struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	SysName       *string `json:"sys_name"`
	IPAddress     string  `json:"ip_address"`
	MACAddress    *string `json:"mac_address"`
	DeviceType    string  `json:"device_type"`
	SNMPCommunity string  `json:"-"`
	SNMPVersion   int     `json:"snmp_version"`
	Vendor        *string `json:"vendor"`
	Model         *string `json:"model"`
	Firmware      *string `json:"firmware"`
	IsOnline      bool    `json:"is_online"`
	LastSeen      *string `json:"last_seen"`
	ImagePath     *string `json:"image_path"`
	PosX          float64 `json:"pos_x"`
	PosY          float64 `json:"pos_y"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type DeviceInput struct {
	Name          string  `json:"name" binding:"required"`
	IPAddress     string  `json:"ip_address" binding:"required"`
	MACAddress    *string `json:"mac_address"`
	DeviceType    string  `json:"device_type"`
	SNMPCommunity string  `json:"snmp_community"`
	SNMPVersion   int     `json:"snmp_version"`
}

type ListQuery struct {
	Page       int
	Limit      int
	Search     string
	DeviceType string
	Status     string
	MaxDevices int
}

type DeviceInterface struct {
	ID            int    `json:"id"`
	DeviceID      int    `json:"device_id"`
	IfIndex       int64  `json:"if_index"`
	IfName        string `json:"if_name"`
	IfDesc        string `json:"if_desc"`
	IfSpeed       int64  `json:"if_speed"`
	IfMAC         string `json:"if_mac"`
	IfStatus      string `json:"if_status"`
	IfAdminStatus int64  `json:"if_admin_status"`
	InOctets      int64  `json:"in_octets"`
	OutOctets     int64  `json:"out_octets"`
	BandwidthIn   int64  `json:"bandwidth_in"`
	BandwidthOut  int64  `json:"bandwidth_out"`
	UpdatedAt     string `json:"updated_at"`
}

type DeleteResult struct {
	ID         string
	Name       string
	IPAddress  string
	MACAddress string
	DeviceType string
}

type DeviceMetric struct {
	ID          int      `json:"id"`
	DeviceID    int      `json:"device_id"`
	CPUUsage    *float64 `json:"cpu_usage"`
	MemoryUsage *float64 `json:"memory_usage"`
	DiskUsage   *float64 `json:"disk_usage"`
	CollectedAt string   `json:"collected_at"`
}

type BulkUpdateInput struct {
	IDs       []int
	Vendor    string
	Model     string
	ImagePath string
}
