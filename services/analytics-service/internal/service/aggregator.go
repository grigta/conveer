package service

import (
	"context"
	"sync"
	"time"

	"github.com/conveer/conveer/pkg/logger"
	"github.com/conveer/conveer/services/analytics-service/internal/models"
	"github.com/conveer/conveer/services/analytics-service/internal/repository"
	proxypb "github.com/conveer/conveer/services/proxy-service/proto"
	warmingpb "github.com/conveer/conveer/services/warming-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Aggregator сервис для агрегации метрик
type Aggregator struct {
	promClient  *PrometheusClient
	metricsRepo *repository.MetricsRepository
	grpcClients map[string]*grpc.ClientConn
	logger      *logger.Logger
	interval    time.Duration
}

// NewAggregator создает новый сервис агрегации
func NewAggregator(
	promClient *PrometheusClient,
	metricsRepo *repository.MetricsRepository,
	grpcClients map[string]*grpc.ClientConn,
	logger *logger.Logger,
) *Aggregator {
	return &Aggregator{
		promClient:  promClient,
		metricsRepo: metricsRepo,
		grpcClients: grpcClients,
		logger:      logger,
		interval:    5 * time.Minute,
	}
}

// Run запускает фоновый воркер агрегации
func (a *Aggregator) Run(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	// Первоначальная агрегация
	if err := a.aggregateMetrics(ctx); err != nil {
		a.logger.WithError(err).Error("Failed initial aggregation")
	}

	for {
		select {
		case <-ticker.C:
			start := time.Now()
			if err := a.aggregateMetrics(ctx); err != nil {
				a.logger.WithError(err).Error("Failed to aggregate metrics")
				aggregationErrors.Inc()
			} else {
				aggregationDuration.Observe(time.Since(start).Seconds())
				aggregationSuccesses.Inc()
			}
		case <-ctx.Done():
			a.logger.Info("Stopping aggregator")
			return
		}
	}
}

// aggregateMetrics выполняет агрегацию метрик из всех источников
func (a *Aggregator) aggregateMetrics(ctx context.Context) error {
	a.logger.Debug("Starting metrics aggregation")

	// Агрегируем метрики для каждой платформы
	platforms := []string{"vk", "telegram", "mail", "max"}
	var wg sync.WaitGroup
	errCh := make(chan error, len(platforms)+1)

	// Агрегация по платформам
	for _, platform := range platforms {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if err := a.aggregatePlatformMetrics(ctx, p); err != nil {
				errCh <- err
			}
		}(platform)
	}

	// Агрегация общих метрик
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := a.aggregateOverallMetrics(ctx); err != nil {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	// Проверяем на ошибки
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	a.logger.Info("Metrics aggregation completed successfully")
	return nil
}

// aggregatePlatformMetrics агрегирует метрики для конкретной платформы
func (a *Aggregator) aggregatePlatformMetrics(ctx context.Context, platform string) error {
	metrics := &models.AggregatedMetrics{
		Timestamp:        time.Now(),
		Platform:         platform,
		AccountsByStatus: make(map[string]int64),
	}

	// Получаем метрики аккаунтов из Prometheus
	accountMetrics, err := a.promClient.GetAccountStats(ctx, platform)
	if err != nil {
		a.logger.WithError(err).WithField("platform", platform).Error("Failed to get account stats")
	} else {
		metrics.TotalAccounts = accountMetrics.Total
		metrics.AccountsByStatus = accountMetrics.ByStatus
		metrics.BanRate = accountMetrics.BanRate
		metrics.SuccessRate = accountMetrics.SuccessRate
	}

	// Получаем метрики прогрева
	warmingMetrics, err := a.promClient.GetWarmingMetrics(ctx, platform)
	if err != nil {
		a.logger.WithError(err).WithField("platform", platform).Error("Failed to get warming metrics")
	} else {
		if active, ok := warmingMetrics["active_tasks"].(int64); ok {
			metrics.WarmingActive = active
		}
		if completed, ok := warmingMetrics["completed_tasks"].(int64); ok {
			metrics.WarmingCompleted = completed
		}
		if avgDays, ok := warmingMetrics["avg_duration_days"].(float64); ok {
			metrics.AvgWarmingDays = avgDays
		}
	}

	// Получаем метрики расходов
	expenseMetrics, err := a.promClient.GetExpenseMetrics(ctx)
	if err != nil {
		a.logger.WithError(err).Error("Failed to get expense metrics")
	} else {
		// Распределяем расходы пропорционально количеству аккаунтов
		if metrics.TotalAccounts > 0 {
			totalAccounts := a.getTotalAccountsAcrossPlatforms(ctx)
			if totalAccounts > 0 {
				ratio := float64(metrics.TotalAccounts) / float64(totalAccounts)
				metrics.SMSSpent = expenseMetrics.TotalSMS * ratio
				metrics.ProxySpent = expenseMetrics.TotalProxy * ratio
				metrics.TotalSpent = metrics.SMSSpent + metrics.ProxySpent
			}
		}
	}

	// Получаем метрики прокси
	proxyMetrics, err := a.promClient.GetProxyMetrics(ctx)
	if err != nil {
		a.logger.WithError(err).Error("Failed to get proxy metrics")
	} else {
		if active, ok := proxyMetrics["active_proxies"].(int64); ok {
			metrics.ActiveProxies = active
		}
		if banned, ok := proxyMetrics["banned_proxies"].(int64); ok {
			metrics.BannedProxies = banned
		}
	}

	// Получаем статистику прокси-провайдеров из proxy-service
	if proxyClient := a.grpcClients["proxy"]; proxyClient != nil {
		client := proxypb.NewProxyServiceClient(proxyClient)
		resp, err := client.GetProviderStatistics(ctx, &proxypb.GetProviderStatisticsRequest{
			Days: 1, // За последний день для текущих метрик
		})

		if err == nil && resp != nil && len(resp.ProviderStats) > 0 {
			// Конвертируем в формат для сохранения
			metrics.ProxyProviderStats = make(map[string]*models.ProxyProviderStat)
			for _, stat := range resp.ProviderStats {
				metrics.ProxyProviderStats[stat.Provider] = &models.ProxyProviderStat{
					ActiveProxies:  stat.ActiveProxies,
					BannedProxies:  stat.BannedProxies,
					SuccessRate:    stat.SuccessRate,
					BanRate:        stat.BanRate,
					AvgLatency:     stat.AvgLatency,
					CostPerAccount: stat.CostPerProxy,
				}
			}
		} else {
			if err != nil && status.Code(err) != codes.Unimplemented {
				a.logger.WithError(err).WithField("platform", platform).Warn("Failed to get provider statistics")
			}
		}
	}

	// Получаем статистику сценариев прогрева из warming-service
	if warmingClient := a.grpcClients["warming"]; warmingClient != nil {
		client := warmingpb.NewWarmingServiceClient(warmingClient)
		resp, err := client.GetScenarioStatistics(ctx, &warmingpb.ScenarioStatisticsRequest{
			Platform: platform,
			Days:     1,
		})

		if err == nil && resp != nil && len(resp.ScenarioStats) > 0 {
			// Конвертируем в формат для сохранения
			metrics.WarmingScenarioStats = make(map[string]*models.WarmingScenarioStat)
			for _, stat := range resp.ScenarioStats {
				metrics.WarmingScenarioStats[stat.ScenarioType] = &models.WarmingScenarioStat{
					SuccessRate:      stat.SuccessRate,
					AvgDurationDays:  stat.AvgDurationDays,
					CompletedTasks:   stat.CompletedTasks,
					FailedTasks:      stat.FailedTasks,
					TotalTasks:       stat.TotalTasks,
				}
			}
		} else {
			if err != nil && status.Code(err) != codes.Unimplemented {
				a.logger.WithError(err).WithField("platform", platform).Warn("Failed to get scenario statistics")
			}
		}
	}

	// Получаем метрики SMS
	smsMetrics, err := a.promClient.GetSMSMetrics(ctx)
	if err != nil {
		a.logger.WithError(err).Error("Failed to get SMS metrics")
	} else {
		if balance, ok := smsMetrics["balance"].(float64); ok {
			metrics.SMSBalance = balance
		}
	}

	// Получаем метрики ошибок
	errorMetrics, err := a.promClient.GetErrorMetrics(ctx, platform)
	if err != nil {
		a.logger.WithError(err).WithField("platform", platform).Error("Failed to get error metrics")
	} else {
		if errorRate, ok := errorMetrics["error_rate"].(float64); ok {
			metrics.ErrorRate = errorRate * 100 // Преобразуем в проценты
			metrics.ErrorCount = int64(errorRate * 300) // Примерная оценка за 5 минут
		}

		// Топ ошибок
		if errors, ok := errorMetrics["by_type"].(map[string]float64); ok {
			topErrors := a.getTopErrors(errors, 5)
			metrics.TopErrors = topErrors
		}
	}

	// Сохраняем метрики в MongoDB
	if err := a.metricsRepo.Save(ctx, metrics); err != nil {
		return err
	}

	a.logger.WithField("platform", platform).Debug("Platform metrics aggregated")
	return nil
}

// aggregateOverallMetrics агрегирует общие метрики по всем платформам
func (a *Aggregator) aggregateOverallMetrics(ctx context.Context) error {
	metrics := &models.AggregatedMetrics{
		Timestamp:        time.Now(),
		Platform:         "all",
		AccountsByStatus: make(map[string]int64),
	}

	// Получаем метрики для всех платформ
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		accountMetrics, err := a.promClient.GetAccountStats(ctx, platform)
		if err != nil {
			continue
		}

		metrics.TotalAccounts += accountMetrics.Total
		for status, count := range accountMetrics.ByStatus {
			metrics.AccountsByStatus[status] += count
		}
	}

	// Рассчитываем общие проценты
	if metrics.TotalAccounts > 0 {
		if banned, ok := metrics.AccountsByStatus["banned"]; ok {
			metrics.BanRate = float64(banned) / float64(metrics.TotalAccounts) * 100
		}
		if ready, ok := metrics.AccountsByStatus["ready"]; ok {
			metrics.SuccessRate = float64(ready) / float64(metrics.TotalAccounts) * 100
		}
	}

	// Получаем общие метрики прогрева
	warmingMetrics, err := a.promClient.GetWarmingMetrics(ctx, "all")
	if err == nil {
		if active, ok := warmingMetrics["active_tasks"].(int64); ok {
			metrics.WarmingActive = active
		}
		if completed, ok := warmingMetrics["completed_tasks"].(int64); ok {
			metrics.WarmingCompleted = completed
		}
		if avgDays, ok := warmingMetrics["avg_duration_days"].(float64); ok {
			metrics.AvgWarmingDays = avgDays
		}
	}

	// Получаем общие метрики расходов
	expenseMetrics, err := a.promClient.GetExpenseMetrics(ctx)
	if err == nil {
		metrics.SMSSpent = expenseMetrics.TotalSMS
		metrics.ProxySpent = expenseMetrics.TotalProxy
		metrics.TotalSpent = expenseMetrics.TotalSpent
	}

	// Получаем метрики прокси
	proxyMetrics, err := a.promClient.GetProxyMetrics(ctx)
	if err == nil {
		if active, ok := proxyMetrics["active_proxies"].(int64); ok {
			metrics.ActiveProxies = active
		}
		if banned, ok := proxyMetrics["banned_proxies"].(int64); ok {
			metrics.BannedProxies = banned
		}
	}

	// Получаем общую статистику прокси-провайдеров
	if proxyClient := a.grpcClients["proxy"]; proxyClient != nil {
		client := proxypb.NewProxyServiceClient(proxyClient)
		resp, err := client.GetProviderStatistics(ctx, &proxypb.GetProviderStatisticsRequest{
			Days: 1,
		})

		if err == nil && resp != nil && len(resp.ProviderStats) > 0 {
			metrics.ProxyProviderStats = make(map[string]*models.ProxyProviderStat)
			for _, stat := range resp.ProviderStats {
				metrics.ProxyProviderStats[stat.Provider] = &models.ProxyProviderStat{
					ActiveProxies:  stat.ActiveProxies,
					BannedProxies:  stat.BannedProxies,
					SuccessRate:    stat.SuccessRate,
					BanRate:        stat.BanRate,
					AvgLatency:     stat.AvgLatency,
					CostPerAccount: stat.CostPerProxy,
				}
			}
		}
	}

	// Получаем общую статистику сценариев для всех платформ
	if warmingClient := a.grpcClients["warming"]; warmingClient != nil {
		for _, p := range platforms {
			client := warmingpb.NewWarmingServiceClient(warmingClient)
			resp, err := client.GetScenarioStatistics(ctx, &warmingpb.ScenarioStatisticsRequest{
				Platform: p,
				Days:     1,
			})

			if err == nil && resp != nil && len(resp.ScenarioStats) > 0 {
				if metrics.WarmingScenarioStats == nil {
					metrics.WarmingScenarioStats = make(map[string]*models.WarmingScenarioStat)
				}
				for _, stat := range resp.ScenarioStats {
					key := p + "_" + stat.ScenarioType
					metrics.WarmingScenarioStats[key] = &models.WarmingScenarioStat{
						SuccessRate:      stat.SuccessRate,
						AvgDurationDays:  stat.AvgDurationDays,
						CompletedTasks:   stat.CompletedTasks,
						FailedTasks:      stat.FailedTasks,
						TotalTasks:       stat.TotalTasks,
					}
				}
			}
		}
	}

	// Получаем метрики SMS
	smsMetrics, err := a.promClient.GetSMSMetrics(ctx)
	if err == nil {
		if balance, ok := smsMetrics["balance"].(float64); ok {
			metrics.SMSBalance = balance
		}
	}

	// Получаем общие метрики ошибок
	errorMetrics, err := a.promClient.GetErrorMetrics(ctx, "all")
	if err == nil {
		if errorRate, ok := errorMetrics["error_rate"].(float64); ok {
			metrics.ErrorRate = errorRate * 100
			metrics.ErrorCount = int64(errorRate * 300 * float64(len(platforms)))
		}

		if errors, ok := errorMetrics["by_type"].(map[string]float64); ok {
			metrics.TopErrors = a.getTopErrors(errors, 5)
		}
	}

	// Сохраняем метрики
	if err := a.metricsRepo.Save(ctx, metrics); err != nil {
		return err
	}

	a.logger.Debug("Overall metrics aggregated")
	return nil
}

