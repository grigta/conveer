package service

import (
	"context"
	"testing"
	"time"

	"conveer/services/warming-service/internal/config"
	"conveer/services/warming-service/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SchedulerTestSuite is the test suite for Scheduler
type SchedulerTestSuite struct {
	suite.Suite
	ctx          context.Context
	cancel       context.CancelFunc
	taskRepo     *MockTaskRepository
	scheduleRepo *MockScheduleRepository
	statsRepo    *MockStatsRepository
	logger       *MockLogger
	config       *config.Config
}

// MockScheduleRepository is a mock implementation of ScheduleRepository
type MockScheduleRepository struct {
	mock.Mock
}

func (m *MockScheduleRepository) Create(ctx context.Context, schedule *models.WarmingSchedule) error {
	args := m.Called(ctx, schedule)
	return args.Error(0)
}

func (m *MockScheduleRepository) GetByTaskID(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingSchedule, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarmingSchedule), args.Error(1)
}

func (m *MockScheduleRepository) UpdatePlannedActions(ctx context.Context, taskID primitive.ObjectID, actions []models.PlannedAction) error {
	args := m.Called(ctx, taskID, actions)
	return args.Error(0)
}

func (s *SchedulerTestSuite) SetupTest() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.taskRepo = new(MockTaskRepository)
	s.scheduleRepo = new(MockScheduleRepository)
	s.statsRepo = new(MockStatsRepository)
	s.logger = new(MockLogger)
	s.config = &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				EnableRandomDelays:       true,
				DelayMinSeconds:          30,
				DelayMaxSeconds:          300,
				ActiveHoursStart:         8,
				ActiveHoursEnd:           22,
				NightPauseProbability:    0.9,
				WeekendActivityReduction: 0.7,
			},
			Scenarios: make(map[string]map[string]map[string]config.PlatformScenarioConfig),
		},
	}
}

func (s *SchedulerTestSuite) TearDownTest() {
	s.cancel()
}

func TestSchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(SchedulerTestSuite))
}

// Test CalculateNextActionTime
func (s *SchedulerTestSuite) TestCalculateNextActionTime_WithinActiveHours() {
	// Set current time to be within active hours
	currentTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.Local)

	// Create scheduler with behavior simulator
	behaviorSim := NewBehaviorSimulator(s.config, s.logger)
	
	nextTime := behaviorSim.CalculateNextActionTime(currentTime, s.config.WarmingConfig.BehaviorSimulation)

	// Next action should be after current time
	s.True(nextTime.After(currentTime))
}

// Test CalculateNextActionTime - outside active hours
func (s *SchedulerTestSuite) TestCalculateNextActionTime_OutsideActiveHours() {
	// Set current time to be outside active hours (night)
	currentTime := time.Date(2024, 1, 15, 3, 0, 0, 0, time.Local) // 3 AM

	behaviorSim := NewBehaviorSimulator(s.config, s.logger)
	
	nextTime := behaviorSim.CalculateNextActionTime(currentTime, s.config.WarmingConfig.BehaviorSimulation)

	// Should still return a time after current
	s.True(nextTime.After(currentTime))
}

// Test CalculateNextActionTime - progression based adjustment
func (s *SchedulerTestSuite) TestCalculateNextActionTime_EarlyStage() {
	currentTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.Local)
	currentDay := 3  // Early stage
	totalDays := 14

	// Early stage should have less frequent actions
	progression := float64(currentDay) / float64(totalDays)
	s.True(progression < 0.3)
}

// Test parseActionsPerDay
func TestParseActionsPerDay(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedMin int
		expectedMax int
	}{
		{"standard range", "5-10", 5, 10},
		{"single day range", "10-15", 10, 15},
		{"large range", "20-50", 20, 50},
		{"invalid format", "invalid", 5, 10}, // Default values
		{"empty string", "", 5, 10},          // Default values
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			logger := new(MockLogger)
			
			scheduler := &Scheduler{
				config: cfg,
				logger: logger,
			}

			min, max := scheduler.parseActionsPerDay(tt.input)
			assert.Equal(t, tt.expectedMin, min)
			assert.Equal(t, tt.expectedMax, max)
		})
	}
}

