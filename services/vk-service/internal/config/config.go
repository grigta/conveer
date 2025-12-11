package config

import (
	"fmt"
	"os"
	"time"

	"conveer/services/vk-service/internal/models"
	"conveer/services/vk-service/internal/service"

	"gopkg.in/yaml.v3"
)

type VKConfig struct {
	Registration   RegistrationConfig   `yaml:"registration"`
	Browser        BrowserConfig        `yaml:"browser"`
	AntiDetection  AntiDetectionConfig  `yaml:"anti_detection"`
	Monitoring     MonitoringConfig     `yaml:"monitoring"`
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

type Config struct {
	VK VKConfig `yaml:"vk"`
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
	c.VK.Registration.MaxRetryAttempts = 3
	c.VK.Registration.RetryBackoffBase = 60
	c.VK.Registration.FormFillDelayMin = 100
	c.VK.Registration.FormFillDelayMax = 500
	c.VK.Registration.SMSWaitTimeout = 300
	c.VK.Registration.PageLoadTimeout = 30
	c.VK.Registration.SMSPollingInterval = 10
	c.VK.Registration.MaxSMSPolls = 30

	c.VK.Browser.PoolSize = 10
	c.VK.Browser.Headless = true
	c.VK.Browser.UserDataDir = "/tmp/vk-profiles"

	c.VK.AntiDetection.EnableStealth = true
	c.VK.AntiDetection.RandomizeFingerprint = true
	c.VK.AntiDetection.MouseEmulation = true

	c.VK.Monitoring.StuckRegistrationTimeout = 30
	c.VK.Monitoring.SessionCleanupInterval = 60
	c.VK.Monitoring.SessionExpiry = 120
}

func (c *Config) overrideFromEnv() {
	// Registration
	if val := getEnvInt("VK_MAX_RETRY_ATTEMPTS"); val > 0 {
		c.VK.Registration.MaxRetryAttempts = val
	}
	if val := getEnvInt("VK_RETRY_BACKOFF_BASE"); val > 0 {
		c.VK.Registration.RetryBackoffBase = val
	}
	if val := getEnvInt("VK_FORM_FILL_DELAY_MIN"); val > 0 {
		c.VK.Registration.FormFillDelayMin = val
	}
	if val := getEnvInt("VK_FORM_FILL_DELAY_MAX"); val > 0 {
		c.VK.Registration.FormFillDelayMax = val
	}
	if val := getEnvInt("VK_SMS_WAIT_TIMEOUT"); val > 0 {
		c.VK.Registration.SMSWaitTimeout = val
	}
	if val := getEnvInt("VK_PAGE_LOAD_TIMEOUT"); val > 0 {
		c.VK.Registration.PageLoadTimeout = val
	}
	if val := getEnvInt("VK_SMS_POLLING_INTERVAL"); val > 0 {
		c.VK.Registration.SMSPollingInterval = val
	}
	if val := getEnvInt("VK_MAX_SMS_POLLS"); val > 0 {
		c.VK.Registration.MaxSMSPolls = val
	}

	// Browser
	if val := getEnvInt("VK_BROWSER_POOL_SIZE"); val > 0 {
		c.VK.Browser.PoolSize = val
	}
	if val := os.Getenv("VK_BROWSER_HEADLESS"); val != "" {
		c.VK.Browser.Headless = val == "true" || val == "1"
	}
	if val := os.Getenv("VK_USER_DATA_DIR"); val != "" {
		c.VK.Browser.UserDataDir = val
	}

	// Anti-detection
	if val := os.Getenv("VK_ENABLE_STEALTH"); val != "" {
		c.VK.AntiDetection.EnableStealth = val == "true" || val == "1"
	}
	if val := os.Getenv("VK_RANDOMIZE_FINGERPRINT"); val != "" {
		c.VK.AntiDetection.RandomizeFingerprint = val == "true" || val == "1"
	}
	if val := os.Getenv("VK_MOUSE_EMULATION"); val != "" {
		c.VK.AntiDetection.MouseEmulation = val == "true" || val == "1"
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
		MaxRetryAttempts:   c.VK.Registration.MaxRetryAttempts,
		RetryBackoffBase:   time.Duration(c.VK.Registration.RetryBackoffBase) * time.Second,
		FormFillDelayMin:   c.VK.Registration.FormFillDelayMin,
		FormFillDelayMax:   c.VK.Registration.FormFillDelayMax,
		SMSWaitTimeout:     time.Duration(c.VK.Registration.SMSWaitTimeout) * time.Second,
		PageLoadTimeout:    time.Duration(c.VK.Registration.PageLoadTimeout) * time.Second,
		SMSPollingInterval: time.Duration(c.VK.Registration.SMSPollingInterval) * time.Second,
		MaxSMSPolls:        c.VK.Registration.MaxSMSPolls,
	}
}

// ToBrowserConfig converts to service.BrowserConfig
func (c *Config) ToBrowserConfig() *service.BrowserConfig {
	return &service.BrowserConfig{
		PoolSize:       c.VK.Browser.PoolSize,
		Headless:       c.VK.Browser.Headless,
		UserDataDir:    c.VK.Browser.UserDataDir,
		DefaultTimeout: time.Duration(c.VK.Registration.PageLoadTimeout) * time.Second,
	}
}