package config

import (
	"fmt"
	"os"
	"time"

	"conveer/services/telegram-service/internal/models"
	"conveer/services/telegram-service/internal/service"

	"gopkg.in/yaml.v3"
)

type TelegramConfig struct {
	Registration   RegistrationConfig   `yaml:"registration"`
	Browser        BrowserConfig        `yaml:"browser"`
	AntiDetection  AntiDetectionConfig  `yaml:"anti_detection"`
	Monitoring     MonitoringConfig     `yaml:"monitoring"`
	API            APIConfig            `yaml:"api"`
}

type RegistrationConfig struct {
	MaxRetryAttempts   int `yaml:"max_retry_attempts"`
	RetryBackoffBase   int `yaml:"retry_backoff_base"`     // seconds
	FormFillDelayMin   int `yaml:"form_fill_delay_min"`     // ms
	FormFillDelayMax   int `yaml:"form_fill_delay_max"`     // ms
	SMSWaitTimeout     int `yaml:"sms_wait_timeout"`        // seconds
	PageLoadTimeout    int `yaml:"page_load_timeout"`       // seconds
	SMSPollingInterval int `yaml:"sms_polling_interval"`    // seconds
	MaxSMSPolls        int `yaml:"max_sms_polls"`
	TwoFactorDelay     int `yaml:"two_factor_delay"`        // seconds
}

type BrowserConfig struct {
	PoolSize     int    `yaml:"pool_size"`
	Headless     bool   `yaml:"headless"`
	UserDataDir  string `yaml:"user_data_dir"`
}

type AntiDetectionConfig struct {
	EnableStealth         bool `yaml:"enable_stealth"`
	RandomizeFingerprint  bool `yaml:"randomize_fingerprint"`
	MouseEmulation        bool `yaml:"mouse_emulation"`
}

type MonitoringConfig struct {
	StuckRegistrationTimeout int `yaml:"stuck_registration_timeout"`  // minutes
	SessionCleanupInterval   int `yaml:"session_cleanup_interval"`    // minutes
	SessionExpiry            int `yaml:"session_expiry"`              // minutes
}

type APIConfig struct {
	DefaultAPIID   int    `yaml:"default_api_id"`
	DefaultAPIHash string `yaml:"default_api_hash"`
	WebURL         string `yaml:"web_url"`
}

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
}

// LoadConfig loads configuration from YAML file and merges with environment variables
func LoadConfig(configPath string) (*Config, error) {
	var config Config

	// Set defaults
	config.setDefaults()

	// Load from file if it exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			// If file doesn't exist, just use defaults
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, &config); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables if set
	config.overrideFromEnv()

	return &config, nil
}

func (c *Config) setDefaults() {
	c.Telegram.Registration.MaxRetryAttempts = 3
	c.Telegram.Registration.RetryBackoffBase = 60
	c.Telegram.Registration.FormFillDelayMin = 100
	c.Telegram.Registration.FormFillDelayMax = 500
	c.Telegram.Registration.SMSWaitTimeout = 300
	c.Telegram.Registration.PageLoadTimeout = 30
	c.Telegram.Registration.SMSPollingInterval = 10
	c.Telegram.Registration.MaxSMSPolls = 30
	c.Telegram.Registration.TwoFactorDelay = 5

	c.Telegram.Browser.PoolSize = 10
	c.Telegram.Browser.Headless = true
	c.Telegram.Browser.UserDataDir = "/tmp/telegram-profiles"

	c.Telegram.AntiDetection.EnableStealth = true
	c.Telegram.AntiDetection.RandomizeFingerprint = true
	c.Telegram.AntiDetection.MouseEmulation = true

	c.Telegram.Monitoring.StuckRegistrationTimeout = 30
	c.Telegram.Monitoring.SessionCleanupInterval = 60
	c.Telegram.Monitoring.SessionExpiry = 120

	c.Telegram.API.WebURL = "https://web.telegram.org/k/"
}

