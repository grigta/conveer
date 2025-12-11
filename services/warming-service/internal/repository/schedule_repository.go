package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/grigta/conveer/services/warming-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ScheduleRepository interface {
	Create(ctx context.Context, schedule *models.WarmingSchedule) error
	GetByTaskID(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingSchedule, error)
	UpdatePlannedActions(ctx context.Context, taskID primitive.ObjectID, actions []models.PlannedAction) error
	GetTodayActions(ctx context.Context, taskID primitive.ObjectID, actionType string) ([]models.PlannedAction, error)
	Delete(ctx context.Context, taskID primitive.ObjectID) error
}

type scheduleRepository struct {
	collection *mongo.Collection
}

func NewScheduleRepository(db *mongo.Database) ScheduleRepository {
	return &scheduleRepository{
		collection: db.Collection("warming_schedules"),
	}
}

func (r *scheduleRepository) Create(ctx context.Context, schedule *models.WarmingSchedule) error {
	if schedule.ID.IsZero() {
		schedule.ID = primitive.NewObjectID()
	}
	schedule.CreatedAt = time.Now()
	schedule.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, schedule)
	return err
}

func (r *scheduleRepository) GetByTaskID(ctx context.Context, taskID primitive.ObjectID) (*models.WarmingSchedule, error) {
	var schedule models.WarmingSchedule
	err := r.collection.FindOne(ctx, bson.M{"task_id": taskID}).Decode(&schedule)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &schedule, nil
}

func (r *scheduleRepository) UpdatePlannedActions(ctx context.Context, taskID primitive.ObjectID, actions []models.PlannedAction) error {
	update := bson.M{
		"$set": bson.M{
			"planned_actions": actions,
			"updated_at":      time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"task_id": taskID}, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("schedule not found for task %s", taskID.Hex())
	}

	return nil
}

func (r *scheduleRepository) GetTodayActions(ctx context.Context, taskID primitive.ObjectID, actionType string) ([]models.PlannedAction, error) {
	schedule, err := r.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if schedule == nil {
		return []models.PlannedAction{}, nil
	}

	today := time.Now().Format("2006-01-02")
	var todayActions []models.PlannedAction

	for _, action := range schedule.PlannedActions {
		if action.PlannedAt.Format("2006-01-02") == today {
			if actionType == "" || action.ActionType == actionType {
				todayActions = append(todayActions, action)
			}
		}
	}

	return todayActions, nil
}

func (r *scheduleRepository) Delete(ctx context.Context, taskID primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"task_id": taskID})
	return err
}
