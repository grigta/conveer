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

type MailExecutor struct {
	BaseExecutor
	client *grpc.ClientConn // Mail service gRPC client
	logger logger.Logger
}

func NewMailExecutor(client *grpc.ClientConn, logger logger.Logger) *MailExecutor {
	return &MailExecutor{
		BaseExecutor: BaseExecutor{
			supportedActions: []string{
				"read_email", "send_email", "mark_spam",
				"create_folder", "move_email",
			},
			actionLimits: map[string]int{
				"send_email":    10, // per day
				"mark_spam":     5,  // per day
				"create_folder": 3,  // per day
			},
		},
		client: client,
		logger: logger,
	}
}

func (e *MailExecutor) ExecuteAction(ctx context.Context, task *models.WarmingTask, actionType string, execCtx *models.ExecutionContext) error {
	e.logger.Info("Executing Mail action: %s for task %s", actionType, task.ID.Hex())

	start := time.Now()

	var err error
	switch actionType {
	case "read_email":
		err = e.readEmail(ctx, execCtx)
	case "send_email":
		err = e.sendEmail(ctx, execCtx)
	case "mark_spam":
		err = e.markSpam(ctx, execCtx)
	case "create_folder":
		err = e.createFolder(ctx, execCtx)
	case "move_email":
		err = e.moveEmail(ctx, execCtx)
	default:
		err = fmt.Errorf("unsupported action type: %s", actionType)
	}

	duration := time.Since(start).Milliseconds()
	e.logger.Info("Mail action %s completed in %dms", actionType, duration)

	return err
}

func (e *MailExecutor) ValidateAccount(ctx context.Context, accountID primitive.ObjectID) error {
	e.logger.Info("Validating Mail account %s", accountID.Hex())
	return nil
}

func (e *MailExecutor) readEmail(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Simulate reading emails
	emailCount := 3 + rand.Intn(5)

	e.logger.Debug("Reading %d emails", emailCount)

	// Open inbox (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	for i := 0; i < emailCount; i++ {
		// Click on email (1-2 seconds)
		time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

		// Read email (5-15 seconds)
		readTime := time.Duration(5+rand.Intn(10)) * time.Second
		time.Sleep(readTime)

		// Sometimes mark as read/important
		if rand.Float64() < 0.3 {
			time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)
		}

		// Back to inbox
		time.Sleep(time.Duration(500) * time.Millisecond)
	}

	return nil
}

func (e *MailExecutor) sendEmail(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["send_email"] {
		return fmt.Errorf("daily limit reached for send_email")
	}

	// Be careful in early days
	if execCtx.CurrentDay < 7 && execCtx.ActionsToday >= 2 {
		return fmt.Errorf("too many emails for early warming stage")
	}

	// Email templates
	subjects := []string{
		"Тестовое письмо",
		"Привет!",
		"Вопрос",
		"Информация",
		"Запрос",
	}

	bodies := []string{
		"Добрый день!\n\nЭто тестовое сообщение.\n\nС уважением.",
		"Привет!\n\nКак дела?\n\nЖду ответа.",
		"Здравствуйте!\n\nСпасибо за информацию.\n\nС уважением.",
		"Добрый день!\n\nПрошу уточнить детали.\n\nСпасибо.",
		"Привет!\n\nОтправляю запрошенную информацию.\n\nХорошего дня!",
	}

	subject := subjects[rand.Intn(len(subjects))]
	body := bodies[rand.Intn(len(bodies))]

	e.logger.Debug("Sending email with subject: %s", subject)

	// Click compose (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Enter recipient (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Type subject
	typingDelay := len(subject) * 150
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Type body
	typingDelay = len(body) * 100
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Send (1 second)
	time.Sleep(time.Second)

	return nil
}

func (e *MailExecutor) markSpam(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["mark_spam"] {
		return fmt.Errorf("daily limit reached for mark_spam")
	}

	e.logger.Debug("Marking email as spam")

	// Find spam email (2-4 seconds)
	time.Sleep(time.Duration(2+rand.Intn(2)) * time.Second)

	// Select and mark (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	return nil
}

func (e *MailExecutor) createFolder(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["create_folder"] {
		return fmt.Errorf("daily limit reached for create_folder")
	}

	// Only create folders after day 3
	if execCtx.CurrentDay < 3 {
		return fmt.Errorf("folder creation not allowed in first 3 days")
	}

	folderNames := []string{
		"Важное",
		"Работа",
		"Личное",
		"Архив",
		"Проекты",
		"Заметки",
	}

	folderName := folderNames[rand.Intn(len(folderNames))]

	e.logger.Debug("Creating folder: %s", folderName)

	// Open folder menu (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Click create (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	// Type name
	typingDelay := len(folderName) * 200
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Confirm (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	return nil
}

func (e *MailExecutor) moveEmail(ctx context.Context, execCtx *models.ExecutionContext) error {
	e.logger.Debug("Moving email to folder")

	// Select email (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Open move menu (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	// Select folder (1 second)
	time.Sleep(time.Second)

	// Confirm (500ms)
	time.Sleep(time.Duration(500) * time.Millisecond)

	return nil
}
