package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaxAccount represents a VK Messenger Max account
type MaxAccount struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	VKAccountID     string             `bson:"vk_account_id,omitempty" json:"vk_account_id,omitempty"`
	Phone           string             `bson:"phone,encrypted" json:"phone"`
	Password        string             `bson:"password,encrypted" json:"password"`
	VKUserID        string             `bson:"vk_user_id" json:"vk_user_id"`
	VKAccessToken   string             `bson:"vk_access_token,encrypted" json:"vk_access_token,omitempty"`
	MaxSessionToken string             `bson:"max_session_token,encrypted" json:"max_session_token,omitempty"`
	FirstName       string             `bson:"first_name" json:"first_name"`
	LastName        string             `bson:"last_name" json:"last_name"`
	Username        string             `bson:"username,omitempty" json:"username,omitempty"`
	AvatarURL       string             `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	Status          AccountStatus      `bson:"status" json:"status"`
	ProxyID         string             `bson:"proxy_id,omitempty" json:"proxy_id,omitempty"`
	ActivationID    string             `bson:"activation_id,omitempty" json:"activation_id,omitempty"`
	Cookies         string             `bson:"cookies,encrypted" json:"cookies,omitempty"`
	UserAgent       string             `bson:"user_agent" json:"user_agent"`
	Fingerprint     Fingerprint        `bson:"fingerprint" json:"fingerprint"`
	RegistrationIP  string             `bson:"registration_ip" json:"registration_ip"`
	CreatedAt       time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time          `bson:"updated_at" json:"updated_at"`
	LastLoginAt     *time.Time         `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	ErrorMessage    string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	RetryCount      int                `bson:"retry_count" json:"retry_count"`
	IsVKLinked      bool               `bson:"is_vk_linked" json:"is_vk_linked"`
}

// AccountStatus represents the status of an account
type AccountStatus string

const (
	AccountStatusCreating  AccountStatus = "creating"
	AccountStatusCreated   AccountStatus = "created"
	AccountStatusWarming   AccountStatus = "warming"
	AccountStatusReady     AccountStatus = "ready"
	AccountStatusBanned    AccountStatus = "banned"
	AccountStatusError     AccountStatus = "error"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusFailed    AccountStatus = "failed"
)

// AccountStatistics represents account statistics
type AccountStatistics struct {
	TotalAccounts    int64            `json:"total_accounts"`
	AccountsByStatus map[string]int64 `json:"accounts_by_status"`
	SuccessRate      float64          `json:"success_rate"`
	AverageRetries   float64          `json:"average_retries"`
	VKLinkedCount    int64            `json:"vk_linked_count"`
	LastHour         int64            `json:"created_last_hour"`
	Last24Hours      int64            `json:"created_last_24_hours"`
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure"`
	HTTPOnly bool      `json:"http_only"`
	SameSite string    `json:"same_site"`
}

// Fingerprint represents browser fingerprint
type Fingerprint struct {
	UserAgent      string   `bson:"user_agent" json:"user_agent"`
	ViewportWidth  int      `bson:"viewport_width" json:"viewport_width"`
	ViewportHeight int      `bson:"viewport_height" json:"viewport_height"`
	Timezone       string   `bson:"timezone" json:"timezone"`
	Locale         string   `bson:"locale" json:"locale"`
	Platform       string   `bson:"platform" json:"platform"`
	ScreenWidth    int      `bson:"screen_width" json:"screen_width"`
	ScreenHeight   int      `bson:"screen_height" json:"screen_height"`
	WebGLVendor    string   `bson:"webgl_vendor" json:"webgl_vendor"`
	WebGLRenderer  string   `bson:"webgl_renderer" json:"webgl_renderer"`
	Languages      []string `bson:"languages" json:"languages"`
}
