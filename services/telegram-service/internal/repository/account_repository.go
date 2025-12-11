package repository

import (
	"context"
	"fmt"
	"time"

	"conveer/services/telegram-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AccountRepository struct {
	collection *mongo.Collection
}

func NewAccountRepository(db *mongo.Database) *AccountRepository {
	return &AccountRepository{
		collection: db.Collection("telegram_accounts"),
	}
}

func (r *AccountRepository) Create(ctx context.Context, account *models.TelegramAccount) error {
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, account)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	account.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.TelegramAccount, error) {
	var account models.TelegramAccount

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	return &account, nil
}

func (r *AccountRepository) GetByPhone(ctx context.Context, phone string) (*models.TelegramAccount, error) {
	var account models.TelegramAccount

	err := r.collection.FindOne(ctx, bson.M{"phone": phone}).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account by phone: %w", err)
	}

	return &account, nil
}

func (r *AccountRepository) ListByStatus(ctx context.Context, status models.AccountStatus, limit, offset int) ([]*models.TelegramAccount, int64, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count accounts: %w", err)
	}

	// Get paginated results
	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(offset))
	findOptions.SetSort(bson.M{"created_at": -1})

	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list accounts: %w", err)
	}
	defer cursor.Close(ctx)

	var accounts []*models.TelegramAccount
	if err := cursor.All(ctx, &accounts); err != nil {
		return nil, 0, fmt.Errorf("failed to decode accounts: %w", err)
	}

	return accounts, total, nil
}

func (r *AccountRepository) Update(ctx context.Context, account *models.TelegramAccount) error {
	account.UpdatedAt = time.Now()

	filter := bson.M{"_id": account.ID}
	update := bson.M{"$set": account}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *AccountRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus, errorMessage string) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":        status,
			"error_message": errorMessage,
			"updated_at":    time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update account status: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *AccountRepository) IncrementRetryCount(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$inc": bson.M{"retry_count": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *AccountRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

func (r *AccountRepository) GetStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	stats := &models.AccountStatistics{
		ByStatus: make(map[models.AccountStatus]int64),
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats.Total = total

	// Get counts by status
	statuses := []models.AccountStatus{
		models.StatusCreating,
		models.StatusCreated,
		models.StatusWarming,
		models.StatusReady,
		models.StatusBanned,
		models.StatusError,
		models.StatusSuspended,
	}

	for _, status := range statuses {
		count, err := r.collection.CountDocuments(ctx, bson.M{"status": status})
		if err != nil {
			return nil, fmt.Errorf("failed to count status %s: %w", status, err)
		}
		stats.ByStatus[status] = count
	}

	// Calculate success rate
	successCount := stats.ByStatus[models.StatusCreated] +
					stats.ByStatus[models.StatusWarming] +
					stats.ByStatus[models.StatusReady]
	if total > 0 {
		stats.SuccessRate = float64(successCount) / float64(total) * 100
	}

	// Get average retry count
	pipeline := []bson.M{
		{"$group": bson.M{
			"_id": nil,
			"avg_retries": bson.M{"$avg": "$retry_count"},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get average retries: %w", err)
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err := cursor.All(ctx, &result); err != nil {
		return nil, fmt.Errorf("failed to decode average retries: %w", err)
	}

	if len(result) > 0 {
		if avgRetries, ok := result[0]["avg_retries"].(float64); ok {
			stats.AverageRetries = avgRetries
		}
	}

	// Get counts for last hour and 24 hours
	now := time.Now()
	lastHour := now.Add(-time.Hour)
	last24Hours := now.Add(-24 * time.Hour)

	stats.LastHour, _ = r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": lastHour},
	})

	stats.Last24Hours, _ = r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": last24Hours},
	})

	return stats, nil
}