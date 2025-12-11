package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grigta/conveer/pkg/config"
	"github.com/grigta/conveer/pkg/crypto"
	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/pkg/messaging"
	"github.com/grigta/conveer/pkg/middleware"
	"github.com/grigta/conveer/pkg/models"
	"github.com/grigta/conveer/services/auth/internal/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthService struct {
	repo         *repository.AuthRepository
	config       *config.Config
	rabbitmq     *messaging.RabbitMQ
	authMiddleware *middleware.AuthMiddleware
}

func NewAuthService(repo *repository.AuthRepository, cfg *config.Config, mq *messaging.RabbitMQ) *AuthService {
	return &AuthService{
		repo:           repo,
		config:         cfg,
		rabbitmq:       mq,
		authMiddleware: middleware.NewAuthMiddleware(cfg.JWT.Secret),
	}
}

func (s *AuthService) Register(ctx context.Context, req *models.RegisterRequest) (*models.TokenResponse, error) {
	existingUser, _ := s.repo.FindUserByEmail(ctx, req.Email)
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	existingUser, _ = s.repo.FindUserByUsername(ctx, req.Username)
	if existingUser != nil {
		return nil, errors.New("username already taken")
	}

	hashedPassword, err := crypto.HashPassword(req.Password)
	if err != nil {
		logger.Error("Failed to hash password", logger.Field{Key: "error", Value: err.Error()})
		return nil, errors.New("failed to process password")
	}

	user := &models.User{
		ID:         primitive.NewObjectID(),
		Email:      req.Email,
		Username:   req.Username,
		Password:   hashedPassword,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Role:       string(models.RoleUser),
		IsActive:   true,
		IsVerified: false,
		Metadata:   make(map[string]interface{}),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		logger.Error("Failed to create user", logger.Field{Key: "error", Value: err.Error()})
		return nil, errors.New("failed to create user")
	}

	token, err := s.authMiddleware.GenerateToken(user.ID.Hex(), user.Email, user.Role)
	if err != nil {
		logger.Error("Failed to generate token", logger.Field{Key: "error", Value: err.Error()})
		return nil, errors.New("failed to generate token")
	}

	refreshToken := uuid.New().String()

	session := &models.Session{
		ID:           primitive.NewObjectID(),
		UserID:       user.ID,
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		logger.Error("Failed to create session", logger.Field{Key: "error", Value: err.Error()})
	}

	verificationToken := uuid.New().String()
	verification := &models.EmailVerification{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     verificationToken,
		Verified:  false,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateEmailVerification(ctx, verification); err != nil {
		logger.Error("Failed to create email verification", logger.Field{Key: "error", Value: err.Error()})
	}

	if err := s.sendWelcomeEmail(user, verificationToken); err != nil {
		logger.Error("Failed to send welcome email", logger.Field{Key: "error", Value: err.Error()})
	}

	user.Password = ""

	return &models.TokenResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    86400,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.TokenResponse, error) {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !crypto.CheckPassword(req.Password, user.Password) {
		return nil, errors.New("invalid credentials")
	}

	if !user.IsActive {
		return nil, errors.New("account is disabled")
	}

	token, err := s.authMiddleware.GenerateToken(user.ID.Hex(), user.Email, user.Role)
	if err != nil {
		logger.Error("Failed to generate token", logger.Field{Key: "error", Value: err.Error()})
		return nil, errors.New("failed to generate token")
	}

	refreshToken := uuid.New().String()

	session := &models.Session{
		ID:           primitive.NewObjectID(),
		UserID:       user.ID,
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		logger.Error("Failed to create session", logger.Field{Key: "error", Value: err.Error()})
	}

	now := time.Now()
	user.LastLoginAt = &now
	if err := s.repo.UpdateUserLastLogin(ctx, user.ID.Hex()); err != nil {
		logger.Error("Failed to update last login", logger.Field{Key: "error", Value: err.Error()})
	}

	user.Password = ""

	return &models.TokenResponse{
		AccessToken:  token,
		RefreshToken: refreshToken,
		ExpiresIn:    86400,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.repo.DeleteSessionByToken(ctx, token)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenResponse, error) {
	session, err := s.repo.FindSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	if time.Now().After(session.ExpiresAt) {
		s.repo.DeleteSession(ctx, session.ID.Hex())
		return nil, errors.New("refresh token expired")
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID.Hex())
	if err != nil {
		return nil, errors.New("user not found")
	}

	newToken, err := s.authMiddleware.GenerateToken(user.ID.Hex(), user.Email, user.Role)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	newRefreshToken := uuid.New().String()

	session.Token = newToken
	session.RefreshToken = newRefreshToken
	session.ExpiresAt = time.Now().Add(24 * time.Hour)

	if err := s.repo.UpdateSession(ctx, session); err != nil {
		logger.Error("Failed to update session", logger.Field{Key: "error", Value: err.Error()})
		return nil, errors.New("failed to update session")
	}

	user.Password = ""

	return &models.TokenResponse{
		AccessToken:  newToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    86400,
		TokenType:    "Bearer",
		User:         user,
	}, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	verification, err := s.repo.FindEmailVerificationByToken(ctx, token)
	if err != nil {
		return errors.New("invalid verification token")
	}

	if verification.Verified {
		return errors.New("email already verified")
	}

	if time.Now().After(verification.ExpiresAt) {
		return errors.New("verification token expired")
	}

	verification.Verified = true
	if err := s.repo.UpdateEmailVerification(ctx, verification); err != nil {
		return errors.New("failed to verify email")
	}

	if err := s.repo.MarkUserAsVerified(ctx, verification.UserID.Hex()); err != nil {
		logger.Error("Failed to mark user as verified", logger.Field{Key: "error", Value: err.Error()})
	}

	return nil
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil
	}

	resetToken := uuid.New().String()

	passwordReset := &models.PasswordReset{
		ID:        primitive.NewObjectID(),
		UserID:    user.ID,
		Token:     resetToken,
		Used:      false,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreatePasswordReset(ctx, passwordReset); err != nil {
		logger.Error("Failed to create password reset", logger.Field{Key: "error", Value: err.Error()})
		return errors.New("failed to process request")
	}

	if err := s.sendPasswordResetEmail(user, resetToken); err != nil {
		logger.Error("Failed to send password reset email", logger.Field{Key: "error", Value: err.Error()})
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	passwordReset, err := s.repo.FindPasswordResetByToken(ctx, token)
	if err != nil {
		return errors.New("invalid reset token")
	}

	if passwordReset.Used {
		return errors.New("reset token already used")
	}

	if time.Now().After(passwordReset.ExpiresAt) {
		return errors.New("reset token expired")
	}

	hashedPassword, err := crypto.HashPassword(newPassword)
	if err != nil {
		return errors.New("failed to process password")
	}

	if err := s.repo.UpdateUserPassword(ctx, passwordReset.UserID.Hex(), hashedPassword); err != nil {
		return errors.New("failed to update password")
	}

	passwordReset.Used = true
	if err := s.repo.UpdatePasswordReset(ctx, passwordReset); err != nil {
		logger.Error("Failed to mark reset token as used", logger.Field{Key: "error", Value: err.Error()})
	}

	if err := s.repo.DeleteAllUserSessions(ctx, passwordReset.UserID.Hex()); err != nil {
		logger.Error("Failed to delete user sessions", logger.Field{Key: "error", Value: err.Error()})
	}

	return nil
}

func (s *AuthService) sendWelcomeEmail(user *models.User, verificationToken string) error {
	message := map[string]interface{}{
		"type":      "welcome_email",
		"recipient": user.Email,
		"data": map[string]interface{}{
			"username":          user.Username,
			"verification_link": fmt.Sprintf("https://conveer.com/verify-email?token=%s", verificationToken),
		},
	}

	return s.rabbitmq.PublishEvent("notification.email.send", message)
}

func (s *AuthService) sendPasswordResetEmail(user *models.User, resetToken string) error {
	message := map[string]interface{}{
		"type":      "password_reset",
		"recipient": user.Email,
		"data": map[string]interface{}{
			"username":   user.Username,
			"reset_link": fmt.Sprintf("https://conveer.com/reset-password?token=%s", resetToken),
		},
	}

	return s.rabbitmq.PublishEvent("notification.email.send", message)
}
