// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/grigta/conveer/pkg/testutil"
	"github.com/grigta/conveer/services/warming-service/internal/models"
	"github.com/grigta/conveer/services/warming-service/internal/repository"
)

// WarmingServiceIntegrationSuite tests warming service with real MongoDB
type WarmingServiceIntegrationSuite struct {
	suite.Suite
	ctx            context.Context
	cancel         context.CancelFunc
	mongoContainer *testutil.MongoContainer
	redisContainer *testutil.RedisContainer
	taskRepo       repository.TaskRepository
	scenarioRepo   repository.ScenarioRepository
	statsRepo      repository.StatsRepository
}

func (s *WarmingServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 5*time.Minute)

	var err error

	// Start MongoDB
	s.mongoContainer, err = testutil.NewMongoContainer(s.ctx)
	s.Require().NoError(err, "Failed to start MongoDB container")

	// Start Redis
	s.redisContainer, err = testutil.NewRedisContainer(s.ctx)
	s.Require().NoError(err, "Failed to start Redis container")

	// Initialize repositories
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")

	s.taskRepo = repository.NewTaskRepository(db)
	s.scenarioRepo = repository.NewScenarioRepository(db)
	s.statsRepo = repository.NewStatsRepository(db)
}

func (s *WarmingServiceIntegrationSuite) TearDownSuite() {
	s.cancel()

	if s.mongoContainer != nil {
		_ = s.mongoContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(context.Background())
	}
}

func (s *WarmingServiceIntegrationSuite) SetupTest() {
	// Clean up collections before each test
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")
	_ = db.Collection("warming_tasks").Drop(s.ctx)
	_ = db.Collection("warming_scenarios").Drop(s.ctx)
	_ = db.Collection("warming_stats").Drop(s.ctx)
}

// TestTaskRepository_CRUD tests task CRUD operations
func (s *WarmingServiceIntegrationSuite) TestTaskRepository_CRUD() {
	ctx := s.ctx

	// Create task
	task := &model.WarmingTask{
		AccountID:  "account-123",
		Platform:   "vk",
		ScenarioID: "scenario-1",
		Status:     model.WarmingStatusPending,
		TotalDays:  7,
		CurrentDay: 0,
	}

	err := s.taskRepo.Create(ctx, task)
	s.Require().NoError(err)
	s.NotEmpty(task.ID)

	// Read task
	found, err := s.taskRepo.GetByID(ctx, task.ID)
	s.Require().NoError(err)
	s.Equal(task.AccountID, found.AccountID)
	s.Equal(task.Platform, found.Platform)

	// Update task
	task.Status = model.WarmingStatusRunning
	task.CurrentDay = 1
	task.ActionsCompleted = 5
	err = s.taskRepo.Update(ctx, task)
	s.Require().NoError(err)

	// Verify update
	found, err = s.taskRepo.GetByID(ctx, task.ID)
	s.Require().NoError(err)
	s.Equal(model.WarmingStatusRunning, found.Status)
	s.Equal(1, found.CurrentDay)
	s.Equal(5, found.ActionsCompleted)

	// Delete task
	err = s.taskRepo.Delete(ctx, task.ID)
	s.Require().NoError(err)

	// Verify delete
	found, err = s.taskRepo.GetByID(ctx, task.ID)
	s.Error(err)
	s.Nil(found)
}

// TestTaskRepository_FindByAccountID tests finding task by account
func (s *WarmingServiceIntegrationSuite) TestTaskRepository_FindByAccountID() {
	ctx := s.ctx

	// Create tasks for different accounts
	tasks := []*model.WarmingTask{
		{AccountID: "account-1", Platform: "vk", Status: model.WarmingStatusRunning},
		{AccountID: "account-1", Platform: "telegram", Status: model.WarmingStatusCompleted},
		{AccountID: "account-2", Platform: "vk", Status: model.WarmingStatusRunning},
	}

	for _, t := range tasks {
		err := s.taskRepo.Create(ctx, t)
		s.Require().NoError(err)
	}

	// Find by account
	foundTasks, err := s.taskRepo.FindByAccountID(ctx, "account-1")
	s.Require().NoError(err)
	s.Len(foundTasks, 2)

	// Find active for account
	activeTask, err := s.taskRepo.FindActiveByAccountID(ctx, "account-1")
	s.Require().NoError(err)
	s.NotNil(activeTask)
	s.Equal(model.WarmingStatusRunning, activeTask.Status)
}

// TestTaskRepository_FindByStatus tests finding tasks by status
func (s *WarmingServiceIntegrationSuite) TestTaskRepository_FindByStatus() {
	ctx := s.ctx

	// Create tasks with different statuses
	statuses := []model.WarmingStatus{
		model.WarmingStatusPending,
		model.WarmingStatusRunning,
		model.WarmingStatusRunning,
		model.WarmingStatusPaused,
		model.WarmingStatusCompleted,
	}

	for i, status := range statuses {
		task := &model.WarmingTask{
			AccountID: "account-" + string(rune('0'+i)),
			Platform:  "vk",
			Status:    status,
		}
		err := s.taskRepo.Create(ctx, task)
		s.Require().NoError(err)
	}

	// Find running tasks
	runningTasks, err := s.taskRepo.FindByStatus(ctx, model.WarmingStatusRunning)
	s.Require().NoError(err)
	s.Len(runningTasks, 2)

	// Count active tasks
	activeCount, err := s.taskRepo.CountActive(ctx)
	s.Require().NoError(err)
	s.Equal(int64(3), activeCount) // pending + running + paused
}

