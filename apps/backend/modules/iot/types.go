package iot

type Device struct {
	ID                  int      `json:"id"`
	Name                string   `json:"name"`
	Protocol            string   `json:"protocol"`
	Host                string   `json:"host,omitempty"`
	Port                int      `json:"port,omitempty"`
	UnitID              int      `json:"unit_id,omitempty"`
	Address             int      `json:"address,omitempty"`
	Quantity            int      `json:"quantity,omitempty"`
	FunctionCode        int      `json:"function_code,omitempty"`
	DataType            string   `json:"data_type,omitempty"`
	ByteOrder           string   `json:"byte_order,omitempty"`
	WordOrder           string   `json:"word_order,omitempty"`
	Scale               float64  `json:"scale"`
	Offset              float64  `json:"offset"`
	Metric              string   `json:"metric,omitempty"`
	Topic               string   `json:"topic,omitempty"`
	PollIntervalSeconds int      `json:"poll_interval_seconds"`
	Enabled             bool     `json:"enabled"`
	LastValue           *float64 `json:"last_value,omitempty"`
	LastRaw             string   `json:"last_raw,omitempty"`
	LastSeen            string   `json:"last_seen,omitempty"`
	LastPolledAt        string   `json:"last_polled_at,omitempty"`
	LastError           string   `json:"last_error,omitempty"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type UpsertDeviceInput struct {
	Name                string  `json:"name"`
	Protocol            string  `json:"protocol"`
	Host                string  `json:"host"`
	Port                int     `json:"port"`
	UnitID              int     `json:"unit_id"`
	Address             int     `json:"address"`
	Quantity            int     `json:"quantity"`
	FunctionCode        int     `json:"function_code"`
	DataType            string  `json:"data_type"`
	ByteOrder           string  `json:"byte_order"`
	WordOrder           string  `json:"word_order"`
	Scale               float64 `json:"scale"`
	Offset              float64 `json:"offset"`
	Metric              string  `json:"metric"`
	Topic               string  `json:"topic"`
	PollIntervalSeconds int     `json:"poll_interval_seconds"`
	Enabled             *bool   `json:"enabled"`
}

type IngestInput struct {
	ExternalID string                 `json:"external_id"`
	Name       string                 `json:"name"`
	Protocol   string                 `json:"protocol"`
	Metric     string                 `json:"metric"`
	Value      float64                `json:"value"`
	Raw        map[string]interface{} `json:"raw"`
}

type Measurement struct {
	ID              int     `json:"id"`
	EventID         string  `json:"event_id,omitempty"`
	DeviceID        *int    `json:"device_id,omitempty"`
	ExternalID      string  `json:"external_id,omitempty"`
	Metric          string  `json:"metric"`
	Value           float64 `json:"value"`
	RawJSON         string  `json:"raw_json,omitempty"`
	ForwardStatus   string  `json:"forward_status,omitempty"`
	ForwardAttempts int     `json:"forward_attempts"`
	ForwardedAt     string  `json:"forwarded_at,omitempty"`
	DropAfter       string  `json:"drop_after,omitempty"`
	LastError       string  `json:"last_error,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type Status struct {
	DeviceCount          int    `json:"device_count"`
	EnabledCount         int    `json:"enabled_count"`
	MeasurementCount     int    `json:"measurement_count"`
	ForwardPendingCount  int    `json:"forward_pending_count"`
	ForwardFailedCount   int    `json:"forward_failed_count"`
	ForwardSentHoldCount int    `json:"forward_sent_hold_count"`
	ForwardEnabled       bool   `json:"forward_enabled"`
	ForwardURLConfigured bool   `json:"forward_url_configured"`
	ForwardLastError     string `json:"forward_last_error,omitempty"`
}

type ProtocolProfile struct {
	Protocol    string   `json:"protocol"`
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
	DataTypes   []string `json:"data_types,omitempty"`
	Endpoint    string   `json:"endpoint,omitempty"`
}

type ForwarderSettings struct {
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url"`
	TokenConfigured  bool   `json:"token_configured"`
	BatchSize        int    `json:"batch_size"`
	RetentionMinutes int    `json:"retention_minutes"`
}

type ForwarderSettingsInput struct {
	Enabled    *bool  `json:"enabled"`
	URL        string `json:"url"`
	Token      string `json:"token"`
	ClearToken bool   `json:"clear_token"`
	BatchSize  int    `json:"batch_size"`
}

type QueueStatus struct {
	Pending  int `json:"pending"`
	Failed   int `json:"failed"`
	SentHold int `json:"sent_hold"`
}
