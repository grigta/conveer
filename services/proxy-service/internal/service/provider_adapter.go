package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"conveer/pkg/crypto"
	"conveer/services/proxy-service/internal/models"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ProviderAdapter interface {
	ListProxies(ctx context.Context) ([]models.ProxyResponse, error)
	PurchaseProxy(ctx context.Context, params models.ProxyPurchaseParams) (*models.ProxyResponse, error)
	ReleaseProxy(ctx context.Context, proxyID string) error
	RotateProxy(ctx context.Context, proxyID string) (*models.ProxyResponse, error)
	CheckProxy(ctx context.Context, proxyID string) (bool, error)
	GetProviderName() string
}

type HTTPProviderAdapter struct {
	provider  models.ProxyProvider
	client    *http.Client
	logger    *logrus.Logger
	encryptor *crypto.Encryptor
	mu        sync.RWMutex
}

type ProviderManager struct {
	providers map[string]ProviderAdapter
	config    *models.ProviderConfig
	logger    *logrus.Logger
	encryptor *crypto.Encryptor
	mu        sync.RWMutex
}

func NewProviderManager(configPath string, logger *logrus.Logger, encryptor *crypto.Encryptor) (*ProviderManager, error) {
	config, err := LoadProviderConfigs(configPath)
	if err != nil {
		return nil, err
	}

	manager := &ProviderManager{
		providers: make(map[string]ProviderAdapter),
		config:    config,
		logger:    logger,
		encryptor: encryptor,
	}

	for _, providerConfig := range config.Providers {
		if !providerConfig.Enabled {
			continue
		}

		adapter := NewHTTPProviderAdapter(providerConfig, logger, encryptor)
		manager.providers[providerConfig.Name] = adapter
	}

	return manager, nil
}

func LoadProviderConfigs(path string) (*models.ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expandedData := os.ExpandEnv(string(data))

	var config models.ProviderConfig
	if err := yaml.Unmarshal([]byte(expandedData), &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (m *ProviderManager) GetProviderByName(name string) (ProviderAdapter, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	adapter, exists := m.providers[name]
	if !exists {
		return nil, errors.New("provider not found or not enabled")
	}

	return adapter, nil
}

func (m *ProviderManager) GetActiveProviders() []ProviderAdapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var providers []ProviderAdapter
	for _, provider := range m.providers {
		providers = append(providers, provider)
	}

	return providers
}

func NewHTTPProviderAdapter(provider models.ProxyProvider, logger *logrus.Logger, encryptor *crypto.Encryptor) *HTTPProviderAdapter {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	if provider.API.AuthKey != "" && encryptor != nil {
		decryptedKey, err := encryptor.Decrypt(provider.API.AuthKey)
		if err == nil {
			provider.API.AuthKey = decryptedKey
		}
	}

	return &HTTPProviderAdapter{
		provider:  provider,
		client:    client,
		logger:    logger,
		encryptor: encryptor,
	}
}

func (a *HTTPProviderAdapter) GetProviderName() string {
	return a.provider.Name
}

func (a *HTTPProviderAdapter) makeRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	url := a.provider.API.BaseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	switch a.provider.API.AuthType {
	case models.AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+a.provider.API.AuthKey)
	case models.AuthTypeBasic:
		parts := strings.SplitN(a.provider.API.AuthKey, ":", 2)
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		}
	case models.AuthTypeAPIKey:
		req.Header.Set("X-API-Key", a.provider.API.AuthKey)
	}

	req.Header.Set("Content-Type", "application/json")

	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(i) * time.Second
			time.Sleep(backoff)
		}

		resp, err := a.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return responseBody, nil
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: %d - %s", resp.StatusCode, string(responseBody))
			continue
		}

		return nil, fmt.Errorf("request failed: %d - %s", resp.StatusCode, string(responseBody))
	}

	return nil, lastErr
}

func (a *HTTPProviderAdapter) ListProxies(ctx context.Context) ([]models.ProxyResponse, error) {
	if a.provider.Endpoints.List == "" {
		return nil, errors.New("list endpoint not configured for this provider")
	}

	responseBody, err := a.makeRequest(ctx, http.MethodGet, a.provider.Endpoints.List, nil)
	if err != nil {
		a.logger.WithError(err).Error("Failed to list proxies from provider")
		return nil, err
	}

	var proxies []models.ProxyResponse
	if err := json.Unmarshal(responseBody, &proxies); err != nil {
		a.logger.WithError(err).Error("Failed to unmarshal proxy list response")
		return nil, err
	}

	return proxies, nil
}

func (a *HTTPProviderAdapter) PurchaseProxy(ctx context.Context, params models.ProxyPurchaseParams) (*models.ProxyResponse, error) {
	if a.provider.Endpoints.Purchase == "" {
		return nil, errors.New("purchase endpoint not configured for this provider")
	}

	requestBody := map[string]interface{}{
		"type":     params.Type,
		"country":  params.Country,
		"protocol": params.Protocol,
	}

	if params.Duration > 0 {
		requestBody["duration"] = params.Duration.Seconds()
	}

	if params.Quantity > 0 {
		requestBody["quantity"] = params.Quantity
	}

	responseBody, err := a.makeRequest(ctx, http.MethodPost, a.provider.Endpoints.Purchase, requestBody)
	if err != nil {
		a.logger.WithError(err).Error("Failed to purchase proxy from provider")
		return nil, err
	}

	var proxy models.ProxyResponse
	if err := json.Unmarshal(responseBody, &proxy); err != nil {
		a.logger.WithError(err).Error("Failed to unmarshal purchase response")
		return nil, err
	}

	return &proxy, nil
}

