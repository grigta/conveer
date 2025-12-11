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
	"github.com/grigta/conveer/pkg/database"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/services/analytics-service/internal/config"
	"github.com/grigta/conveer/services/analytics-service/internal/handlers"
	"github.com/grigta/conveer/services/analytics-service/internal/models"
	"github.com/grigta/conveer/services/analytics-service/internal/repository"
	"github.com/grigta/conveer/services/analytics-service/internal/service"
	pb "github.com/grigta/conveer/services/analytics-service/proto"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

func main() {
	// Инициализация логгера
	log := logger.NewLogger("analytics-service")

	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/analytics_config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to load config")
	}

	// Контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Подключение к MongoDB
	mongoClient, err := database.NewMongoClient(cfg.MongoDB.URI)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to MongoDB")
	}
	defer mongoClient.Disconnect(ctx)

	db := mongoClient.Database(cfg.MongoDB.Database)

	// Создание индексов
	if err := setupIndexes(ctx, db); err != nil {
		log.WithError(err).Error("Failed to setup indexes")
	}

	// Подключение к Redis
	redisClient, err := cache.NewRedisClient(cfg.Redis.URL, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
	}

	// Подключение к RabbitMQ
	rabbitmq, err := messaging.NewRabbitMQ(cfg.RabbitMQ.URL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to RabbitMQ")
	}
	defer rabbitmq.Close()

	// Инициализация репозиториев
	metricsRepo := repository.NewMetricsRepository(db)
	forecastRepo := repository.NewForecastRepository(db)
	recommendationRepo := repository.NewRecommendationRepository(db)
	alertRepo := repository.NewAlertRepository(db)

	// Инициализация Prometheus клиента
	promClient, err := service.NewPrometheusClient(cfg.Prometheus.URL, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Prometheus client")
	}

	// Инициализация gRPC клиентов к другим сервисам
	grpcClients := initializeGRPCClients(cfg.GRPCServices, log)

	// Инициализация сервисов
	aggregator := service.NewAggregator(promClient, metricsRepo, grpcClients, log)
	forecaster := service.NewForecaster(metricsRepo, forecastRepo, redisClient, log)
	recommender := service.NewRecommender(metricsRepo, recommendationRepo, grpcClients, redisClient, log)
	alertManager := service.NewAlertManager(alertRepo, metricsRepo, rabbitmq, log, cfg.Alerts.MonthlyBudget, cfg.Alerts.BudgetPeriod)

	analyticsService := service.NewAnalyticsService(
		metricsRepo, forecastRepo, recommendationRepo, alertRepo,
		aggregator, forecaster, recommender, alertManager, log,
	)

	// Инициализация предустановленных правил алертов
	if err := initializeAlertRules(ctx, alertRepo, cfg.Alerts.Rules); err != nil {
		log.WithError(err).Error("Failed to initialize alert rules")
	}

	// Запуск фоновых воркеров
	go aggregator.Run(ctx)
	go forecaster.Run(ctx)
	go recommender.Run(ctx)
	go alertManager.Run(ctx)

	// Инициализация обработчиков
	handler := handlers.NewAnalyticsHandler(analyticsService, log)

	// Запуск gRPC сервера
	go startGRPCServer(cfg.Service.GRPCPort, handler, log)

	// Запуск HTTP сервера
	go startHTTPServer(cfg.Service.HTTPPort, handler, log)

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down analytics service...")
	cancel()
	time.Sleep(2 * time.Second)
}

func startGRPCServer(port int, handler *handlers.AnalyticsHandler, log *logger.Logger) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAnalyticsServiceServer(grpcServer, handler)

	log.WithField("port", port).Info("Starting gRPC server")
	if err := grpcServer.Serve(lis); err != nil {
		log.WithError(err).Fatal("Failed to serve gRPC")
	}
}

