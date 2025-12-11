package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	analyticspb "github.com/grigta/conveer/services/analytics-service/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StatsService interface {
	GetAccountStats(ctx context.Context, platform string) (*AccountStats, error)
	GetWarmingStats(ctx context.Context, platform string) (*WarmingStats, error)
	GetProxyStats(ctx context.Context) (*ProxyStats, error)
	GetSMSStats(ctx context.Context) (*SMSStats, error)
	GetOverallStats(ctx context.Context) (*OverallStats, error)
	GetDetailedStats(ctx context.Context, platform string) (*DetailedStats, error)
}

type AccountStats struct {
	Total       int64              `json:"total"`
	ByStatus    map[string]int64   `json:"by_status"`
	SuccessRate float64            `json:"success_rate"`
	LastHour    int64              `json:"last_hour"`
	Last24Hours int64              `json:"last_24_hours"`
}

type WarmingStats struct {
	InProgress int64   `json:"in_progress"`
	Completed  int64   `json:"completed"`
	Failed     int64   `json:"failed"`
	AvgDuration float64 `json:"avg_duration_days"`
}

type ProxyStats struct {
	Total    int64 `json:"total"`
	Active   int64 `json:"active"`
	Expired  int64 `json:"expired"`
	Banned   int64 `json:"banned"`
}

type SMSStats struct {
	TotalSpent       float64 `json:"total_spent"`
	ActivationsToday int64   `json:"activations_today"`
	Balance          float64 `json:"balance"`
}

type OverallStats struct {
	TotalAccounts      int64              `json:"total_accounts"`
	AccountsByPlatform map[string]int64   `json:"accounts_by_platform"`
	AccountsByStatus   map[string]int64   `json:"accounts_by_status"`
	WarmingTasks       WarmingStats       `json:"warming_tasks"`
	ProxyStats         ProxyStats         `json:"proxy_stats"`
	SMSStats           SMSStats           `json:"sms_stats"`
	SuccessRate        float64            `json:"success_rate"`
	Last24HoursCreated int64              `json:"last_24_hours_created"`
}

type DetailedStats struct {
	Platform           string             `json:"platform"`
	StatusDistribution map[string]int64   `json:"status_distribution"`
	SuccessRate        float64            `json:"success_rate"`
	AvgWarmingDuration float64            `json:"avg_warming_duration"`
	TopErrors          []string           `json:"top_errors"`
	Last7DaysActivity  map[string]int64   `json:"last_7_days_activity"`
}

type statsService struct {
	grpcClients *GRPCClients
	cache       *CacheHelper
	analyticsClient analyticspb.AnalyticsServiceClient
}

func NewStatsService(grpcClients *GRPCClients) StatsService {
	var cache *CacheHelper
	if grpcClients.RedisClient != nil {
		cache = NewCacheHelper(grpcClients.RedisClient)
	}

	return &statsService{
		grpcClients: grpcClients,
		cache:       cache,
		analyticsClient: grpcClients.AnalyticsServiceClient,
	}
}

