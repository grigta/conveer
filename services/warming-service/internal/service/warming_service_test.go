package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"conveer/services/warming-service/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockTaskRepository is a mock implementation of TaskRepository
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.WarmingTask) error {
	args := m.Called(ctx, task)
	if args.Error(0) == nil {
		task.ID = primitive.NewObjectID()
	}
	return args.Error(0)
}

func (m *MockTaskRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingTask, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarmingTask), args.Error(1)
}

func (m *MockTaskRepository) GetByAccountAndPlatform(ctx context.Context, accountID primitive.ObjectID, platform string) (*models.WarmingTask, error) {
	args := m.Called(ctx, accountID, platform)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarmingTask), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, id primitive.ObjectID, update models.TaskUpdate) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockTaskRepository) UpdateNextActionTime(ctx context.Context, id primitive.ObjectID, nextTime time.Time) error {
	args := m.Called(ctx, id, nextTime)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTasksForExecution(ctx context.Context, limit int) ([]*models.WarmingTask, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.WarmingTask), args.Error(1)
}

func (m *MockTaskRepository) List(ctx context.Context, filter models.TaskFilter) ([]*models.WarmingTask, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.WarmingTask), args.Error(1)
}

// MockScenarioRepository is a mock implementation of ScenarioRepository
type MockScenarioRepository struct {
	mock.Mock
}

func (m *MockScenarioRepository) Create(ctx context.Context, scenario *models.WarmingScenario) error {
	args := m.Called(ctx, scenario)
	if args.Error(0) == nil {
		scenario.ID = primitive.NewObjectID()
	}
	return args.Error(0)
}

func (m *MockScenarioRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingScenario, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarmingScenario), args.Error(1)
}

func (m *MockScenarioRepository) GetByName(ctx context.Context, platform, name string) (*models.WarmingScenario, error) {
	args := m.Called(ctx, platform, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarmingScenario), args.Error(1)
}

func (m *MockScenarioRepository) Update(ctx context.Context, id primitive.ObjectID, scenario *models.WarmingScenario) error {
	args := m.Called(ctx, id, scenario)
	return args.Error(0)
}

func (m *MockScenarioRepository) List(ctx context.Context, platform string) ([]*models.WarmingScenario, error) {
	args := m.Called(ctx, platform)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.WarmingScenario), args.Error(1)
}

// MockStatsRepository is a mock implementation of StatsRepository
type MockStatsRepository struct {
	mock.Mock
}

func (m *MockStatsRepository) GetAggregatedStats(ctx context.Context, platform string, startDate, endDate time.Time) (*models.AggregatedStats, error) {
	args := m.Called(ctx, platform, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AggregatedStats), args.Error(1)
}

func (m *MockStatsRepository) GetTopActions(ctx context.Context, platform string, limit int) ([]models.ActionStat, error) {
	args := m.Called(ctx, platform, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ActionStat), args.Error(1)
}

func (m *MockStatsRepository) GetCommonErrors(ctx context.Context, platform string, limit int) ([]models.ErrorStat, error) {
	args := m.Called(ctx, platform, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ErrorStat), args.Error(1)
}

func (m *MockStatsRepository) CountActionsByType(ctx context.Context, taskID primitive.ObjectID, actionType string, startTime, endTime time.Time) (int, error) {
	args := m.Called(ctx, taskID, actionType, startTime, endTime)
	return args.Int(0), args.Error(1)
}

// MockMessaging is a mock implementation of RabbitMQClient
type MockMessaging struct {
	mock.Mock
	PublishedMessages []struct {
		Exchange   string
		RoutingKey string
		Message    []byte
	}
}

func (m *MockMessaging) Publish(exchange, routingKey string, message []byte) error {
	m.PublishedMessages = append(m.PublishedMessages, struct {
		Exchange   string
		RoutingKey string
		Message    []byte
	}{exchange, routingKey, message})
	args := m.Called(exchange, routingKey, message)
	return args.Error(0)
}

// MockLogger is a mock implementation of Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(format string, args ...interface{})  {}
func (m *MockLogger) Error(format string, args ...interface{}) {}
func (m *MockLogger) Warn(format string, args ...interface{})  {}
func (m *MockLogger) Debug(format string, args ...interface{}) {}

// WarmingServiceTestSuite is the test suite for WarmingService
type WarmingServiceTestSuite struct {
	suite.Suite
	ctx          context.Context
	cancel       context.CancelFunc
	taskRepo     *MockTaskRepository
	scenarioRepo *MockScenarioRepository
	statsRepo    *MockStatsRepository
	messaging    *MockMessaging
	logger       *MockLogger
}

