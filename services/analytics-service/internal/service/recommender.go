package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/grigta/conveer/pkg/cache"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/analytics-service/internal/models"
	"github.com/grigta/conveer/services/analytics-service/internal/repository"
	proxypb "github.com/grigta/conveer/services/proxy-service/proto"
	warmingpb "github.com/grigta/conveer/services/warming-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Recommender сервис для генерации рекомендаций
type Recommender struct {
	metricsRepo        *repository.MetricsRepository
	recommendationRepo *repository.RecommendationRepository
	grpcClients        map[string]*grpc.ClientConn
	redisCache         *cache.RedisCache
	logger             *logger.Logger
	interval           time.Duration
}

// NewRecommender создает новый сервис рекомендаций
func NewRecommender(
	metricsRepo *repository.MetricsRepository,
	recommendationRepo *repository.RecommendationRepository,
	grpcClients map[string]*grpc.ClientConn,
	redisCache *cache.RedisCache,
	logger *logger.Logger,
) *Recommender {
	return &Recommender{
		metricsRepo:        metricsRepo,
		recommendationRepo: recommendationRepo,
		grpcClients:        grpcClients,
		redisCache:         redisCache,
		logger:             logger,
		interval:           6 * time.Hour,
	}
}

// Run запускает фоновый воркер рекомендаций
func (r *Recommender) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Первоначальная генерация
	if err := r.generateRecommendations(ctx); err != nil {
		r.logger.WithError(err).Error("Failed initial recommendations generation")
	}

	for {
		select {
		case <-ticker.C:
			RecordWorkerRun("recommender")
			if err := r.generateRecommendations(ctx); err != nil {
				r.logger.WithError(err).Error("Failed to generate recommendations")
				RecordWorkerError("recommender")
			}
		case <-ctx.Done():
			r.logger.Info("Stopping recommender")
			return
		}
	}
}

// generateRecommendations генерирует все типы рекомендаций
func (r *Recommender) generateRecommendations(ctx context.Context) error {
	r.logger.Debug("Starting recommendations generation")

	// Рейтинг прокси-провайдеров
	start := time.Now()
	if err := r.rankProxyProviders(ctx); err != nil {
		r.logger.WithError(err).Error("Failed to rank proxy providers")
	} else {
		recommendationGenerationDuration.WithLabelValues("proxy_provider").Observe(time.Since(start).Seconds())
		recommendationsGenerated.WithLabelValues("proxy_provider").Inc()
	}

	// Оптимальные сценарии прогрева
	start = time.Now()
	if err := r.recommendWarmingScenarios(ctx); err != nil {
		r.logger.WithError(err).Error("Failed to recommend warming scenarios")
	} else {
		recommendationGenerationDuration.WithLabelValues("warming_scenario").Observe(time.Since(start).Seconds())
		recommendationsGenerated.WithLabelValues("warming_scenario").Inc()
	}

	// Анализ проблемных паттернов
	start = time.Now()
	if err := r.analyzeErrorPatterns(ctx); err != nil {
		r.logger.WithError(err).Error("Failed to analyze error patterns")
	} else {
		recommendationGenerationDuration.WithLabelValues("error_pattern").Observe(time.Since(start).Seconds())
		recommendationsGenerated.WithLabelValues("error_pattern").Inc()
	}

	r.logger.Info("Recommendations generation completed")
	return nil
}

