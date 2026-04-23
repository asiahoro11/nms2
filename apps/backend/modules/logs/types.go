package logs

type AuditEntry struct {
	ID           int    `json:"id"`
	OccurredAt   string `json:"occurred_at"`
	Username     string `json:"username"`
	SourceIP     string `json:"source_ip"`
	SourceMAC    string `json:"source_mac"`
	Module       string `json:"module"`
	Action       string `json:"action"`
	Resource     string `json:"resource"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceIP   string `json:"resource_ip"`
	ResourceMAC  string `json:"resource_mac"`
	Status       string `json:"status"`
	Detail       string `json:"detail"`
	DetailJSON   string `json:"detail_json"`
	RecordsetID  string `json:"recordset_id"`
	Correlation  string `json:"correlation_id"`
	ReviewStatus string `json:"review_status"`
	ReviewedBy   string `json:"reviewed_by"`
	ReviewedAt   string `json:"reviewed_at"`
	ReviewNote   string `json:"review_note"`
}

type SystemLogEntry struct {
	ID           int    `json:"id"`
	OccurredAt   string `json:"occurred_at"`
	Level        string `json:"level"`
	Service      string `json:"service"`
	EventCode    string `json:"event_code"`
	Message      string `json:"message"`
	ContextJSON  string `json:"context_json"`
	NodeName     string `json:"node_name"`
	BuildVersion string `json:"build_version"`
	ReviewStatus string `json:"review_status"`
	ReviewedBy   string `json:"reviewed_by"`
	ReviewedAt   string `json:"reviewed_at"`
	ReviewNote   string `json:"review_note"`
}

type DeviceLogEntry struct {
	ID                int    `json:"id"`
	OccurredAt        string `json:"occurred_at"`
	DeviceID          *int   `json:"device_id"`
	DeviceName        string `json:"device_name"`
	IPAddress         string `json:"ip_address"`
	MACAddress        string `json:"mac_address"`
	Facility          string `json:"facility"`
	Severity          string `json:"severity"`
	RawMessage        string `json:"raw_message"`
	NormalizedMessage string `json:"normalized_message"`
	MatchedRule       string `json:"matched_rule"`
	AckStatus         string `json:"ack_status"`
	ContextJSON       string `json:"context_json"`
	ReviewStatus      string `json:"review_status"`
	ReviewedBy        string `json:"reviewed_by"`
	ReviewedAt        string `json:"reviewed_at"`
	ReviewNote        string `json:"review_note"`
}

type ConfigChangeLogEntry struct {
	ID            int    `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	Username      string `json:"username"`
	SourceIP      string `json:"source_ip"`
	Module        string `json:"module"`
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	TargetName    string `json:"target_name"`
	TargetIP      string `json:"target_ip"`
	TargetMAC     string `json:"target_mac"`
	Action        string `json:"action"`
	ChangeScope   string `json:"change_scope"`
	Status        string `json:"status"`
	ChangeSource  string `json:"change_source"`
	RecordsetID   string `json:"recordset_id"`
	CorrelationID string `json:"correlation_id"`
	OldValuesJSON string `json:"old_values_json"`
	NewValuesJSON string `json:"new_values_json"`
	DetailJSON    string `json:"detail_json"`
	ReviewStatus  string `json:"review_status"`
	ReviewedBy    string `json:"reviewed_by"`
	ReviewedAt    string `json:"reviewed_at"`
	ReviewNote    string `json:"review_note"`
}

type QueryFilters struct {
	Level        string
	Service      string
	ReviewStatus string
	Search       string
	DateFrom     string
	DateTo       string
	DeviceID     string
	Device       string
	Severity     string
	Facility     string
	AckStatus    string
	Module       string
	TargetType   string
	Username     string
	Action       string
	Status       string
	Scope        string
	Actor        string
	Start        string
	End          string
}

type LogCenterFilters struct {
	Severity string `json:"severity"`
	Search   string `json:"search"`
	Scope    string `json:"scope"`
	Actor    string `json:"actor"`
	Device   string `json:"device"`
	Status   string `json:"status"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type Options struct {
	SystemServices   []string `json:"system_services"`
	DeviceFacilities []string `json:"device_facilities"`
	AuditModules     []string `json:"audit_modules"`
	ConfigModules    []string `json:"config_modules"`
}
