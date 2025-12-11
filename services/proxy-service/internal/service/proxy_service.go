package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"conveer/pkg/cache"
	"conveer/pkg/config"
	"conveer/pkg/messaging"
	"conveer/services/proxy-service/internal/models"
	"conveer/services/proxy-service/internal/repository"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProxyService struct {
	proxyRepo       *repository.ProxyRepository
	providerRepo    *repository.ProviderRepository
	providerManager *ProviderManager
	healthChecker   *HealthChecker
	rotationManager *RotationManager
	rabbitmq        *messaging.RabbitMQ
	redis           *cache.RedisCache
	logger          *logrus.Logger
	config          *config.Config
	mu              sync.RWMutex
}

type AllocationEvent struct {
	ProxyID   string    `json:"proxy_id"`
	AccountID string    `json:"account_id"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Type      string    `json:"type"`
	Country   string    `json:"country"`
	Timestamp time.Time `json:"timestamp"`
}

func NewProxyService(
	proxyRepo *repository.ProxyRepository,
	providerRepo *repository.ProviderRepository,
	providerManager *ProviderManager,
	healthChecker *HealthChecker,
	rotationManager *RotationManager,
	rabbitmq *messaging.RabbitMQ,
	redis *cache.RedisCache,
	logger *logrus.Logger,
	config *config.Config,
) *ProxyService {
	return &ProxyService{
		proxyRepo:       proxyRepo,
		providerRepo:    providerRepo,
		providerManager: providerManager,
		healthChecker:   healthChecker,
		rotationManager: rotationManager,
		rabbitmq:        rabbitmq,
		redis:           redis,
		logger:          logger,
		config:          config,
	}
}

func (s *ProxyService) Start(ctx context.Context) {
	s.healthChecker.Start(ctx)
	s.rotationManager.Start(ctx)

	go s.consumeAllocationRequests(ctx)
	go s.consumeReleaseRequests(ctx)
	go s.RefreshProxyPoolPeriodically(ctx)
}

func (s *ProxyService) Stop() {
	s.healthChecker.Stop()
	s.rotationManager.Stop()
}

func (s *ProxyService) AllocateProxy(ctx context.Context, request models.ProxyAllocationRequest) (*models.Proxy, error) {
	s.logger.Infof("Allocating proxy for account %s", request.AccountID)

	existingProxy, err := s.proxyRepo.GetProxyByAccountID(ctx, request.AccountID)
	if err != nil {
		return nil, err
	}

	if existingProxy != nil {
		s.logger.Infof("Account %s already has proxy %s", request.AccountID, existingProxy.ID.Hex())
		return existingProxy, nil
	}

	cacheKey := fmt.Sprintf("proxy:account:%s", request.AccountID)
	if cachedProxyID, err := s.redis.Get(ctx, cacheKey); err == nil && cachedProxyID != "" {
		if proxyID, err := primitive.ObjectIDFromHex(cachedProxyID); err == nil {
			if proxy, err := s.proxyRepo.GetProxyByID(ctx, proxyID); err == nil && proxy.Status == models.ProxyStatusActive {
				s.logger.Infof("Found cached proxy %s for account %s", proxy.ID.Hex(), request.AccountID)
				return proxy, nil
			}
		}
	}

	filters := models.ProxyFilters{
		Type:    request.Type,
		Country: request.Country,
		Status:  models.ProxyStatusActive,
	}

	availableProxies, err := s.proxyRepo.GetAvailableProxies(ctx, filters)
	if err != nil {
		return nil, err
	}

	var proxy *models.Proxy

	if len(availableProxies) > 0 {
		for _, p := range availableProxies {
			if err := s.proxyRepo.BindProxyToAccount(ctx, p.ID, request.AccountID); err == nil {
				proxy = &p
				break
			}
		}
	}

	if proxy == nil {
		s.logger.Info("No available proxies, purchasing new one")

		newProxy, err := s.purchaseNewProxy(ctx, request)
		if err != nil {
			return nil, err
		}

		if err := s.proxyRepo.CreateProxy(ctx, newProxy); err != nil {
			return nil, err
		}

		if err := s.proxyRepo.BindProxyToAccount(ctx, newProxy.ID, request.AccountID); err != nil {
			return nil, err
		}

		proxy = newProxy
	}

	if err := s.redis.Set(ctx, cacheKey, proxy.ID.Hex(), 1*time.Hour); err != nil {
		s.logger.WithError(err).Warn("Failed to cache proxy allocation")
	}

	if err := s.rotationManager.ScheduleRotation(ctx, proxy.ID, request.AccountID, proxy.ExpiresAt); err != nil {
		s.logger.WithError(err).Warn("Failed to schedule rotation")
	}

	event := AllocationEvent{
		ProxyID:   proxy.ID.Hex(),
		AccountID: request.AccountID,
		IP:        proxy.IP,
		Port:      proxy.Port,
		Type:      string(proxy.Type),
		Country:   proxy.Country,
		Timestamp: time.Now(),
	}

	if err := s.rabbitmq.Publish("proxy.events", "proxy.allocated", event); err != nil {
		s.logger.WithError(err).Error("Failed to publish allocation event")
	}

	RecordProxyAllocation(string(proxy.Type), proxy.Country)

	s.logger.Infof("Successfully allocated proxy %s for account %s", proxy.ID.Hex(), request.AccountID)

	return proxy, nil
}

func (s *ProxyService) ReleaseProxy(ctx context.Context, accountID string) error {
	s.logger.Infof("Releasing proxy for account %s", accountID)

	proxy, err := s.proxyRepo.GetProxyByAccountID(ctx, accountID)
	if err != nil {
		return err
	}

	if proxy == nil {
		return errors.New("no proxy found for account")
	}

	if err := s.proxyRepo.ReleaseProxyBinding(ctx, proxy.ID); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("proxy:account:%s", accountID)
	if err := s.redis.Delete(ctx, cacheKey); err != nil {
		s.logger.WithError(err).Warn("Failed to delete cache entry")
	}

	s.rotationManager.CancelScheduledRotation(proxy.ID, accountID)

	provider, err := s.providerManager.GetProviderByName(proxy.Provider)
	if err == nil {
		if err := provider.ReleaseProxy(ctx, fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)); err != nil {
			s.logger.WithError(err).Warn("Failed to release proxy from provider")
		}
	}

	RecordProxyRelease()

	event := map[string]interface{}{
		"proxy_id":   proxy.ID.Hex(),
		"account_id": accountID,
		"timestamp":  time.Now(),
	}

	if err := s.rabbitmq.Publish("proxy.events", "proxy.released", event); err != nil {
		s.logger.WithError(err).Error("Failed to publish release event")
	}

	if err := s.providerRepo.IncrementProviderCounter(ctx, proxy.Provider, "total_released"); err != nil {
		s.logger.WithError(err).Warn("Failed to increment provider release counter")
	}

	s.logger.Infof("Successfully released proxy %s for account %s", proxy.ID.Hex(), accountID)

	return nil
}

func (s *ProxyService) GetProxyForAccount(ctx context.Context, accountID string) (*models.Proxy, error) {
	cacheKey := fmt.Sprintf("proxy:account:%s", accountID)
	if cachedProxyID, err := s.redis.Get(ctx, cacheKey); err == nil && cachedProxyID != "" {
		if proxyID, err := primitive.ObjectIDFromHex(cachedProxyID); err == nil {
			if proxy, err := s.proxyRepo.GetProxyByID(ctx, proxyID); err == nil {
				return proxy, nil
			}
		}
	}

	proxy, err := s.proxyRepo.GetProxyByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if proxy != nil {
		if err := s.redis.Set(ctx, cacheKey, proxy.ID.Hex(), 1*time.Hour); err != nil {
			s.logger.WithError(err).Warn("Failed to cache proxy lookup")
		}
	}

	return proxy, nil
}

func (s *ProxyService) RefreshProxyPool(ctx context.Context) error {
	s.logger.Info("Refreshing proxy pool")

	stats, err := s.proxyRepo.GetProxyStatistics(ctx)
	if err != nil {
		return err
	}

	minPoolSize := 10
	targetPoolSize := int(stats.TotalBindings) + minPoolSize

	if stats.ActiveProxies >= int64(targetPoolSize) {
		s.logger.Info("Proxy pool is sufficient")
		return nil
	}

	needed := targetPoolSize - int(stats.ActiveProxies)
	s.logger.Infof("Need to purchase %d new proxies", needed)

	providers := s.providerManager.GetActiveProviders()
	if len(providers) == 0 {
		return errors.New("no active providers available")
	}

	var wg sync.WaitGroup
	errorsChan := make(chan error, needed)

	for i := 0; i < needed; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			provider := providers[index%len(providers)]
			params := models.ProxyPurchaseParams{
				Provider: provider.GetProviderName(),
				Type:     models.ProxyTypeMobile,
				Protocol: models.ProtocolHTTP,
				Duration: 24 * time.Hour,
				Quantity: 1,
			}

			proxyResp, err := provider.PurchaseProxy(ctx, params)
			if err != nil {
				errorsChan <- err
				return
			}

			proxy := &models.Proxy{
				Provider:  provider.GetProviderName(),
				IP:        proxyResp.IP,
				Port:      proxyResp.Port,
				Protocol:  proxyResp.Protocol,
				Username:  proxyResp.Username,
				Password:  proxyResp.Password,
				Type:      params.Type,
				Country:   proxyResp.Country,
				City:      proxyResp.City,
				Status:    models.ProxyStatusActive,
				ExpiresAt: proxyResp.ExpireAt,
			}

			if err := s.proxyRepo.CreateProxy(ctx, proxy); err != nil {
				errorsChan <- err
				return
			}

			s.logger.Infof("Added new proxy %s to pool", proxy.ID.Hex())
		}(i)
	}

	wg.Wait()
	close(errorsChan)

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		s.logger.Warnf("Failed to purchase %d proxies", len(errors))
	}

	s.logger.Info("Proxy pool refresh completed")
	return nil
}

func (s *ProxyService) RefreshProxyPoolPeriodically(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.RefreshProxyPool(ctx); err != nil {
				s.logger.WithError(err).Error("Failed to refresh proxy pool")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *ProxyService) GetProxyStatistics(ctx context.Context) (*models.ProxyStats, error) {
	return s.proxyRepo.GetProxyStatistics(ctx)
}

func (s *ProxyService) purchaseNewProxy(ctx context.Context, request models.ProxyAllocationRequest) (*models.Proxy, error) {
	providers := s.providerManager.GetActiveProviders()
	if len(providers) == 0 {
		return nil, errors.New("no active providers available")
	}

	var lastError error

	for _, provider := range providers {
		params := models.ProxyPurchaseParams{
			Provider: provider.GetProviderName(),
			Type:     request.Type,
			Country:  request.Country,
			Protocol: request.Protocol,
			Duration: 24 * time.Hour,
			Quantity: 1,
		}

		proxyResp, err := provider.PurchaseProxy(ctx, params)
		if err != nil {
			lastError = err
			s.logger.WithError(err).Warnf("Failed to purchase proxy from %s", provider.GetProviderName())
			continue
		}

		proxy := &models.Proxy{
			Provider:  provider.GetProviderName(),
			IP:        proxyResp.IP,
			Port:      proxyResp.Port,
			Protocol:  proxyResp.Protocol,
			Username:  proxyResp.Username,
			Password:  proxyResp.Password,
			Type:      request.Type,
			Country:   proxyResp.Country,
			City:      proxyResp.City,
			Status:    models.ProxyStatusActive,
			ExpiresAt: proxyResp.ExpireAt,
		}

		if err := s.providerRepo.IncrementProviderCounter(ctx, provider.GetProviderName(), "total_allocated"); err != nil {
			s.logger.WithError(err).Warn("Failed to increment provider allocation counter")
		}

		return proxy, nil
	}

	if lastError != nil {
		return nil, lastError
	}

	return nil, errors.New("failed to purchase proxy from any provider")
}

func (s *ProxyService) consumeAllocationRequests(ctx context.Context) {
	s.logger.Info("Starting allocation request consumer")

	handler := func(msg []byte) error {
		var request models.ProxyAllocationRequest

		if err := json.Unmarshal(msg, &request); err != nil {
			s.logger.WithError(err).Error("Failed to unmarshal allocation request")
			return err
		}

		_, err := s.AllocateProxy(ctx, request)
		if err != nil {
			s.logger.WithError(err).Error("Failed to allocate proxy")
			RecordAllocationError()
			return err
		}

		return nil
	}

	if err := s.rabbitmq.ConsumeWithHandler(ctx, "proxy.allocate", "proxy-allocate-consumer", handler); err != nil {
		s.logger.WithError(err).Error("Failed to start allocation consumer")
	}
}

func (s *ProxyService) consumeReleaseRequests(ctx context.Context) {
	s.logger.Info("Starting release request consumer")

	handler := func(msg []byte) error {
		var request struct {
			AccountID string `json:"account_id"`
		}

		if err := json.Unmarshal(msg, &request); err != nil {
			s.logger.WithError(err).Error("Failed to unmarshal release request")
			return err
		}

		if err := s.ReleaseProxy(ctx, request.AccountID); err != nil {
			s.logger.WithError(err).Error("Failed to release proxy")
			return err
		}

		return nil
	}

	if err := s.rabbitmq.ConsumeWithHandler(ctx, "proxy.release", "proxy-release-consumer", handler); err != nil {
		s.logger.WithError(err).Error("Failed to start release consumer")
	}
}

func (s *ProxyService) ForceRotateProxy(ctx context.Context, accountID string) (*models.Proxy, error) {
	s.logger.Infof("Force rotating proxy for account %s", accountID)

	proxy, err := s.proxyRepo.GetProxyByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if proxy == nil {
		return nil, errors.New("no proxy found for account")
	}

	if err := s.rotationManager.RotateProxy(ctx, proxy.ID, accountID); err != nil {
		return nil, err
	}

	RecordProxyRotation("manual")

	time.Sleep(1 * time.Second)

	newProxy, err := s.proxyRepo.GetProxyByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return newProxy, nil
}
