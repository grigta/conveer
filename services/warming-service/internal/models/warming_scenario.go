package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WarmingScenario struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Name        string                 `bson:"name" json:"name"`
	Description string                 `bson:"description" json:"description"`
	Platform    string                 `bson:"platform" json:"platform"`
	Actions     []ScenarioAction       `bson:"actions" json:"actions"`
	Schedule    ScenarioSchedule       `bson:"schedule" json:"schedule"`
	CreatedBy   string                 `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time              `bson:"updated_at" json:"updated_at"`
	IsActive    bool                   `bson:"is_active" json:"is_active"`
	Metadata    map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type ScenarioAction struct {
	Type   string                 `bson:"type" json:"type"`
	Weight int                    `bson:"weight" json:"weight"`
	Params map[string]interface{} `bson:"params,omitempty" json:"params,omitempty"`
}

type ScenarioSchedule struct {
	Days1_7   DaySchedule `bson:"days_1_7" json:"days_1_7"`
	Days8_14  DaySchedule `bson:"days_8_14" json:"days_8_14"`
	Days15_30 DaySchedule `bson:"days_15_30" json:"days_15_30"`
	Days31_60 DaySchedule `bson:"days_31_60,omitempty" json:"days_31_60,omitempty"`
}

type DaySchedule struct {
	MinActions int              `bson:"min_actions" json:"min_actions"`
	MaxActions int              `bson:"max_actions" json:"max_actions"`
	Actions    []ScenarioAction `bson:"actions" json:"actions"`
}

type Action struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TaskID        primitive.ObjectID `bson:"task_id" json:"task_id"`
	Type          string             `bson:"type" json:"type"`
	Status        string             `bson:"status" json:"status"` // pending, executing, completed, failed
	ScheduledAt   time.Time          `bson:"scheduled_at" json:"scheduled_at"`
	ExecutedAt    *time.Time         `bson:"executed_at,omitempty" json:"executed_at,omitempty"`
	CompletedAt   *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	DurationMs    int64              `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"`
	Error         string             `bson:"error,omitempty" json:"error,omitempty"`
	RetryCount    int                `bson:"retry_count" json:"retry_count"`
	Metadata      map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type ActionType string

const (
	// VK Actions
	ActionVKViewProfile     ActionType = "view_profile"
	ActionVKViewFeed        ActionType = "view_feed"
	ActionVKLikePost        ActionType = "like_post"
	ActionVKSubscribeGroup  ActionType = "subscribe_group"
	ActionVKCommentPost     ActionType = "comment_post"
	ActionVKSendMessage     ActionType = "send_message"
	ActionVKCreatePost      ActionType = "create_post"

	// Telegram Actions
	ActionTelegramReadChannel      ActionType = "read_channel"
	ActionTelegramReactMessage     ActionType = "react_message"
	ActionTelegramJoinGroup        ActionType = "join_group"
	ActionTelegramSendMessage      ActionType = "send_message"
	ActionTelegramCommentPost      ActionType = "comment_post"
	ActionTelegramCreateChannelPost ActionType = "create_channel_post"

	// Mail Actions
	ActionMailReadEmail    ActionType = "read_email"
	ActionMailSendEmail    ActionType = "send_email"
	ActionMailMarkSpam     ActionType = "mark_spam"
	ActionMailCreateFolder ActionType = "create_folder"
	ActionMailMoveEmail    ActionType = "move_email"

	// Max Actions
	ActionMaxReadMessages ActionType = "read_messages"
	ActionMaxSendMessage  ActionType = "send_message"
	ActionMaxUpdateStatus ActionType = "update_status"
	ActionMaxCreateChat   ActionType = "create_chat"
)