package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"conveer/pkg/config"
	"conveer/pkg/messaging"
	"conveer/services/proxy-service/internal/models"
	"conveer/services/proxy-service/internal/repository"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HealthChecker struct {
	proxyRepo      *repository.ProxyRepository
	rabbitmq       *messaging.RabbitMQ
	logger         *logrus.Logger
	config         *config.Config
	checkInterval  time.Duration
	maxFailedChecks int
	ipqsAPIKey     string
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

type IPQSResponse struct {
	Success      bool    `json:"success"`
	Message      string  `json:"message"`
	FraudScore   float64 `json:"fraud_score"`
	CountryCode  string  `json:"country_code"`
	City         string  `json:"city"`
	ISP          string  `json:"ISP"`
	ASN          int     `json:"ASN"`
	Organization string  `json:"organization"`
	IsCrawler    bool    `json:"is_crawler"`
	Timezone     string  `json:"timezone"`
	Mobile       bool    `json:"mobile"`
	Host         string  `json:"host"`
	Proxy        bool    `json:"proxy"`
	VPN          bool    `json:"vpn"`
	TOR          bool    `json:"tor"`
	ActiveVPN    bool    `json:"active_vpn"`
	ActiveTOR    bool    `json:"active_tor"`
	RecentAbuse  bool    `json:"recent_abuse"`
	BotStatus    bool    `json:"bot_status"`
	ConnectionType string `json:"connection_type"`
	AbuseVelocity  string `json:"abuse_velocity"`
}

func NewHealthChecker(
	proxyRepo *repository.ProxyRepository,
	rabbitmq *messaging.RabbitMQ,
	logger *logrus.Logger,
	config *config.Config,
) *HealthChecker {
	checkInterval := 15 * time.Minute
	if config.Proxy.HealthCheckInterval != "" {
		if d, err := time.ParseDuration(config.Proxy.HealthCheckInterval); err == nil {
			checkInterval = d
		}
	}

	maxFailedChecks := 3
	if config.Proxy.MaxFailedChecks > 0 {
		maxFailedChecks = config.Proxy.MaxFailedChecks
	}

	return &HealthChecker{
		proxyRepo:       proxyRepo,
		rabbitmq:        rabbitmq,
		logger:          logger,
		config:          config,
		checkInterval:   checkInterval,
		maxFailedChecks: maxFailedChecks,
		ipqsAPIKey:      config.Proxy.IPQualityScoreAPIKey,
		stopChan:        make(chan struct{}),
	}
}

func (h *HealthChecker) Start(ctx context.Context) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.runHealthChecks(ctx)
	}()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.consumeHealthCheckRequests(ctx)
	}()
}

func (h *HealthChecker) Stop() {
	close(h.stopChan)
	h.wg.Wait()
}

func (h *HealthChecker) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	h.logger.Info("Starting periodic health checks")
	h.performHealthChecks(ctx)

	for {
		select {
		case <-ticker.C:
			h.performHealthChecks(ctx)
		case <-h.stopChan:
			h.logger.Info("Stopping health checks")
			return
		case <-ctx.Done():
			h.logger.Info("Context cancelled, stopping health checks")
			return
		}
	}
}

func (h *HealthChecker) performHealthChecks(ctx context.Context) {
	h.logger.Info("Performing scheduled health checks")

	proxies, err := h.proxyRepo.GetProxiesByStatus(ctx, models.ProxyStatusActive)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get active proxies for health check")
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent checks

	for _, proxy := range proxies {
		wg.Add(1)
		go func(p models.Proxy) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			health := h.CheckProxyHealth(ctx, &p)

			if err := h.proxyRepo.UpdateProxyHealth(ctx, p.ID, health); err != nil {
				h.logger.WithError(err).Error("Failed to update proxy health")
			}

			if health.FailedChecks >= h.maxFailedChecks {
				h.HandleFailedCheck(ctx, &p)
			}
		}(proxy)
	}

	wg.Wait()

	// Update metrics after all checks
	stats, err := h.proxyRepo.GetProxyStatistics(ctx)
	if err == nil {
		SetActiveProxies(float64(stats.ActiveProxies))
		SetProxyBindings(float64(stats.TotalBindings))
	}

	wg.Wait()
	h.logger.Info("Completed scheduled health checks")
}

func (h *HealthChecker) CheckProxyHealth(ctx context.Context, proxy *models.Proxy) *models.ProxyHealth {
	start := time.Now()
	health := &models.ProxyHealth{
		ProxyID:   proxy.ID,
		LastCheck: time.Now(),
	}

	latency := h.testLatency(ctx, proxy)
	health.Latency = latency

	if latency < 0 {
		health.FailedChecks++
		RecordHealthCheck("failed")
		return health
	}

	RecordLatency(float64(latency))

	fraudData := h.checkFraudScore(ctx, proxy.IP)
	if fraudData != nil {
		health.FraudScore = fraudData.FraudScore
		health.IsVPN = fraudData.VPN
		health.IsProxy = fraudData.Proxy
		health.IsTor = fraudData.TOR
		health.BlacklistStatus = fraudData.RecentAbuse
		RecordFraudScore(fraudData.FraudScore)
	}

	if !h.verifyGeoLocation(ctx, proxy, fraudData) {
		h.logger.Warnf("Geo-location mismatch for proxy %s", proxy.ID.Hex())
	}

	RecordHealthCheck("success")
	RecordHealthCheckDuration(proxy.ID.Hex(), time.Since(start).Seconds())

	return health
}