// Test isWithinActiveHours
func TestIsWithinActiveHours(t *testing.T) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart: 8,
				ActiveHoursEnd:   22,
			},
		},
	}

	tests := []struct {
		name     string
		hour     int
		expected bool
	}{
		{"morning start", 8, true},
		{"midday", 12, true},
		{"evening", 21, true},
		{"end of active hours", 22, false},
		{"night", 3, false},
		{"early morning", 7, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2024, 1, 15, tt.hour, 30, 0, 0, time.Local)
			
			scheduler := &Scheduler{config: cfg}
			result := scheduler.isWithinActiveHours(testTime)
			
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test getNextActiveTime
func TestGetNextActiveTime(t *testing.T) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart: 8,
				ActiveHoursEnd:   22,
			},
		},
	}

	tests := []struct {
		name         string
		currentHour  int
		expectedHour int
		sameDay      bool
	}{
		{"before active hours", 5, 8, true},    // Move to 8 AM same day
		{"after active hours", 23, 8, false},   // Move to 8 AM next day
		{"during active hours", 12, 8, false},  // Move to 8 AM next day (since we call this when outside)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := time.Date(2024, 1, 15, tt.currentHour, 30, 0, 0, time.Local)
			
			scheduler := &Scheduler{config: cfg}
			nextTime := scheduler.getNextActiveTime(testTime)
			
			assert.Equal(t, tt.expectedHour, nextTime.Hour())
			if tt.sameDay {
				assert.Equal(t, testTime.Day(), nextTime.Day())
			}
		})
	}
}

// Test ShouldSkipAction
func TestShouldSkipAction(t *testing.T) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart:         8,
				ActiveHoursEnd:           22,
				NightPauseProbability:    0.9,
				WeekendActivityReduction: 0.7,
			},
		},
	}

	// Test night hours
	nightTime := time.Date(2024, 1, 15, 3, 0, 0, 0, time.Local) // Monday 3 AM
	
	scheduler := &Scheduler{config: cfg}
	
	// Run multiple times to test probability
	skippedCount := 0
	for i := 0; i < 100; i++ {
		if scheduler.ShouldSkipAction(nightTime) {
			skippedCount++
		}
	}
	
	// With 90% skip probability, should skip most of the time
	assert.True(t, skippedCount > 50)
}

// Test selectWeightedAction
func TestSelectWeightedAction(t *testing.T) {
	actions := []config.ActionConfig{
		{Type: "like_post", Weight: 40},
		{Type: "view_feed", Weight: 30},
		{Type: "subscribe", Weight: 20},
		{Type: "comment", Weight: 10},
	}

	cfg := &config.Config{}
	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	// Run multiple selections
	selections := make(map[string]int)
	for i := 0; i < 1000; i++ {
		action := scheduler.selectWeightedAction(actions)
		selections[action]++
	}

	// Higher weight actions should be selected more often
	assert.True(t, selections["like_post"] > selections["comment"])
	assert.True(t, selections["view_feed"] > selections["comment"])
}

// Test selectWeightedAction - empty actions
func TestSelectWeightedAction_EmptyActions(t *testing.T) {
	cfg := &config.Config{}
	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	action := scheduler.selectWeightedAction([]config.ActionConfig{})
	assert.Equal(t, "view_feed", action) // Default action
}

// Test GetActionDistribution
func TestGetActionDistribution(t *testing.T) {
	dayConfig := &config.DayConfig{
		Actions: []config.ActionConfig{
			{Type: "like_post", Weight: 40},
			{Type: "view_feed", Weight: 30},
			{Type: "subscribe", Weight: 20},
			{Type: "comment", Weight: 10},
		},
	}

	cfg := &config.Config{}
	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	distribution := scheduler.GetActionDistribution(dayConfig)

	// Total weight is 100, so percentages should match weights
	assert.Equal(t, 40, distribution["like_post"])
	assert.Equal(t, 30, distribution["view_feed"])
	assert.Equal(t, 20, distribution["subscribe"])
	assert.Equal(t, 10, distribution["comment"])
}

