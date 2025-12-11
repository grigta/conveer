package service

import (
	"context"
	"fmt"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/analytics-service/internal/models"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusClient клиент для работы с Prometheus
type PrometheusClient struct {
	api    v1.API
	logger *logger.Logger
}

// NewPrometheusClient создает новый клиент Prometheus
func NewPrometheusClient(url string, logger *logger.Logger) (*PrometheusClient, error) {
	client, err := api.NewClient(api.Config{
		Address: url,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	return &PrometheusClient{
		api:    v1.NewAPI(client),
		logger: logger,
	}, nil
}

// GetAccountStats получает статистику аккаунтов
func (c *PrometheusClient) GetAccountStats(ctx context.Context, platform string) (*models.AccountMetrics, error) {
	metrics := &models.AccountMetrics{
		ByStatus: make(map[string]int64),
	}

	// Общее количество аккаунтов
	query := fmt.Sprintf(`sum(%s_accounts_total)`, platform)
	result, err := c.queryInstant(ctx, query)
	if err == nil && result != nil {
		metrics.Total = int64(result.Value)
	}

	// Аккаунты по статусам
	statusQuery := fmt.Sprintf(`sum by (status) (%s_accounts_total)`, platform)
	statusResults, err := c.queryInstantVector(ctx, statusQuery)
	if err == nil {
		for _, sample := range statusResults {
			if status, ok := sample.Metric["status"]; ok {
				metrics.ByStatus[string(status)] = int64(sample.Value)
			}
		}
	}

	// Процент банов
	banRateQuery := fmt.Sprintf(`
		rate(%s_accounts_total{status="banned"}[1h]) /
		rate(%s_accounts_total[1h])`, platform, platform)
	banRate, err := c.queryInstant(ctx, banRateQuery)
	if err == nil && banRate != nil {
		metrics.BanRate = float64(banRate.Value) * 100
	}

	// Процент успеха
	successRateQuery := fmt.Sprintf(`
		rate(%s_accounts_total{status="ready"}[1h]) /
		rate(%s_accounts_total[1h])`, platform, platform)
	successRate, err := c.queryInstant(ctx, successRateQuery)
	if err == nil && successRate != nil {
		metrics.SuccessRate = float64(successRate.Value) * 100
	}

	// Созданные сегодня
	createdTodayQuery := fmt.Sprintf(`increase(%s_accounts_total[24h])`, platform)
	createdToday, err := c.queryInstant(ctx, createdTodayQuery)
	if err == nil && createdToday != nil {
		metrics.CreatedToday = int64(createdToday.Value)
	}

	// Забаненные сегодня
	bannedTodayQuery := fmt.Sprintf(`increase(%s_accounts_total{status="banned"}[24h])`, platform)
	bannedToday, err := c.queryInstant(ctx, bannedTodayQuery)
	if err == nil && bannedToday != nil {
		metrics.BannedToday = int64(bannedToday.Value)
	}

	return metrics, nil
}

// GetBanRate получает процент банов за период
func (c *PrometheusClient) GetBanRate(ctx context.Context, platform string, window string) (float64, error) {
	query := fmt.Sprintf(`
		rate(%s_accounts_total{status="banned"}[%s]) /
		rate(%s_accounts_total[%s])`, platform, window, platform, window)

	result, err := c.queryInstant(ctx, query)
	if err != nil {
		return 0, err
	}

	if result != nil {
		return float64(result.Value) * 100, nil
	}

	return 0, nil
}

// GetExpenseMetrics получает метрики расходов
func (c *PrometheusClient) GetExpenseMetrics(ctx context.Context) (*models.ExpenseMetrics, error) {
	metrics := &models.ExpenseMetrics{}

	// Расходы на SMS
	smsQuery := `sum(sms_purchase_price)`
	smsResult, err := c.queryInstant(ctx, smsQuery)
	if err == nil && smsResult != nil {
		metrics.TotalSMS = float64(smsResult.Value)
	}

	// Расходы на прокси
	proxyQuery := `sum(proxy_cost)`
	proxyResult, err := c.queryInstant(ctx, proxyQuery)
	if err == nil && proxyResult != nil {
		metrics.TotalProxy = float64(proxyResult.Value)
	}

	// Общие расходы
	metrics.TotalSpent = metrics.TotalSMS + metrics.TotalProxy

	// Средние расходы за день
	dailyQuery := `sum(rate(sms_purchase_price[24h])) + sum(rate(proxy_cost[24h]))`
	dailyResult, err := c.queryInstant(ctx, dailyQuery)
	if err == nil && dailyResult != nil {
		metrics.DailyAvg = float64(dailyResult.Value) * 86400 // Конвертация rate в день
	}

	// Расходы за неделю
	weeklyQuery := `sum(increase(sms_purchase_price[7d])) + sum(increase(proxy_cost[7d]))`
	weeklyResult, err := c.queryInstant(ctx, weeklyQuery)
	if err == nil && weeklyResult != nil {
		metrics.WeeklyTotal = float64(weeklyResult.Value)
	}

	// Расходы за месяц
	monthlyQuery := `sum(increase(sms_purchase_price[30d])) + sum(increase(proxy_cost[30d]))`
	monthlyResult, err := c.queryInstant(ctx, monthlyQuery)
	if err == nil && monthlyResult != nil {
		metrics.MonthlyTotal = float64(monthlyResult.Value)
	}

	return metrics, nil
}

// GetWarmingMetrics получает метрики прогрева
func (c *PrometheusClient) GetWarmingMetrics(ctx context.Context, platform string) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Активные задачи прогрева
	activeQuery := fmt.Sprintf(`warming_tasks_active{platform="%s"}`, platform)
	if platform == "all" {
		activeQuery = `sum(warming_tasks_active)`
	}
	activeResult, err := c.queryInstant(ctx, activeQuery)
	if err == nil && activeResult != nil {
		metrics["active_tasks"] = int64(activeResult.Value)
	}

	// Завершенные задачи
	completedQuery := fmt.Sprintf(`warming_tasks_completed_total{platform="%s"}`, platform)
	if platform == "all" {
		completedQuery = `sum(warming_tasks_completed_total)`
	}
	completedResult, err := c.queryInstant(ctx, completedQuery)
	if err == nil && completedResult != nil {
		metrics["completed_tasks"] = int64(completedResult.Value)
	}

	// Средняя длительность прогрева
	avgDurationQuery := fmt.Sprintf(`avg(warming_duration_days{platform="%s"})`, platform)
	if platform == "all" {
		avgDurationQuery = `avg(warming_duration_days)`
	}
	avgDurationResult, err := c.queryInstant(ctx, avgDurationQuery)
	if err == nil && avgDurationResult != nil {
		metrics["avg_duration_days"] = float64(avgDurationResult.Value)
	}

	// Готовые аккаунты
	readyQuery := fmt.Sprintf(`warming_accounts_ready_total{platform="%s"}`, platform)
	if platform == "all" {
		readyQuery = `sum(warming_accounts_ready_total)`
	}
	readyResult, err := c.queryInstant(ctx, readyQuery)
	if err == nil && readyResult != nil {
		metrics["ready_accounts"] = int64(readyResult.Value)
	}

	return metrics, nil
}

// GetProxyMetrics получает метрики прокси
func (c *PrometheusClient) GetProxyMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Активные прокси
	activeQuery := `sum(proxy_active_total)`
	activeResult, err := c.queryInstant(ctx, activeQuery)
	if err == nil && activeResult != nil {
		metrics["active_proxies"] = int64(activeResult.Value)
	}

	// Забаненные прокси
	bannedQuery := `sum(proxy_banned_total)`
	bannedResult, err := c.queryInstant(ctx, bannedQuery)
	if err == nil && bannedResult != nil {
		metrics["banned_proxies"] = int64(bannedResult.Value)
	}

	// Средняя задержка
	latencyQuery := `avg(proxy_response_time_seconds)`
	latencyResult, err := c.queryInstant(ctx, latencyQuery)
	if err == nil && latencyResult != nil {
		metrics["avg_latency"] = float64(latencyResult.Value)
	}

	// Прокси по провайдерам
	providerQuery := `sum by (provider) (proxy_active_total)`
	providerResults, err := c.queryInstantVector(ctx, providerQuery)
	if err == nil {
		providers := make(map[string]int64)
		for _, sample := range providerResults {
			if provider, ok := sample.Metric["provider"]; ok {
				providers[string(provider)] = int64(sample.Value)
			}
		}
		metrics["by_provider"] = providers
	}

	return metrics, nil
}

// GetSMSMetrics получает метрики SMS
func (c *PrometheusClient) GetSMSMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Баланс SMS
	balanceQuery := `sms_balance`
	balanceResult, err := c.queryInstant(ctx, balanceQuery)
	if err == nil && balanceResult != nil {
		metrics["balance"] = float64(balanceResult.Value)
	}

	// Использованные SMS за сегодня
	usedTodayQuery := `increase(sms_used_total[24h])`
	usedTodayResult, err := c.queryInstant(ctx, usedTodayQuery)
	if err == nil && usedTodayResult != nil {
		metrics["used_today"] = int64(usedTodayResult.Value)
	}

	// Средняя цена SMS
	avgPriceQuery := `avg(sms_purchase_price)`
	avgPriceResult, err := c.queryInstant(ctx, avgPriceQuery)
	if err == nil && avgPriceResult != nil {
		metrics["avg_price"] = float64(avgPriceResult.Value)
	}

	return metrics, nil
}

