package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/models"
	"github.com/grigta/conveer/services/sms-service/internal/repository"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SMSService struct {
	phoneRepo        *repository.PhoneRepository
	activationRepo   *repository.ActivationRepository
	providerAdapter  *ProviderAdapter
	smsActivate      *SMSActivateClient
	cache            *CacheService
	retryManager     *RetryManager
	metrics          *MetricsCollector
	logger           *logrus.Logger
}

func NewSMSService(
	phoneRepo *repository.PhoneRepository,
	activationRepo *repository.ActivationRepository,
	providerAdapter *ProviderAdapter,
	smsActivate *SMSActivateClient,
	cache *CacheService,
	retryManager *RetryManager,
	metrics *MetricsCollector,
	logger *logrus.Logger,
) *SMSService {
	return &SMSService{
		phoneRepo:        phoneRepo,
		activationRepo:   activationRepo,
		providerAdapter:  providerAdapter,
		smsActivate:      smsActivate,
		cache:            cache,
		retryManager:     retryManager,
		metrics:          metrics,
		logger:           logger,
	}
}

func (s *SMSService) PurchaseNumber(ctx context.Context, userID, service, country, operator, provider string, maxPrice int32) (*models.Activation, error) {
	// Generate activation ID
	activationID := uuid.New().String()

	// Select provider if not specified
	if provider == "" {
		provider = s.providerAdapter.SelectProvider(service, country)
	}

	// Purchase number from provider
	var phone *models.Phone
	var err error

	switch provider {
	case "smsactivate":
		phone, err = s.smsActivate.PurchaseNumber(ctx, service, country, operator, float64(maxPrice))
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if err != nil {
		s.logger.Errorf("Failed to purchase number from %s: %v", provider, err)
		s.metrics.IncrementPurchaseFailed(provider, service)
		return nil, err
	}

	// Save phone to database
	phone.UserID = userID
	phone.ActivationID = activationID
	phone.Status = models.PhoneStatusActive
	phone.ExpiresAt = time.Now().Add(30 * time.Minute)

	if err := s.phoneRepo.Create(ctx, phone); err != nil {
		s.logger.Errorf("Failed to save phone: %v", err)
		return nil, err
	}

	// Create activation
	activation := &models.Activation{
		ActivationID: activationID,
		UserID:       userID,
		PhoneID:      phone.ID,
		PhoneNumber:  phone.Number,
		Service:      service,
		Country:      country,
		Provider:     provider,
		Status:       models.ActivationStatusWaiting,
		Price:        phone.Price,
		ExpiresAt:    phone.ExpiresAt,
	}

	if err := s.activationRepo.Create(ctx, activation); err != nil {
		s.logger.Errorf("Failed to create activation: %v", err)
		return nil, err
	}

	// Cache activation
	s.cache.SetActivation(ctx, activationID, activation, 30*time.Minute)

	// Update metrics
	s.metrics.IncrementPurchaseSuccess(provider, service)
	s.metrics.RecordPurchasePrice(provider, phone.Price)

	s.logger.Infof("Successfully purchased number %s for user %s, activation %s",
		phone.Number, userID, activationID)

	return activation, nil
}

func (s *SMSService) GetSMSCode(ctx context.Context, activationID, userID string) (string, string, error) {
	// Get activation from cache or database
	activation, err := s.cache.GetActivation(ctx, activationID)
	if err != nil || activation == nil {
		activation, err = s.activationRepo.FindByActivationID(ctx, activationID)
		if err != nil {
			return "", "", err
		}
		if activation == nil {
			return "", "", fmt.Errorf("activation not found")
		}
	}

	// Verify user ownership
	if activation.UserID != userID {
		return "", "", fmt.Errorf("activation does not belong to user")
	}

	// Check if code already received
	if activation.Code != "" {
		return activation.Code, activation.FullSMS, nil
	}

	// Check activation status
	if activation.Status == models.ActivationStatusCancelled {
		return "", "", fmt.Errorf("activation is cancelled")
	}
	if activation.Status == models.ActivationStatusExpired {
		return "", "", fmt.Errorf("activation is expired")
	}

	// Get code from provider
	var code, fullSMS string

	switch activation.Provider {
	case "smsactivate":
		code, fullSMS, err = s.smsActivate.GetSMSCode(ctx, activation.ActivationID)
	default:
		return "", "", fmt.Errorf("unsupported provider: %s", activation.Provider)
	}

	if err != nil {
		s.logger.Errorf("Failed to get SMS code for activation %s: %v", activationID, err)
		// Schedule retry
		s.retryManager.ScheduleRetry(ctx, activation, 1*time.Minute)
		return "", "", fmt.Errorf("SMS code not yet received, please try again later")
	}

	// Update activation with code
	if err := s.activationRepo.UpdateCode(ctx, activationID, code, fullSMS); err != nil {
		s.logger.Errorf("Failed to update activation code: %v", err)
		return "", "", err
	}

	// Update cache
	activation.Code = code
	activation.FullSMS = fullSMS
	activation.Status = models.ActivationStatusReceived
	now := time.Now()
	activation.CodeReceivedAt = &now
	s.cache.SetActivation(ctx, activationID, activation, 10*time.Minute)

	// Update metrics
	s.metrics.IncrementCodeReceived(activation.Provider, activation.Service)

	s.logger.Infof("Successfully received SMS code for activation %s", activationID)

	return code, fullSMS, nil
}

func (s *SMSService) CancelActivation(ctx context.Context, activationID, userID, reason string) (bool, float64, error) {
	// Get activation
	activation, err := s.activationRepo.FindByActivationID(ctx, activationID)
	if err != nil {
		return false, 0, err
	}
	if activation == nil {
		return false, 0, fmt.Errorf("activation not found")
	}

	// Verify user ownership
	if activation.UserID != userID {
		return false, 0, fmt.Errorf("activation does not belong to user")
	}

	// Check if already cancelled
	if activation.Status == models.ActivationStatusCancelled {
		return false, 0, fmt.Errorf("activation already cancelled")
	}

	// Check if can be cancelled (no code received)
	refunded := false
	refundAmount := 0.0

	if activation.Code == "" && activation.Status == models.ActivationStatusWaiting {
		// Cancel with provider
		switch activation.Provider {
		case "smsactivate":
			refunded, refundAmount, err = s.smsActivate.CancelActivation(ctx, activation.ActivationID)
		default:
			return false, 0, fmt.Errorf("unsupported provider: %s", activation.Provider)
		}

		if err != nil {
			s.logger.Errorf("Failed to cancel activation with provider: %v", err)
		}
	}

	// Update activation status
	if err := s.activationRepo.CancelActivation(ctx, activationID, reason, refunded, refundAmount); err != nil {
		return false, 0, err
	}

	// Update phone status
	if err := s.phoneRepo.UpdateStatus(ctx, activation.PhoneID, models.PhoneStatusCancelled); err != nil {
		s.logger.Errorf("Failed to update phone status: %v", err)
	}

	// Clear cache
	s.cache.DeleteActivation(ctx, activationID)

	// Update metrics
	s.metrics.IncrementCancellation(activation.Provider, activation.Service, refunded)

	s.logger.Infof("Successfully cancelled activation %s, refunded: %v, amount: %.2f",
		activationID, refunded, refundAmount)

	return refunded, refundAmount, nil
}

func (s *SMSService) GetActivationStatus(ctx context.Context, activationID, userID string) (*models.Activation, error) {
	// Get activation from cache or database
	activation, err := s.cache.GetActivation(ctx, activationID)
	if err != nil || activation == nil {
		activation, err = s.activationRepo.FindByActivationID(ctx, activationID)
		if err != nil {
			return nil, err
		}
		if activation == nil {
			return nil, fmt.Errorf("activation not found")
		}
		// Cache it
		s.cache.SetActivation(ctx, activationID, activation, 5*time.Minute)
	}

	// Verify user ownership
	if activation.UserID != userID {
		return nil, fmt.Errorf("activation does not belong to user")
	}

	return activation, nil
}

func (s *SMSService) GetStatistics(ctx context.Context, userID string, fromDate, toDate time.Time, service, country string) (*models.GetStatisticsResponse, error) {
	filter := bson.M{"user_id": userID}

	if !fromDate.IsZero() {
		filter["created_at"] = bson.M{"$gte": fromDate}
	}
	if !toDate.IsZero() {
		if filter["created_at"] != nil {
			filter["created_at"].(bson.M)["$lte"] = toDate
		} else {
			filter["created_at"] = bson.M{"$lte": toDate}
		}
	}
	if service != "" {
		filter["service"] = service
	}
	if country != "" {
		filter["country"] = country
	}

	stats, err := s.activationRepo.GetStatistics(ctx, filter)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *SMSService) GetProviderBalance(ctx context.Context, provider string) (float64, string, error) {
	// Check cache first
	balance, currency, err := s.cache.GetProviderBalance(ctx, provider)
	if err == nil && balance > 0 {
		return balance, currency, nil
	}

	// Get from provider
	switch provider {
	case "smsactivate":
		balance, currency, err = s.smsActivate.GetBalance(ctx)
	default:
		return 0, "", fmt.Errorf("unsupported provider: %s", provider)
	}

	if err != nil {
		return 0, "", err
	}

	// Cache the balance
	s.cache.SetProviderBalance(ctx, provider, balance, currency, 1*time.Minute)

	return balance, currency, nil
}

func (s *SMSService) StartCodePoller(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollPendingActivations(ctx)
		}
	}
}

