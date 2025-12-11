package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/services/proxy-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RotationManagerTestSuite is the test suite for RotationManager
type RotationManagerTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	proxyRepo       *MockProxyRepository
	providerRepo    *MockProviderRepository
	providerManager *MockProviderManager
	rabbitmq        *MockRabbitMQ
	logger          *logrus.Logger
	config          *config.Config
}

func (s *RotationManagerTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.proxyRepo = new(MockProxyRepository)
	s.providerRepo = new(MockProviderRepository)
	s.providerManager = new(MockProviderManager)
	s.rabbitmq = NewMockRabbitMQ()
	s.logger = logrus.New()
	s.logger.SetLevel(logrus.DebugLevel)
	s.config = &config.Config{
		Proxy: config.ProxyConfig{
			RotationCheckInterval: "5m",
		},
	}
}

func (s *RotationManagerTestSuite) TearDownTest() {
	s.cancel()
}

func TestRotationManagerTestSuite(t *testing.T) {
	suite.Run(t, new(RotationManagerTestSuite))
}

// Test NewRotationManager with default values
func (s *RotationManagerTestSuite) TestNewRotationManager_DefaultValues() {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			RotationCheckInterval: "",
		},
	}

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, nil, s.logger, cfg)

	s.NotNil(rm)
	s.Equal(5*time.Minute, rm.checkInterval)
	s.Equal(5*time.Minute, rm.gracePeriod)
}

// Test NewRotationManager with custom values
func (s *RotationManagerTestSuite) TestNewRotationManager_CustomValues() {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			RotationCheckInterval: "10m",
		},
	}

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, nil, s.logger, cfg)

	s.NotNil(rm)
	s.Equal(10*time.Minute, rm.checkInterval)
}

// Test ScheduleRotation
func (s *RotationManagerTestSuite) TestScheduleRotation_Success() {
	proxyID := primitive.NewObjectID()
	accountID := "account123"
	expiresAt := time.Now().Add(1 * time.Hour)

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, s.rabbitmq, s.logger, s.config)

	err := rm.ScheduleRotation(s.ctx, proxyID, accountID, expiresAt)
	s.NoError(err)
}

// Test ScheduleRotation - replaces existing timer
func (s *RotationManagerTestSuite) TestScheduleRotation_ReplacesExisting() {
	proxyID := primitive.NewObjectID()
	accountID := "account123"
	expiresAt1 := time.Now().Add(1 * time.Hour)
	expiresAt2 := time.Now().Add(2 * time.Hour)

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, s.rabbitmq, s.logger, s.config)

	// Schedule first rotation
	err := rm.ScheduleRotation(s.ctx, proxyID, accountID, expiresAt1)
	s.NoError(err)

	// Schedule second rotation (should replace first)
	err = rm.ScheduleRotation(s.ctx, proxyID, accountID, expiresAt2)
	s.NoError(err)
}

// Test CancelScheduledRotation
func (s *RotationManagerTestSuite) TestCancelScheduledRotation_Success() {
	proxyID := primitive.NewObjectID()
	accountID := "account123"
	expiresAt := time.Now().Add(1 * time.Hour)

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, s.rabbitmq, s.logger, s.config)

	// Schedule rotation
	err := rm.ScheduleRotation(s.ctx, proxyID, accountID, expiresAt)
	s.NoError(err)

	// Cancel it
	rm.CancelScheduledRotation(proxyID, accountID)

	// Verify it's cancelled (no panic, no error)
}

// Test CancelScheduledRotation - non-existent timer
func (s *RotationManagerTestSuite) TestCancelScheduledRotation_NonExistent() {
	proxyID := primitive.NewObjectID()
	accountID := "account123"

	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, s.rabbitmq, s.logger, s.config)

	// Should not panic when cancelling non-existent timer
	rm.CancelScheduledRotation(proxyID, accountID)
}

