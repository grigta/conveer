package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/services/analytics-service/internal/models"
	"github.com/grigta/conveer/services/analytics-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AlertManager менеджер алертов
type AlertManager struct {
	alertRepo      *repository.AlertRepository
	metricsRepo    *repository.MetricsRepository
	rabbitmq       *messaging.RabbitMQ
	logger         *logger.Logger
	interval       time.Duration
	monthlyBudget  float64
	budgetPeriod   time.Duration
}

// NewAlertManager создает новый менеджер алертов
func NewAlertManager(
	alertRepo *repository.AlertRepository,
	metricsRepo *repository.MetricsRepository,
	rabbitmq *messaging.RabbitMQ,
	logger *logger.Logger,
	monthlyBudget float64,
	budgetPeriod time.Duration,
) *AlertManager {
	return &AlertManager{
		alertRepo:     alertRepo,
		metricsRepo:   metricsRepo,
		rabbitmq:      rabbitmq,
		logger:        logger,
		interval:      1 * time.Minute,
		monthlyBudget: monthlyBudget,
		budgetPeriod:  budgetPeriod,
	}
}

// Run запускает фоновый воркер проверки алертов
func (a *AlertManager) Run(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	// Первоначальная проверка
	if err := a.checkAlerts(ctx); err != nil {
		a.logger.WithError(err).Error("Failed initial alert check")
	}

	for {
		select {
		case <-ticker.C:
			RecordWorkerRun("alert_manager")
			start := time.Now()
			if err := a.checkAlerts(ctx); err != nil {
				a.logger.WithError(err).Error("Failed to check alerts")
				RecordWorkerError("alert_manager")
			} else {
				alertCheckDuration.Observe(time.Since(start).Seconds())
			}
		case <-ctx.Done():
			a.logger.Info("Stopping alert manager")
			return
		}
	}
}

// checkAlerts проверяет все активные правила алертов
func (a *AlertManager) checkAlerts(ctx context.Context) error {
	// Получаем все активные правила
	rules, err := a.alertRepo.GetActiveRules(ctx)
	if err != nil {
		return err
	}

	// Подсчитываем активные алерты по severity
	alertCounts := make(map[string]int)

	for _, rule := range rules {
		// Проверяем cooldown
		if rule.LastFired != nil && time.Since(*rule.LastFired) < time.Duration(rule.Cooldown)*time.Minute {
			continue
		}

		// Получаем текущее значение метрики
		currentValue, err := a.getCurrentMetricValue(ctx, rule)
		if err != nil {
			a.logger.WithError(err).WithField("rule", rule.Name).Error("Failed to get metric value")
			continue
		}

		// Проверяем условие
		if a.evaluateCondition(currentValue, rule.Threshold) {
			// Создаем событие алерта
			alert := &models.AlertEvent{
				RuleID:       rule.ID,
				RuleName:     rule.Name,
				Severity:     rule.Severity,
				Platform:     rule.Platform,
				Message:      a.generateAlertMessage(rule, currentValue),
				CurrentValue: currentValue,
				Threshold:    rule.Threshold.Value,
				FiredAt:      time.Now(),
				Acknowledged: false,
			}

			// Сохраняем в БД
			if err := a.alertRepo.SaveAlertEvent(ctx, alert); err != nil {
				a.logger.WithError(err).Error("Failed to save alert event")
				continue
			}

			// Публикуем в RabbitMQ
			if err := a.publishAlertEvent(ctx, alert); err != nil {
				a.logger.WithError(err).Error("Failed to publish alert event")
			}

			// Обновляем LastFired
			now := time.Now()
			rule.LastFired = &now
			if err := a.alertRepo.UpdateRuleField(ctx, rule.ID, "last_fired", now); err != nil {
				a.logger.WithError(err).Error("Failed to update rule last_fired")
			}

			// Обновляем метрики
			alertsFired.WithLabelValues(rule.Severity, rule.Type, rule.Platform).Inc()
			alertCounts[rule.Severity]++

			a.logger.WithFields(map[string]interface{}{
				"rule":     rule.Name,
				"severity": rule.Severity,
				"value":    currentValue,
				"threshold": rule.Threshold.Value,
			}).Warn("Alert fired")
		}
	}

	// Обновляем gauge метрики активных алертов
	for severity, count := range alertCounts {
		activeAlerts.WithLabelValues(severity).Set(float64(count))
	}

	return nil
}