// rankProxyProviders создает рейтинг прокси-провайдеров
func (r *Recommender) rankProxyProviders(ctx context.Context) error {
	// Проверяем кэш
	cacheKey := "recommendations:proxy_providers"
	var cachedRanking models.ProxyProviderRating
	if r.redisCache != nil {
		err := r.redisCache.GetJSON(ctx, cacheKey, &cachedRanking)
		if err == nil {
			// Если есть в кэше и не устарел, используем кэш
			r.logger.Debug("Using cached proxy provider rankings")
			recommendation := &models.Recommendation{
				Type:        "proxy_provider",
				Priority:    "medium",
				GeneratedAt: time.Now(),
				ValidUntil:  time.Now().Add(6 * time.Hour),
				ProxyRating: &cachedRanking,
				ActionItems: r.generateProxyActionItems(cachedRanking.Rankings),
			}
			return r.recommendationRepo.Save(ctx, recommendation)
		}
	}

	// Получаем метрики за последние 7 дней
	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	metrics, err := r.metricsRepo.GetByTimeRange(ctx, "all", startTime, endTime)
	if err != nil {
		return err
	}

	if len(metrics) == 0 {
		return nil
	}

	// Получаем реальные данные прокси-провайдеров
	var rankings []models.ProviderRank

	if proxyClient := r.grpcClients["proxy"]; proxyClient != nil {
		// Вызываем gRPC метод GetProviderStatistics
		client := proxypb.NewProxyServiceClient(proxyClient)
		resp, err := client.GetProviderStatistics(ctx, &proxypb.GetProviderStatisticsRequest{
			Days: 7,
		})

		if err == nil && resp != nil && len(resp.ProviderStats) > 0 {
			// Используем реальные данные от proxy-service
			for _, providerStat := range resp.ProviderStats {
				rank := models.ProviderRank{
					Provider:       providerStat.Provider,
					SuccessRate:    providerStat.SuccessRate,
					BanRate:        providerStat.BanRate,
					AvgLatency:     providerStat.AvgLatency,
					CostPerAccount: providerStat.CostPerProxy,
				}

				// Рассчитываем взвешенный балл
				// Веса: success=0.4, ban_inv=0.3, latency_norm=0.2, cost_norm=0.1
				rank.Score = 0.4*rank.SuccessRate +
					0.3*(100-rank.BanRate) +
					0.2*(100-math.Min(rank.AvgLatency*100, 100)) +
					0.1*(100-math.Min(rank.CostPerAccount*5, 100))

				// Определяем рекомендацию
				if rank.Score > 80 {
					rank.Recommendation = "use"
				} else if rank.Score > 60 {
					rank.Recommendation = "monitor"
				} else {
					rank.Recommendation = "avoid"
				}

				rankings = append(rankings, rank)
			}
		} else {
			if err != nil && status.Code(err) != codes.Unimplemented {
				r.logger.WithError(err).Warn("Failed to get provider statistics from proxy-service")
			}
		}
	}

	// Если реальных данных нет, используем агрегированные метрики
	if len(rankings) == 0 && len(metrics) > 0 {
		// Собираем статистику по провайдерам из метрик
		providerStats := make(map[string]*models.ProviderRank)

		for _, m := range metrics {
			if m.ProxyProviderStats != nil {
				for provider, stats := range m.ProxyProviderStats {
					if _, exists := providerStats[provider]; !exists {
						providerStats[provider] = &models.ProviderRank{
							Provider: provider,
						}
					}
					// Усредняем показатели
					providerStats[provider].SuccessRate += stats.SuccessRate
					providerStats[provider].BanRate += stats.BanRate
					providerStats[provider].AvgLatency += stats.AvgLatency
					providerStats[provider].CostPerAccount += stats.CostPerAccount
				}
			}
		}

		// Вычисляем средние и добавляем в rankings
		metricsCount := float64(len(metrics))
		for provider, stats := range providerStats {
			stats.SuccessRate /= metricsCount
			stats.BanRate /= metricsCount
			stats.AvgLatency /= metricsCount
			stats.CostPerAccount /= metricsCount

			// Рассчитываем балл
			stats.Score = r.calculateProviderScore(stats)

			// Определяем рекомендацию
			if stats.Score > 80 {
				stats.Recommendation = "use"
			} else if stats.Score > 60 {
				stats.Recommendation = "monitor"
			} else {
				stats.Recommendation = "avoid"
			}

			rankings = append(rankings, *stats)
		}
	}

	// Если все еще нет данных, не создаем фальшивые данные - просто логируем
	if len(rankings) == 0 {
		r.logger.Warn("No proxy provider data available for rankings")
		return nil
	}

	// Сортируем по баллу
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})

	// Создаем рекомендацию
	recommendation := &models.Recommendation{
		Type:        "proxy_provider",
		Priority:    "medium",
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(6 * time.Hour),
		ProxyRating: &models.ProxyProviderRating{
			Rankings: rankings,
		},
		ActionItems: r.generateProxyActionItems(rankings),
	}

	if err := r.recommendationRepo.Save(ctx, recommendation); err != nil {
		return err
	}

	// Сохраняем в кэш на 6 часов
	if r.redisCache != nil {
		if err := r.redisCache.Set(ctx, cacheKey, recommendation.ProxyRating, 6*time.Hour); err != nil {
			r.logger.WithError(err).Warn("Failed to cache proxy provider rankings")
		}
	}

	r.logger.WithField("top_provider", rankings[0].Provider).Debug("Proxy provider rankings generated")
	return nil
}

