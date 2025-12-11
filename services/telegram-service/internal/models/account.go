package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AccountStatus string

const (
	StatusCreating   AccountStatus = "creating"
	StatusCreated    AccountStatus = "created"
	StatusWarming    AccountStatus = "warming"
	StatusReady      AccountStatus = "ready"
	StatusBanned     AccountStatus = "banned"
	StatusError      AccountStatus = "error"
	StatusSuspended  AccountStatus = "suspended"
)

type TelegramAccount struct {
	ID              primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Phone           string                 `bson:"phone,encrypted" json:"phone,omitempty"`
	Password        string                 `bson:"password,encrypted" json:"-"`
	TwoFactorSecret string                 `bson:"two_factor_secret,encrypted" json:"-"`
	FirstName       string                 `bson:"first_name" json:"first_name"`
	LastName        string                 `bson:"last_name" json:"last_name"`
	Username        string                 `bson:"username" json:"username,omitempty"`
	UserID          string                 `bson:"user_id" json:"user_id,omitempty"`
	Bio             string                 `bson:"bio,omitempty" json:"bio,omitempty"`
	AvatarURL       string                 `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	Status          AccountStatus          `bson:"status" json:"status"`
	ProxyID         primitive.ObjectID     `bson:"proxy_id,omitempty" json:"proxy_id,omitempty"`
	ActivationID    string                 `bson:"activation_id,omitempty" json:"activation_id,omitempty"`
	SessionString   string                 `bson:"session_string,encrypted" json:"-"`
	Cookies         []byte                 `bson:"cookies,encrypted" json:"-"`
	UserAgent       string                 `bson:"user_agent" json:"user_agent,omitempty"`
	Fingerprint     map[string]interface{} `bson:"fingerprint" json:"fingerprint,omitempty"`
	RegistrationIP  string                 `bson:"registration_ip" json:"registration_ip,omitempty"`
	CreatedAt       time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time              `bson:"updated_at" json:"updated_at"`
	LastLoginAt     *time.Time             `bson:"last_login_at,omitempty" json:"last_login_at,omitempty"`
	ErrorMessage    string                 `bson:"error_message,omitempty" json:"error_message,omitempty"`
	RetryCount      int                    `bson:"retry_count" json:"retry_count"`
	ApiID           int                    `bson:"api_id,omitempty" json:"api_id,omitempty"`
	ApiHash         string                 `bson:"api_hash,encrypted" json:"-"`
}

type AccountStatistics struct {
	Total         int64                     `json:"total"`
	ByStatus      map[AccountStatus]int64   `json:"by_status"`
	SuccessRate   float64                   `json:"success_rate"`
	AverageRetries float64                  `json:"average_retries"`
	LastHour      int64                     `json:"last_hour"`
	Last24Hours   int64                     `json:"last_24_hours"`
}

type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires,omitempty"`
	HTTPOnly bool      `json:"httpOnly"`
	Secure   bool      `json:"secure"`
	SameSite string    `json:"sameSite,omitempty"`
}
