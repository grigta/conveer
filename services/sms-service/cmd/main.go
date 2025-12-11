package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/handlers"
	"github.com/grigta/conveer/services/sms-service/internal/repository"
	"github.com/grigta/conveer/services/sms-service/internal/service"
	pb "github.com/grigta/conveer/services/sms-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath("/app/configs")
	viper.AddConfigPath("./configs")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Support common env names used in our .env / docker-compose
	_ = viper.BindEnv("rabbitmq.uri", "RABBITMQ_URL")
	_ = viper.BindEnv("mongodb.uri", "MONGODB_URI", "MONGO_URI")
	_ = viper.BindEnv("mongodb.database", "MONGO_DB_NAME")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR", "REDIS_ADDRESS")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	if err := viper.ReadInConfig(); err != nil {
		logger.Warnf("Config file not found, using env variables: %v", err)
	}

	// Set defaults
	viper.SetDefault("service.name", "sms-service")
	viper.SetDefault("grpc.port", "50058")
	viper.SetDefault("http.port", "8008")
	viper.SetDefault("mongodb.uri", "mongodb://mongodb:27017")
	viper.SetDefault("mongodb.database", "sms_service")
	viper.SetDefault("redis.addr", "redis:6379")
	viper.SetDefault("rabbitmq.uri", "amqp://guest:guest@rabbitmq:5672/")
	viper.SetDefault("sms.provider_config_path", "/app/configs/providers.yaml")
	viper.SetDefault("sms.max_retry_attempts", 3)
	viper.SetDefault("sms.code_wait_timeout", "5m")
	viper.SetDefault("sms.activation_expiry", "30m")

	// Initialize MongoDB
	ctx := context.Background()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(viper.GetString("mongodb.uri")))
	if err != nil {
		logger.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	database := mongoClient.Database(viper.GetString("mongodb.database"))

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     viper.GetString("redis.addr"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})

	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize RabbitMQ
	rabbitConn, err := amqp.Dial(viper.GetString("rabbitmq.uri"))
	if err != nil {
		logger.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()

	rabbitChannel, err := rabbitConn.Channel()
	if err != nil {
		logger.Fatalf("Failed to open RabbitMQ channel: %v", err)
	}
	defer rabbitChannel.Close()

	// Setup RabbitMQ topology
	if err := setupRabbitMQTopology(rabbitChannel); err != nil {
		logger.Fatalf("Failed to setup RabbitMQ topology: %v", err)
	}

	// Initialize repositories
	phoneRepo := repository.NewPhoneRepository(database, logger)
	activationRepo := repository.NewActivationRepository(database, logger)

	// Initialize services
	providerAdapter := service.NewProviderAdapter(logger)
	smsActivateClient := service.NewSMSActivateClient(
		viper.GetString("SMS_ACTIVATE_API_KEY"),
		logger,
	)

	cacheService := service.NewCacheService(redisClient, logger)
	retryManager := service.NewRetryManager(rabbitChannel, logger)
	metricsCollector := service.NewMetricsCollector()

	smsService := service.NewSMSService(
		phoneRepo,
		activationRepo,
		providerAdapter,
		smsActivateClient,
		cacheService,
		retryManager,
		metricsCollector,
		logger,
	)

	// Start background workers
	go retryManager.StartWorker(ctx, smsService)
	go smsService.StartCodePoller(ctx)

	// Initialize handlers
	grpcHandler := handlers.NewGRPCHandler(smsService, logger)
	httpHandler := handlers.NewHTTPHandler(smsService, logger)

	// Start gRPC server
	grpcPort := viper.GetString("grpc.port")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		logger.Fatalf("Failed to listen on gRPC port %s: %v", grpcPort, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSMSServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	go func() {
		logger.Infof("Starting gRPC server on port %s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health", "/metrics"},
	}))

	// Register HTTP routes
	router.GET("/health", httpHandler.Health)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/v1")
	{
		api.POST("/purchase", httpHandler.PurchaseNumber)
		api.GET("/code/:activation_id", httpHandler.GetSMSCode)
		api.POST("/cancel/:activation_id", httpHandler.CancelActivation)
		api.GET("/status/:activation_id", httpHandler.GetActivationStatus)
		api.GET("/statistics", httpHandler.GetStatistics)
		api.GET("/balance", httpHandler.GetProviderBalance)
	}

	httpPort := viper.GetString("http.port")
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}

	go func() {
		logger.Infof("Starting HTTP server on port %s", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcServer.GracefulStop()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Errorf("HTTP server forced to shutdown: %v", err)
	}

	logger.Info("Servers exited")
}

func setupRabbitMQTopology(ch *amqp.Channel) error {
	// Declare exchanges
	if err := ch.ExchangeDeclare(
		"sms.events", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	); err != nil {
		return fmt.Errorf("failed to declare sms.events exchange: %w", err)
	}

	if err := ch.ExchangeDeclare(
		"sms.commands", // name
		"direct",       // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	); err != nil {
		return fmt.Errorf("failed to declare sms.commands exchange: %w", err)
	}

	// Declare queues
	queues := []string{"sms.purchase", "sms.get_code", "sms.cancel", "sms.retry"}
	for _, queueName := range queues {
		if _, err := ch.QueueDeclare(
			queueName, // name
			true,      // durable
			false,     // delete when unused
			false,     // exclusive
			false,     // no-wait
			nil,       // arguments
		); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
		}
	}

	// Bind queues to exchanges
	bindings := []struct {
		queue    string
		exchange string
		key      string
	}{
		{"sms.purchase", "sms.commands", "purchase"},
		{"sms.get_code", "sms.commands", "get_code"},
		{"sms.cancel", "sms.commands", "cancel"},
		{"sms.retry", "sms.commands", "retry"},
	}

	for _, binding := range bindings {
		if err := ch.QueueBind(
			binding.queue,    // queue name
			binding.key,      // routing key
			binding.exchange, // exchange
			false,            // no-wait
			nil,              // arguments
		); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w",
				binding.queue, binding.exchange, err)
		}
	}

	return nil
}
