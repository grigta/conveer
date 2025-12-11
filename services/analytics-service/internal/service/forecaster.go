package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/grigta/conveer/pkg/cache"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/analytics-service/internal/models"
	"github.com/grigta/conveer/services/analytics-service/internal/repository"

	"gonum.org/v1/gonum/stat"
)

// Forecaster сервис для прогнозирования
type Forecaster struct {
	metricsRepo  *repository.MetricsRepository
	forecastRepo *repository.ForecastRepository
	cache        *cache.RedisClient
	logger       *logger.Logger
	interval     time.Duration
}

// NewForecaster создает новый сервис прогнозирования
func NewForecaster(
	metricsRepo *repository.MetricsRepository,
	forecastRepo *repository.ForecastRepository,
	cache *cache.RedisClient,
	logger *logger.Logger,
) *Forecaster {
	return &Forecaster{
		metricsRepo:  metricsRepo,
		forecastRepo: forecastRepo,
		cache:        cache,
		logger:       logger,
		interval:     1 * time.Hour,
	}
}

// Run запускает фоновый воркер прогнозирования
func (f *Forecaster) Run(ctx context.Context) {
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	// Первоначальное прогнозирование
	if err := f.generateForecasts(ctx); err != nil {
		f.logger.WithError(err).Error("Failed initial forecast generation")
	}

	for {
		select {
		case <-ticker.C:
			RecordWorkerRun("forecaster")
			if err := f.generateForecasts(ctx); err != nil {
				f.logger.WithError(err).Error("Failed to generate forecasts")
				RecordWorkerError("forecaster")
			}
		case <-ctx.Done():
			f.logger.Info("Stopping forecaster")
			return
		}
	}
}

// generateForecasts генерирует все типы прогнозов
func (f *Forecaster) generateForecasts(ctx context.Context) error {
	f.logger.Debug("Starting forecast generation")

	// Прогноз расходов
	for _, period := range []string{"7d", "30d"} {
		start := time.Now()
		if err := f.forecastExpenses(ctx, period); err != nil {
			f.logger.WithError(err).WithField("period", period).Error("Failed to forecast expenses")
		} else {
			forecastGenerationDuration.WithLabelValues("expense").Observe(time.Since(start).Seconds())
			forecastsGenerated.WithLabelValues("expense").Inc()
		}
	}

	// Прогноз готовности аккаунтов
	start := time.Now()
	if err := f.forecastAccountReadiness(ctx); err != nil {
		f.logger.WithError(err).Error("Failed to forecast account readiness")
	} else {
		forecastGenerationDuration.WithLabelValues("readiness").Observe(time.Since(start).Seconds())
		forecastsGenerated.WithLabelValues("readiness").Inc()
	}

	// Оптимальное время регистрации
	start = time.Now()
	if err := f.analyzeOptimalRegistrationTime(ctx); err != nil {
		f.logger.WithError(err).Error("Failed to analyze optimal registration time")
	} else {
		forecastGenerationDuration.WithLabelValues("optimal_time").Observe(time.Since(start).Seconds())
		forecastsGenerated.WithLabelValues("optimal_time").Inc()
	}

	f.logger.Info("Forecast generation completed")
	return nil
}

