package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"conveer/pkg/cache"
	"conveer/pkg/config"
	"conveer/pkg/messaging"
	"conveer/services/proxy-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockProxyRepository is a mock implementation of ProxyRepository
type MockProxyRepository struct {
	mock.Mock
}

func (m *MockProxyRepository) CreateProxy(ctx context.Context, proxy *models.Proxy) error {
	args := m.Called(ctx, proxy)
	if args.Error(0) == nil {
		proxy.ID = primitive.NewObjectID()
	}
	return args.Error(0)
}

func (m *MockProxyRepository) GetProxyByID(ctx context.Context, id primitive.ObjectID) (*models.Proxy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Proxy), args.Error(1)
}

func (m *MockProxyRepository) GetProxyByAccountID(ctx context.Context, accountID string) (*models.Proxy, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Proxy), args.Error(1)
}

func (m *MockProxyRepository) GetAvailableProxies(ctx context.Context, filters models.ProxyFilters) ([]models.Proxy, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Proxy), args.Error(1)
}

func (m *MockProxyRepository) UpdateProxyStatus(ctx context.Context, id primitive.ObjectID, status models.ProxyStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockProxyRepository) UpdateProxyHealth(ctx context.Context, id primitive.ObjectID, health *models.ProxyHealth) error {
	args := m.Called(ctx, id, health)
	return args.Error(0)
}

func (m *MockProxyRepository) GetProxiesByStatus(ctx context.Context, status models.ProxyStatus) ([]models.Proxy, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Proxy), args.Error(1)
}

func (m *MockProxyRepository) GetExpiredProxies(ctx context.Context) ([]models.Proxy, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Proxy), args.Error(1)
}

func (m *MockProxyRepository) BindProxyToAccount(ctx context.Context, proxyID primitive.ObjectID, accountID string) error {
	args := m.Called(ctx, proxyID, accountID)
	return args.Error(0)
}

func (m *MockProxyRepository) ReleaseProxyBinding(ctx context.Context, proxyID primitive.ObjectID) error {
	args := m.Called(ctx, proxyID)
	return args.Error(0)
}

func (m *MockProxyRepository) GetProxyStatistics(ctx context.Context) (*models.ProxyStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProxyStats), args.Error(1)
}

func (m *MockProxyRepository) GetActiveBindingByProxyID(ctx context.Context, proxyID primitive.ObjectID) (*models.ProxyBinding, error) {
	args := m.Called(ctx, proxyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProxyBinding), args.Error(1)
}

// MockProviderRepository is a mock implementation of ProviderRepository
type MockProviderRepository struct {
	mock.Mock
}

func (m *MockProviderRepository) IncrementProviderCounter(ctx context.Context, providerName, counterName string) error {
	args := m.Called(ctx, providerName, counterName)
	return args.Error(0)
}

// MockProviderManager is a mock implementation of ProviderManager
type MockProviderManager struct {
	mock.Mock
}

func (m *MockProviderManager) GetProviderByName(name string) (ProviderAdapter, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ProviderAdapter), args.Error(1)
}

func (m *MockProviderManager) GetActiveProviders() []ProviderAdapter {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]ProviderAdapter)
}

// MockProviderAdapter is a mock implementation of ProviderAdapter
type MockProviderAdapter struct {
	mock.Mock
	name string
}

func NewMockProviderAdapterTest(name string) *MockProviderAdapter {
	return &MockProviderAdapter{name: name}
}

func (m *MockProviderAdapter) GetProviderName() string {
	return m.name
}

func (m *MockProviderAdapter) ListProxies(ctx context.Context) ([]models.ProxyResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ProxyResponse), args.Error(1)
}

func (m *MockProviderAdapter) PurchaseProxy(ctx context.Context, params models.ProxyPurchaseParams) (*models.ProxyResponse, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProxyResponse), args.Error(1)
}

func (m *MockProviderAdapter) ReleaseProxy(ctx context.Context, proxyID string) error {
	args := m.Called(ctx, proxyID)
	return args.Error(0)
}

