package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"conveer/pkg/logger"
	"conveer/services/warming-service/internal/config"
	"conveer/services/warming-service/internal/models"
	"conveer/services/warming-service/internal/repository"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Scheduler struct {
	service      *warmingService
	config       *config.Config
	logger       logger.Logger
	behaviorSim  *BehaviorSimulator
	scheduleRepo repository.ScheduleRepository
	statsRepo    repository.StatsRepository
}

func NewScheduler(service *warmingService, scheduleRepo repository.ScheduleRepository, statsRepo repository.StatsRepository, config *config.Config, logger logger.Logger) *Scheduler {
	return &Scheduler{
		service:      service,
		config:       config,
		logger:       logger,
		behaviorSim:  NewBehaviorSimulator(config, logger),
		scheduleRepo: scheduleRepo,
		statsRepo:    statsRepo,
	}
}

func (s *Scheduler) ScheduleTaskActions(ctx context.Context, task *models.WarmingTask) error {
	// Get scenario configuration
	scenarioConfig := s.getScenarioConfig(task)
	if scenarioConfig == nil {
		return fmt.Errorf("scenario configuration not found")
	}

	// Get day configuration based on current day
	dayConfig := s.getDayConfig(scenarioConfig, task.CurrentDay, task.DurationDays)
	if dayConfig == nil {
		return fmt.Errorf("day configuration not found for day %d", task.CurrentDay)
	}

	// Parse actions per day range
	minActions, maxActions := s.parseActionsPerDay(dayConfig.ActionsPerDay)
	actionsToday := randomInRange(minActions, maxActions)

	// Generate action schedule for today
	schedule := s.generateDailySchedule(actionsToday, dayConfig.Actions, task.CurrentDay)

	// Save schedule to database
	existingSchedule, err := s.scheduleRepo.GetByTaskID(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing schedule: %w", err)
	}

	if existingSchedule == nil {
		// Create new schedule
		warmingSchedule := &models.WarmingSchedule{
			ID:             primitive.NewObjectID(),
			TaskID:         task.ID,
			PlannedActions: schedule,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.scheduleRepo.Create(ctx, warmingSchedule); err != nil {
			return fmt.Errorf("failed to create schedule: %w", err)
		}
	} else {
		// Update existing schedule
		if err := s.scheduleRepo.UpdatePlannedActions(ctx, task.ID, schedule); err != nil {
			return fmt.Errorf("failed to update schedule: %w", err)
		}
	}

	// Calculate next action time
	nextActionTime := s.CalculateNextActionTime(time.Now(), task.CurrentDay, task.DurationDays)

	// Update task with next action time
	if err := s.service.taskRepo.UpdateNextActionTime(ctx, task.ID, nextActionTime); err != nil {
		return fmt.Errorf("failed to update next action time: %w", err)
	}

	s.logger.Info("Scheduled %d actions for task %s on day %d", actionsToday, task.ID.Hex(), task.CurrentDay)
	return nil
}

func (s *Scheduler) CalculateNextActionTime(currentTime time.Time, currentDay, totalDays int) time.Time {
	// Use behavior simulator to calculate realistic next action time
	nextTime := s.behaviorSim.CalculateNextActionTime(currentTime, s.config.WarmingConfig.BehaviorSimulation)

	// Check if it's within active hours
	if !s.isWithinActiveHours(nextTime) {
		// Move to next active period
		nextTime = s.getNextActiveTime(nextTime)
	}

	// Apply progression-based adjustments (more frequent actions as warming progresses)
	progression := float64(currentDay) / float64(totalDays)
	if progression < 0.3 {
		// Early stage: less frequent actions
		additionalDelay := time.Duration(rand.Intn(1800)) * time.Second // 0-30 minutes
		nextTime = nextTime.Add(additionalDelay)
	}

	return nextTime
}

func (s *Scheduler) SelectNextAction(task *models.WarmingTask, dayConfig *config.DayConfig) string {
	if len(dayConfig.Actions) == 0 {
		return ""
	}

	// Calculate total weight
	totalWeight := 0
	for _, action := range dayConfig.Actions {
		totalWeight += action.Weight
	}

	// Weighted random selection
	random := rand.Intn(totalWeight)
	currentWeight := 0

	for _, action := range dayConfig.Actions {
		currentWeight += action.Weight
		if random < currentWeight {
			// Check if action has daily limits
			if s.hasReachedDailyLimit(task, action) {
				// Try to select another action
				continue
			}
			return action.Type
		}
	}

	// Fallback to first available action
	return dayConfig.Actions[0].Type
}

func (s *Scheduler) getScenarioConfig(task *models.WarmingTask) *config.PlatformScenarioConfig {
	scenarios := s.config.WarmingConfig.Scenarios[task.ScenarioType]
	if scenarios == nil {
		return nil
	}

	platformConfig := scenarios[task.Platform]
	if platformConfig == nil {
		return nil
	}

	// Return config based on duration
	if task.DurationDays <= 30 {
		return &platformConfig["duration_14_30"]
	}
	return &platformConfig["duration_30_60"]
}

func (s *Scheduler) getDayConfig(scenarioConfig *config.PlatformScenarioConfig, currentDay, totalDays int) *config.DayConfig {
	var dayConfig *config.DayConfig

	if totalDays <= 30 {
		// 14-30 day scenario
		switch {
		case currentDay <= 7:
			dayConfig = &scenarioConfig.Duration14_30.Days1_7
		case currentDay <= 14:
			dayConfig = &scenarioConfig.Duration14_30.Days8_14
		case currentDay <= 30:
			dayConfig = &scenarioConfig.Duration14_30.Days15_30
		}
	} else {
		// 30-60 day scenario
		switch {
		case currentDay <= 7:
			dayConfig = &scenarioConfig.Duration30_60.Days1_7
		case currentDay <= 14:
			dayConfig = &scenarioConfig.Duration30_60.Days8_14
		case currentDay <= 30:
			dayConfig = &scenarioConfig.Duration30_60.Days15_30
		default:
			dayConfig = &scenarioConfig.Duration30_60.Days31_60
		}
	}

	return dayConfig
}

func (s *Scheduler) parseActionsPerDay(actionsPerDay string) (min, max int) {
	// Parse format like "5-10" or "10-15"
	parts := strings.Split(actionsPerDay, "-")
	if len(parts) != 2 {
		return 5, 10 // Default
	}

	min, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	max, _ = strconv.Atoi(strings.TrimSpace(parts[1]))

	if min <= 0 {
		min = 5
	}
	if max <= min {
		max = min + 5
	}

	return min, max
}

func (s *Scheduler) generateDailySchedule(actionsCount int, actions []config.ActionConfig, currentDay int) []models.PlannedAction {
	var schedule []models.PlannedAction

	// Get active hours
	activeStart := s.config.WarmingConfig.BehaviorSimulation.ActiveHoursStart
	activeEnd := s.config.WarmingConfig.BehaviorSimulation.ActiveHoursEnd
	activeDuration := activeEnd - activeStart

	// Distribute actions throughout the day
	for i := 0; i < actionsCount; i++ {
		// Random time within active hours
		hour := activeStart + rand.Intn(activeDuration)
		minute := rand.Intn(60)

		scheduledTime := time.Now().Truncate(24*time.Hour).Add(
			time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute,
		)

		// Select action type based on weights
		actionType := s.selectWeightedAction(actions)

		schedule = append(schedule, models.PlannedAction{
			ActionType:  actionType,
			PlannedAt:   scheduledTime,
			TimeWindow:  30, // 30 minute window
			Priority:    1,
			Executed:    false,
		})
	}

	return schedule
}

func (s *Scheduler) selectWeightedAction(actions []config.ActionConfig) string {
	if len(actions) == 0 {
		return "view_feed" // Default action
	}

	totalWeight := 0
	for _, action := range actions {
		totalWeight += action.Weight
	}

	random := rand.Intn(totalWeight)
	currentWeight := 0

	for _, action := range actions {
		currentWeight += action.Weight
		if random < currentWeight {
			return action.Type
		}
	}

	return actions[0].Type
}

func (s *Scheduler) isWithinActiveHours(t time.Time) bool {
	hour := t.Hour()
	activeStart := s.config.WarmingConfig.BehaviorSimulation.ActiveHoursStart
	activeEnd := s.config.WarmingConfig.BehaviorSimulation.ActiveHoursEnd

	return hour >= activeStart && hour < activeEnd
}

func (s *Scheduler) getNextActiveTime(t time.Time) time.Time {
	activeStart := s.config.WarmingConfig.BehaviorSimulation.ActiveHoursStart

	// If before active hours today, move to start of active hours
	if t.Hour() < activeStart {
		return time.Date(t.Year(), t.Month(), t.Day(), activeStart, 0, 0, 0, t.Location())
	}

	// Otherwise, move to start of active hours tomorrow
	tomorrow := t.Add(24 * time.Hour)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), activeStart, 0, 0, 0, tomorrow.Location())
}

