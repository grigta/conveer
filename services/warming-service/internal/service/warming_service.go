package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"conveer/pkg/cache"
	"conveer/pkg/logger"
	"conveer/pkg/messaging"
	"conveer/services/warming-service/internal/config"
	"conveer/services/warming-service/internal/models"
	"conveer/services/warming-service/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
)

type WarmingService interface {
	StartWarming(ctx context.Context, accountID primitive.ObjectID, platform, scenarioType string, scenarioID *primitive.ObjectID, durationDays int) (*models.WarmingTask, error)
	PauseWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error)
	ResumeWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error)
	StopWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error)
	GetWarmingStatus(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error)
	GetWarmingStatistics(ctx context.Context, platform string, startDate, endDate time.Time) (*models.AggregatedStats, error)
	CreateCustomScenario(ctx context.Context, scenario *models.WarmingScenario) (*models.WarmingScenario, error)
	UpdateCustomScenario(ctx context.Context, scenarioID primitive.ObjectID, scenario *models.WarmingScenario) (*models.WarmingScenario, error)
	ListScenarios(ctx context.Context, platform string) ([]*models.WarmingScenario, error)
	ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.WarmingTask, error)
	StartWorkers(ctx context.Context)
}

type warmingService struct {
	taskRepo        repository.TaskRepository
	scenarioRepo    repository.ScenarioRepository
	statsRepo       repository.StatsRepository
	scheduleRepo    repository.ScheduleRepository
	messaging       *messaging.RabbitMQClient
	cache           *cache.RedisClient
	vkClient        *grpc.ClientConn
	telegramClient  *grpc.ClientConn
	mailClient      *grpc.ClientConn
	maxClient       *grpc.ClientConn
	config          *config.Config
	logger          logger.Logger
	scheduler       *Scheduler
	behaviorSim     *BehaviorSimulator
	platformExecs   map[string]PlatformExecutor
	metrics         *Metrics
}

func NewWarmingService(
	taskRepo repository.TaskRepository,
	scenarioRepo repository.ScenarioRepository,
	statsRepo repository.StatsRepository,
	scheduleRepo repository.ScheduleRepository,
	messaging *messaging.RabbitMQClient,
	cache *cache.RedisClient,
	vkClient, telegramClient, mailClient, maxClient *grpc.ClientConn,
	config *config.Config,
	logger logger.Logger,
) WarmingService {
	ws := &warmingService{
		taskRepo:       taskRepo,
		scenarioRepo:   scenarioRepo,
		statsRepo:      statsRepo,
		scheduleRepo:   scheduleRepo,
		messaging:      messaging,
		cache:          cache,
		vkClient:       vkClient,
		telegramClient: telegramClient,
		mailClient:     mailClient,
		maxClient:      maxClient,
		config:         config,
		logger:         logger,
		metrics:        NewMetrics(),
	}

	// Initialize components
	ws.scheduler = NewScheduler(ws, scheduleRepo, statsRepo, config, logger)
	ws.behaviorSim = NewBehaviorSimulator(config, logger)

	// Initialize platform executors
	ws.platformExecs = map[string]PlatformExecutor{
		"vk":       NewVKExecutor(vkClient, logger),
		"telegram": NewTelegramExecutor(telegramClient, logger),
		"mail":     NewMailExecutor(mailClient, logger),
		"max":      NewMaxExecutor(maxClient, logger),
	}

	return ws
}

func (s *warmingService) StartWarming(ctx context.Context, accountID primitive.ObjectID, platform, scenarioType string, scenarioID *primitive.ObjectID, durationDays int) (*models.WarmingTask, error) {
	// Check if task already exists for this account
	existingTask, err := s.taskRepo.GetByAccountAndPlatform(ctx, accountID, platform)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing task: %w", err)
	}

	if existingTask != nil && existingTask.Status != string(models.TaskStatusCompleted) && existingTask.Status != string(models.TaskStatusFailed) {
		return nil, fmt.Errorf("warming task already exists for this account")
	}

	// Validate duration
	if durationDays < 14 || durationDays > 60 {
		return nil, fmt.Errorf("invalid duration: must be between 14 and 60 days")
	}

	// Create new task
	task := &models.WarmingTask{
		AccountID:    accountID,
		Platform:     platform,
		ScenarioType: scenarioType,
		DurationDays: durationDays,
		Status:       string(models.TaskStatusScheduled),
		CurrentDay:   0,
	}

	// Set ScenarioID only if it's not nil
	if scenarioID != nil {
		task.ScenarioID = *scenarioID
	}

	// Save task to database
	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create warming task: %w", err)
	}

	// Publish start command
	command := map[string]interface{}{
		"task_id":    task.ID.Hex(),
		"account_id": accountID.Hex(),
		"platform":   platform,
	}

	commandJSON, _ := json.Marshal(command)
	if err := s.messaging.Publish("warming.commands", "start", commandJSON); err != nil {
		s.logger.Error("Failed to publish start command: %v", err)
	}

	// Update account status in platform service
	s.updateAccountStatus(ctx, accountID, platform, "warming")

	// Increment metrics
	s.metrics.IncrementTasksTotal(platform, scenarioType, "scheduled")

	// Log event
	s.logger.Info("Started warming task %s for account %s on platform %s", task.ID.Hex(), accountID.Hex(), platform)

	return task, nil
}

