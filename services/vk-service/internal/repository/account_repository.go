package repository

import (
	"context"
	"fmt"
	"time"

	"conveer/pkg/crypto"
	"conveer/pkg/logger"
	"conveer/services/vk-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AccountRepository interface {
	CreateAccount(ctx context.Context, account *models.VKAccount) error
	GetAccountByID(ctx context.Context, id primitive.ObjectID) (*models.VKAccount, error)
	GetAccountByPhone(ctx context.Context, phone string) (*models.VKAccount, error)
	UpdateAccountStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus, errorMsg string) error
	UpdateAccountCredentials(ctx context.Context, id primitive.ObjectID, cookies []byte, userID string) error
	UpdateAccountFullCredentials(ctx context.Context, id primitive.ObjectID, phone, password string, cookies []byte, userID string, status models.AccountStatus) error
	GetAccountsByStatus(ctx context.Context, status models.AccountStatus, limit int64) ([]*models.VKAccount, error)
	IncrementRetryCount(ctx context.Context, id primitive.ObjectID) error
	GetAccountStatistics(ctx context.Context) (*models.AccountStatistics, error)
	CreateIndexes(ctx context.Context) error
	UpdateAccount(ctx context.Context, id primitive.ObjectID, update bson.M) error
	GetStuckAccounts(ctx context.Context, duration time.Duration) ([]*models.VKAccount, error)
	DeleteAccount(ctx context.Context, id primitive.ObjectID) error
}

type accountRepository struct {
	db        *mongo.Database
	encryptor crypto.Encryptor
	logger    logger.Logger
}

func NewAccountRepository(db *mongo.Database, encryptor crypto.Encryptor, logger logger.Logger) AccountRepository {
	return &accountRepository{
		db:        db,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (r *accountRepository) collection() *mongo.Collection {
	return r.db.Collection("vk_accounts")
}

func (r *accountRepository) CreateAccount(ctx context.Context, account *models.VKAccount) error {
	if account.Phone != "" {
		encrypted, err := r.encryptor.Encrypt(account.Phone)
		if err != nil {
			return fmt.Errorf("failed to encrypt phone: %w", err)
		}
		account.Phone = encrypted
	}

	if account.Email != "" {
		encrypted, err := r.encryptor.Encrypt(account.Email)
		if err != nil {
			return fmt.Errorf("failed to encrypt email: %w", err)
		}
		account.Email = encrypted
	}

	if account.Password != "" {
		encrypted, err := r.encryptor.Encrypt(account.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		account.Password = encrypted
	}

	if len(account.Cookies) > 0 {
		encrypted, err := r.encryptor.EncryptBytes(account.Cookies)
		if err != nil {
			return fmt.Errorf("failed to encrypt cookies: %w", err)
		}
		account.Cookies = encrypted
	}

	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()
	account.Status = models.StatusCreating

	result, err := r.collection().InsertOne(ctx, account)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	account.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *accountRepository) GetAccountByID(ctx context.Context, id primitive.ObjectID) (*models.VKAccount, error) {
	var account models.VKAccount
	err := r.collection().FindOne(ctx, bson.M{"_id": id}).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("account not found")
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	if err := r.decryptAccount(&account); err != nil {
		return nil, fmt.Errorf("failed to decrypt account: %w", err)
	}

	return &account, nil
}

func (r *accountRepository) GetAccountByPhone(ctx context.Context, phone string) (*models.VKAccount, error) {
	encrypted, err := r.encryptor.Encrypt(phone)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt phone for search: %w", err)
	}

	var account models.VKAccount
	err = r.collection().FindOne(ctx, bson.M{"phone": encrypted}).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account by phone: %w", err)
	}

	if err := r.decryptAccount(&account); err != nil {
		return nil, fmt.Errorf("failed to decrypt account: %w", err)
	}

	return &account, nil
}

func (r *accountRepository) UpdateAccountStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus, errorMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if errorMsg != "" {
		update["$set"].(bson.M)["error_message"] = errorMsg
	}

	_, err := r.collection().UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("failed to update account status: %w", err)
	}

	return nil
}

