// Made by YTSworks
// YTS工作室製作
package accesscontrol

import "time"

type Door struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	Location     string     `json:"location"`
	IPAddress    string     `json:"ip_address"`
	Port         int        `json:"port"`
	Username     string     `json:"username"`
	Manufacturer string     `json:"manufacturer"`
	Model        string     `json:"model"`
	Protocol     string     `json:"protocol"`
	IsEnabled    bool       `json:"is_enabled"`
	Status       string     `json:"status"`
	LastSeen     *time.Time `json:"last_seen"`
	CreatedAt    time.Time  `json:"created_at"`
}

type DoorRequest struct {
	Name         string `json:"name" binding:"required"`
	Location     string `json:"location"`
	IPAddress    string `json:"ip_address"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Protocol     string `json:"protocol"`
}

type Card struct {
	ID         int       `json:"id"`
	CardNumber string    `json:"card_number"`
	HolderName string    `json:"holder_name"`
	Department string    `json:"department"`
	IsActive   bool      `json:"is_active"`
	ValidFrom  *string   `json:"valid_from"`
	ValidUntil *string   `json:"valid_until"`
	CreatedAt  time.Time `json:"created_at"`
}

type CardRequest struct {
	CardNumber string `json:"card_number" binding:"required"`
	HolderName string `json:"holder_name" binding:"required"`
	Department string `json:"department"`
	IsActive   *bool  `json:"is_active"`
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
}

type Event struct {
	ID         int       `json:"id"`
	DoorID     *int      `json:"door_id"`
	DoorName   string    `json:"door_name"`
	CardNumber string    `json:"card_number"`
	HolderName string    `json:"holder_name"`
	EventType  string    `json:"event_type"`
	OccurredAt time.Time `json:"occurred_at"`
}

type EventCreateRequest struct {
	DoorID     *int   `json:"door_id"`
	CardNumber string `json:"card_number"`
	HolderName string `json:"holder_name"`
	EventType  string `json:"event_type" binding:"required"`
}

type ModuleStatus struct {
	Enabled   bool `json:"enabled"`
	DoorCount int  `json:"door_count"`
	CardCount int  `json:"card_count"`
}

type Schedule struct {
	ID         int    `json:"id"`
	CardID     *int   `json:"card_id"`
	CardNumber string `json:"card_number"`
	DoorID     *int   `json:"door_id"`
	DoorName   string `json:"door_name"`
	HolderName string `json:"holder_name"`
	Department string `json:"department"`
	AllowDays  string `json:"allow_days"`
	TimeFrom   string `json:"time_from"`
	TimeUntil  string `json:"time_until"`
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
	Status     string `json:"status"`
	Note       string `json:"note"`
	CreatedAt  string `json:"created_at"`
	ApprovedAt string `json:"approved_at"`
	ApprovedBy string `json:"approved_by"`
}

type ScheduleRequest struct {
	CardNumber string `json:"card_number" binding:"required"`
	DoorID     *int   `json:"door_id"`
	HolderName string `json:"holder_name"`
	Department string `json:"department"`
	AllowDays  string `json:"allow_days"`
	TimeFrom   string `json:"time_from"`
	TimeUntil  string `json:"time_until"`
	ValidFrom  string `json:"valid_from" binding:"required"`
	ValidUntil string `json:"valid_until" binding:"required"`
	Note       string `json:"note"`
}

type EventsQuery struct {
	DoorID     string
	CardNumber string
	EventType  string
	Limit      int
}

type ControlDoorResult struct {
	Action string `json:"action"`
	Door   string `json:"door"`
}
