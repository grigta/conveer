package repository

import (
	"context"
	"fmt"
	"time"

	"conveer/services/warming-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type StatsRepository interface {
	SaveActionLog(ctx context.Context, log *models.WarmingActionLog) error
	GetActionLogs(ctx context.Context, taskID primitive.ObjectID, limit int) ([]*models.WarmingActionLog, error)
	UpdateDailyStats(ctx context.Context, platform string, stats *models.WarmingStats) error
	GetDailyStats(ctx context.Context, platform string, startDate, endDate time.Time) ([]*models.WarmingStats, error)
	GetAggregatedStats(ctx context.Context, platform string, startDate, endDate time.Time) (*models.AggregatedStats, error)
	GetTopActions(ctx context.Context, platform string, limit int) ([]models.ActionStatistic, error)
	GetCommonErrors(ctx context.Context, platform string, limit int) ([]models.ErrorStatistic, error)
	CleanupOldLogs(ctx context.Context, retentionDays int) error
	CountActionsByType(ctx context.Context, taskID primitive.ObjectID, actionType string, startTime, endTime time.Time) (int, error)
}

type statsRepository struct {
	actionLogCollection *mongo.Collection
	statsCollection     *mongo.Collection
}

func NewStatsRepository(db *mongo.Database) StatsRepository {
	return &statsRepository{
		actionLogCollection: db.Collection("warming_actions_log"),
		statsCollection:     db.Collection("warming_stats"),
	}
}

func (r *statsRepository) SaveActionLog(ctx context.Context, log *models.WarmingActionLog) error {
	log.Timestamp = time.Now()

	_, err := r.actionLogCollection.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("failed to save action log: %w", err)
	}

	return nil
}

func (r *statsRepository) GetActionLogs(ctx context.Context, taskID primitive.ObjectID, limit int) ([]*models.WarmingActionLog, error) {
	filter := bson.M{"task_id": taskID}

	findOptions := options.Find().
		SetSort(bson.D{{"timestamp", -1}}).
		SetLimit(int64(limit))

	cursor, err := r.actionLogCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get action logs: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.WarmingActionLog
	if err = cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode action logs: %w", err)
	}

	return logs, nil
}

func (r *statsRepository) UpdateDailyStats(ctx context.Context, platform string, stats *models.WarmingStats) error {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	filter := bson.M{
		"platform": platform,
		"date":     startOfDay,
	}

	update := bson.M{
		"$set": bson.M{
			"platform":           platform,
			"date":               startOfDay,
			"total_tasks":        stats.TotalTasks,
			"completed_tasks":    stats.CompletedTasks,
			"failed_tasks":       stats.FailedTasks,
			"in_progress_tasks":  stats.InProgressTasks,
			"paused_tasks":       stats.PausedTasks,
			"total_actions":      stats.TotalActions,
			"successful_actions": stats.SuccessfulActions,
			"failed_actions":     stats.FailedActions,
			"avg_duration_days":  stats.AvgDurationDays,
			"success_rate":       stats.SuccessRate,
			"by_scenario_type":   stats.ByScenarioType,
			"by_action_type":     stats.ByActionType,
			"error_types":        stats.ErrorTypes,
			"updated_at":         now,
		},
		"$setOnInsert": bson.M{
			"created_at": now,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.statsCollection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update daily stats: %w", err)
	}

	return nil
}

func (r *statsRepository) GetDailyStats(ctx context.Context, platform string, startDate, endDate time.Time) ([]*models.WarmingStats, error) {
	filter := bson.M{
		"date": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}

	if platform != "" {
		filter["platform"] = platform
	}

	findOptions := options.Find().SetSort(bson.D{{"date", 1}})

	cursor, err := r.statsCollection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer cursor.Close(ctx)

	var stats []*models.WarmingStats
	if err = cursor.All(ctx, &stats); err != nil {
		return nil, fmt.Errorf("failed to decode daily stats: %w", err)
	}

	return stats, nil
}

func (r *statsRepository) GetAggregatedStats(ctx context.Context, platform string, startDate, endDate time.Time) (*models.AggregatedStats, error) {
	// Build aggregation pipeline
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"date": bson.M{
					"$gte": startDate,
					"$lte": endDate,
				},
			},
		},
	}

	if platform != "" {
		pipeline[0]["$match"].(bson.M)["platform"] = platform
	}

	pipeline = append(pipeline, bson.M{
		"$group": bson.M{
			"_id": nil,
			"total_tasks": bson.M{"$sum": "$total_tasks"},
			"completed_tasks": bson.M{"$sum": "$completed_tasks"},
			"failed_tasks": bson.M{"$sum": "$failed_tasks"},
			"in_progress_tasks": bson.M{"$last": "$in_progress_tasks"},
			"total_actions": bson.M{"$sum": "$total_actions"},
			"successful_actions": bson.M{"$sum": "$successful_actions"},
			"failed_actions": bson.M{"$sum": "$failed_actions"},
			"avg_duration_days": bson.M{"$avg": "$avg_duration_days"},
		},
	})

	cursor, err := r.statsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode aggregated stats: %w", err)
	}

	if len(results) == 0 {
		return &models.AggregatedStats{
			Platform:  platform,
			DateRange: models.DateRange{StartDate: startDate, EndDate: endDate},
		}, nil
	}

	result := results[0]
	stats := &models.AggregatedStats{
		Platform:        platform,
		DateRange:       models.DateRange{StartDate: startDate, EndDate: endDate},
		TotalTasks:      getInt64(result, "total_tasks"),
		CompletedTasks:  getInt64(result, "completed_tasks"),
		FailedTasks:     getInt64(result, "failed_tasks"),
		InProgressTasks: getInt64(result, "in_progress_tasks"),
	}

	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}
	stats.AvgDurationDays = getFloat64(result, "avg_duration_days")

	// Get daily breakdown
	dailyStats, err := r.GetDailyStats(ctx, platform, startDate, endDate)
	if err == nil {
		for _, daily := range dailyStats {
			stats.DailyBreakdown = append(stats.DailyBreakdown, models.DailyStatistic{
				Date:            daily.Date,
				TasksStarted:    daily.TotalTasks,
				TasksCompleted:  daily.CompletedTasks,
				TasksFailed:     daily.FailedTasks,
				ActionsExecuted: daily.TotalActions,
				SuccessRate:     daily.SuccessRate,
			})
		}
	}

	return stats, nil
}

