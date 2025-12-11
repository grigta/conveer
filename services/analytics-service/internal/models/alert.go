package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AlertRule правило для генерации алертов
type AlertRule struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Name        string             `bson:"name"`
	Type        string             `bson:"type"` // ban_rate/error_rate/budget/balance
	Platform    string             `bson:"platform,omitempty"` // или "all"
	Enabled     bool               `bson:"enabled"`
	Threshold   AlertThreshold     `bson:"threshold"`
	Severity    string             `bson:"severity"` // critical/warning/info
	Cooldown    int                `bson:"cooldown"` // Минуты между алертами
	LastFired   *time.Time         `bson:"last_fired,omitempty"`
}

// AlertThreshold порог для алерта
type AlertThreshold struct {
	Operator string  `bson:"operator"` // >, <, >=, <=
	Value    float64 `bson:"value"`
}

// AlertEvent событие алерта
type AlertEvent struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	RuleID      primitive.ObjectID `bson:"rule_id"`
	RuleName    string             `bson:"rule_name"`
	Severity    string             `bson:"severity"`
	Platform    string             `bson:"platform,omitempty"`
	Message     string             `bson:"message"`
	CurrentValue float64           `bson:"current_value"`
	Threshold    float64           `bson:"threshold"`
	FiredAt      time.Time         `bson:"fired_at"`
	Acknowledged bool              `bson:"acknowledged"`
	AcknowledgedAt *time.Time       `bson:"acknowledged_at,omitempty"`
	AcknowledgedBy string            `bson:"acknowledged_by,omitempty"`
}

// AlertSummary сводка по алертам
type AlertSummary struct {
	TotalAlerts       int64                    `bson:"total_alerts"`
	UnacknowledgedCount int64                  `bson:"unacknowledged_count"`
	BySeverity        map[string]int64         `bson:"by_severity"`
	ByPlatform        map[string]int64         `bson:"by_platform"`
	RecentAlerts      []AlertEvent             `bson:"recent_alerts"`
}

// AlertCondition условие для проверки алерта
type AlertCondition struct {
	MetricName   string
	CurrentValue float64
	Rule         AlertRule
	Platform     string
}

// Event событие для RabbitMQ
type Event struct {
	Type      string                 `json:"type"`
	Platform  string                 `json:"platform,omitempty"`
	Message   string                 `json:"message"`
	Priority  string                 `json:"priority"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}