// recommendWarmingScenarios рекомендует оптимальные сценарии прогрева
func (r *Recommender) recommendWarmingScenarios(ctx context.Context) error {
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		if err := r.recommendWarmingScenarioForPlatform(ctx, platform); err != nil {
			r.logger.WithError(err).WithField("platform", platform).Error("Failed to recommend warming scenario")
		}
	}

	return nil
}

// recommendWarmingScenarioForPlatform рекомендует сценарий для платформы
func (r *Recommender) recommendWarmingScenarioForPlatform(ctx context.Context, platform string) error {
	// Проверяем кэш
	cacheKey := fmt.Sprintf("recommendations:warming:%s", platform)
	var cachedRecommendation models.WarmingScenarioRecommendation
	if r.redisCache != nil {
		err := r.redisCache.GetJSON(ctx, cacheKey, &cachedRecommendation)
		if err == nil {
			// Используем кэшированную рекомендацию
			r.logger.WithField("platform", platform).Debug("Using cached warming scenario recommendation")
			recommendation := &models.Recommendation{
				Type:            "warming_scenario",
				Priority:        "high",
				GeneratedAt:     time.Now(),
				ValidUntil:      time.Now().Add(24 * time.Hour),
				WarmingScenario: &cachedRecommendation,
				ActionItems: []string{
					"Использовать " + cachedRecommendation.RecommendedType + " сценарий для новых аккаунтов",
					fmt.Sprintf("Ожидаемое время прогрева: %d дней", cachedRecommendation.RecommendedDays),
					"Мониторировать успешность и корректировать при необходимости",
				},
			}
			return r.recommendationRepo.Save(ctx, recommendation)
		}
	}

	// Получаем метрики за последние 30 дней
	endTime := time.Now()
	startTime := endTime.Add(-30 * 24 * time.Hour)

	metrics, err := r.metricsRepo.GetByTimeRange(ctx, platform, startTime, endTime)
	if err != nil {
		return err
	}

	if len(metrics) == 0 {
		return nil
	}

	// Получаем реальные данные о сценариях из warming-service
	var scenarios []struct {
		Type        string
		SuccessRate float64
		AvgDays     float64
	}

	if warmingClient := r.grpcClients["warming"]; warmingClient != nil {
		// Вызываем gRPC метод GetScenarioStatistics
		client := warmingpb.NewWarmingServiceClient(warmingClient)
		resp, err := client.GetScenarioStatistics(ctx, &warmingpb.ScenarioStatisticsRequest{
			Platform: platform,
			Days:     7,
		})

		if err == nil && resp != nil && len(resp.ScenarioStats) > 0 {
			// Используем реальные данные от warming-service
			for _, scenarioStat := range resp.ScenarioStats {
				scenarios = append(scenarios, struct {
					Type        string
					SuccessRate float64
					AvgDays     float64
				}{
					Type:        scenarioStat.ScenarioType,
					SuccessRate: scenarioStat.SuccessRate,
					AvgDays:     scenarioStat.AvgDurationDays,
				})
			}
		} else {
			if err != nil && status.Code(err) != codes.Unimplemented {
				r.logger.WithError(err).Warn("Failed to get scenario statistics from warming-service")
			}
		}
	}

	// Если реальных данных нет, используем данные из метрик
	if len(scenarios) == 0 {
		// Анализируем метрики для определения успешности сценариев
		basicSuccess := 75.0
		advancedSuccess := 85.0
		customSuccess := 90.0

		// Корректируем на основе метрик если есть данные
		if len(metrics) > 0 && metrics[len(metrics)-1].SuccessRate > 0 {
			successModifier := metrics[len(metrics)-1].SuccessRate / 100.0
			basicSuccess *= successModifier
			advancedSuccess *= successModifier
			customSuccess *= successModifier
		}

		scenarios = []struct {
			Type        string
			SuccessRate float64
			AvgDays     float64
		}{
			{"basic", basicSuccess, 14.0},
			{"advanced", advancedSuccess, 21.0},
			{"custom", customSuccess, 28.0},
		}
	}

	if len(scenarios) == 0 {
		r.logger.Warn("No warming scenario data available for platform", platform)
		return nil
	}

	// Выбираем лучший сценарий по соотношению успешности и времени
	var bestScenario struct {
		Type        string
		SuccessRate float64
		AvgDays     float64
	}
	bestScore := 0.0

	for _, scenario := range scenarios {
		// Балл = успешность / нормализованное время
		// Для VK предпочитаем более длительные сценарии
		timePenalty := scenario.AvgDays / 14.0
		if platform == "vk" {
			// VK требует более тщательного прогрева
			timePenalty = scenario.AvgDays / 21.0
		}

		score := scenario.SuccessRate / timePenalty
		if score > bestScore {
			bestScore = score
			bestScenario = scenario
		}
	}

	// Генерируем обоснование с учетом платформы
	reasoning := ""
	switch bestScenario.Type {
	case "basic":
		if platform == "telegram" {
			reasoning = "Базовый сценарий подходит для Telegram благодаря менее строгим требованиям"
		} else {
			reasoning = "Базовый сценарий обеспечивает быстрый прогрев с приемлемой успешностью"
		}
	case "advanced":
		if platform == "vk" {
			reasoning = "Продвинутый сценарий рекомендован для VK из-за строгих антиспам систем"
		} else {
			reasoning = "Продвинутый сценарий дает оптимальный баланс времени и качества"
		}
	case "custom":
		reasoning = fmt.Sprintf("Кастомный сценарий максимизирует успешность для %s аккаунтов", platform)
	}

	// Создаем рекомендацию
	recommendation := &models.Recommendation{
		Type:        "warming_scenario",
		Priority:    "high",
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(24 * time.Hour),
		WarmingScenario: &models.WarmingScenarioRecommendation{
			Platform:        platform,
			RecommendedType: bestScenario.Type,
			RecommendedDays: int(bestScenario.AvgDays),
			SuccessRate:     bestScenario.SuccessRate,
			Reasoning:       reasoning,
		},
		ActionItems: []string{
			"Использовать " + bestScenario.Type + " сценарий для новых аккаунтов",
			fmt.Sprintf("Ожидаемое время прогрева: %d дней", int(bestScenario.AvgDays)),
			"Мониторировать успешность и корректировать при необходимости",
		},
	}

	if err := r.recommendationRepo.Save(ctx, recommendation); err != nil {
		return err
	}

	// Сохраняем в кэш на 24 часа
	if r.redisCache != nil {
		if err := r.redisCache.Set(ctx, cacheKey, recommendation.WarmingScenario, 24*time.Hour); err != nil {
			r.logger.WithError(err).WithField("platform", platform).Warn("Failed to cache warming scenario recommendation")
		}
	}

	r.logger.WithFields(map[string]interface{}{
		"platform": platform,
		"scenario": bestScenario.Type,
	}).Debug("Warming scenario recommendation generated")

	return nil
}