func (a *HTTPProviderAdapter) ReleaseProxy(ctx context.Context, proxyID string) error {
	if a.provider.Endpoints.Release == "" {
		return errors.New("release endpoint not configured for this provider")
	}

	endpoint := strings.Replace(a.provider.Endpoints.Release, "{id}", proxyID, 1)
	requestBody := map[string]interface{}{
		"proxy_id": proxyID,
	}

	_, err := a.makeRequest(ctx, http.MethodDelete, endpoint, requestBody)
	if err != nil {
		a.logger.WithError(err).Error("Failed to release proxy")
		return err
	}

	return nil
}

func (a *HTTPProviderAdapter) RotateProxy(ctx context.Context, proxyID string) (*models.ProxyResponse, error) {
	if a.provider.Endpoints.Rotate == "" {
		return nil, errors.New("rotate endpoint not configured for this provider")
	}

	endpoint := strings.Replace(a.provider.Endpoints.Rotate, "{id}", proxyID, 1)
	requestBody := map[string]interface{}{
		"proxy_id": proxyID,
	}

	responseBody, err := a.makeRequest(ctx, http.MethodPost, endpoint, requestBody)
	if err != nil {
		a.logger.WithError(err).Error("Failed to rotate proxy")
		return nil, err
	}

	var proxy models.ProxyResponse
	if err := json.Unmarshal(responseBody, &proxy); err != nil {
		a.logger.WithError(err).Error("Failed to unmarshal rotation response")
		return nil, err
	}

	return &proxy, nil
}

func (a *HTTPProviderAdapter) CheckProxy(ctx context.Context, proxyID string) (bool, error) {
	if a.provider.Endpoints.Check == "" {
		return false, errors.New("check endpoint not configured for this provider")
	}

	endpoint := strings.Replace(a.provider.Endpoints.Check, "{id}", proxyID, 1)

	responseBody, err := a.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		a.logger.WithError(err).Error("Failed to check proxy")
		return false, err
	}

	var result struct {
		Status  string `json:"status"`
		Active  bool   `json:"active"`
		Healthy bool   `json:"healthy"`
	}

	if err := json.Unmarshal(responseBody, &result); err != nil {
		a.logger.WithError(err).Error("Failed to unmarshal check response")
		return false, err
	}

	return result.Active || result.Healthy || result.Status == "active", nil
}

func (a *HTTPProviderAdapter) GetProviderConfig() models.ProxyProvider {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.provider
}

type MockProviderAdapter struct {
	name   string
	logger *logrus.Logger
}

func NewMockProviderAdapter(name string, logger *logrus.Logger) *MockProviderAdapter {
	return &MockProviderAdapter{
		name:   name,
		logger: logger,
	}
}

func (m *MockProviderAdapter) GetProviderName() string {
	return m.name
}

func (m *MockProviderAdapter) ListProxies(ctx context.Context) ([]models.ProxyResponse, error) {
	m.logger.Info("Mock: Listing proxies")
	return []models.ProxyResponse{
		{
			IP:       "192.168.1.100",
			Port:     8080,
			Username: "user1",
			Password: "pass1",
			Protocol: models.ProtocolHTTP,
			Country:  "US",
			City:     "New York",
			ExpireAt: time.Now().Add(24 * time.Hour),
		},
	}, nil
}

func (m *MockProviderAdapter) PurchaseProxy(ctx context.Context, params models.ProxyPurchaseParams) (*models.ProxyResponse, error) {
	m.logger.Info("Mock: Purchasing proxy")
	return &models.ProxyResponse{
		IP:       "10.0.0.1",
		Port:     3128,
		Username: "proxy_user",
		Password: "proxy_pass",
		Protocol: params.Protocol,
		Country:  params.Country,
		City:     "Mock City",
		ExpireAt: time.Now().Add(time.Duration(params.Duration)),
	}, nil
}

func (m *MockProviderAdapter) ReleaseProxy(ctx context.Context, proxyID string) error {
	m.logger.Info("Mock: Releasing proxy")
	return nil
}

func (m *MockProviderAdapter) RotateProxy(ctx context.Context, proxyID string) (*models.ProxyResponse, error) {
	m.logger.Info("Mock: Rotating proxy")
	return &models.ProxyResponse{
		IP:       "10.0.0.2",
		Port:     3128,
		Username: "new_user",
		Password: "new_pass",
		Protocol: models.ProtocolHTTP,
		Country:  "US",
		City:     "Mock City",
		ExpireAt: time.Now().Add(24 * time.Hour),
	}, nil
}

func (m *MockProviderAdapter) CheckProxy(ctx context.Context, proxyID string) (bool, error) {
	m.logger.Info("Mock: Checking proxy")
	return true, nil
}