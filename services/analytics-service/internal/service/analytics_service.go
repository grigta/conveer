package service

import (
	"context"
	"time"

	"github.com/conveer/conveer/pkg/logger"
	"github.com/conveer/conveer/services/analytics-service/internal/models"
	"github.com/conveer/conveer/services/analytics-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AnalyticsService основной сервис аналитики
type AnalyticsService struct {
	metricsRepo        *repository.MetricsRepository
	forecastRepo       *repository.ForecastRepository
	recommendationRepo *repository.RecommendationRepository
	alertRepo          *repository.AlertRepository

	aggregator   *Aggregator
	forecaster   *Forecaster
	recommender  *Recommender
	alertManager *AlertManager

	logger *logger.Logger
}

// NewAnalyticsService создает новый сервис аналитики
func NewAnalyticsService(
	metricsRepo *repository.MetricsRepository,
	forecastRepo *repository.ForecastRepository,
	recommendationRepo *repository.RecommendationRepository,
	alertRepo *repository.AlertRepository,
	aggregator *Aggregator,
	forecaster *Forecaster,
	recommender *Recommender,
	alertManager *AlertManager,
	logger *logger.Logger,
) *AnalyticsService {
	return &AnalyticsService{
		metricsRepo:        metricsRepo,
		forecastRepo:       forecastRepo,
		recommendationRepo: recommendationRepo,
		alertRepo:          alertRepo,
		aggregator:         aggregator,
		forecaster:         forecaster,
		recommender:        recommender,
		alertManager:       alertManager,
		logger:             logger,
	}
}

// GetOverallAnalytics получает общую аналитику
func (s *AnalyticsService) GetOverallAnalytics(ctx context.Context, startDate, endDate time.Time) (*OverallAnalytics, error) {
	// Получаем последние метрики
	latestMetrics, err := s.metricsRepo.GetLatest(ctx, "all")
	if err != nil {
		return nil, err
	}

	// Получаем тренды за последние 7 дней
	trends, err := s.metricsRepo.GetTrends(ctx, "all", 7)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get trends")
	}

	// Преобразуем тренды
	var trendData []TrendData
	for _, trend := range trends {
		trendData = append(trendData, TrendData{
			Date:            trend.Timestamp,
			AccountsCreated: trend.TotalAccounts,
			AccountsBanned:  trend.AccountsByStatus["banned"],
			Expenses:        trend.TotalSpent,
		})
	}

	// Получаем агрегированную статистику
	stats, err := s.metricsRepo.GetAggregatedStats(ctx, "all", 24*time.Hour)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get aggregated stats")
	}

	// Собираем общую аналитику
	analytics := &OverallAnalytics{
		TotalAccounts:       latestMetrics.TotalAccounts,
		AccountsByPlatform:  s.getAccountsByPlatform(ctx),
		AccountsByStatus:    latestMetrics.AccountsByStatus,
		OverallSuccessRate:  latestMetrics.SuccessRate,
		OverallBanRate:      latestMetrics.BanRate,
		Expenses: ExpensesSummary{
			TotalSpentToday: s.getSpentForPeriod(ctx, 24*time.Hour),
			TotalSpentWeek:  s.getSpentForPeriod(ctx, 7*24*time.Hour),
			TotalSpentMonth: s.getSpentForPeriod(ctx, 30*24*time.Hour),
			SMSSpent:        latestMetrics.SMSSpent,
			ProxySpent:      latestMetrics.ProxySpent,
		},
		Resources: ResourcesSummary{
			ActiveProxies:      latestMetrics.ActiveProxies,
			BannedProxies:      latestMetrics.BannedProxies,
			SMSBalance:         latestMetrics.SMSBalance,
			WarmingTasksActive: latestMetrics.WarmingActive,
		},
		Performance: PerformanceSummary{
			AvgWarmingDays:       latestMetrics.AvgWarmingDays,
			AccountsCreatedToday: s.getAccountsCreatedToday(ctx),
			AccountsReadyToday:   s.getAccountsReadyToday(ctx),
			ErrorRate:            latestMetrics.ErrorRate,
			TopErrors:            latestMetrics.TopErrors,
		},
		Trends: trendData,
	}

	// Обновляем бизнес-метрики
	UpdateBusinessMetrics(latestMetrics)

	return analytics, nil
}