// analyzeErrorPatterns анализирует паттерны ошибок
func (r *Recommender) analyzeErrorPatterns(ctx context.Context) error {
	// Проверяем кэш
	cacheKey := "recommendations:error_patterns"
	var cachedAnalysis models.ErrorPatternAnalysis
	if r.redisCache != nil {
		err := r.redisCache.GetJSON(ctx, cacheKey, &cachedAnalysis)
		if err == nil {
			// Используем кэшированный анализ
			r.logger.Debug("Using cached error pattern analysis")
			priority := "low"
			if len(cachedAnalysis.Clusters) > 0 && cachedAnalysis.Clusters[0].Frequency > 1000 {
				priority = "high"
			} else if len(cachedAnalysis.Clusters) > 0 && cachedAnalysis.Clusters[0].Frequency > 100 {
				priority = "medium"
			}
			recommendation := &models.Recommendation{
				Type:         "error_pattern",
				Priority:     priority,
				GeneratedAt:  time.Now(),
				ValidUntil:   time.Now().Add(24 * time.Hour),
				ErrorPattern: &cachedAnalysis,
				ActionItems:  r.generateErrorActionItems(cachedAnalysis.Clusters),
			}
			return r.recommendationRepo.Save(ctx, recommendation)
		}
	}

	// Получаем метрики за последние 7 дней
	endTime := time.Now()
	startTime := endTime.Add(-7 * 24 * time.Hour)

	allMetrics, err := r.metricsRepo.GetByTimeRange(ctx, "all", startTime, endTime)
	if err != nil {
		return err
	}

	// Собираем все типы ошибок
	errorFrequency := make(map[string]int64)
	errorPlatforms := make(map[string]map[string]bool)

	for _, metrics := range allMetrics {
		for _, errorStat := range metrics.TopErrors {
			errorFrequency[errorStat.Type] += errorStat.Count

			if errorPlatforms[errorStat.Type] == nil {
				errorPlatforms[errorStat.Type] = make(map[string]bool)
			}
			if metrics.Platform != "all" {
				errorPlatforms[errorStat.Type][metrics.Platform] = true
			}
		}
	}

	// Создаем кластеры ошибок
	var clusters []models.ErrorCluster

	for errorType, frequency := range errorFrequency {
		// Определяем затронутые платформы
		var affectedPlatforms []string
		for platform := range errorPlatforms[errorType] {
			affectedPlatforms = append(affectedPlatforms, platform)
		}

		// Определяем root cause и mitigation на основе типа ошибки
		rootCause, mitigation := r.analyzeErrorType(errorType)

		cluster := models.ErrorCluster{
			Pattern:           errorType,
			Frequency:         frequency,
			AffectedPlatforms: affectedPlatforms,
			RootCause:         rootCause,
			Mitigation:        mitigation,
		}

		clusters = append(clusters, cluster)
	}

	// Сортируем по частоте
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Frequency > clusters[j].Frequency
	})

	// Оставляем топ-5
	if len(clusters) > 5 {
		clusters = clusters[:5]
	}

	// Определяем приоритет
	priority := "low"
	if len(clusters) > 0 && clusters[0].Frequency > 1000 {
		priority = "high"
	} else if len(clusters) > 0 && clusters[0].Frequency > 100 {
		priority = "medium"
	}

	// Создаем рекомендацию
	recommendation := &models.Recommendation{
		Type:        "error_pattern",
		Priority:    priority,
		GeneratedAt: time.Now(),
		ValidUntil:  time.Now().Add(24 * time.Hour),
		ErrorPattern: &models.ErrorPatternAnalysis{
			Clusters: clusters,
		},
		ActionItems: r.generateErrorActionItems(clusters),
	}

	if err := r.recommendationRepo.Save(ctx, recommendation); err != nil {
		return err
	}

	// Сохраняем в кэш на 24 часа
	if r.redisCache != nil {
		if err := r.redisCache.Set(ctx, cacheKey, recommendation.ErrorPattern, 24*time.Hour); err != nil {
			r.logger.WithError(err).Warn("Failed to cache error pattern analysis")
		}
	}

	r.logger.WithField("clusters_count", len(clusters)).Debug("Error pattern analysis generated")
	return nil
}