func (r *accountRepository) UpdateAccountCredentials(ctx context.Context, id primitive.ObjectID, cookies []byte, userID string) error {
	encryptedCookies, err := r.encryptor.EncryptBytes(cookies)
	if err != nil {
		return fmt.Errorf("failed to encrypt cookies: %w", err)
	}

	update := bson.M{
		"$set": bson.M{
			"cookies":    encryptedCookies,
			"user_id":    userID,
			"updated_at": time.Now(),
		},
	}

	_, err = r.collection().UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("failed to update account credentials: %w", err)
	}

	return nil
}

func (r *accountRepository) UpdateAccountFullCredentials(ctx context.Context, id primitive.ObjectID, phone, password string, cookies []byte, userID string, status models.AccountStatus) error {
	update := bson.M{
		"user_id":    userID,
		"status":     status,
		"updated_at": time.Now(),
	}

	// Encrypt phone if provided
	if phone != "" {
		encryptedPhone, err := r.encryptor.Encrypt(phone)
		if err != nil {
			return fmt.Errorf("failed to encrypt phone: %w", err)
		}
		update["phone"] = encryptedPhone
	}

	// Encrypt password if provided
	if password != "" {
		encryptedPassword, err := r.encryptor.Encrypt(password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		update["password"] = encryptedPassword
	}

	// Encrypt cookies if provided
	if len(cookies) > 0 {
		encryptedCookies, err := r.encryptor.EncryptBytes(cookies)
		if err != nil {
			return fmt.Errorf("failed to encrypt cookies: %w", err)
		}
		update["cookies"] = encryptedCookies
	}

	_, err := r.collection().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("failed to update account full credentials: %w", err)
	}

	return nil
}

func (r *accountRepository) GetAccountsByStatus(ctx context.Context, status models.AccountStatus, limit int64) ([]*models.VKAccount, error) {
	opts := options.Find().SetLimit(limit).SetSort(bson.M{"created_at": -1})
	cursor, err := r.collection().Find(ctx, bson.M{"status": status}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by status: %w", err)
	}
	defer cursor.Close(ctx)

	var accounts []*models.VKAccount
	for cursor.Next(ctx) {
		var account models.VKAccount
		if err := cursor.Decode(&account); err != nil {
			r.logger.Error("Failed to decode account", "error", err)
			continue
		}

		if err := r.decryptAccount(&account); err != nil {
			r.logger.Error("Failed to decrypt account", "error", err, "account_id", account.ID)
			continue
		}

		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func (r *accountRepository) IncrementRetryCount(ctx context.Context, id primitive.ObjectID) error {
	update := bson.M{
		"$inc": bson.M{"retry_count": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	_, err := r.collection().UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	return nil
}

func (r *accountRepository) GetAccountStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	stats := &models.AccountStatistics{
		ByStatus: make(map[models.AccountStatus]int64),
	}

	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"total": []bson.M{
					{"$count": "count"},
				},
				"byStatus": []bson.M{
					{"$group": bson.M{
						"_id":   "$status",
						"count": bson.M{"$sum": 1},
					}},
				},
				"successRate": []bson.M{
					{"$group": bson.M{
						"_id": nil,
						"success": bson.M{
							"$sum": bson.M{
								"$cond": []interface{}{
									bson.M{"$in": []interface{}{"$status", []models.AccountStatus{models.StatusCreated, models.StatusWarming, models.StatusReady}}},
									1, 0,
								},
							},
						},
						"total": bson.M{"$sum": 1},
					}},
				},
				"avgRetries": []bson.M{
					{"$group": bson.M{
						"_id": nil,
						"avg": bson.M{"$avg": "$retry_count"},
					}},
				},
				"lastHour": []bson.M{
					{"$match": bson.M{
						"created_at": bson.M{"$gte": time.Now().Add(-1 * time.Hour)},
					}},
					{"$count": "count"},
				},
				"last24Hours": []bson.M{
					{"$match": bson.M{
						"created_at": bson.M{"$gte": time.Now().Add(-24 * time.Hour)},
					}},
					{"$count": "count"},
				},
			},
		},
	}

	cursor, err := r.collection().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode statistics: %w", err)
	}

	if len(results) > 0 {
		result := results[0]

		if total, ok := result["total"].([]interface{}); ok && len(total) > 0 {
			if t, ok := total[0].(bson.M)["count"].(int32); ok {
				stats.Total = int64(t)
			}
		}

		if byStatus, ok := result["byStatus"].([]interface{}); ok {
			for _, s := range byStatus {
				if statusMap, ok := s.(bson.M); ok {
					if status, ok := statusMap["_id"].(string); ok {
						if count, ok := statusMap["count"].(int32); ok {
							stats.ByStatus[models.AccountStatus(status)] = int64(count)
						}
					}
				}
			}
		}

		if successRate, ok := result["successRate"].([]interface{}); ok && len(successRate) > 0 {
			if sr, ok := successRate[0].(bson.M); ok {
				success := float64(sr["success"].(int32))
				total := float64(sr["total"].(int32))
				if total > 0 {
					stats.SuccessRate = success / total * 100
				}
			}
		}

		if avgRetries, ok := result["avgRetries"].([]interface{}); ok && len(avgRetries) > 0 {
			if ar, ok := avgRetries[0].(bson.M)["avg"].(float64); ok {
				stats.AverageRetries = ar
			}
		}

		if lastHour, ok := result["lastHour"].([]interface{}); ok && len(lastHour) > 0 {
			if lh, ok := lastHour[0].(bson.M)["count"].(int32); ok {
				stats.LastHour = int64(lh)
			}
		}

		if last24Hours, ok := result["last24Hours"].([]interface{}); ok && len(last24Hours) > 0 {
			if l24, ok := last24Hours[0].(bson.M)["count"].(int32); ok {
				stats.Last24Hours = int64(l24)
			}
		}
	}

	return stats, nil
}