// GetErrorMetrics получает метрики ошибок
func (c *PrometheusClient) GetErrorMetrics(ctx context.Context, platform string) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Общее количество ошибок
	errorQuery := fmt.Sprintf(`sum(rate(%s_errors_total[5m]))`, platform)
	if platform == "all" {
		errorQuery = `sum(rate({__name__=~".*_errors_total"}[5m]))`
	}
	errorResult, err := c.queryInstant(ctx, errorQuery)
	if err == nil && errorResult != nil {
		metrics["error_rate"] = float64(errorResult.Value)
	}

	// Ошибки по типам
	typeQuery := fmt.Sprintf(`sum by (type) (rate(%s_errors_total[5m]))`, platform)
	if platform == "all" {
		typeQuery = `sum by (type) (rate({__name__=~".*_errors_total"}[5m]))`
	}
	typeResults, err := c.queryInstantVector(ctx, typeQuery)
	if err == nil {
		errors := make(map[string]float64)
		for _, sample := range typeResults {
			if errType, ok := sample.Metric["type"]; ok {
				errors[string(errType)] = float64(sample.Value)
			}
		}
		metrics["by_type"] = errors
	}

	return metrics, nil
}

// queryInstant выполняет instant запрос к Prometheus
func (c *PrometheusClient) queryInstant(ctx context.Context, query string) (*model.Sample, error) {
	result, warnings, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		c.logger.WithError(err).WithField("query", query).Error("Prometheus query failed")
		return nil, err
	}

	if len(warnings) > 0 {
		c.logger.WithField("warnings", warnings).Warn("Prometheus query warnings")
	}

	switch v := result.(type) {
	case *model.Scalar:
		return (*model.Sample)(v), nil
	case model.Vector:
		if len(v) > 0 {
			return (*model.Sample)(&v[0]), nil
		}
	}

	return nil, nil
}

// queryInstantVector выполняет instant vector запрос к Prometheus
func (c *PrometheusClient) queryInstantVector(ctx context.Context, query string) (model.Vector, error) {
	result, warnings, err := c.api.Query(ctx, query, time.Now())
	if err != nil {
		c.logger.WithError(err).WithField("query", query).Error("Prometheus query failed")
		return nil, err
	}

	if len(warnings) > 0 {
		c.logger.WithField("warnings", warnings).Warn("Prometheus query warnings")
	}

	switch v := result.(type) {
	case model.Vector:
		return v, nil
	}

	return nil, fmt.Errorf("unexpected result type for vector query")
}

// QueryRange выполняет range запрос к Prometheus
func (c *PrometheusClient) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (model.Matrix, error) {
	result, warnings, err := c.api.QueryRange(ctx, query, v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	})
	if err != nil {
		c.logger.WithError(err).WithField("query", query).Error("Prometheus range query failed")
		return nil, err
	}

	if len(warnings) > 0 {
		c.logger.WithField("warnings", warnings).Warn("Prometheus range query warnings")
	}

	switch v := result.(type) {
	case model.Matrix:
		return v, nil
	}

	return nil, fmt.Errorf("unexpected result type for range query")
}