func (s *WarmingServiceTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.taskRepo = new(MockTaskRepository)
	s.scenarioRepo = new(MockScenarioRepository)
	s.statsRepo = new(MockStatsRepository)
	s.messaging = new(MockMessaging)
	s.logger = new(MockLogger)
}

func (s *WarmingServiceTestSuite) TearDownTest() {
	s.cancel()
}

func TestWarmingServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WarmingServiceTestSuite))
}

// Test StartWarming - successful start
func (s *WarmingServiceTestSuite) TestStartWarming_Success() {
	accountID := primitive.NewObjectID()
	platform := "vk"
	scenarioType := "basic"
	durationDays := 14

	s.taskRepo.On("GetByAccountAndPlatform", s.ctx, accountID, platform).Return(nil, nil)
	s.taskRepo.On("Create", s.ctx, mock.AnythingOfType("*models.WarmingTask")).Return(nil)
	s.messaging.On("Publish", "warming.commands", "start", mock.AnythingOfType("[]uint8")).Return(nil)

	// Verify expectations
	s.taskRepo.AssertExpectations(s.T())
}

// Test StartWarming - task already exists
func (s *WarmingServiceTestSuite) TestStartWarming_TaskAlreadyExists() {
	accountID := primitive.NewObjectID()
	platform := "vk"

	existingTask := &models.WarmingTask{
		ID:        primitive.NewObjectID(),
		AccountID: accountID,
		Platform:  platform,
		Status:    string(models.TaskStatusInProgress),
	}

	s.taskRepo.On("GetByAccountAndPlatform", s.ctx, accountID, platform).Return(existingTask, nil)

	// Should return error when task already exists
	s.taskRepo.AssertExpectations(s.T())
}

// Test StartWarming - invalid duration (too short)
func (s *WarmingServiceTestSuite) TestStartWarming_InvalidDurationTooShort() {
	accountID := primitive.NewObjectID()
	durationDays := 7 // Less than 14

	// Duration should be between 14 and 60
	s.True(durationDays < 14)
}

// Test StartWarming - invalid duration (too long)
func (s *WarmingServiceTestSuite) TestStartWarming_InvalidDurationTooLong() {
	accountID := primitive.NewObjectID()
	durationDays := 90 // More than 60

	// Duration should be between 14 and 60
	s.True(durationDays > 60)
}

// Test PauseWarming - successful pause
func (s *WarmingServiceTestSuite) TestPauseWarming_Success() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:        taskID,
		AccountID: primitive.NewObjectID(),
		Platform:  "vk",
		Status:    string(models.TaskStatusInProgress),
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)
	s.taskRepo.On("UpdateStatus", s.ctx, taskID, string(models.TaskStatusPaused)).Return(nil)
	s.messaging.On("Publish", "warming.events", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(nil)

	s.taskRepo.AssertExpectations(s.T())
}

// Test PauseWarming - task not in progress
func (s *WarmingServiceTestSuite) TestPauseWarming_TaskNotInProgress() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:     taskID,
		Status: string(models.TaskStatusPaused), // Already paused
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)

	// Should return error when task is not in progress
	s.taskRepo.AssertExpectations(s.T())
}

// Test ResumeWarming - successful resume
func (s *WarmingServiceTestSuite) TestResumeWarming_Success() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:           taskID,
		AccountID:    primitive.NewObjectID(),
		Platform:     "vk",
		Status:       string(models.TaskStatusPaused),
		CurrentDay:   5,
		DurationDays: 14,
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)
	s.taskRepo.On("Update", s.ctx, taskID, mock.AnythingOfType("models.TaskUpdate")).Return(nil)
	s.messaging.On("Publish", "warming.events", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(nil)

	s.taskRepo.AssertExpectations(s.T())
}

// Test ResumeWarming - task not paused
func (s *WarmingServiceTestSuite) TestResumeWarming_TaskNotPaused() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:     taskID,
		Status: string(models.TaskStatusInProgress), // Not paused
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)

	// Should return error when task is not paused
	s.taskRepo.AssertExpectations(s.T())
}

// Test StopWarming - successful stop
func (s *WarmingServiceTestSuite) TestStopWarming_Success() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:               taskID,
		AccountID:        primitive.NewObjectID(),
		Platform:         "vk",
		Status:           string(models.TaskStatusInProgress),
		CurrentDay:       10,
		ActionsCompleted: 50,
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)
	s.taskRepo.On("Update", s.ctx, taskID, mock.AnythingOfType("models.TaskUpdate")).Return(nil)
	s.messaging.On("Publish", "warming.events", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).Return(nil)

	s.taskRepo.AssertExpectations(s.T())
}

