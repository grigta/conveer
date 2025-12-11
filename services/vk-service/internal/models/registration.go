package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

type RegistrationStep string

const (
	StepProxyAllocation   RegistrationStep = "proxy_allocation"
	StepPhonePurchase     RegistrationStep = "phone_purchase"
	StepFormFilling       RegistrationStep = "form_filling"
	StepSMSVerification   RegistrationStep = "sms_verification"
	StepProfileSetup      RegistrationStep = "profile_setup"
	StepComplete          RegistrationStep = "complete"
)

type RegistrationRequest struct {
	FirstName         string    `json:"first_name" validate:"required,min=2,max=50"`
	LastName          string    `json:"last_name" validate:"required,min=2,max=50"`
	BirthDate         time.Time `json:"birth_date,omitempty"`
	Gender            Gender    `json:"gender,omitempty"`
	PreferredCountry  string    `json:"preferred_country,omitempty"`
	UseRandomProfile  bool      `json:"use_random_profile,omitempty"`
}

type RegistrationSession struct {
	ID                primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	AccountID         primitive.ObjectID     `bson:"account_id" json:"account_id"`
	CurrentStep       RegistrationStep       `bson:"current_step" json:"current_step"`
	ProxyID           primitive.ObjectID     `bson:"proxy_id,omitempty" json:"proxy_id,omitempty"`
	ProxyURL          string                 `bson:"proxy_url,omitempty" json:"proxy_url,omitempty"`
	Phone             string                 `bson:"phone,omitempty" json:"phone,omitempty"`
	ActivationID      string                 `bson:"activation_id,omitempty" json:"activation_id,omitempty"`
	BrowserContext    map[string]interface{} `bson:"browser_context,omitempty" json:"browser_context,omitempty"`
	Cookies           []Cookie               `bson:"cookies,omitempty" json:"cookies,omitempty"`
	LastError         string                 `bson:"last_error,omitempty" json:"last_error,omitempty"`
	RetryCount        int                    `bson:"retry_count" json:"retry_count"`
	StartedAt         time.Time              `bson:"started_at" json:"started_at"`
	LastActivityAt    time.Time              `bson:"last_activity_at" json:"last_activity_at"`
	CompletedAt       *time.Time             `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	StepCheckpoints   map[string]interface{} `bson:"step_checkpoints,omitempty" json:"step_checkpoints,omitempty"`
}

type RegistrationResult struct {
	Success      bool       `json:"success"`
	AccountID    string     `json:"account_id,omitempty"`
	UserID       string     `json:"user_id,omitempty"`
	Phone        string     `json:"phone,omitempty"`
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
}

type ProfileData struct {
	FirstName  string    `json:"first_name"`
	LastName   string    `json:"last_name"`
	BirthDate  time.Time `json:"birth_date"`
	Gender     Gender    `json:"gender"`
	City       string    `json:"city,omitempty"`
	About      string    `json:"about,omitempty"`
	Interests  []string  `json:"interests,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
}
