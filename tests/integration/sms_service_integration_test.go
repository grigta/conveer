// +build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"conveer/pkg/testutil"
	"conveer/services/sms-service/internal/model"
	"conveer/services/sms-service/internal/repository"
)

// SMSServiceIntegrationSuite tests SMS service with real MongoDB and mock provider
type SMSServiceIntegrationSuite struct {
	suite.Suite
	ctx              context.Context
	cancel           context.CancelFunc
	mongoContainer   *testutil.MongoContainer
	redisContainer   *testutil.RedisContainer
	rabbitContainer  *testutil.RabbitMQContainer
	activationRepo   repository.ActivationRepository
	mockSMSServer    *httptest.Server
}

func (s *SMSServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 5*time.Minute)

	var err error

	// Start MongoDB
	s.mongoContainer, err = testutil.NewMongoContainer(s.ctx)
	s.Require().NoError(err, "Failed to start MongoDB container")

	// Start Redis
	s.redisContainer, err = testutil.NewRedisContainer(s.ctx)
	s.Require().NoError(err, "Failed to start Redis container")

	// Start RabbitMQ
	s.rabbitContainer, err = testutil.NewRabbitMQContainer(s.ctx)
	s.Require().NoError(err, "Failed to start RabbitMQ container")

	// Initialize repository
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")
	s.activationRepo = repository.NewActivationRepository(db)

	// Setup mock SMS provider
	s.mockSMSServer = s.setupMockSMSServer()
}

func (s *SMSServiceIntegrationSuite) setupMockSMSServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")

		switch action {
		case "getNumber":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ACCESS_NUMBER:12345:79001234567`))

		case "getStatus":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`STATUS_OK:123456`))

		case "setStatus":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ACCESS_ACTIVATION_CANCELED`))

		case "getBalance":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`ACCESS_BALANCE:100.50`))

		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`BAD_ACTION`))
		}
	}))
}

func (s *SMSServiceIntegrationSuite) TearDownSuite() {
	s.cancel()

	if s.mockSMSServer != nil {
		s.mockSMSServer.Close()
	}

	if s.mongoContainer != nil {
		_ = s.mongoContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(context.Background())
	}
	if s.rabbitContainer != nil {
		_ = s.rabbitContainer.Terminate(context.Background())
	}
}

func (s *SMSServiceIntegrationSuite) SetupTest() {
	// Clean up collections before each test
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")
	_ = db.Collection("sms_activations").Drop(s.ctx)
}

// TestActivationRepository_CRUD tests activation CRUD operations
func (s *SMSServiceIntegrationSuite) TestActivationRepository_CRUD() {
	ctx := s.ctx

	// Create activation
	activation := &model.SMSActivation{
		Phone:    "+79001234567",
		Service:  "vk",
		Country:  "ru",
		Status:   model.SMSStatusPending,
		Provider: "sms-activate",
	}

	err := s.activationRepo.Create(ctx, activation)
	s.Require().NoError(err)
	s.NotEmpty(activation.ID)

	// Read activation
	found, err := s.activationRepo.GetByID(ctx, activation.ID)
	s.Require().NoError(err)
	s.Equal(activation.Phone, found.Phone)
	s.Equal(activation.Service, found.Service)

	// Update activation
	activation.Status = model.SMSStatusCodeReceived
	activation.Code = "123456"
	err = s.activationRepo.Update(ctx, activation)
	s.Require().NoError(err)

	// Verify update
	found, err = s.activationRepo.GetByID(ctx, activation.ID)
	s.Require().NoError(err)
	s.Equal(model.SMSStatusCodeReceived, found.Status)
	s.Equal("123456", found.Code)

	// Delete activation
	err = s.activationRepo.Delete(ctx, activation.ID)
	s.Require().NoError(err)

	// Verify delete
	found, err = s.activationRepo.GetByID(ctx, activation.ID)
	s.Error(err)
	s.Nil(found)
}

// TestActivationRepository_FindPending tests finding pending activations
func (s *SMSServiceIntegrationSuite) TestActivationRepository_FindPending() {
	ctx := s.ctx

	// Create activations with different statuses
	activations := []*model.SMSActivation{
		{Phone: "+79001111111", Service: "vk", Status: model.SMSStatusPending},
		{Phone: "+79002222222", Service: "vk", Status: model.SMSStatusWaitingCode},
		{Phone: "+79003333333", Service: "telegram", Status: model.SMSStatusPending},
		{Phone: "+79004444444", Service: "vk", Status: model.SMSStatusCompleted},
	}

	for _, a := range activations {
		err := s.activationRepo.Create(ctx, a)
		s.Require().NoError(err)
	}

	// Find all pending
	pending, err := s.activationRepo.FindByStatus(ctx, model.SMSStatusPending)
	s.Require().NoError(err)
	s.Len(pending, 2)

	// Find waiting for code
	waiting, err := s.activationRepo.FindByStatus(ctx, model.SMSStatusWaitingCode)
	s.Require().NoError(err)
	s.Len(waiting, 1)
}

