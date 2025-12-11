package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegistrationStep represents a step in the registration flow
type RegistrationStep string

const (
	StepProxyAllocation   RegistrationStep = "proxy_allocation"
	StepVKAccountCheck    RegistrationStep = "vk_account_check"
	StepVKRegistration    RegistrationStep = "vk_registration"
	StepVKLogin           RegistrationStep = "vk_login"
	StepMaxActivation     RegistrationStep = "max_activation"
	StepMaxProfileSetup   RegistrationStep = "max_profile_setup"
	StepComplete          RegistrationStep = "complete"
)

// RegistrationRequest represents a request to register a new account
type RegistrationRequest struct {
	VKAccountID         string `json:"vk_account_id,omitempty"`
	FirstName           string `json:"first_name" validate:"required"`
	LastName            string `json:"last_name" validate:"required"`
	Username            string `json:"username,omitempty"`
	AvatarURL           string `json:"avatar_url,omitempty"`
	PreferredCountry    string `json:"preferred_country,omitempty"`
	CreateNewVKAccount  bool   `json:"create_new_vk_account"`
}

// RegistrationSession represents an active registration session
type RegistrationSession struct {
	ID                 primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	AccountID          primitive.ObjectID     `bson:"account_id" json:"account_id"`
	CurrentStep        RegistrationStep       `bson:"current_step" json:"current_step"`
	VKAccountID        string                 `bson:"vk_account_id,omitempty" json:"vk_account_id"`
	CreateNewVKAccount bool                   `bson:"create_new_vk_account" json:"create_new_vk_account"`
	ProxyID            string                 `bson:"proxy_id,omitempty" json:"proxy_id"`
	ProxyURL           string                 `bson:"proxy_url,omitempty" json:"proxy_url"`
	Phone              string                 `bson:"phone,omitempty" json:"phone"`
	ActivationID       string                 `bson:"activation_id,omitempty" json:"activation_id"`
	VKUserID           string                 `bson:"vk_user_id,omitempty" json:"vk_user_id"`
	VKAccessToken      string                 `bson:"vk_access_token,omitempty" json:"vk_access_token"`
	MaxSessionToken    string                 `bson:"max_session_token,omitempty" json:"max_session_token"`
	StepCheckpoints    map[string]interface{} `bson:"step_checkpoints" json:"step_checkpoints"`
	RetryCount         int                    `bson:"retry_count" json:"retry_count"`
	StartedAt          time.Time              `bson:"started_at" json:"started_at"`
	LastActivityAt     time.Time              `bson:"last_activity_at" json:"last_activity_at"`
	CompletedAt        *time.Time             `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	ErrorMessage     string                 `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

// RegistrationResult represents the result of a registration attempt
type RegistrationResult struct {
	Success         bool      `json:"success"`
	AccountID       string    `json:"account_id,omitempty"`
	VKUserID        string    `json:"vk_user_id,omitempty"`
	MaxSessionToken string    `json:"max_session_token,omitempty"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	Retryable       bool      `json:"retryable"`
	CompletedAt     time.Time `json:"completed_at"`
}

// RegistrationConfig represents configuration for registration
type RegistrationConfig struct {
	MaxRetryAttempts      int           `yaml:"max_retry_attempts"`
	RetryBackoffBase      time.Duration `yaml:"retry_backoff_base"`
	FormFillDelayMin      int           `yaml:"form_fill_delay_min"`
	FormFillDelayMax      int           `yaml:"form_fill_delay_max"`
	PageLoadTimeout       time.Duration `yaml:"page_load_timeout"`
	VKLoginTimeout        time.Duration `yaml:"vk_login_timeout"`
	MaxActivationTimeout  time.Duration `yaml:"max_activation_timeout"`
	RequireRussianPhone   bool          `yaml:"require_russian_phone"`
}
