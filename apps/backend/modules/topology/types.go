package topology

type Data struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

type CanonicalGraph struct {
	SchemaVersion string            `json:"schema_version"`
	GraphID       string            `json:"graph_id"`
	GeneratedAt   string            `json:"generated_at"`
	Nodes         []CanonicalNode   `json:"nodes"`
	Links         []CanonicalLink   `json:"links"`
	Layouts       []CanonicalLayout `json:"layouts"`
	Meta          map[string]any    `json:"meta"`
}

type CanonicalNode struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Type        string         `json:"type"`
	IPAddress   string         `json:"ip_address"`
	MACAddress  string         `json:"mac_address,omitempty"`
	Status      string         `json:"status"`
	Role        string         `json:"role,omitempty"`
	LastSeen    string         `json:"last_seen,omitempty"`
	Source      string         `json:"source_of_truth"`
	ExternalID  string         `json:"external_id,omitempty"`
	Confidence  float64        `json:"confidence"`
	Attributes  map[string]any `json:"attributes,omitempty"`
}

type CanonicalLink struct {
	ID              string         `json:"id"`
	SourceNodeID    string         `json:"source_node_id"`
	TargetNodeID    string         `json:"target_node_id"`
	SourcePortID    string         `json:"source_port_id,omitempty"`
	TargetPortID    string         `json:"target_port_id,omitempty"`
	LinkType        string         `json:"link_type"`
	Status          string         `json:"status"`
	SpeedLimit      int64          `json:"speed_limit"`
	TrafficInBps    int64          `json:"traffic_in_bps"`
	TrafficOutBps   int64          `json:"traffic_out_bps"`
	DiscoverySource string         `json:"discovery_source"`
	Confidence      float64        `json:"confidence"`
	Attributes      map[string]any `json:"attributes,omitempty"`
}

type CanonicalLayout struct {
	LayoutID      string             `json:"layout_id"`
	GraphID       string             `json:"graph_id"`
	LayoutType    string             `json:"layout_type"`
	NodePositions []CanonicalNodePos `json:"node_positions"`
}

type CanonicalNodePos struct {
	NodeID string  `json:"node_id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

type MutationMeta struct {
	Actor         string
	SourceIP      string
	CorrelationID string
}

type CreateLinkResult struct {
	ID           int64
	SourceIfName string
	TargetIfName string
	LinkSpeed    int64
}

type UpdateLinkResult struct {
	Before map[string]interface{}
	After  map[string]interface{}
}

type DeleteLinkResult struct {
	Before map[string]interface{}
}

type Node struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	IPAddress    string  `json:"ip_address"`
	DeviceType   string  `json:"device_type"`
	IsOnline     bool    `json:"is_online"`
	ImagePath    *string `json:"image_path"`
	SysName      *string `json:"sys_name"`
	SysUptime    *string `json:"sys_uptime"`
	SysLocation  *string `json:"sys_location"`
	PosX         float64 `json:"x"`
	PosY         float64 `json:"y"`
	SnmpVersion  int     `json:"snmp_version"`
	IsNameCustom bool    `json:"is_name_custom"`
}

type Link struct {
	ID             int     `json:"id"`
	Source         int     `json:"source"`
	Target         int     `json:"target"`
	SourceIfID     *int    `json:"source_if_id"`
	TargetIfID     *int    `json:"target_if_id"`
	SourceIfName   *string `json:"source_if_name"`
	TargetIfName   *string `json:"target_if_name"`
	LinkSpeed      int64   `json:"link_speed"`
	BandwidthUsage int64   `json:"bandwidth_usage"`
	BandwidthIn    int64   `json:"bandwidth_in"`
	BandwidthOut   int64   `json:"bandwidth_out"`
	LinkType       string  `json:"link_type"`
	LinkLabel      *string `json:"link_label"`
	IsManual       bool    `json:"is_manual"`
}

type CreateLinkInput struct {
	SourceDeviceID int    `json:"source_device_id" binding:"required"`
	TargetDeviceID int    `json:"target_device_id" binding:"required"`
	SourceIfID     *int   `json:"source_if_id"`
	TargetIfID     *int   `json:"target_if_id"`
	SourceIfName   string `json:"source_if_name"`
	TargetIfName   string `json:"target_if_name"`
	LinkSpeed      int64  `json:"link_speed"`
	LinkType       string `json:"link_type"`
	LinkLabel      string `json:"link_label"`
}

type UpdatePositionsInput struct {
	Positions []PositionUpdate `json:"positions" binding:"required"`
}

type PositionUpdate struct {
	DeviceID int     `json:"device_id" binding:"required"`
	PosX     float64 `json:"x"`
	PosY     float64 `json:"y"`
}

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}
