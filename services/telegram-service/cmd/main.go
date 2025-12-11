package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"conveer/pkg/database"
	"conveer/pkg/encryption"
	"conveer/pkg/logger"
	"conveer/pkg/rabbitmq"
	"conveer/services/telegram-service/internal/config"
	"conveer/services/telegram-service/internal/handlers"
	"conveer/services/telegram-service/internal/service"
	proxypb "conveer/services/proxy-service/proto"
	smspb "conveer/services/sms-service/proto"
	pb "conveer/services/telegram-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize logger
	log := logger.New(logger.Config{
		Level:  "info",
		Format: "json",
	})

	log.Info("Starting Telegram service")

	// Initialize MongoDB
	db, err := database.Connect(context.Background(), database.Config{
		URI:      getEnvOrDefault("MONGODB_URI", "mongodb://localhost:27017"),
		Database: getEnvOrDefault("MONGODB_DATABASE", "conveer"),
	})
	if err != nil {
		log.Fatal("Failed to connect to MongoDB", "error", err)
	}

	// Initialize encryption
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
	}

	if err := encryption.Initialize(encryptionKey); err != nil {
		log.Fatal("Failed to initialize encryption", "error", err)
	}

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     getEnvOrDefault("REDIS_URL", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", "error", err)
	}

	// Initialize RabbitMQ
	var rabbitPublisher rabbitmq.Publisher
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL != "" {
		rabbitPublisher, err = rabbitmq.NewPublisher(rabbitURL, log)
		if err != nil {
			log.Error("Failed to connect to RabbitMQ", "error", err)
			rabbitPublisher = nil
		}
	}

	// Initialize gRPC clients
	proxyServiceURL := getEnvOrDefault("PROXY_SERVICE_GRPC_URL", "proxy-service:50050")
	smsServiceURL := getEnvOrDefault("SMS_SERVICE_GRPC_URL", "sms-service:50055")

	proxyConn, err := grpc.Dial(proxyServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Failed to connect to proxy service", "error", err)
	}
	defer proxyConn.Close()

	smsConn, err := grpc.Dial(smsServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("Failed to connect to SMS service", "error", err)
	}
	defer smsConn.Close()

	proxyClient := proxypb.NewProxyServiceClient(proxyConn)
	smsClient := smspb.NewSMSServiceClient(smsConn)

	// Initialize browser manager
	browserManager := service.NewBrowserManager(
		cfg.ToBrowserConfig(),
		service.NewMetricsCollector("telegram"),
		log,
	)

	if err := browserManager.Initialize(context.Background()); err != nil {
		log.Fatal("Failed to initialize browser manager", "error", err)
	}

	// Initialize Telegram service
	telegramService, err := service.NewTelegramService(
		db,
		browserManager,
		proxyClient,
		smsClient,
		redisClient,
		rabbitPublisher,
		cfg,
		log,
	)
	if err != nil {
		log.Fatal("Failed to create telegram service", "error", err)
	}

	// Start monitoring
	if err := telegramService.StartMonitoring(context.Background()); err != nil {
		log.Error("Failed to start monitoring", "error", err)
	}

	// Initialize handlers
	httpHandler := handlers.NewHTTPHandler(telegramService, log)
	grpcHandler := handlers.NewGRPCHandler(telegramService, log)

	// Setup HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Register HTTP routes
	httpHandler.RegisterRoutes(router)

	// Start HTTP server
	httpPort := getEnvOrDefault("HTTP_PORT", "8010")
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%s", httpPort),
		Handler: router,
	}

	go func() {
		log.Info("Starting HTTP server", "port", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", "error", err)
		}
	}()

	// Setup gRPC server
	grpcPort := getEnvOrDefault("GRPC_PORT", "50060")
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatal("Failed to listen on gRPC port", "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTelegramServiceServer(grpcServer, grpcHandler)
	reflection.Register(grpcServer)

	go func() {
		log.Info("Starting gRPC server", "port", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("Failed to start gRPC server", "error", err)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("Telegram service started successfully")
	<-sigChan

	log.Info("Shutting down telegram service")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("Failed to shutdown HTTP server", "error", err)
	}

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown service
	if err := telegramService.Shutdown(ctx); err != nil {
		log.Error("Failed to shutdown telegram service", "error", err)
	}

	// Close database connection
	if err := database.Disconnect(ctx); err != nil {
		log.Error("Failed to disconnect from database", "error", err)
	}

	// Close RabbitMQ connection
	if rabbitPublisher != nil {
		if err := rabbitPublisher.Close(); err != nil {
			log.Error("Failed to close RabbitMQ connection", "error", err)
		}
	}

	log.Info("Telegram service shutdown complete")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
