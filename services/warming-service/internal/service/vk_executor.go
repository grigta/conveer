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

type VKExecutor struct {
	BaseExecutor
	client *grpc.ClientConn // VK service gRPC client
	logger logger.Logger
}

func NewVKExecutor(client *grpc.ClientConn, logger logger.Logger) *VKExecutor {
	return &VKExecutor{
		BaseExecutor: BaseExecutor{
			supportedActions: []string{
				"view_profile", "view_feed", "like_post", "subscribe_group",
				"comment_post", "send_message", "create_post",
			},
			actionLimits: map[string]int{
				"like_post":       50,  // per day
				"subscribe_group": 10,  // per day
				"comment_post":    20,  // per day
				"send_message":    10,  // per day
				"create_post":     3,   // per day
			},
		},
		client: client,
		logger: logger,
	}
}

func (e *VKExecutor) ExecuteAction(ctx context.Context, task *models.WarmingTask, actionType string, execCtx *models.ExecutionContext) error {
	e.logger.Info("Executing VK action: %s for task %s", actionType, task.ID.Hex())

	// Get account credentials from VK service
	// credentials, err := e.getAccountCredentials(ctx, task.AccountID)
	// if err != nil {
	//     return fmt.Errorf("failed to get VK credentials: %w", err)
	// }

	start := time.Now()

	var err error
	switch actionType {
	case "view_profile":
		err = e.viewProfile(ctx, execCtx)
	case "view_feed":
		err = e.viewFeed(ctx, execCtx)
	case "like_post":
		err = e.likePost(ctx, execCtx)
	case "subscribe_group":
		err = e.subscribeGroup(ctx, execCtx)
	case "comment_post":
		err = e.commentPost(ctx, execCtx)
	case "send_message":
		err = e.sendMessage(ctx, execCtx)
	case "create_post":
		err = e.createPost(ctx, execCtx)
	default:
		err = fmt.Errorf("unsupported action type: %s", actionType)
	}

	duration := time.Since(start).Milliseconds()
	e.logger.Info("VK action %s completed in %dms", actionType, duration)

	return err
}

func (e *VKExecutor) ValidateAccount(ctx context.Context, accountID primitive.ObjectID) error {
	// Check account status via VK service
	// This would call gRPC method to validate account
	e.logger.Info("Validating VK account %s", accountID.Hex())
	return nil
}

func (e *VKExecutor) viewProfile(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Simulate viewing a random profile
	// 1. Navigate to VK profile page
	// 2. Scroll through the profile (10-30 seconds)
	// 3. Random pauses while "reading"

	scrollDuration := time.Duration(10+rand.Intn(20)) * time.Second
	e.logger.Debug("Viewing VK profile for %v", scrollDuration)

	// Simulate action with delay
	time.Sleep(scrollDuration)

	return nil
}

func (e *VKExecutor) viewFeed(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Simulate browsing the feed
	// 1. Navigate to feed
	// 2. Scroll through posts
	// 3. Random stops to "read" posts

	browsingDuration := time.Duration(30+rand.Intn(60)) * time.Second
	e.logger.Debug("Browsing VK feed for %v", browsingDuration)

	// Simulate browsing with multiple scroll actions
	scrollCount := 5 + rand.Intn(10)
	for i := 0; i < scrollCount; i++ {
		scrollDelay := time.Duration(2+rand.Intn(5)) * time.Second
		time.Sleep(scrollDelay)

		// Sometimes pause to "read"
		if rand.Float64() < 0.3 {
			readDelay := time.Duration(5+rand.Intn(10)) * time.Second
			time.Sleep(readDelay)
		}
	}

	return nil
}

func (e *VKExecutor) likePost(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["like_post"] {
		return fmt.Errorf("daily limit reached for like_post")
	}

	// Simulate liking a post
	// 1. Find a post in feed
	// 2. Click like button
	// 3. Wait for response

	e.logger.Debug("Liking a VK post")

	// Find post (2-5 seconds)
	time.Sleep(time.Duration(2+rand.Intn(3)) * time.Second)

	// Click like (human-like delay)
	time.Sleep(time.Duration(200+rand.Intn(500)) * time.Millisecond)

	return nil
}

