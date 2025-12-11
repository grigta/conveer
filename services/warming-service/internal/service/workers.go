package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"conveer/services/warming-service/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (s *warmingService) runActionExecutorWorker(ctx context.Context) {
	// Consumer for execute_action commands
	err := s.messaging.ConsumeQueue(ctx, "warming.execute_action", func(msg []byte) error {
		var command struct {
			TaskID    string `json:"task_id"`
			AccountID string `json:"account_id"`
			Platform  string `json:"platform"`
			Day       int    `json:"day"`
		}

		if err := json.Unmarshal(msg, &command); err != nil {
			return fmt.Errorf("failed to unmarshal command: %w", err)
		}

		taskID, _ := primitive.ObjectIDFromHex(command.TaskID)
		accountID, _ := primitive.ObjectIDFromHex(command.AccountID)

		// Execute action
		return s.executeTaskAction(ctx, taskID, accountID, command.Platform, command.Day)
	})

	if err != nil {
		s.logger.Error("Action executor worker error: %v", err)
	}
}

func (s *warmingService) executeTaskAction(ctx context.Context, taskID, accountID primitive.ObjectID, platform string, day int) error {
	// Get task details
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Check if task should be executed
	if task.Status != string(models.TaskStatusInProgress) {
		s.logger.Info("Skipping task %s with status %s", taskID.Hex(), task.Status)
		return nil
	}

	// Get scenario configuration
	scenarioConfig := s.scheduler.getScenarioConfig(task)
	if scenarioConfig == nil {
		return fmt.Errorf("scenario config not found")
	}

	dayConfig := s.scheduler.getDayConfig(scenarioConfig, task.CurrentDay, task.DurationDays)
	if dayConfig == nil {
		return fmt.Errorf("day config not found")
	}

	// Select action to execute
	actionType := s.scheduler.SelectNextAction(task, dayConfig)
	if actionType == "" {
		return fmt.Errorf("no action selected")
	}

	// Check if should skip due to behavior simulation
	if s.scheduler.ShouldSkipAction(time.Now()) {
		s.logger.Info("Skipping action due to behavior simulation")
		// Schedule next action
		nextTime := s.scheduler.CalculateNextActionTime(time.Now(), task.CurrentDay, task.DurationDays)
		return s.taskRepo.UpdateNextActionTime(ctx, taskID, nextTime)
	}

	// Get today's action count
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)
	actionsToday, err := s.statsRepo.CountActionsByType(ctx, taskID, actionType, today, tomorrow)
	if err != nil {
		s.logger.Error("Failed to get today's action count: %v", err)
		actionsToday = 0
	}

	// Create execution context
	execCtx := &models.ExecutionContext{
		TaskID:       taskID,
		AccountID:    accountID,
		Platform:     platform,
		CurrentDay:   day,
		TotalDays:    task.DurationDays,
		ActionsToday: actionsToday,
	}

	// Get platform executor
	executor, ok := s.platformExecs[platform]
	if !ok {
		return fmt.Errorf("executor not found for platform %s", platform)
	}

	// Execute action
	start := time.Now()
	err = executor.ExecuteAction(ctx, task, actionType, execCtx)
	duration := time.Since(start).Milliseconds()

	// Log action
	actionLog := &models.WarmingActionLog{
		TaskID:     taskID,
		AccountID:  accountID,
		Platform:   platform,
		ActionType: actionType,
		Day:        day,
		DurationMs: duration,
		Timestamp:  time.Now(),
	}

	if err != nil {
		actionLog.Status = "failed"
		actionLog.Error = err.Error()
		actionLog.ErrorType = categorizeError(err)

		// Check if error requires special handling
		if actionErr, ok := err.(*ActionExecutionError); ok {
			if actionErr.ShouldPause {
				s.pauseTask(ctx, taskID, actionErr.Message)
			} else if actionErr.ShouldStop {
				s.stopTask(ctx, taskID, actionErr.Message)
			}
		}

		// Increment failed counter
		s.taskRepo.IncrementCounters(ctx, taskID, 0, 1)
		s.metrics.IncrementActionsTotal(platform, actionType, "failed")
	} else {
		actionLog.Status = "success"

		// Increment completed counter
		s.taskRepo.IncrementCounters(ctx, taskID, 1, 0)
		s.metrics.IncrementActionsTotal(platform, actionType, "success")
	}

	// Save action log
	if err := s.statsRepo.SaveActionLog(ctx, actionLog); err != nil {
		s.logger.Error("Failed to save action log: %v", err)
	}

	// Update task progress
	if err := s.updateTaskProgress(ctx, task); err != nil {
		s.logger.Error("Failed to update task progress: %v", err)
	}

	// Schedule next action
	nextTime := s.scheduler.CalculateNextActionTime(time.Now(), task.CurrentDay, task.DurationDays)
	if err := s.taskRepo.UpdateNextActionTime(ctx, taskID, nextTime); err != nil {
		s.logger.Error("Failed to update next action time: %v", err)
	}

	// Publish action executed event
	s.publishEvent("warming.action.executed", platform, map[string]interface{}{
		"task_id":     taskID.Hex(),
		"action_type": actionType,
		"status":      actionLog.Status,
		"duration_ms": duration,
	})

	return nil
}

