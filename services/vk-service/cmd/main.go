package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"conveer/pkg/config"
	"conveer/pkg/crypto"
	"conveer/pkg/database"
	"conveer/pkg/logger"
	"conveer/pkg/messaging"
	vkconfig "conveer/services/vk-service/internal/config"
	"conveer/services/vk-service/internal/handlers"
	"conveer/services/vk-service/internal/repository"
	"conveer/services/vk-service/internal/service"
	pb "conveer/services/vk-service/proto"
	proxypb "conveer/services/proxy-service/proto"
	smspb "conveer/services/sms-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Initialize logger
	log := logger.New("vk-service")
	log.Info("Starting VK Service")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	// Load VK-specific config
	vkCfg, err := vkconfig.LoadConfig(getEnv("VK_CONFIG_PATH", "./configs/vk_config.yaml"))
	if err != nil {
		log.Fatal("Failed to load VK config", "error", err)
	}

	// Initialize MongoDB
	mongoClient, mongoDB, err := database.NewMongoDB(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB", "error", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Initialize RabbitMQ
	messagingClient, err := messaging.NewClient(cfg.RabbitMQ.URL, log)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ", "error", err)
	}
	defer messagingClient.Close()

	// Setup RabbitMQ topology
	if err := setupRabbitMQTopology(messagingClient); err != nil {
		log.Fatal("Failed to setup RabbitMQ topology", "error", err)
	}

	// Initialize encryptor
	encryptor, err := crypto.NewEncryptor(cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatal("Failed to initialize encryptor", "error", err)
	}

	// Initialize password generator
	passwordGen := crypto.NewPasswordGenerator()

	// Initialize repositories
	accountRepo := repository.NewAccountRepository(mongoDB, encryptor, log)
	sessionRepo := repository.NewSessionRepository(mongoDB, redisClient, log)

	// Create indexes
	if err := accountRepo.CreateIndexes(context.Background()); err != nil {
		log.Error("Failed to create indexes", "error", err)
	}

	// Initialize gRPC clients
	proxyClient, err := createProxyClient(cfg)
	if err != nil {
		log.Fatal("Failed to create proxy service client", "error", err)
	}

	smsClient, err := createSMSClient(cfg)
	if err != nil {
		log.Fatal("Failed to create SMS service client", "error", err)
	}

	// Initialize metrics first
	metrics := service.NewMetricsCollector()

	// Initialize browser manager using config
	browserConfig := vkCfg.ToBrowserConfig()
	browserManager := service.NewBrowserManager(browserConfig, metrics, log)
	if err := browserManager.Initialize(context.Background()); err != nil {
		log.Fatal("Failed to initialize browser manager", "error", err)
	}
	defer browserManager.Shutdown(context.Background())

	// Initialize services
	stealthInjector := service.NewStealthInjector(log)
	fingerprintGen := service.NewFingerprintGenerator()

	// Initialize registration config from file
	registrationConfig := vkCfg.ToRegistrationConfig()

	// Initialize registration flow
	registrationFlow := service.NewRegistrationFlow(
		accountRepo,
		sessionRepo,
		browserManager,
		stealthInjector,
		fingerprintGen,
		proxyClient,
		smsClient,
		encryptor,
		passwordGen,
		registrationConfig,
		messagingClient,
		log,
	)

	// Initialize VK service
	vkService := service.NewVKService(
		accountRepo,
		sessionRepo,
		registrationFlow,
		proxyClient,
		messagingClient,
		metrics,
		log,
	)

	// Start background workers
	if err := vkService.StartWorkers(context.Background()); err != nil {
		log.Error("Failed to start workers", "error", err)
	}

	// Initialize HTTP handler
	httpHandler := handlers.NewHTTPHandler(vkService, log)

	// Initialize gRPC handler
	grpcHandler := handlers.NewGRPCHandler(vkService, log)

	// Start gRPC server
	grpcPort := getEnvInt("GRPC_PORT", 50059)
	go startGRPCServer(grpcPort, grpcHandler, log)

	// Start HTTP server
	httpPort := getEnvInt("HTTP_PORT", 8009)
	go startHTTPServer(httpPort, httpHandler, log)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down VK Service")

	// Shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown services
	if err := vkService.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to shutdown VK service", "error", err)
	}

	log.Info("VK Service stopped")
}

func setupRabbitMQTopology(client messaging.Client) error {
	// Declare exchanges
	if err := client.DeclareExchange("vk.events", "topic"); err != nil {
		return fmt.Errorf("failed to declare events exchange: %w", err)
	}

	if err := client.DeclareExchange("vk.commands", "direct"); err != nil {
		return fmt.Errorf("failed to declare commands exchange: %w", err)
	}

	// Declare queues
	queues := []string{"vk.register", "vk.retry", "vk.manual_intervention"}
	for _, queue := range queues {
		if err := client.DeclareQueue(queue); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}
	}

	// Bind queues to exchanges
	bindings := map[string]string{
		"vk.register":            "vk.commands",
		"vk.retry":               "vk.commands",
		"vk.manual_intervention": "vk.commands",
	}

	for queue, exchange := range bindings {
		if err := client.BindQueue(queue, exchange, queue); err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", queue, err)
		}
	}

	return nil
}

func createProxyClient(cfg *config.Config) (proxypb.ProxyServiceClient, error) {
	proxyServiceURL := getEnv("PROXY_SERVICE_URL", "proxy-service:50057")
	conn, err := grpc.Dial(proxyServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy service: %w", err)
	}
	return proxypb.NewProxyServiceClient(conn), nil
}

func createSMSClient(cfg *config.Config) (smspb.SMSServiceClient, error) {
	smsServiceURL := getEnv("SMS_SERVICE_URL", "sms-service:50058")
	conn, err := grpc.Dial(smsServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMS service: %w", err)
	}
	return smspb.NewSMSServiceClient(conn), nil
}

func startGRPCServer(port int, handler *handlers.GRPCHandler, log logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal("Failed to listen on gRPC port", "port", port, "error", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterVKServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	log.Info("Starting gRPC server", "port", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve gRPC", "error", err)
	}
}

func startHTTPServer(port int, handler *handlers.HTTPHandler, log logger.Logger) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	handler.RegisterRoutes(router)

	log.Info("Starting HTTP server", "port", port)
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatal("Failed to start HTTP server", "error", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
