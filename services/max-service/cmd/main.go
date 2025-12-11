package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/conveer/max-service/internal/config"
	"github.com/conveer/max-service/internal/handlers"
	"github.com/conveer/max-service/internal/repository"
	"github.com/conveer/max-service/internal/service"
	pb "github.com/conveer/max-service/proto"
	"github.com/conveer/pkg/encryption"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	configPath := os.Getenv("MAX_CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/max_config.yaml"
	}
	
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Connect to MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)
	
	db := mongoClient.Database(cfg.MongoDB.Database)
	
	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()
	
	// Connect to RabbitMQ
	rabbitmqConn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmqConn.Close()
	
	rabbitmqChannel, err := rabbitmqConn.Channel()
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ channel: %v", err)
	}
	defer rabbitmqChannel.Close()
	
	// Setup RabbitMQ topology
	if err := setupRabbitMQ(rabbitmqChannel); err != nil {
		log.Fatalf("Failed to setup RabbitMQ: %v", err)
	}
	
	// Initialize encryptor
	encryptor := encryption.NewAESEncryptor(cfg.Encryption.Key)
	
	// Initialize repositories
	accountRepo := repository.NewAccountRepository(db, encryptor)
	sessionRepo := repository.NewSessionRepository(db, redisClient)
	
	// Create indexes
	if err := accountRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Failed to create account indexes: %v", err)
	}
	if err := sessionRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Failed to create session indexes: %v", err)
	}
	
	// Connect to proxy service
	proxyConn, err := grpc.Dial(cfg.ProxyService.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to proxy service: %v", err)
	}
	defer proxyConn.Close()
	
	// Connect to SMS service
	smsConn, err := grpc.Dial(cfg.SMSService.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to SMS service: %v", err)
	}
	defer smsConn.Close()

	// Connect to VK service
	vkConn, err := grpc.Dial(cfg.VKService.Address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to VK service: %v", err)
	}
	defer vkConn.Close()

	// Initialize browser manager
	browserManager, err := service.NewBrowserManager(cfg.Browser.PoolSize, cfg.Browser.Headless)
	if err != nil {
		log.Fatalf("Failed to create browser manager: %v", err)
	}
	defer browserManager.Shutdown()

	// Initialize service
	maxService := service.NewMaxService(
		accountRepo,
		sessionRepo,
		proxyConn,
		smsConn,
		vkConn,
		rabbitmqChannel,
		browserManager,
		&cfg.Registration,
	)
	
	// Start background workers
	maxService.StartWorkers(ctx)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handlers.NewGRPCHandler(maxService)
	pb.RegisterMaxServiceServer(grpcServer, grpcHandler)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", ":"+cfg.Service.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port: %v", err)
	}

	go func() {
		log.Printf("Starting gRPC server on port %s", cfg.Service.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Create HTTP server
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	httpHandler := handlers.NewHTTPHandler(maxService)
	httpHandler.RegisterRoutes(router)
	
	// Start HTTP server
	go func() {
		log.Printf("Starting HTTP server on port %s", cfg.Service.HTTPPort)
		if err := router.Run(":" + cfg.Service.HTTPPort); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
	
	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	
	log.Println("Shutting down...")
	grpcServer.GracefulStop()
	cancel()
}

// setupRabbitMQ creates exchanges and queues
func setupRabbitMQ(ch *amqp.Channel) error {
	// Declare exchanges
	exchanges := []struct {
		name string
		kind string
	}{
		{"max.events", "topic"},
		{"max.commands", "direct"},
	}
	
	for _, ex := range exchanges {
		if err := ch.ExchangeDeclare(
			ex.name,
			ex.kind,
			true,  // durable
			false, // auto-delete
			false, // internal
			false, // no-wait
			nil,   // arguments
		); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", ex.name, err)
		}
	}
	
	// Declare queues
	queues := []string{
		"max.register",
		"max.retry",
		"max.manual_intervention",
	}
	
	for _, queue := range queues {
		if _, err := ch.QueueDeclare(
			queue,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}
	}
	
	// Bind queues
	bindings := []struct {
		queue    string
		exchange string
		key      string
	}{
		{"max.register", "max.commands", "max.register"},
		{"max.retry", "max.commands", "max.retry"},
		{"max.manual_intervention", "max.events", "max.manual_intervention"},
	}
	
	for _, binding := range bindings {
		if err := ch.QueueBind(
			binding.queue,
			binding.key,
			binding.exchange,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", binding.queue, err)
		}
	}
	
	return nil
}