func (s *warmingService) runStatusSyncWorker(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAccountStatuses(ctx)
		}
	}
}

func (s *warmingService) syncAccountStatuses(ctx context.Context) {
	// Get all in-progress tasks
	tasks, err := s.taskRepo.List(ctx, models.TaskFilter{
		Status: string(models.TaskStatusInProgress),
		Limit:  100,
	})

	if err != nil {
		s.logger.Error("Failed to get tasks for status sync: %v", err)
		return
	}

	for _, task := range tasks {
		// Validate account with platform service
		executor, ok := s.platformExecs[task.Platform]
		if !ok {
			continue
		}

		if err := executor.ValidateAccount(ctx, task.AccountID); err != nil {
			s.logger.Error("Account %s validation failed: %v", task.AccountID.Hex(), err)

			// Check if account is banned
			if contains(err.Error(), "ban", "blocked", "suspended") {
				s.stopTask(ctx, task.ID, "Account banned")
				s.updateAccountStatus(ctx, task.AccountID, task.Platform, "banned")
			}
		}
	}
}

func (s *warmingService) runStuckTaskMonitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.handleStuckTasks(ctx)
		}
	}
}

func (s *warmingService) handleStuckTasks(ctx context.Context) {
	// Find tasks that haven't been updated in 2 hours
	stuckTasks, err := s.taskRepo.GetStuckTasks(ctx, 2*time.Hour)
	if err != nil {
		s.logger.Error("Failed to get stuck tasks: %v", err)
		return
	}

	for _, task := range stuckTasks {
		s.logger.Warn("Found stuck task %s, attempting recovery", task.ID.Hex())

		// Try to resume task
		nextTime := s.scheduler.CalculateNextActionTime(time.Now(), task.CurrentDay, task.DurationDays)
		update := models.TaskUpdate{
			Status:       stringPtr(string(models.TaskStatusInProgress)),
			NextActionAt: &nextTime,
			LastError:    stringPtr("Task recovered from stuck state"),
		}

		if err := s.taskRepo.Update(ctx, task.ID, update); err != nil {
			s.logger.Error("Failed to recover stuck task %s: %v", task.ID.Hex(), err)

			// Mark as failed if recovery fails
			s.taskRepo.UpdateStatus(ctx, task.ID, string(models.TaskStatusFailed))
		} else {
			s.logger.Info("Successfully recovered stuck task %s", task.ID.Hex())
		}
	}
}

func (s *warmingService) runAutoStartConsumer(ctx context.Context) {
	// Consumer for account creation events
	err := s.messaging.ConsumeQueue(ctx, "warming.auto_start", func(msg []byte) error {
		var event struct {
			AccountID string `json:"account_id"`
			Platform  string `json:"platform"`
		}

		if err := json.Unmarshal(msg, &event); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}

		accountID, _ := primitive.ObjectIDFromHex(event.AccountID)

		// Auto-start warming with basic scenario (14-30 days)
		task, err := s.StartWarming(ctx, accountID, event.Platform, "basic", nil, 21)
		if err != nil {
			s.logger.Error("Failed to auto-start warming for account %s: %v", event.AccountID, err)
			return err
		}

		s.logger.Info("Auto-started warming task %s for new account %s on %s",
			task.ID.Hex(), event.AccountID, event.Platform)

		return nil
	})

	if err != nil {
		s.logger.Error("Auto-start consumer error: %v", err)
	}
}

func (s *warmingService) runStatsAggregator(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.aggregateStats(ctx)
		}
	}
}

