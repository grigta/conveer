package models

import "errors"

// Common errors
var (
	ErrInvalidTelegramID = errors.New("invalid telegram ID")
	ErrInvalidRole       = errors.New("invalid role")
	ErrUserNotFound      = errors.New("user not found")
	ErrAccessDenied      = errors.New("access denied")
)