func (s *Scheduler) hasReachedDailyLimit(task *models.WarmingTask, action config.ActionConfig) bool {
	// Check if action has daily limit in params
	if action.Params == nil {
		return false
	}

	maxPerDay, ok := action.Params["max_per_day"]
	if !ok {
		return false
	}

	limit := 0
	switch v := maxPerDay.(type) {
	case int:
		limit = v
	case float64:
		limit = int(v)
	case string:
		limit, _ = strconv.Atoi(v)
	}

	if limit <= 0 {
		return false
	}

	// Check today's action count for this type
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	// Count actions performed today
	count, err := s.statsRepo.CountActionsByType(context.Background(), task.ID, action.Type, today, tomorrow)
	if err != nil {
		s.logger.Error("Failed to count today's actions: %v", err)
		return false
	}

	return count >= limit
}

func (s *Scheduler) ShouldSkipAction(currentTime time.Time) bool {
	// Check if we should skip this action based on behavior patterns
	hour := currentTime.Hour()
	dayOfWeek := currentTime.Weekday()

	// Night time skip
	if hour < s.config.WarmingConfig.BehaviorSimulation.ActiveHoursStart ||
	   hour >= s.config.WarmingConfig.BehaviorSimulation.ActiveHoursEnd {
		nightSkipProb := s.config.WarmingConfig.BehaviorSimulation.NightPauseProbability
		if rand.Float64() < nightSkipProb {
			return true
		}
	}

	// Weekend activity reduction
	if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
		weekendReduction := s.config.WarmingConfig.BehaviorSimulation.WeekendActivityReduction
		if rand.Float64() > weekendReduction {
			return true
		}
	}

	return false
}

func (s *Scheduler) GetActionDistribution(dayConfig *config.DayConfig) map[string]int {
	distribution := make(map[string]int)

	totalWeight := 0
	for _, action := range dayConfig.Actions {
		totalWeight += action.Weight
	}

	// Calculate percentage distribution
	for _, action := range dayConfig.Actions {
		percentage := (action.Weight * 100) / totalWeight
		distribution[action.Type] = percentage
	}

	return distribution
}