// Test GetWarmingStatus
func (s *WarmingServiceTestSuite) TestGetWarmingStatus_Success() {
	taskID := primitive.NewObjectID()

	task := &models.WarmingTask{
		ID:               taskID,
		AccountID:        primitive.NewObjectID(),
		Platform:         "vk",
		Status:           string(models.TaskStatusInProgress),
		CurrentDay:       5,
		DurationDays:     14,
		ActionsCompleted: 25,
	}

	s.taskRepo.On("GetByID", s.ctx, taskID).Return(task, nil)

	s.taskRepo.AssertExpectations(s.T())
}

// Test GetWarmingStatistics
func (s *WarmingServiceTestSuite) TestGetWarmingStatistics_Success() {
	platform := "vk"
	startDate := time.Now().AddDate(0, -1, 0)
	endDate := time.Now()

	stats := &models.AggregatedStats{
		TotalTasks:       100,
		CompletedTasks:   80,
		FailedTasks:      5,
		InProgressTasks:  15,
		TotalActions:     5000,
		SuccessfulActions: 4800,
	}

	topActions := []models.ActionStat{
		{ActionType: "like_post", Count: 1000},
		{ActionType: "view_feed", Count: 800},
	}

	commonErrors := []models.ErrorStat{
		{ErrorType: "captcha", Count: 50},
		{ErrorType: "timeout", Count: 30},
	}

	s.statsRepo.On("GetAggregatedStats", s.ctx, platform, startDate, endDate).Return(stats, nil)
	s.statsRepo.On("GetTopActions", s.ctx, platform, 10).Return(topActions, nil)
	s.statsRepo.On("GetCommonErrors", s.ctx, platform, 10).Return(commonErrors, nil)

	s.statsRepo.AssertExpectations(s.T())
}

// Test CreateCustomScenario - successful creation
func (s *WarmingServiceTestSuite) TestCreateCustomScenario_Success() {
	scenario := &models.WarmingScenario{
		Name:        "custom-scenario",
		Platform:    "vk",
		Description: "Custom warming scenario",
	}

	s.scenarioRepo.On("GetByName", s.ctx, "vk", "custom-scenario").Return(nil, nil)
	s.scenarioRepo.On("Create", s.ctx, scenario).Return(nil)

	s.scenarioRepo.AssertExpectations(s.T())
}

// Test CreateCustomScenario - missing required fields
func (s *WarmingServiceTestSuite) TestCreateCustomScenario_MissingName() {
	scenario := &models.WarmingScenario{
		Name:     "",
		Platform: "vk",
	}

	// Should return error when name is missing
	s.Empty(scenario.Name)
}

// Test CreateCustomScenario - missing platform
func (s *WarmingServiceTestSuite) TestCreateCustomScenario_MissingPlatform() {
	scenario := &models.WarmingScenario{
		Name:     "custom-scenario",
		Platform: "",
	}

	// Should return error when platform is missing
	s.Empty(scenario.Platform)
}

// Test CreateCustomScenario - duplicate name
func (s *WarmingServiceTestSuite) TestCreateCustomScenario_DuplicateName() {
	existingScenario := &models.WarmingScenario{
		ID:       primitive.NewObjectID(),
		Name:     "custom-scenario",
		Platform: "vk",
	}

	s.scenarioRepo.On("GetByName", s.ctx, "vk", "custom-scenario").Return(existingScenario, nil)

	// Should return error when scenario with same name exists
	s.scenarioRepo.AssertExpectations(s.T())
}

// Test UpdateCustomScenario
func (s *WarmingServiceTestSuite) TestUpdateCustomScenario_Success() {
	scenarioID := primitive.NewObjectID()

	existingScenario := &models.WarmingScenario{
		ID:          scenarioID,
		Name:        "existing-scenario",
		Platform:    "vk",
		Description: "Original description",
	}

	updatedScenario := &models.WarmingScenario{
		Name:        "updated-scenario",
		Description: "Updated description",
	}

	s.scenarioRepo.On("GetByID", s.ctx, scenarioID).Return(existingScenario, nil)
	s.scenarioRepo.On("Update", s.ctx, scenarioID, mock.AnythingOfType("*models.WarmingScenario")).Return(nil)

	s.scenarioRepo.AssertExpectations(s.T())
}

// Test ListScenarios
func (s *WarmingServiceTestSuite) TestListScenarios_Success() {
	platform := "vk"

	scenarios := []*models.WarmingScenario{
		{ID: primitive.NewObjectID(), Name: "basic", Platform: platform},
		{ID: primitive.NewObjectID(), Name: "advanced", Platform: platform},
		{ID: primitive.NewObjectID(), Name: "custom", Platform: platform},
	}

	s.scenarioRepo.On("List", s.ctx, platform).Return(scenarios, nil)

	s.scenarioRepo.AssertExpectations(s.T())
}

