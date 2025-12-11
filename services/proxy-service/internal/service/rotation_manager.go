package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/services/proxy-service/internal/models"
	"github.com/grigta/conveer/services/proxy-service/internal/repository"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RotationManager struct {
	proxyRepo        *repository.ProxyRepository
	providerRepo     *repository.ProviderRepository
	providerManager  *ProviderManager
	rabbitmq         *messaging.RabbitMQ
	logger           *logrus.Logger
	config           *config.Config
	checkInterval    time.Duration
	gracePeriod      time.Duration
	stopChan         chan struct{}
	wg               sync.WaitGroup
	rotationSchedule map[string]*time.Timer
	scheduleMutex    sync.RWMutex
}

type RotationRequest struct {
	ProxyID   string `json:"proxy_id"`
	AccountID string `json:"account_id"`
}

type RotationEvent struct {
	OldProxyID string    `json:"old_proxy_id"`
	NewProxyID string    `json:"new_proxy_id"`
	AccountID  string    `json:"account_id"`
	Timestamp  time.Time `json:"timestamp"`
}

func NewRotationManager(
	proxyRepo *repository.ProxyRepository,
	providerRepo *repository.ProviderRepository,
	providerManager *ProviderManager,
	rabbitmq *messaging.RabbitMQ,
	logger *logrus.Logger,
	config *config.Config,
) *RotationManager {
	checkInterval := 5 * time.Minute
	if config.Proxy.RotationCheckInterval != "" {
		if d, err := time.ParseDuration(config.Proxy.RotationCheckInterval); err == nil {
			checkInterval = d
		}
	}

	return &RotationManager{
		proxyRepo:        proxyRepo,
		providerRepo:     providerRepo,
		providerManager:  providerManager,
		rabbitmq:         rabbitmq,
		logger:           logger,
		config:           config,
		checkInterval:    checkInterval,
		gracePeriod:      5 * time.Minute,
		stopChan:         make(chan struct{}),
		rotationSchedule: make(map[string]*time.Timer),
	}
}

func (r *RotationManager) Start(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.monitorExpirations(ctx)
	}()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.consumeRotationRequests(ctx)
	}()
}

func (r *RotationManager) Stop() {
	close(r.stopChan)
	r.wg.Wait()
}

func (r *RotationManager) monitorExpirations(ctx context.Context) {
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	r.logger.Info("Starting expiration monitor")
	r.checkExpiredProxies(ctx)

	for {
		select {
		case <-ticker.C:
			r.checkExpiredProxies(ctx)
		case <-r.stopChan:
			r.logger.Info("Stopping expiration monitor")
			return
		case <-ctx.Done():
			r.logger.Info("Context cancelled, stopping expiration monitor")
			return
		}
	}
}

func (r *RotationManager) checkExpiredProxies(ctx context.Context) {
	r.logger.Debug("Checking for expired proxies")

	proxies, err := r.proxyRepo.GetExpiredProxies(ctx)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get expired proxies")
		return
	}

	if len(proxies) == 0 {
		r.logger.Debug("No expired proxies found")
		return
	}

	r.logger.Infof("Found %d expired proxies", len(proxies))

	for _, proxy := range proxies {
		binding, err := r.getActiveBinding(ctx, proxy.ID)
		if err != nil {
			r.logger.WithError(err).Errorf("Failed to get binding for proxy %s", proxy.ID.Hex())
			continue
		}

		if binding != nil {
			r.logger.Infof("Rotating expired proxy %s for account %s", proxy.ID.Hex(), binding.AccountID)
			if err := r.RotateProxy(ctx, proxy.ID, binding.AccountID); err != nil {
				r.logger.WithError(err).Errorf("Failed to rotate proxy %s", proxy.ID.Hex())
			}
		} else {
			r.logger.Infof("Releasing unbound expired proxy %s", proxy.ID.Hex())
			if err := r.releaseExpiredProxy(ctx, &proxy); err != nil {
				r.logger.WithError(err).Errorf("Failed to release proxy %s", proxy.ID.Hex())
			}
		}
	}
}

