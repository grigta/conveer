package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ForecastResult представляет результат прогнозирования
type ForecastResult struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Type          string             `bson:"type"` // expense/readiness/optimal_time
	Platform      string             `bson:"platform,omitempty"`
	GeneratedAt   time.Time          `bson:"generated_at"`
	ValidUntil    time.Time          `bson:"valid_until"` // TTL индекс

	// Прогноз расходов (7/30 дней)
	ExpenseForecast *ExpenseForecast `bson:"expense_forecast,omitempty"`

	// Прогноз готовности аккаунтов
	ReadinessForecast *ReadinessForecast `bson:"readiness_forecast,omitempty"`

	// Оптимальное время регистрации
	OptimalTimeForecast *OptimalTimeForecast `bson:"optimal_time_forecast,omitempty"`

	Confidence    float64            `bson:"confidence"` // 0-1
	Model         string             `bson:"model"` // linear_regression/ema/arima
}

// ExpenseForecast прогноз расходов
type ExpenseForecast struct {
	Period        string             `bson:"period"` // 7d/30d
	PredictedCost float64            `bson:"predicted_cost"`
	UpperBound    float64            `bson:"upper_bound"` // 95% CI
	LowerBound    float64            `bson:"lower_bound"`
	Breakdown     map[string]float64 `bson:"breakdown"` // sms/proxy
}

// ReadinessForecast прогноз готовности аккаунтов
type ReadinessForecast struct {
	AccountID       string    `bson:"account_id"`
	EstimatedDays   int       `bson:"estimated_days"`
	CompletionDate  time.Time `bson:"completion_date"`
	CurrentProgress float64   `bson:"current_progress"` // %
}

// OptimalTimeForecast прогноз оптимального времени регистрации
type OptimalTimeForecast struct {
	BestHours    []int     `bson:"best_hours"` // 0-23
	BestDays     []string  `bson:"best_days"`  // Mon-Sun
	SuccessRate  float64   `bson:"success_rate"` // %
	SampleSize   int64     `bson:"sample_size"`
}

// PredictionModel параметры модели прогнозирования
type PredictionModel struct {
	Type       string             `bson:"type"`
	Parameters map[string]float64 `bson:"parameters"`
	R2Score    float64            `bson:"r2_score"`
	MAE        float64            `bson:"mae"` // Mean Absolute Error
	RMSE       float64            `bson:"rmse"` // Root Mean Square Error
	UpdatedAt  time.Time          `bson:"updated_at"`
}