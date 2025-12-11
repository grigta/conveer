package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App          AppConfig
	Database     DatabaseConfigNew
	// Legacy/top-level compatibility (config.yaml uses `redis.*` and `rabbitmq.*`)
	Redis        RedisConfig
	RabbitMQ     RabbitMQConfig
	Cache        CacheConfig
	MessageQueue MessageQueueConfig
	Crypto       CryptoConfig
	Services     ServicesConfig
	Monitoring   MonitoringConfig
	RateLimit    RateLimitConfig
	Proxy        ProxyConfig
	JWT          JWTConfig
	Encryption   EncryptionConfig
	SMS          SMSConfig
}

type AppConfig struct {
	Env      string
	Port     int
	Debug    bool
	LogLevel string
}

type DatabaseConfig struct {
	URI    string
	DBName string
}

type DatabaseConfigNew struct {
	// Legacy/top-level compatibility (config.yaml uses `database.uri` and `database.dbname`)
	URI    string
	DBName string
	MongoDB MongoDBConfig
}

type MongoDBConfig struct {
	URI    string
	DBName string
}

type CacheConfig struct {
	Redis RedisConfig
}

type RedisConfig struct {
	Addr     string
	Host     string
	Port     int
	Password string
	DB       int
}

type MessageQueueConfig struct {
	RabbitMQ RabbitMQConfig
}

type RabbitMQConfig struct {
	URL string
}

type CryptoConfig struct {
	EncryptionKey string
}

type JWTConfig struct {
	Secret    string
	ExpiresIn time.Duration
}

type EncryptionConfig struct {
	Key string
}

type ServicesConfig struct {
	AuthServiceURL         string
	UserServiceURL         string
	ProductServiceURL      string
	OrderServiceURL        string
	NotificationServiceURL string
	AnalyticsServiceURL    string
	ProxyServiceURL        string
	SMSServiceURL          string
	VKServiceURL           string
	TelegramServiceURL     string
	MailServiceURL         string
	MaxServiceURL          string
	WarmingServiceURL      string
}

type SMSConfig struct {
	ProviderConfigPath string
	MaxRetryAttempts   int
	CodeWaitTimeout    string
	ActivationExpiry   string
}

type ProxyConfig struct {
	HealthCheckInterval   string
	RotationCheckInterval string
	MaxFailedChecks       int
	IPQualityScoreAPIKey  string
	ProviderConfigPath    string
}

type MonitoringConfig struct {
	PrometheusPort int
	GrafanaPort    int
}

type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Window   time.Duration
}

func LoadConfig() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("CONVEER")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Printf("error reading config file: %v\n", err)
		}
	}

	setDefaults()
	bindEnvVariables()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		fmt.Printf("unable to decode into struct: %v\n", err)
		return getDefaultConfig()
	}

	return &config
}

// Совместимость со старой версией
func LoadConfigOld(path string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("CONVEER")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	setDefaults()
	bindEnvVariables()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct: %w", err)
	}

	return &config, nil
}

func getDefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfigNew{
			URI:    "mongodb://localhost:27017",
			DBName: "conveer",
			MongoDB: MongoDBConfig{
				URI:    "mongodb://localhost:27017",
				DBName: "conveer",
			},
		},
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       0,
		},
		RabbitMQ: RabbitMQConfig{
			URL: "amqp://guest:guest@localhost:5672/",
		},
		Cache: CacheConfig{
			Redis: RedisConfig{
				Addr:     "localhost:6379",
				Host:     "localhost",
				Port:     6379,
				Password: "",
				DB:       0,
			},
		},
		MessageQueue: MessageQueueConfig{
			RabbitMQ: RabbitMQConfig{
				URL: "amqp://guest:guest@localhost:5672/",
			},
		},
		Crypto: CryptoConfig{
			EncryptionKey: "",
		},
		Proxy: ProxyConfig{
			HealthCheckInterval:   "15m",
			RotationCheckInterval: "5m",
			MaxFailedChecks:       3,
			IPQualityScoreAPIKey:  "",
			ProviderConfigPath:    "./configs/providers.yaml",
		},
	}
}