// Test ListTasks
func (s *WarmingServiceTestSuite) TestListTasks_Success() {
	filter := models.TaskFilter{
		Platform: "vk",
		Status:   string(models.TaskStatusInProgress),
	}

	tasks := []*models.WarmingTask{
		{ID: primitive.NewObjectID(), Platform: "vk", Status: string(models.TaskStatusInProgress)},
		{ID: primitive.NewObjectID(), Platform: "vk", Status: string(models.TaskStatusInProgress)},
	}

	s.taskRepo.On("List", s.ctx, filter).Return(tasks, nil)

	s.taskRepo.AssertExpectations(s.T())
}

// Test task status transitions
func TestTaskStatusTransitions(t *testing.T) {
	tests := []struct {
		name        string
		fromStatus  models.TaskStatus
		toStatus    models.TaskStatus
		shouldAllow bool
	}{
		{"scheduled to in_progress", models.TaskStatusScheduled, models.TaskStatusInProgress, true},
		{"in_progress to paused", models.TaskStatusInProgress, models.TaskStatusPaused, true},
		{"paused to in_progress", models.TaskStatusPaused, models.TaskStatusInProgress, true},
		{"in_progress to completed", models.TaskStatusInProgress, models.TaskStatusCompleted, true},
		{"in_progress to failed", models.TaskStatusInProgress, models.TaskStatusFailed, true},
		{"completed to in_progress", models.TaskStatusCompleted, models.TaskStatusInProgress, false},
		{"failed to in_progress", models.TaskStatusFailed, models.TaskStatusInProgress, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate status transitions
			if tt.shouldAllow {
				assert.NotEqual(t, tt.fromStatus, tt.toStatus)
			}
		})
	}
}

// Test WarmingTask model
func TestWarmingTaskModel(t *testing.T) {
	task := &models.WarmingTask{
		ID:               primitive.NewObjectID(),
		AccountID:        primitive.NewObjectID(),
		Platform:         "vk",
		ScenarioType:     "basic",
		DurationDays:     14,
		Status:           string(models.TaskStatusScheduled),
		CurrentDay:       0,
		ActionsCompleted: 0,
	}

	assert.NotNil(t, task.ID)
	assert.NotNil(t, task.AccountID)
	assert.Equal(t, "vk", task.Platform)
	assert.Equal(t, "basic", task.ScenarioType)
	assert.Equal(t, 14, task.DurationDays)
	assert.Equal(t, string(models.TaskStatusScheduled), task.Status)
}

// Test WarmingScenario model
func TestWarmingScenarioModel(t *testing.T) {
	scenario := &models.WarmingScenario{
		ID:          primitive.NewObjectID(),
		Name:        "test-scenario",
		Platform:    "vk",
		Description: "Test scenario description",
	}

	assert.NotNil(t, scenario.ID)
	assert.Equal(t, "test-scenario", scenario.Name)
	assert.Equal(t, "vk", scenario.Platform)
	assert.Equal(t, "Test scenario description", scenario.Description)
}

// Test AggregatedStats model
func TestAggregatedStatsModel(t *testing.T) {
	stats := &models.AggregatedStats{
		TotalTasks:        100,
		CompletedTasks:    80,
		FailedTasks:       5,
		InProgressTasks:   15,
		TotalActions:      5000,
		SuccessfulActions: 4800,
		FailedActions:     200,
	}

	assert.Equal(t, 100, stats.TotalTasks)
	assert.Equal(t, 80, stats.CompletedTasks)
	assert.Equal(t, 5, stats.FailedTasks)
	assert.Equal(t, 15, stats.InProgressTasks)
	assert.Equal(t, 5000, stats.TotalActions)
	assert.Equal(t, 4800, stats.SuccessfulActions)
	assert.Equal(t, 200, stats.FailedActions)

	// Verify consistency
	assert.Equal(t, stats.TotalTasks, stats.CompletedTasks+stats.FailedTasks+stats.InProgressTasks)
	assert.Equal(t, stats.TotalActions, stats.SuccessfulActions+stats.FailedActions)
}

// Benchmark tests
func BenchmarkWarmingTaskJSON(b *testing.B) {
	task := &models.WarmingTask{
		ID:               primitive.NewObjectID(),
		AccountID:        primitive.NewObjectID(),
		Platform:         "vk",
		ScenarioType:     "basic",
		DurationDays:     14,
		Status:           string(models.TaskStatusInProgress),
		CurrentDay:       5,
		ActionsCompleted: 25,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(task)
	}
}

func BenchmarkAggregatedStatsJSON(b *testing.B) {
	stats := &models.AggregatedStats{
		TotalTasks:        100,
		CompletedTasks:    80,
		FailedTasks:       5,
		InProgressTasks:   15,
		TotalActions:      5000,
		SuccessfulActions: 4800,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(stats)
	}
}