func (c *Config) overrideFromEnv() {
	// Registration
	if val := getEnvInt("TELEGRAM_MAX_RETRY_ATTEMPTS"); val > 0 {
		c.Telegram.Registration.MaxRetryAttempts = val
	}
	if val := getEnvInt("TELEGRAM_RETRY_BACKOFF_BASE"); val > 0 {
		c.Telegram.Registration.RetryBackoffBase = val
	}
	if val := getEnvInt("TELEGRAM_FORM_FILL_DELAY_MIN"); val > 0 {
		c.Telegram.Registration.FormFillDelayMin = val
	}
	if val := getEnvInt("TELEGRAM_FORM_FILL_DELAY_MAX"); val > 0 {
		c.Telegram.Registration.FormFillDelayMax = val
	}
	if val := getEnvInt("TELEGRAM_SMS_WAIT_TIMEOUT"); val > 0 {
		c.Telegram.Registration.SMSWaitTimeout = val
	}
	if val := getEnvInt("TELEGRAM_PAGE_LOAD_TIMEOUT"); val > 0 {
		c.Telegram.Registration.PageLoadTimeout = val
	}
	if val := getEnvInt("TELEGRAM_SMS_POLLING_INTERVAL"); val > 0 {
		c.Telegram.Registration.SMSPollingInterval = val
	}
	if val := getEnvInt("TELEGRAM_MAX_SMS_POLLS"); val > 0 {
		c.Telegram.Registration.MaxSMSPolls = val
	}
	if val := getEnvInt("TELEGRAM_TWO_FACTOR_DELAY"); val > 0 {
		c.Telegram.Registration.TwoFactorDelay = val
	}

	// Browser
	if val := getEnvInt("TELEGRAM_BROWSER_POOL_SIZE"); val > 0 {
		c.Telegram.Browser.PoolSize = val
	}
	if val := os.Getenv("TELEGRAM_BROWSER_HEADLESS"); val != "" {
		c.Telegram.Browser.Headless = val == "true" || val == "1"
	}
	if val := os.Getenv("TELEGRAM_USER_DATA_DIR"); val != "" {
		c.Telegram.Browser.UserDataDir = val
	}

	// Anti-detection
	if val := os.Getenv("TELEGRAM_ENABLE_STEALTH"); val != "" {
		c.Telegram.AntiDetection.EnableStealth = val == "true" || val == "1"
	}
	if val := os.Getenv("TELEGRAM_RANDOMIZE_FINGERPRINT"); val != "" {
		c.Telegram.AntiDetection.RandomizeFingerprint = val == "true" || val == "1"
	}
	if val := os.Getenv("TELEGRAM_MOUSE_EMULATION"); val != "" {
		c.Telegram.AntiDetection.MouseEmulation = val == "true" || val == "1"
	}

	// API
	if val := getEnvInt("TELEGRAM_DEFAULT_API_ID"); val > 0 {
		c.Telegram.API.DefaultAPIID = val
	}
	if val := os.Getenv("TELEGRAM_DEFAULT_API_HASH"); val != "" {
		c.Telegram.API.DefaultAPIHash = val
	}
	if val := os.Getenv("TELEGRAM_WEB_URL"); val != "" {
		c.Telegram.API.WebURL = val
	}
}

func getEnvInt(key string) int {
	if val := os.Getenv(key); val != "" {
		var intVal int
		fmt.Sscanf(val, "%d", &intVal)
		return intVal
	}
	return 0
}

// ToRegistrationConfig converts to models.RegistrationConfig
func (c *Config) ToRegistrationConfig() *models.RegistrationConfig {
	return &models.RegistrationConfig{
		MaxRetryAttempts:   c.Telegram.Registration.MaxRetryAttempts,
		RetryBackoffBase:   time.Duration(c.Telegram.Registration.RetryBackoffBase) * time.Second,
		FormFillDelayMin:   c.Telegram.Registration.FormFillDelayMin,
		FormFillDelayMax:   c.Telegram.Registration.FormFillDelayMax,
		SMSWaitTimeout:     time.Duration(c.Telegram.Registration.SMSWaitTimeout) * time.Second,
		PageLoadTimeout:    time.Duration(c.Telegram.Registration.PageLoadTimeout) * time.Second,
		SMSPollingInterval: time.Duration(c.Telegram.Registration.SMSPollingInterval) * time.Second,
		MaxSMSPolls:        c.Telegram.Registration.MaxSMSPolls,
		TwoFactorDelay:     time.Duration(c.Telegram.Registration.TwoFactorDelay) * time.Second,
	}
}

// ToBrowserConfig converts to service.BrowserConfig
func (c *Config) ToBrowserConfig() *service.BrowserConfig {
	return &service.BrowserConfig{
		PoolSize:       c.Telegram.Browser.PoolSize,
		Headless:       c.Telegram.Browser.Headless,
		UserDataDir:    c.Telegram.Browser.UserDataDir,
		DefaultTimeout: time.Duration(c.Telegram.Registration.PageLoadTimeout) * time.Second,
	}
}