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

type MaxExecutor struct {
	BaseExecutor
	client *grpc.ClientConn // Max service gRPC client
	logger logger.Logger
}

func NewMaxExecutor(client *grpc.ClientConn, logger logger.Logger) *MaxExecutor {
	return &MaxExecutor{
		BaseExecutor: BaseExecutor{
			supportedActions: []string{
				"read_messages", "send_message", "update_status", "create_chat",
			},
			actionLimits: map[string]int{
				"send_message":  15, // per day
				"update_status": 5,  // per day
				"create_chat":   3,  // per day
			},
		},
		client: client,
		logger: logger,
	}
}

func (e *MaxExecutor) ExecuteAction(ctx context.Context, task *models.WarmingTask, actionType string, execCtx *models.ExecutionContext) error {
	e.logger.Info("Executing Max action: %s for task %s", actionType, task.ID.Hex())

	start := time.Now()

	var err error
	switch actionType {
	case "read_messages":
		err = e.readMessages(ctx, execCtx)
	case "send_message":
		err = e.sendMessage(ctx, execCtx)
	case "update_status":
		err = e.updateStatus(ctx, execCtx)
	case "create_chat":
		err = e.createChat(ctx, execCtx)
	default:
		err = fmt.Errorf("unsupported action type: %s", actionType)
	}

	duration := time.Since(start).Milliseconds()
	e.logger.Info("Max action %s completed in %dms", actionType, duration)

	return err
}

func (e *MaxExecutor) ValidateAccount(ctx context.Context, accountID primitive.ObjectID) error {
	e.logger.Info("Validating Max account %s", accountID.Hex())
	return nil
}

func (e *MaxExecutor) readMessages(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Simulate reading messages
	messageCount := 5 + rand.Intn(10)

	e.logger.Debug("Reading %d Max messages", messageCount)

	// Open chat list (2 seconds)
	time.Sleep(2 * time.Second)

	for i := 0; i < messageCount; i++ {
		// Open chat (1-2 seconds)
		time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

		// Read messages (3-8 seconds)
		readTime := time.Duration(3+rand.Intn(5)) * time.Second
		time.Sleep(readTime)

		// Sometimes scroll up for history
		if rand.Float64() < 0.2 {
			time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)
		}

		// Back to chat list
		time.Sleep(time.Duration(500) * time.Millisecond)
	}

	return nil
}

func (e *MaxExecutor) sendMessage(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["send_message"] {
		return fmt.Errorf("daily limit reached for send_message")
	}

	// Be careful in early days
	if execCtx.CurrentDay < 5 && execCtx.ActionsToday >= 3 {
		return fmt.Errorf("too many messages for early warming stage")
	}

	messages := []string{
		"Привет!",
		"Как дела?",
		"Что нового?",
		"Спасибо!",
		"Понял, хорошо",
		"До связи!",
		"👍",
		"Отлично!",
		"Договорились",
	}

	message := messages[rand.Intn(len(messages))]

	e.logger.Debug("Sending Max message: %s", message)

	// Open chat (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Type message
	typingDelay := len(message) * 180
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Send (300-500ms)
	time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)

	return nil
}

func (e *MaxExecutor) updateStatus(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["update_status"] {
		return fmt.Errorf("daily limit reached for update_status")
	}

	statuses := []string{
		"Онлайн",
		"Занят",
		"Отошел",
		"На встрече",
		"Работаю",
		"Перерыв",
		"В пути",
	}

	status := statuses[rand.Intn(len(statuses))]

	e.logger.Debug("Updating Max status to: %s", status)

	// Open status menu (1 second)
	time.Sleep(time.Second)

	// Select status (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	// Type custom status if needed
	if rand.Float64() < 0.3 {
		customStatus := "🎯 " + status
		typingDelay := len(customStatus) * 150
		time.Sleep(time.Duration(typingDelay) * time.Millisecond)
	}

	// Save (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	return nil
}

func (e *MaxExecutor) createChat(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["create_chat"] {
		return fmt.Errorf("daily limit reached for create_chat")
	}

	// Only create chats after day 5
	if execCtx.CurrentDay < 5 {
		return fmt.Errorf("chat creation not allowed in first 5 days")
	}

	chatNames := []string{
		"Рабочий чат",
		"Проект",
		"Команда",
		"Обсуждение",
		"Встреча",
		"Общий чат",
	}

	chatName := chatNames[rand.Intn(len(chatNames))]

	e.logger.Debug("Creating Max chat: %s", chatName)

	// Open create chat menu (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Type chat name
	typingDelay := len(chatName) * 200
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Add participants (2-4 seconds)
	participantCount := 2 + rand.Intn(3)
	for i := 0; i < participantCount; i++ {
		time.Sleep(time.Duration(1) * time.Second)
	}

	// Create (1 second)
	time.Sleep(time.Second)

	return nil
}
