package service

import (
	"context"
	"fmt"

	"github.com/grigta/conveer/services/warming-service/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PlatformExecutor interface {
	ExecuteAction(ctx context.Context, task *models.WarmingTask, actionType string, context *models.ExecutionContext) error
	ValidateAccount(ctx context.Context, accountID primitive.ObjectID) error
	GetSupportedActions() []string
	GetActionLimits() map[string]int
}

type BaseExecutor struct {
	supportedActions []string
	actionLimits     map[string]int
}

func (b *BaseExecutor) GetSupportedActions() []string {
	return b.supportedActions
}

func (b *BaseExecutor) GetActionLimits() map[string]int {
	return b.actionLimits
}

type ActionResult struct {
	Success      bool
	Error        error
	ErrorType    string
	DurationMs   int64
	ResponseCode int
	Metadata     map[string]interface{}
}

const (
	ErrorTypeCaptcha    = "captcha"
	ErrorTypeBan        = "ban"
	ErrorTypeNetwork    = "network"
	ErrorTypeTimeout    = "timeout"
	ErrorTypeRateLimit  = "rate_limit"
	ErrorTypeAuthFailed = "auth_failed"
	ErrorTypeUnknown    = "unknown"
)

func categorizeError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	switch {
	case contains(errStr, "captcha"):
		return ErrorTypeCaptcha
	case contains(errStr, "ban", "blocked", "suspended"):
		return ErrorTypeBan
	case contains(errStr, "network", "connection", "dns"):
		return ErrorTypeNetwork
	case contains(errStr, "timeout", "deadline"):
		return ErrorTypeTimeout
	case contains(errStr, "rate", "limit", "too many"):
		return ErrorTypeRateLimit
	case contains(errStr, "auth", "login", "credential"):
		return ErrorTypeAuthFailed
	default:
		return ErrorTypeUnknown
	}
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

type ActionContext struct {
	TaskID       primitive.ObjectID
	AccountID    primitive.ObjectID
	Platform     string
	ActionType   string
	Day          int
	Attempt      int
	MaxAttempts  int
	SessionID    string
	BrowserInfo  map[string]interface{}
	Credentials  map[string]interface{}
}

func (ac *ActionContext) ShouldRetry() bool {
	return ac.Attempt < ac.MaxAttempts
}

func (ac *ActionContext) IncrementAttempt() {
	ac.Attempt++
}

type ActionExecutionError struct {
	Type        string
	Message     string
	Retryable   bool
	ShouldPause bool
	ShouldStop  bool
}

func (e *ActionExecutionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func NewCaptchaError(message string) *ActionExecutionError {
	return &ActionExecutionError{
		Type:        ErrorTypeCaptcha,
		Message:     message,
		Retryable:   false,
		ShouldPause: true,
		ShouldStop:  false,
	}
}

func NewBanError(message string) *ActionExecutionError {
	return &ActionExecutionError{
		Type:        ErrorTypeBan,
		Message:     message,
		Retryable:   false,
		ShouldPause: false,
		ShouldStop:  true,
	}
}

func NewNetworkError(message string) *ActionExecutionError {
	return &ActionExecutionError{
		Type:        ErrorTypeNetwork,
		Message:     message,
		Retryable:   true,
		ShouldPause: false,
		ShouldStop:  false,
	}
}

func NewRateLimitError(message string) *ActionExecutionError {
	return &ActionExecutionError{
		Type:        ErrorTypeRateLimit,
		Message:     message,
		Retryable:   true,
		ShouldPause: true,
		ShouldStop:  false,
	}
}
