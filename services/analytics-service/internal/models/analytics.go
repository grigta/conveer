package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AggregatedMetrics представляет агрегированные метрики за период
type AggregatedMetrics struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	Timestamp        time.Time          `bson:"timestamp"`
	Platform         string             `bson:"platform"` // vk/telegram/mail/max/all

	// Аккаунты
	TotalAccounts    int64              `bson:"total_accounts"`
	AccountsByStatus map[string]int64   `bson:"accounts_by_status"`
	BanRate          float64            `bson:"ban_rate"`    // %
	SuccessRate      float64            `bson:"success_rate"` // %

	// Прогрев
	WarmingActive    int64              `bson:"warming_active"`
	WarmingCompleted int64              `bson:"warming_completed"`
	AvgWarmingDays   float64            `bson:"avg_warming_days"`

	// Расходы
	SMSSpent         float64            `bson:"sms_spent"`   // За период
	ProxySpent       float64            `bson:"proxy_spent"`
	TotalSpent       float64            `bson:"total_spent"`

	// Ресурсы
	ActiveProxies    int64              `bson:"active_proxies"`
	BannedProxies    int64              `bson:"banned_proxies"`
	SMSBalance       float64            `bson:"sms_balance"`

	// Ошибки
	ErrorCount       int64              `bson:"error_count"`
	ErrorRate        float64            `bson:"error_rate"` // %
	TopErrors        []ErrorStat        `bson:"top_errors"` // Top 5

	// Детальная статистика провайдеров и сценариев
	ProxyProviderStats   map[string]*ProxyProviderStat   `bson:"proxy_provider_stats,omitempty"`
	WarmingScenarioStats map[string]*WarmingScenarioStat `bson:"warming_scenario_stats,omitempty"`
}

// ErrorStat представляет статистику по типу ошибки
type ErrorStat struct {
	Type  string `bson:"type"`
	Count int64  `bson:"count"`
}

// TimeSeriesData представляет данные временного ряда
type TimeSeriesData struct {
	Timestamp time.Time `bson:"timestamp"`
	Value     float64   `bson:"value"`
	Platform  string    `bson:"platform,omitempty"`
}

// AccountMetrics метрики аккаунтов из Prometheus
type AccountMetrics struct {
	Total           int64
	ByStatus        map[string]int64
	BanRate         float64
	SuccessRate     float64
	CreatedToday    int64
	BannedToday     int64
}

// ExpenseMetrics метрики расходов
type ExpenseMetrics struct {
	TotalSMS    float64
	TotalProxy  float64
	TotalSpent  float64
	DailyAvg    float64
	WeeklyTotal float64
	MonthlyTotal float64
}

// PerformanceMetrics метрики производительности
type PerformanceMetrics struct {
	AvgWarmingDuration float64
	CompletionRate     float64
	ErrorRate          float64
	ResponseTime       float64
}

// ProxyProviderStat статистика по прокси провайдеру
type ProxyProviderStat struct {
	ActiveProxies  int64   `bson:"active_proxies"`
	BannedProxies  int64   `bson:"banned_proxies"`
	SuccessRate    float64 `bson:"success_rate"`
	BanRate        float64 `bson:"ban_rate"`
	AvgLatency     float64 `bson:"avg_latency"`
	CostPerAccount float64 `bson:"cost_per_account"`
}

// WarmingScenarioStat статистика по сценарию прогрева
type WarmingScenarioStat struct {
	SuccessRate      float64 `bson:"success_rate"`
	AvgDurationDays  float64 `bson:"avg_duration_days"`
	CompletedTasks   int64   `bson:"completed_tasks"`
	FailedTasks      int64   `bson:"failed_tasks"`
	TotalTasks       int64   `bson:"total_tasks"`
}