// getCurrentMetricValue получает текущее значение метрики для правила
func (a *AlertManager) getCurrentMetricValue(ctx context.Context, rule models.AlertRule) (float64, error) {
	// Получаем последние метрики
	metrics, err := a.metricsRepo.GetLatest(ctx, rule.Platform)
	if err != nil {
		return 0, err
	}

	switch rule.Type {
	case "ban_rate":
		return metrics.BanRate, nil
	case "error_rate":
		return metrics.ErrorRate, nil
	case "budget":
		// Получаем процент использования бюджета
		if a.monthlyBudget > 0 {
			// Получаем расходы за период бюджета
			startTime := time.Now().Add(-a.budgetPeriod)
			periodMetrics, err := a.metricsRepo.GetByTimeRange(ctx, rule.Platform, startTime, time.Now())
			if err != nil {
				return 0, err
			}

			var totalSpent float64
			for _, m := range periodMetrics {
				totalSpent += m.TotalSpent
			}

			return (totalSpent / a.monthlyBudget) * 100, nil
		}
		return 0, nil
	case "balance":
		return metrics.SMSBalance, nil
	case "success_rate":
		return metrics.SuccessRate, nil
	case "warming_duration":
		return metrics.AvgWarmingDays, nil
	default:
		return 0, fmt.Errorf("unknown metric type: %s", rule.Type)
	}
}

// evaluateCondition проверяет условие алерта
func (a *AlertManager) evaluateCondition(value float64, threshold models.AlertThreshold) bool {
	switch threshold.Operator {
	case ">":
		return value > threshold.Value
	case ">=":
		return value >= threshold.Value
	case "<":
		return value < threshold.Value
	case "<=":
		return value <= threshold.Value
	case "==":
		return value == threshold.Value
	case "!=":
		return value != threshold.Value
	default:
		return false
	}
}

// generateAlertMessage генерирует сообщение алерта
func (a *AlertManager) generateAlertMessage(rule models.AlertRule, currentValue float64) string {
	platform := rule.Platform
	if platform == "" || platform == "all" {
		platform = "все платформы"
	}

	switch rule.Type {
	case "ban_rate":
		return fmt.Sprintf("⚠️ Высокий процент банов на %s: %.1f%% (порог: %.1f%%)",
			platform, currentValue, rule.Threshold.Value)
	case "error_rate":
		return fmt.Sprintf("🚨 Высокий процент ошибок на %s: %.1f%% (порог: %.1f%%)",
			platform, currentValue, rule.Threshold.Value)
	case "budget":
		return fmt.Sprintf("💰 Превышение бюджета: использовано %.1f%% (порог: %.1f%%)",
			currentValue, rule.Threshold.Value)
	case "balance":
		return fmt.Sprintf("📱 Низкий баланс SMS: %.0f (порог: %.0f)",
			currentValue, rule.Threshold.Value)
	case "success_rate":
		return fmt.Sprintf("📉 Низкая успешность на %s: %.1f%% (порог: %.1f%%)",
			platform, currentValue, rule.Threshold.Value)
	default:
		return fmt.Sprintf("Alert: %s %s %.2f (threshold: %.2f)",
			rule.Name, rule.Threshold.Operator, currentValue, rule.Threshold.Value)
	}
}