func setDefaults() {
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("app.debug", false)
	viper.SetDefault("app.loglevel", "info")

	viper.SetDefault("database.uri", "mongodb://localhost:27017")
	viper.SetDefault("database.dbname", "conveer")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("rabbitmq.url", "amqp://guest:guest@localhost:5672/")

	viper.SetDefault("jwt.expiresin", "24h")

	viper.SetDefault("monitoring.prometheusport", 9090)
	viper.SetDefault("monitoring.grafanaport", 3000)

	viper.SetDefault("ratelimit.enabled", true)
	viper.SetDefault("ratelimit.requests", 100)
	viper.SetDefault("ratelimit.window", "60s")

	viper.SetDefault("sms.providerconfigpath", "./configs/providers.yaml")
	viper.SetDefault("sms.maxretryattempts", 3)
	viper.SetDefault("sms.codewaittimeout", "5m")
	viper.SetDefault("sms.activationexpiry", "30m")
}

func bindEnvVariables() {
	viper.BindEnv("app.env", "APP_ENV")
	viper.BindEnv("app.port", "APP_PORT")
	viper.BindEnv("app.debug", "APP_DEBUG")
	viper.BindEnv("app.loglevel", "LOG_LEVEL")

	viper.BindEnv("database.uri", "MONGO_URI")
	viper.BindEnv("database.dbname", "MONGO_DB_NAME")

	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_PORT")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")

	viper.BindEnv("rabbitmq.url", "RABBITMQ_URL")

	viper.BindEnv("jwt.secret", "JWT_SECRET")
	viper.BindEnv("jwt.expiresin", "JWT_EXPIRES_IN")

	viper.BindEnv("encryption.key", "ENCRYPTION_KEY")

	viper.BindEnv("services.authserviceurl", "AUTH_SERVICE_URL")
	viper.BindEnv("services.userserviceurl", "USER_SERVICE_URL")
	viper.BindEnv("services.productserviceurl", "PRODUCT_SERVICE_URL")
	viper.BindEnv("services.orderserviceurl", "ORDER_SERVICE_URL")
	viper.BindEnv("services.notificationserviceurl", "NOTIFICATION_SERVICE_URL")
	viper.BindEnv("services.analyticsserviceurl", "ANALYTICS_SERVICE_URL")
	viper.BindEnv("services.proxyserviceurl", "PROXY_SERVICE_HTTP_URL")
	viper.BindEnv("services.smsserviceurl", "SMS_SERVICE_HTTP_URL")
	viper.BindEnv("services.vkserviceurl", "VK_SERVICE_HTTP_URL")
	viper.BindEnv("services.telegramserviceurl", "TELEGRAM_SERVICE_HTTP_URL")
	viper.BindEnv("services.warmingserviceurl", "WARMING_SERVICE_HTTP_URL")

	viper.BindEnv("sms.providerconfigpath", "SMS_PROVIDER_CONFIG_PATH")
	viper.BindEnv("sms.maxretryattempts", "SMS_MAX_RETRY_ATTEMPTS")
	viper.BindEnv("sms.codewaittimeout", "SMS_CODE_WAIT_TIMEOUT")
	viper.BindEnv("sms.activationexpiry", "SMS_ACTIVATION_EXPIRY")

	viper.BindEnv("proxy.healthcheckinterval", "PROXY_HEALTH_CHECK_INTERVAL")
	viper.BindEnv("proxy.rotationcheckinterval", "PROXY_ROTATION_CHECK_INTERVAL")
	viper.BindEnv("proxy.maxfailedchecks", "PROXY_MAX_FAILED_CHECKS")
	viper.BindEnv("proxy.ipqualityscoreapikey", "IPQS_API_KEY")
	viper.BindEnv("proxy.providerconfigpath", "PROXY_PROVIDER_CONFIG_PATH")

	viper.BindEnv("monitoring.prometheusport", "PROMETHEUS_PORT")
	viper.BindEnv("monitoring.grafanaport", "GRAFANA_PORT")

	viper.BindEnv("ratelimit.enabled", "RATE_LIMIT_ENABLED")
	viper.BindEnv("ratelimit.requests", "RATE_LIMIT_REQUESTS")
	viper.BindEnv("ratelimit.window", "RATE_LIMIT_WINDOW")
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
