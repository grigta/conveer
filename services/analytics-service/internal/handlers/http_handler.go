package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/conveer/conveer/services/analytics-service/internal/models"
	"github.com/conveer/conveer/services/analytics-service/internal/service"

	"github.com/gin-gonic/gin"
)

// GetOverallAnalyticsHTTP получает общую аналитику через HTTP
func (h *AnalyticsHandler) GetOverallAnalyticsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/overall", time.Since(start).Seconds(), c.Writer.Status())
	}()

	// Парсим параметры дат
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	if startDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			endDate = parsed
		}
	}

	analytics, err := h.analyticsService.GetOverallAnalytics(c, startDate, endDate)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get overall analytics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetPlatformAnalyticsHTTP получает аналитику по платформе через HTTP
func (h *AnalyticsHandler) GetPlatformAnalyticsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/platform", time.Since(start).Seconds(), c.Writer.Status())
	}()

	platform := c.Param("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Platform is required"})
		return
	}

	analytics, err := h.analyticsService.GetPlatformAnalytics(c, platform)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get platform analytics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get platform analytics"})
		return
	}

	c.JSON(http.StatusOK, analytics)
}

// GetExpenseForecastHTTP получает прогноз расходов через HTTP
func (h *AnalyticsHandler) GetExpenseForecastHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/forecast/expenses", time.Since(start).Seconds(), c.Writer.Status())
	}()

	period := c.Query("period")
	if period == "" {
		period = "7d"
	}

	forecast, err := h.analyticsService.GetExpenseForecast(c, period)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get expense forecast")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get expense forecast"})
		return
	}

	if forecast.ExpenseForecast == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No forecast available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"period":         forecast.ExpenseForecast.Period,
		"predicted_cost": forecast.ExpenseForecast.PredictedCost,
		"upper_bound":    forecast.ExpenseForecast.UpperBound,
		"lower_bound":    forecast.ExpenseForecast.LowerBound,
		"breakdown":      forecast.ExpenseForecast.Breakdown,
		"confidence":     forecast.Confidence,
		"generated_at":   forecast.GeneratedAt,
	})
}

// GetReadinessForecastHTTP получает прогноз готовности через HTTP
func (h *AnalyticsHandler) GetReadinessForecastHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/forecast/readiness", time.Since(start).Seconds(), c.Writer.Status())
	}()

	accountID := c.Param("account_id")
	platform := c.Query("platform")

	forecast, err := h.analyticsService.GetAccountReadinessForecast(c, accountID, platform)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get readiness forecast")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get readiness forecast"})
		return
	}

	if forecast.ReadinessForecast == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No forecast available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"account_id":       forecast.ReadinessForecast.AccountID,
		"estimated_days":   forecast.ReadinessForecast.EstimatedDays,
		"completion_date":  forecast.ReadinessForecast.CompletionDate,
		"current_progress": forecast.ReadinessForecast.CurrentProgress,
	})
}

// GetOptimalTimeHTTP получает оптимальное время регистрации через HTTP
func (h *AnalyticsHandler) GetOptimalTimeHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/forecast/optimal-time", time.Since(start).Seconds(), c.Writer.Status())
	}()

	platform := c.Query("platform")
	if platform == "" {
		platform = "all"
	}

	forecast, err := h.analyticsService.GetOptimalRegistrationTime(c, platform)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get optimal time forecast")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get optimal time forecast"})
		return
	}

	if forecast.OptimalTimeForecast == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No forecast available"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"best_hours":   forecast.OptimalTimeForecast.BestHours,
		"best_days":    forecast.OptimalTimeForecast.BestDays,
		"success_rate": forecast.OptimalTimeForecast.SuccessRate,
		"sample_size":  forecast.OptimalTimeForecast.SampleSize,
	})
}

// GetProxyRankingsHTTP получает рейтинг прокси провайдеров через HTTP
func (h *AnalyticsHandler) GetProxyRankingsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/recommendations/proxies", time.Since(start).Seconds(), c.Writer.Status())
	}()

	rankings, err := h.analyticsService.GetProxyProviderRankings(c)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get proxy rankings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get proxy rankings"})
		return
	}

	c.JSON(http.StatusOK, rankings)
}

// GetWarmingRecommendationsHTTP получает рекомендации по сценариям прогрева через HTTP
func (h *AnalyticsHandler) GetWarmingRecommendationsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/recommendations/warming", time.Since(start).Seconds(), c.Writer.Status())
	}()

	platform := c.Param("platform")
	if platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Platform is required"})
		return
	}

	recommendation, err := h.analyticsService.GetWarmingScenarioRecommendations(c, platform)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get warming recommendations")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get warming recommendations"})
		return
	}

	c.JSON(http.StatusOK, recommendation)
}