func (r *accountRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"phone": 1},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.M{"status": 1},
		},
		{
			Keys: bson.M{"proxy_id": 1},
		},
		{
			Keys: bson.M{"created_at": -1},
		},
		{
			Keys: bson.M{"user_id": 1},
			Options: options.Index().SetSparse(true),
		},
	}

	_, err := r.collection().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func (r *accountRepository) UpdateAccount(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()

	_, err := r.collection().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("failed to update account: %w", err)
	}

	return nil
}

func (r *accountRepository) GetStuckAccounts(ctx context.Context, duration time.Duration) ([]*models.VKAccount, error) {
	filter := bson.M{
		"status": models.StatusCreating,
		"updated_at": bson.M{"$lt": time.Now().Add(-duration)},
	}

	cursor, err := r.collection().Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get stuck accounts: %w", err)
	}
	defer cursor.Close(ctx)

	var accounts []*models.VKAccount
	for cursor.Next(ctx) {
		var account models.VKAccount
		if err := cursor.Decode(&account); err != nil {
			r.logger.Error("Failed to decode account", "error", err)
			continue
		}

		if err := r.decryptAccount(&account); err != nil {
			r.logger.Error("Failed to decrypt account", "error", err, "account_id", account.ID)
			continue
		}

		accounts = append(accounts, &account)
	}

	return accounts, nil
}

func (r *accountRepository) DeleteAccount(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection().DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	return nil
}

func (r *accountRepository) decryptAccount(account *models.VKAccount) error {
	if account.Phone != "" {
		decrypted, err := r.encryptor.Decrypt(account.Phone)
		if err != nil {
			return fmt.Errorf("failed to decrypt phone: %w", err)
		}
		account.Phone = decrypted
	}

	if account.Email != "" {
		decrypted, err := r.encryptor.Decrypt(account.Email)
		if err != nil {
			return fmt.Errorf("failed to decrypt email: %w", err)
		}
		account.Email = decrypted
	}

	if account.Password != "" {
		decrypted, err := r.encryptor.Decrypt(account.Password)
		if err != nil {
			return fmt.Errorf("failed to decrypt password: %w", err)
		}
		account.Password = decrypted
	}

	if len(account.Cookies) > 0 {
		decrypted, err := r.encryptor.DecryptBytes(account.Cookies)
		if err != nil {
			return fmt.Errorf("failed to decrypt cookies: %w", err)
		}
		account.Cookies = decrypted
	}

	return nil
}
