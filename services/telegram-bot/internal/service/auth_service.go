package service

import (
	"context"
	"fmt"

	"github.com/conveer/telegram-bot/internal/models"
	"github.com/conveer/telegram-bot/internal/repository"
)

type AuthService interface {
	CheckAccess(ctx context.Context, telegramID int64, requiredRole string) (bool, error)
	RegisterUser(ctx context.Context, telegramID int64, username, firstName, lastName, role string) error
	GetUser(ctx context.Context, telegramID int64) (*models.TelegramBotUser, error)
	UpdateUser(ctx context.Context, telegramID int64, updates map[string]interface{}) error
	ListUsers(ctx context.Context, filter map[string]interface{}) ([]*models.TelegramBotUser, error)
	DeleteUser(ctx context.Context, telegramID int64) error
}

type authService struct {
	userRepo repository.UserRepository
}

func NewAuthService(userRepo repository.UserRepository) AuthService {
	return &authService{
		userRepo: userRepo,
	}
}

func (s *authService) CheckAccess(ctx context.Context, telegramID int64, requiredRole string) (bool, error) {
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		if err == models.ErrUserNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check access: %w", err)
	}

	if !user.IsActive || !user.Whitelist {
		return false, nil
	}

	return user.HasPermission(requiredRole), nil
}

func (s *authService) RegisterUser(ctx context.Context, telegramID int64, username, firstName, lastName, role string) error {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil && err != models.ErrUserNotFound {
		return fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		return fmt.Errorf("user with telegram ID %d already exists", telegramID)
	}

	// Create new user
	user := &models.TelegramBotUser{
		TelegramID:       telegramID,
		TelegramUsername: username,
		FirstName:        firstName,
		LastName:         lastName,
		Role:             role,
		IsActive:         true,
		Whitelist:        true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to register user: %w", err)
	}

	return nil
}

func (s *authService) GetUser(ctx context.Context, telegramID int64) (*models.TelegramBotUser, error) {
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *authService) UpdateUser(ctx context.Context, telegramID int64, updates map[string]interface{}) error {
	if err := s.userRepo.Update(ctx, telegramID, updates); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (s *authService) ListUsers(ctx context.Context, filter map[string]interface{}) ([]*models.TelegramBotUser, error) {
	users, err := s.userRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	return users, nil
}

func (s *authService) DeleteUser(ctx context.Context, telegramID int64) error {
	if err := s.userRepo.Delete(ctx, telegramID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}