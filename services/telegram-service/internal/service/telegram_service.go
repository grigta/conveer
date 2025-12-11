package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/services/telegram-service/internal/config"
	"github.com/grigta/conveer/services/telegram-service/internal/models"
	"github.com/grigta/conveer/services/telegram-service/internal/repository"
	proxypb "github.com/grigta/conveer/services/proxy-service/proto"
	smspb "github.com/grigta/conveer/services/sms-service/proto"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TelegramService interface {
	CreateAccount(ctx context.Context, req *models.RegistrationRequest) (*models.TelegramAccount, error)
	GetAccount(ctx context.Context, accountID primitive.ObjectID) (*models.TelegramAccount, error)
	ListAccounts(ctx context.Context, status models.AccountStatus, limit, offset int) ([]*models.TelegramAccount, int64, error)
	UpdateAccountStatus(ctx context.Context, accountID primitive.ObjectID, status models.AccountStatus) (*models.TelegramAccount, error)
	RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.TelegramAccount, error)
	DeleteAccount(ctx context.Context, accountID primitive.ObjectID) error
	GetStatistics(ctx context.Context) (*models.AccountStatistics, error)
	StartMonitoring(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type telegramService struct {
	accountRepo      *repository.AccountRepository
	sessionRepo      *repository.SessionRepository
	browserManager   BrowserManager
	registrationFlow RegistrationFlow
	proxyClient      proxypb.ProxyServiceClient
	smsClient        smspb.SMSServiceClient
	redisClient      *redis.Client
	rabbitPublisher  rabbitmq.Publisher
	config           *config.Config
	logger           logger.Logger
	metrics          MetricsCollector
	shutdownCh       chan struct{}
}

func NewTelegramService(
	db *mongo.Database,
	browserManager BrowserManager,
	proxyClient proxypb.ProxyServiceClient,
	smsClient smspb.SMSServiceClient,
	redisClient *redis.Client,
	rabbitPublisher rabbitmq.Publisher,
	config *config.Config,
	logger logger.Logger,
) (TelegramService, error) {
	// Create repositories
	accountRepo := repository.NewAccountRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	// Create metrics collector
	metrics := NewMetricsCollector("telegram")

	// Create stealth injector and fingerprint generator
	stealthInjector := NewStealthInjector()
	fingerprintGen := NewFingerprintGenerator()

	// Create registration flow
	registrationFlow := NewRegistrationFlow(
		accountRepo,
		sessionRepo,
		browserManager,
		stealthInjector,
		fingerprintGen,
		proxyClient,
		smsClient,
		config.ToRegistrationConfig(),
		logger,
		metrics,
	)

	return &telegramService{
		accountRepo:      accountRepo,
		sessionRepo:      sessionRepo,
		browserManager:   browserManager,
		registrationFlow: registrationFlow,
		proxyClient:      proxyClient,
		smsClient:        smsClient,
		redisClient:      redisClient,
		rabbitPublisher:  rabbitPublisher,
		config:           config,
		logger:           logger,
		metrics:          metrics,
		shutdownCh:       make(chan struct{}),
	}, nil
}

func (s *telegramService) CreateAccount(ctx context.Context, req *models.RegistrationRequest) (*models.TelegramAccount, error) {
	s.logger.Info("Creating new Telegram account", "first_name", req.FirstName)

	// Validate request
	if req.FirstName == "" {
		return nil, fmt.Errorf("first name is required")
	}

	// Use random profile if requested
	if req.UseRandomProfile {
		req = s.generateRandomProfile()
	}

	// Start registration flow
	result, err := s.registrationFlow.StartRegistration(ctx, req)
	if err != nil {
		s.logger.Error("Registration failed", "error", err)
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	// Get created account
	accountID, _ := primitive.ObjectIDFromHex(result.AccountID)
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created account: %w", err)
	}

	// Publish event
	s.publishAccountEvent("account.created", account)

	return account, nil
}

func (s *telegramService) GetAccount(ctx context.Context, accountID primitive.ObjectID) (*models.TelegramAccount, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Cache account data
	s.cacheAccountData(ctx, account)

	return account, nil
}

func (s *telegramService) ListAccounts(ctx context.Context, status models.AccountStatus, limit, offset int) ([]*models.TelegramAccount, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	accounts, total, err := s.accountRepo.ListByStatus(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list accounts: %w", err)
	}

	return accounts, total, nil
}

func (s *telegramService) UpdateAccountStatus(ctx context.Context, accountID primitive.ObjectID, status models.AccountStatus) (*models.TelegramAccount, error) {
	// Get current account
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	oldStatus := account.Status

	// Update status
	if err := s.accountRepo.UpdateStatus(ctx, accountID, status, ""); err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	// Get updated account
	account, _ = s.accountRepo.GetByID(ctx, accountID)

	// Record metrics
	s.metrics.IncrementAccountStatusChange(oldStatus, status)

	// Publish event
	s.publishAccountEvent("account.status.changed", account)

	return account, nil
}

func (s *telegramService) RetryRegistration(ctx context.Context, accountID primitive.ObjectID) (*models.TelegramAccount, error) {
	s.logger.Info("Retrying registration", "account_id", accountID.Hex())

	// Check retry limit
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	if account.RetryCount >= s.config.Telegram.Registration.MaxRetryAttempts {
		return nil, fmt.Errorf("maximum retry attempts exceeded")
	}

	// Retry registration
	result, err := s.registrationFlow.RetryRegistration(ctx, accountID)
	if err != nil {
		s.logger.Error("Retry failed", "error", err)
		return nil, fmt.Errorf("retry failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("retry failed: %s", result.ErrorMessage)
	}

	// Get updated account
	account, _ = s.accountRepo.GetByID(ctx, accountID)

	// Publish event
	s.publishAccountEvent("account.retry.success", account)

	return account, nil
}

func (s *telegramService) DeleteAccount(ctx context.Context, accountID primitive.ObjectID) error {
	// Get account before deletion
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Release proxy if allocated
	if account.ProxyID != primitive.NilObjectID {
		s.proxyClient.ReleaseProxy(ctx, &proxypb.ReleaseProxyRequest{
			ProxyId: account.ProxyID.Hex(),
		})
	}

	// Cancel SMS activation if active
	if account.ActivationID != "" {
		s.smsClient.CancelActivation(ctx, &smspb.CancelActivationRequest{
			ActivationId: account.ActivationID,
		})
	}

	// Delete account
	if err := s.accountRepo.Delete(ctx, accountID); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	// Publish event
	s.publishAccountEvent("account.deleted", account)

	return nil
}

func (s *telegramService) GetStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	stats, err := s.accountRepo.GetStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	// Cache statistics
	s.cacheStatistics(ctx, stats)

	return stats, nil
}

func (s *telegramService) StartMonitoring(ctx context.Context) error {
	// Start session cleanup
	go s.cleanupStaleSessions(ctx)

	// Start stuck registration monitoring
	go s.monitorStuckRegistrations(ctx)

	// Start metrics updater
	go s.updateMetrics(ctx)

	s.logger.Info("Monitoring started")
	return nil
}

func (s *telegramService) Shutdown(ctx context.Context) error {
	close(s.shutdownCh)

	// Shutdown browser manager
	if err := s.browserManager.Shutdown(ctx); err != nil {
		s.logger.Error("Failed to shutdown browser manager", "error", err)
	}

	// Close Redis connection
	if err := s.redisClient.Close(); err != nil {
		s.logger.Error("Failed to close Redis connection", "error", err)
	}

	s.logger.Info("Telegram service shutdown complete")
	return nil
}

func (s *telegramService) generateRandomProfile() *models.RegistrationRequest {
	firstNames := []string{"Alex", "Sam", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Avery"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis"}

	n1, _ := rand.Int(rand.Reader, big.NewInt(int64(len(firstNames))))
	n2, _ := rand.Int(rand.Reader, big.NewInt(int64(len(lastNames))))

	return &models.RegistrationRequest{
		FirstName: firstNames[n1.Int64()],
		LastName:  lastNames[n2.Int64()],
		Bio:       "Hi there! I'm using Telegram.",
	}
}

func (s *telegramService) publishAccountEvent(eventType string, account *models.TelegramAccount) {
	if s.rabbitPublisher == nil {
		return
	}

	event := map[string]interface{}{
		"type":       eventType,
		"account_id": account.ID.Hex(),
		"status":     account.Status,
		"timestamp":  time.Now().Unix(),
	}

	if err := s.rabbitPublisher.Publish("telegram.events", eventType, event); err != nil {
		s.logger.Error("Failed to publish event", "type", eventType, "error", err)
	}
}

func (s *telegramService) cacheAccountData(ctx context.Context, account *models.TelegramAccount) {
	if s.redisClient == nil {
		return
	}

	key := fmt.Sprintf("telegram:account:%s", account.ID.Hex())
	s.redisClient.HSet(ctx, key, map[string]interface{}{
		"status":     account.Status,
		"phone":      account.Phone,
		"username":   account.Username,
		"created_at": account.CreatedAt.Unix(),
	})
	s.redisClient.Expire(ctx, key, 1*time.Hour)
}

func (s *telegramService) cacheStatistics(ctx context.Context, stats *models.AccountStatistics) {
	if s.redisClient == nil {
		return
	}

	key := "telegram:statistics"
	s.redisClient.HSet(ctx, key, map[string]interface{}{
		"total":          stats.Total,
		"success_rate":   stats.SuccessRate,
		"last_hour":      stats.LastHour,
		"last_24_hours":  stats.Last24Hours,
	})
	s.redisClient.Expire(ctx, key, 5*time.Minute)
}

func (s *telegramService) cleanupStaleSessions(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.config.Telegram.Monitoring.SessionCleanupInterval) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			maxAge := time.Duration(s.config.Telegram.Monitoring.SessionExpiry) * time.Minute
			if err := s.sessionRepo.CleanupOldSessions(ctx, maxAge); err != nil {
				s.logger.Error("Failed to cleanup sessions", "error", err)
			}
		}
	}
}

func (s *telegramService) monitorStuckRegistrations(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			stuckDuration := time.Duration(s.config.Telegram.Monitoring.StuckRegistrationTimeout) * time.Minute
			sessions, err := s.sessionRepo.GetStuckSessions(ctx, stuckDuration)
			if err != nil {
				s.logger.Error("Failed to get stuck sessions", "error", err)
				continue
			}

			for _, session := range sessions {
				s.logger.Warn("Found stuck registration", "account_id", session.AccountID.Hex())
				s.metrics.IncrementManualInterventions()

				// Update account status to error
				s.accountRepo.UpdateStatus(ctx, session.AccountID, models.StatusError, "Registration stuck")
			}
		}
	}
}

func (s *telegramService) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdownCh:
			return
		case <-ticker.C:
			// Update browser pool metrics
			poolStats := s.browserManager.GetPoolStats()
			s.metrics.UpdateBrowserPoolSize(poolStats.TotalBrowsers)
		}
	}
}