// GetPlatformAnalytics получает аналитику по платформе
func (s *AnalyticsService) GetPlatformAnalytics(ctx context.Context, platform string) (*PlatformAnalytics, error) {
	// Получаем последние метрики для платформы
	metrics, err := s.metricsRepo.GetLatest(ctx, platform)
	if err != nil {
		return nil, err
	}

	// Получаем рекомендации для платформы
	warmingRec, _ := s.recommendationRepo.GetWarmingScenarioRecommendation(ctx, platform)

	var recommendations []string
	if warmingRec != nil {
		recommendations = append(recommendations, warmingRec.Reasoning)
	}

	analytics := &PlatformAnalytics{
		Platform:       platform,
		TotalAccounts:  metrics.TotalAccounts,
		ByStatus:       metrics.AccountsByStatus,
		SuccessRate:    metrics.SuccessRate,
		BanRate:        metrics.BanRate,
		AvgWarmingDays: metrics.AvgWarmingDays,
		TotalSpent:     metrics.TotalSpent,
		Recommendations: recommendations,
	}

	return analytics, nil
}

// GetExpenseForecast получает прогноз расходов
func (s *AnalyticsService) GetExpenseForecast(ctx context.Context, period string) (*models.ForecastResult, error) {
	return s.forecaster.GetExpenseForecast(ctx, period)
}

// GetAccountReadinessForecast получает прогноз готовности аккаунта
func (s *AnalyticsService) GetAccountReadinessForecast(ctx context.Context, accountID, platform string) (*models.ForecastResult, error) {
	return s.forecaster.GetReadinessForecast(ctx, accountID)
}

// GetOptimalRegistrationTime получает оптимальное время регистрации
func (s *AnalyticsService) GetOptimalRegistrationTime(ctx context.Context, platform string) (*models.ForecastResult, error) {
	return s.forecaster.GetOptimalTimeForecast(ctx, platform)
}

// GetProxyProviderRankings получает рейтинг прокси-провайдеров
func (s *AnalyticsService) GetProxyProviderRankings(ctx context.Context) (*models.ProxyProviderRating, error) {
	return s.recommender.GetProxyRankings(ctx)
}

// GetWarmingScenarioRecommendations получает рекомендации по сценариям прогрева
func (s *AnalyticsService) GetWarmingScenarioRecommendations(ctx context.Context, platform string) (*models.WarmingScenarioRecommendation, error) {
	return s.recommender.GetWarmingRecommendations(ctx, platform)
}

// GetErrorPatternAnalysis получает анализ паттернов ошибок
func (s *AnalyticsService) GetErrorPatternAnalysis(ctx context.Context, days int) (*models.ErrorPatternAnalysis, error) {
	return s.recommender.GetErrorPatterns(ctx)
}

// GetActiveAlerts получает активные алерты
func (s *AnalyticsService) GetActiveAlerts(ctx context.Context, unacknowledgedOnly bool, severity string) ([]models.AlertEvent, error) {
	// Используем новый метод с поддержкой фильтров
	return s.alertManager.GetAlerts(ctx, unacknowledgedOnly, severity)
}

// AcknowledgeAlert подтверждает алерт
func (s *AnalyticsService) AcknowledgeAlert(ctx context.Context, alertID, acknowledgedBy string) error {
	return s.alertManager.AcknowledgeAlert(ctx, alertID, acknowledgedBy)
}

// CreateAlertRule создает правило алерта
func (s *AnalyticsService) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return s.alertManager.CreateAlertRule(ctx, rule)
}

// UpdateAlertRule обновляет правило алерта
func (s *AnalyticsService) UpdateAlertRule(ctx context.Context, ruleID string, enabled bool, threshold *models.AlertThreshold, cooldown int) error {
	id, err := primitive.ObjectIDFromHex(ruleID)
	if err != nil {
		return err
	}

	rule, err := s.alertRepo.GetRuleByID(ctx, id)
	if err != nil {
		return err
	}

	rule.Enabled = enabled
	if threshold != nil {
		rule.Threshold = *threshold
	}
	if cooldown > 0 {
		rule.Cooldown = cooldown
	}

	return s.alertManager.UpdateAlertRule(ctx, rule)
}

// DeleteAlertRule удаляет правило алерта
func (s *AnalyticsService) DeleteAlertRule(ctx context.Context, ruleID string) error {
	return s.alertManager.DeleteAlertRule(ctx, ruleID)
}

// ListAlertRules получает список правил алертов
func (s *AnalyticsService) ListAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	return s.alertManager.GetAlertRules(ctx)
}