// forecastExpenses прогнозирует расходы
func (f *Forecaster) forecastExpenses(ctx context.Context, period string) error {
	// Получаем исторические данные за последние 30 дней
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour)

	metrics, err := f.metricsRepo.GetByTimeRange(ctx, "all", startTime, endTime)
	if err != nil {
		return err
	}

	if len(metrics) < 3 {
		f.logger.Warn("Insufficient data for expense forecasting")
		return nil
	}

	// Подготовка данных для регрессии
	var xData, yData []float64
	for _, m := range metrics {
		xData = append(xData, float64(m.Timestamp.Unix()))
		yData = append(yData, m.TotalSpent)
	}

	// Линейная регрессия
	alpha, beta := stat.LinearRegression(xData, yData, nil, false)

	// Прогноз на период
	days := 7
	if period == "30d" {
		days = 30
	}

	futureTimestamp := float64(time.Now().Add(time.Duration(days) * 24 * time.Hour).Unix())
	predictedCost := alpha + beta*futureTimestamp

	// Расчет доверительного интервала (95%)
	variance := stat.Variance(yData, nil)
	stdError := math.Sqrt(variance / float64(len(yData)))
	margin := 1.96 * stdError // 95% доверительный интервал

	upperBound := predictedCost + margin
	lowerBound := math.Max(0, predictedCost-margin)

	// Расчет R² для оценки точности
	var ssTot, ssRes float64
	yMean := stat.Mean(yData, nil)
	for i, y := range yData {
		predicted := alpha + beta*xData[i]
		ssTot += math.Pow(y-yMean, 2)
		ssRes += math.Pow(y-predicted, 2)
	}
	r2 := 1 - (ssRes / ssTot)

	// Разбивка по типам расходов
	breakdown := make(map[string]float64)
	if len(metrics) > 0 {
		lastMetric := metrics[len(metrics)-1]
		if lastMetric.TotalSpent > 0 {
			smsRatio := lastMetric.SMSSpent / lastMetric.TotalSpent
			proxyRatio := lastMetric.ProxySpent / lastMetric.TotalSpent
			breakdown["sms"] = predictedCost * smsRatio
			breakdown["proxy"] = predictedCost * proxyRatio
		}
	}

	// Создаем прогноз
	forecast := &models.ForecastResult{
		Type:        "expense",
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(1 * time.Hour),
		ExpenseForecast: &models.ExpenseForecast{
			Period:        period,
			PredictedCost: predictedCost,
			UpperBound:    upperBound,
			LowerBound:    lowerBound,
			Breakdown:     breakdown,
		},
		Confidence: r2,
		Model:      "linear_regression",
	}

	// Сохраняем в БД
	if err := f.forecastRepo.Save(ctx, forecast); err != nil {
		return err
	}

	// Кэшируем
	cacheKey := fmt.Sprintf("forecast:expense:%s", period)
	data, _ := json.Marshal(forecast)
	f.cache.Set(ctx, cacheKey, string(data), 1*time.Hour)

	// Обновляем метрику точности
	forecastAccuracy.WithLabelValues("expense").Set(r2)

	f.logger.WithFields(map[string]interface{}{
		"period":    period,
		"predicted": predictedCost,
		"r2":        r2,
	}).Debug("Expense forecast generated")

	return nil
}

// forecastAccountReadiness прогнозирует готовность аккаунтов
func (f *Forecaster) forecastAccountReadiness(ctx context.Context) error {
	// Получаем данные о прогреве за последние 30 дней
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour)

	metrics, err := f.metricsRepo.GetByTimeRange(ctx, "all", startTime, endTime)
	if err != nil {
		return err
	}

	if len(metrics) < 3 {
		return nil
	}

	// Рассчитываем среднюю скорость прогрева
	var completionRates []float64
	for i := 1; i < len(metrics); i++ {
		if metrics[i].WarmingActive > 0 {
			rate := float64(metrics[i].WarmingCompleted-metrics[i-1].WarmingCompleted) / float64(metrics[i].WarmingActive)
			completionRates = append(completionRates, rate)
		}
	}

	if len(completionRates) == 0 {
		return nil
	}

	// Экспоненциальное сглаживание для более актуальных данных
	alpha := 0.3 // Параметр сглаживания
	ema := completionRates[0]
	for i := 1; i < len(completionRates); i++ {
		ema = alpha*completionRates[i] + (1-alpha)*ema
	}

	// Прогнозируем для активных задач
	if len(metrics) > 0 {
		lastMetric := metrics[len(metrics)-1]
		if lastMetric.WarmingActive > 0 {
			// Примерный прогноз на основе средней скорости
			estimatedDays := int(float64(lastMetric.WarmingActive) / (ema * 24))
			completionDate := time.Now().Add(time.Duration(estimatedDays) * 24 * time.Hour)

			forecast := &models.ForecastResult{
				Type:        "readiness",
				Platform:    "all",
				GeneratedAt: time.Now(),
				ValidUntil:  time.Now().Add(1 * time.Hour),
				ReadinessForecast: &models.ReadinessForecast{
					AccountID:       "all",
					EstimatedDays:   estimatedDays,
					CompletionDate:  completionDate,
					CurrentProgress: float64(lastMetric.WarmingCompleted) / float64(lastMetric.WarmingCompleted+lastMetric.WarmingActive) * 100,
				},
				Confidence: 0.7, // Средняя уверенность для EMA
				Model:      "ema",
			}

			if err := f.forecastRepo.Save(ctx, forecast); err != nil {
				return err
			}

			// Кэшируем
			cacheKey := "forecast:readiness:all"
			data, _ := json.Marshal(forecast)
			f.cache.Set(ctx, cacheKey, string(data), 1*time.Hour)
		}
	}

	return nil
}

