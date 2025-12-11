package handlers

import (
	"context"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/analytics-service/internal/models"
	"github.com/grigta/conveer/services/analytics-service/internal/service"
	pb "github.com/grigta/conveer/services/analytics-service/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AnalyticsHandler обработчик gRPC запросов
type AnalyticsHandler struct {
	pb.UnimplementedAnalyticsServiceServer
	analyticsService *service.AnalyticsService
	logger           *logger.Logger
}

// NewAnalyticsHandler создает новый обработчик
func NewAnalyticsHandler(analyticsService *service.AnalyticsService, logger *logger.Logger) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
		logger:           logger,
	}
}

// GetOverallAnalytics получает общую аналитику
func (h *AnalyticsHandler) GetOverallAnalytics(ctx context.Context, req *pb.AnalyticsRequest) (*pb.OverallAnalytics, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetOverallAnalytics", time.Since(start).Seconds())
	}()

	// Получаем даты из запроса
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	if req.StartDate != nil {
		startDate = req.StartDate.AsTime()
	}
	if req.EndDate != nil {
		endDate = req.EndDate.AsTime()
	}

	// Получаем аналитику
	analytics, err := h.analyticsService.GetOverallAnalytics(ctx, startDate, endDate)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get overall analytics")
		return nil, status.Error(codes.Internal, "Failed to get analytics")
	}

	// Конвертируем в protobuf
	response := &pb.OverallAnalytics{
		TotalAccounts:      analytics.TotalAccounts,
		AccountsByPlatform: analytics.AccountsByPlatform,
		AccountsByStatus:   analytics.AccountsByStatus,
		OverallSuccessRate: analytics.OverallSuccessRate,
		OverallBanRate:     analytics.OverallBanRate,
		Expenses: &pb.ExpensesSummary{
			TotalSpentToday:  analytics.Expenses.TotalSpentToday,
			TotalSpentWeek:   analytics.Expenses.TotalSpentWeek,
			TotalSpentMonth:  analytics.Expenses.TotalSpentMonth,
			SmsSpent:         analytics.Expenses.SMSSpent,
			ProxySpent:       analytics.Expenses.ProxySpent,
			AvgCostPerAccount: analytics.Expenses.AvgCostPerAccount,
		},
		Resources: &pb.ResourcesSummary{
			ActiveProxies:      analytics.Resources.ActiveProxies,
			BannedProxies:      analytics.Resources.BannedProxies,
			SmsBalance:         analytics.Resources.SMSBalance,
			WarmingTasksActive: analytics.Resources.WarmingTasksActive,
		},
		Performance: &pb.PerformanceSummary{
			AvgWarmingDays:       analytics.Performance.AvgWarmingDays,
			AccountsCreatedToday: analytics.Performance.AccountsCreatedToday,
			AccountsReadyToday:   analytics.Performance.AccountsReadyToday,
			ErrorRate:            analytics.Performance.ErrorRate,
			TopErrors:            convertErrorStats(analytics.Performance.TopErrors),
		},
		Trends: convertTrends(analytics.Trends),
	}

	return response, nil
}

// GetPlatformAnalytics получает аналитику по платформе
func (h *AnalyticsHandler) GetPlatformAnalytics(ctx context.Context, req *pb.PlatformRequest) (*pb.PlatformAnalytics, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetPlatformAnalytics", time.Since(start).Seconds())
	}()

	if req.Platform == "" {
		return nil, status.Error(codes.InvalidArgument, "Platform is required")
	}

	analytics, err := h.analyticsService.GetPlatformAnalytics(ctx, req.Platform)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get platform analytics")
		return nil, status.Error(codes.Internal, "Failed to get platform analytics")
	}

	return &pb.PlatformAnalytics{
		Platform:        analytics.Platform,
		TotalAccounts:   analytics.TotalAccounts,
		ByStatus:        analytics.ByStatus,
		SuccessRate:     analytics.SuccessRate,
		BanRate:         analytics.BanRate,
		AvgWarmingDays:  analytics.AvgWarmingDays,
		TotalSpent:      analytics.TotalSpent,
		Recommendations: analytics.Recommendations,
	}, nil
}

