package service

import (
	"context"
	"fmt"
	"time"

	"conveer/pkg/logger"
	"conveer/pkg/messaging"
	"conveer/services/vk-service/internal/models"
	"conveer/services/vk-service/internal/repository"
	proxypb "conveer/services/proxy-service/proto"

	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VKService interface {
	CreateAccount(ctx context.Context, request *models.RegistrationRequest) (*models.VKAccount, error)
	GetAccount(ctx context.Context, id primitive.ObjectID) (*models.VKAccount, error)
	GetAccountsByStatus(ctx context.Context, status models.AccountStatus, limit int64) ([]*models.VKAccount, error)
	UpdateAccountStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus) error
	RetryRegistration(ctx context.Context, accountID primitive.ObjectID) error
	DeleteAccount(ctx context.Context, id primitive.ObjectID) error
	GetStatistics(ctx context.Context) (*models.AccountStatistics, error)
	PublishManualInterventionRequired(ctx context.Context, accountID primitive.ObjectID, reason string, details map[string]interface{}) error
	StartWorkers(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type vkService struct {
	accountRepo      repository.AccountRepository
	sessionRepo      repository.SessionRepository
	registrationFlow RegistrationFlow
	proxyClient      proxypb.ProxyServiceClient
	messagingClient  messaging.Client
	metrics          MetricsCollector
	logger           logger.Logger
	workerCtx        context.Context
	workerCancel     context.CancelFunc
}

func NewVKService(
	accountRepo repository.AccountRepository,
	sessionRepo repository.SessionRepository,
	registrationFlow RegistrationFlow,
	proxyClient proxypb.ProxyServiceClient,
	messagingClient messaging.Client,
	metrics MetricsCollector,
	logger logger.Logger,
) VKService {
	return &vkService{
		accountRepo:      accountRepo,
		sessionRepo:      sessionRepo,
		registrationFlow: registrationFlow,
		proxyClient:      proxyClient,
		messagingClient:  messagingClient,
		metrics:          metrics,
		logger:           logger,
	}
}

func (s *vkService) CreateAccount(ctx context.Context, request *models.RegistrationRequest) (*models.VKAccount, error) {
	// Generate profile if requested
	if request.UseRandomProfile {
		profile := NewFingerprintGenerator().GenerateRandomProfile()
		request.FirstName = profile.FirstName
		request.LastName = profile.LastName
		request.BirthDate = profile.BirthDate
		request.Gender = models.Gender(profile.Gender)
	}

	// Create account record
	account := &models.VKAccount{
		FirstName:  request.FirstName,
		LastName:   request.LastName,
		Gender:     string(request.Gender),
		Status:     models.StatusCreating,
		RetryCount: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if !request.BirthDate.IsZero() {
		account.BirthDate = &request.BirthDate
	}

	if err := s.accountRepo.CreateAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	// Publish registration command to queue
	command := map[string]interface{}{
		"account_id": account.ID.Hex(),
		"request":    request,
		"timestamp":  time.Now(),
	}

	if err := s.messagingClient.PublishToQueue("vk.register", command); err != nil {
		s.logger.Error("Failed to publish registration command", "error", err, "account_id", account.ID)
		// Update status to error
		s.accountRepo.UpdateAccountStatus(ctx, account.ID, models.StatusError, "failed to queue registration")
		return nil, fmt.Errorf("failed to queue registration: %w", err)
	}

	s.metrics.IncrementAccountsTotal(string(models.StatusCreating))
	s.logger.Info("Account creation initiated", "account_id", account.ID)

	return account, nil
}

func (s *vkService) GetAccount(ctx context.Context, id primitive.ObjectID) (*models.VKAccount, error) {
	return s.accountRepo.GetAccountByID(ctx, id)
}

func (s *vkService) GetAccountsByStatus(ctx context.Context, status models.AccountStatus, limit int64) ([]*models.VKAccount, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.accountRepo.GetAccountsByStatus(ctx, status, limit)
}

func (s *vkService) UpdateAccountStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus) error {
	// Get current status for metrics
	account, err := s.accountRepo.GetAccountByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Update status
	if err := s.accountRepo.UpdateAccountStatus(ctx, id, status, ""); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Update metrics
	s.metrics.DecrementAccountsTotal(string(account.Status))
	s.metrics.IncrementAccountsTotal(string(status))

	// Publish status change event
	event := map[string]interface{}{
		"account_id": id.Hex(),
		"old_status": account.Status,
		"new_status": status,
		"timestamp":  time.Now(),
	}

	routingKey := fmt.Sprintf("vk.account.status_changed.%s", status)
	if err := s.messagingClient.PublishEvent("vk.events", routingKey, event); err != nil {
		s.logger.Warn("Failed to publish status change event", "error", err, "account_id", id)
	}

	return nil
}

func (s *vkService) RetryRegistration(ctx context.Context, accountID primitive.ObjectID) error {
	// Check if account exists
	account, err := s.accountRepo.GetAccountByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Check retry count
	if account.RetryCount >= 3 {
		return fmt.Errorf("maximum retry attempts exceeded")
	}

	// Publish retry command
	command := map[string]interface{}{
		"account_id":  accountID.Hex(),
		"retry_count": account.RetryCount + 1,
		"timestamp":   time.Now(),
	}

	if err := s.messagingClient.PublishToQueue("vk.retry", command); err != nil {
		return fmt.Errorf("failed to queue retry: %w", err)
	}

	s.metrics.IncrementRetryAttempts()
	s.logger.Info("Registration retry queued", "account_id", accountID, "retry_count", account.RetryCount+1)

	return nil
}

func (s *vkService) DeleteAccount(ctx context.Context, id primitive.ObjectID) error {
	// Get account details
	account, err := s.accountRepo.GetAccountByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Release proxy if allocated
	if !account.ProxyID.IsZero() {
		if _, err := s.proxyClient.ReleaseProxy(ctx, &proxypb.ReleaseProxyRequest{
			AccountId: id.Hex(),
		}); err != nil {
			s.logger.Error("Failed to release proxy", "error", err, "account_id", id, "proxy_id", account.ProxyID)
		}
	}

	// Delete session
	if err := s.sessionRepo.DeleteSession(ctx, id); err != nil {
		s.logger.Warn("Failed to delete session", "error", err, "account_id", id)
	}

	// Delete account
	if err := s.accountRepo.DeleteAccount(ctx, id); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	// Update metrics
	s.metrics.DecrementAccountsTotal(string(account.Status))

	s.logger.Info("Account deleted", "account_id", id)
	return nil
}

func (s *vkService) GetStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	stats, err := s.accountRepo.GetAccountStatistics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	// Add current metrics
	stats.Total = s.metrics.GetTotalAccounts()

	return stats, nil
}

func (s *vkService) PublishManualInterventionRequired(ctx context.Context, accountID primitive.ObjectID, reason string, details map[string]interface{}) error {
	// Create intervention message
	message := map[string]interface{}{
		"account_id": accountID.Hex(),
		"reason":     reason,
		"details":    details,
		"timestamp":  time.Now(),
		"retry_count": 0,
	}

	// Get account details for context
	if account, err := s.accountRepo.GetAccountByID(ctx, accountID); err == nil {
		message["phone"] = account.Phone
		message["status"] = string(account.Status)
		message["retry_count"] = account.RetryCount
	}

	// Publish to manual intervention queue
	if err := s.messagingClient.PublishToQueue("vk.manual_intervention", message); err != nil {
		return fmt.Errorf("failed to publish manual intervention request: %w", err)
	}

	// Update account status to indicate manual intervention required
	if err := s.accountRepo.UpdateAccountStatus(ctx, accountID, models.StatusSuspended, fmt.Sprintf("Manual intervention required: %s", reason)); err != nil {
		s.logger.Error("Failed to update account status for manual intervention", "error", err, "account_id", accountID)
	}

	// Update metrics
	s.metrics.IncrementManualInterventions()

	s.logger.Info("Manual intervention requested",
		"account_id", accountID,
		"reason", reason,
		"details", details)

	return nil
}

func (s *vkService) StartWorkers(ctx context.Context) error {
	s.workerCtx, s.workerCancel = context.WithCancel(ctx)

	// Start registration command consumer
	go s.consumeRegistrationCommands(s.workerCtx)

	// Start retry command consumer
	go s.consumeRetryCommands(s.workerCtx)

	// Start stuck registration monitor
	go s.monitorStuckRegistrations(s.workerCtx)

	// Start session cleanup worker
	go s.cleanupExpiredSessions(s.workerCtx)

	s.logger.Info("VK service workers started")
	return nil
}

func (s *vkService) consumeRegistrationCommands(ctx context.Context) {
	consumer := func(delivery amqp.Delivery) error {
		var command struct {
			AccountID string                       `json:"account_id"`
			Request   models.RegistrationRequest   `json:"request"`
		}

		if err := messaging.DecodeMessage(delivery.Body, &command); err != nil {
			s.logger.Error("Failed to decode registration command", "error", err)
			return err
		}

		accountID, err := primitive.ObjectIDFromHex(command.AccountID)
		if err != nil {
			s.logger.Error("Invalid account ID", "error", err, "account_id", command.AccountID)
			return err
		}

		s.logger.Info("Processing registration command", "account_id", accountID)
		s.metrics.IncrementActiveRegistrations()
		defer s.metrics.DecrementActiveRegistrations()

		// Execute registration
		startTime := time.Now()
		result, err := s.registrationFlow.RegisterAccount(ctx, accountID, &command.Request)
		duration := time.Since(startTime)
		s.metrics.RecordRegistrationDuration(duration)

		if err != nil {
			s.logger.Error("Registration failed", "error", err, "account_id", accountID)
			s.metrics.IncrementRegistrationsTotal("failed")
			s.metrics.IncrementErrorsTotal("registration_error")

			// Publish error event
			s.publishAccountEvent(accountID, "error", err.Error())
			return err
		}

		if result.Success {
			s.metrics.IncrementRegistrationsTotal("success")
			s.publishAccountEvent(accountID, "created", "")
			s.logger.Info("Registration completed", "account_id", accountID, "user_id", result.UserID)
		} else {
			s.metrics.IncrementRegistrationsTotal("failed")
			s.publishAccountEvent(accountID, "error", result.ErrorMessage)
		}

		return nil
	}

	if err := s.messagingClient.ConsumeQueue("vk.register", consumer); err != nil {
		s.logger.Error("Failed to start registration consumer", "error", err)
	}
}

func (s *vkService) consumeRetryCommands(ctx context.Context) {
	consumer := func(delivery amqp.Delivery) error {
		var command struct {
			AccountID  string `json:"account_id"`
			RetryCount int    `json:"retry_count"`
		}

		if err := messaging.DecodeMessage(delivery.Body, &command); err != nil {
			s.logger.Error("Failed to decode retry command", "error", err)
			return err
		}

		accountID, err := primitive.ObjectIDFromHex(command.AccountID)
		if err != nil {
			s.logger.Error("Invalid account ID", "error", err, "account_id", command.AccountID)
			return err
		}

		s.logger.Info("Processing retry command", "account_id", accountID, "retry_count", command.RetryCount)
		s.metrics.IncrementRetryAttempts()

		// Wait with exponential backoff
		backoffDuration := time.Duration(command.RetryCount) * time.Minute
		if backoffDuration > 15*time.Minute {
			backoffDuration = 15 * time.Minute
		}
		time.Sleep(backoffDuration)

		// Execute retry
		result, err := s.registrationFlow.RetryRegistration(ctx, accountID)
		if err != nil {
			s.logger.Error("Retry failed", "error", err, "account_id", accountID)
			s.metrics.IncrementErrorsTotal("retry_error")
			return err
		}

		if result.Success {
			s.publishAccountEvent(accountID, "created", "")
		} else {
			s.publishAccountEvent(accountID, "error", result.ErrorMessage)
		}

		return nil
	}

	if err := s.messagingClient.ConsumeQueue("vk.retry", consumer); err != nil {
		s.logger.Error("Failed to start retry consumer", "error", err)
	}
}

func (s *vkService) monitorStuckRegistrations(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkStuckRegistrations(ctx)
		}
	}
}

