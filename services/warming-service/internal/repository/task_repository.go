package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/grigta/conveer/services/warming-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.WarmingTask) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingTask, error)
	GetByAccountAndPlatform(ctx context.Context, accountID primitive.ObjectID, platform string) (*models.WarmingTask, error)
	Update(ctx context.Context, id primitive.ObjectID, update models.TaskUpdate) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error
	UpdateNextActionTime(ctx context.Context, id primitive.ObjectID, nextActionAt time.Time) error
	IncrementCounters(ctx context.Context, id primitive.ObjectID, completed, failed int) error
	List(ctx context.Context, filter models.TaskFilter) ([]*models.WarmingTask, error)
	GetTasksForExecution(ctx context.Context, limit int) ([]*models.WarmingTask, error)
	GetStuckTasks(ctx context.Context, stuckDuration time.Duration) ([]*models.WarmingTask, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	Count(ctx context.Context, filter models.TaskFilter) (int64, error)
}

type taskRepository struct {
	collection *mongo.Collection
}

func NewTaskRepository(db *mongo.Database) TaskRepository {
	return &taskRepository{
		collection: db.Collection("warming_tasks"),
	}
}

func (r *taskRepository) Create(ctx context.Context, task *models.WarmingTask) error {
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to create warming task: %w", err)
	}

	task.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *taskRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingTask, error) {
	var task models.WarmingTask

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&task)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("warming task not found")
		}
		return nil, fmt.Errorf("failed to get warming task: %w", err)
	}

	return &task, nil
}

func (r *taskRepository) GetByAccountAndPlatform(ctx context.Context, accountID primitive.ObjectID, platform string) (*models.WarmingTask, error) {
	var task models.WarmingTask

	filter := bson.M{
		"account_id": accountID,
		"platform":   platform,
	}

	err := r.collection.FindOne(ctx, filter).Decode(&task)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get warming task: %w", err)
	}

	return &task, nil
}

func (r *taskRepository) Update(ctx context.Context, id primitive.ObjectID, update models.TaskUpdate) error {
	updateDoc := bson.M{"$set": bson.M{"updated_at": time.Now()}}

	if update.Status != nil {
		updateDoc["$set"].(bson.M)["status"] = *update.Status
	}
	if update.CurrentDay != nil {
		updateDoc["$set"].(bson.M)["current_day"] = *update.CurrentDay
	}
	if update.NextActionAt != nil {
		updateDoc["$set"].(bson.M)["next_action_at"] = *update.NextActionAt
	}
	if update.ActionsCompleted != nil {
		updateDoc["$set"].(bson.M)["actions_completed"] = *update.ActionsCompleted
	}
	if update.ActionsFailed != nil {
		updateDoc["$set"].(bson.M)["actions_failed"] = *update.ActionsFailed
	}
	if update.LastError != nil {
		updateDoc["$set"].(bson.M)["last_error"] = *update.LastError
	}
	if update.CompletedAt != nil {
		updateDoc["$set"].(bson.M)["completed_at"] = *update.CompletedAt
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update warming task: %w", err)
	}

	return nil
}

func (r *taskRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	updateDoc := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == string(models.TaskStatusCompleted) {
		now := time.Now()
		updateDoc["$set"].(bson.M)["completed_at"] = &now
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

func (r *taskRepository) UpdateNextActionTime(ctx context.Context, id primitive.ObjectID, nextActionAt time.Time) error {
	updateDoc := bson.M{
		"$set": bson.M{
			"next_action_at": nextActionAt,
			"updated_at":     time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update next action time: %w", err)
	}

	return nil
}

func (r *taskRepository) IncrementCounters(ctx context.Context, id primitive.ObjectID, completed, failed int) error {
	updateDoc := bson.M{
		"$inc": bson.M{
			"actions_completed": completed,
			"actions_failed":    failed,
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to increment counters: %w", err)
	}

	return nil
}

func (r *taskRepository) List(ctx context.Context, filter models.TaskFilter) ([]*models.WarmingTask, error) {
	findFilter := bson.M{}

	if filter.Platform != "" {
		findFilter["platform"] = filter.Platform
	}
	if filter.Status != "" {
		findFilter["status"] = filter.Status
	}
	if filter.AccountID != nil {
		findFilter["account_id"] = *filter.AccountID
	}
	if filter.NextActionAt != nil {
		findFilter["next_action_at"] = bson.M{"$lte": *filter.NextActionAt}
	}

	findOptions := options.Find()
	if filter.Limit > 0 {
		findOptions.SetLimit(int64(filter.Limit))
	}
	if filter.Offset > 0 {
		findOptions.SetSkip(int64(filter.Offset))
	}
	findOptions.SetSort(bson.D{{"created_at", -1}})

	cursor, err := r.collection.Find(ctx, findFilter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list warming tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*models.WarmingTask
	if err = cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("failed to decode warming tasks: %w", err)
	}

	return tasks, nil
}

func (r *taskRepository) GetTasksForExecution(ctx context.Context, limit int) ([]*models.WarmingTask, error) {
	now := time.Now()
	filter := bson.M{
		"status": string(models.TaskStatusInProgress),
		"next_action_at": bson.M{"$lte": now},
	}

	findOptions := options.Find()
	if limit > 0 {
		findOptions.SetLimit(int64(limit))
	}
	findOptions.SetSort(bson.D{{"next_action_at", 1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks for execution: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*models.WarmingTask
	if err = cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("failed to decode tasks: %w", err)
	}

	return tasks, nil
}

func (r *taskRepository) GetStuckTasks(ctx context.Context, stuckDuration time.Duration) ([]*models.WarmingTask, error) {
	threshold := time.Now().Add(-stuckDuration)

	filter := bson.M{
		"status": string(models.TaskStatusInProgress),
		"updated_at": bson.M{"$lt": threshold},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get stuck tasks: %w", err)
	}
	defer cursor.Close(ctx)

	var tasks []*models.WarmingTask
	if err = cursor.All(ctx, &tasks); err != nil {
		return nil, fmt.Errorf("failed to decode stuck tasks: %w", err)
	}

	return tasks, nil
}

func (r *taskRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete warming task: %w", err)
	}

	return nil
}

func (r *taskRepository) Count(ctx context.Context, filter models.TaskFilter) (int64, error) {
	countFilter := bson.M{}

	if filter.Platform != "" {
		countFilter["platform"] = filter.Platform
	}
	if filter.Status != "" {
		countFilter["status"] = filter.Status
	}
	if filter.AccountID != nil {
		countFilter["account_id"] = *filter.AccountID
	}

	count, err := r.collection.CountDocuments(ctx, countFilter)
	if err != nil {
		return 0, fmt.Errorf("failed to count warming tasks: %w", err)
	}

	return count, nil
}