func (e *VKExecutor) subscribeGroup(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Check daily limit and progression
	if execCtx.ActionsToday >= e.actionLimits["subscribe_group"] {
		return fmt.Errorf("daily limit reached for subscribe_group")
	}

	// Be more careful in early days
	if execCtx.CurrentDay < 7 && execCtx.ActionsToday >= 2 {
		return fmt.Errorf("too many group subscriptions for early warming stage")
	}

	// Simulate subscribing to a group
	// 1. Find recommended group
	// 2. Visit group page
	// 3. Click subscribe button

	e.logger.Debug("Subscribing to a VK group")

	// Navigate to group (3-5 seconds)
	time.Sleep(time.Duration(3+rand.Intn(2)) * time.Second)

	// Browse group content (5-15 seconds)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Second)

	// Subscribe
	time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)

	return nil
}

func (e *VKExecutor) commentPost(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Only allow comments after day 7
	if execCtx.CurrentDay < 7 {
		return fmt.Errorf("commenting not allowed in early warming stage")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["comment_post"] {
		return fmt.Errorf("daily limit reached for comment_post")
	}

	// Select comment from templates
	comments := []string{
		"👍",
		"Интересно!",
		"Спасибо за информацию",
		"Класс!",
		"Согласен",
		"Отлично",
		"👌",
		"Полезно",
	}

	comment := comments[rand.Intn(len(comments))]

	e.logger.Debug("Commenting on VK post: %s", comment)

	// Find post and open comments (3-5 seconds)
	time.Sleep(time.Duration(3+rand.Intn(2)) * time.Second)

	// Type comment (simulate typing)
	typingDelay := len(comment) * 200 // 200ms per character average
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Submit
	time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)

	return nil
}

func (e *VKExecutor) sendMessage(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Only allow messages after day 14 for basic scenario
	if execCtx.CurrentDay < 14 {
		return fmt.Errorf("messaging not allowed in early warming stage")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["send_message"] {
		return fmt.Errorf("daily limit reached for send_message")
	}

	// Message templates
	messages := []string{
		"Привет!",
		"Как дела?",
		"Добрый день!",
		"Спасибо!",
		"Хорошего дня!",
	}

	message := messages[rand.Intn(len(messages))]

	e.logger.Debug("Sending VK message: %s", message)

	// Open messenger (2-4 seconds)
	time.Sleep(time.Duration(2+rand.Intn(2)) * time.Second)

	// Select contact (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Type message
	typingDelay := len(message) * 250
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Send
	time.Sleep(time.Duration(300+rand.Intn(200)) * time.Millisecond)

	return nil
}

func (e *VKExecutor) createPost(ctx context.Context, execCtx *models.ExecutionContext) error {
	// Only for advanced scenarios after day 21
	if execCtx.CurrentDay < 21 {
		return fmt.Errorf("posting not allowed before day 21")
	}

	// Check daily limit
	if execCtx.ActionsToday >= e.actionLimits["create_post"] {
		return fmt.Errorf("daily limit reached for create_post")
	}

	// Post templates
	posts := []string{
		"Хорошего дня всем! ☀️",
		"Отличная погода сегодня!",
		"Время для кофе ☕",
		"Понедельник - день тяжелый 😅",
		"Пятница! 🎉",
	}

	post := posts[rand.Intn(len(posts))]

	e.logger.Debug("Creating VK post: %s", post)

	// Open post creator (2-3 seconds)
	time.Sleep(time.Duration(2+rand.Intn(1)) * time.Second)

	// Type post
	typingDelay := len(post) * 300
	time.Sleep(time.Duration(typingDelay) * time.Millisecond)

	// Add emoji/formatting (1-2 seconds)
	time.Sleep(time.Duration(1+rand.Intn(1)) * time.Second)

	// Publish
	time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)

	return nil
}
