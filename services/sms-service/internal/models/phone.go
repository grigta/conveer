package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Phone struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Number       string             `bson:"number" json:"number"`
	CountryCode  string             `bson:"country_code" json:"country_code"`
	Country      string             `bson:"country" json:"country"`
	Operator     string             `bson:"operator" json:"operator"`
	Provider     string             `bson:"provider" json:"provider"`
	Status       PhoneStatus        `bson:"status" json:"status"`
	UserID       string             `bson:"user_id" json:"user_id"`
	Service      string             `bson:"service" json:"service"`
	Price        float64            `bson:"price" json:"price"`
	ActivationID string             `bson:"activation_id" json:"activation_id"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
	ExpiresAt    time.Time          `bson:"expires_at" json:"expires_at"`
	Encrypted    bool               `bson:"encrypted" json:"-"`
}

type PhoneStatus string

const (
	PhoneStatusAvailable PhoneStatus = "available"
	PhoneStatusActive    PhoneStatus = "active"
	PhoneStatusUsed      PhoneStatus = "used"
	PhoneStatusExpired   PhoneStatus = "expired"
	PhoneStatusCancelled PhoneStatus = "cancelled"
)