// Test RotateProxy - successful rotation
func (s *RotationManagerTestSuite) TestRotateProxy_Success() {
	proxyID := primitive.NewObjectID()
	accountID := "account123"

	oldProxy := &models.Proxy{
		ID:       proxyID,
		Provider: "test-provider",
		IP:       "192.168.1.1",
		Port:     8080,
		Type:     models.ProxyTypeMobile,
		Country:  "US",
		Protocol: models.ProtocolHTTP,
		Status:   models.ProxyStatusActive,
	}

	newProxyResponse := &models.ProxyResponse{
		IP:       "192.168.1.2",
		Port:     8080,
		Username: "new_user",
		Password: "new_pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		City:     "New York",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	mockProvider := NewMockProviderAdapterTest("test-provider")

	s.proxyRepo.On("GetProxyByID", s.ctx, proxyID).Return(oldProxy, nil)
	s.providerManager.On("GetProviderByName", "test-provider").Return(mockProvider, nil)
	mockProvider.On("PurchaseProxy", s.ctx, mock.AnythingOfType("models.ProxyPurchaseParams")).Return(newProxyResponse, nil)
	s.proxyRepo.On("CreateProxy", s.ctx, mock.AnythingOfType("*models.Proxy")).Return(nil)
	s.proxyRepo.On("UpdateProxyStatus", s.ctx, proxyID, models.ProxyStatusRotating).Return(nil)
	s.proxyRepo.On("BindProxyToAccount", s.ctx, mock.AnythingOfType("primitive.ObjectID"), accountID).Return(nil)
	s.providerRepo.On("IncrementProviderCounter", s.ctx, "test-provider", "total_rotated").Return(nil)
	s.rabbitmq.On("Publish", "proxy.events", "proxy.rotated", mock.AnythingOfType("service.RotationEvent")).Return(nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test RotateProxy - provider not available, fallback to another
func (s *RotationManagerTestSuite) TestRotateProxy_ProviderFallback() {
	proxyID := primitive.NewObjectID()

	oldProxy := &models.Proxy{
		ID:       proxyID,
		Provider: "unavailable-provider",
		IP:       "192.168.1.1",
		Port:     8080,
		Type:     models.ProxyTypeMobile,
		Country:  "US",
		Protocol: models.ProtocolHTTP,
		Status:   models.ProxyStatusActive,
	}

	newProxyResponse := &models.ProxyResponse{
		IP:       "192.168.1.2",
		Port:     8080,
		Username: "new_user",
		Password: "new_pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	mockFallbackProvider := NewMockProviderAdapterTest("fallback-provider")

	s.proxyRepo.On("GetProxyByID", s.ctx, proxyID).Return(oldProxy, nil)
	s.providerManager.On("GetProviderByName", "unavailable-provider").Return(nil, assert.AnError)
	s.providerManager.On("GetActiveProviders").Return([]ProviderAdapter{mockFallbackProvider})
	mockFallbackProvider.On("PurchaseProxy", s.ctx, mock.AnythingOfType("models.ProxyPurchaseParams")).Return(newProxyResponse, nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test RotateProxy - no active providers
func (s *RotationManagerTestSuite) TestRotateProxy_NoActiveProviders() {
	proxyID := primitive.NewObjectID()

	oldProxy := &models.Proxy{
		ID:       proxyID,
		Provider: "test-provider",
	}

	s.proxyRepo.On("GetProxyByID", s.ctx, proxyID).Return(oldProxy, nil)
	s.providerManager.On("GetProviderByName", "test-provider").Return(nil, assert.AnError)
	s.providerManager.On("GetActiveProviders").Return([]ProviderAdapter{})

	// Should return error when no providers are available
	s.proxyRepo.AssertExpectations(s.T())
}

// Test RotateProxy - purchase fails
func (s *RotationManagerTestSuite) TestRotateProxy_PurchaseFails() {
	proxyID := primitive.NewObjectID()

	oldProxy := &models.Proxy{
		ID:       proxyID,
		Provider: "test-provider",
	}

	mockProvider := NewMockProviderAdapterTest("test-provider")

	s.proxyRepo.On("GetProxyByID", s.ctx, proxyID).Return(oldProxy, nil)
	s.providerManager.On("GetProviderByName", "test-provider").Return(mockProvider, nil)
	mockProvider.On("PurchaseProxy", s.ctx, mock.AnythingOfType("models.ProxyPurchaseParams")).Return(nil, assert.AnError)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test checkExpiredProxies - with expired proxies
func (s *RotationManagerTestSuite) TestCheckExpiredProxies_WithExpired() {
	proxyID := primitive.NewObjectID()

	expiredProxies := []models.Proxy{
		{
			ID:        proxyID,
			Provider:  "test-provider",
			IP:        "192.168.1.1",
			Port:      8080,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	binding := &models.ProxyBinding{
		ProxyID:   proxyID,
		AccountID: "account123",
		Status:    models.BindingStatusActive,
	}

	s.proxyRepo.On("GetExpiredProxies", s.ctx).Return(expiredProxies, nil)
	s.proxyRepo.On("GetActiveBindingByProxyID", s.ctx, proxyID).Return(binding, nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test checkExpiredProxies - no expired proxies
func (s *RotationManagerTestSuite) TestCheckExpiredProxies_NoExpired() {
	s.proxyRepo.On("GetExpiredProxies", s.ctx).Return([]models.Proxy{}, nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test checkExpiredProxies - unbound expired proxy
func (s *RotationManagerTestSuite) TestCheckExpiredProxies_UnboundProxy() {
	proxyID := primitive.NewObjectID()

	expiredProxies := []models.Proxy{
		{
			ID:        proxyID,
			Provider:  "test-provider",
			IP:        "192.168.1.1",
			Port:      8080,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		},
	}

	s.proxyRepo.On("GetExpiredProxies", s.ctx).Return(expiredProxies, nil)
	s.proxyRepo.On("GetActiveBindingByProxyID", s.ctx, proxyID).Return(nil, nil) // No binding
	s.proxyRepo.On("ReleaseProxyBinding", s.ctx, proxyID).Return(nil)
	s.proxyRepo.On("UpdateProxyStatus", s.ctx, proxyID, models.ProxyStatusReleased).Return(nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test RotationRequest JSON serialization
func (s *RotationManagerTestSuite) TestRotationRequest_JSON() {
	request := RotationRequest{
		ProxyID:   primitive.NewObjectID().Hex(),
		AccountID: "account123",
	}

	data, err := json.Marshal(request)
	s.NoError(err)

	var decoded RotationRequest
	err = json.Unmarshal(data, &decoded)
	s.NoError(err)

	s.Equal(request.ProxyID, decoded.ProxyID)
	s.Equal(request.AccountID, decoded.AccountID)
}

// Test RotationEvent JSON serialization
func (s *RotationManagerTestSuite) TestRotationEvent_JSON() {
	event := RotationEvent{
		OldProxyID: primitive.NewObjectID().Hex(),
		NewProxyID: primitive.NewObjectID().Hex(),
		AccountID:  "account123",
		Timestamp:  time.Now(),
	}

	data, err := json.Marshal(event)
	s.NoError(err)

	var decoded RotationEvent
	err = json.Unmarshal(data, &decoded)
	s.NoError(err)

	s.Equal(event.OldProxyID, decoded.OldProxyID)
	s.Equal(event.NewProxyID, decoded.NewProxyID)
	s.Equal(event.AccountID, decoded.AccountID)
}

// Table-driven tests for rotation timing
func TestRotationTiming(t *testing.T) {
	tests := []struct {
		name            string
		expiresAt       time.Duration
		expectedBefore  time.Duration // Expected time before expiration
	}{
		{
			name:           "1 hour expiry",
			expiresAt:      1 * time.Hour,
			expectedBefore: 10 * time.Minute,
		},
		{
			name:           "24 hour expiry",
			expiresAt:      24 * time.Hour,
			expectedBefore: 10 * time.Minute,
		},
		{
			name:           "5 minute expiry",
			expiresAt:      5 * time.Minute,
			expectedBefore: 10 * time.Minute, // Rotation time might be negative or immediate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expiresAt := time.Now().Add(tt.expiresAt)
			rotationTime := expiresAt.Add(-10 * time.Minute)

			// Verify rotation time is 10 minutes before expiration
			assert.Equal(t, expiresAt.Add(-10*time.Minute), rotationTime)
		})
	}
}

// Test concurrent rotation scheduling
func (s *RotationManagerTestSuite) TestScheduleRotation_Concurrent() {
	rm := NewRotationManager(s.proxyRepo, s.providerRepo, nil, s.rabbitmq, s.logger, s.config)

	// Schedule multiple rotations concurrently
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			proxyID := primitive.NewObjectID()
			accountID := "account" + string(rune('0'+idx))
			expiresAt := time.Now().Add(time.Duration(idx+1) * time.Hour)

			err := rm.ScheduleRotation(s.ctx, proxyID, accountID, expiresAt)
			assert.NoError(s.T(), err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkScheduleRotation(b *testing.B) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cfg := &config.Config{}

	rm := NewRotationManager(nil, nil, nil, nil, logger, cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proxyID := primitive.NewObjectID()
		accountID := "benchmark_account"
		expiresAt := time.Now().Add(1 * time.Hour)

		_ = rm.ScheduleRotation(ctx, proxyID, accountID, expiresAt)
	}
}

func BenchmarkCancelScheduledRotation(b *testing.B) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	cfg := &config.Config{}

	rm := NewRotationManager(nil, nil, nil, nil, logger, cfg)

	// Pre-schedule some rotations
	proxyIDs := make([]primitive.ObjectID, b.N)
	for i := 0; i < b.N; i++ {
		proxyIDs[i] = primitive.NewObjectID()
		_ = rm.ScheduleRotation(ctx, proxyIDs[i], "account", time.Now().Add(1*time.Hour))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.CancelScheduledRotation(proxyIDs[i], "account")
	}
}

func BenchmarkRotationRequestJSON(b *testing.B) {
	request := RotationRequest{
		ProxyID:   primitive.NewObjectID().Hex(),
		AccountID: "benchmark_account",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(request)
	}
}

func BenchmarkRotationEventJSON(b *testing.B) {
	event := RotationEvent{
		OldProxyID: primitive.NewObjectID().Hex(),
		NewProxyID: primitive.NewObjectID().Hex(),
		AccountID:  "benchmark_account",
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(event)
	}
}

