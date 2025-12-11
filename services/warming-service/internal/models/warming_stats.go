package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WarmingStats struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Platform         string             `bson:"platform" json:"platform"`
	Date             time.Time          `bson:"date" json:"date"`
	TotalTasks       int64              `bson:"total_tasks" json:"total_tasks"`
	CompletedTasks   int64              `bson:"completed_tasks" json:"completed_tasks"`
	FailedTasks      int64              `bson:"failed_tasks" json:"failed_tasks"`
	InProgressTasks  int64              `bson:"in_progress_tasks" json:"in_progress_tasks"`
	PausedTasks      int64              `bson:"paused_tasks" json:"paused_tasks"`
	TotalActions     int64              `bson:"total_actions" json:"total_actions"`
	SuccessfulActions int64             `bson:"successful_actions" json:"successful_actions"`
	FailedActions    int64              `bson:"failed_actions" json:"failed_actions"`
	AvgDurationDays  float64            `bson:"avg_duration_days" json:"avg_duration_days"`
	SuccessRate      float64            `bson:"success_rate" json:"success_rate"`
	ByScenarioType   map[string]int64   `bson:"by_scenario_type" json:"by_scenario_type"`
	ByActionType     map[string]int64   `bson:"by_action_type" json:"by_action_type"`
	ErrorTypes       map[string]int64   `bson:"error_types" json:"error_types"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

type WarmingActionLog struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID       primitive.ObjectID `bson:"task_id" json:"task_id"`
	AccountID    primitive.ObjectID `bson:"account_id" json:"account_id"`
	Platform     string             `bson:"platform" json:"platform"`
	ActionType   string             `bson:"action_type" json:"action_type"`
	Status       string             `bson:"status" json:"status"` // success, failed, skipped
	DurationMs   int64              `bson:"duration_ms" json:"duration_ms"`
	Error        string             `bson:"error,omitempty" json:"error,omitempty"`
	ErrorType    string             `bson:"error_type,omitempty" json:"error_type,omitempty"` // captcha, ban, network, timeout
	Timestamp    time.Time          `bson:"timestamp" json:"timestamp"`
	Day          int                `bson:"day" json:"day"`
	SessionID    string             `bson:"session_id" json:"session_id"`
	UserAgent    string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	IPAddress    string             `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	ResponseCode int                `bson:"response_code,omitempty" json:"response_code,omitempty"`
	Metadata     map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type WarmingMetrics struct {
	TasksTotal            map[string]map[string]int64 // platform -> scenario_type -> count
	TasksActive           map[string]int64            // platform -> count
	ActionsTotal          map[string]map[string]int64 // platform -> action_type -> count
	ActionDurationSeconds map[string]map[string][]float64 // platform -> action_type -> durations
	TaskDurationDays      map[string]map[string][]float64 // platform -> scenario_type -> durations
	ErrorsTotal           map[string]map[string]int64 // platform -> error_type -> count
	AccountsReady         map[string]int64            // platform -> count
}

type AggregatedStats struct {
	Platform          string                 `json:"platform"`
	DateRange         DateRange              `json:"date_range"`
	TotalTasks        int64                  `json:"total_tasks"`
	CompletedTasks    int64                  `json:"completed_tasks"`
	FailedTasks       int64                  `json:"failed_tasks"`
	InProgressTasks   int64                  `json:"in_progress_tasks"`
	SuccessRate       float64                `json:"success_rate"`
	AvgDurationDays   float64                `json:"avg_duration_days"`
	ByPlatform        map[string]int64       `json:"by_platform"`
	ByScenario        map[string]int64       `json:"by_scenario"`
	TopActions        []ActionStatistic      `json:"top_actions"`
	CommonErrors      []ErrorStatistic       `json:"common_errors"`
	DailyBreakdown    []DailyStatistic       `json:"daily_breakdown"`
}

type DateRange struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

type ActionStatistic struct {
	ActionType string  `json:"action_type"`
	Count      int64   `json:"count"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration float64 `json:"avg_duration_ms"`
}

type ErrorStatistic struct {
	ErrorType string `json:"error_type"`
	Count     int64  `json:"count"`
	Percentage float64 `json:"percentage"`
}

type DailyStatistic struct {
	Date             time.Time `json:"date"`
	TasksStarted     int64     `json:"tasks_started"`
	TasksCompleted   int64     `json:"tasks_completed"`
	TasksFailed      int64     `json:"tasks_failed"`
	ActionsExecuted  int64     `json:"actions_executed"`
	SuccessRate      float64   `json:"success_rate"`
}