// analyzeOptimalRegistrationTime анализирует оптимальное время регистрации
func (f *Forecaster) analyzeOptimalRegistrationTime(ctx context.Context) error {
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		if err := f.analyzeOptimalTimeForPlatform(ctx, platform); err != nil {
			f.logger.WithError(err).WithField("platform", platform).Error("Failed to analyze optimal time")
		}
	}

	return nil
}

// analyzeOptimalTimeForPlatform анализирует оптимальное время для платформы
func (f *Forecaster) analyzeOptimalTimeForPlatform(ctx context.Context, platform string) error {
	// Получаем данные за последние 30 дней
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour)

	metrics, err := f.metricsRepo.GetByTimeRange(ctx, platform, startTime, endTime)
	if err != nil {
		return err
	}

	if len(metrics) < 10 {
		return nil
	}

	// Анализируем успешность по часам
	hourStats := make(map[int]struct {
		success int
		total   int
	})

	// Анализируем успешность по дням недели
	dayStats := make(map[string]struct {
		success int
		total   int
	})

	for _, m := range metrics {
		hour := m.Timestamp.Hour()
		day := m.Timestamp.Weekday().String()

		// Учитываем успешные аккаунты
		successCount := int(m.SuccessRate * float64(m.TotalAccounts) / 100)

		// Обновляем статистику по часам
		stats := hourStats[hour]
		stats.success += successCount
		stats.total += int(m.TotalAccounts)
		hourStats[hour] = stats

		// Обновляем статистику по дням
		dayStatsVal := dayStats[day]
		dayStatsVal.success += successCount
		dayStatsVal.total += int(m.TotalAccounts)
		dayStats[day] = dayStatsVal
	}

	// Определяем лучшие часы
	type hourScore struct {
		hour        int
		successRate float64
	}
	var hourScores []hourScore

	for hour, stats := range hourStats {
		if stats.total > 0 {
			rate := float64(stats.success) / float64(stats.total)
			hourScores = append(hourScores, hourScore{hour, rate})
		}
	}

	sort.Slice(hourScores, func(i, j int) bool {
		return hourScores[i].successRate > hourScores[j].successRate
	})

	var bestHours []int
	for i := 0; i < len(hourScores) && i < 3; i++ {
		bestHours = append(bestHours, hourScores[i].hour)
	}

	// Определяем лучшие дни
	type dayScore struct {
		day         string
		successRate float64
	}
	var dayScores []dayScore

	for day, stats := range dayStats {
		if stats.total > 0 {
			rate := float64(stats.success) / float64(stats.total)
			dayScores = append(dayScores, dayScore{day, rate})
		}
	}

	sort.Slice(dayScores, func(i, j int) bool {
		return dayScores[i].successRate > dayScores[j].successRate
	})

	var bestDays []string
	for i := 0; i < len(dayScores) && i < 3; i++ {
		bestDays = append(bestDays, dayScores[i].day)
	}

	// Общая статистика
	var totalSuccess, totalAccounts int64
	for _, m := range metrics {
		totalAccounts += m.TotalAccounts
		totalSuccess += int64(m.SuccessRate * float64(m.TotalAccounts) / 100)
	}

	overallSuccessRate := float64(totalSuccess) / float64(totalAccounts) * 100

	// Создаем прогноз
	forecast := &models.ForecastResult{
		Type:        "optimal_time",
		Platform:    platform,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(24 * time.Hour),
		OptimalTimeForecast: &models.OptimalTimeForecast{
			BestHours:   bestHours,
			BestDays:    bestDays,
			SuccessRate: overallSuccessRate,
			SampleSize:  totalAccounts,
		},
		Confidence: 0.8, // Высокая уверенность для статистического анализа
		Model:      "statistical_analysis",
	}

	if err := f.forecastRepo.Save(ctx, forecast); err != nil {
		return err
	}

	// Кэшируем
	cacheKey := fmt.Sprintf("forecast:optimal_time:%s", platform)
	data, _ := json.Marshal(forecast)
	f.cache.Set(ctx, cacheKey, string(data), 24*time.Hour)

	f.logger.WithFields(map[string]interface{}{
		"platform":    platform,
		"best_hours":  bestHours,
		"best_days":   bestDays,
		"success_rate": overallSuccessRate,
	}).Debug("Optimal time forecast generated")

	return nil
}