func (s *vkService) checkStuckRegistrations(ctx context.Context) {
	stuckAccounts, err := s.accountRepo.GetStuckAccounts(ctx, 30*time.Minute)
	if err != nil {
		s.logger.Error("Failed to get stuck accounts", "error", err)
		return
	}

	for _, account := range stuckAccounts {
		s.logger.Warn("Found stuck registration", "account_id", account.ID, "status", account.Status)

		// Queue for retry
		if err := s.RetryRegistration(ctx, account.ID); err != nil {
			s.logger.Error("Failed to queue stuck account for retry", "error", err, "account_id", account.ID)
			// Mark as error after too many attempts
			if account.RetryCount >= 3 {
				s.accountRepo.UpdateAccountStatus(ctx, account.ID, models.StatusError, "stuck in registration")
			}
		}
	}

	if len(stuckAccounts) > 0 {
		s.logger.Info("Processed stuck registrations", "count", len(stuckAccounts))
	}
}

func (s *vkService) cleanupExpiredSessions(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := s.sessionRepo.CleanupExpiredSessions(ctx, 2*time.Hour)
			if err != nil {
				s.logger.Error("Failed to cleanup expired sessions", "error", err)
			} else if deleted > 0 {
				s.logger.Info("Cleaned up expired sessions", "count", deleted)
			}
		}
	}
}

func (s *vkService) publishAccountEvent(accountID primitive.ObjectID, eventType string, errorMsg string) {
	event := map[string]interface{}{
		"account_id": accountID.Hex(),
		"type":       eventType,
		"timestamp":  time.Now(),
	}

	if errorMsg != "" {
		event["error"] = errorMsg
	}

	routingKey := fmt.Sprintf("vk.account.%s", eventType)
	if err := s.messagingClient.PublishEvent("vk.events", routingKey, event); err != nil {
		s.logger.Error("Failed to publish account event", "error", err, "account_id", accountID, "event_type", eventType)
	}
}

func (s *vkService) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down VK service workers")

	// Cancel worker context
	if s.workerCancel != nil {
		s.workerCancel()
	}

	// Give workers time to finish
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case <-shutdownCtx.Done():
		s.logger.Warn("Worker shutdown timeout exceeded")
	case <-time.After(5 * time.Second):
		s.logger.Info("Workers shut down gracefully")
	}

	return nil
}