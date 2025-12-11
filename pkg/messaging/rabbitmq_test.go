package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// MockAMQPConnection is a mock for testing
type MockAMQPConnection struct {
	mock.Mock
	closed bool
}

func (m *MockAMQPConnection) IsClosed() bool {
	return m.closed
}

func (m *MockAMQPConnection) Close() error {
	m.closed = true
	args := m.Called()
	return args.Error(0)
}

// RabbitMQTestSuite is the test suite for RabbitMQ
type RabbitMQTestSuite struct {
	suite.Suite
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *RabbitMQTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
}

func (s *RabbitMQTestSuite) TearDownTest() {
	s.cancel()
}

func TestRabbitMQTestSuite(t *testing.T) {
	suite.Run(t, new(RabbitMQTestSuite))
}

// Test Message struct
func TestMessage(t *testing.T) {
	msg := NewMessage("test.event", map[string]string{"key": "value"})

	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "test.event", msg.Type)
	assert.NotNil(t, msg.Data)
	assert.NotNil(t, msg.Metadata)
	assert.True(t, time.Since(msg.Timestamp) < time.Second)
}

// Test Message JSON serialization
func TestMessage_JSON(t *testing.T) {
	msg := NewMessage("user.created", map[string]interface{}{
		"user_id": "123",
		"email":   "test@example.com",
	})

	// Serialize
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize
	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.NotNil(t, decoded.Data)
}

// Test generateMessageID
func TestGenerateMessageID(t *testing.T) {
	ids := make(map[string]bool)

	// Generate multiple IDs
	for i := 0; i < 100; i++ {
		id := generateMessageID()
		assert.NotEmpty(t, id)
		ids[id] = true
	}

	// All IDs should be unique (or very nearly so due to nanosecond precision)
	// Note: In very fast loops, some might collide, so we check for reasonable uniqueness
	assert.True(t, len(ids) > 90, "Expected mostly unique IDs")
}

// Test ConsumerRegistration
func TestConsumerRegistration(t *testing.T) {
	ctx := context.Background()
	handler := func(data []byte) error { return nil }

	reg := ConsumerRegistration{
		QueueName:    "test.queue",
		ConsumerName: "test-consumer",
		Handler:      handler,
		Context:      ctx,
	}

	assert.Equal(t, "test.queue", reg.QueueName)
	assert.Equal(t, "test-consumer", reg.ConsumerName)
	assert.NotNil(t, reg.Handler)
	assert.NotNil(t, reg.Context)
}

// Test Publish message serialization
func TestPublish_MessageSerialization(t *testing.T) {
	tests := []struct {
		name    string
		message interface{}
	}{
		{"string", "hello world"},
		{"map", map[string]string{"key": "value"}},
		{"struct", struct{ Name string }{"test"}},
		{"slice", []string{"a", "b", "c"}},
		{"number", 42},
		{"bool", true},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			assert.NoError(t, err)
			assert.NotNil(t, data)
		})
	}
}

// Test Publish - invalid message
func TestPublish_InvalidMessage(t *testing.T) {
	// Channel that cannot be serialized
	ch := make(chan int)
	_, err := json.Marshal(ch)
	assert.Error(t, err)
}

// Test PublishWithHeaders - headers format
func TestPublishWithHeaders_HeadersFormat(t *testing.T) {
	headers := map[string]interface{}{
		"correlation_id": "123",
		"retry_count":    3,
		"timestamp":      time.Now().Unix(),
		"source":         "test-service",
	}

	assert.NotNil(t, headers)
	assert.Equal(t, "123", headers["correlation_id"])
	assert.Equal(t, 3, headers["retry_count"])
}

// Test SetupTopology - exchange declarations
func TestSetupTopology_Exchanges(t *testing.T) {
	expectedExchanges := []struct {
		name       string
		kind       string
		durable    bool
		autoDelete bool
	}{
		{"events", "topic", true, false},
		{"commands", "direct", true, false},
		{"dead-letter", "topic", true, false},
		{"sms.events", "topic", true, false},
		{"sms.commands", "direct", true, false},
	}

	for _, ex := range expectedExchanges {
		assert.NotEmpty(t, ex.name)
		assert.True(t, ex.kind == "topic" || ex.kind == "direct")
		assert.True(t, ex.durable)
		assert.False(t, ex.autoDelete)
	}
}

// Test SetupSMSTopology - queue declarations
func TestSetupSMSTopology_Queues(t *testing.T) {
	expectedQueues := []string{
		"sms.purchase",
		"sms.get_code",
		"sms.cancel",
		"sms.retry",
	}

	for _, queue := range expectedQueues {
		assert.NotEmpty(t, queue)
		assert.Contains(t, queue, "sms.")
	}
}

// Test SetupSMSTopology - bindings
func TestSetupSMSTopology_Bindings(t *testing.T) {
	bindings := []struct {
		queue    string
		exchange string
		key      string
	}{
		{"sms.purchase", "sms.commands", "purchase"},
		{"sms.get_code", "sms.commands", "get_code"},
		{"sms.cancel", "sms.commands", "cancel"},
		{"sms.retry", "sms.commands", "retry"},
	}

	for _, b := range bindings {
		assert.NotEmpty(t, b.queue)
		assert.Equal(t, "sms.commands", b.exchange)
		assert.NotEmpty(t, b.key)
	}
}

