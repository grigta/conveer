package config

import (
	"fmt"
	"os"
	"time"

	"github.com/conveer/conveer/services/max-service/internal/models"
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
	VKService    VKServiceConfig    `yaml:"vk_service"`
	Registration models.RegistrationConfig `yaml:"registration"`
	Browser      BrowserConfig      `yaml:"browser"`
	Encryption   EncryptionConfig   `yaml:"encryption"`
}

// VKServiceConfig represents VK service configuration
type VKServiceConfig struct {
	Address string        `yaml:"address"`
	Timeout time.Duration `yaml:"timeout"`
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
			Name:     "max-service",
			GRPCPort: "50062",
			HTTPPort: "8012",
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
		VKService: VKServiceConfig{
			Address: "vk-service:50059",
			Timeout: 30 * time.Second,
		},
		Registration: models.RegistrationConfig{
			MaxRetryAttempts:      3,
			RetryBackoffBase:      5 * time.Minute,
			FormFillDelayMin:      500,
			FormFillDelayMax:      2000,
			PageLoadTimeout:       30 * time.Second,
			VKLoginTimeout:        2 * time.Minute,
			MaxActivationTimeout:  3 * time.Minute,
			RequireRussianPhone:   true,
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
	if grpcPort := os.Getenv("MAX_SERVICE_GRPC_PORT"); grpcPort != "" {
		config.Service.GRPCPort = grpcPort
	}
	if httpPort := os.Getenv("MAX_SERVICE_HTTP_PORT"); httpPort != "" {
		config.Service.HTTPPort = httpPort
	}
	if vkServiceURL := os.Getenv("VK_SERVICE_GRPC_URL"); vkServiceURL != "" {
		config.VKService.Address = vkServiceURL
	}
	
	return config, nil
}