func (r *statsRepository) GetTopActions(ctx context.Context, platform string, limit int) ([]models.ActionStatistic, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"platform": platform,
				"timestamp": bson.M{
					"$gte": time.Now().AddDate(0, 0, -30), // Last 30 days
				},
			},
		},
		{
			"$group": bson.M{
				"_id": "$action_type",
				"count": bson.M{"$sum": 1},
				"successful": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$status", "success"}},
							"then": 1,
							"else": 0,
						},
					},
				},
				"avg_duration": bson.M{"$avg": "$duration_ms"},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := r.actionLogCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get top actions: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode top actions: %w", err)
	}

	var actions []models.ActionStatistic
	for _, result := range results {
		count := getInt64(result, "count")
		successful := getInt64(result, "successful")
		successRate := float64(0)
		if count > 0 {
			successRate = float64(successful) / float64(count) * 100
		}

		actions = append(actions, models.ActionStatistic{
			ActionType:  result["_id"].(string),
			Count:       count,
			SuccessRate: successRate,
			AvgDuration: getFloat64(result, "avg_duration"),
		})
	}

	return actions, nil
}

func (r *statsRepository) GetCommonErrors(ctx context.Context, platform string, limit int) ([]models.ErrorStatistic, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"platform": platform,
				"status":   "failed",
				"timestamp": bson.M{
					"$gte": time.Now().AddDate(0, 0, -30), // Last 30 days
				},
			},
		},
		{
			"$group": bson.M{
				"_id":   "$error_type",
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"count": -1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := r.actionLogCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get common errors: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode common errors: %w", err)
	}

	// Calculate total errors for percentage
	totalErrors := int64(0)
	for _, result := range results {
		totalErrors += getInt64(result, "count")
	}

	var errors []models.ErrorStatistic
	for _, result := range results {
		count := getInt64(result, "count")
		percentage := float64(0)
		if totalErrors > 0 {
			percentage = float64(count) / float64(totalErrors) * 100
		}

		if errorType, ok := result["_id"].(string); ok && errorType != "" {
			errors = append(errors, models.ErrorStatistic{
				ErrorType:  errorType,
				Count:      count,
				Percentage: percentage,
			})
		}
	}

	return errors, nil
}

func (r *statsRepository) CleanupOldLogs(ctx context.Context, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	filter := bson.M{
		"timestamp": bson.M{"$lt": cutoffDate},
	}

	result, err := r.actionLogCollection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	if result.DeletedCount > 0 {
		// Log cleanup info
		fmt.Printf("Cleaned up %d old action logs\n", result.DeletedCount)
	}

	return nil
}

func (r *statsRepository) CountActionsByType(ctx context.Context, taskID primitive.ObjectID, actionType string, startTime, endTime time.Time) (int, error) {
	filter := bson.M{
		"task_id":     taskID,
		"action_type": actionType,
		"status":      "success",
		"timestamp": bson.M{
			"$gte": startTime,
			"$lt":  endTime,
		},
	}

	count, err := r.actionLogCollection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count actions: %w", err)
	}

	return int(count), nil
}

// Helper functions
func getInt64(m bson.M, key string) int64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int32:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getFloat64(m bson.M, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int32:
			return float64(v)
		}
	}
	return 0
}