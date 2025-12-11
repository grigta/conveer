package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	ServiceName        string
	GRPCPort           int
	HTTPPort           int
	MongoURI           string
	DatabaseName       string
	RedisURL           string
	RabbitMQURL        string
	LogLevel           string
	VKServiceURL       string
	TelegramServiceURL string
	MailServiceURL     string
	MaxServiceURL      string
	WarmingConfig      WarmingConfig
}

type WarmingConfig struct {
	Scheduler           SchedulerConfig           `yaml:"scheduler"`
	BehaviorSimulation  BehaviorSimulationConfig  `yaml:"behavior_simulation"`
	Scenarios           map[string]ScenarioConfig `yaml:"scenarios"`
	MaxConcurrentTasks  int                       `yaml:"max_concurrent_tasks"`
	EnableAutoStart     bool                      `yaml:"enable_auto_start"`
}

type SchedulerConfig struct {
	CheckInterval      time.Duration `yaml:"check_interval"`
	MaxConcurrentTasks int           `yaml:"max_concurrent_tasks"`
	ActionTimeout      time.Duration `yaml:"action_timeout"`
}

type BehaviorSimulationConfig struct {
	EnableRandomDelays        bool    `yaml:"enable_random_delays"`
	DelayMinSeconds           int     `yaml:"delay_min_seconds"`
	DelayMaxSeconds           int     `yaml:"delay_max_seconds"`
	ActiveHoursStart          int     `yaml:"active_hours_start"`
	ActiveHoursEnd            int     `yaml:"active_hours_end"`
	WeekendActivityReduction  float64 `yaml:"weekend_activity_reduction"`
	NightPauseProbability     float64 `yaml:"night_pause_probability"`
}

type ScenarioConfig map[string]PlatformScenarioConfig

type PlatformScenarioConfig struct {
	Duration14_30 DurationConfig `yaml:"duration_14_30"`
	Duration30_60 DurationConfig `yaml:"duration_30_60"`
}

type DurationConfig struct {
	Days1_7   DayConfig `yaml:"days_1_7"`
	Days8_14  DayConfig `yaml:"days_8_14"`
	Days15_30 DayConfig `yaml:"days_15_30"`
	Days31_60 DayConfig `yaml:"days_31_60"`
}

type DayConfig struct {
	ActionsPerDay string         `yaml:"actions_per_day"`
	Actions       []ActionConfig `yaml:"actions"`
}

type ActionConfig struct {
	Type   string                 `yaml:"type"`
	Weight int                    `yaml:"weight"`
	Params map[string]interface{} `yaml:"params"`
}

func Load() *Config {
	cfg := &Config{
		ServiceName:        getEnv("SERVICE_NAME", "warming-service"),
		GRPCPort:           getEnvAsInt("GRPC_PORT", 50063),
		HTTPPort:           getEnvAsInt("HTTP_PORT", 8013),
		MongoURI:           getEnv("MONGO_URI", "mongodb://root:password@mongodb:27017"),
		DatabaseName:       getEnv("DATABASE_NAME", "conveer"),
		RedisURL:           getEnv("REDIS_URL", "redis://redis:6379"),
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		VKServiceURL:       getEnv("VK_SERVICE_URL", "vk-service:50059"),
		TelegramServiceURL: getEnv("TELEGRAM_SERVICE_URL", "telegram-service:50060"),
		MailServiceURL:     getEnv("MAIL_SERVICE_URL", "mail-service:50061"),
		MaxServiceURL:      getEnv("MAX_SERVICE_URL", "max-service:50062"),
	}

	// Load warming config from YAML file
	configPath := getEnv("WARMING_CONFIG_PATH", "./configs/warming_config.yaml")
	warmingConfig, err := loadWarmingConfig(configPath)
	if err != nil {
		log.Printf("Failed to load warming config from %s, using defaults: %v", configPath, err)
		cfg.WarmingConfig = getDefaultWarmingConfig()
	} else {
		cfg.WarmingConfig = *warmingConfig
	}

	// Override with environment variables if present
	if maxTasks := getEnvAsInt("WARMING_MAX_CONCURRENT_TASKS", 0); maxTasks > 0 {
		cfg.WarmingConfig.MaxConcurrentTasks = maxTasks
	}

	if enableAutoStart := getEnv("WARMING_ENABLE_AUTO_START", ""); enableAutoStart != "" {
		cfg.WarmingConfig.EnableAutoStart = enableAutoStart == "true"
	}

	return cfg
}

func loadWarmingConfig(path string) (*WarmingConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config struct {
		Warming WarmingConfig `yaml:"warming"`
	}

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	// Parse durations
	if config.Warming.Scheduler.CheckInterval == 0 {
		config.Warming.Scheduler.CheckInterval = 5 * time.Minute
	}
	if config.Warming.Scheduler.ActionTimeout == 0 {
		config.Warming.Scheduler.ActionTimeout = 5 * time.Minute
	}

	return &config.Warming, nil
}

func getDefaultWarmingConfig() WarmingConfig {
	return WarmingConfig{
		Scheduler: SchedulerConfig{
			CheckInterval:      5 * time.Minute,
			MaxConcurrentTasks: 50,
			ActionTimeout:      5 * time.Minute,
		},
		BehaviorSimulation: BehaviorSimulationConfig{
			EnableRandomDelays:       true,
			DelayMinSeconds:          30,
			DelayMaxSeconds:          300,
			ActiveHoursStart:         8,
			ActiveHoursEnd:           23,
			WeekendActivityReduction: 0.7,
			NightPauseProbability:    0.9,
		},
		MaxConcurrentTasks: 50,
		EnableAutoStart:    true,
		Scenarios:          make(map[string]ScenarioConfig),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}