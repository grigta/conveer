package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WarmingTask struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	AccountID        primitive.ObjectID `bson:"account_id" json:"account_id"`
	Platform         string             `bson:"platform" json:"platform"` // vk, telegram, mail, max
	ScenarioType     string             `bson:"scenario_type" json:"scenario_type"` // basic, advanced, custom
	ScenarioID       primitive.ObjectID `bson:"scenario_id,omitempty" json:"scenario_id,omitempty"`
	DurationDays     int                `bson:"duration_days" json:"duration_days"` // 14-30 or 30-60
	Status           string             `bson:"status" json:"status"` // scheduled, in_progress, paused, completed, failed
	CurrentDay       int                `bson:"current_day" json:"current_day"`
	NextActionAt     *time.Time         `bson:"next_action_at,omitempty" json:"next_action_at,omitempty"`
	ActionsCompleted int                `bson:"actions_completed" json:"actions_completed"`
	ActionsFailed    int                `bson:"actions_failed" json:"actions_failed"`
	LastError        string             `bson:"last_error,omitempty" json:"last_error,omitempty"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
	CompletedAt      *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	Metadata         map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type WarmingTaskStatus string

const (
	TaskStatusScheduled  WarmingTaskStatus = "scheduled"
	TaskStatusInProgress WarmingTaskStatus = "in_progress"
	TaskStatusPaused     WarmingTaskStatus = "paused"
	TaskStatusCompleted  WarmingTaskStatus = "completed"
	TaskStatusFailed     WarmingTaskStatus = "failed"
)

type WarmingPlatform string

const (
	PlatformVK       WarmingPlatform = "vk"
	PlatformTelegram WarmingPlatform = "telegram"
	PlatformMail     WarmingPlatform = "mail"
	PlatformMax      WarmingPlatform = "max"
)

type ScenarioType string

const (
	ScenarioBasic    ScenarioType = "basic"
	ScenarioAdvanced ScenarioType = "advanced"
	ScenarioCustom   ScenarioType = "custom"
)

type TaskFilter struct {
	Platform     string
	Status       string
	AccountID    *primitive.ObjectID
	NextActionAt *time.Time
	Limit        int
	Offset       int
}

type TaskUpdate struct {
	Status           *string
	CurrentDay       *int
	NextActionAt     *time.Time
	ActionsCompleted *int
	ActionsFailed    *int
	LastError        *string
	CompletedAt      *time.Time
}