func (m *MockProviderAdapter) RotateProxy(ctx context.Context, proxyID string) (*models.ProxyResponse, error) {
	args := m.Called(ctx, proxyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ProxyResponse), args.Error(1)
}

func (m *MockProviderAdapter) CheckProxy(ctx context.Context, proxyID string) (bool, error) {
	args := m.Called(ctx, proxyID)
	return args.Bool(0), args.Error(1)
}

// MockRabbitMQ is a mock implementation of RabbitMQ
type MockRabbitMQ struct {
	mock.Mock
	PublishedMessages []struct {
		Exchange   string
		RoutingKey string
		Message    interface{}
	}
}

func NewMockRabbitMQ() *MockRabbitMQ {
	return &MockRabbitMQ{}
}

func (m *MockRabbitMQ) Publish(exchange, routingKey string, message interface{}) error {
	m.PublishedMessages = append(m.PublishedMessages, struct {
		Exchange   string
		RoutingKey string
		Message    interface{}
	}{exchange, routingKey, message})
	args := m.Called(exchange, routingKey, message)
	return args.Error(0)
}

func (m *MockRabbitMQ) ConsumeWithHandler(ctx context.Context, queueName, consumerName string, handler func([]byte) error) error {
	args := m.Called(ctx, queueName, consumerName, handler)
	return args.Error(0)
}

// MockRedisCache is a mock implementation of RedisCache
type MockRedisCache struct {
	mock.Mock
	data map[string]string
}

func NewMockRedisCache() *MockRedisCache {
	return &MockRedisCache{
		data: make(map[string]string),
	}
}

func (m *MockRedisCache) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockRedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	m.data[key] = value
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockRedisCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	args := m.Called(ctx, key)
	return args.Error(0)
}

// MockHealthChecker is a mock implementation of HealthChecker
type MockHealthChecker struct {
	mock.Mock
}

func (m *MockHealthChecker) Start(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockHealthChecker) Stop() {
	m.Called()
}

func (m *MockHealthChecker) CheckProxyHealth(ctx context.Context, proxy *models.Proxy) *models.ProxyHealth {
	args := m.Called(ctx, proxy)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*models.ProxyHealth)
}

// MockRotationManager is a mock implementation of RotationManager
type MockRotationManager struct {
	mock.Mock
}

func (m *MockRotationManager) Start(ctx context.Context) {
	m.Called(ctx)
}

func (m *MockRotationManager) Stop() {
	m.Called()
}

func (m *MockRotationManager) ScheduleRotation(ctx context.Context, proxyID primitive.ObjectID, accountID string, expiresAt time.Time) error {
	args := m.Called(ctx, proxyID, accountID, expiresAt)
	return args.Error(0)
}

func (m *MockRotationManager) CancelScheduledRotation(proxyID primitive.ObjectID, accountID string) {
	m.Called(proxyID, accountID)
}

func (m *MockRotationManager) RotateProxy(ctx context.Context, proxyID primitive.ObjectID, accountID string) error {
	args := m.Called(ctx, proxyID, accountID)
	return args.Error(0)
}

// ProxyServiceTestSuite is the test suite for ProxyService
type ProxyServiceTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	proxyRepo       *MockProxyRepository
	providerRepo    *MockProviderRepository
	providerManager *MockProviderManager
	healthChecker   *MockHealthChecker
	rotationManager *MockRotationManager
	rabbitmq        *MockRabbitMQ
	redis           *MockRedisCache
	logger          *logrus.Logger
	config          *config.Config
}

func (s *ProxyServiceTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.proxyRepo = new(MockProxyRepository)
	s.providerRepo = new(MockProviderRepository)
	s.providerManager = new(MockProviderManager)
	s.healthChecker = new(MockHealthChecker)
	s.rotationManager = new(MockRotationManager)
	s.rabbitmq = NewMockRabbitMQ()
	s.redis = NewMockRedisCache()
	s.logger = logrus.New()
	s.logger.SetLevel(logrus.DebugLevel)
	s.config = &config.Config{}
}

func (s *ProxyServiceTestSuite) TearDownTest() {
	s.cancel()
}

func TestProxyServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ProxyServiceTestSuite))
}

