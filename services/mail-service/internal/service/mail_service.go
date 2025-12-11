package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/grigta/conveer/services/mail-service/internal/models"
	"github.com/grigta/conveer/services/mail-service/internal/repository"
	proxypb "github.com/grigta/conveer/services/proxy-service/proto"
	smspb "github.com/grigta/conveer/services/sms-service/proto"
	"github.com/streadway/amqp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

// RegistrationTaskPayload represents the payload for registration tasks
type RegistrationTaskPayload struct {
	AccountID            string                       `json:"accountID"`
	RegistrationRequest  *models.RegistrationRequest  `json:"registrationRequest"`
}

// RetryTaskPayload represents the payload for retry tasks
type RetryTaskPayload struct {
	AccountID   string `json:"accountID"`
	RetryCount  int    `json:"retryCount,omitempty"`
}

// MailService represents the mail service
type MailService struct {
	accountRepo      *repository.AccountRepository
	sessionRepo      *repository.SessionRepository
	proxyClient      proxypb.ProxyServiceClient
	smsClient        smspb.SMSServiceClient
	rabbitmqChannel  *amqp.Channel
	browserManager   *BrowserManager
	config           *models.RegistrationConfig
	metrics          *MetricsCollector
}

// NewMailService creates a new mail service instance
func NewMailService(
	accountRepo *repository.AccountRepository,
	sessionRepo *repository.SessionRepository,
	proxyConn *grpc.ClientConn,
	smsConn *grpc.ClientConn,
	rabbitmqChannel *amqp.Channel,
	browserManager *BrowserManager,
	config *models.RegistrationConfig,
) *MailService {
	return &MailService{
		accountRepo:      accountRepo,
		sessionRepo:      sessionRepo,
		proxyClient:      proxypb.NewProxyServiceClient(proxyConn),
		smsClient:        smspb.NewSMSServiceClient(smsConn),
		rabbitmqChannel:  rabbitmqChannel,
		browserManager:   browserManager,
		config:           config,
		metrics:          NewMetricsCollector(),
	}
}

// CreateAccount creates a new mail account
func (s *MailService) CreateAccount(ctx context.Context, req *models.RegistrationRequest) (*models.RegistrationResult, error) {
	s.metrics.IncrementRegistrationAttempts()
	
	// Create account document
	account := &models.MailAccount{
		ID:        primitive.NewObjectID(),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		BirthDate: req.BirthDate,
		Gender:    req.Gender,
		Status:    models.AccountStatusCreating,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Save account to database
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}
	
	// Create registration session
	session := &models.RegistrationSession{
		ID:                   primitive.NewObjectID(),
		AccountID:            account.ID,
		CurrentStep:          models.StepProxyAllocation,
		UsePhoneVerification: req.UsePhoneVerification && s.config.EnablePhoneVerification,
		StepCheckpoints:      make(map[string]interface{}),
		StartedAt:            time.Now(),
		LastActivityAt:       time.Now(),
	}
	
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	// Publish to registration queue
	if err := s.publishRegistrationTask(account.ID.Hex(), req); err != nil {
		return nil, fmt.Errorf("failed to publish registration task: %w", err)
	}
	
	return &models.RegistrationResult{
		Success:     true,
		AccountID:   account.ID.Hex(),
		Status:      string(models.AccountStatusCreating),
		CompletedAt: time.Now(),
	}, nil
}

// GetAccount retrieves an account by ID
func (s *MailService) GetAccount(ctx context.Context, accountID string) (*models.MailAccount, error) {
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %w", err)
	}
	
	return s.accountRepo.GetByID(ctx, id)
}

// ListAccounts lists all accounts with filters
func (s *MailService) ListAccounts(ctx context.Context, filter map[string]interface{}, limit, offset int) ([]*models.MailAccount, int64, error) {
	return s.accountRepo.List(ctx, filter, limit, offset)
}

// UpdateAccountStatus updates the status of an account
func (s *MailService) UpdateAccountStatus(ctx context.Context, accountID string, status models.AccountStatus, errorMsg string) error {
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}
	
	return s.accountRepo.UpdateAccountStatus(ctx, id, status, errorMsg)
}

// RetryRegistration retries a failed registration
func (s *MailService) RetryRegistration(ctx context.Context, accountID string) error {
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}
	
	// Get account
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	
	// Check retry count
	if account.RetryCount >= s.config.MaxRetryAttempts {
		return fmt.Errorf("max retry attempts reached")
	}
	
	// Increment retry count
	if err := s.accountRepo.IncrementRetryCount(ctx, id); err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}
	
	// Get session
	session, err := s.sessionRepo.GetSession(ctx, id)
	if err != nil {
		// Create new session if not exists
		session = &models.RegistrationSession{
			ID:              primitive.NewObjectID(),
			AccountID:       id,
			CurrentStep:     models.StepProxyAllocation,
			StepCheckpoints: make(map[string]interface{}),
			StartedAt:       time.Now(),
			LastActivityAt:  time.Now(),
			RetryCount:      account.RetryCount + 1,
		}
		
		if err := s.sessionRepo.Create(ctx, session); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}
	
	// Publish retry task
	if err := s.publishRetryTask(accountID); err != nil {
		return fmt.Errorf("failed to publish retry task: %w", err)
	}
	
	return nil
}

// DeleteAccount deletes an account
func (s *MailService) DeleteAccount(ctx context.Context, accountID string) error {
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}
	
	return s.accountRepo.Delete(ctx, id)
}

// GetStatistics returns account statistics
func (s *MailService) GetStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	return s.accountRepo.GetStatistics(ctx)
}