// GetExpenseForecast получает прогноз расходов
func (h *AnalyticsHandler) GetExpenseForecast(ctx context.Context, req *pb.ForecastRequest) (*pb.ExpenseForecastResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetExpenseForecast", time.Since(start).Seconds())
	}()

	if req.Period == "" {
		req.Period = "7d"
	}

	forecast, err := h.analyticsService.GetExpenseForecast(ctx, req.Period)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get expense forecast")
	}

	if forecast.ExpenseForecast == nil {
		return nil, status.Error(codes.NotFound, "No forecast available")
	}

	return &pb.ExpenseForecastResponse{
		Period:        forecast.ExpenseForecast.Period,
		PredictedCost: forecast.ExpenseForecast.PredictedCost,
		UpperBound:    forecast.ExpenseForecast.UpperBound,
		LowerBound:    forecast.ExpenseForecast.LowerBound,
		Breakdown:     forecast.ExpenseForecast.Breakdown,
		Confidence:    forecast.Confidence,
		GeneratedAt:   timestamppb.New(forecast.GeneratedAt),
	}, nil
}

// GetAccountReadinessForecast получает прогноз готовности аккаунта
func (h *AnalyticsHandler) GetAccountReadinessForecast(ctx context.Context, req *pb.ReadinessRequest) (*pb.ReadinessForecastResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetAccountReadinessForecast", time.Since(start).Seconds())
	}()

	forecast, err := h.analyticsService.GetAccountReadinessForecast(ctx, req.AccountId, req.Platform)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get readiness forecast")
	}

	if forecast.ReadinessForecast == nil {
		return nil, status.Error(codes.NotFound, "No forecast available")
	}

	return &pb.ReadinessForecastResponse{
		AccountId:       forecast.ReadinessForecast.AccountID,
		EstimatedDays:   int32(forecast.ReadinessForecast.EstimatedDays),
		CompletionDate:  timestamppb.New(forecast.ReadinessForecast.CompletionDate),
		CurrentProgress: forecast.ReadinessForecast.CurrentProgress,
	}, nil
}

// GetOptimalRegistrationTime получает оптимальное время регистрации
func (h *AnalyticsHandler) GetOptimalRegistrationTime(ctx context.Context, req *pb.OptimalTimeRequest) (*pb.OptimalTimeResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetOptimalRegistrationTime", time.Since(start).Seconds())
	}()

	forecast, err := h.analyticsService.GetOptimalRegistrationTime(ctx, req.Platform)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get optimal time forecast")
	}

	if forecast.OptimalTimeForecast == nil {
		return nil, status.Error(codes.NotFound, "No forecast available")
	}

	var bestHours []int32
	for _, h := range forecast.OptimalTimeForecast.BestHours {
		bestHours = append(bestHours, int32(h))
	}

	return &pb.OptimalTimeResponse{
		BestHours:   bestHours,
		BestDays:    forecast.OptimalTimeForecast.BestDays,
		SuccessRate: forecast.OptimalTimeForecast.SuccessRate,
		SampleSize:  forecast.OptimalTimeForecast.SampleSize,
	}, nil
}

// GetProxyProviderRankings получает рейтинг прокси провайдеров
func (h *AnalyticsHandler) GetProxyProviderRankings(ctx context.Context, req *emptypb.Empty) (*pb.ProxyRankingsResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetProxyProviderRankings", time.Since(start).Seconds())
	}()

	rankings, err := h.analyticsService.GetProxyProviderRankings(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get proxy rankings")
	}

	var pbRankings []*pb.ProviderRanking
	for _, rank := range rankings.Rankings {
		pbRankings = append(pbRankings, &pb.ProviderRanking{
			Provider:       rank.Provider,
			Score:          rank.Score,
			SuccessRate:    rank.SuccessRate,
			AvgLatency:     rank.AvgLatency,
			BanRate:        rank.BanRate,
			CostPerAccount: rank.CostPerAccount,
			Recommendation: rank.Recommendation,
		})
	}

	return &pb.ProxyRankingsResponse{
		Rankings:    pbRankings,
		GeneratedAt: timestamppb.Now(),
	}, nil
}

