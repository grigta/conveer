package config

import (
	"fmt"
	"os"
	"time"

	"github.com/grigta/conveer/services/mail-service/internal/models"
	"gopkg.in/yaml.v3"
)

// Config represents the service configuration
type Config struct {
	Service      ServiceConfig      `yaml:"service"`
	MongoDB      MongoDBConfig      `yaml:"mongodb"`
	Redis        RedisConfig        `yaml:"redis"`
	RabbitMQ     RabbitMQConfig     `yaml:"rabbitmq"`
	ProxyService ProxyServiceConfig `yaml:"proxy_service"`
	SMSService   SMSServiceConfig   `yaml:"sms_service"`
	Registration models.RegistrationConfig `yaml:"registration"`
	Browser      BrowserConfig      `yaml:"browser"`
	Encryption   EncryptionConfig   `yaml:"encryption"`
}

// ServiceConfig represents service configuration
type ServiceConfig struct {
	Name     string `yaml:"name"`
	GRPCPort string `yaml:"grpc_port"`
	HTTPPort string `yaml:"http_port"`
}

// MongoDBConfig represents MongoDB configuration
type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// RabbitMQConfig represents RabbitMQ configuration
type RabbitMQConfig struct {
	URL string `yaml:"url"`
}

// ProxyServiceConfig represents proxy service configuration
type ProxyServiceConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
}

// SMSServiceConfig represents SMS service configuration
type SMSServiceConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
}

// BrowserConfig represents browser configuration
type BrowserConfig struct {
	PoolSize       int           `yaml:"pool_size"`
	Headless       bool          `yaml:"headless"`
	Timeout        time.Duration `yaml:"timeout"`
	ViewportWidth  int           `yaml:"viewport_width"`
	ViewportHeight int           `yaml:"viewport_height"`
}

// EncryptionConfig represents encryption configuration
type EncryptionConfig struct {
	Key string `yaml:"key"`
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	// Set defaults
	config := &Config{
		Service: ServiceConfig{
			Name:     "mail-service",
			GRPCPort: "50061",
			HTTPPort: "8011",
		},
		MongoDB: MongoDBConfig{
			URI:      os.Getenv("MONGODB_URI"),
			Database: os.Getenv("MONGODB_DATABASE"),
		},
		Redis: RedisConfig{
			Address:  os.Getenv("REDIS_ADDRESS"),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       0,
		},
		RabbitMQ: RabbitMQConfig{
			URL: os.Getenv("RABBITMQ_URL"),
		},
		ProxyService: ProxyServiceConfig{
			Address: "proxy-service:50057",
			Timeout: 30 * time.Second,
		},
		SMSService: SMSServiceConfig{
			Address: "sms-service:50058",
			Timeout: 30 * time.Second,
		},
		Registration: models.RegistrationConfig{
			MaxRetryAttempts:      3,
			RetryBackoffBase:      5 * time.Minute,
			FormFillDelayMin:      500,
			FormFillDelayMax:      2000,
			SMSWaitTimeout:        5 * time.Minute,
			PageLoadTimeout:       30 * time.Second,
			SMSPollingInterval:    10 * time.Second,
			MaxSMSPolls:           30,
			EnablePhoneVerification: true,
			CaptchaTimeout:        10 * time.Minute,
		},
		Browser: BrowserConfig{
			PoolSize:       10,
			Headless:       true,
			Timeout:        30 * time.Second,
			ViewportWidth:  1920,
			ViewportHeight: 1080,
		},
		Encryption: EncryptionConfig{
			Key: os.Getenv("ENCRYPTION_KEY"),
		},
	}
	
	// Load from file if exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}
	
	// Override with environment variables
	if grpcPort := os.Getenv("MAIL_SERVICE_GRPC_PORT"); grpcPort != "" {
		config.Service.GRPCPort = grpcPort
	}
	if httpPort := os.Getenv("MAIL_SERVICE_HTTP_PORT"); httpPort != "" {
		config.Service.HTTPPort = httpPort
	}
	
	return config, nil
}
