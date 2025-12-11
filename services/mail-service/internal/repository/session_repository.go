package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/conveer/conveer/services/mail-service/internal/models"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SessionRepository handles session database operations
type SessionRepository struct {
	collection *mongo.Collection
	redis      *redis.Client
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *mongo.Database, redisClient *redis.Client) *SessionRepository {
	return &SessionRepository{
		collection: db.Collection("mail_sessions"),
		redis:      redisClient,
	}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *models.RegistrationSession) error {
	_, err := r.collection.InsertOne(ctx, session)
	if err != nil {
		return err
	}
	
	// Cache in Redis
	if r.redis != nil {
		key := fmt.Sprintf("mail:session:%s", session.AccountID.Hex())
		data, _ := json.Marshal(session)
		r.redis.Set(ctx, key, data, time.Hour)
	}
	
	return nil
}

// GetSession retrieves active session for account
func (r *SessionRepository) GetSession(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationSession, error) {
	// Check Redis cache first
	if r.redis != nil {
		key := fmt.Sprintf("mail:session:%s", accountID.Hex())
		data, err := r.redis.Get(ctx, key).Result()
		if err == nil {
			var session models.RegistrationSession
			if json.Unmarshal([]byte(data), &session) == nil {
				return &session, nil
			}
		}
	}
	
	// Get from MongoDB
	var session models.RegistrationSession
	err := r.collection.FindOne(ctx, bson.M{
		"account_id":   accountID,
		"completed_at": nil,
	}).Decode(&session)
	
	if err != nil {
		return nil, err
	}
	
	// Update cache
	if r.redis != nil {
		key := fmt.Sprintf("mail:session:%s", accountID.Hex())
		data, _ := json.Marshal(session)
		r.redis.Set(ctx, key, data, time.Hour)
	}
	
	return &session, nil
}

// UpdateSession updates session fields
func (r *SessionRepository) UpdateSession(ctx context.Context, accountID primitive.ObjectID, updates map[string]interface{}) error {
	updates["last_activity_at"] = time.Now()
	
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"account_id":   accountID,
			"completed_at": nil,
		},
		bson.M{"$set": updates},
	)
	
	// Invalidate cache
	if r.redis != nil {
		key := fmt.Sprintf("mail:session:%s", accountID.Hex())
		r.redis.Del(ctx, key)
	}
	
	return err
}

// UpdateStep updates current step and checkpoint
func (r *SessionRepository) UpdateStep(ctx context.Context, sessionID primitive.ObjectID, step models.RegistrationStep, checkpoint interface{}) error {
	update := bson.M{
		"current_step":     step,
		"last_activity_at": time.Now(),
	}
	
	if checkpoint != nil {
		update[fmt.Sprintf("step_checkpoints.%s", step)] = checkpoint
	}
	
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": sessionID},
		bson.M{"$set": update},
	)
	
	return err
}

// Complete marks session as completed
func (r *SessionRepository) Complete(ctx context.Context, sessionID primitive.ObjectID) error {
	now := time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": sessionID},
		bson.M{
			"$set": bson.M{
				"completed_at":     now,
				"last_activity_at": now,
				"current_step":     models.StepComplete,
			},
		},
	)
	
	return err
}

// Delete removes a session
func (r *SessionRepository) Delete(ctx context.Context, sessionID primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": sessionID})
	return err
}

// GetStuckSessions finds sessions stuck in same step
func (r *SessionRepository) GetStuckSessions(ctx context.Context, stuckDuration time.Duration) ([]*models.RegistrationSession, error) {
	threshold := time.Now().Add(-stuckDuration)
	
	cursor, err := r.collection.Find(ctx, bson.M{
		"completed_at":     nil,
		"last_activity_at": bson.M{"$lt": threshold},
		"current_step":     bson.M{"$ne": models.StepComplete},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var sessions []*models.RegistrationSession
	for cursor.Next(ctx) {
		var session models.RegistrationSession
		if err := cursor.Decode(&session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}
	
	return sessions, nil
}

// CleanupStuckSessions removes old stuck sessions
func (r *SessionRepository) CleanupStuckSessions(ctx context.Context, age time.Duration) error {
	threshold := time.Now().Add(-age)
	
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"completed_at":     nil,
		"last_activity_at": bson.M{"$lt": threshold},
	})
	
	return err
}

// CreateIndexes creates database indexes
func (r *SessionRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.M{"account_id": 1},
		},
		{
			Keys: bson.M{"current_step": 1},
		},
		{
			Keys: bson.M{"completed_at": 1},
		},
		{
			Keys: bson.M{"last_activity_at": 1},
		},
	}
	
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
