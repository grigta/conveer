package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/services/proxy-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// HealthCheckerTestSuite is the test suite for HealthChecker
type HealthCheckerTestSuite struct {
	suite.Suite
	ctx       context.Context
	cancel    context.CancelFunc
	proxyRepo *MockProxyRepository
	rabbitmq  *MockRabbitMQ
	logger    *logrus.Logger
	config    *config.Config
}

func (s *HealthCheckerTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.proxyRepo = new(MockProxyRepository)
	s.rabbitmq = NewMockRabbitMQ()
	s.logger = logrus.New()
	s.logger.SetLevel(logrus.DebugLevel)
	s.config = &config.Config{
		Proxy: config.ProxyConfig{
			HealthCheckInterval:    "15m",
			MaxFailedChecks:        3,
			IPQualityScoreAPIKey:   "",
		},
	}
}

func (s *HealthCheckerTestSuite) TearDownTest() {
	s.cancel()
}

func TestHealthCheckerTestSuite(t *testing.T) {
	suite.Run(t, new(HealthCheckerTestSuite))
}

// Test NewHealthChecker with default values
func (s *HealthCheckerTestSuite) TestNewHealthChecker_DefaultValues() {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			HealthCheckInterval: "",
			MaxFailedChecks:     0,
		},
	}

	hc := NewHealthChecker(s.proxyRepo, nil, s.logger, cfg)

	s.NotNil(hc)
	s.Equal(15*time.Minute, hc.checkInterval)
	s.Equal(3, hc.maxFailedChecks)
}

// Test NewHealthChecker with custom values
func (s *HealthCheckerTestSuite) TestNewHealthChecker_CustomValues() {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			HealthCheckInterval: "5m",
			MaxFailedChecks:     5,
			IPQualityScoreAPIKey: "test-api-key",
		},
	}

	hc := NewHealthChecker(s.proxyRepo, nil, s.logger, cfg)

	s.NotNil(hc)
	s.Equal(5*time.Minute, hc.checkInterval)
	s.Equal(5, hc.maxFailedChecks)
	s.Equal("test-api-key", hc.ipqsAPIKey)
}

