package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Activation struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	ActivationID     string             `bson:"activation_id" json:"activation_id"`
	UserID           string             `bson:"user_id" json:"user_id"`
	PhoneID          primitive.ObjectID `bson:"phone_id" json:"phone_id"`
	PhoneNumber      string             `bson:"phone_number" json:"phone_number"`
	Service          string             `bson:"service" json:"service"`
	Country          string             `bson:"country" json:"country"`
	Provider         string             `bson:"provider" json:"provider"`
	Status           ActivationStatus   `bson:"status" json:"status"`
	Code             string             `bson:"code" json:"code"`
	FullSMS          string             `bson:"full_sms" json:"full_sms"`
	Price            float64            `bson:"price" json:"price"`
	RefundAmount     float64            `bson:"refund_amount" json:"refund_amount"`
	Refunded         bool               `bson:"refunded" json:"refunded"`
	RetryCount       int                `bson:"retry_count" json:"retry_count"`
	LastRetryAt      *time.Time         `bson:"last_retry_at" json:"last_retry_at"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
	ExpiresAt        time.Time          `bson:"expires_at" json:"expires_at"`
	CompletedAt      *time.Time         `bson:"completed_at" json:"completed_at"`
	CodeReceivedAt   *time.Time         `bson:"code_received_at" json:"code_received_at"`
	CancelledAt      *time.Time         `bson:"cancelled_at" json:"cancelled_at"`
	CancellationNote string             `bson:"cancellation_note" json:"cancellation_note"`
	Encrypted        bool               `bson:"encrypted" json:"-"`
}

type ActivationStatus string

const (
	ActivationStatusPending    ActivationStatus = "pending"
	ActivationStatusWaiting    ActivationStatus = "waiting"
	ActivationStatusReceived   ActivationStatus = "received"
	ActivationStatusCompleted  ActivationStatus = "completed"
	ActivationStatusCancelled  ActivationStatus = "cancelled"
	ActivationStatusExpired    ActivationStatus = "expired"
	ActivationStatusFailed     ActivationStatus = "failed"
)