// ForceAggregation принудительно запускает агрегацию
func (s *AnalyticsService) ForceAggregation(ctx context.Context) error {
	return s.aggregator.ForceAggregate(ctx)
}

// Helper методы

func (s *AnalyticsService) getAccountsByPlatform(ctx context.Context) map[string]int64 {
	result := make(map[string]int64)
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		metrics, err := s.metricsRepo.GetLatest(ctx, platform)
		if err == nil {
			result[platform] = metrics.TotalAccounts
		}
	}

	return result
}

func (s *AnalyticsService) getSpentForPeriod(ctx context.Context, period time.Duration) float64 {
	startTime := time.Now().Add(-period)
	metrics, err := s.metricsRepo.GetByTimeRange(ctx, "all", startTime, time.Now())
	if err != nil || len(metrics) == 0 {
		return 0
	}

	var total float64
	for _, m := range metrics {
		total += m.TotalSpent
	}
	return total
}

func (s *AnalyticsService) getAccountsCreatedToday(ctx context.Context) int64 {
	startTime := time.Now().Add(-24 * time.Hour)
	metrics, err := s.metricsRepo.GetByTimeRange(ctx, "all", startTime, time.Now())
	if err != nil || len(metrics) < 2 {
		return 0
	}

	// Разница между последним и первым значением
	return metrics[len(metrics)-1].TotalAccounts - metrics[0].TotalAccounts
}

func (s *AnalyticsService) getAccountsReadyToday(ctx context.Context) int64 {
	startTime := time.Now().Add(-24 * time.Hour)
	metrics, err := s.metricsRepo.GetByTimeRange(ctx, "all", startTime, time.Now())
	if err != nil || len(metrics) < 2 {
		return 0
	}

	// Разница в ready аккаунтах
	lastReady := metrics[len(metrics)-1].AccountsByStatus["ready"]
	firstReady := metrics[0].AccountsByStatus["ready"]
	return lastReady - firstReady
}

// DTO структуры

type OverallAnalytics struct {
	TotalAccounts      int64                     `json:"total_accounts"`
	AccountsByPlatform map[string]int64          `json:"accounts_by_platform"`
	AccountsByStatus   map[string]int64          `json:"accounts_by_status"`
	OverallSuccessRate float64                   `json:"overall_success_rate"`
	OverallBanRate     float64                   `json:"overall_ban_rate"`
	Expenses           ExpensesSummary           `json:"expenses"`
	Resources          ResourcesSummary          `json:"resources"`
	Performance        PerformanceSummary        `json:"performance"`
	Trends             []TrendData               `json:"trends"`
}

type ExpensesSummary struct {
	TotalSpentToday float64 `json:"total_spent_today"`
	TotalSpentWeek  float64 `json:"total_spent_week"`
	TotalSpentMonth float64 `json:"total_spent_month"`
	SMSSpent        float64 `json:"sms_spent"`
	ProxySpent      float64 `json:"proxy_spent"`
	AvgCostPerAccount float64 `json:"avg_cost_per_account"`
}

type ResourcesSummary struct {
	ActiveProxies      int64   `json:"active_proxies"`
	BannedProxies      int64   `json:"banned_proxies"`
	SMSBalance         float64 `json:"sms_balance"`
	WarmingTasksActive int64   `json:"warming_tasks_active"`
}

type PerformanceSummary struct {
	AvgWarmingDays       float64           `json:"avg_warming_days"`
	AccountsCreatedToday int64             `json:"accounts_created_today"`
	AccountsReadyToday   int64             `json:"accounts_ready_today"`
	ErrorRate            float64           `json:"error_rate"`
	TopErrors            []models.ErrorStat `json:"top_errors"`
}

type TrendData struct {
	Date            time.Time `json:"date"`
	AccountsCreated int64     `json:"accounts_created"`
	AccountsBanned  int64     `json:"accounts_banned"`
	Expenses        float64   `json:"expenses"`
}

type PlatformAnalytics struct {
	Platform        string           `json:"platform"`
	TotalAccounts   int64            `json:"total_accounts"`
	ByStatus        map[string]int64 `json:"by_status"`
	SuccessRate     float64          `json:"success_rate"`
	BanRate         float64          `json:"ban_rate"`
	AvgWarmingDays  float64          `json:"avg_warming_days"`
	TotalSpent      float64          `json:"total_spent"`
	Recommendations []string         `json:"recommendations"`
}