package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"conveer/pkg/cache"
	"conveer/pkg/database"
	"conveer/pkg/logger"
	"conveer/pkg/messaging"
	"conveer/services/warming-service/internal/config"
	"conveer/services/warming-service/internal/handlers"
	"conveer/services/warming-service/internal/repository"
	"conveer/services/warming-service/internal/service"
	pb "conveer/services/warming-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

type GRPCClients struct {
	VKClient       *grpc.ClientConn
	TelegramClient *grpc.ClientConn
	MailClient     *grpc.ClientConn
	MaxClient      *grpc.ClientConn
}

func main() {
	cfg := config.Load()
	log := logger.New(cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize MongoDB
	mongoClient := database.ConnectMongoDB(cfg.MongoURI)
	defer mongoClient.Disconnect(ctx)
	db := mongoClient.Database(cfg.DatabaseName)

	// Ensure indexes are created
	if err := ensureIndexes(db); err != nil {
		log.Error("Failed to ensure indexes: %v", err)
		// Continue anyway, indexes may already exist
	}

	// Initialize Redis
	redisClient := cache.ConnectRedis(cfg.RedisURL)
	defer redisClient.Close()

	// Initialize RabbitMQ
	messagingClient := messaging.NewRabbitMQClient(cfg.RabbitMQURL)
	defer messagingClient.Close()

	// Setup RabbitMQ topology
	if err := setupRabbitMQTopology(messagingClient); err != nil {
		log.Error("Failed to setup RabbitMQ topology: %v", err)
		panic(err)
	}

	// Initialize gRPC clients for other services
	grpcClients := initializeGRPCClients(cfg)

	// Initialize repositories
	taskRepo := repository.NewTaskRepository(db)
	scenarioRepo := repository.NewScenarioRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)

	// Initialize services
	warmingService := service.NewWarmingService(
		taskRepo,
		scenarioRepo,
		statsRepo,
		scheduleRepo,
		messagingClient,
		redisClient,
		grpcClients.VKClient,
		grpcClients.TelegramClient,
		grpcClients.MailClient,
		grpcClients.MaxClient,
		cfg,
		log,
	)

	// Initialize metrics
	metrics := service.NewMetrics()
	metrics.Register()

	// Start background workers
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		warmingService.StartWorkers(ctx)
	}()

	// Start gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		startGRPCServer(cfg.GRPCPort, warmingService, log)
	}()

	// Start HTTP server
	wg.Add(1)
	go func() {
		defer wg.Done()
		startHTTPServer(cfg.HTTPPort, warmingService, log)
	}()

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down warming-service...")
	cancel()

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("Warming-service shutdown complete")
	case <-time.After(30 * time.Second):
		log.Error("Shutdown timeout exceeded")
	}
}

func setupRabbitMQTopology(client *messaging.RabbitMQClient) error {
	// Declare exchanges
	exchanges := []struct {
		name string
		kind string
	}{
		{"warming.events", "topic"},
		{"warming.commands", "direct"},
	}

	for _, ex := range exchanges {
		if err := client.DeclareExchange(ex.name, ex.kind); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %v", ex.name, err)
		}
	}

	// Declare queues and bindings
	queues := []struct {
		name       string
		exchange   string
		routingKey string
	}{
		{"warming.start", "warming.commands", "start"},
		{"warming.execute_action", "warming.commands", "execute_action"},
		{"warming.pause", "warming.commands", "pause"},
		{"warming.resume", "warming.commands", "resume"},
		{"warming.status_sync", "warming.commands", "status_sync"},
		{"warming.auto_start", "", ""}, // Will bind to multiple exchanges
	}

	for _, q := range queues {
		if err := client.DeclareQueue(q.name); err != nil {
			return fmt.Errorf("failed to declare queue %s: %v", q.name, err)
		}

		if q.exchange != "" {
			if err := client.BindQueue(q.name, q.exchange, q.routingKey); err != nil {
				return fmt.Errorf("failed to bind queue %s: %v", q.name, err)
			}
		}
	}

	// Bind auto_start queue to account creation events
	platforms := []string{"vk", "telegram", "mail", "max"}
	for _, platform := range platforms {
		exchange := fmt.Sprintf("%s.events", platform)
		routingKey := fmt.Sprintf("%s.account.created", platform)
		if err := client.BindQueue("warming.auto_start", exchange, routingKey); err != nil {
			return fmt.Errorf("failed to bind auto_start to %s: %v", platform, err)
		}
	}

	return nil
}

func initializeGRPCClients(cfg *config.Config) *GRPCClients {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(50 * 1024 * 1024), // 50MB
			grpc.MaxCallSendMsgSize(50 * 1024 * 1024), // 50MB
		),
	}

	// Connect to VK service
	vkConn, err := grpc.Dial(cfg.VKServiceURL, opts...)
	if err != nil {
		log.Printf("Failed to connect to VK service: %v", err)
	}

	// Connect to Telegram service
	telegramConn, err := grpc.Dial(cfg.TelegramServiceURL, opts...)
	if err != nil {
		log.Printf("Failed to connect to Telegram service: %v", err)
	}

	// Connect to Mail service
	mailConn, err := grpc.Dial(cfg.MailServiceURL, opts...)
	if err != nil {
		log.Printf("Failed to connect to Mail service: %v", err)
	}

	// Connect to Max service
	maxConn, err := grpc.Dial(cfg.MaxServiceURL, opts...)
	if err != nil {
		log.Printf("Failed to connect to Max service: %v", err)
	}

	return &GRPCClients{
		VKClient:       vkConn,
		TelegramClient: telegramConn,
		MailClient:     mailConn,
		MaxClient:      maxConn,
	}
}

func startGRPCServer(port int, warmingService service.WarmingService, log logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Error("Failed to listen on port %d: %v", port, err)
		return
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(50 * 1024 * 1024), // 50MB
		grpc.MaxSendMsgSize(50 * 1024 * 1024), // 50MB
	)

	handler := handlers.NewGRPCHandler(warmingService, log)
	pb.RegisterWarmingServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	log.Info("gRPC server listening on port %d", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Error("gRPC server failed: %v", err)
	}
}

func startHTTPServer(port int, warmingService service.WarmingService, log logger.Logger) {
	router := gin.Default()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Initialize HTTP handler
	httpHandler := handlers.NewHTTPHandler(warmingService, log)
	httpHandler.RegisterRoutes(router)

	log.Info("HTTP server listening on port %d", port)
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Error("HTTP server failed: %v", err)
	}
}

func ensureIndexes(db *mongo.Database) error {
	collections := map[string][]mongo.IndexModel{
		"warming_tasks": {
			{Keys: map[string]interface{}{"account_id": 1, "platform": 1}, Options: nil},
			{Keys: map[string]interface{}{"status": 1, "next_action_at": 1}, Options: nil},
			{Keys: map[string]interface{}{"platform": 1, "status": 1}, Options: nil},
		},
		"warming_scenarios": {
			{Keys: map[string]interface{}{"platform": 1, "name": 1}, Options: nil},
		},
		"warming_actions_log": {
			{Keys: map[string]interface{}{"task_id": 1, "timestamp": -1}, Options: nil},
		},
	}

	for collName, indexes := range collections {
		coll := db.Collection(collName)
		for _, index := range indexes {
			if _, err := coll.Indexes().CreateOne(context.Background(), index); err != nil {
				return fmt.Errorf("failed to create index on %s: %v", collName, err)
			}
		}
	}

	return nil
}
