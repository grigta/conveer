package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Recommendation представляет рекомендацию
type Recommendation struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Type        string             `bson:"type"` // proxy_provider/warming_scenario/error_pattern
	Priority    string             `bson:"priority"` // high/medium/low
	GeneratedAt time.Time          `bson:"generated_at"`
	ValidUntil  time.Time          `bson:"valid_until"` // TTL

	// Рейтинг прокси-провайдеров
	ProxyRating *ProxyProviderRating `bson:"proxy_rating,omitempty"`

	// Оптимальные сценарии прогрева
	WarmingScenario *WarmingScenarioRecommendation `bson:"warming_scenario,omitempty"`

	// Проблемные паттерны
	ErrorPattern *ErrorPatternAnalysis `bson:"error_pattern,omitempty"`

	ActionItems []string `bson:"action_items"` // Конкретные действия
}

// ProxyProviderRating рейтинг прокси-провайдеров
type ProxyProviderRating struct {
	Rankings []ProviderRank `bson:"rankings"`
}

// ProviderRank рейтинг одного провайдера
type ProviderRank struct {
	Provider     string  `bson:"provider"`
	Score        float64 `bson:"score"` // 0-100
	SuccessRate  float64 `bson:"success_rate"`
	AvgLatency   float64 `bson:"avg_latency"`
	BanRate      float64 `bson:"ban_rate"`
	CostPerAccount float64 `bson:"cost_per_account"`
	Recommendation string `bson:"recommendation"` // use/avoid/monitor
}

// WarmingScenarioRecommendation рекомендация по сценарию прогрева
type WarmingScenarioRecommendation struct {
	Platform         string             `bson:"platform"`
	RecommendedType  string             `bson:"recommended_type"` // basic/advanced/custom
	RecommendedDays  int                `bson:"recommended_days"`
	SuccessRate      float64            `bson:"success_rate"`
	Reasoning        string             `bson:"reasoning"`
}

// ErrorPatternAnalysis анализ паттернов ошибок
type ErrorPatternAnalysis struct {
	Clusters []ErrorCluster `bson:"clusters"`
}

// ErrorCluster кластер ошибок
type ErrorCluster struct {
	Pattern      string   `bson:"pattern"` // Общий паттерн
	Frequency    int64    `bson:"frequency"`
	AffectedPlatforms []string `bson:"affected_platforms"`
	RootCause    string   `bson:"root_cause"` // Предполагаемая причина
	Mitigation   string   `bson:"mitigation"` // Рекомендация
}

// ScenarioEfficiency эффективность сценария
type ScenarioEfficiency struct {
	ScenarioType string  `bson:"scenario_type"`
	Platform     string  `bson:"platform"`
	SuccessRate  float64 `bson:"success_rate"`
	AvgDuration  float64 `bson:"avg_duration"`
	SampleSize   int64   `bson:"sample_size"`
}
