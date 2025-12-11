package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
)

type BotService interface {
	Start(ctx context.Context) error
	SendMessage(ctx context.Context, chatID int64, text string, opts ...botmodels.SendMessageParams) error
	SendDocument(ctx context.Context, chatID int64, document []byte, filename string) error
	SendAlert(ctx context.Context, userID int64, message string) error
	EditMessage(ctx context.Context, chatID int64, messageID int, text string, opts ...botmodels.EditMessageTextParams) error
	GetBot() *bot.Bot
}

type botService struct {
	bot         *bot.Bot
	authService AuthService
}

func NewBotService(token string, authService AuthService) (BotService, error) {
	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &botService{
		bot:         b,
		authService: authService,
	}, nil
}

func (s *botService) Start(ctx context.Context) error {
	s.bot.Start(ctx)
	return nil
}

func (s *botService) SendMessage(ctx context.Context, chatID int64, text string, opts ...botmodels.SendMessageParams) error {
	params := &botmodels.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: botmodels.ParseModeMarkdown,
	}

	if len(opts) > 0 {
		params = &opts[0]
		params.ChatID = chatID
		params.Text = text
		if params.ParseMode == "" {
			params.ParseMode = botmodels.ParseModeMarkdown
		}
	}

	_, err := s.bot.SendMessage(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (s *botService) SendDocument(ctx context.Context, chatID int64, document []byte, filename string) error {
	params := &botmodels.SendDocumentParams{
		ChatID: chatID,
		Document: &botmodels.InputFileUpload{
			Filename: filename,
			Data:     bot.FileReader(bytes.NewReader(document)),
		},
	}

	_, err := s.bot.SendDocument(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send document: %w", err)
	}

	return nil
}

func (s *botService) SendAlert(ctx context.Context, userID int64, message string) error {
	return s.SendMessage(ctx, userID, message)
}

func (s *botService) EditMessage(ctx context.Context, chatID int64, messageID int, text string, opts ...botmodels.EditMessageTextParams) error {
	params := &botmodels.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: botmodels.ParseModeMarkdown,
	}

	if len(opts) > 0 {
		params = &opts[0]
		params.ChatID = chatID
		params.MessageID = messageID
		params.Text = text
		if params.ParseMode == "" {
			params.ParseMode = botmodels.ParseModeMarkdown
		}
	}

	_, err := s.bot.EditMessageText(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to edit message: %w", err)
	}

	return nil
}

func (s *botService) GetBot() *bot.Bot {
	return s.bot
}

func defaultHandler(ctx context.Context, b *bot.Bot, update *botmodels.Update) {
	// Default handler for unhandled updates
}
