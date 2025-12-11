package models

import "time"

// ExportFormat represents available export formats
type ExportFormat string

const (
	FormatTData    ExportFormat = "tdata"
	FormatTelethon ExportFormat = "telethon"
	FormatPyrogram ExportFormat = "pyrogram"
	FormatJSON     ExportFormat = "json"
	FormatCSV      ExportFormat = "csv"
)

// Account represents an account for export
type Account struct {
	ID           string                 `json:"id"`
	Platform     string                 `json:"platform"`
	Phone        string                 `json:"phone"`
	Email        string                 `json:"email,omitempty"`
	Password     string                 `json:"password,omitempty"`
	Username     string                 `json:"username,omitempty"`
	UserID       string                 `json:"user_id,omitempty"`
	SessionString string                `json:"session_string,omitempty"`
	Cookies      string                 `json:"cookies,omitempty"`
	LocalStorage string                 `json:"local_storage,omitempty"`
	ProxyID      string                 `json:"proxy_id,omitempty"`
	Status       string                 `json:"status"`
	CreatedAt    time.Time             `json:"created_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ExportRequest represents a request to export accounts
type ExportRequest struct {
	Platform   string       `json:"platform"`
	AccountIDs []string     `json:"account_ids"`
	Format     ExportFormat `json:"format"`
}

// ExportResult represents the result of an export operation
type ExportResult struct {
	Data     []byte `json:"-"` // Binary data (file content)
	Filename string `json:"filename"`
	Format   ExportFormat `json:"format"`
	Count    int    `json:"count"`
}

// SessionData represents session data for Telegram accounts
type SessionData struct {
	AccountID     string `json:"account_id"`
	Phone         string `json:"phone"`
	SessionString string `json:"session_string"`
	Cookies       string `json:"cookies"`
	LocalStorage  string `json:"local_storage"`
	AuthKey       []byte `json:"auth_key,omitempty"`
	DCID          int    `json:"dc_id,omitempty"`
	ServerAddress string `json:"server_address,omitempty"`
	Port          int    `json:"port,omitempty"`
}
