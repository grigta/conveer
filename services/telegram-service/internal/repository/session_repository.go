package repository

import (
	"context"
	"fmt"
	"time"

	"conveer/services/telegram-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SessionRepository struct {
	collection *mongo.Collection
}

func NewSessionRepository(db *mongo.Database) *SessionRepository {
	return &SessionRepository{
		collection: db.Collection("telegram_registration_sessions"),
	}
}

func (r *SessionRepository) Create(ctx context.Context, session *models.RegistrationSession) error {
	session.StartedAt = time.Now()
	session.LastActivityAt = time.Now()

	result, err := r.collection.InsertOne(ctx, session)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	session.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *SessionRepository) GetByAccountID(ctx context.Context, accountID primitive.ObjectID) (*models.RegistrationSession, error) {
	var session models.RegistrationSession

	err := r.collection.FindOne(ctx, bson.M{"account_id": accountID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (r *SessionRepository) Update(ctx context.Context, session *models.RegistrationSession) error {
	session.LastActivityAt = time.Now()

	filter := bson.M{"_id": session.ID}
	update := bson.M{"$set": session}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *SessionRepository) UpdateStep(ctx context.Context, sessionID primitive.ObjectID, step models.RegistrationStep, checkpoint interface{}) error {
	filter := bson.M{"_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"current_step":     step,
			"last_activity_at": time.Now(),
			fmt.Sprintf("step_checkpoints.%s", step): checkpoint,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update session step: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *SessionRepository) Complete(ctx context.Context, sessionID primitive.ObjectID) error {
	now := time.Now()
	filter := bson.M{"_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"current_step":     models.StepComplete,
			"completed_at":     now,
			"last_activity_at": now,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *SessionRepository) SetError(ctx context.Context, sessionID primitive.ObjectID, errorMessage string) error {
	filter := bson.M{"_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"last_error":       errorMessage,
			"last_activity_at": time.Now(),
		},
		"$inc": bson.M{"retry_count": 1},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to set session error: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

func (r *SessionRepository) CleanupOldSessions(ctx context.Context, maxAge time.Duration) error {
	filter := bson.M{
		"completed_at": bson.M{"$eq": nil},
		"last_activity_at": bson.M{
			"$lt": time.Now().Add(-maxAge),
		},
	}

	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to cleanup old sessions: %w", err)
	}

	if result.DeletedCount > 0 {
		// Log cleanup activity
		fmt.Printf("Cleaned up %d stale registration sessions\n", result.DeletedCount)
	}

	return nil
}

func (r *SessionRepository) GetStuckSessions(ctx context.Context, stuckDuration time.Duration) ([]*models.RegistrationSession, error) {
	filter := bson.M{
		"completed_at": bson.M{"$eq": nil},
		"last_activity_at": bson.M{
			"$lt": time.Now().Add(-stuckDuration),
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find stuck sessions: %w", err)
	}
	defer cursor.Close(ctx)

	var sessions []*models.RegistrationSession
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	return sessions, nil
}