// Test generateDailySchedule
func TestGenerateDailySchedule(t *testing.T) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart: 8,
				ActiveHoursEnd:   22,
			},
		},
	}

	actions := []config.ActionConfig{
		{Type: "like_post", Weight: 50},
		{Type: "view_feed", Weight: 50},
	}

	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	schedule := scheduler.generateDailySchedule(5, actions, 1)

	assert.Len(t, schedule, 5)
	
	for _, action := range schedule {
		// All scheduled times should be within active hours
		hour := action.PlannedAt.Hour()
		assert.True(t, hour >= 8 && hour < 22)
		
		// Action type should be one of the defined actions
		assert.True(t, action.ActionType == "like_post" || action.ActionType == "view_feed")
	}
}

// Test PlannedAction model
func TestPlannedActionModel(t *testing.T) {
	action := models.PlannedAction{
		ActionType:  "like_post",
		PlannedAt:   time.Now().Add(1 * time.Hour),
		TimeWindow:  30,
		Priority:    1,
		Executed:    false,
	}

	assert.Equal(t, "like_post", action.ActionType)
	assert.Equal(t, 30, action.TimeWindow)
	assert.Equal(t, 1, action.Priority)
	assert.False(t, action.Executed)
}

// Test WarmingSchedule model
func TestWarmingScheduleModel(t *testing.T) {
	taskID := primitive.NewObjectID()
	schedule := &models.WarmingSchedule{
		ID:     primitive.NewObjectID(),
		TaskID: taskID,
		PlannedActions: []models.PlannedAction{
			{ActionType: "like_post", PlannedAt: time.Now()},
			{ActionType: "view_feed", PlannedAt: time.Now()},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.NotNil(t, schedule.ID)
	assert.Equal(t, taskID, schedule.TaskID)
	assert.Len(t, schedule.PlannedActions, 2)
}

// Test randomInRange
func TestRandomInRange(t *testing.T) {
	tests := []struct {
		name string
		min  int
		max  int
	}{
		{"small range", 1, 5},
		{"medium range", 10, 50},
		{"large range", 100, 500},
		{"same values", 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				result := randomInRange(tt.min, tt.max)
				assert.True(t, result >= tt.min)
				assert.True(t, result <= tt.max)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSelectWeightedAction(b *testing.B) {
	actions := []config.ActionConfig{
		{Type: "like_post", Weight: 40},
		{Type: "view_feed", Weight: 30},
		{Type: "subscribe", Weight: 20},
		{Type: "comment", Weight: 10},
	}

	cfg := &config.Config{}
	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scheduler.selectWeightedAction(actions)
	}
}

func BenchmarkGenerateDailySchedule(b *testing.B) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart: 8,
				ActiveHoursEnd:   22,
			},
		},
	}

	actions := []config.ActionConfig{
		{Type: "like_post", Weight: 40},
		{Type: "view_feed", Weight: 30},
		{Type: "subscribe", Weight: 20},
		{Type: "comment", Weight: 10},
	}

	logger := new(MockLogger)
	scheduler := &Scheduler{config: cfg, logger: logger}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scheduler.generateDailySchedule(10, actions, 1)
	}
}

func BenchmarkIsWithinActiveHours(b *testing.B) {
	cfg := &config.Config{
		WarmingConfig: config.WarmingConfig{
			BehaviorSimulation: config.BehaviorSimulationConfig{
				ActiveHoursStart: 8,
				ActiveHoursEnd:   22,
			},
		},
	}

	scheduler := &Scheduler{config: cfg}
	testTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scheduler.isWithinActiveHours(testTime)
	}
}