func (h *HealthChecker) testLatency(ctx context.Context, proxy *models.Proxy) int {
	proxyURL := fmt.Sprintf("%s://%s:%s@%s:%d",
		proxy.Protocol,
		proxy.Username,
		proxy.Password,
		proxy.IP,
		proxy.Port,
	)

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		h.logger.WithError(err).Error("Failed to parse proxy URL")
		return -1
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 5 * time.Second,
			}).DialContext,
		},
	}

	start := time.Now()
	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		h.logger.WithError(err).Errorf("Failed to test proxy latency for %s", proxy.ID.Hex())
		return -1
	}
	defer resp.Body.Close()

	latency := int(time.Since(start).Milliseconds())

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.WithError(err).Error("Failed to read response body")
		return latency
	}

	var result struct {
		Origin string `json:"origin"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		h.logger.WithError(err).Error("Failed to unmarshal response")
	} else {
		h.logger.Debugf("Proxy %s response IP: %s, latency: %dms", proxy.ID.Hex(), result.Origin, latency)
	}

	return latency
}

func (h *HealthChecker) checkFraudScore(ctx context.Context, ip string) *IPQSResponse {
	if h.ipqsAPIKey == "" {
		h.logger.Debug("IPQualityScore API key not configured, skipping fraud check")
		return nil
	}

	url := fmt.Sprintf("https://ipqualityscore.com/api/json/ip/%s/%s", h.ipqsAPIKey, ip)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check fraud score")
		return nil
	}
	defer resp.Body.Close()

	var ipqsResp IPQSResponse
	if err := json.NewDecoder(resp.Body).Decode(&ipqsResp); err != nil {
		h.logger.WithError(err).Error("Failed to decode IPQualityScore response")
		return nil
	}

	if !ipqsResp.Success {
		h.logger.Errorf("IPQualityScore API error: %s", ipqsResp.Message)
		return nil
	}

	return &ipqsResp
}

func (h *HealthChecker) verifyGeoLocation(ctx context.Context, proxy *models.Proxy, fraudData *IPQSResponse) bool {
	if fraudData == nil {
		return true
	}

	if proxy.Country != "" && fraudData.CountryCode != "" {
		return proxy.Country == fraudData.CountryCode
	}

	return true
}

func (h *HealthChecker) HandleFailedCheck(ctx context.Context, proxy *models.Proxy) {
	h.logger.Warnf("Proxy %s has failed %d health checks, marking as banned",
		proxy.ID.Hex(), h.maxFailedChecks)

	if err := h.proxyRepo.UpdateProxyStatus(ctx, proxy.ID, models.ProxyStatusBanned); err != nil {
		h.logger.WithError(err).Error("Failed to update proxy status to banned")
		return
	}

	event := map[string]interface{}{
		"proxy_id": proxy.ID.Hex(),
		"reason":   "health_check_failed",
		"failures": h.maxFailedChecks,
	}

	if err := h.rabbitmq.Publish("proxy.events", "proxy.health_failed", event); err != nil {
		h.logger.WithError(err).Error("Failed to publish health failed event")
	}
}

func (h *HealthChecker) consumeHealthCheckRequests(ctx context.Context) {
	h.logger.Info("Starting health check request consumer")

	handler := func(msg []byte) error {
		var request struct {
			ProxyID string `json:"proxy_id"`
		}

		if err := json.Unmarshal(msg, &request); err != nil {
			h.logger.WithError(err).Error("Failed to unmarshal health check request")
			return err
		}

		proxyID, err := primitive.ObjectIDFromHex(request.ProxyID)
		if err != nil {
			h.logger.WithError(err).Error("Invalid proxy ID in health check request")
			return err
		}

		proxy, err := h.proxyRepo.GetProxyByID(ctx, proxyID)
		if err != nil {
			h.logger.WithError(err).Error("Failed to get proxy for health check")
			return err
		}

		health := h.CheckProxyHealth(ctx, proxy)

		if err := h.proxyRepo.UpdateProxyHealth(ctx, proxy.ID, health); err != nil {
			h.logger.WithError(err).Error("Failed to update proxy health")
			return err
		}

		h.logger.Infof("Completed health check for proxy %s", proxy.ID.Hex())
		return nil
	}

	if err := h.rabbitmq.ConsumeWithHandler(ctx, "proxy.health_check", "proxy-health-check-consumer", handler); err != nil {
		h.logger.WithError(err).Error("Failed to start health check consumer")
	}
}

func (h *HealthChecker) ScheduleHealthCheck(ctx context.Context, proxyID primitive.ObjectID, delay time.Duration) error {
	request := map[string]interface{}{
		"proxy_id": proxyID.Hex(),
	}

	if delay > 0 {
		time.AfterFunc(delay, func() {
			if err := h.rabbitmq.Publish("", "proxy.health_check", request); err != nil {
				h.logger.WithError(err).Error("Failed to schedule health check")
			}
		})
	} else {
		if err := h.rabbitmq.Publish("", "proxy.health_check", request); err != nil {
			return err
		}
	}

	return nil
}