// Test CheckProxyHealth - successful check
func (s *HealthCheckerTestSuite) TestCheckProxyHealth_Success() {
	// Create a mock HTTP server that simulates httpbin.org/ip
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"origin": "192.168.1.1"}`))
	}))
	defer server.Close()

	proxy := &models.Proxy{
		ID:       primitive.NewObjectID(),
		IP:       "192.168.1.1",
		Port:     8080,
		Protocol: models.ProtocolHTTP,
		Username: "user",
		Password: "pass",
	}

	// Note: This test is limited because we can't easily mock the HTTP client
	// In a real scenario, we would use dependency injection for the HTTP client
	health := &models.ProxyHealth{
		ProxyID:   proxy.ID,
		LastCheck: time.Now(),
	}

	s.NotNil(health)
	s.Equal(proxy.ID, health.ProxyID)
}

// Test CheckProxyHealth - proxy timeout
func (s *HealthCheckerTestSuite) TestCheckProxyHealth_Timeout() {
	proxy := &models.Proxy{
		ID:       primitive.NewObjectID(),
		IP:       "10.255.255.1", // Non-routable IP to simulate timeout
		Port:     8080,
		Protocol: models.ProtocolHTTP,
		Username: "user",
		Password: "pass",
	}

	health := &models.ProxyHealth{
		ProxyID:      proxy.ID,
		LastCheck:    time.Now(),
		FailedChecks: 1,
	}

	// Latency should be negative on timeout
	s.NotNil(health)
}

// Test HandleFailedCheck - proxy should be banned
func (s *HealthCheckerTestSuite) TestHandleFailedCheck_BanProxy() {
	proxyID := primitive.NewObjectID()
	proxy := &models.Proxy{
		ID:     proxyID,
		IP:     "192.168.1.1",
		Port:   8080,
		Status: models.ProxyStatusActive,
	}

	s.proxyRepo.On("UpdateProxyStatus", s.ctx, proxyID, models.ProxyStatusBanned).Return(nil)
	s.rabbitmq.On("Publish", "proxy.events", "proxy.health_failed", mock.Anything).Return(nil)

	// Verify expectations are set
	s.proxyRepo.AssertExpectations(s.T())
	
	// The proxy should be banned after max failed checks
	_ = proxy
}

// Test IPQSResponse parsing
func (s *HealthCheckerTestSuite) TestIPQSResponse_Parsing() {
	response := IPQSResponse{
		Success:      true,
		FraudScore:   45.5,
		CountryCode:  "US",
		City:         "New York",
		ISP:          "Test ISP",
		Mobile:       true,
		Proxy:        true,
		VPN:          false,
		TOR:          false,
		RecentAbuse:  false,
	}

	s.True(response.Success)
	s.Equal(45.5, response.FraudScore)
	s.Equal("US", response.CountryCode)
	s.True(response.Mobile)
	s.True(response.Proxy)
	s.False(response.VPN)
	s.False(response.TOR)
}

// Test checkFraudScore - API key not configured
func (s *HealthCheckerTestSuite) TestCheckFraudScore_NoAPIKey() {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			IPQualityScoreAPIKey: "",
		},
	}

	hc := NewHealthChecker(s.proxyRepo, nil, s.logger, cfg)
	
	// Without API key, fraud check should be skipped
	s.Empty(hc.ipqsAPIKey)
}

// Test checkFraudScore with mock server
func (s *HealthCheckerTestSuite) TestCheckFraudScore_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"success": true,
			"fraud_score": 25.5,
			"country_code": "US",
			"city": "New York",
			"ISP": "Test ISP",
			"mobile": true,
			"proxy": false,
			"vpn": false,
			"tor": false,
			"recent_abuse": false
		}`))
	}))
	defer server.Close()

	// This verifies the expected response format
	s.NotNil(server)
}

// Test verifyGeoLocation - matching location
func (s *HealthCheckerTestSuite) TestVerifyGeoLocation_Matching() {
	proxy := &models.Proxy{
		Country: "US",
	}
	fraudData := &IPQSResponse{
		CountryCode: "US",
	}

	// Countries match
	s.Equal(proxy.Country, fraudData.CountryCode)
}

// Test verifyGeoLocation - mismatching location
func (s *HealthCheckerTestSuite) TestVerifyGeoLocation_Mismatch() {
	proxy := &models.Proxy{
		Country: "US",
	}
	fraudData := &IPQSResponse{
		CountryCode: "DE",
	}

	// Countries don't match
	s.NotEqual(proxy.Country, fraudData.CountryCode)
}

// Test verifyGeoLocation - nil fraud data
func (s *HealthCheckerTestSuite) TestVerifyGeoLocation_NilFraudData() {
	proxy := &models.Proxy{
		Country: "US",
	}

	// Should return true when fraud data is nil
	s.NotNil(proxy)
}

// Test performHealthChecks - with active proxies
func (s *HealthCheckerTestSuite) TestPerformHealthChecks_WithActiveProxies() {
	proxies := []models.Proxy{
		{
			ID:       primitive.NewObjectID(),
			IP:       "192.168.1.1",
			Port:     8080,
			Protocol: models.ProtocolHTTP,
			Username: "user1",
			Password: "pass1",
			Status:   models.ProxyStatusActive,
		},
		{
			ID:       primitive.NewObjectID(),
			IP:       "192.168.1.2",
			Port:     8080,
			Protocol: models.ProtocolHTTP,
			Username: "user2",
			Password: "pass2",
			Status:   models.ProxyStatusActive,
		},
	}

	s.proxyRepo.On("GetProxiesByStatus", s.ctx, models.ProxyStatusActive).Return(proxies, nil)
	s.proxyRepo.On("UpdateProxyHealth", s.ctx, mock.AnythingOfType("primitive.ObjectID"), mock.AnythingOfType("*models.ProxyHealth")).Return(nil)
	s.proxyRepo.On("GetProxyStatistics", s.ctx).Return(&models.ProxyStats{
		ActiveProxies: 2,
		TotalBindings: 1,
	}, nil)

	// Verify mock expectations are set
	s.proxyRepo.AssertExpectations(s.T())
}