func startHTTPServer(port int, handler *handlers.AnalyticsHandler, log *logger.Logger) {
	router := gin.Default()

	// API routes
	v1 := router.Group("/api/v1/analytics")
	{
		v1.GET("/overall", handler.GetOverallAnalyticsHTTP)
		v1.GET("/platform/:platform", handler.GetPlatformAnalyticsHTTP)
		v1.GET("/forecast/expenses", handler.GetExpenseForecastHTTP)
		v1.GET("/forecast/readiness/:account_id", handler.GetReadinessForecastHTTP)
		v1.GET("/forecast/optimal-time", handler.GetOptimalTimeHTTP)
		v1.GET("/recommendations/proxies", handler.GetProxyRankingsHTTP)
		v1.GET("/recommendations/warming/:platform", handler.GetWarmingRecommendationsHTTP)
		v1.GET("/recommendations/errors", handler.GetErrorPatternsHTTP)
		v1.GET("/alerts", handler.GetAlertsHTTP)
		v1.POST("/alerts/:id/acknowledge", handler.AcknowledgeAlertHTTP)
		v1.GET("/rules", handler.ListAlertRulesHTTP)
		v1.POST("/rules", handler.CreateAlertRuleHTTP)
		v1.PUT("/rules/:id", handler.UpdateAlertRuleHTTP)
		v1.DELETE("/rules/:id", handler.DeleteAlertRuleHTTP)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Prometheus metrics
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	log.WithField("port", port).Info("Starting HTTP server")
	if err := router.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.WithError(err).Fatal("Failed to start HTTP server")
	}
}

func setupIndexes(ctx context.Context, db *mongo.Database) error {
	// aggregated_metrics indexes
	metricsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "timestamp", Value: -1},
				{Key: "platform", Value: 1},
			},
		},
		{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(90 * 24 * 3600), // 90 days
		},
	}
	if _, err := db.Collection("aggregated_metrics").Indexes().CreateMany(ctx, metricsIndexes); err != nil {
		return err
	}

	// forecasts indexes
	forecastIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "platform", Value: 1},
				{Key: "generated_at", Value: -1},
			},
		},
		{
			Keys:    bson.D{{Key: "valid_until", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}
	if _, err := db.Collection("forecasts").Indexes().CreateMany(ctx, forecastIndexes); err != nil {
		return err
	}

	// recommendations indexes
	recommendationIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "priority", Value: 1},
				{Key: "generated_at", Value: -1},
			},
		},
		{
			Keys:    bson.D{{Key: "valid_until", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}
	if _, err := db.Collection("recommendations").Indexes().CreateMany(ctx, recommendationIndexes); err != nil {
		return err
	}

	// alert_rules index
	alertRulesIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "enabled", Value: 1},
			{Key: "platform", Value: 1},
		},
	}
	if _, err := db.Collection("alert_rules").Indexes().CreateOne(ctx, alertRulesIndex); err != nil {
		return err
	}

	// alert_events indexes
	alertEventIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "fired_at", Value: -1}},
		},
		{
			Keys:    bson.D{{Key: "fired_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(30 * 24 * 3600), // 30 days
		},
	}
	if _, err := db.Collection("alert_events").Indexes().CreateMany(ctx, alertEventIndexes); err != nil {
		return err
	}

	return nil
}

func initializeAlertRules(ctx context.Context, repo *repository.AlertRepository, rules []config.AlertRuleConfig) error {
	for _, ruleConfig := range rules {
		existing, _ := repo.GetRuleByName(ctx, ruleConfig.Name)
		if existing == nil {
			rule := &models.AlertRule{
				Name:     ruleConfig.Name,
				Type:     ruleConfig.Type,
				Platform: ruleConfig.Platform,
				Enabled:  true,
				Threshold: models.AlertThreshold{
					Operator: ruleConfig.Threshold.Operator,
					Value:    ruleConfig.Threshold.Value,
				},
				Severity: ruleConfig.Severity,
				Cooldown: ruleConfig.Cooldown,
			}
			if err := repo.CreateRule(ctx, rule); err != nil {
				return err
			}
		}
	}
	return nil
}

func initializeGRPCClients(services map[string]string, log *logger.Logger) map[string]*grpc.ClientConn {
	clients := make(map[string]*grpc.ClientConn)

	for service, address := range services {
		conn, err := grpc.Dial(address, grpc.WithInsecure())
		if err != nil {
			log.WithError(err).WithField("service", service).Error("Failed to connect to service")
			continue
		}
		clients[service] = conn
	}

	return clients
}
