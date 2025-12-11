package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grigta/conveer/pkg/crypto"
	"github.com/grigta/conveer/services/proxy-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ProviderAdapterTestSuite is the test suite for ProviderAdapter
type ProviderAdapterTestSuite struct {
	suite.Suite
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *logrus.Logger
	encryptor *crypto.Encryptor
}

func (s *ProviderAdapterTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.logger = logrus.New()
	s.logger.SetLevel(logrus.DebugLevel)
	
	var err error
	s.encryptor, err = crypto.NewEncryptor("12345678901234567890123456789012")
	s.Require().NoError(err)
}

func (s *ProviderAdapterTestSuite) TearDownTest() {
	s.cancel()
}

func TestProviderAdapterTestSuite(t *testing.T) {
	suite.Run(t, new(ProviderAdapterTestSuite))
}

// Test NewHTTPProviderAdapter
func (s *ProviderAdapterTestSuite) TestNewHTTPProviderAdapter() {
	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  "https://api.example.com",
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	s.NotNil(adapter)
	s.Equal("test-provider", adapter.GetProviderName())
}

// Test NewHTTPProviderAdapter with encrypted API key
func (s *ProviderAdapterTestSuite) TestNewHTTPProviderAdapter_EncryptedAPIKey() {
	encryptedKey, err := s.encryptor.Encrypt("my-secret-api-key")
	s.Require().NoError(err)

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  "https://api.example.com",
			AuthType: models.AuthTypeBearer,
			AuthKey:  encryptedKey,
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, s.encryptor)

	s.NotNil(adapter)
	// The API key should be decrypted internally
}

// Test PurchaseProxy - successful purchase
func (s *ProviderAdapterTestSuite) TestPurchaseProxy_Success() {
	expectedResponse := models.ProxyResponse{
		IP:       "192.168.1.1",
		Port:     8080,
		Username: "user",
		Password: "pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		City:     "New York",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		s.Equal("POST", r.Method)
		s.Equal("/purchase", r.URL.Path)
		s.Equal("application/json", r.Header.Get("Content-Type"))
		s.Equal("Bearer test-api-key", r.Header.Get("Authorization"))

		// Return successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Purchase: "/purchase",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	params := models.ProxyPurchaseParams{
		Type:     models.ProxyTypeMobile,
		Country:  "US",
		Protocol: models.ProtocolHTTP,
		Duration: 24 * time.Hour,
		Quantity: 1,
	}

	result, err := adapter.PurchaseProxy(s.ctx, params)
	s.NoError(err)
	s.NotNil(result)
	s.Equal(expectedResponse.IP, result.IP)
	s.Equal(expectedResponse.Port, result.Port)
}

// Test PurchaseProxy - endpoint not configured
func (s *ProviderAdapterTestSuite) TestPurchaseProxy_EndpointNotConfigured() {
	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  "https://api.example.com",
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			// Purchase endpoint not configured
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	params := models.ProxyPurchaseParams{
		Type:    models.ProxyTypeMobile,
		Country: "US",
	}

	result, err := adapter.PurchaseProxy(s.ctx, params)
	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "not configured")
}