// publishAlertEvent публикует событие алерта в RabbitMQ
func (a *AlertManager) publishAlertEvent(ctx context.Context, alert *models.AlertEvent) error {
	// Определяем правильный routing key в зависимости от severity/типа алерта
	var routingKey string
	switch alert.Severity {
	case "critical":
		routingKey = "analytics.manual_intervention"
	case "high":
		if strings.Contains(strings.ToLower(alert.Message), "баланс") {
			routingKey = "sms.balance.low"
		} else {
			routingKey = "analytics.manual_intervention"
		}
	default:
		routingKey = "analytics.manual_intervention"
	}

	// Формируем Type события, который соответствует ожиданиям бота
	eventType := fmt.Sprintf("analytics.alert.%s", alert.Severity)
	if strings.Contains(strings.ToLower(alert.Message), "высокий процент банов") {
		eventType = "analytics.account.banned"
	} else if strings.Contains(strings.ToLower(alert.Message), "ошибок") {
		eventType = "analytics.task.failed"
	} else if strings.Contains(strings.ToLower(alert.Message), "баланс") {
		eventType = "sms.balance.low"
	}

	event := &models.Event{
		Type:      eventType,
		Platform:  alert.Platform,
		Message:   alert.Message,
		Priority:  alert.Severity,
		Timestamp: alert.FiredAt,
		Metadata: map[string]interface{}{
			"alert_id":      alert.ID.Hex(),
			"rule_name":     alert.RuleName,
			"current_value": alert.CurrentValue,
			"threshold":     alert.Threshold,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return a.rabbitmq.Publish("bot.events", routingKey, data)
}

// CreateAlertRule создает новое правило алерта
func (a *AlertManager) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	rule.Enabled = true
	return a.alertRepo.CreateRule(ctx, rule)
}

// UpdateAlertRule обновляет правило алерта
func (a *AlertManager) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return a.alertRepo.UpdateRule(ctx, rule)
}

// DeleteAlertRule удаляет правило алерта
func (a *AlertManager) DeleteAlertRule(ctx context.Context, ruleID string) error {
	id, err := primitive.ObjectIDFromHex(ruleID)
	if err != nil {
		return err
	}
	return a.alertRepo.DeleteRule(ctx, id)
}

// GetAlertRules получает все правила алертов
func (a *AlertManager) GetAlertRules(ctx context.Context) ([]models.AlertRule, error) {
	return a.alertRepo.GetAllRules(ctx)
}

// GetActiveAlerts получает активные алерты (для обратной совместимости)
func (a *AlertManager) GetActiveAlerts(ctx context.Context) ([]models.AlertEvent, error) {
	alerts, err := a.alertRepo.GetActiveAlerts(ctx)
	if err != nil {
		return nil, err
	}

	// Обновляем метрику
	for _, alert := range alerts {
		activeAlerts.WithLabelValues(alert.Severity).Add(1)
	}

	return alerts, nil
}

// GetAlerts получает алерты с поддержкой фильтров
func (a *AlertManager) GetAlerts(ctx context.Context, unacknowledgedOnly bool, severity string) ([]models.AlertEvent, error) {
	alerts, err := a.alertRepo.GetAlerts(ctx, unacknowledgedOnly, severity)
	if err != nil {
		return nil, err
	}

	// Обновляем метрики
	for _, alert := range alerts {
		if !alert.Acknowledged {
			activeAlerts.WithLabelValues(alert.Severity).Add(1)
		}
	}

	return alerts, nil
}

// GetAlertsBySeverity получает алерты по уровню критичности
func (a *AlertManager) GetAlertsBySeverity(ctx context.Context, severity string) ([]models.AlertEvent, error) {
	return a.alertRepo.GetAlertsBySeverity(ctx, severity, 100)
}

// AcknowledgeAlert подтверждает алерт
func (a *AlertManager) AcknowledgeAlert(ctx context.Context, alertID, acknowledgedBy string) error {
	id, err := primitive.ObjectIDFromHex(alertID)
	if err != nil {
		return err
	}

	if err := a.alertRepo.AcknowledgeAlert(ctx, id, acknowledgedBy); err != nil {
		return err
	}

	alertsAcknowledged.Inc()
	return nil
}

// GetAlertSummary получает сводку по алертам
func (a *AlertManager) GetAlertSummary(ctx context.Context) (*models.AlertSummary, error) {
	return a.alertRepo.GetAlertSummary(ctx)
}