// GetErrorPatternsHTTP получает анализ паттернов ошибок через HTTP
func (h *AnalyticsHandler) GetErrorPatternsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/recommendations/errors", time.Since(start).Seconds(), c.Writer.Status())
	}()

	daysStr := c.Query("days")
	days := 7
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil {
			days = parsed
		}
	}

	analysis, err := h.analyticsService.GetErrorPatternAnalysis(c, days)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get error pattern analysis")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get error pattern analysis"})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

// GetAlertsHTTP получает активные алерты через HTTP
func (h *AnalyticsHandler) GetAlertsHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/alerts", time.Since(start).Seconds(), c.Writer.Status())
	}()

	unacknowledgedOnly := c.Query("unacknowledged_only") == "true"
	severity := c.Query("severity")

	alerts, err := h.analyticsService.GetActiveAlerts(c, unacknowledgedOnly, severity)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get active alerts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get active alerts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

// AcknowledgeAlertHTTP подтверждает алерт через HTTP
func (h *AnalyticsHandler) AcknowledgeAlertHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("POST", "/alerts/acknowledge", time.Since(start).Seconds(), c.Writer.Status())
	}()

	alertID := c.Param("id")
	if alertID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Alert ID is required"})
		return
	}

	var req struct {
		AcknowledgedBy string `json:"acknowledged_by"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.AcknowledgedBy == "" {
		req.AcknowledgedBy = "user"
	}

	if err := h.analyticsService.AcknowledgeAlert(c, alertID, req.AcknowledgedBy); err != nil {
		h.logger.WithError(err).Error("Failed to acknowledge alert")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to acknowledge alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListAlertRulesHTTP получает список правил алертов через HTTP
func (h *AnalyticsHandler) ListAlertRulesHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("GET", "/rules", time.Since(start).Seconds(), c.Writer.Status())
	}()

	rules, err := h.analyticsService.ListAlertRules(c)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list alert rules")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list alert rules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// CreateAlertRuleHTTP создает правило алерта через HTTP
func (h *AnalyticsHandler) CreateAlertRuleHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("POST", "/rules", time.Since(start).Seconds(), c.Writer.Status())
	}()

	var req struct {
		Name      string  `json:"name" binding:"required"`
		Type      string  `json:"type" binding:"required"`
		Platform  string  `json:"platform"`
		Threshold struct {
			Operator string  `json:"operator" binding:"required"`
			Value    float64 `json:"value" binding:"required"`
		} `json:"threshold" binding:"required"`
		Severity string `json:"severity" binding:"required"`
		Cooldown int    `json:"cooldown"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
		Cooldown: req.Cooldown,
	}

	if rule.Cooldown == 0 {
		rule.Cooldown = 30 // Default 30 minutes
	}

	if err := h.analyticsService.CreateAlertRule(c, rule); err != nil {
		h.logger.WithError(err).Error("Failed to create alert rule")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create alert rule"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":       rule.ID.Hex(),
		"name":     rule.Name,
		"type":     rule.Type,
		"platform": rule.Platform,
		"enabled":  rule.Enabled,
		"threshold": gin.H{
			"operator": rule.Threshold.Operator,
			"value":    rule.Threshold.Value,
		},
		"severity": rule.Severity,
		"cooldown": rule.Cooldown,
	})
}

// UpdateAlertRuleHTTP обновляет правило алерта через HTTP
func (h *AnalyticsHandler) UpdateAlertRuleHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("PUT", "/rules", time.Since(start).Seconds(), c.Writer.Status())
	}()

	ruleID := c.Param("id")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule ID is required"})
		return
	}

	var req struct {
		Enabled   *bool `json:"enabled"`
		Threshold *struct {
			Operator string  `json:"operator"`
			Value    float64 `json:"value"`
		} `json:"threshold"`
		Cooldown *int `json:"cooldown"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var threshold *models.AlertThreshold
	if req.Threshold != nil {
		threshold = &models.AlertThreshold{
			Operator: req.Threshold.Operator,
			Value:    req.Threshold.Value,
		}
	}

	cooldown := 0
	if req.Cooldown != nil {
		cooldown = *req.Cooldown
	}

	if err := h.analyticsService.UpdateAlertRule(c, ruleID, enabled, threshold, cooldown); err != nil {
		h.logger.WithError(err).Error("Failed to update alert rule")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update alert rule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAlertRuleHTTP удаляет правило алерта через HTTP
func (h *AnalyticsHandler) DeleteAlertRuleHTTP(c *gin.Context) {
	start := time.Now()
	defer func() {
		service.RecordHTTPRequest("DELETE", "/rules", time.Since(start).Seconds(), c.Writer.Status())
	}()

	ruleID := c.Param("id")
	if ruleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rule ID is required"})
		return
	}

	if err := h.analyticsService.DeleteAlertRule(c, ruleID); err != nil {
		h.logger.WithError(err).Error("Failed to delete alert rule")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete alert rule"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}