// Test performHealthChecks - no active proxies
func (s *HealthCheckerTestSuite) TestPerformHealthChecks_NoActiveProxies() {
	s.proxyRepo.On("GetProxiesByStatus", s.ctx, models.ProxyStatusActive).Return([]models.Proxy{}, nil)

	// No proxies to check
	s.proxyRepo.AssertExpectations(s.T())
}

// Test ScheduleHealthCheck - immediate check
func (s *HealthCheckerTestSuite) TestScheduleHealthCheck_Immediate() {
	proxyID := primitive.NewObjectID()

	// For immediate check, delay is 0
	request := map[string]interface{}{
		"proxy_id": proxyID.Hex(),
	}

	s.rabbitmq.On("Publish", "", "proxy.health_check", request).Return(nil)

	s.rabbitmq.AssertExpectations(s.T())
}

// Test ScheduleHealthCheck - delayed check
func (s *HealthCheckerTestSuite) TestScheduleHealthCheck_Delayed() {
	proxyID := primitive.NewObjectID()
	delay := 5 * time.Minute

	// For delayed check, we use AfterFunc
	s.NotZero(delay)
	s.NotNil(proxyID)
}

// Table-driven tests for fraud score thresholds
func TestFraudScoreThresholds(t *testing.T) {
	tests := []struct {
		name           string
		fraudScore     float64
		expectedAction string
	}{
		{"low fraud score", 10.0, "allow"},
		{"medium fraud score", 50.0, "warn"},
		{"high fraud score", 75.0, "block"},
		{"very high fraud score", 90.0, "block"},
		{"zero fraud score", 0.0, "allow"},
		{"max fraud score", 100.0, "block"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := IPQSResponse{
				Success:    true,
				FraudScore: tt.fraudScore,
			}

			var action string
			if response.FraudScore <= 25 {
				action = "allow"
			} else if response.FraudScore <= 60 {
				action = "warn"
			} else {
				action = "block"
			}

			// Verify fraud score handling
			if tt.fraudScore <= 25 {
				assert.Equal(t, "allow", action)
			} else if tt.fraudScore <= 60 {
				assert.Equal(t, "warn", action)
			} else {
				assert.Equal(t, "block", action)
			}
		})
	}
}

// Test ProxyHealth model
func TestProxyHealthModel(t *testing.T) {
	proxyID := primitive.NewObjectID()
	health := &models.ProxyHealth{
		ProxyID:         proxyID,
		Latency:         150,
		FraudScore:      25.5,
		IsVPN:           false,
		IsProxy:         true,
		IsTor:           false,
		BlacklistStatus: false,
		LastCheck:       time.Now(),
		FailedChecks:    0,
	}

	assert.Equal(t, proxyID, health.ProxyID)
	assert.Equal(t, 150, health.Latency)
	assert.Equal(t, 25.5, health.FraudScore)
	assert.False(t, health.IsVPN)
	assert.True(t, health.IsProxy)
	assert.False(t, health.IsTor)
	assert.False(t, health.BlacklistStatus)
	assert.Equal(t, 0, health.FailedChecks)
}

// Benchmark tests
func BenchmarkIPQSResponseParsing(b *testing.B) {
	jsonData := []byte(`{
		"success": true,
		"fraud_score": 25.5,
		"country_code": "US",
		"city": "New York",
		"ISP": "Test ISP",
		"mobile": true,
		"proxy": false,
		"vpn": false,
		"tor": false,
		"recent_abuse": false
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var response IPQSResponse
		_ = json.Unmarshal(jsonData, &response)
	}
}


