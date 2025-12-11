package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/conveer/telegram-bot/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRepository interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.TelegramBotUser, error)
	Create(ctx context.Context, user *models.TelegramBotUser) error
	Update(ctx context.Context, telegramID int64, updates map[string]interface{}) error
	List(ctx context.Context, filter map[string]interface{}) ([]*models.TelegramBotUser, error)
	Delete(ctx context.Context, telegramID int64) error
	CreateIndexes(ctx context.Context) error
}

type userRepository struct {
	collection *mongo.Collection
}

func NewUserRepository(db *mongo.Database) UserRepository {
	return &userRepository{
		collection: db.Collection("telegram_bot_users"),
	}
}

func (r *userRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.TelegramBotUser, error) {
	var user models.TelegramBotUser
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, models.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.TelegramBotUser) error {
	if user.ID.IsZero() {
		user.ID = primitive.NewObjectID()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.UpdatedAt = time.Now()

	if err := user.Validate(); err != nil {
		return err
	}

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return fmt.Errorf("user with telegram_id %d already exists", user.TelegramID)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, telegramID int64, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"telegram_id": telegramID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) List(ctx context.Context, filter map[string]interface{}) ([]*models.TelegramBotUser, error) {
	if filter == nil {
		filter = make(map[string]interface{})
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer cursor.Close(ctx)

	var users []*models.TelegramBotUser
	if err := cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	return users, nil
}

func (r *userRepository) Delete(ctx context.Context, telegramID int64) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"telegram_id": telegramID})
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if result.DeletedCount == 0 {
		return models.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "telegram_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "role", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "is_active", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "whitelist", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	return nil
}