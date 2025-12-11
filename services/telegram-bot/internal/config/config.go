package config

import (
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BotToken         string            `yaml:"bot_token" envconfig:"BOT_TOKEN"`
	Mode             string            `yaml:"mode" envconfig:"BOT_MODE" default:"long_polling"`
	WebhookURL       string            `yaml:"webhook_url" envconfig:"WEBHOOK_URL"`
	MongoURI         string            `yaml:"mongo_uri" envconfig:"MONGO_URI" default:"mongodb://localhost:27017"`
	DatabaseName     string            `yaml:"database_name" envconfig:"DATABASE_NAME" default:"conveer"`
	RabbitMQURL      string            `yaml:"rabbitmq_url" envconfig:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
	RedisURL         string            `yaml:"redis_url" envconfig:"REDIS_URL" default:"redis://localhost:6379"`
	LogLevel         string            `yaml:"log_level" envconfig:"LOG_LEVEL" default:"info"`
	AdminTelegramIDs []int64           `yaml:"admin_telegram_ids" envconfig:"ADMIN_TELEGRAM_IDS"`
	EncryptionKey    string            `yaml:"encryption_key" envconfig:"ENCRYPTION_KEY"`
	GRPCServices     map[string]string `yaml:"grpc_services"`
	Features         Features          `yaml:"features"`
}

type Features struct {
	EnableGrafanaIntegration bool   `yaml:"enable_grafana_integration" envconfig:"ENABLE_GRAFANA" default:"false"`
	GrafanaURL               string `yaml:"grafana_url" envconfig:"GRAFANA_URL"`
	GrafanaAPIKey            string `yaml:"grafana_api_key" envconfig:"GRAFANA_API_KEY"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		GRPCServices: make(map[string]string),
	}

	// Try to load from YAML file if path is provided
	if path != "" {
		file, err := os.Open(path)
		if err != nil {
			// File doesn't exist, proceed with env vars only
			fmt.Printf("Config file not found at %s, using environment variables\n", path)
		} else {
			defer file.Close()
			decoder := yaml.NewDecoder(file)
			if err := decoder.Decode(cfg); err != nil {
				return nil, fmt.Errorf("failed to decode config: %w", err)
			}
		}
	}

	// Override with environment variables
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}

	// Load GRPC service URLs from env if not set in config
	if cfg.GRPCServices["vk"] == "" {
		cfg.GRPCServices["vk"] = os.Getenv("VK_SERVICE_URL")
	}
	if cfg.GRPCServices["telegram"] == "" {
		cfg.GRPCServices["telegram"] = os.Getenv("TELEGRAM_SERVICE_URL")
	}
	if cfg.GRPCServices["mail"] == "" {
		cfg.GRPCServices["mail"] = os.Getenv("MAIL_SERVICE_URL")
	}
	if cfg.GRPCServices["max"] == "" {
		cfg.GRPCServices["max"] = os.Getenv("MAX_SERVICE_URL")
	}
	if cfg.GRPCServices["warming"] == "" {
		cfg.GRPCServices["warming"] = os.Getenv("WARMING_SERVICE_URL")
	}
	if cfg.GRPCServices["proxy"] == "" {
		cfg.GRPCServices["proxy"] = os.Getenv("PROXY_SERVICE_URL")
	}
	if cfg.GRPCServices["sms"] == "" {
		cfg.GRPCServices["sms"] = os.Getenv("SMS_SERVICE_URL")
	}
	if cfg.GRPCServices["analytics"] == "" {
		cfg.GRPCServices["analytics"] = os.Getenv("ANALYTICS_SERVICE_URL")
	}

	// Validate required fields
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN is required")
	}
	if cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	if len(cfg.EncryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes")
	}

	return cfg, nil
}