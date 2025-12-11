package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config основная конфигурация сервиса
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Prometheus    PrometheusConfig    `yaml:"prometheus"`
	MongoDB       MongoDBConfig       `yaml:"mongodb"`
	Redis         RedisConfig         `yaml:"redis"`
	RabbitMQ      RabbitMQConfig      `yaml:"rabbitmq"`
	Aggregation   AggregationConfig   `yaml:"aggregation"`
	Forecasting   ForecastingConfig   `yaml:"forecasting"`
	Recommendations RecommendationConfig `yaml:"recommendations"`
	Alerts        AlertsConfig        `yaml:"alerts"`
	Cache         CacheConfig         `yaml:"cache"`
	GRPCServices  map[string]string   `yaml:"grpc_services"`
}

// ServiceConfig конфигурация сервиса
type ServiceConfig struct {
	Name     string `yaml:"name"`
	GRPCPort int    `yaml:"grpc_port"`
	HTTPPort int    `yaml:"http_port"`
}

// PrometheusConfig конфигурация Prometheus
type PrometheusConfig struct {
	URL string `yaml:"url"`
}

// MongoDBConfig конфигурация MongoDB
type MongoDBConfig struct {
	URI        string                   `yaml:"uri"`
	Database   string                   `yaml:"database"`
	Collections MongoCollectionsConfig `yaml:"collections"`
}

// MongoCollectionsConfig конфигурация коллекций MongoDB
type MongoCollectionsConfig struct {
	MetricsTTL         time.Duration `yaml:"metrics_ttl"`
	ForecastsTTL       time.Duration `yaml:"forecasts_ttl"`
	RecommendationsTTL time.Duration `yaml:"recommendations_ttl"`
	AlertsTTL          time.Duration `yaml:"alerts_ttl"`
}

// RedisConfig конфигурация Redis
type RedisConfig struct {
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// RabbitMQConfig конфигурация RabbitMQ
type RabbitMQConfig struct {
	URL      string `yaml:"url"`
	Exchange string `yaml:"exchange"`
}

// AggregationConfig конфигурация агрегации
type AggregationConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// ForecastingConfig конфигурация прогнозирования
type ForecastingConfig struct {
	Interval         time.Duration `yaml:"interval"`
	ExpensePeriods   []string      `yaml:"expense_periods"`
	ConfidenceLevel  float64       `yaml:"confidence_level"`
}

// RecommendationConfig конфигурация рекомендаций
type RecommendationConfig struct {
	Interval           time.Duration            `yaml:"interval"`
	ProxyScoreWeights ProxyScoreWeightsConfig `yaml:"proxy_score_weights"`
}

// ProxyScoreWeightsConfig веса для расчета рейтинга прокси
type ProxyScoreWeightsConfig struct {
	SuccessRate float64 `yaml:"success_rate"`
	BanRate     float64 `yaml:"ban_rate"`
	Latency     float64 `yaml:"latency"`
	Cost        float64 `yaml:"cost"`
}

// AlertsConfig конфигурация алертов
type AlertsConfig struct {
	CheckInterval   time.Duration      `yaml:"check_interval"`
	DefaultCooldown time.Duration      `yaml:"default_cooldown"`
	MonthlyBudget   float64            `yaml:"monthly_budget"`
	BudgetPeriod    time.Duration      `yaml:"budget_period"`
	Rules           []AlertRuleConfig  `yaml:"rules"`
}

// AlertRuleConfig конфигурация правила алерта
type AlertRuleConfig struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	Platform  string                 `yaml:"platform"`
	Threshold AlertThresholdConfig   `yaml:"threshold"`
	Severity  string                 `yaml:"severity"`
	Cooldown  int                    `yaml:"cooldown"`
}

// AlertThresholdConfig конфигурация порога алерта
type AlertThresholdConfig struct {
	Operator string  `yaml:"operator"`
	Value    float64 `yaml:"value"`
}

// CacheConfig конфигурация кэширования
type CacheConfig struct {
	ForecastTTL         time.Duration `yaml:"forecast_ttl"`
	RecommendationsTTL  time.Duration `yaml:"recommendations_ttl"`
}

// Load загружает конфигурацию
func Load(configPath string) (*Config, error) {
	config := &Config{}

	// Загрузка из файла если указан путь
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, err
		}
	}

	// Переопределение из переменных окружения
	loadFromEnv(config)

	// Установка значений по умолчанию
	setDefaults(config)

	return config, nil
}

