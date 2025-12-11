package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WarmingSchedule struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID       primitive.ObjectID `bson:"task_id" json:"task_id"`
	Day          int                `bson:"day" json:"day"`
	PlannedActions []PlannedAction   `bson:"planned_actions" json:"planned_actions"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}

type PlannedAction struct {
	ActionType   string    `bson:"action_type" json:"action_type"`
	ScheduledAt  time.Time `bson:"scheduled_at" json:"scheduled_at"`
	TimeWindow   int       `bson:"time_window" json:"time_window"` // Minutes within which action can be executed
	Priority     int       `bson:"priority" json:"priority"`
	Completed    bool      `bson:"completed" json:"completed"`
	CompletedAt  *time.Time `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	SkipReason   string    `bson:"skip_reason,omitempty" json:"skip_reason,omitempty"`
}

type ActivityPattern struct {
	Hour           int     `bson:"hour" json:"hour"`
	ActivityLevel  float64 `bson:"activity_level" json:"activity_level"` // 0.0 to 1.0
	ActionTypes    []string `bson:"action_types" json:"action_types"`
}

type BehaviorPattern struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Platform          string             `bson:"platform" json:"platform"`
	PatternName       string             `bson:"pattern_name" json:"pattern_name"`
	Description       string             `bson:"description" json:"description"`
	WeekdayPatterns   []ActivityPattern  `bson:"weekday_patterns" json:"weekday_patterns"`
	WeekendPatterns   []ActivityPattern  `bson:"weekend_patterns" json:"weekend_patterns"`
	BurstProbability  float64            `bson:"burst_probability" json:"burst_probability"` // Probability of action bursts
	BurstSize         [2]int             `bson:"burst_size" json:"burst_size"` // Min and max actions in a burst
	IdlePeriod        [2]int             `bson:"idle_period" json:"idle_period"` // Min and max minutes of idle time
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
}

type ExecutionContext struct {
	TaskID            primitive.ObjectID     `bson:"task_id" json:"task_id"`
	AccountID         primitive.ObjectID     `bson:"account_id" json:"account_id"`
	Platform          string                 `bson:"platform" json:"platform"`
	CurrentDay        int                    `bson:"current_day" json:"current_day"`
	TotalDays         int                    `bson:"total_days" json:"total_days"`
	ActionsToday      int                    `bson:"actions_today" json:"actions_today"`
	LastActionTime    *time.Time             `bson:"last_action_time,omitempty" json:"last_action_time,omitempty"`
	SessionStartTime  time.Time              `bson:"session_start_time" json:"session_start_time"`
	Credentials       map[string]interface{} `bson:"credentials,omitempty" json:"credentials,omitempty"`
	BrowserProfile    map[string]interface{} `bson:"browser_profile,omitempty" json:"browser_profile,omitempty"`
}