// TestScenarioRepository tests scenario operations
func (s *WarmingServiceIntegrationSuite) TestScenarioRepository() {
	ctx := s.ctx

	// Create scenario
	scenario := &model.WarmingScenario{
		Name:        "basic",
		Description: "Basic warming scenario",
		Platform:    "vk",
		Type:        "basic",
		IsDefault:   true,
		DailyActions: []model.DailyActionPlan{
			{
				Day: 1,
				Actions: []model.WarmingAction{
					{Type: "view_profile", Count: 5, Weight: 1.0},
					{Type: "like_post", Count: 3, Weight: 0.8},
				},
			},
			{
				Day: 2,
				Actions: []model.WarmingAction{
					{Type: "view_profile", Count: 10, Weight: 1.0},
					{Type: "like_post", Count: 5, Weight: 0.8},
					{Type: "subscribe", Count: 2, Weight: 0.5},
				},
			},
		},
	}

	err := s.scenarioRepo.Create(ctx, scenario)
	s.Require().NoError(err)
	s.NotEmpty(scenario.ID)

	// Read scenario
	found, err := s.scenarioRepo.GetByID(ctx, scenario.ID)
	s.Require().NoError(err)
	s.Equal(scenario.Name, found.Name)
	s.Len(found.DailyActions, 2)

	// Find by platform
	scenarios, err := s.scenarioRepo.FindByPlatform(ctx, "vk")
	s.Require().NoError(err)
	s.Len(scenarios, 1)

	// Get default scenario
	defaultScenario, err := s.scenarioRepo.GetDefault(ctx, "vk")
	s.Require().NoError(err)
	s.True(defaultScenario.IsDefault)
}

// TestStatsRepository tests statistics operations
func (s *WarmingServiceIntegrationSuite) TestStatsRepository() {
	ctx := s.ctx

	taskID := "task-123"

	// Create stats entries
	stats := []*model.WarmingStat{
		{
			TaskID:     taskID,
			ActionType: "view_profile",
			Success:    true,
			Duration:   1500,
			Day:        1,
		},
		{
			TaskID:     taskID,
			ActionType: "view_profile",
			Success:    true,
			Duration:   1200,
			Day:        1,
		},
		{
			TaskID:     taskID,
			ActionType: "like_post",
			Success:    false,
			Error:      "captcha required",
			Day:        1,
		},
	}

	for _, stat := range stats {
		err := s.statsRepo.Create(ctx, stat)
		s.Require().NoError(err)
	}

	// Get aggregated stats
	aggregated, err := s.statsRepo.GetAggregated(ctx, taskID)
	s.Require().NoError(err)
	s.Equal(3, aggregated.TotalActions)
	s.Equal(2, aggregated.SuccessfulActions)
	s.Equal(1, aggregated.FailedActions)

	// Get stats by action type
	byType, err := s.statsRepo.GetByActionType(ctx, taskID)
	s.Require().NoError(err)
	s.Equal(2, byType["view_profile"])
	s.Equal(1, byType["like_post"])

	// Get top errors
	errors, err := s.statsRepo.GetTopErrors(ctx, taskID, 10)
	s.Require().NoError(err)
	s.Len(errors, 1)
	s.Equal("captcha required", errors[0].Error)
}

// TestTaskWithScheduling tests task scheduling with next action time
func (s *WarmingServiceIntegrationSuite) TestTaskWithScheduling() {
	ctx := s.ctx

	// Create task with next action time
	nextAction := time.Now().Add(5 * time.Minute)
	task := &model.WarmingTask{
		AccountID:    "account-123",
		Platform:     "vk",
		Status:       model.WarmingStatusRunning,
		NextActionAt: &nextAction,
	}

	err := s.taskRepo.Create(ctx, task)
	s.Require().NoError(err)

	// Find tasks due for execution
	now := time.Now().Add(10 * time.Minute) // 10 minutes in the future
	dueTasks, err := s.taskRepo.FindDue(ctx, now)
	s.Require().NoError(err)
	s.Len(dueTasks, 1)

	// Update next action
	newNextAction := time.Now().Add(1 * time.Hour)
	task.NextActionAt = &newNextAction
	err = s.taskRepo.Update(ctx, task)
	s.Require().NoError(err)

	// Verify not in due tasks now
	dueTasks, err = s.taskRepo.FindDue(ctx, now)
	s.Require().NoError(err)
	s.Len(dueTasks, 0)
}

func TestWarmingServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(WarmingServiceIntegrationSuite))
}