// loadFromEnv загружает конфигурацию из переменных окружения
func loadFromEnv(config *Config) {
	if val := os.Getenv("SERVICE_NAME"); val != "" {
		config.Service.Name = val
	}

	if val := os.Getenv("GRPC_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.Service.GRPCPort = port
		}
	}

	if val := os.Getenv("HTTP_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.Service.HTTPPort = port
		}
	}

	if val := os.Getenv("PROMETHEUS_URL"); val != "" {
		config.Prometheus.URL = val
	}

	if val := os.Getenv("MONGODB_URI"); val != "" {
		config.MongoDB.URI = val
	}

	if val := os.Getenv("MONGODB_DATABASE"); val != "" {
		config.MongoDB.Database = val
	}

	if val := os.Getenv("REDIS_URL"); val != "" {
		config.Redis.URL = val
	}

	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		config.Redis.Password = val
	}

	if val := os.Getenv("RABBITMQ_URL"); val != "" {
		config.RabbitMQ.URL = val
	}

	// Загрузка gRPC сервисов из переменных окружения
	config.GRPCServices = make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GRPC_SERVICE_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				serviceName := strings.ToLower(strings.TrimPrefix(parts[0], "GRPC_SERVICE_"))
				serviceName = strings.ReplaceAll(serviceName, "_", "-")
				config.GRPCServices[serviceName] = parts[1]
			}
		}
	}
}

// setDefaults устанавливает значения по умолчанию
func setDefaults(config *Config) {
	if config.Service.Name == "" {
		config.Service.Name = "analytics-service"
	}

	if config.Service.GRPCPort == 0 {
		config.Service.GRPCPort = 50056
	}

	if config.Service.HTTPPort == 0 {
		config.Service.HTTPPort = 8014
	}

	if config.Prometheus.URL == "" {
		config.Prometheus.URL = "http://prometheus:9090"
	}

	if config.MongoDB.URI == "" {
		config.MongoDB.URI = "mongodb://admin:admin123@mongodb:27017/conveer?authSource=admin"
	}

	if config.MongoDB.Database == "" {
		config.MongoDB.Database = "conveer"
	}

	if config.Redis.URL == "" {
		config.Redis.URL = "redis:6379"
	}

	if config.RabbitMQ.URL == "" {
		config.RabbitMQ.URL = "amqp://guest:guest@rabbitmq:5672/"
	}

	if config.RabbitMQ.Exchange == "" {
		config.RabbitMQ.Exchange = "bot.events"
	}

	if config.Aggregation.Interval == 0 {
		config.Aggregation.Interval = 5 * time.Minute
	}

	if config.Forecasting.Interval == 0 {
		config.Forecasting.Interval = 1 * time.Hour
	}

	if len(config.Forecasting.ExpensePeriods) == 0 {
		config.Forecasting.ExpensePeriods = []string{"7d", "30d"}
	}

	if config.Forecasting.ConfidenceLevel == 0 {
		config.Forecasting.ConfidenceLevel = 0.95
	}

	if config.Recommendations.Interval == 0 {
		config.Recommendations.Interval = 6 * time.Hour
	}

	if config.Alerts.CheckInterval == 0 {
		config.Alerts.CheckInterval = 1 * time.Minute
	}

	if config.Alerts.DefaultCooldown == 0 {
		config.Alerts.DefaultCooldown = 30 * time.Minute
	}

	if config.Alerts.MonthlyBudget == 0 {
		config.Alerts.MonthlyBudget = 10000.0 // Default to $10,000
	}

	if config.Alerts.BudgetPeriod == 0 {
		config.Alerts.BudgetPeriod = 30 * 24 * time.Hour // Default to 30 days
	}

	if config.Cache.ForecastTTL == 0 {
		config.Cache.ForecastTTL = 1 * time.Hour
	}

	if config.Cache.RecommendationsTTL == 0 {
		config.Cache.RecommendationsTTL = 6 * time.Hour
	}

	// Установка gRPC сервисов по умолчанию
	if config.GRPCServices == nil {
		config.GRPCServices = make(map[string]string)
	}

	defaultServices := map[string]string{
		"vk-service":      "vk-service:50051",
		"telegram-service": "telegram-service:50052",
		"mail-service":    "mail-service:50053",
		"max-service":     "max-service:50054",
		"warming-service": "warming-service:50055",
		"proxy-service":   "proxy-service:50057",
		"sms-service":     "sms-service:50058",
	}

	for service, address := range defaultServices {
		if _, exists := config.GRPCServices[service]; !exists {
			config.GRPCServices[service] = address
		}
	}
}