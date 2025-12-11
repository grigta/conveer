package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"conveer/pkg/logger"
	"conveer/services/warming-service/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type TelegramExecutor struct {
	BaseExecutor
	client *grpc.ClientConn // Telegram service gRPC client
	logger logger.Logger
}

func NewTelegramExecutor(client *grpc.ClientConn, logger logger.Logger) *TelegramExecutor {
	return &TelegramExecutor{
		BaseExecutor: BaseExecutor{
			supportedActions: []string{
				"read_channel", "react_message", "join_group",
				"send_message", "comment_post", "create_channel_post",
			},
			actionLimits: map[string]int{
				"join_group":          2,  // per day (first 14 days)
				"join_group_later":    5,  // per day (after 14 days)
				"send_message":        5,  // per day
				"comment_post":        10, // per day
				"create_channel_post": 2,  // per day
				"react_message":       30, // per day
			},
		},
		client: client,
		logger: logger,
	}
}

func (e *TelegramExecutor) ExecuteAction(ctx context.Context, task *models.WarmingTask, actionType string, execCtx *models.ExecutionContext) error {
	e.logger.Info("Executing Telegram action: %s for task %s", actionType, task.ID.Hex())

	start := time.Now()

	var err error
	switch actionType {
	case "read_channel":
		err = e.readChannel(ctx, execCtx)
	case "react_message":
		err = e.reactMessage(ctx, execCtx)
	case "join_group":
		err = e.joinGroup(ctx, execCtx)
	case "send_message":
		err = e.sendMessage(ctx, execCtx)
	case "comment_post":
		err = e.commentPost(ctx, execCtx)
	case "create_channel_post":
		err = e.createChannelPost(ctx, execCtx)
	default:
		err = fmt.Errorf("unsupported action type: %s", actionType)
	}

	duration := time.Since(start).Milliseconds()
	e.logger.Info("Telegram action %s completed in %dms", actionType, duration)

	return err
}

func (e *TelegramExecutor) ValidateAccount(ctx context.Context, accountID primitive.ObjectID) error {
	e.logger.Info("Validating Telegram account %s", accountID.Hex())
	return nil
}

func (e *TelegramExecutor) readChannel(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Simulate reading channel messages
	// 1. Open channel
	// 2. Scroll and read messages
	// 3. Random pauses

	readDuration := time.Duration(30+rand.Intn(30)) * time.Second
	e.logger.Debug("Reading Telegram channel for %v", readDuration)

	// Open channel (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Simulate scrolling and reading
	scrollCount := 3 + rand.Intn(5)
	for i := 0; i < scrollCount; i++ {
		scrollDelay := time.Duration(3+rand.Intn(5)) * time.Second
		time.Sleep(scrollDelay)

		// Sometimes pause to read longer
		if rand.Float64() < 0.4 {
			readDelay := time.Duration(5+rand.Intn(10)) * time.Second
			time.Sleep(readDelay)
		}
	}

	return nil
}

func (e *TelegramExecutor) reactMessage(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["react_message"] {
		return fmt.Errorf("daily limit reached for react_message")
	}

	// Available reactions
	reactions := []string{"👍", "❤️", "🔥", "😁", "👌", "😍", "🎉"}
	reaction := reactions[rand.Intn(len(reactions))]

	e.logger.Debug("Reacting to Telegram message with %s", reaction)

	// Find message (1-3 seconds)
	time.Sleep(time.Duration(1+rand.Intn(2)) * time.Second)

	// React (human-like delay)
	time.Sleep(time.Duration(300+rand.Intn(500)) * time.Millisecond)

	return nil
}

func (e *TelegramExecutor) joinGroup(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Telegram has strict limits for new accounts
	limit := e.actionLimits["join_group"]
	if execCtx.CurrentDay > 14 {
		limit = e.actionLimits["join_group_later"]
	}

	if execCtx.ActionsToday >= limit {
		return fmt.Errorf("daily limit reached for join_group")
	}

	// Very careful in first days
	if execCtx.CurrentDay < 3 && execCtx.ActionsToday >= 1 {
		return fmt.Errorf("too many group joins for very early warming stage")
	}

	e.logger.Debug("Joining Telegram group")

	// Search for group (3-5 seconds)
	time.Sleep(time.Duration(3+rand.Intn(2)) * time.Second)

	// View group info (5-10 seconds)
	time.Sleep(time.Duration(5+rand.Intn(5)) * time.Second)

	// Join
	time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)

	// Read some messages after joining (10-20 seconds)
	time.Sleep(time.Duration(10+rand.Intn(10)) * time.Second)

	return nil
}

func (e *TelegramExecutor) sendMessage(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Telegram restricts DMs for new accounts
	if execCtx.CurrentDay < 14 {
		return fmt.Errorf("direct messaging not allowed before day 14")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["send_message"] {
		return fmt.Errorf("daily limit reached for send_message")
	}

	messages := []string{
		"Привет!",
		"Добрый день",
		"Спасибо за информацию",
		"Интересно",
		"👍",
	}

	message := messages[rand.Intn(len(messages))]

	e.logger.Debug("Sending Telegram message: %s", message)

	// Open chat (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Type message
	typingDelay := len(message) * 200
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Send
	time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)

	return nil
}

func (e *TelegramExecutor) commentPost(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Allow comments after day 7
	if execCtx.CurrentDay < 7 {
		return fmt.Errorf("commenting not allowed before day 7")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["comment_post"] {
		return fmt.Errorf("daily limit reached for comment_post")
	}

	comments := []string{
		"Полезная информация",
		"Спасибо!",
		"👍",
		"Интересно",
		"Класс",
		"+",
		"Согласен",
	}

	comment := comments[rand.Intn(len(comments))]

	e.logger.Debug("Commenting in Telegram group: %s", comment)

	// Find post (2-4 seconds)
	time.Sleep(time.Duration(2+rand.Intn(2)) * time.Second)

	// Type comment
	typingDelay := len(comment) * 250
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Send
	time.Sleep(time.Duration(400+rand.Intn(400)) * time.Millisecond)

	return nil
}

func (e *TelegramExecutor) createChannelPost(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Only for advanced scenarios after day 21
	if execCtx.CurrentDay < 21 {
		return fmt.Errorf("channel posting not allowed before day 21")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["create_channel_post"] {
		return fmt.Errorf("daily limit reached for create_channel_post")
	}

	posts := []string{
		"Доброе утро! ☀️",
		"Хорошего дня!",
		"Интересная статья: [ссылка]",
		"Полезный совет дня...",
		"Мысли вслух...",
	}

	post := posts[rand.Intn(len(posts))]

	e.logger.Debug("Creating Telegram channel post: %s", post)

	// Open channel (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Type post
	typingDelay := len(post) * 300
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Format/preview (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Publish
	time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)

	return nil
}