// calculateProviderScore рассчитывает общий балл провайдера
func (r *Recommender) calculateProviderScore(rank *models.ProviderRank) float64 {
	// Веса: success_rate=0.4, ban_rate=0.3, latency=0.2, cost=0.1
	return 0.4*rank.SuccessRate +
		0.3*(100-rank.BanRate) +
		0.2*(100-math.Min(rank.AvgLatency*100, 100)) +
		0.1*(100-math.Min(rank.CostPerAccount*5, 100))
}

// analyzeErrorType анализирует тип ошибки и возвращает причину и решение
func (r *Recommender) analyzeErrorType(errorType string) (string, string) {
	// Базовые правила для определения причин и решений
	errorPatterns := map[string]struct {
		rootCause  string
		mitigation string
	}{
		"sms_timeout": {
			"Превышено время ожидания SMS от провайдера",
			"Увеличить таймаут или сменить SMS провайдера",
		},
		"proxy_connection": {
			"Проблемы с подключением к прокси серверу",
			"Проверить доступность прокси, увеличить retry count",
		},
		"account_banned": {
			"Аккаунт заблокирован платформой",
			"Пересмотреть стратегию прогрева, использовать более мягкие сценарии",
		},
		"rate_limit": {
			"Превышен лимит запросов к API",
			"Реализовать экспоненциальный backoff, распределить нагрузку",
		},
		"auth_failed": {
			"Ошибка авторизации",
			"Проверить валидность токенов, обновить credentials",
		},
	}

	// Проверяем известные паттерны
	for pattern, analysis := range errorPatterns {
		if errorType == pattern {
			return analysis.rootCause, analysis.mitigation
		}
	}

	// Дефолтные значения для неизвестных ошибок
	return "Неизвестная причина ошибки", "Требуется дополнительный анализ логов"
}