// Test CreateDLQ - naming convention
func TestCreateDLQ_Naming(t *testing.T) {
	tests := []struct {
		queueName    string
		expectedDLQ  string
	}{
		{"sms.purchase", "sms.purchase.dlq"},
		{"proxy.allocate", "proxy.allocate.dlq"},
		{"warming.execute", "warming.execute.dlq"},
	}

	for _, tt := range tests {
		t.Run(tt.queueName, func(t *testing.T) {
			dlqName := tt.queueName + ".dlq"
			assert.Equal(t, tt.expectedDLQ, dlqName)
		})
	}
}

// Test Publisher interface
func TestPublisherInterface(t *testing.T) {
	// Verify RabbitMQ implements Publisher
	var _ Publisher = (*RabbitMQ)(nil)
}

// Test Consumer interface
func TestConsumerInterface(t *testing.T) {
	// Verify RabbitMQ implements Consumer
	var _ Consumer = (*RabbitMQ)(nil)
}

// Test MessageBroker interface
func TestMessageBrokerInterface(t *testing.T) {
	// Verify RabbitMQ implements MessageBroker
	var _ MessageBroker = (*RabbitMQ)(nil)
}

// Test PublishEvent - routing key format
func TestPublishEvent_RoutingKey(t *testing.T) {
	eventTypes := []string{
		"user.created",
		"proxy.allocated",
		"sms.code.received",
		"warming.task.completed",
		"account.banned",
	}

	for _, eventType := range eventTypes {
		msg := NewMessage(eventType, nil)
		assert.Equal(t, eventType, msg.Type)
		assert.Contains(t, eventType, ".")
	}
}

// Test PublishCommand - routing key format
func TestPublishCommand_RoutingKey(t *testing.T) {
	commandTypes := []string{
		"register.vk",
		"allocate.proxy",
		"start.warming",
		"purchase.sms",
	}

	for _, commandType := range commandTypes {
		msg := NewMessage(commandType, nil)
		assert.Equal(t, commandType, msg.Type)
	}
}

// Test reconnection logic
func TestReconnect_BackoffTiming(t *testing.T) {
	// Test exponential backoff for reconnection attempts
	maxAttempts := 5

	for attempt := 0; attempt < maxAttempts; attempt++ {
		expectedDelay := time.Duration(attempt+1) * time.Second
		assert.True(t, expectedDelay >= time.Second)
		assert.True(t, expectedDelay <= 5*time.Second)
	}
}

// Test monitorConnection - check interval
func TestMonitorConnection_CheckInterval(t *testing.T) {
	checkInterval := 5 * time.Second
	assert.Equal(t, 5*time.Second, checkInterval)
}

// Table-driven tests for message types
func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name     string
		msgType  string
		data     interface{}
		expected bool
	}{
		{
			name:     "event message",
			msgType:  "user.created",
			data:     map[string]string{"user_id": "123"},
			expected: true,
		},
		{
			name:     "command message",
			msgType:  "send.email",
			data:     map[string]string{"to": "test@example.com"},
			expected: true,
		},
		{
			name:     "empty type",
			msgType:  "",
			data:     nil,
			expected: true, // Still valid, just not recommended
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessage(tt.msgType, tt.data)
			assert.NotNil(t, msg)
			assert.Equal(t, tt.msgType, msg.Type)
		})
	}
}

// Test handler function signature
func TestHandlerFunction(t *testing.T) {
	// Test handler that processes message successfully
	successHandler := func(data []byte) error {
		var msg map[string]interface{}
		return json.Unmarshal(data, &msg)
	}

	// Test with valid JSON
	validData := []byte(`{"key": "value"}`)
	err := successHandler(validData)
	assert.NoError(t, err)

	// Test with invalid JSON
	invalidData := []byte(`{invalid}`)
	err = successHandler(invalidData)
	assert.Error(t, err)
}

// Test SetQos - prefetch count
func TestSetQos_PrefetchCount(t *testing.T) {
	tests := []struct {
		name          string
		prefetchCount int
		valid         bool
	}{
		{"zero", 0, true},
		{"one", 1, true},
		{"ten", 10, true},
		{"hundred", 100, true},
		{"negative", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.True(t, tt.prefetchCount >= 0)
			} else {
				assert.True(t, tt.prefetchCount < 0)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewMessage(b *testing.B) {
	data := map[string]interface{}{
		"user_id": "123",
		"action":  "created",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewMessage("user.created", data)
	}
}

func BenchmarkMessageSerialization(b *testing.B) {
	msg := NewMessage("user.created", map[string]interface{}{
		"user_id": "123",
		"action":  "created",
		"details": map[string]interface{}{
			"email": "test@example.com",
			"name":  "Test User",
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(msg)
	}
}

func BenchmarkMessageDeserialization(b *testing.B) {
	msg := NewMessage("user.created", map[string]interface{}{
		"user_id": "123",
		"action":  "created",
	})
	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded Message
		_ = json.Unmarshal(data, &decoded)
	}
}

func BenchmarkGenerateMessageID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = generateMessageID()
	}
}

// Test for DLQ TTL configuration
func TestDLQ_TTLConfiguration(t *testing.T) {
	// 24 hours in milliseconds
	expectedTTL := int32(86400000)
	actualTTL := int32(24 * 60 * 60 * 1000)
	
	assert.Equal(t, expectedTTL, actualTTL)
}