func (s *statsService) GetAccountStats(ctx context.Context, platform string) (*AccountStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Try to get from cache first
	if s.cache != nil {
		if cachedStats, err := s.cache.GetAccountStats(ctx, platform); err == nil {
			return cachedStats, nil
		}
	}

	// Get fresh data from service
	var stats *AccountStats
	var err error

	switch platform {
	case "vk":
		stats, err = s.getVKStats(ctx)
	case "telegram":
		stats, err = s.getTelegramStats(ctx)
	case "mail", "max":
		return nil, fmt.Errorf("platform %s not yet implemented", platform)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	if err != nil {
		// Try to return stale cache if available
		if s.cache != nil {
			if cachedStats, cacheErr := s.cache.GetAccountStats(ctx, platform); cacheErr == nil {
				return cachedStats, fmt.Errorf("service unavailable, returning cached data: %w", err)
			}
		}
		return nil, err
	}

	// Cache the fresh data
	if s.cache != nil && stats != nil {
		_ = s.cache.SetAccountStats(ctx, platform, stats) // ignore cache errors
	}

	return stats, nil
}

func (s *statsService) getVKStats(ctx context.Context) (*AccountStats, error) {
	if s.grpcClients.VKServiceClient == nil {
		return nil, fmt.Errorf("VK service not available")
	}

	pbStats, err := s.grpcClients.VKServiceClient.GetStatistics(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get VK statistics: %w", err)
	}

	stats := &AccountStats{
		Total:       pbStats.Total,
		ByStatus:    pbStats.ByStatus,
		SuccessRate: pbStats.SuccessRate,
		LastHour:    pbStats.LastHour,
		Last24Hours: pbStats.Last24Hours,
	}

	return stats, nil
}

func (s *statsService) getTelegramStats(ctx context.Context) (*AccountStats, error) {
	if s.grpcClients.TelegramServiceClient == nil {
		return nil, fmt.Errorf("Telegram service not available")
	}

	pbStats, err := s.grpcClients.TelegramServiceClient.GetStatistics(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Telegram statistics: %w", err)
	}

	stats := &AccountStats{
		Total:       pbStats.Total,
		ByStatus:    pbStats.ByStatus,
		SuccessRate: pbStats.SuccessRate,
		LastHour:    pbStats.LastHour,
		Last24Hours: pbStats.Last24Hours,
	}

	return stats, nil
}

func (s *statsService) GetWarmingStats(ctx context.Context, platform string) (*WarmingStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.grpcClients.WarmingClient == nil {
		return nil, fmt.Errorf("Warming service not available")
	}

	// TODO: Реализовать gRPC вызовы к warming service
	// Пока используем заглушку
	stats := &WarmingStats{
		InProgress:  80,
		Completed:   240,
		Failed:      10,
		AvgDuration: 21.5,
	}
	return stats, nil
}

func (s *statsService) GetProxyStats(ctx context.Context) (*ProxyStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.grpcClients.ProxyClient == nil {
		return nil, fmt.Errorf("Proxy service not available")
	}

	// TODO: Реализовать gRPC вызовы к proxy service
	// Пока используем заглушку
	stats := &ProxyStats{
		Total:   500,
		Active:  450,
		Expired: 30,
		Banned:  20,
	}
	return stats, nil
}

func (s *statsService) GetSMSStats(ctx context.Context) (*SMSStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.grpcClients.SMSClient == nil {
		return nil, fmt.Errorf("SMS service not available")
	}

	// TODO: Реализовать gRPC вызовы к SMS service
	// Пока используем заглушку
	stats := &SMSStats{
		TotalSpent:       1250.00,
		ActivationsToday: 45,
		Balance:          5000.00,
	}
	return stats, nil
}

func (s *statsService) GetOverallStats(ctx context.Context) (*OverallStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Try to get from cache first
	if s.cache != nil {
		if cachedStats, err := s.cache.GetOverallStats(ctx); err == nil {
			return cachedStats, nil
		}
	}

	// Use analytics service if available
	if s.analyticsClient != nil {
		// Get analytics for the last 30 days
		endTime := time.Now()
		startTime := endTime.AddDate(0, -1, 0)

		req := &analyticspb.AnalyticsRequest{
			StartDate: timestamppb.New(startTime),
			EndDate:   timestamppb.New(endTime),
		}

		analyticsData, err := s.analyticsClient.GetOverallAnalytics(ctx, req)
		if err == nil {
			// Map analytics data to our OverallStats structure
			stats := &OverallStats{
				TotalAccounts:      analyticsData.TotalAccounts,
				AccountsByPlatform: analyticsData.AccountsByPlatform,
				AccountsByStatus:   analyticsData.AccountsByStatus,
				SuccessRate:        analyticsData.OverallSuccessRate,
			}

			// Map resources summary to our structures
			if analyticsData.Resources != nil {
				stats.ProxyStats = ProxyStats{
					Active: analyticsData.Resources.ActiveProxies,
					Banned: analyticsData.Resources.BannedProxies,
				}

				stats.SMSStats = SMSStats{
					Balance: analyticsData.Resources.SmsBalance,
				}

				stats.WarmingTasks = WarmingStats{
					InProgress: analyticsData.Resources.WarmingTasksActive,
				}
			}

			// Map expenses
			if analyticsData.Expenses != nil {
				stats.SMSStats.TotalSpent = analyticsData.Expenses.SmsSpent
			}

			// Map performance
			if analyticsData.Performance != nil {
				stats.WarmingTasks.AvgDuration = analyticsData.Performance.AvgWarmingDays
				stats.Last24HoursCreated = analyticsData.Performance.AccountsCreatedToday
			}

			// Cache the result
			if s.cache != nil && stats != nil {
				_ = s.cache.SetOverallStats(ctx, stats)
			}

			return stats, nil
		}
	}

	// Fallback to original implementation if analytics service is not available
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	stats := &OverallStats{
		AccountsByPlatform: make(map[string]int64),
		AccountsByStatus:   make(map[string]int64),
	}

	// Get stats for each platform in parallel
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			accountStats, err := s.GetAccountStats(ctx, p)
			mu.Lock()
			if err != nil {
				if p == "vk" || p == "telegram" { // критичные платформы
					errors = append(errors, fmt.Errorf("failed to get %s stats: %w", p, err))
				}
				// для mail/max ошибки не критичны
			} else {
				stats.AccountsByPlatform[p] = accountStats.Total
				stats.TotalAccounts += accountStats.Total

				for status, count := range accountStats.ByStatus {
					stats.AccountsByStatus[status] += count
				}
			}
			mu.Unlock()
		}(platform)
	}

	// Get warming stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		warmingStats, err := s.GetWarmingStats(ctx, "")
		mu.Lock()
		if err != nil {
			// Warming stats не критичны, просто логируем
			errors = append(errors, fmt.Errorf("warming stats unavailable: %w", err))
		} else {
			stats.WarmingTasks = *warmingStats
		}
		mu.Unlock()
	}()

	// Get proxy stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		proxyStats, err := s.GetProxyStats(ctx)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("proxy stats unavailable: %w", err))
		} else {
			stats.ProxyStats = *proxyStats
		}
		mu.Unlock()
	}()

	// Get SMS stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		smsStats, err := s.GetSMSStats(ctx)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("SMS stats unavailable: %w", err))
		} else {
			stats.SMSStats = *smsStats
		}
		mu.Unlock()
	}()

	wg.Wait()

	// Calculate overall success rate
	if stats.TotalAccounts > 0 {
		readyAccounts := stats.AccountsByStatus["ready"]
		stats.SuccessRate = float64(readyAccounts) / float64(stats.TotalAccounts)
	}

	// Calculate last 24 hours created from platform stats
	var total24h int64
	for _, platform := range platforms {
		if accountStats, err := s.GetAccountStats(ctx, platform); err == nil {
			total24h += accountStats.Last24Hours
		}
	}
	stats.Last24HoursCreated = total24h

	// Cache the result before checking for errors
	if s.cache != nil && stats != nil {
		_ = s.cache.SetOverallStats(ctx, stats) // ignore cache errors
	}

	// Если есть критические ошибки, возвращаем их
	if len(errors) > 0 {
		// Проверяем, есть ли критические ошибки
		criticalErrors := make([]error, 0)
		for _, err := range errors {
			if err != nil && (fmt.Sprintf("%v", err) == "failed to get vk stats" || fmt.Sprintf("%v", err) == "failed to get telegram stats") {
				criticalErrors = append(criticalErrors, err)
			}
		}
		if len(criticalErrors) > 0 {
			// Return cached data if available, otherwise error
			if s.cache != nil {
				if cachedStats, cacheErr := s.cache.GetOverallStats(ctx); cacheErr == nil {
					return cachedStats, fmt.Errorf("critical services unavailable, returning cached data: %v", criticalErrors)
				}
			}
			return stats, fmt.Errorf("critical services unavailable: %v", criticalErrors)
		}
	}

	return stats, nil
}

