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

	"github.com/grigta/conveer/pkg/cache"
	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/pkg/crypto"
	"github.com/grigta/conveer/pkg/database"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/services/auth/internal/repository"
	"github.com/grigta/conveer/services/auth/internal/service"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	cfg, err := config.LoadConfig("./config")
	if err != nil {
		logger.Fatal("Failed to load config", logger.Field{Key: "error", Value: err.Error()})
	}

	log := logger.New(cfg.App.LogLevel, "json")
	logger.SetDefault(log)

	// Validate AES encryption configuration at startup
	if cfg.Encryption.Key == "" {
		logger.Fatal("Encryption key is not configured")
	}
	encryptor, err := crypto.NewEncryptor(cfg.Encryption.Key)
	if err != nil {
		logger.Fatal("Failed to initialize encryptor", logger.Field{Key: "error", Value: err.Error()})
	}
	_ = encryptor // Store for later use if needed

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewMongoDB(cfg.Database.URI, cfg.Database.DBName, 10*time.Second)
	if err != nil {
		logger.Fatal("Failed to connect to database", logger.Field{Key: "error", Value: err.Error()})
	}
	defer db.Close()

	redisCache, err := cache.NewRedisCache(cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", logger.Field{Key: "error", Value: err.Error()})
	}
	defer redisCache.Close()

	rabbitmq, err := messaging.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ", logger.Field{Key: "error", Value: err.Error()})
	}
	defer rabbitmq.Close()

	if err := rabbitmq.SetupTopology(); err != nil {
		logger.Fatal("Failed to setup RabbitMQ topology", logger.Field{Key: "error", Value: err.Error()})
	}

	authRepo := repository.NewAuthRepository(db, redisCache)
	authService := service.NewAuthService(authRepo, cfg, rabbitmq)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatal("Failed to listen", logger.Field{Key: "error", Value: err.Error()})
	}

	grpcServer := grpc.NewServer()

	go func() {
		logger.Info("Starting Auth gRPC Service", logger.Field{Key: "port", Value: 50051})
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	// Start HTTP server as a wrapper for REST API
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Setup HTTP handlers that wrap the service
	setupHTTPHandlers(router, authService)

	httpServer := &http.Server{
		Addr:    ":8001",
		Handler: router,
	}

	go func() {
		logger.Info("Starting Auth HTTP Service", logger.Field{Key: "port", Value: 8001})
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to serve HTTP", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down Auth Service...")

	// Graceful shutdown of HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server forced to shutdown", logger.Field{Key: "error", Value: err.Error()})
	}

	grpcServer.GracefulStop()
	logger.Info("Auth Service exited")
}

func setupHTTPHandlers(router *gin.Engine, authService *service.AuthService) {
	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "auth-service",
		})
	})

	// Auth endpoints
	router.POST("/register", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Registration endpoint"})
	})

	router.POST("/login", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Login endpoint"})
	})

	router.POST("/logout", func(c *gin.Context) {
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Logout endpoint"})
	})

	router.POST("/refresh", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Refresh endpoint"})
	})

	router.POST("/forgot-password", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Forgot password endpoint"})
	})

	router.POST("/reset-password", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Reset password endpoint"})
	})

	router.POST("/verify-email", func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// TODO: Implement proper handler that calls authService methods
		c.JSON(http.StatusOK, gin.H{"message": "Verify email endpoint"})
	})
}
