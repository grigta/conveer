package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grigta/conveer/pkg/logger"
	"github.com/grigta/conveer/services/vk-service/internal/models"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SessionRepository interface {
	SaveSession(ctx context.Context, session *models.RegistrationSession) error
	GetSession(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationSession, error)
	UpdateSession(ctx context.Context, accountID primitive.ObjectID, updates bson.M) error
	DeleteSession(ctx context.Context, accountID primitive.ObjectID) error
	GetActiveSessions(ctx context.Context) ([]*models.RegistrationSession, error)
	SaveBrowserContext(ctx context.Context, accountID primitive.ObjectID, context map[string]interface{}) error
	GetBrowserContext(ctx context.Context, accountID primitive.ObjectID) (map[string]interface{}, error)
	CleanupExpiredSessions(ctx context.Context, expiry time.Duration) (int64, error)
}

type sessionRepository struct {
	db     *mongo.Database
	redis  *redis.Client
	logger logger.Logger
}

func NewSessionRepository(db *mongo.Database, redisClient *redis.Client, logger logger.Logger) SessionRepository {
	return &sessionRepository{
		db:     db,
		redis:  redisClient,
		logger: logger,
	}
}

func (r *sessionRepository) collection() *mongo.Collection {
	return r.db.GetCollection("vk_registration_sessions")
}

func (r *sessionRepository) SaveSession(ctx context.Context, session *models.RegistrationSession) error {
	if session.ID.IsZero() {
		session.ID = primitive.NewObjectID()
	}

	session.LastActivityAt = time.Now()

	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}

	_, err := r.collection().InsertOne(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Cache browser context in Redis for quick access
	if session.BrowserContext != nil {
		if err := r.SaveBrowserContext(ctx, session.AccountID, session.BrowserContext); err != nil {
			r.logger.Warn("Failed to cache browser context", "error", err, "account_id", session.AccountID)
		}
	}

	return nil
}

func (r *sessionRepository) GetSession(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationSession, error) {
	var session models.RegistrationSession
	err := r.collection().FindOne(ctx, bson.M{"account_id": accountID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Try to get browser context from Redis cache
	if cachedContext, err := r.GetBrowserContext(ctx, accountID); err == nil && cachedContext != nil {
		session.BrowserContext = cachedContext
	}

	return &session, nil
}

func (r *sessionRepository) UpdateSession(ctx context.Context, accountID primitive.ObjectID, updates bson.M) error {
	updates["last_activity_at"] = time.Now()

	_, err := r.collection().UpdateOne(
		ctx,
		bson.M{"account_id": accountID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

func (r *sessionRepository) DeleteSession(ctx context.Context, accountID primitive.ObjectID) error {
	_, err := r.collection().DeleteOne(ctx, bson.M{"account_id": accountID})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Clean up Redis cache
	key := fmt.Sprintf("vk:browser_context:%s", accountID.Hex())
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		r.logger.Warn("Failed to delete browser context from cache", "error", err, "account_id", accountID)
	}

	return nil
}

func (r *sessionRepository) GetActiveSessions(ctx context.Context) ([]*models.RegistrationSession, error) {
	filter := bson.M{
		"completed_at": nil,
		"current_step": bson.M{"$ne": models.StepComplete},
	}

	cursor, err := r.collection().Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	defer cursor.Close(ctx)

	var sessions []*models.RegistrationSession
	for cursor.Next(ctx) {
		var session models.RegistrationSession
		if err := cursor.Decode(&session); err != nil {
			r.logger.Error("Failed to decode session", "error", err)
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *sessionRepository) SaveBrowserContext(ctx context.Context, accountID primitive.ObjectID, context map[string]interface{}) error {
	key := fmt.Sprintf("vk:browser_context:%s", accountID.Hex())

	data, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal browser context: %w", err)
	}

	// Store with 1 hour expiry
	if err := r.redis.Set(ctx, key, data, 1*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to save browser context to Redis: %w", err)
	}

	// Also update in MongoDB
	_, err = r.collection().UpdateOne(
		ctx,
		bson.M{"account_id": accountID},
		bson.M{"$set": bson.M{
			"browser_context": context,
			"last_activity_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update browser context in MongoDB: %w", err)
	}

	return nil
}

func (r *sessionRepository) GetBrowserContext(ctx context.Context, accountID primitive.ObjectID) (map[string]interface{}, error) {
	key := fmt.Sprintf("vk:browser_context:%s", accountID.Hex())

	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Try to get from MongoDB
			var session models.RegistrationSession
			err := r.collection().FindOne(ctx, bson.M{"account_id": accountID}).Decode(&session)
			if err != nil {
				return nil, nil
			}
			return session.BrowserContext, nil
		}
		return nil, fmt.Errorf("failed to get browser context from Redis: %w", err)
	}

	var context map[string]interface{}
	if err := json.Unmarshal([]byte(data), &context); err != nil {
		return nil, fmt.Errorf("failed to unmarshal browser context: %w", err)
	}

	return context, nil
}

func (r *sessionRepository) CleanupExpiredSessions(ctx context.Context, expiry time.Duration) (int64, error) {
	filter := bson.M{
		"last_activity_at": bson.M{"$lt": time.Now().Add(-expiry)},
		"completed_at": nil,
	}

	result, err := r.collection().DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}

	// Clean up Redis keys
	pattern := "vk:browser_context:*"
	iter := r.redis.Scan(ctx, 0, pattern, 0).Iterator()
	deletedCount := int64(0)

	for iter.Next(ctx) {
		key := iter.Val()
		if err := r.redis.Del(ctx, key).Err(); err != nil {
			r.logger.Warn("Failed to delete expired Redis key", "key", key, "error", err)
		} else {
			deletedCount++
		}
	}

	if err := iter.Err(); err != nil {
		r.logger.Error("Error during Redis scan", "error", err)
	}

	r.logger.Info("Cleaned up expired sessions",
		"mongodb_deleted", result.DeletedCount,
		"redis_deleted", deletedCount)

	return result.DeletedCount, nil
}
