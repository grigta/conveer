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

type ScenarioRepository interface {
	Create(ctx context.Context, scenario *models.WarmingScenario) error
	GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingScenario, error)
	GetByName(ctx context.Context, platform, name string) (*models.WarmingScenario, error)
	Update(ctx context.Context, id primitive.ObjectID, scenario *models.WarmingScenario) error
	List(ctx context.Context, platform string) ([]*models.WarmingScenario, error)
	Delete(ctx context.Context, id primitive.ObjectID) error
	SetActive(ctx context.Context, id primitive.ObjectID, active bool) error
}

type scenarioRepository struct {
	collection *mongo.Collection
}

func NewScenarioRepository(db *mongo.Database) ScenarioRepository {
	return &scenarioRepository{
		collection: db.Collection("warming_scenarios"),
	}
}

func (r *scenarioRepository) Create(ctx context.Context, scenario *models.WarmingScenario) error {
	scenario.CreatedAt = time.Now()
	scenario.UpdatedAt = time.Now()
	scenario.IsActive = true

	result, err := r.collection.InsertOne(ctx, scenario)
	if err != nil {
		return fmt.Errorf("failed to create scenario: %w", err)
	}

	scenario.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *scenarioRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.WarmingScenario, error) {
	var scenario models.WarmingScenario

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&scenario)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("scenario not found")
		}
		return nil, fmt.Errorf("failed to get scenario: %w", err)
	}

	return &scenario, nil
}

func (r *scenarioRepository) GetByName(ctx context.Context, platform, name string) (*models.WarmingScenario, error) {
	var scenario models.WarmingScenario

	filter := bson.M{
		"platform": platform,
		"name":     name,
		"is_active": true,
	}

	err := r.collection.FindOne(ctx, filter).Decode(&scenario)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get scenario: %w", err)
	}

	return &scenario, nil
}

func (r *scenarioRepository) Update(ctx context.Context, id primitive.ObjectID, scenario *models.WarmingScenario) error {
	scenario.UpdatedAt = time.Now()

	updateDoc := bson.M{
		"$set": bson.M{
			"name":        scenario.Name,
			"description": scenario.Description,
			"actions":     scenario.Actions,
			"schedule":    scenario.Schedule,
			"updated_at":  scenario.UpdatedAt,
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update scenario: %w", err)
	}

	return nil
}

func (r *scenarioRepository) List(ctx context.Context, platform string) ([]*models.WarmingScenario, error) {
	filter := bson.M{"is_active": true}
	if platform != "" {
		filter["platform"] = platform
	}

	findOptions := options.Find().SetSort(bson.D{{"created_at", -1}})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list scenarios: %w", err)
	}
	defer cursor.Close(ctx)

	var scenarios []*models.WarmingScenario
	if err = cursor.All(ctx, &scenarios); err != nil {
		return nil, fmt.Errorf("failed to decode scenarios: %w", err)
	}

	return scenarios, nil
}

func (r *scenarioRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	// Soft delete by setting is_active to false
	updateDoc := bson.M{
		"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to delete scenario: %w", err)
	}

	return nil
}

func (r *scenarioRepository) SetActive(ctx context.Context, id primitive.ObjectID, active bool) error {
	updateDoc := bson.M{
		"$set": bson.M{
			"is_active":  active,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update scenario active status: %w", err)
	}

	return nil
}
