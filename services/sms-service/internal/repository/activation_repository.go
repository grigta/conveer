package repository

import (
	"context"
	"fmt"
	"time"

	"conveer/sms-service/internal/models"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ActivationRepository struct {
	collection *mongo.Collection
	logger     *logrus.Logger
}

func NewActivationRepository(db *mongo.Database, logger *logrus.Logger) *ActivationRepository {
	return &ActivationRepository{
		collection: db.Collection("activations"),
		logger:     logger,
	}
}

func (r *ActivationRepository) Create(ctx context.Context, activation *models.Activation) error {
	activation.CreatedAt = time.Now()
	activation.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, activation)
	if err != nil {
		return fmt.Errorf("failed to insert activation: %w", err)
	}

	activation.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ActivationRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Activation, error) {
	var activation models.Activation
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&activation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find activation: %w", err)
	}

	return &activation, nil
}

func (r *ActivationRepository) FindByActivationID(ctx context.Context, activationID string) (*models.Activation, error) {
	var activation models.Activation
	err := r.collection.FindOne(ctx, bson.M{"activation_id": activationID}).Decode(&activation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find activation: %w", err)
	}

	return &activation, nil
}

func (r *ActivationRepository) FindByUserID(ctx context.Context, userID string, limit int64) ([]*models.Activation, error) {
	filter := bson.M{"user_id": userID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(limit)

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find activations: %w", err)
	}
	defer cursor.Close(ctx)

	var activations []*models.Activation
	for cursor.Next(ctx) {
		var activation models.Activation
		if err := cursor.Decode(&activation); err != nil {
			return nil, fmt.Errorf("failed to decode activation: %w", err)
		}
		activations = append(activations, &activation)
	}

	return activations, nil
}

func (r *ActivationRepository) FindPending(ctx context.Context) ([]*models.Activation, error) {
	filter := bson.M{
		"status": bson.M{"$in": []models.ActivationStatus{
			models.ActivationStatusPending,
			models.ActivationStatusWaiting,
		}},
		"expires_at": bson.M{"$gt": time.Now()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find pending activations: %w", err)
	}
	defer cursor.Close(ctx)

	var activations []*models.Activation
	for cursor.Next(ctx) {
		var activation models.Activation
		if err := cursor.Decode(&activation); err != nil {
			return nil, fmt.Errorf("failed to decode activation: %w", err)
		}
		activations = append(activations, &activation)
	}

	return activations, nil
}

func (r *ActivationRepository) Update(ctx context.Context, activation *models.Activation) error {
	activation.UpdatedAt = time.Now()

	filter := bson.M{"_id": activation.ID}
	update := bson.M{"$set": activation}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update activation: %w", err)
	}

	return nil
}

func (r *ActivationRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.ActivationStatus) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == models.ActivationStatusCompleted {
		now := time.Now()
		update["$set"].(bson.M)["completed_at"] = &now
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update activation status: %w", err)
	}

	return nil
}

