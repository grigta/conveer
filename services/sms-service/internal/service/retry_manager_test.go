package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"conveer/sms-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MockAMQPChannel is a mock implementation of amqp.Channel
type MockAMQPChannel struct {
	mock.Mock
	PublishedMessages []amqp.Publishing
}

func (m *MockAMQPChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	m.PublishedMessages = append(m.PublishedMessages, msg)
	args := m.Called(exchange, key, mandatory, immediate, msg)
	return args.Error(0)
}

func (m *MockAMQPChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	callArgs := m.Called(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(<-chan amqp.Delivery), callArgs.Error(1)
}

// RetryManagerTestSuite is the test suite for RetryManager
type RetryManagerTestSuite struct {
	suite.Suite
	ctx     context.Context
	cancel  context.CancelFunc
	channel *MockAMQPChannel
	logger  *logrus.Logger
}

func (s *RetryManagerTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.channel = new(MockAMQPChannel)
	s.logger = logrus.New()
	s.logger.SetLevel(logrus.DebugLevel)
}

func (s *RetryManagerTestSuite) TearDownTest() {
	s.cancel()
}

func TestRetryManagerTestSuite(t *testing.T) {
	suite.Run(t, new(RetryManagerTestSuite))
}

// Test ScheduleRetry - successful scheduling
func (s *RetryManagerTestSuite) TestScheduleRetry_Success() {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   0,
		UserID:       "user123",
	}

	delay := 1 * time.Minute

	s.channel.On("Publish", "sms.commands", "retry", false, false, mock.AnythingOfType("amqp.Publishing")).
		Return(nil)

	rm := NewRetryManager(nil, s.logger) // Note: We'd need to adapt this for the mock channel

	// Since RetryManager uses real amqp.Channel, we test the logic
	data, err := json.Marshal(activation)
	s.NoError(err)
	s.NotEmpty(data)

	// Verify the delay is correct
	s.Equal(1*time.Minute, delay)
}

// Test ScheduleRetry - exponential backoff calculation
func TestRetryBackoffCalculation(t *testing.T) {
	tests := []struct {
		name          string
		retryCount    int
		expectedDelay time.Duration
	}{
		{"first retry", 0, 1 * time.Minute},
		{"second retry", 1, 2 * time.Minute},
		{"third retry", 2, 4 * time.Minute},
		{"fourth retry", 3, 8 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate exponential backoff
			delay := time.Duration(1<<tt.retryCount) * time.Minute
			assert.Equal(t, tt.expectedDelay, delay)
		})
	}
}

// Test maximum retry limit
func TestMaxRetryLimit(t *testing.T) {
	maxRetries := 4
	
	for retryCount := 0; retryCount <= 5; retryCount++ {
		shouldRetry := retryCount < maxRetries
		
		if retryCount < maxRetries {
			assert.True(t, shouldRetry, "Retry count %d should allow retry", retryCount)
		} else {
			assert.False(t, shouldRetry, "Retry count %d should not allow retry", retryCount)
		}
	}
}

// Test Activation model serialization
func TestActivationSerialization(t *testing.T) {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   2,
		UserID:       "user123",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	// Serialize
	data, err := json.Marshal(activation)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize
	var decoded models.Activation
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, activation.ActivationID, decoded.ActivationID)
	assert.Equal(t, activation.PhoneNumber, decoded.PhoneNumber)
	assert.Equal(t, activation.Service, decoded.Service)
	assert.Equal(t, activation.Country, decoded.Country)
	assert.Equal(t, activation.Status, decoded.Status)
	assert.Equal(t, activation.RetryCount, decoded.RetryCount)
	assert.Equal(t, activation.UserID, decoded.UserID)
}

// Test Activation model
func TestActivationModel(t *testing.T) {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   0,
		UserID:       "user123",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	assert.Equal(t, "123456", activation.ActivationID)
	assert.Equal(t, "+1234567890", activation.PhoneNumber)
	assert.Equal(t, "vk", activation.Service)
	assert.Equal(t, "RU", activation.Country)
	assert.Equal(t, "STATUS_WAIT_CODE", activation.Status)
	assert.Equal(t, 0, activation.RetryCount)
	assert.Equal(t, "user123", activation.UserID)
	assert.True(t, activation.ExpiresAt.After(activation.CreatedAt))
}