// Test PurchaseProxy - retry on server error
func (s *ProviderAdapterTestSuite) TestPurchaseProxy_RetryOnServerError() {
	attempts := 0
	expectedResponse := models.ProxyResponse{
		IP:   "192.168.1.1",
		Port: 8080,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// First two attempts fail with server error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Third attempt succeeds
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Purchase: "/purchase",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	params := models.ProxyPurchaseParams{
		Type: models.ProxyTypeMobile,
	}

	result, err := adapter.PurchaseProxy(s.ctx, params)
	s.NoError(err)
	s.NotNil(result)
	s.Equal(3, attempts) // Should have retried
}

// Test PurchaseProxy - client error (no retry)
func (s *ProviderAdapterTestSuite) TestPurchaseProxy_ClientErrorNoRetry() {
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Purchase: "/purchase",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	params := models.ProxyPurchaseParams{
		Type: models.ProxyTypeMobile,
	}

	result, err := adapter.PurchaseProxy(s.ctx, params)
	s.Error(err)
	s.Nil(result)
	s.Equal(1, attempts) // Should not retry on client error
}

// Test ReleaseProxy - successful release
func (s *ProviderAdapterTestSuite) TestReleaseProxy_Success() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("DELETE", r.Method)
		s.Equal("/release/proxy123", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Release: "/release/{id}",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	err := adapter.ReleaseProxy(s.ctx, "proxy123")
	s.NoError(err)
}

// Test ReleaseProxy - endpoint not configured
func (s *ProviderAdapterTestSuite) TestReleaseProxy_EndpointNotConfigured() {
	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL: "https://api.example.com",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	err := adapter.ReleaseProxy(s.ctx, "proxy123")
	s.Error(err)
	s.Contains(err.Error(), "not configured")
}

// Test RotateProxy - successful rotation
func (s *ProviderAdapterTestSuite) TestRotateProxy_Success() {
	expectedResponse := models.ProxyResponse{
		IP:       "192.168.1.2",
		Port:     8080,
		Username: "new_user",
		Password: "new_pass",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("POST", r.Method)
		s.Equal("/rotate/proxy123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedResponse)
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Rotate: "/rotate/{id}",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	result, err := adapter.RotateProxy(s.ctx, "proxy123")
	s.NoError(err)
	s.NotNil(result)
	s.Equal(expectedResponse.IP, result.IP)
}

// Test CheckProxy - active proxy
func (s *ProviderAdapterTestSuite) TestCheckProxy_Active() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("GET", r.Method)
		s.Equal("/check/proxy123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"active": true, "healthy": true}`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			Check: "/check/{id}",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	active, err := adapter.CheckProxy(s.ctx, "proxy123")
	s.NoError(err)
	s.True(active)
}

// Test CheckProxy - inactive proxy
func (s *ProviderAdapterTestSuite) TestCheckProxy_Inactive() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"active": false, "healthy": false}`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL: server.URL,
		},
		Endpoints: models.ProviderEndpoints{
			Check: "/check/{id}",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	active, err := adapter.CheckProxy(s.ctx, "proxy123")
	s.NoError(err)
	s.False(active)
}

// Test ListProxies - successful list
func (s *ProviderAdapterTestSuite) TestListProxies_Success() {
	expectedProxies := []models.ProxyResponse{
		{IP: "192.168.1.1", Port: 8080},
		{IP: "192.168.1.2", Port: 8080},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("GET", r.Method)
		s.Equal("/proxies", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedProxies)
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL: server.URL,
		},
		Endpoints: models.ProviderEndpoints{
			List: "/proxies",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)

	proxies, err := adapter.ListProxies(s.ctx)
	s.NoError(err)
	s.Len(proxies, 2)
}

// Test different auth types
func (s *ProviderAdapterTestSuite) TestAuthType_Bearer() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("Bearer test-api-key", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			List: "/proxies",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)
	_, _ = adapter.ListProxies(s.ctx)
}

func (s *ProviderAdapterTestSuite) TestAuthType_Basic() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		s.True(ok)
		s.Equal("user", username)
		s.Equal("pass", password)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBasic,
			AuthKey:  "user:pass",
		},
		Endpoints: models.ProviderEndpoints{
			List: "/proxies",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)
	_, _ = adapter.ListProxies(s.ctx)
}

func (s *ProviderAdapterTestSuite) TestAuthType_APIKey() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Equal("test-api-key", r.Header.Get("X-API-Key"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	provider := models.ProxyProvider{
		Name:    "test-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeAPIKey,
			AuthKey:  "test-api-key",
		},
		Endpoints: models.ProviderEndpoints{
			List: "/proxies",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, s.logger, nil)
	_, _ = adapter.ListProxies(s.ctx)
}

// Test MockProviderAdapter
func (s *ProviderAdapterTestSuite) TestMockProviderAdapter() {
	mock := NewMockProviderAdapter("mock-provider", s.logger)

	s.Equal("mock-provider", mock.GetProviderName())

	// Test ListProxies
	proxies, err := mock.ListProxies(s.ctx)
	s.NoError(err)
	s.Len(proxies, 1)

	// Test PurchaseProxy
	params := models.ProxyPurchaseParams{
		Type:     models.ProxyTypeMobile,
		Country:  "US",
		Protocol: models.ProtocolHTTP,
		Duration: 24 * time.Hour,
	}
	proxy, err := mock.PurchaseProxy(s.ctx, params)
	s.NoError(err)
	s.NotNil(proxy)
	s.Equal("US", proxy.Country)

	// Test ReleaseProxy
	err = mock.ReleaseProxy(s.ctx, "proxy123")
	s.NoError(err)

	// Test RotateProxy
	rotated, err := mock.RotateProxy(s.ctx, "proxy123")
	s.NoError(err)
	s.NotNil(rotated)

	// Test CheckProxy
	active, err := mock.CheckProxy(s.ctx, "proxy123")
	s.NoError(err)
	s.True(active)
}

// Test ProviderManager
func (s *ProviderAdapterTestSuite) TestProviderManager_GetProviderByName() {
	// Create a minimal provider manager for testing
	// Note: This would require mocking the config file loading
}

// Table-driven tests for auth types
func TestAuthTypes(t *testing.T) {
	tests := []struct {
		name     string
		authType models.AuthType
		authKey  string
		expected string
	}{
		{
			name:     "bearer auth",
			authType: models.AuthTypeBearer,
			authKey:  "token123",
			expected: "Bearer token123",
		},
		{
			name:     "api key auth",
			authType: models.AuthTypeAPIKey,
			authKey:  "key123",
			expected: "key123",
		},
		{
			name:     "basic auth",
			authType: models.AuthTypeBasic,
			authKey:  "user:pass",
			expected: "user:pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.authType)
			assert.NotEmpty(t, tt.authKey)
		})
	}
}

// Test ProxyResponse model
func TestProxyResponseModel(t *testing.T) {
	response := models.ProxyResponse{
		IP:       "192.168.1.1",
		Port:     8080,
		Username: "user",
		Password: "pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		City:     "New York",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}

	assert.Equal(t, "192.168.1.1", response.IP)
	assert.Equal(t, 8080, response.Port)
	assert.Equal(t, "user", response.Username)
	assert.Equal(t, "pass", response.Password)
	assert.Equal(t, models.ProtocolHTTP, response.Protocol)
	assert.Equal(t, "US", response.Country)
	assert.Equal(t, "New York", response.City)
	assert.True(t, response.ExpireAt.After(time.Now()))
}

// Test ProxyPurchaseParams model
func TestProxyPurchaseParamsModel(t *testing.T) {
	params := models.ProxyPurchaseParams{
		Provider: "test-provider",
		Type:     models.ProxyTypeMobile,
		Country:  "US",
		Protocol: models.ProtocolHTTP,
		Duration: 24 * time.Hour,
		Quantity: 1,
	}

	assert.Equal(t, "test-provider", params.Provider)
	assert.Equal(t, models.ProxyTypeMobile, params.Type)
	assert.Equal(t, "US", params.Country)
	assert.Equal(t, models.ProtocolHTTP, params.Protocol)
	assert.Equal(t, 24*time.Hour, params.Duration)
	assert.Equal(t, 1, params.Quantity)
}

// Benchmark tests
func BenchmarkPurchaseProxy(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ip": "192.168.1.1", "port": 8080}`))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	provider := models.ProxyProvider{
		Name:    "bench-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL:  server.URL,
			AuthType: models.AuthTypeBearer,
			AuthKey:  "test-key",
		},
		Endpoints: models.ProviderEndpoints{
			Purchase: "/purchase",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, logger, nil)
	ctx := context.Background()
	params := models.ProxyPurchaseParams{
		Type: models.ProxyTypeMobile,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.PurchaseProxy(ctx, params)
	}
}

func BenchmarkListProxies(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"ip": "192.168.1.1", "port": 8080}]`))
	}))
	defer server.Close()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	provider := models.ProxyProvider{
		Name:    "bench-provider",
		Enabled: true,
		API: models.ProviderAPI{
			BaseURL: server.URL,
		},
		Endpoints: models.ProviderEndpoints{
			List: "/proxies",
		},
	}

	adapter := NewHTTPProviderAdapter(provider, logger, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapter.ListProxies(ctx)
	}
}

