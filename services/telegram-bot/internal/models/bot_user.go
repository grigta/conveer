package models

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TelegramBotUser represents a Telegram bot user with specific permissions
type TelegramBotUser struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TelegramID       int64              `bson:"telegram_id" json:"telegram_id" validate:"required"`
	TelegramUsername string             `bson:"telegram_username,omitempty" json:"telegram_username,omitempty"`
	FirstName        string             `bson:"first_name,omitempty" json:"first_name,omitempty"`
	LastName         string             `bson:"last_name,omitempty" json:"last_name,omitempty"`
	Role             string             `bson:"role" json:"role" validate:"required,oneof=admin operator viewer"`
	IsActive         bool               `bson:"is_active" json:"is_active"`
	Whitelist        bool               `bson:"whitelist" json:"whitelist"`
	CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}

// User roles
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Errors
var (
	ErrInvalidTelegramID = errors.New("invalid telegram ID")
	ErrInvalidRole       = errors.New("invalid role")
	ErrUserNotFound      = errors.New("user not found")
	ErrAccessDenied      = errors.New("access denied")
)

// Validate validates TelegramBotUser fields
func (u *TelegramBotUser) Validate() error {
	if u.TelegramID == 0 {
		return ErrInvalidTelegramID
	}
	if u.Role != RoleAdmin && u.Role != RoleOperator && u.Role != RoleViewer {
		return ErrInvalidRole
	}
	return nil
}

// HasPermission checks if user has permission for a given role
func (u *TelegramBotUser) HasPermission(requiredRole string) bool {
	if !u.IsActive || !u.Whitelist {
		return false
	}

	// Role hierarchy: admin > operator > viewer
	roleLevel := map[string]int{
		RoleViewer:   1,
		RoleOperator: 2,
		RoleAdmin:    3,
	}

	userLevel, ok1 := roleLevel[u.Role]
	requiredLevel, ok2 := roleLevel[requiredRole]

	if !ok1 || !ok2 {
		return false
	}

	return userLevel >= requiredLevel
}
