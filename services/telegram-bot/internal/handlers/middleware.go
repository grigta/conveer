package handlers

import (
	"context"
	"log"

	"github.com/grigta/conveer/services/telegram-bot/internal/models"
	"github.com/grigta/conveer/services/telegram-bot/internal/service"
	"github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
)

type contextKey string

const (
	userContextKey contextKey = "telegram_user"
)

func AuthMiddleware(authService service.AuthService, requiredRole string) bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
			var telegramID int64
			var chatID int64

			// Extract telegram ID from update
			if update.Message != nil && update.Message.From != nil {
				telegramID = update.Message.From.ID
				chatID = update.Message.Chat.ID
			} else if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
				telegramID = update.CallbackQuery.From.ID
				chatID = update.CallbackQuery.Message.Chat.ID
			} else {
				// Can't identify user
				return
			}

			// Check access
			hasAccess, err := authService.CheckAccess(ctx, telegramID, requiredRole)
			if err != nil {
				log.Printf("Error checking access for user %d: %v", telegramID, err)
				b.SendMessage(ctx, &botmodels.SendMessageParams{
					ChatID: chatID,
					Text:   "❌ Произошла ошибка при проверке доступа.",
				})
				return
			}

			if !hasAccess {
				b.SendMessage(ctx, &botmodels.SendMessageParams{
					ChatID: chatID,
					Text:   "🚫 Доступ запрещен. Обратитесь к администратору.",
				})
				return
			}

			// Get user and add to context
			user, err := authService.GetUser(ctx, telegramID)
			if err == nil && user != nil {
				ctx = context.WithValue(ctx, userContextKey, user)
			}

			// Call next handler
			next(ctx, b, update)
		}
	}
}

func LoggingMiddleware() bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
			// Log incoming update
			if update.Message != nil {
				if update.Message.Text != "" {
					log.Printf("[TelegramBot] User %d executed command: %s",
						update.Message.From.ID, update.Message.Text)
				}
			} else if update.CallbackQuery != nil {
				log.Printf("[TelegramBot] User %d executed callback: %s",
					update.CallbackQuery.From.ID, update.CallbackQuery.Data)
			}

			// Call next handler
			next(ctx, b, update)
		}
	}
}

func GetUserFromContext(ctx context.Context) (*models.TelegramBotUser, bool) {
	user, ok := ctx.Value(userContextKey).(*models.TelegramBotUser)
	return user, ok
}
