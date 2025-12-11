package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"conveer/pkg/cache"
	"conveer/pkg/config"
	"conveer/pkg/crypto"
	"conveer/pkg/database"
	"conveer/pkg/logger"
	"conveer/pkg/messaging"
	"conveer/services/proxy-service/internal/handlers"
	"conveer/services/proxy-service/internal/repository"
	"conveer/services/proxy-service/internal/service"
	pb "conveer/services/proxy-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.LoadConfig()
	log := logger.SetupLogger()

	encryptor, err := crypto.NewEncryptor(cfg.Crypto.EncryptionKey)
	if err != nil {
		log.Fatal("Failed to create encryptor: ", err)
	}

	mongodb, err := database.NewMongoDB(ctx, cfg.Database.MongoDB.URI)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB: ", err)
	}
	defer mongodb.Disconnect(ctx)

	redis, err := cache.NewRedisCache(ctx, cfg.Cache.Redis.Addr, cfg.Cache.Redis.Password, cfg.Cache.Redis.DB)
	if err != nil {
		log.Fatal("Failed to connect to Redis: ", err)
	}
	defer redis.Close()

	rabbitmq, err := messaging.NewRabbitMQ(cfg.MessageQueue.RabbitMQ.URL)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ: ", err)
	}
	defer rabbitmq.Close()

	if err := setupRabbitMQ(rabbitmq, log); err != nil {
		log.Fatal("Failed to setup RabbitMQ: ", err)
	}

	proxyRepo := repository.NewProxyRepository(mongodb, encryptor, log)
	providerRepo := repository.NewProviderRepository(mongodb, log)

	if err := proxyRepo.CreateIndexes(ctx); err != nil {
		log.WithError(err).Error("Failed to create proxy indexes")
	}

	if err := providerRepo.CreateIndexes(ctx); err != nil {
		log.WithError(err).Error("Failed to create provider indexes")
	}

	providerConfigPath := "./configs/providers.yaml"
	if cfg.Proxy.ProviderConfigPath != "" {
		providerConfigPath = cfg.Proxy.ProviderConfigPath
	}

	providerManager, err := service.NewProviderManager(providerConfigPath, log, encryptor)
	if err != nil {
		log.Fatal("Failed to create provider manager: ", err)
	}

	healthChecker := service.NewHealthChecker(proxyRepo, rabbitmq, log, cfg)
	rotationManager := service.NewRotationManager(proxyRepo, providerRepo, providerManager, rabbitmq, log, cfg)
	proxyService := service.NewProxyService(
		proxyRepo,
		providerRepo,
		providerManager,
		healthChecker,
		rotationManager,
		rabbitmq,
		redis,
		log,
		cfg,
	)

	proxyService.Start(ctx)
	defer proxyService.Stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		startGRPCServer(proxyService, proxyRepo, log, cfg)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		startHTTPServer(proxyService, proxyRepo, providerRepo, log, cfg)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down servers...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("All servers shut down gracefully")
	case <-shutdownCtx.Done():
		log.Warn("Shutdown timeout exceeded")
	}
}

func setupRabbitMQ(rabbitmq *messaging.RabbitMQ, log *logrus.Logger) error {
	if err := rabbitmq.DeclareExchange("proxy.events", "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare events exchange: %w", err)
	}

	if err := rabbitmq.DeclareExchange("proxy.commands", "direct", true, false); err != nil {
		return fmt.Errorf("failed to declare commands exchange: %w", err)
	}

	queues := []string{
		"proxy.allocate",
		"proxy.release",
		"proxy.health_check",
		"proxy.rotation",
	}

	for _, queue := range queues {
		if _, err := rabbitmq.DeclareQueue(queue, true, false, false); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queue, err)
		}

		if err := rabbitmq.BindQueue(queue, queue, "proxy.commands"); err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", queue, err)
		}
	}

	log.Info("RabbitMQ topology setup completed")
	return nil
}

func startGRPCServer(proxyService *service.ProxyService, proxyRepo *repository.ProxyRepository, log *logrus.Logger, cfg *config.Config) {
	port := 50057
	if cfg.Services.ProxyServiceURL != "" {
		// Parse port from URL if needed
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal("Failed to listen on gRPC port: ", err)
	}

	grpcServer := grpc.NewServer()
	grpcHandler := handlers.NewGRPCHandler(proxyService, proxyRepo, log)
	pb.RegisterProxyServiceServer(grpcServer, grpcHandler)

	reflection.Register(grpcServer)

	log.Infof("Starting gRPC server on port %d", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("Failed to serve gRPC: ", err)
	}
}

func startHTTPServer(proxyService *service.ProxyService, proxyRepo *repository.ProxyRepository, providerRepo *repository.ProviderRepository, log *logrus.Logger, cfg *config.Config) {
	port := 8007

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: log.Out,
	}))

	httpHandler := handlers.NewHTTPHandler(proxyService, proxyRepo, providerRepo, log)
	httpHandler.SetupRoutes(router)

	// Add Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.Infof("Starting HTTP server on port %d", port)
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatal("Failed to start HTTP server: ", err)
	}
}