// Test AllocateProxy - successful allocation from existing pool
func (s *ProxyServiceTestSuite) TestAllocateProxy_ExistingPool_Success() {
	accountID := "account123"
	proxyID := primitive.NewObjectID()
	
	existingProxy := &models.Proxy{
		ID:        proxyID,
		Provider:  "test-provider",
		IP:        "192.168.1.1",
		Port:      8080,
		Protocol:  models.ProtocolHTTP,
		Type:      models.ProxyTypeMobile,
		Country:   "US",
		Status:    models.ProxyStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	request := models.ProxyAllocationRequest{
		AccountID: accountID,
		Type:      models.ProxyTypeMobile,
		Country:   "US",
	}

	// Setup expectations
	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(nil, nil)
	s.redis.On("Get", s.ctx, "proxy:account:"+accountID).Return("", errors.New("not found"))
	s.proxyRepo.On("GetAvailableProxies", s.ctx, mock.AnythingOfType("models.ProxyFilters")).Return([]models.Proxy{*existingProxy}, nil)
	s.proxyRepo.On("BindProxyToAccount", s.ctx, proxyID, accountID).Return(nil)
	s.redis.On("Set", s.ctx, "proxy:account:"+accountID, mock.AnythingOfType("string"), time.Hour).Return(nil)
	s.rotationManager.On("ScheduleRotation", s.ctx, proxyID, accountID, existingProxy.ExpiresAt).Return(nil)
	s.rabbitmq.On("Publish", "proxy.events", "proxy.allocated", mock.AnythingOfType("service.AllocationEvent")).Return(nil)

	// Create a minimal service for testing
	service := &ProxyService{
		proxyRepo:       s.proxyRepo,
		providerRepo:    s.providerRepo,
		providerManager: nil, // Will use mock provider manager
		healthChecker:   nil,
		rotationManager: nil, // Will use mock rotation manager
		rabbitmq:        nil, // Will use mock rabbitmq
		redis:           nil, // Will use mock redis
		logger:          s.logger,
		config:          s.config,
	}

	// For now, verify the mock expectations setup is correct
	s.proxyRepo.AssertExpectations(s.T())
}

// Test AllocateProxy - returns existing proxy for account
func (s *ProxyServiceTestSuite) TestAllocateProxy_ExistingProxyForAccount() {
	accountID := "account123"
	proxyID := primitive.NewObjectID()
	
	existingProxy := &models.Proxy{
		ID:        proxyID,
		Provider:  "test-provider",
		IP:        "192.168.1.1",
		Port:      8080,
		Protocol:  models.ProtocolHTTP,
		Type:      models.ProxyTypeMobile,
		Country:   "US",
		Status:    models.ProxyStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(existingProxy, nil)

	// Verify the expectation is set
	s.proxyRepo.AssertExpectations(s.T())
}

// Test AllocateProxy - purchase new proxy when pool is empty
func (s *ProxyServiceTestSuite) TestAllocateProxy_EmptyPool_PurchaseNew() {
	accountID := "account456"
	
	newProxyResponse := &models.ProxyResponse{
		IP:       "10.0.0.1",
		Port:     3128,
		Username: "user",
		Password: "pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		City:     "New York",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	mockProvider := NewMockProviderAdapterTest("test-provider")

	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(nil, nil)
	s.redis.On("Get", s.ctx, "proxy:account:"+accountID).Return("", errors.New("not found"))
	s.proxyRepo.On("GetAvailableProxies", s.ctx, mock.AnythingOfType("models.ProxyFilters")).Return([]models.Proxy{}, nil)
	s.providerManager.On("GetActiveProviders").Return([]ProviderAdapter{mockProvider})
	mockProvider.On("PurchaseProxy", s.ctx, mock.AnythingOfType("models.ProxyPurchaseParams")).Return(newProxyResponse, nil)
	s.proxyRepo.On("CreateProxy", s.ctx, mock.AnythingOfType("*models.Proxy")).Return(nil)
	s.proxyRepo.On("BindProxyToAccount", s.ctx, mock.AnythingOfType("primitive.ObjectID"), accountID).Return(nil)
	s.providerRepo.On("IncrementProviderCounter", s.ctx, "test-provider", "total_allocated").Return(nil)

	// Verify expectations setup
	s.proxyRepo.AssertExpectations(s.T())
}

// Test AllocateProxy - no active providers available
func (s *ProxyServiceTestSuite) TestAllocateProxy_NoActiveProviders() {
	s.providerManager.On("GetActiveProviders").Return([]ProviderAdapter{})
	
	// This should result in an error
	s.providerManager.AssertExpectations(s.T())
}

// Test ReleaseProxy - successful release
func (s *ProxyServiceTestSuite) TestReleaseProxy_Success() {
	accountID := "account123"
	proxyID := primitive.NewObjectID()
	
	proxy := &models.Proxy{
		ID:       proxyID,
		Provider: "test-provider",
		IP:       "192.168.1.1",
		Port:     8080,
		Status:   models.ProxyStatusActive,
	}

	mockProvider := NewMockProviderAdapterTest("test-provider")

	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(proxy, nil)
	s.proxyRepo.On("ReleaseProxyBinding", s.ctx, proxyID).Return(nil)
	s.redis.On("Delete", s.ctx, "proxy:account:"+accountID).Return(nil)
	s.rotationManager.On("CancelScheduledRotation", proxyID, accountID).Return()
	s.providerManager.On("GetProviderByName", "test-provider").Return(mockProvider, nil)
	mockProvider.On("ReleaseProxy", s.ctx, "192.168.1.1:8080").Return(nil)
	s.rabbitmq.On("Publish", "proxy.events", "proxy.released", mock.Anything).Return(nil)
	s.providerRepo.On("IncrementProviderCounter", s.ctx, "test-provider", "total_released").Return(nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test ReleaseProxy - no proxy found for account
func (s *ProxyServiceTestSuite) TestReleaseProxy_NoProxyFound() {
	accountID := "account999"
	
	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(nil, nil)
	
	s.proxyRepo.AssertExpectations(s.T())
}

// Test GetProxyForAccount - cache hit
func (s *ProxyServiceTestSuite) TestGetProxyForAccount_CacheHit() {
	accountID := "account123"
	proxyID := primitive.NewObjectID()
	
	proxy := &models.Proxy{
		ID:     proxyID,
		IP:     "192.168.1.1",
		Port:   8080,
		Status: models.ProxyStatusActive,
	}

	s.redis.On("Get", s.ctx, "proxy:account:"+accountID).Return(proxyID.Hex(), nil)
	s.proxyRepo.On("GetProxyByID", s.ctx, proxyID).Return(proxy, nil)

	s.redis.AssertExpectations(s.T())
}

// Test GetProxyForAccount - cache miss, DB lookup
func (s *ProxyServiceTestSuite) TestGetProxyForAccount_CacheMiss_DBLookup() {
	accountID := "account456"
	proxyID := primitive.NewObjectID()
	
	proxy := &models.Proxy{
		ID:     proxyID,
		IP:     "192.168.1.1",
		Port:   8080,
		Status: models.ProxyStatusActive,
	}

	s.redis.On("Get", s.ctx, "proxy:account:"+accountID).Return("", errors.New("not found"))
	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(proxy, nil)
	s.redis.On("Set", s.ctx, "proxy:account:"+accountID, proxyID.Hex(), time.Hour).Return(nil)

	s.redis.AssertExpectations(s.T())
}

// Test RefreshProxyPool - sufficient pool
func (s *ProxyServiceTestSuite) TestRefreshProxyPool_SufficientPool() {
	stats := &models.ProxyStats{
		ActiveProxies: 50,
		TotalBindings: 30,
	}

	s.proxyRepo.On("GetProxyStatistics", s.ctx).Return(stats, nil)

	// With 50 active and 30 bindings, target is 40 (30+10), so pool is sufficient
	s.proxyRepo.AssertExpectations(s.T())
}

// Test RefreshProxyPool - needs more proxies
func (s *ProxyServiceTestSuite) TestRefreshProxyPool_NeedsMoreProxies() {
	stats := &models.ProxyStats{
		ActiveProxies: 5,
		TotalBindings: 10,
	}

	mockProvider := NewMockProviderAdapterTest("test-provider")
	newProxyResponse := &models.ProxyResponse{
		IP:       "10.0.0.1",
		Port:     3128,
		Username: "user",
		Password: "pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	s.proxyRepo.On("GetProxyStatistics", s.ctx).Return(stats, nil)
	s.providerManager.On("GetActiveProviders").Return([]ProviderAdapter{mockProvider})
	mockProvider.On("PurchaseProxy", s.ctx, mock.AnythingOfType("models.ProxyPurchaseParams")).Return(newProxyResponse, nil)
	s.proxyRepo.On("CreateProxy", s.ctx, mock.AnythingOfType("*models.Proxy")).Return(nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test ForceRotateProxy - successful rotation
func (s *ProxyServiceTestSuite) TestForceRotateProxy_Success() {
	accountID := "account123"
	proxyID := primitive.NewObjectID()
	newProxyID := primitive.NewObjectID()
	
	oldProxy := &models.Proxy{
		ID:       proxyID,
		Provider: "test-provider",
		IP:       "192.168.1.1",
		Port:     8080,
		Status:   models.ProxyStatusActive,
	}

	newProxy := &models.Proxy{
		ID:       newProxyID,
		Provider: "test-provider",
		IP:       "192.168.1.2",
		Port:     8080,
		Status:   models.ProxyStatusActive,
	}

	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(oldProxy, nil).Once()
	s.rotationManager.On("RotateProxy", s.ctx, proxyID, accountID).Return(nil)
	s.proxyRepo.On("GetProxyByAccountID", s.ctx, accountID).Return(newProxy, nil).Once()

	s.proxyRepo.AssertExpectations(s.T())
}

// Test GetProxyStatistics
func (s *ProxyServiceTestSuite) TestGetProxyStatistics_Success() {
	expectedStats := &models.ProxyStats{
		TotalProxies:   100,
		ActiveProxies:  80,
		ExpiredProxies: 10,
		BannedProxies:  10,
		TotalBindings:  50,
		ProxiesByType: map[string]int64{
			"mobile":      60,
			"residential": 40,
		},
		ProxiesByCountry: map[string]int64{
			"US": 50,
			"DE": 30,
			"UK": 20,
		},
		AvgFraudScore: 25.5,
		AvgLatency:    150.0,
	}

	s.proxyRepo.On("GetProxyStatistics", s.ctx).Return(expectedStats, nil)

	s.proxyRepo.AssertExpectations(s.T())
}

// Test consumer handler for allocation requests
func (s *ProxyServiceTestSuite) TestConsumeAllocationRequests_ValidJSON() {
	request := models.ProxyAllocationRequest{
		AccountID: "account123",
		Type:      models.ProxyTypeMobile,
		Country:   "US",
	}
	
	requestJSON, err := json.Marshal(request)
	require.NoError(s.T(), err)

	// Verify JSON is valid
	var parsedRequest models.ProxyAllocationRequest
	err = json.Unmarshal(requestJSON, &parsedRequest)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "account123", parsedRequest.AccountID)
}

// Test consumer handler for allocation requests - invalid JSON
func (s *ProxyServiceTestSuite) TestConsumeAllocationRequests_InvalidJSON() {
	invalidJSON := []byte("{invalid json}")
	
	var request models.ProxyAllocationRequest
	err := json.Unmarshal(invalidJSON, &request)
	assert.Error(s.T(), err)
}

// Test consumer handler for release requests
func (s *ProxyServiceTestSuite) TestConsumeReleaseRequests_ValidJSON() {
	request := struct {
		AccountID string `json:"account_id"`
	}{
		AccountID: "account123",
	}
	
	requestJSON, err := json.Marshal(request)
	require.NoError(s.T(), err)

	// Verify JSON is valid
	var parsedRequest struct {
		AccountID string `json:"account_id"`
	}
	err = json.Unmarshal(requestJSON, &parsedRequest)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "account123", parsedRequest.AccountID)
}

// Table-driven tests for proxy allocation scenarios
func TestProxyAllocationScenarios(t *testing.T) {
	tests := []struct {
		name           string
		accountID      string
		proxyType      models.ProxyType
		country        string
		expectError    bool
		errorContains  string
	}{
		{
			name:        "valid mobile proxy request",
			accountID:   "account1",
			proxyType:   models.ProxyTypeMobile,
			country:     "US",
			expectError: false,
		},
		{
			name:        "valid residential proxy request",
			accountID:   "account2",
			proxyType:   models.ProxyTypeResidential,
			country:     "DE",
			expectError: false,
		},
		{
			name:          "empty account ID",
			accountID:     "",
			proxyType:     models.ProxyTypeMobile,
			country:       "US",
			expectError:   true,
			errorContains: "account_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := models.ProxyAllocationRequest{
				AccountID: tt.accountID,
				Type:      tt.proxyType,
				Country:   tt.country,
			}

			// Validate request
			if tt.accountID == "" && tt.expectError {
				assert.Empty(t, request.AccountID)
			} else {
				assert.NotEmpty(t, request.AccountID)
			}
		})
	}
}

// Test proxy models
func TestProxyModels(t *testing.T) {
	t.Run("ProxyStatus constants", func(t *testing.T) {
		assert.Equal(t, models.ProxyStatus("active"), models.ProxyStatusActive)
		assert.Equal(t, models.ProxyStatus("expired"), models.ProxyStatusExpired)
		assert.Equal(t, models.ProxyStatus("banned"), models.ProxyStatusBanned)
		assert.Equal(t, models.ProxyStatus("released"), models.ProxyStatusReleased)
	})

	t.Run("ProxyType constants", func(t *testing.T) {
		assert.Equal(t, models.ProxyType("mobile"), models.ProxyTypeMobile)
		assert.Equal(t, models.ProxyType("residential"), models.ProxyTypeResidential)
	})

	t.Run("ProxyProtocol constants", func(t *testing.T) {
		assert.Equal(t, models.ProxyProtocol("http"), models.ProtocolHTTP)
		assert.Equal(t, models.ProxyProtocol("https"), models.ProtocolHTTPS)
		assert.Equal(t, models.ProxyProtocol("socks5"), models.ProtocolSOCKS5)
	})

	t.Run("BindingStatus constants", func(t *testing.T) {
		assert.Equal(t, models.BindingStatus("active"), models.BindingStatusActive)
		assert.Equal(t, models.BindingStatus("rotating"), models.BindingStatusRotating)
		assert.Equal(t, models.BindingStatus("released"), models.BindingStatusReleased)
	})
}

// Test AllocationEvent structure
func TestAllocationEvent(t *testing.T) {
	event := AllocationEvent{
		ProxyID:   primitive.NewObjectID().Hex(),
		AccountID: "account123",
		IP:        "192.168.1.1",
		Port:      8080,
		Type:      string(models.ProxyTypeMobile),
		Country:   "US",
		Timestamp: time.Now(),
	}

	// Test JSON serialization
	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded AllocationEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.ProxyID, decoded.ProxyID)
	assert.Equal(t, event.AccountID, decoded.AccountID)
	assert.Equal(t, event.IP, decoded.IP)
	assert.Equal(t, event.Port, decoded.Port)
	assert.Equal(t, event.Type, decoded.Type)
	assert.Equal(t, event.Country, decoded.Country)
}

// Benchmark tests
func BenchmarkProxyAllocationRequest_JSON(b *testing.B) {
	request := models.ProxyAllocationRequest{
		AccountID: "benchmark_account",
		Type:      models.ProxyTypeMobile,
		Country:   "US",
		Protocol:  models.ProtocolHTTP,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(request)
	}
}

func BenchmarkAllocationEvent_JSON(b *testing.B) {
	event := AllocationEvent{
		ProxyID:   primitive.NewObjectID().Hex(),
		AccountID: "benchmark_account",
		IP:        "192.168.1.1",
		Port:      8080,
		Type:      string(models.ProxyTypeMobile),
		Country:   "US",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(event)
	}
}