func (s *warmingService) aggregateStats(ctx context.Context) {
	platforms := []string{"vk", "telegram", "mail", "max"}

	for _, platform := range platforms {
		// Count tasks by status
		stats := &models.WarmingStats{
			Platform: platform,
			Date:     time.Now(),
		}

		// Get counts
		stats.TotalTasks, _ = s.taskRepo.Count(ctx, models.TaskFilter{Platform: platform})
		stats.CompletedTasks, _ = s.taskRepo.Count(ctx, models.TaskFilter{
			Platform: platform,
			Status:   string(models.TaskStatusCompleted),
		})
		stats.FailedTasks, _ = s.taskRepo.Count(ctx, models.TaskFilter{
			Platform: platform,
			Status:   string(models.TaskStatusFailed),
		})
		stats.InProgressTasks, _ = s.taskRepo.Count(ctx, models.TaskFilter{
			Platform: platform,
			Status:   string(models.TaskStatusInProgress),
		})

		// Calculate success rate
		if stats.TotalTasks > 0 {
			stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
		}

		// Update daily stats
		if err := s.statsRepo.UpdateDailyStats(ctx, platform, stats); err != nil {
			s.logger.Error("Failed to update daily stats for %s: %v", platform, err)
		}
	}

	// Cleanup old logs (older than 90 days)
	if err := s.statsRepo.CleanupOldLogs(ctx, 90); err != nil {
		s.logger.Error("Failed to cleanup old logs: %v", err)
	}
}

func (s *warmingService) updateTaskProgress(ctx context.Context, task *models.WarmingTask) error {
	// Check if day should be incremented
	now := time.Now()
	if task.UpdatedAt.Day() != now.Day() {
		// New day, increment current day
		currentDay := task.CurrentDay + 1

		// Check if warming is complete
		if currentDay >= task.DurationDays {
			return s.completeTask(ctx, task.ID)
		}

		update := models.TaskUpdate{
			CurrentDay: &currentDay,
		}

		return s.taskRepo.Update(ctx, task.ID, update)
	}

	return nil
}

func (s *warmingService) pauseTask(ctx context.Context, taskID primitive.ObjectID, reason string) error {
	update := models.TaskUpdate{
		Status:    stringPtr(string(models.TaskStatusPaused)),
		LastError: &reason,
	}

	if err := s.taskRepo.Update(ctx, taskID, update); err != nil {
		return err
	}

	// Publish pause event
	s.publishEvent("warming.task.paused", "", map[string]interface{}{
		"task_id": taskID.Hex(),
		"reason":  reason,
	})

	// Publish manual intervention needed event
	s.publishEvent("warming.manual_intervention", "", map[string]interface{}{
		"task_id": taskID.Hex(),
		"reason":  reason,
	})

	return nil
}

func (s *warmingService) stopTask(ctx context.Context, taskID primitive.ObjectID, reason string) error {
	now := time.Now()
	update := models.TaskUpdate{
		Status:      stringPtr(string(models.TaskStatusFailed)),
		LastError:   &reason,
		CompletedAt: &now,
	}

	if err := s.taskRepo.Update(ctx, taskID, update); err != nil {
		return err
	}

	// Publish failure event
	s.publishEvent("warming.task.failed", "", map[string]interface{}{
		"task_id": taskID.Hex(),
		"reason":  reason,
	})

	return nil
}

func (s *warmingService) completeTask(ctx context.Context, taskID primitive.ObjectID) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	update := models.TaskUpdate{
		Status:      stringPtr(string(models.TaskStatusCompleted)),
		CompletedAt: &now,
	}

	if err := s.taskRepo.Update(ctx, taskID, update); err != nil {
		return err
	}

	// Update account status to ready
	s.updateAccountStatus(ctx, task.AccountID, task.Platform, "ready")

	// Publish completion event
	s.publishEvent("warming.task.completed", task.Platform, map[string]interface{}{
		"task_id":           taskID.Hex(),
		"account_id":        task.AccountID.Hex(),
		"duration_days":     task.DurationDays,
		"actions_completed": task.ActionsCompleted,
	})

	// Publish account ready event
	s.publishEvent("warming.account.ready", task.Platform, map[string]interface{}{
		"account_id": task.AccountID.Hex(),
	})

	s.metrics.IncrementAccountsReady(task.Platform)

	return nil
}