// TestActivationRepository_ExpiredActivations tests handling of expired activations
func (s *SMSServiceIntegrationSuite) TestActivationRepository_ExpiredActivations() {
	ctx := s.ctx

	// Create activations with different expiry times
	now := time.Now()
	activations := []*model.SMSActivation{
		{
			Phone:     "+79001111111",
			Service:   "vk",
			Status:    model.SMSStatusWaitingCode,
			ExpiresAt: now.Add(-5 * time.Minute), // Expired
		},
		{
			Phone:     "+79002222222",
			Service:   "vk",
			Status:    model.SMSStatusWaitingCode,
			ExpiresAt: now.Add(5 * time.Minute), // Not expired
		},
	}

	for _, a := range activations {
		err := s.activationRepo.Create(ctx, a)
		s.Require().NoError(err)
	}

	// Find expired activations
	expired, err := s.activationRepo.FindExpired(ctx, now)
	s.Require().NoError(err)
	s.Len(expired, 1)
	s.Equal("+79001111111", expired[0].Phone)
}

// TestRetryMechanismWithRedis tests retry state in Redis
func (s *SMSServiceIntegrationSuite) TestRetryMechanismWithRedis() {
	ctx := s.ctx

	redisClient := s.redisContainer.GetClient()

	activationID := "activation-123"

	// Store retry state
	retryKey := "sms:retry:" + activationID
	err := redisClient.HSet(ctx, retryKey,
		"attempt", 1,
		"last_error", "timeout",
		"next_retry", time.Now().Add(time.Minute).Unix(),
	).Err()
	s.Require().NoError(err)

	// Set TTL
	err = redisClient.Expire(ctx, retryKey, 10*time.Minute).Err()
	s.Require().NoError(err)

	// Read retry state
	attempt, err := redisClient.HGet(ctx, retryKey, "attempt").Int()
	s.Require().NoError(err)
	s.Equal(1, attempt)

	lastError, err := redisClient.HGet(ctx, retryKey, "last_error").Result()
	s.Require().NoError(err)
	s.Equal("timeout", lastError)

	// Increment attempt
	newAttempt, err := redisClient.HIncrBy(ctx, retryKey, "attempt", 1).Result()
	s.Require().NoError(err)
	s.Equal(int64(2), newAttempt)
}

// TestSMSEventsWithRabbitMQ tests SMS event publishing
func (s *SMSServiceIntegrationSuite) TestSMSEventsWithRabbitMQ() {
	ctx := s.ctx

	rabbitURL := s.rabbitContainer.GetURL()

	// Create RabbitMQ client
	rabbit, err := testutil.NewTestRabbitMQ(rabbitURL)
	s.Require().NoError(err)
	defer rabbit.Close()

	// Setup exchange and queue
	err = rabbit.DeclareExchange("conveer.events", "topic")
	s.Require().NoError(err)

	queueName := "test.sms.code_received"
	err = rabbit.DeclareQueue(queueName)
	s.Require().NoError(err)

	err = rabbit.BindQueue(queueName, "conveer.events", "sms.code_received")
	s.Require().NoError(err)

	// Publish event
	event := map[string]interface{}{
		"activation_id": "activation-123",
		"phone":         "+79001234567",
		"service":       "vk",
		"code":          "123456",
		"timestamp":     time.Now().Unix(),
	}

	err = rabbit.Publish("conveer.events", "sms.code_received", event)
	s.Require().NoError(err)

	// Consume event
	received := make(chan map[string]interface{}, 1)
	go func() {
		msg, err := rabbit.ConsumeOne(ctx, queueName, 5*time.Second)
		if err == nil {
			received <- msg
		}
	}()

	select {
	case msg := <-received:
		s.Equal("activation-123", msg["activation_id"])
		s.Equal("123456", msg["code"])
	case <-time.After(10 * time.Second):
		s.Fail("Timeout waiting for message")
	}
}

// TestMockProviderIntegration tests integration with mock SMS provider
func (s *SMSServiceIntegrationSuite) TestMockProviderIntegration() {
	// Test get number
	resp, err := http.Get(s.mockSMSServer.URL + "?action=getNumber&service=vk&country=0")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test get status
	resp, err = http.Get(s.mockSMSServer.URL + "?action=getStatus&id=12345")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test get balance
	resp, err = http.Get(s.mockSMSServer.URL + "?action=getBalance")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestSMSServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(SMSServiceIntegrationSuite))
}

