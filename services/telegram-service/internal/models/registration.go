package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RegistrationStep string

const (
	StepProxyAllocation   RegistrationStep = "proxy_allocation"
	StepPhonePurchase     RegistrationStep = "phone_purchase"
	StepPhoneEntry        RegistrationStep = "phone_entry"
	StepSMSVerification   RegistrationStep = "sms_verification"
	StepProfileSetup      RegistrationStep = "profile_setup"
	StepUsernameSetup     RegistrationStep = "username_setup"
	StepAvatarUpload      RegistrationStep = "avatar_upload"
	StepTwoFactorSetup    RegistrationStep = "two_factor_setup"
	StepComplete          RegistrationStep = "complete"
)

type RegistrationRequest struct {
	FirstName         string    `json:"first_name" validate:"required,min=2,max=50"`
	LastName          string    `json:"last_name,omitempty"`
	Username          string    `json:"username,omitempty"`
	Bio               string    `json:"bio,omitempty"`
	AvatarURL         string    `json:"avatar_url,omitempty"`
	EnableTwoFactor   bool      `json:"enable_two_factor,omitempty"`
	PreferredCountry  string    `json:"preferred_country,omitempty"`
	UseRandomProfile  bool      `json:"use_random_profile,omitempty"`
	ApiID             int       `json:"api_id,omitempty"`
	ApiHash           string    `json:"api_hash,omitempty"`
}

type RegistrationSession struct {
	ID                primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	AccountID         primitive.ObjectID     `bson:"account_id" json:"account_id"`
	CurrentStep       RegistrationStep       `bson:"current_step" json:"current_step"`
	ProxyID           primitive.ObjectID     `bson:"proxy_id,omitempty" json:"proxy_id,omitempty"`
	ProxyURL          string                 `bson:"proxy_url,omitempty" json:"proxy_url,omitempty"`
	Phone             string                 `bson:"phone,omitempty" json:"phone,omitempty"`
	ActivationID      string                 `bson:"activation_id,omitempty" json:"activation_id,omitempty"`
	PhoneCodeHash     string                 `bson:"phone_code_hash,omitempty" json:"phone_code_hash,omitempty"`
	BrowserContext    map[string]interface{} `bson:"browser_context,omitempty" json:"browser_context,omitempty"`
	Cookies           []Cookie               `bson:"cookies,omitempty" json:"cookies,omitempty"`
	SessionString     string                 `bson:"session_string,omitempty" json:"session_string,omitempty"`
	LastError         string                 `bson:"last_error,omitempty" json:"last_error,omitempty"`
	RetryCount        int                    `bson:"retry_count" json:"retry_count"`
	StartedAt         time.Time              `bson:"started_at" json:"started_at"`
	LastActivityAt    time.Time              `bson:"last_activity_at" json:"last_activity_at"`
	CompletedAt       *time.Time             `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	StepCheckpoints   map[string]interface{} `bson:"step_checkpoints,omitempty" json:"step_checkpoints,omitempty"`
	TwoFactorSecret   string                 `bson:"two_factor_secret,omitempty" json:"two_factor_secret,omitempty"`
}

type RegistrationResult struct {
	Success      bool       `json:"success"`
	AccountID    string     `json:"account_id,omitempty"`
	UserID       string     `json:"user_id,omitempty"`
	Phone        string     `json:"phone,omitempty"`
	Username     string     `json:"username,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Step         string     `json:"step,omitempty"`
	Duration     float64    `json:"duration_seconds"`
	RetryCount   int        `json:"retry_count"`
}

type RegistrationConfig struct {
	MaxRetryAttempts    int           `json:"max_retry_attempts"`
	RetryBackoffBase    time.Duration `json:"retry_backoff_base"`
	FormFillDelayMin    int           `json:"form_fill_delay_min"`
	FormFillDelayMax    int           `json:"form_fill_delay_max"`
	SMSWaitTimeout      time.Duration `json:"sms_wait_timeout"`
	PageLoadTimeout     time.Duration `json:"page_load_timeout"`
	SMSPollingInterval  time.Duration `json:"sms_polling_interval"`
	MaxSMSPolls         int           `json:"max_sms_polls"`
	TwoFactorDelay      time.Duration `json:"two_factor_delay"`
}

type ProfileData struct {
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	Username   string    `json:"username"`
	Bio        string    `json:"bio,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
}

// BrowserConfig holds browser pool configuration
type BrowserConfig struct {
	PoolSize       int           `json:"pool_size"`
	Headless       bool          `json:"headless"`
	UserDataDir    string        `json:"user_data_dir"`
	DefaultTimeout time.Duration `json:"default_timeout"`
}