// GetWarmingScenarioRecommendations получает рекомендации по сценариям прогрева
func (h *AnalyticsHandler) GetWarmingScenarioRecommendations(ctx context.Context, req *pb.PlatformRequest) (*pb.WarmingRecommendationsResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetWarmingScenarioRecommendations", time.Since(start).Seconds())
	}()

	recommendation, err := h.analyticsService.GetWarmingScenarioRecommendations(ctx, req.Platform)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get warming recommendations")
	}

	return &pb.WarmingRecommendationsResponse{
		Platform:        recommendation.Platform,
		RecommendedType: recommendation.RecommendedType,
		RecommendedDays: int32(recommendation.RecommendedDays),
		SuccessRate:     recommendation.SuccessRate,
		Reasoning:       recommendation.Reasoning,
	}, nil
}

// GetErrorPatternAnalysis получает анализ паттернов ошибок
func (h *AnalyticsHandler) GetErrorPatternAnalysis(ctx context.Context, req *pb.AnalysisRequest) (*pb.ErrorPatternResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetErrorPatternAnalysis", time.Since(start).Seconds())
	}()

	days := 7
	if req.Days > 0 {
		days = int(req.Days)
	}

	analysis, err := h.analyticsService.GetErrorPatternAnalysis(ctx, days)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get error pattern analysis")
	}

	var clusters []*pb.ErrorCluster
	for _, cluster := range analysis.Clusters {
		clusters = append(clusters, &pb.ErrorCluster{
			Pattern:           cluster.Pattern,
			Frequency:         cluster.Frequency,
			AffectedPlatforms: cluster.AffectedPlatforms,
			RootCause:         cluster.RootCause,
			Mitigation:        cluster.Mitigation,
		})
	}

	return &pb.ErrorPatternResponse{
		Clusters: clusters,
	}, nil
}

// GetActiveAlerts получает активные алерты
func (h *AnalyticsHandler) GetActiveAlerts(ctx context.Context, req *pb.AlertsRequest) (*pb.AlertsResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("GetActiveAlerts", time.Since(start).Seconds())
	}()

	alerts, err := h.analyticsService.GetActiveAlerts(ctx, req.UnacknowledgedOnly, req.Severity)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get active alerts")
	}

	var pbAlerts []*pb.AlertEvent
	for _, alert := range alerts {
		pbAlerts = append(pbAlerts, &pb.AlertEvent{
			Id:           alert.ID.Hex(),
			RuleName:     alert.RuleName,
			Severity:     alert.Severity,
			Platform:     alert.Platform,
			Message:      alert.Message,
			CurrentValue: alert.CurrentValue,
			Threshold:    alert.Threshold,
			FiredAt:      timestamppb.New(alert.FiredAt),
			Acknowledged: alert.Acknowledged,
		})
	}

	return &pb.AlertsResponse{
		Alerts: pbAlerts,
	}, nil
}

// AcknowledgeAlert подтверждает алерт
func (h *AnalyticsHandler) AcknowledgeAlert(ctx context.Context, req *pb.AcknowledgeRequest) (*emptypb.Empty, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("AcknowledgeAlert", time.Since(start).Seconds())
	}()

	if req.AlertId == "" {
		return nil, status.Error(codes.InvalidArgument, "Alert ID is required")
	}

	if err := h.analyticsService.AcknowledgeAlert(ctx, req.AlertId, "user"); err != nil {
		return nil, status.Error(codes.Internal, "Failed to acknowledge alert")
	}

	return &emptypb.Empty{}, nil
}

// CreateAlertRule создает правило алерта
func (h *AnalyticsHandler) CreateAlertRule(ctx context.Context, req *pb.CreateRuleRequest) (*pb.AlertRuleResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("CreateAlertRule", time.Since(start).Seconds())
	}()

	if req.Name == "" || req.Type == "" {
		return nil, status.Error(codes.InvalidArgument, "Name and type are required")
	}

	rule := &models.AlertRule{
		Name:     req.Name,
		Type:     req.Type,
		Platform: req.Platform,
		Enabled:  true,
		Threshold: models.AlertThreshold{
			Operator: req.Threshold.Operator,
			Value:    req.Threshold.Value,
		},
		Severity: req.Severity,
		Cooldown: int(req.Cooldown),
	}

	if err := h.analyticsService.CreateAlertRule(ctx, rule); err != nil {
		return nil, status.Error(codes.Internal, "Failed to create alert rule")
	}

	return &pb.AlertRuleResponse{
		Id:       rule.ID.Hex(),
		Name:     rule.Name,
		Type:     rule.Type,
		Platform: rule.Platform,
		Enabled:  rule.Enabled,
		Threshold: &pb.AlertThreshold{
			Operator: rule.Threshold.Operator,
			Value:    rule.Threshold.Value,
		},
		Severity: rule.Severity,
		Cooldown: int32(rule.Cooldown),
	}, nil
}