func (s *SMSService) pollPendingActivations(ctx context.Context) {
	activations, err := s.activationRepo.FindPending(ctx)
	if err != nil {
		s.logger.Errorf("Failed to find pending activations: %v", err)
		return
	}

	for _, activation := range activations {
		// Skip if recently checked
		if activation.LastRetryAt != nil && time.Since(*activation.LastRetryAt) < 20*time.Second {
			continue
		}

		// Try to get code
		code, fullSMS, err := s.GetSMSCode(ctx, activation.ActivationID, activation.UserID)
		if err == nil && code != "" {
			s.logger.Infof("Received code for activation %s through polling", activation.ActivationID)
		}

		_ = fullSMS // Suppress unused variable warning
	}
}

func (s *SMSService) HandleExpiredActivations(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processExpiredActivations(ctx)
		}
	}
}

func (s *SMSService) processExpiredActivations(ctx context.Context) {
	activations, err := s.activationRepo.FindExpired(ctx)
	if err != nil {
		s.logger.Errorf("Failed to find expired activations: %v", err)
		return
	}

	for _, activation := range activations {
		// Update status to expired
		if err := s.activationRepo.UpdateStatus(ctx, activation.ID, models.ActivationStatusExpired); err != nil {
			s.logger.Errorf("Failed to update activation status to expired: %v", err)
			continue
		}

		// Update phone status
		if err := s.phoneRepo.UpdateStatus(ctx, activation.PhoneID, models.PhoneStatusExpired); err != nil {
			s.logger.Errorf("Failed to update phone status to expired: %v", err)
		}

		// Clear cache
		s.cache.DeleteActivation(ctx, activation.ActivationID)

		s.logger.Infof("Marked activation %s as expired", activation.ActivationID)
	}
}
