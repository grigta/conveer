package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/conveer/conveer/pkg/config"
	"github.com/conveer/conveer/pkg/crypto"
	"github.com/conveer/conveer/pkg/logger"
	"github.com/conveer/conveer/services/api-gateway/internal/handlers"
	"github.com/conveer/conveer/services/api-gateway/internal/routes"
	"github.com/gin-gonic/gin"
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

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	h := handlers.NewHandlers(cfg)
	routes.SetupRoutes(router, h, cfg)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.App.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("Starting API Gateway", logger.Field{Key: "port", Value: cfg.App.Port})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", logger.Field{Key: "error", Value: err.Error()})
	}

	logger.Info("Server exited")
}