// UpdateAlertRule обновляет правило алерта
func (h *AnalyticsHandler) UpdateAlertRule(ctx context.Context, req *pb.UpdateRuleRequest) (*pb.AlertRuleResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("UpdateAlertRule", time.Since(start).Seconds())
	}()

	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "Rule ID is required")
	}

	var threshold *models.AlertThreshold
	if req.Threshold != nil {
		threshold = &models.AlertThreshold{
			Operator: req.Threshold.Operator,
			Value:    req.Threshold.Value,
		}
	}

	if err := h.analyticsService.UpdateAlertRule(ctx, req.RuleId, req.Enabled, threshold, int(req.Cooldown)); err != nil {
		return nil, status.Error(codes.Internal, "Failed to update alert rule")
	}

	// Получаем обновленное правило для ответа
	rules, err := h.analyticsService.ListAlertRules(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to get updated rule")
	}

	for _, rule := range rules {
		if rule.ID.Hex() == req.RuleId {
			return &pb.AlertRuleResponse{
				Id:       rule.ID.Hex(),
				Name:     rule.Name,
				Type:     rule.Type,
				Platform: rule.Platform,
				Enabled:  rule.Enabled,
				Threshold: &pb.AlertThreshold{
					Operator: rule.Threshold.Operator,
					Value:    rule.Threshold.Value,
				},
				Severity: rule.Severity,
				Cooldown: int32(rule.Cooldown),
			}, nil
		}
	}

	return nil, status.Error(codes.NotFound, "Rule not found")
}

// DeleteAlertRule удаляет правило алерта
func (h *AnalyticsHandler) DeleteAlertRule(ctx context.Context, req *pb.DeleteRuleRequest) (*emptypb.Empty, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("DeleteAlertRule", time.Since(start).Seconds())
	}()

	if req.RuleId == "" {
		return nil, status.Error(codes.InvalidArgument, "Rule ID is required")
	}

	if err := h.analyticsService.DeleteAlertRule(ctx, req.RuleId); err != nil {
		return nil, status.Error(codes.Internal, "Failed to delete alert rule")
	}

	return &emptypb.Empty{}, nil
}

// ListAlertRules получает список правил алертов
func (h *AnalyticsHandler) ListAlertRules(ctx context.Context, req *emptypb.Empty) (*pb.AlertRulesResponse, error) {
	start := time.Now()
	defer func() {
		service.RecordGRPCRequest("ListAlertRules", time.Since(start).Seconds())
	}()

	rules, err := h.analyticsService.ListAlertRules(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to list alert rules")
	}

	var pbRules []*pb.AlertRuleResponse
	for _, rule := range rules {
		pbRules = append(pbRules, &pb.AlertRuleResponse{
			Id:       rule.ID.Hex(),
			Name:     rule.Name,
			Type:     rule.Type,
			Platform: rule.Platform,
			Enabled:  rule.Enabled,
			Threshold: &pb.AlertThreshold{
				Operator: rule.Threshold.Operator,
				Value:    rule.Threshold.Value,
			},
			Severity: rule.Severity,
			Cooldown: int32(rule.Cooldown),
		})
	}

	return &pb.AlertRulesResponse{
		Rules: pbRules,
	}, nil
}

// Helper функции

func convertErrorStats(errors []models.ErrorStat) []*pb.ErrorStat {
	var result []*pb.ErrorStat
	for _, err := range errors {
		result = append(result, &pb.ErrorStat{
			Type:  err.Type,
			Count: err.Count,
		})
	}
	return result
}

func convertTrends(trends []service.TrendData) []*pb.TrendData {
	var result []*pb.TrendData
	for _, trend := range trends {
		result = append(result, &pb.TrendData{
			Date:            timestamppb.New(trend.Date),
			AccountsCreated: trend.AccountsCreated,
			AccountsBanned:  trend.AccountsBanned,
			Expenses:        trend.Expenses,
		})
	}
	return result
}