func (r *RotationManager) RotateProxy(ctx context.Context, proxyID primitive.ObjectID, accountID string) error {
	r.logger.Infof("Starting rotation for proxy %s, account %s", proxyID.Hex(), accountID)

	oldProxy, err := r.proxyRepo.GetProxyByID(ctx, proxyID)
	if err != nil {
		return err
	}

	provider, err := r.providerManager.GetProviderByName(oldProxy.Provider)
	if err != nil {
		r.logger.Warnf("Provider %s not available, trying another provider", oldProxy.Provider)
		providers := r.providerManager.GetActiveProviders()
		if len(providers) == 0 {
			return errors.New("no active providers available")
		}
		provider = providers[0]
	}

	params := models.ProxyPurchaseParams{
		Provider: provider.GetProviderName(),
		Type:     oldProxy.Type,
		Country:  oldProxy.Country,
		Protocol: oldProxy.Protocol,
		Duration: 24 * time.Hour,
		Quantity: 1,
	}

	newProxyResponse, err := provider.PurchaseProxy(ctx, params)
	if err != nil {
		r.logger.WithError(err).Error("Failed to purchase new proxy")
		return err
	}

	newProxy := &models.Proxy{
		Provider:  provider.GetProviderName(),
		IP:        newProxyResponse.IP,
		Port:      newProxyResponse.Port,
		Protocol:  newProxyResponse.Protocol,
		Username:  newProxyResponse.Username,
		Password:  newProxyResponse.Password,
		Type:      oldProxy.Type,
		Country:   newProxyResponse.Country,
		City:      newProxyResponse.City,
		Status:    models.ProxyStatusActive,
		ExpiresAt: newProxyResponse.ExpireAt,
	}

	if err := r.proxyRepo.CreateProxy(ctx, newProxy); err != nil {
		r.logger.WithError(err).Error("Failed to create new proxy")
		return err
	}

	if err := r.proxyRepo.UpdateProxyStatus(ctx, oldProxy.ID, models.ProxyStatusRotating); err != nil {
		r.logger.WithError(err).Error("Failed to update old proxy status to rotating")
	}

	if err := r.proxyRepo.BindProxyToAccount(ctx, newProxy.ID, accountID); err != nil {
		r.logger.WithError(err).Error("Failed to bind new proxy to account")
		if releaseErr := provider.ReleaseProxy(ctx, fmt.Sprintf("%s:%d", newProxy.IP, newProxy.Port)); releaseErr != nil {
			r.logger.WithError(releaseErr).Error("Failed to release unused proxy")
		}
		return err
	}

	go func() {
		time.Sleep(r.gracePeriod)

		if err := r.proxyRepo.ReleaseProxyBinding(ctx, oldProxy.ID); err != nil {
			r.logger.WithError(err).Error("Failed to release old proxy binding")
		}

		if err := provider.ReleaseProxy(ctx, fmt.Sprintf("%s:%d", oldProxy.IP, oldProxy.Port)); err != nil {
			r.logger.WithError(err).Error("Failed to release old proxy from provider")
		}

		if err := r.proxyRepo.UpdateProxyStatus(ctx, oldProxy.ID, models.ProxyStatusReleased); err != nil {
			r.logger.WithError(err).Error("Failed to update old proxy status to released")
		}
	}()

	if err := r.providerRepo.IncrementProviderCounter(ctx, provider.GetProviderName(), "total_rotated"); err != nil {
		r.logger.WithError(err).Error("Failed to increment provider rotation counter")
	}

	event := RotationEvent{
		OldProxyID: oldProxy.ID.Hex(),
		NewProxyID: newProxy.ID.Hex(),
		AccountID:  accountID,
		Timestamp:  time.Now(),
	}

	if err := r.rabbitmq.Publish("proxy.events", "proxy.rotated", event); err != nil {
		r.logger.WithError(err).Error("Failed to publish rotation event")
	}

	RecordProxyRotation("auto_expiry")

	r.logger.Infof("Successfully rotated proxy from %s to %s for account %s",
		oldProxy.ID.Hex(), newProxy.ID.Hex(), accountID)

	return nil
}

func (r *RotationManager) PreventServerIPExposure(ctx context.Context, oldProxyID, newProxyID primitive.ObjectID) error {
	newProxy, err := r.proxyRepo.GetProxyByID(ctx, newProxyID)
	if err != nil {
		return err
	}

	if newProxy.Status != models.ProxyStatusActive {
		return errors.New("new proxy is not active")
	}

	return nil
}

