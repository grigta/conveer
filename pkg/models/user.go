package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID               primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Email            string                 `bson:"email" json:"email" validate:"required,email"`
	Username         string                 `bson:"username" json:"username" validate:"required,min=3,max=50"`
	Password         string                 `bson:"password" json:"-"`
	FirstName        string                 `bson:"first_name" json:"first_name"`
	LastName         string                 `bson:"last_name" json:"last_name"`
	Role             string                 `bson:"role" json:"role"`
	IsActive         bool                   `bson:"is_active" json:"is_active"`
	IsVerified       bool                   `bson:"is_verified" json:"is_verified"`
	ProfileImage     string                 `bson:"profile_image" json:"profile_image"`
	TelegramID       int64                  `bson:"telegram_id,omitempty" json:"telegram_id,omitempty"`
	TelegramUsername string                 `bson:"telegram_username,omitempty" json:"telegram_username,omitempty"`
	TelegramRole     string                 `bson:"telegram_role,omitempty" json:"telegram_role,omitempty"`
	Metadata         map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt        time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time              `bson:"updated_at" json:"updated_at"`
	LastLoginAt      *time.Time             `bson:"last_login_at" json:"last_login_at"`
}

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleUser     UserRole = "user"
	RoleModerator UserRole = "moderator"
	RoleOperator UserRole = "operator"
	RoleViewer   UserRole = "viewer"
)

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Username  string `json:"username" validate:"required,min=3,max=50"`
	Password  string `json:"password" validate:"required,min=6"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type UpdateUserRequest struct {
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	ProfileImage string                 `json:"profile_image"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int       `json:"expires_in"`
	TokenType    string    `json:"token_type"`
	User         *User     `json:"user"`
}

type Session struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token        string             `bson:"token" json:"token"`
	RefreshToken string             `bson:"refresh_token" json:"refresh_token"`
	UserAgent    string             `bson:"user_agent" json:"user_agent"`
	IPAddress    string             `bson:"ip_address" json:"ip_address"`
	ExpiresAt    time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

type PasswordReset struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token     string             `bson:"token" json:"token"`
	Used      bool               `bson:"used" json:"used"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

type EmailVerification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    primitive.ObjectID `bson:"user_id" json:"user_id"`
	Token     string             `bson:"token" json:"token"`
	Verified  bool               `bson:"verified" json:"verified"`
	ExpiresAt time.Time          `bson:"expires_at" json:"expires_at"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
