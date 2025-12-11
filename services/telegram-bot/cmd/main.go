package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grigta/conveer/services/telegram-bot/internal/config"
	"github.com/grigta/conveer/services/telegram-bot/internal/handlers"
	"github.com/grigta/conveer/services/telegram-bot/internal/models"
	"github.com/grigta/conveer/services/telegram-bot/internal/repository"
	"github.com/grigta/conveer/services/telegram-bot/internal/service"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/go-telegram/bot"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.LoadConfig("configs/bot_config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to MongoDB
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	db := mongoClient.Database(cfg.DatabaseName)
	log.Println("Connected to MongoDB")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)

	// Create indexes
	if err := userRepo.CreateIndexes(ctx); err != nil {
		log.Printf("Warning: Failed to create indexes: %v", err)
	}

	// Initialize admin users from config
	for _, adminID := range cfg.AdminTelegramIDs {
		user, err := userRepo.GetByTelegramID(ctx, adminID)
		if err != nil || user == nil {
			newUser := &models.TelegramBotUser{
				TelegramID: adminID,
				Role:       models.RoleAdmin,
				IsActive:   true,
				Whitelist:  true,
			}
			if err := userRepo.Create(ctx, newUser); err != nil {
				log.Printf("Warning: Failed to create admin user %d: %v", adminID, err)
			} else {
				log.Printf("Created admin user: %d", adminID)
			}
		}
	}

	// Connect to RabbitMQ
	rabbitmq, err := messaging.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmq.Close()
	log.Println("Connected to RabbitMQ")

	// Initialize gRPC clients
	grpcClients, err := service.InitializeGRPCClients(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize all gRPC clients: %v", err)
		// Continue anyway, some services might not be available
	}
	if grpcClients != nil {
		defer grpcClients.Close()
	}
	log.Println("Initialized gRPC clients")

	// Create export repository
	var exportClients *repository.ExportClients
	if grpcClients != nil {
		exportClients = &repository.ExportClients{
			VKServiceClient:       grpcClients.VKServiceClient,
			TelegramServiceClient: grpcClients.TelegramServiceClient,
			Encryptor:             grpcClients.Encryptor,
		}
	}
	exportRepo := repository.NewExportRepository(exportClients)

	// Initialize services
	authService := service.NewAuthService(userRepo)
	commandService := service.NewCommandService(rabbitmq)
	exportService := service.NewExportService(exportRepo)
	statsService := service.NewStatsService(grpcClients)
	botService, err := service.NewBotService(cfg.BotToken, authService)
	if err != nil {
		log.Fatalf("Failed to create bot service: %v", err)
	}

	// Initialize event consumer
	eventConsumer := service.NewEventConsumer(rabbitmq, botService, authService)
	if err := eventConsumer.Start(ctx); err != nil {
		log.Printf("Warning: Failed to start event consumer: %v", err)
	}

	// Initialize handlers
	commandHandlers := handlers.NewCommandHandlers(
		authService,
		commandService,
		exportService,
		statsService,
		botService,
	)

	callbackHandlers := handlers.NewCallbackHandlers(
		authService,
		commandService,
		exportService,
		statsService,
		botService,
	)

	// Get bot instance
	b := botService.GetBot()

	// Register middlewares
	b.Use(handlers.LoggingMiddleware())

	// Register command handlers with auth middleware
	registerCommand := func(command string, handler bot.HandlerFunc, requiredRole string) {
		b.RegisterHandler(
			bot.HandlerTypeMessageText,
			command,
			bot.MatchTypePrefix,
			handlers.AuthMiddleware(authService, requiredRole)(handler),
		)
	}

	// Register commands
	registerCommand("/start", commandHandlers.HandleStart, models.RoleViewer)
	registerCommand("/help", commandHandlers.HandleHelp, models.RoleViewer)
	registerCommand("/accounts", commandHandlers.HandleAccounts, models.RoleViewer)
	registerCommand("/stats", commandHandlers.HandleStats, models.RoleViewer)
	registerCommand("/export", commandHandlers.HandleExport, models.RoleOperator)
	registerCommand("/register", commandHandlers.HandleRegister, models.RoleOperator)
	registerCommand("/warming", commandHandlers.HandleWarming, models.RoleOperator)
	registerCommand("/proxies", commandHandlers.HandleProxies, models.RoleOperator)
	registerCommand("/sms", commandHandlers.HandleSMS, models.RoleOperator)

	// Register callback handler
	b.RegisterHandler(
		bot.HandlerTypeCallbackQueryData,
		"",
		bot.MatchTypePrefix,
		handlers.AuthMiddleware(authService, models.RoleViewer)(callbackHandlers.HandleCallback),
	)

	log.Println("Bot handlers registered")

	// Start the bot
	go func() {
		log.Println("Starting bot...")
		botService.Start(ctx)
	}()

	log.Println("Telegram bot is running...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Give the bot time to finish current operations
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop event consumer
	if err := eventConsumer.Stop(); err != nil {
		log.Printf("Error stopping event consumer: %v", err)
	}

	// Wait for shutdown or timeout
	select {
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded")
	case <-time.After(1 * time.Second):
		log.Println("Graceful shutdown completed")
	}
}