func (s *statsService) GetDetailedStats(ctx context.Context, platform string) (*DetailedStats, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Try using analytics service first
	if s.analyticsClient != nil {
		req := &analyticspb.PlatformRequest{
			Platform: platform,
		}

		platformData, err := s.analyticsClient.GetPlatformAnalytics(ctx, req)
		if err == nil {
			detailedStats := &DetailedStats{
				Platform:           platform,
				StatusDistribution: platformData.ByStatus,
				SuccessRate:        platformData.SuccessRate,
				AvgWarmingDuration: platformData.AvgWarmingDays,
			}

			// Get recommendations as top errors/insights
			if len(platformData.Recommendations) > 0 {
				detailedStats.TopErrors = platformData.Recommendations
			} else {
				detailedStats.TopErrors = []string{
					"SMS verification timeout",
					"Captcha failed",
					"Proxy connection error",
				}
			}

			// Get warming scenario recommendations
			warmingResp, err := s.analyticsClient.GetWarmingScenarioRecommendations(ctx, req)
			if err == nil && warmingResp != nil {
				if warmingResp.RecommendedDays > 0 {
					detailedStats.AvgWarmingDuration = float64(warmingResp.RecommendedDays)
				}
			}

			// Get error pattern analysis
			analysisReq := &analyticspb.AnalysisRequest{
				Days: 7,
			}
			errorPatterns, err := s.analyticsClient.GetErrorPatternAnalysis(ctx, analysisReq)
			if err == nil && len(errorPatterns.Clusters) > 0 {
				detailedStats.TopErrors = make([]string, 0, len(errorPatterns.Clusters))
				for _, cluster := range errorPatterns.Clusters {
					for _, p := range cluster.AffectedPlatforms {
						if p == platform {
							detailedStats.TopErrors = append(detailedStats.TopErrors, cluster.Pattern)
							break
						}
					}
				}
			}

			// For now, use static 7-day activity data
			detailedStats.Last7DaysActivity = map[string]int64{
				"2025-12-05": 15,
				"2025-12-06": 20,
				"2025-12-07": 18,
				"2025-12-08": 22,
				"2025-12-09": 25,
				"2025-12-10": 30,
				"2025-12-11": 35,
			}

			return detailedStats, nil
		}
	}

	// Fallback to original implementation
	// Сначала получаем базовую статистику
	accountStats, err := s.GetAccountStats(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to get basic stats for %s: %w", platform, err)
	}

	detailedStats := &DetailedStats{
		Platform:           platform,
		StatusDistribution: accountStats.ByStatus,
		SuccessRate:        accountStats.SuccessRate,
	}

	// Получаем статистику warming для этой платформы
	warmingStats, err := s.GetWarmingStats(ctx, platform)
	if err == nil {
		detailedStats.AvgWarmingDuration = warmingStats.AvgDuration
	} else {
		detailedStats.AvgWarmingDuration = 21.5 // default value
	}

	// TODO: Реализовать получение топ ошибок через gRPC
	// Пока используем заглушку
	detailedStats.TopErrors = []string{
		"SMS verification timeout",
		"Captcha failed",
		"Proxy connection error",
		"Account locked",
		"Invalid credentials",
	}

	// TODO: Реализовать получение активности за 7 дней через gRPC
	// Пока используем заглушку
	detailedStats.Last7DaysActivity = map[string]int64{
		"2025-12-05": 15,
		"2025-12-06": 20,
		"2025-12-07": 18,
		"2025-12-08": 22,
		"2025-12-09": 25,
		"2025-12-10": 30,
		"2025-12-11": 35,
	}

	return detailedStats, nil
}
