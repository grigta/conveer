package service

import (
	"github.com/conveer/conveer/services/analytics-service/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Метрики агрегации
	aggregationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "analytics_aggregation_duration_seconds",
		Help: "Duration of metrics aggregation in seconds",
		Buckets: prometheus.DefBuckets,
	})

	aggregationSuccesses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_aggregation_successes_total",
		Help: "Total number of successful aggregations",
	})

	aggregationErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_aggregation_errors_total",
		Help: "Total number of aggregation errors",
	})

	// Метрики прогнозирования
	forecastGenerationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analytics_forecast_generation_duration_seconds",
		Help: "Duration of forecast generation in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})

	forecastsGenerated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_forecasts_generated_total",
		Help: "Total number of forecasts generated",
	}, []string{"type"})

	forecastAccuracy = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_forecast_accuracy",
		Help: "Accuracy of forecasts (R2 score)",
	}, []string{"type"})

	// Метрики рекомендаций
	recommendationsGenerated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_recommendations_generated_total",
		Help: "Total number of recommendations generated",
	}, []string{"type"})

	recommendationGenerationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analytics_recommendation_generation_duration_seconds",
		Help: "Duration of recommendation generation in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})

	// Метрики алертов
	alertsFired = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_alerts_fired_total",
		Help: "Total number of alerts fired",
	}, []string{"severity", "type", "platform"})

	alertsAcknowledged = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_alerts_acknowledged_total",
		Help: "Total number of alerts acknowledged",
	})

	activeAlerts = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_active_alerts",
		Help: "Number of currently active alerts",
	}, []string{"severity"})

	alertCheckDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "analytics_alert_check_duration_seconds",
		Help: "Duration of alert rule checking",
		Buckets: prometheus.DefBuckets,
	})

	// Метрики gRPC
	grpcRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analytics_grpc_request_duration_seconds",
		Help: "Duration of gRPC requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"})

	grpcRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_grpc_requests_total",
		Help: "Total number of gRPC requests",
	}, []string{"method", "status"})

	// Метрики HTTP
	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analytics_http_request_duration_seconds",
		Help: "Duration of HTTP requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "endpoint"})

	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "endpoint", "status"})

	// Метрики кэша
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_cache_hits_total",
		Help: "Total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_cache_misses_total",
		Help: "Total number of cache misses",
	})

	// Метрики MongoDB
	mongoOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analytics_mongo_operation_duration_seconds",
		Help: "Duration of MongoDB operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation", "collection"})

	mongoOperationErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_mongo_operation_errors_total",
		Help: "Total number of MongoDB operation errors",
	}, []string{"operation", "collection"})

	// Бизнес-метрики
	totalAccountsMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_total_accounts",
		Help: "Total number of accounts",
	}, []string{"platform", "status"})

	banRateMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_ban_rate",
		Help: "Current ban rate percentage",
	}, []string{"platform"})

	successRateMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_success_rate",
		Help: "Current success rate percentage",
	}, []string{"platform"})

	totalSpentMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_total_spent",
		Help: "Total amount spent",
	}, []string{"type"}) // type: sms, proxy, total

	avgWarmingDaysMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_avg_warming_days",
		Help: "Average warming duration in days",
	}, []string{"platform"})

	errorRateMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_error_rate",
		Help: "Current error rate percentage",
	}, []string{"platform"})

	// Метрики производительности воркеров
	workerLastRun = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "analytics_worker_last_run_timestamp",
		Help: "Unix timestamp of last worker run",
	}, []string{"worker"})

	workerRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_worker_runs_total",
		Help: "Total number of worker runs",
	}, []string{"worker"})

	workerErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "analytics_worker_errors_total",
		Help: "Total number of worker errors",
	}, []string{"worker"})
)

// UpdateBusinessMetrics обновляет бизнес-метрики на основе агрегированных данных
func UpdateBusinessMetrics(metrics *models.AggregatedMetrics) {
	if metrics == nil {
		return
	}

	platform := metrics.Platform
	if platform == "" {
		platform = "unknown"
	}

	// Обновляем метрики аккаунтов
	for status, count := range metrics.AccountsByStatus {
		totalAccountsMetric.WithLabelValues(platform, status).Set(float64(count))
	}

	// Обновляем процентные метрики
	banRateMetric.WithLabelValues(platform).Set(metrics.BanRate)
	successRateMetric.WithLabelValues(platform).Set(metrics.SuccessRate)
	errorRateMetric.WithLabelValues(platform).Set(metrics.ErrorRate)

	// Обновляем метрики расходов
	if platform == "all" {
		totalSpentMetric.WithLabelValues("sms").Set(metrics.SMSSpent)
		totalSpentMetric.WithLabelValues("proxy").Set(metrics.ProxySpent)
		totalSpentMetric.WithLabelValues("total").Set(metrics.TotalSpent)
	}

	// Обновляем среднее время прогрева
	avgWarmingDaysMetric.WithLabelValues(platform).Set(metrics.AvgWarmingDays)
}

// RecordWorkerRun записывает выполнение воркера
func RecordWorkerRun(workerName string) {
	workerLastRun.WithLabelValues(workerName).SetToCurrentTime()
	workerRunsTotal.WithLabelValues(workerName).Inc()
}

// RecordWorkerError записывает ошибку воркера
func RecordWorkerError(workerName string) {
	workerErrors.WithLabelValues(workerName).Inc()
}

// RecordGRPCRequest записывает gRPC запрос
func RecordGRPCRequest(method string, duration float64) {
	grpcRequestDuration.WithLabelValues(method).Observe(duration)
	grpcRequestsTotal.WithLabelValues(method, "success").Inc()
}

// RecordHTTPRequest записывает HTTP запрос
func RecordHTTPRequest(method, endpoint string, duration float64, statusCode int) {
	httpRequestDuration.WithLabelValues(method, endpoint).Observe(duration)
	status := "success"
	if statusCode >= 400 {
		status = "error"
	}
	httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}