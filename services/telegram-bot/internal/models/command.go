package models

import "time"

// Command represents a command to be sent via RabbitMQ
type Command struct {
	Command     string                 `json:"command"`
	Platform    string                 `json:"platform,omitempty"`
	Params      map[string]interface{} `json:"params"`
	InitiatedBy string                 `json:"initiated_by"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Event represents an event received from RabbitMQ
type Event struct {
	Type        string                 `json:"type"`
	Platform    string                 `json:"platform,omitempty"`
	AccountID   string                 `json:"account_id,omitempty"`
	TaskID      string                 `json:"task_id,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Priority    string                 `json:"priority,omitempty"` // critical, warning, info
	Timestamp   time.Time              `json:"timestamp"`
}

// Registration command types
type RegisterCommand struct {
	Count       int    `json:"count"`
	InitiatedBy string `json:"initiated_by"`
}

// Warming command types
type WarmingCommand struct {
	AccountID     string `json:"account_id"`
	Platform      string `json:"platform"`
	Scenario      string `json:"scenario"`
	DurationDays  int    `json:"duration_days"`
	InitiatedBy   string `json:"initiated_by"`
}

// Proxy command types
type ProxyCommand struct {
	AccountID   string `json:"account_id"`
	Type        string `json:"type"` // mobile, residential
	Action      string `json:"action"` // allocate, release
	InitiatedBy string `json:"initiated_by"`
}

// SMS command types
type SMSCommand struct {
	Service     string `json:"service"` // vk, telegram, mail, max
	Country     string `json:"country"`
	Action      string `json:"action"` // purchase, cancel
	ActivationID string `json:"activation_id,omitempty"`
	InitiatedBy string `json:"initiated_by"`
}
