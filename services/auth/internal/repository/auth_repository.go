package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/conveer/conveer/pkg/cache"
	"github.com/conveer/conveer/pkg/database"
	"github.com/conveer/conveer/pkg/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuthRepository struct {
	db    *database.MongoDB
	cache *cache.RedisCache
}

func NewAuthRepository(db *database.MongoDB, cache *cache.RedisCache) *AuthRepository {
	return &AuthRepository{
		db:    db,
		cache: cache,
	}
}

func (r *AuthRepository) CreateUser(ctx context.Context, user *models.User) error {
	_, err := r.db.InsertOne(ctx, "users", user)
	return err
}

func (r *AuthRepository) FindUserByID(ctx context.Context, id string) (*models.User, error) {
	cacheKey := fmt.Sprintf("user:%s", id)

	var user models.User
	if err := r.cache.GetJSON(ctx, cacheKey, &user); err == nil {
		return &user, nil
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": objectID}
	if err := r.db.FindOne(ctx, "users", filter, &user); err != nil {
		return nil, err
	}

	r.cache.Set(ctx, cacheKey, user, 5*time.Minute)

	return &user, nil
}

func (r *AuthRepository) FindUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	filter := bson.M{"email": email}
	if err := r.db.FindOne(ctx, "users", filter, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	filter := bson.M{"username": username}
	if err := r.db.FindOne(ctx, "users", filter, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) UpdateUserLastLogin(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"last_login_at": time.Now(),
			"updated_at":    time.Now(),
		},
	}

	_, err = r.db.UpdateOne(ctx, "users", filter, update)

	if err == nil {
		r.cache.Delete(ctx, fmt.Sprintf("user:%s", id))
	}

	return err
}

func (r *AuthRepository) UpdateUserPassword(ctx context.Context, id, hashedPassword string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		},
	}

	_, err = r.db.UpdateOne(ctx, "users", filter, update)

	if err == nil {
		r.cache.Delete(ctx, fmt.Sprintf("user:%s", id))
	}

	return err
}

func (r *AuthRepository) MarkUserAsVerified(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"is_verified": true,
			"updated_at":  time.Now(),
		},
	}

	_, err = r.db.UpdateOne(ctx, "users", filter, update)

	if err == nil {
		r.cache.Delete(ctx, fmt.Sprintf("user:%s", id))
	}

	return err
}

func (r *AuthRepository) CreateSession(ctx context.Context, session *models.Session) error {
	_, err := r.db.InsertOne(ctx, "sessions", session)
	if err != nil {
		return err
	}

	sessionData, _ := json.Marshal(session)
	r.cache.Set(ctx, fmt.Sprintf("session:token:%s", session.Token), sessionData, 24*time.Hour)
	r.cache.Set(ctx, fmt.Sprintf("session:refresh:%s", session.RefreshToken), sessionData, 24*time.Hour)

	return nil
}

func (r *AuthRepository) FindSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	cacheKey := fmt.Sprintf("session:token:%s", token)

	var session models.Session
	if err := r.cache.GetJSON(ctx, cacheKey, &session); err == nil {
		return &session, nil
	}

	filter := bson.M{"token": token}
	if err := r.db.FindOne(ctx, "sessions", filter, &session); err != nil {
		return nil, err
	}

	sessionData, _ := json.Marshal(session)
	r.cache.Set(ctx, cacheKey, sessionData, 1*time.Hour)

	return &session, nil
}

func (r *AuthRepository) FindSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	cacheKey := fmt.Sprintf("session:refresh:%s", refreshToken)

	var session models.Session
	if err := r.cache.GetJSON(ctx, cacheKey, &session); err == nil {
		return &session, nil
	}

	filter := bson.M{"refresh_token": refreshToken}
	if err := r.db.FindOne(ctx, "sessions", filter, &session); err != nil {
		return nil, err
	}

	sessionData, _ := json.Marshal(session)
	r.cache.Set(ctx, cacheKey, sessionData, 1*time.Hour)

	return &session, nil
}

func (r *AuthRepository) UpdateSession(ctx context.Context, session *models.Session) error {
	filter := bson.M{"_id": session.ID}
	update := bson.M{
		"$set": bson.M{
			"token":         session.Token,
			"refresh_token": session.RefreshToken,
			"expires_at":    session.ExpiresAt,
		},
	}

	_, err := r.db.UpdateOne(ctx, "sessions", filter, update)

	if err == nil {
		r.cache.Delete(ctx, fmt.Sprintf("session:token:%s", session.Token))
		r.cache.Delete(ctx, fmt.Sprintf("session:refresh:%s", session.RefreshToken))
	}

	return err
}

func (r *AuthRepository) DeleteSession(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	var session models.Session
	filter := bson.M{"_id": objectID}
	r.db.FindOne(ctx, "sessions", filter, &session)

	_, err = r.db.DeleteOne(ctx, "sessions", filter)

	if err == nil && session.Token != "" {
		r.cache.Delete(ctx, fmt.Sprintf("session:token:%s", session.Token))
		r.cache.Delete(ctx, fmt.Sprintf("session:refresh:%s", session.RefreshToken))
	}

	return err
}

func (r *AuthRepository) DeleteSessionByToken(ctx context.Context, token string) error {
	filter := bson.M{"token": token}

	var session models.Session
	r.db.FindOne(ctx, "sessions", filter, &session)

	_, err := r.db.DeleteOne(ctx, "sessions", filter)

	if err == nil {
		r.cache.Delete(ctx, fmt.Sprintf("session:token:%s", token))
		if session.RefreshToken != "" {
			r.cache.Delete(ctx, fmt.Sprintf("session:refresh:%s", session.RefreshToken))
		}
	}

	return err
}

func (r *AuthRepository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return err
	}

	filter := bson.M{"user_id": objectID}
	_, err = r.db.DeleteMany(ctx, "sessions", filter)

	return err
}

func (r *AuthRepository) CreatePasswordReset(ctx context.Context, reset *models.PasswordReset) error {
	_, err := r.db.InsertOne(ctx, "password_resets", reset)
	return err
}

func (r *AuthRepository) FindPasswordResetByToken(ctx context.Context, token string) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	filter := bson.M{"token": token}
	if err := r.db.FindOne(ctx, "password_resets", filter, &reset); err != nil {
		return nil, err
	}
	return &reset, nil
}

func (r *AuthRepository) UpdatePasswordReset(ctx context.Context, reset *models.PasswordReset) error {
	filter := bson.M{"_id": reset.ID}
	update := bson.M{
		"$set": bson.M{
			"used": reset.Used,
		},
	}

	_, err := r.db.UpdateOne(ctx, "password_resets", filter, update)
	return err
}

func (r *AuthRepository) CreateEmailVerification(ctx context.Context, verification *models.EmailVerification) error {
	_, err := r.db.InsertOne(ctx, "email_verifications", verification)
	return err
}

func (r *AuthRepository) FindEmailVerificationByToken(ctx context.Context, token string) (*models.EmailVerification, error) {
	var verification models.EmailVerification
	filter := bson.M{"token": token}
	if err := r.db.FindOne(ctx, "email_verifications", filter, &verification); err != nil {
		return nil, err
	}
	return &verification, nil
}

func (r *AuthRepository) UpdateEmailVerification(ctx context.Context, verification *models.EmailVerification) error {
	filter := bson.M{"_id": verification.ID}
	update := bson.M{
		"$set": bson.M{
			"verified": verification.Verified,
		},
	}

	_, err := r.db.UpdateOne(ctx, "email_verifications", filter, update)
	return err
}