// GetExpenseForecast получает прогноз расходов
func (f *Forecaster) GetExpenseForecast(ctx context.Context, period string) (*models.ForecastResult, error) {
	// Проверяем кэш
	cacheKey := fmt.Sprintf("forecast:expense:%s", period)
	if cached, err := f.cache.Get(ctx, cacheKey); err == nil && cached != "" {
		cacheHits.Inc()
		var forecast models.ForecastResult
		if err := json.Unmarshal([]byte(cached), &forecast); err == nil {
			return &forecast, nil
		}
	}
	cacheMisses.Inc()

	// Получаем из БД
	forecast, err := f.forecastRepo.GetExpenseForecast(ctx, period)
	if err != nil {
		// Генерируем новый прогноз
		if err := f.forecastExpenses(ctx, period); err != nil {
			return nil, err
		}
		return f.forecastRepo.GetExpenseForecast(ctx, period)
	}

	return forecast, nil
}

// GetReadinessForecast получает прогноз готовности
func (f *Forecaster) GetReadinessForecast(ctx context.Context, accountID string) (*models.ForecastResult, error) {
	// Проверяем кэш
	cacheKey := fmt.Sprintf("forecast:readiness:%s", accountID)
	if cached, err := f.cache.Get(ctx, cacheKey); err == nil && cached != "" {
		cacheHits.Inc()
		var forecast models.ForecastResult
		if err := json.Unmarshal([]byte(cached), &forecast); err == nil {
			return &forecast, nil
		}
	}
	cacheMisses.Inc()

	// Получаем из БД
	return f.forecastRepo.GetReadinessForecast(ctx, accountID)
}

// GetOptimalTimeForecast получает прогноз оптимального времени
func (f *Forecaster) GetOptimalTimeForecast(ctx context.Context, platform string) (*models.ForecastResult, error) {
	// Проверяем кэш
	cacheKey := fmt.Sprintf("forecast:optimal_time:%s", platform)
	if cached, err := f.cache.Get(ctx, cacheKey); err == nil && cached != "" {
		cacheHits.Inc()
		var forecast models.ForecastResult
		if err := json.Unmarshal([]byte(cached), &forecast); err == nil {
			return &forecast, nil
		}
	}
	cacheMisses.Inc()

	// Получаем из БД
	return f.forecastRepo.GetOptimalTimeForecast(ctx, platform)
}