func (r *ActivationRepository) UpdateCode(ctx context.Context, activationID, code, fullSMS string) error {
	now := time.Now()
	filter := bson.M{"activation_id": activationID}
	update := bson.M{
		"$set": bson.M{
			"code":              code,
			"full_sms":          fullSMS,
			"code_received_at":  &now,
			"status":            models.ActivationStatusReceived,
			"updated_at":        time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update activation code: %w", err)
	}

	return nil
}

func (r *ActivationRepository) CancelActivation(ctx context.Context, activationID, reason string, refunded bool, refundAmount float64) error {
	now := time.Now()
	filter := bson.M{"activation_id": activationID}
	update := bson.M{
		"$set": bson.M{
			"status":            models.ActivationStatusCancelled,
			"cancellation_note": reason,
			"cancelled_at":      &now,
			"refunded":          refunded,
			"refund_amount":     refundAmount,
			"updated_at":        time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to cancel activation: %w", err)
	}

	return nil
}

func (r *ActivationRepository) IncrementRetryCount(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	filter := bson.M{"_id": id}
	update := bson.M{
		"$inc": bson.M{"retry_count": 1},
		"$set": bson.M{
			"last_retry_at": &now,
			"updated_at":    time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	return nil
}

func (r *ActivationRepository) FindExpired(ctx context.Context) ([]*models.Activation, error) {
	filter := bson.M{
		"status": bson.M{"$in": []models.ActivationStatus{
			models.ActivationStatusPending,
			models.ActivationStatusWaiting,
		}},
		"expires_at": bson.M{"$lt": time.Now()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find expired activations: %w", err)
	}
	defer cursor.Close(ctx)

	var activations []*models.Activation
	for cursor.Next(ctx) {
		var activation models.Activation
		if err := cursor.Decode(&activation); err != nil {
			return nil, fmt.Errorf("failed to decode activation: %w", err)
		}
		activations = append(activations, &activation)
	}

	return activations, nil
}

func (r *ActivationRepository) GetStatistics(ctx context.Context, filter bson.M) (*models.GetStatisticsResponse, error) {
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": nil,
			"total_activations": bson.M{"$sum": 1},
			"successful_activations": bson.M{
				"$sum": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []interface{}{"$status", models.ActivationStatusCompleted}},
						1, 0,
					},
				},
			},
			"failed_activations": bson.M{
				"$sum": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []interface{}{"$status", models.ActivationStatusFailed}},
						1, 0,
					},
				},
			},
			"cancelled_activations": bson.M{
				"$sum": bson.M{
					"$cond": []interface{}{
						bson.M{"$eq": []interface{}{"$status", models.ActivationStatusCancelled}},
						1, 0,
					},
				},
			},
			"total_spent": bson.M{"$sum": "$price"},
			"average_price": bson.M{"$avg": "$price"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}
	defer cursor.Close(ctx)

	stats := &models.GetStatisticsResponse{
		ByService:  make(map[string]int32),
		ByCountry:  make(map[string]int32),
		ByProvider: make(map[string]float32),
	}

	if cursor.Next(ctx) {
		var result struct {
			TotalActivations      int32   `bson:"total_activations"`
			SuccessfulActivations int32   `bson:"successful_activations"`
			FailedActivations     int32   `bson:"failed_activations"`
			CancelledActivations  int32   `bson:"cancelled_activations"`
			TotalSpent            float32 `bson:"total_spent"`
			AveragePrice          float32 `bson:"average_price"`
		}

		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode statistics: %w", err)
		}

		stats.TotalActivations = result.TotalActivations
		stats.SuccessfulActivations = result.SuccessfulActivations
		stats.FailedActivations = result.FailedActivations
		stats.CancelledActivations = result.CancelledActivations
		stats.TotalSpent = result.TotalSpent
		stats.AveragePrice = result.AveragePrice
	}

	// Get statistics by service
	servicePipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":   "$service",
			"count": bson.M{"$sum": 1},
		}},
	}

	serviceCursor, err := r.collection.Aggregate(ctx, servicePipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get service statistics: %w", err)
	}
	defer serviceCursor.Close(ctx)

	for serviceCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int32  `bson:"count"`
		}
		if err := serviceCursor.Decode(&result); err != nil {
			continue
		}
		stats.ByService[result.ID] = result.Count
	}

	// Get statistics by country
	countryPipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":   "$country",
			"count": bson.M{"$sum": 1},
		}},
	}

	countryCursor, err := r.collection.Aggregate(ctx, countryPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get country statistics: %w", err)
	}
	defer countryCursor.Close(ctx)

	for countryCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int32  `bson:"count"`
		}
		if err := countryCursor.Decode(&result); err != nil {
			continue
		}
		stats.ByCountry[result.ID] = result.Count
	}

	// Get statistics by provider
	providerPipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":   "$provider",
			"spent": bson.M{"$sum": "$price"},
		}},
	}

	providerCursor, err := r.collection.Aggregate(ctx, providerPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider statistics: %w", err)
	}
	defer providerCursor.Close(ctx)

	for providerCursor.Next(ctx) {
		var result struct {
			ID    string  `bson:"_id"`
			Spent float32 `bson:"spent"`
		}
		if err := providerCursor.Decode(&result); err != nil {
			continue
		}
		stats.ByProvider[result.ID] = result.Spent
	}

	return stats, nil
}

func (r *ActivationRepository) CreateIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "activation_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "expires_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "phone_id", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