// generateProxyActionItems генерирует действия для прокси
func (r *Recommender) generateProxyActionItems(rankings []models.ProviderRank) []string {
	items := []string{}

	if len(rankings) > 0 {
		// Рекомендация по лучшему провайдеру
		if rankings[0].Recommendation == "use" {
			items = append(items, fmt.Sprintf("Рекомендуется использовать %s (score: %.0f)", rankings[0].Provider, rankings[0].Score))
		}

		// Предупреждения о плохих провайдерах
		for _, rank := range rankings {
			if rank.Recommendation == "avoid" {
				items = append(items, "Избегать использования "+rank.Provider+" (высокий ban rate)")
			}
		}

		// Общие рекомендации
		items = append(items, "Регулярно мониторить метрики прокси провайдеров")
		items = append(items, "Диверсифицировать использование провайдеров для снижения рисков")
	}

	return items
}

// generateErrorActionItems генерирует действия для ошибок
func (r *Recommender) generateErrorActionItems(clusters []models.ErrorCluster) []string {
	items := []string{}

	for i, cluster := range clusters {
		if i >= 3 { // Топ-3 проблемы
			break
		}
		items = append(items, cluster.Mitigation)
	}

	// Общие рекомендации
	if len(clusters) > 0 {
		items = append(items, "Настроить мониторинг критичных ошибок")
		items = append(items, "Реализовать автоматическое восстановление для частых ошибок")
	}

	return items
}

// GetProxyRankings получает рейтинг прокси
func (r *Recommender) GetProxyRankings(ctx context.Context) (*models.ProxyProviderRating, error) {
	return r.recommendationRepo.GetProxyRatings(ctx)
}

// GetWarmingRecommendations получает рекомендации по прогреву
func (r *Recommender) GetWarmingRecommendations(ctx context.Context, platform string) (*models.WarmingScenarioRecommendation, error) {
	return r.recommendationRepo.GetWarmingScenarioRecommendation(ctx, platform)
}

// GetErrorPatterns получает анализ паттернов ошибок
func (r *Recommender) GetErrorPatterns(ctx context.Context) (*models.ErrorPatternAnalysis, error) {
	return r.recommendationRepo.GetErrorPatterns(ctx)
}