func (s *warmingService) PauseWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.Status != string(models.TaskStatusInProgress) {
		return nil, fmt.Errorf("can only pause tasks that are in progress")
	}

	// Update task status
	if err := s.taskRepo.UpdateStatus(ctx, taskID, string(models.TaskStatusPaused)); err != nil {
		return nil, fmt.Errorf("failed to pause task: %w", err)
	}

	// Publish pause event
	s.publishEvent("warming.task.paused", task.Platform, map[string]interface{}{
		"task_id":    taskID.Hex(),
		"account_id": task.AccountID.Hex(),
	})

	task.Status = string(models.TaskStatusPaused)
	return task, nil
}

func (s *warmingService) ResumeWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.Status != string(models.TaskStatusPaused) {
		return nil, fmt.Errorf("can only resume tasks that are paused")
	}

	// Update task status and next action time
	nextActionAt := s.scheduler.CalculateNextActionTime(time.Now(), task.CurrentDay, task.DurationDays)
	update := models.TaskUpdate{
		Status:       stringPtr(string(models.TaskStatusInProgress)),
		NextActionAt: &nextActionAt,
	}

	if err := s.taskRepo.Update(ctx, taskID, update); err != nil {
		return nil, fmt.Errorf("failed to resume task: %w", err)
	}

	// Publish resume event
	s.publishEvent("warming.task.resumed", task.Platform, map[string]interface{}{
		"task_id":    taskID.Hex(),
		"account_id": task.AccountID.Hex(),
	})

	task.Status = string(models.TaskStatusInProgress)
	task.NextActionAt = &nextActionAt
	return task, nil
}

func (s *warmingService) StopWarming(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// Update task status
	now := time.Now()
	update := models.TaskUpdate{
		Status:      stringPtr(string(models.TaskStatusCompleted)),
		CompletedAt: &now,
	}

	if err := s.taskRepo.Update(ctx, taskID, update); err != nil {
		return nil, fmt.Errorf("failed to stop task: %w", err)
	}

	// Update account status in platform service
	s.updateAccountStatus(ctx, task.AccountID, task.Platform, "ready")

	// Publish completion event
	s.publishEvent("warming.task.completed", task.Platform, map[string]interface{}{
		"task_id":         taskID.Hex(),
		"account_id":      task.AccountID.Hex(),
		"duration_days":   task.CurrentDay,
		"actions_completed": task.ActionsCompleted,
	})

	task.Status = string(models.TaskStatusCompleted)
	task.CompletedAt = &now
	return task, nil
}

func (s *warmingService) GetWarmingStatus(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingTask, error) {
	return s.taskRepo.GetByID(ctx, taskID)
}

func (s *warmingService) GetWarmingStatistics(ctx context.Context, platform string, startDate, endDate time.Time) (*models.AggregatedStats, error) {
	stats, err := s.statsRepo.GetAggregatedStats(ctx, platform, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get warming statistics: %w", err)
	}

	// Get top actions
	topActions, err := s.statsRepo.GetTopActions(ctx, platform, 10)
	if err == nil {
		stats.TopActions = topActions
	}

	// Get common errors
	commonErrors, err := s.statsRepo.GetCommonErrors(ctx, platform, 10)
	if err == nil {
		stats.CommonErrors = commonErrors
	}

	return stats, nil
}

func (s *warmingService) CreateCustomScenario(ctx context.Context, scenario *models.WarmingScenario) (*models.WarmingScenario, error) {
	// Validate scenario
	if scenario.Name == "" || scenario.Platform == "" {
		return nil, fmt.Errorf("scenario name and platform are required")
	}

	// Check if scenario with same name exists
	existing, err := s.scenarioRepo.GetByName(ctx, scenario.Platform, scenario.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing scenario: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("scenario with name %s already exists", scenario.Name)
	}

	// Create scenario
	if err := s.scenarioRepo.Create(ctx, scenario); err != nil {
		return nil, fmt.Errorf("failed to create scenario: %w", err)
	}

	s.logger.Info("Created custom scenario %s for platform %s", scenario.Name, scenario.Platform)
	return scenario, nil
}

func (s *warmingService) UpdateCustomScenario(ctx context.Context, scenarioID primitive.ObjectID, scenario *models.WarmingScenario) (*models.WarmingScenario, error) {
	// Get existing scenario
	existing, err := s.scenarioRepo.GetByID(ctx, scenarioID)
	if err != nil {
		return nil, err
	}

	// Update scenario
	existing.Name = scenario.Name
	existing.Description = scenario.Description
	existing.Actions = scenario.Actions
	existing.Schedule = scenario.Schedule

	if err := s.scenarioRepo.Update(ctx, scenarioID, existing); err != nil {
		return nil, fmt.Errorf("failed to update scenario: %w", err)
	}

	return existing, nil
}