// StartWorkers starts background workers
func (s *MailService) StartWorkers(ctx context.Context) {
	go s.registrationWorker(ctx)
	go s.retryWorker(ctx)
	go s.cleanupWorker(ctx)
	go s.stuckSessionMonitor(ctx)
}

// registrationWorker processes registration tasks
func (s *MailService) registrationWorker(ctx context.Context) {
	msgs, err := s.rabbitmqChannel.Consume(
		"mail.register",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("Failed to register consumer: %v", err)
		return
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			// Process registration
			if err := s.processRegistration(ctx, msg.Body); err != nil {
				log.Printf("Registration failed: %v", err)
				msg.Nack(false, true)
			} else {
				msg.Ack(false)
			}
		}
	}
}

// retryWorker processes retry tasks
func (s *MailService) retryWorker(ctx context.Context) {
	msgs, err := s.rabbitmqChannel.Consume(
		"mail.retry",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("Failed to register retry consumer: %v", err)
		return
	}
	
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			// Process retry
			if err := s.processRetry(ctx, msg.Body); err != nil {
				log.Printf("Retry failed: %v", err)
				msg.Nack(false, true)
			} else {
				msg.Ack(false)
			}
		}
	}
}

// cleanupWorker cleans up stuck sessions
func (s *MailService) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Clean up sessions older than 1 hour
			if err := s.sessionRepo.CleanupStuckSessions(ctx, 1*time.Hour); err != nil {
				log.Printf("Failed to cleanup stuck sessions: %v", err)
			}
		}
	}
}

// stuckSessionMonitor monitors for stuck sessions
func (s *MailService) stuckSessionMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Find sessions stuck in same step for >30 minutes
			sessions, err := s.sessionRepo.GetStuckSessions(ctx, 30*time.Minute)
			if err != nil {
				log.Printf("Failed to get stuck sessions: %v", err)
				continue
			}
			
			for _, session := range sessions {
				// Trigger retry or manual intervention
				if session.RetryCount < s.config.MaxRetryAttempts {
					s.publishRetryTask(session.AccountID.Hex())
				} else {
					s.publishManualIntervention(session.AccountID.Hex(), "Session stuck for >30 minutes")
				}
			}
		}
	}
}

// Helper methods

func (s *MailService) publishRegistrationTask(accountID string, req *models.RegistrationRequest) error {
	payload := RegistrationTaskPayload{
		AccountID:           accountID,
		RegistrationRequest: req,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration task: %w", err)
	}

	return s.rabbitmqChannel.Publish(
		"mail.commands",  // exchange
		"mail.register", // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:       data,
		},
	)
}

func (s *MailService) publishRetryTask(accountID string) error {
	// Get account to include retry count
	id, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	account, err := s.accountRepo.GetByID(context.Background(), id)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	payload := RetryTaskPayload{
		AccountID:  accountID,
		RetryCount: account.RetryCount,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal retry task: %w", err)
	}

	return s.rabbitmqChannel.Publish(
		"mail.commands", // exchange
		"mail.retry",   // routing key
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:       data,
		},
	)
}

func (s *MailService) publishManualIntervention(accountID string, reason string) error {
	s.metrics.IncrementManualIntervention(reason)

	// Create payload
	payload := map[string]interface{}{
		"account_id": accountID,
		"reason":     reason,
		"service":    "mail-service",
		"timestamp":  time.Now().Unix(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal manual intervention payload: %w", err)
	}

	// Publish to RabbitMQ
	return s.rabbitmqChannel.Publish(
		"mail.events",             // exchange
		"mail.manual_intervention", // routing key
		false,                     // mandatory
		false,                     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:       data,
		},
	)
}

func (s *MailService) processRegistration(ctx context.Context, data []byte) error {
	var payload RegistrationTaskPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal registration task: %w", err)
	}

	// Convert string ID to ObjectID
	accountID, err := primitive.ObjectIDFromHex(payload.AccountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	// Create registration flow
	flow, err := s.NewRegistrationFlow(ctx, accountID)
	if err != nil {
		s.metrics.IncrementRegistrationFailures(err.Error())
		return fmt.Errorf("failed to create registration flow: %w", err)
	}

	// Execute registration
	if err := flow.Execute(); err != nil {
		s.metrics.IncrementRegistrationFailures(err.Error())
		// Update account status to failed
		s.accountRepo.UpdateAccountStatus(ctx, accountID, models.AccountStatusFailed, err.Error())
		return fmt.Errorf("registration execution failed: %w", err)
	}

	s.metrics.IncrementRegistrationSuccess()
	return nil
}

func (s *MailService) processRetry(ctx context.Context, data []byte) error {
	var payload RetryTaskPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal retry task: %w", err)
	}

	// Convert string ID to ObjectID
	accountID, err := primitive.ObjectIDFromHex(payload.AccountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	// Get account for retry count check
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Create registration flow for retry (it will fetch session internally)
	flow, err := s.NewRegistrationFlow(ctx, accountID)
	if err != nil {
		s.metrics.IncrementRegistrationFailures(err.Error())
		return fmt.Errorf("failed to create registration flow: %w", err)
	}

	// Execute retry from current step
	if err := flow.Execute(); err != nil {
		s.metrics.IncrementRegistrationFailures(err.Error())
		// Update account status
		if account.RetryCount >= s.config.MaxRetryAttempts {
			s.accountRepo.UpdateAccountStatus(ctx, accountID, models.AccountStatusFailed,
				fmt.Sprintf("Max retries exceeded: %v", err))
		}
		return fmt.Errorf("retry execution failed: %w", err)
	}

	s.metrics.IncrementRegistrationSuccess()
	return nil
}