// getTotalAccountsAcrossPlatforms получает общее количество аккаунтов
func (a *Aggregator) getTotalAccountsAcrossPlatforms(ctx context.Context) int64 {
	var total int64
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		accountMetrics, err := a.promClient.GetAccountStats(ctx, platform)
		if err == nil {
			total += accountMetrics.Total
		}
	}

	return total
}

// getTopErrors возвращает топ N ошибок
func (a *Aggregator) getTopErrors(errors map[string]float64, limit int) []models.ErrorStat {
	// Конвертируем в слайс для сортировки
	var errorStats []models.ErrorStat
	for errType, rate := range errors {
		errorStats = append(errorStats, models.ErrorStat{
			Type:  errType,
			Count: int64(rate * 300), // Примерная оценка количества за 5 минут
		})
	}

	// Сортировка по убыванию количества
	for i := 0; i < len(errorStats); i++ {
		for j := i + 1; j < len(errorStats); j++ {
			if errorStats[j].Count > errorStats[i].Count {
				errorStats[i], errorStats[j] = errorStats[j], errorStats[i]
			}
		}
	}

	// Возвращаем топ N
	if len(errorStats) > limit {
		return errorStats[:limit]
	}
	return errorStats
}

// ForceAggregate принудительно запускает агрегацию
func (a *Aggregator) ForceAggregate(ctx context.Context) error {
	return a.aggregateMetrics(ctx)
}