func (r *RotationManager) ScheduleRotation(ctx context.Context, proxyID primitive.ObjectID, accountID string, expiresAt time.Time) error {
	r.scheduleMutex.Lock()
	defer r.scheduleMutex.Unlock()

	key := fmt.Sprintf("%s:%s", proxyID.Hex(), accountID)

	// Cancel existing timer if present
	if existingTimer, exists := r.rotationSchedule[key]; exists {
		existingTimer.Stop()
	}

	rotationTime := expiresAt.Add(-10 * time.Minute) // Rotate 10 minutes before expiration
	delay := time.Until(rotationTime)

	if delay > 0 {
		timer := time.NewTimer(delay)
		r.rotationSchedule[key] = timer

		go func() {
			<-timer.C

			// Check if timer is still valid
			r.scheduleMutex.RLock()
			if currentTimer, exists := r.rotationSchedule[key]; exists && currentTimer == timer {
				r.scheduleMutex.RUnlock()

				request := RotationRequest{
					ProxyID:   proxyID.Hex(),
					AccountID: accountID,
				}

				if err := r.rabbitmq.Publish("", "proxy.rotation", request); err != nil {
					r.logger.WithError(err).Error("Failed to schedule rotation")
				}

				// Clean up from schedule
				r.scheduleMutex.Lock()
				delete(r.rotationSchedule, key)
				r.scheduleMutex.Unlock()
			} else {
				r.scheduleMutex.RUnlock()
			}
		}()

		r.logger.Infof("Scheduled rotation for proxy %s at %s", proxyID.Hex(), rotationTime)
	}

	return nil
}

func (r *RotationManager) consumeRotationRequests(ctx context.Context) {
	r.logger.Info("Starting rotation request consumer")

	handler := func(msg []byte) error {
		var request RotationRequest

		if err := json.Unmarshal(msg, &request); err != nil {
			r.logger.WithError(err).Error("Failed to unmarshal rotation request")
			return err
		}

		proxyID, err := primitive.ObjectIDFromHex(request.ProxyID)
		if err != nil {
			r.logger.WithError(err).Error("Invalid proxy ID in rotation request")
			return err
		}

		if err := r.RotateProxy(ctx, proxyID, request.AccountID); err != nil {
			r.logger.WithError(err).Error("Failed to rotate proxy")
			RecordRotationError()
			return err
		}

		return nil
	}

	if err := r.rabbitmq.ConsumeWithHandler(ctx, "proxy.rotation", "proxy-rotation-consumer", handler); err != nil {
		r.logger.WithError(err).Error("Failed to start rotation consumer")
	}
}

func (r *RotationManager) getActiveBinding(ctx context.Context, proxyID primitive.ObjectID) (*models.ProxyBinding, error) {
	return r.proxyRepo.GetActiveBindingByProxyID(ctx, proxyID)
}

func (r *RotationManager) releaseExpiredProxy(ctx context.Context, proxy *models.Proxy) error {
	// First release any active bindings
	if err := r.proxyRepo.ReleaseProxyBinding(ctx, proxy.ID); err != nil {
		r.logger.WithError(err).Errorf("Failed to release binding for proxy %s", proxy.ID.Hex())
	}

	provider, err := r.providerManager.GetProviderByName(proxy.Provider)
	if err != nil {
		r.logger.WithError(err).Warnf("Provider %s not available for releasing proxy", proxy.Provider)
	} else {
		if err := provider.ReleaseProxy(ctx, fmt.Sprintf("%s:%d", proxy.IP, proxy.Port)); err != nil {
			r.logger.WithError(err).Error("Failed to release proxy from provider")
		}
	}

	if err := r.proxyRepo.UpdateProxyStatus(ctx, proxy.ID, models.ProxyStatusReleased); err != nil {
		return err
	}

	return nil
}

func (r *RotationManager) CancelScheduledRotation(proxyID primitive.ObjectID, accountID string) {
	r.scheduleMutex.Lock()
	defer r.scheduleMutex.Unlock()

	key := fmt.Sprintf("%s:%s", proxyID.Hex(), accountID)

	if timer, exists := r.rotationSchedule[key]; exists {
		timer.Stop()
		delete(r.rotationSchedule, key)
		r.logger.Infof("Cancelled scheduled rotation for proxy %s", proxyID.Hex())
	}
}
