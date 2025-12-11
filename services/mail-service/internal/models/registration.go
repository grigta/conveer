package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegistrationStep represents a step in the registration flow
type RegistrationStep string

const (
	StepProxyAllocation   RegistrationStep = "proxy_allocation"
	StepEmailGeneration   RegistrationStep = "email_generation"
	StepFormFilling       RegistrationStep = "form_filling"
	StepPhoneVerification RegistrationStep = "phone_verification"
	StepCaptchaHandling   RegistrationStep = "captcha_handling"
	StepEmailConfirmation RegistrationStep = "email_confirmation"
	StepProfileSetup      RegistrationStep = "profile_setup"
	StepComplete          RegistrationStep = "complete"
)

// RegistrationRequest represents a request to register a new account
type RegistrationRequest struct {
	FirstName              string `json:"first_name" validate:"required"`
	LastName               string `json:"last_name" validate:"required"`
	BirthDate              string `json:"birth_date" validate:"required"`
	Gender                 string `json:"gender" validate:"required,oneof=male female"`
	PreferredCountry       string `json:"preferred_country,omitempty"`
	UsePhoneVerification   bool   `json:"use_phone_verification"`
	CustomEmailPrefix      string `json:"custom_email_prefix,omitempty"`
}

// RegistrationSession represents an active registration session
type RegistrationSession struct {
	ID                   primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	AccountID            primitive.ObjectID     `bson:"account_id" json:"account_id"`
	CurrentStep          RegistrationStep       `bson:"current_step" json:"current_step"`
	ProxyID              string                 `bson:"proxy_id,omitempty" json:"proxy_id"`
	ProxyURL             string                 `bson:"proxy_url,omitempty" json:"proxy_url"`
	Phone                string                 `bson:"phone,omitempty" json:"phone"`
	ActivationID         string                 `bson:"activation_id,omitempty" json:"activation_id"`
	Email                string                 `bson:"email" json:"email"`
	Password             string                 `bson:"password" json:"password"`
	UsePhoneVerification bool                   `bson:"use_phone_verification" json:"use_phone_verification"`
	CaptchaDetected      bool                   `bson:"captcha_detected" json:"captcha_detected"`
	StepCheckpoints      map[string]interface{} `bson:"step_checkpoints" json:"step_checkpoints"`
	RetryCount           int                    `bson:"retry_count" json:"retry_count"`
	StartedAt            time.Time              `bson:"started_at" json:"started_at"`
	LastActivityAt       time.Time              `bson:"last_activity_at" json:"last_activity_at"`
	CompletedAt          *time.Time             `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	ErrorMessage         string                 `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

// RegistrationResult represents the result of a registration attempt
type RegistrationResult struct {
	Success      bool      `json:"success"`
	AccountID    string    `json:"account_id,omitempty"`
	Email        string    `json:"email,omitempty"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Retryable    bool      `json:"retryable"`
	CompletedAt  time.Time `json:"completed_at"`
}

// RegistrationConfig represents configuration for registration
type RegistrationConfig struct {
	MaxRetryAttempts      int           `yaml:"max_retry_attempts"`
	RetryBackoffBase      time.Duration `yaml:"retry_backoff_base"`
	FormFillDelayMin      int           `yaml:"form_fill_delay_min"`
	FormFillDelayMax      int           `yaml:"form_fill_delay_max"`
	SMSWaitTimeout        time.Duration `yaml:"sms_wait_timeout"`
	PageLoadTimeout       time.Duration `yaml:"page_load_timeout"`
	SMSPollingInterval    time.Duration `yaml:"sms_polling_interval"`
	MaxSMSPolls           int           `yaml:"max_sms_polls"`
	EnablePhoneVerification bool        `yaml:"enable_phone_verification"`
	CaptchaTimeout        time.Duration `yaml:"captcha_timeout"`
}