func (s *warmingService) ListScenarios(ctx context.Context, platform string) ([]*models.WarmingScenario, error) {
	return s.scenarioRepo.List(ctx, platform)
}

func (s *warmingService) ListTasks(ctx context.Context, filter models.TaskFilter) ([]*models.WarmingTask, error) {
	return s.taskRepo.List(ctx, filter)
}

func (s *warmingService) StartWorkers(ctx context.Context) {
	// Start scheduler worker
	go s.runSchedulerWorker(ctx)

	// Start action executor worker
	go s.runActionExecutorWorker(ctx)

	// Start status sync worker
	go s.runStatusSyncWorker(ctx)

	// Start stuck task monitor
	go s.runStuckTaskMonitor(ctx)

	// Start auto-start consumer
	if s.config.WarmingConfig.EnableAutoStart {
		go s.runAutoStartConsumer(ctx)
	}

	// Start stats aggregator
	go s.runStatsAggregator(ctx)

	s.logger.Info("All warming service workers started")
}

func (s *warmingService) runSchedulerWorker(ctx context.Context) {
	ticker := time.NewTicker(s.config.WarmingConfig.Scheduler.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processScheduledTasks(ctx)
		}
	}
}

func (s *warmingService) processScheduledTasks(ctx context.Context) {
	// Get tasks ready for execution
	tasks, err := s.taskRepo.GetTasksForExecution(ctx, s.config.WarmingConfig.MaxConcurrentTasks)
	if err != nil {
		s.logger.Error("Failed to get tasks for execution: %v", err)
		return
	}

	for _, task := range tasks {
		// Publish execute action command
		command := map[string]interface{}{
			"task_id":    task.ID.Hex(),
			"account_id": task.AccountID.Hex(),
			"platform":   task.Platform,
			"day":        task.CurrentDay,
		}

		commandJSON, _ := json.Marshal(command)
		if err := s.messaging.Publish("warming.commands", "execute_action", commandJSON); err != nil {
			s.logger.Error("Failed to publish execute action command: %v", err)
		}
	}
}

// Helper functions
func (s *warmingService) updateAccountStatus(ctx context.Context, accountID primitive.ObjectID, platform, status string) {
	var err error

	switch platform {
	case "vk":
		if s.vkClient != nil {
			// Create a generic request structure
			req := map[string]string{
				"account_id": accountID.Hex(),
				"status":     status,
			}
			data, _ := json.Marshal(req)

			// Make the gRPC call using the connection
			err = s.vkClient.Invoke(ctx, "/vk.VKService/UpdateAccountStatus", data, nil)
			if err != nil {
				s.logger.Error("Failed to update VK account status: %v", err)
			} else {
				s.logger.Info("Updated VK account %s status to %s", accountID.Hex(), status)
			}
		}

	case "telegram":
		if s.telegramClient != nil {
			req := map[string]string{
				"account_id": accountID.Hex(),
				"status":     status,
			}
			data, _ := json.Marshal(req)

			err = s.telegramClient.Invoke(ctx, "/telegram.TelegramService/UpdateAccountStatus", data, nil)
			if err != nil {
				s.logger.Error("Failed to update Telegram account status: %v", err)
			} else {
				s.logger.Info("Updated Telegram account %s status to %s", accountID.Hex(), status)
			}
		}

	case "mail":
		if s.mailClient != nil {
			req := map[string]string{
				"account_id": accountID.Hex(),
				"status":     status,
			}
			data, _ := json.Marshal(req)

			err = s.mailClient.Invoke(ctx, "/mail.MailService/UpdateAccountStatus", data, nil)
			if err != nil {
				s.logger.Error("Failed to update Mail account status: %v", err)
			} else {
				s.logger.Info("Updated Mail account %s status to %s", accountID.Hex(), status)
			}
		}

	case "max":
		if s.maxClient != nil {
			req := map[string]string{
				"account_id": accountID.Hex(),
				"status":     status,
			}
			data, _ := json.Marshal(req)

			err = s.maxClient.Invoke(ctx, "/max.MaxService/UpdateAccountStatus", data, nil)
			if err != nil {
				s.logger.Error("Failed to update Max account status: %v", err)
			} else {
				s.logger.Info("Updated Max account %s status to %s", accountID.Hex(), status)
			}
		}

	default:
		s.logger.Error("Unknown platform: %s", platform)
	}

	// Update metrics
	if s.metrics != nil {
		if err != nil {
			s.metrics.IncrementFailuresTotal(platform, "update_status")
		} else {
			s.metrics.IncrementSuccessTotal(platform, "update_status")
		}
	}
}

func (s *warmingService) publishEvent(eventType, platform string, data map[string]interface{}) {
	routingKey := fmt.Sprintf("%s.%s", eventType, platform)
	eventData, _ := json.Marshal(data)

	if err := s.messaging.Publish("warming.events", routingKey, eventData); err != nil {
		s.logger.Error("Failed to publish event %s: %v", eventType, err)
	}
}

func stringPtr(s string) *string {
	return &s
}

func randomInRange(min, max int) int {
	if min == max {
		return min
	}
	return rand.Intn(max-min+1) + min
}