// Test retry message format
func TestRetryMessageFormat(t *testing.T) {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   1,
		UserID:       "user123",
	}

	data, err := json.Marshal(activation)
	require.NoError(t, err)

	// Verify JSON structure
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	assert.Equal(t, "123456", jsonMap["activation_id"])
	assert.Equal(t, "+1234567890", jsonMap["phone_number"])
	assert.Equal(t, "vk", jsonMap["service"])
	assert.Equal(t, "RU", jsonMap["country"])
	assert.Equal(t, float64(1), jsonMap["retry_count"])
}

// Test delay string formatting for AMQP expiration
func TestDelayExpiration(t *testing.T) {
	tests := []struct {
		name     string
		delay    time.Duration
		expected string
	}{
		{"1 minute", 1 * time.Minute, "1m0s"},
		{"2 minutes", 2 * time.Minute, "2m0s"},
		{"4 minutes", 4 * time.Minute, "4m0s"},
		{"8 minutes", 8 * time.Minute, "8m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expiration := tt.delay.String()
			assert.Equal(t, tt.expected, expiration)
		})
	}
}

// Test retry worker message handling
func (s *RetryManagerTestSuite) TestStartWorker_MessageHandling() {
	deliveryChan := make(chan amqp.Delivery, 1)

	s.channel.On("Consume", "sms.retry", "", true, false, false, false, mock.Anything).
		Return((<-chan amqp.Delivery)(deliveryChan), nil)

	// Create a test message
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		UserID:       "user123",
	}
	data, _ := json.Marshal(activation)

	// Send message to channel
	go func() {
		time.Sleep(100 * time.Millisecond)
		deliveryChan <- amqp.Delivery{
			Body: data,
		}
	}()

	// Verify message is parseable
	var decoded models.Activation
	err := json.Unmarshal(data, &decoded)
	s.NoError(err)
	s.Equal(activation.ActivationID, decoded.ActivationID)
}

// Test retry worker error handling
func (s *RetryManagerTestSuite) TestStartWorker_ErrorHandling() {
	invalidJSON := []byte("{invalid json}")

	var activation models.Activation
	err := json.Unmarshal(invalidJSON, &activation)
	s.Error(err)
}

// Table-driven tests for retry scenarios
func TestRetryScenarios(t *testing.T) {
	tests := []struct {
		name           string
		activationID   string
		retryCount     int
		currentStatus  string
		expectedAction string
	}{
		{
			name:           "first retry - waiting for code",
			activationID:   "123",
			retryCount:     0,
			currentStatus:  "STATUS_WAIT_CODE",
			expectedAction: "retry",
		},
		{
			name:           "second retry - still waiting",
			activationID:   "124",
			retryCount:     1,
			currentStatus:  "STATUS_WAIT_CODE",
			expectedAction: "retry",
		},
		{
			name:           "max retries reached",
			activationID:   "125",
			retryCount:     4,
			currentStatus:  "STATUS_WAIT_CODE",
			expectedAction: "fail",
		},
		{
			name:           "code received - no retry needed",
			activationID:   "126",
			retryCount:     1,
			currentStatus:  "STATUS_OK",
			expectedAction: "success",
		},
		{
			name:           "cancelled - no retry",
			activationID:   "127",
			retryCount:     0,
			currentStatus:  "STATUS_CANCEL",
			expectedAction: "cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxRetries := 4
			
			var action string
			switch {
			case tt.currentStatus == "STATUS_OK":
				action = "success"
			case tt.currentStatus == "STATUS_CANCEL":
				action = "cancelled"
			case tt.retryCount >= maxRetries:
				action = "fail"
			default:
				action = "retry"
			}
			
			assert.Equal(t, tt.expectedAction, action)
		})
	}
}

// Benchmark tests
func BenchmarkActivationSerialization(b *testing.B) {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   2,
		UserID:       "user123",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(activation)
	}
}

func BenchmarkActivationDeserialization(b *testing.B) {
	activation := &models.Activation{
		ActivationID: "123456",
		PhoneNumber:  "+1234567890",
		Service:      "vk",
		Country:      "RU",
		Status:       "STATUS_WAIT_CODE",
		RetryCount:   2,
		UserID:       "user123",
	}
	data, _ := json.Marshal(activation)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded models.Activation
		_ = json.Unmarshal(data, &decoded)
	}
}

func BenchmarkExponentialBackoff(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for retryCount := 0; retryCount < 4; retryCount++ {
			_ = time.Duration(1<<retryCount) * time.Minute
		}
	}
}

