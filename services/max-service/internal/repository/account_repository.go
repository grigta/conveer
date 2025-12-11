package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/grigta/conveer/services/max-service/internal/models"
	"github.com/grigta/conveer/pkg/crypto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AccountRepository handles account database operations
type AccountRepository struct {
	collection *mongo.Collection
	encryptor  *crypto.Encryptor
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *mongo.Database, encryptor *crypto.Encryptor) *AccountRepository {
	return &AccountRepository{
		collection: db.Collection("max_accounts"),
		encryptor:  encryptor,
	}
}

// Create creates a new account
func (r *AccountRepository) Create(ctx context.Context, account *models.MaxAccount) error {
	// Encrypt sensitive fields
	if account.VKAccessToken != "" {
		encrypted, err := r.encryptor.Encrypt(account.VKAccessToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt vk access token: %w", err)
		}
		account.VKAccessToken = encrypted
	}

	if account.MaxSessionToken != "" {
		encrypted, err := r.encryptor.Encrypt(account.MaxSessionToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt max session token: %w", err)
		}
		account.MaxSessionToken = encrypted
	}
	
	if account.Password != "" {
		encrypted, err := r.encryptor.Encrypt(account.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		account.Password = encrypted
	}
	
	if account.Phone != "" {
		encrypted, err := r.encryptor.Encrypt(account.Phone)
		if err != nil {
			return fmt.Errorf("failed to encrypt phone: %w", err)
		}
		account.Phone = encrypted
	}
	
	if account.Cookies != "" {
		encrypted, err := r.encryptor.Encrypt(account.Cookies)
		if err != nil {
			return fmt.Errorf("failed to encrypt cookies: %w", err)
		}
		account.Cookies = encrypted
	}
	
	_, err := r.collection.InsertOne(ctx, account)
	return err
}

// GetByID retrieves an account by ID
func (r *AccountRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.MaxAccount, error) {
	var account models.MaxAccount
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&account)
	if err != nil {
		return nil, err
	}
	
	// Decrypt sensitive fields
	if account.VKAccessToken != "" {
		decrypted, err := r.encryptor.Decrypt(account.VKAccessToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt vk access token: %w", err)
		}
		account.VKAccessToken = decrypted
	}

	if account.MaxSessionToken != "" {
		decrypted, err := r.encryptor.Decrypt(account.MaxSessionToken)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt max session token: %w", err)
		}
		account.MaxSessionToken = decrypted
	}
	
	if account.Password != "" {
		decrypted, err := r.encryptor.Decrypt(account.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt password: %w", err)
		}
		account.Password = decrypted
	}
	
	if account.Phone != "" {
		decrypted, err := r.encryptor.Decrypt(account.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt phone: %w", err)
		}
		account.Phone = decrypted
	}
	
	if account.Cookies != "" {
		decrypted, err := r.encryptor.Decrypt(account.Cookies)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt cookies: %w", err)
		}
		account.Cookies = decrypted
	}
	
	return &account, nil
}

// List lists accounts with filters and pagination
func (r *AccountRepository) List(ctx context.Context, filter map[string]interface{}, limit, offset int) ([]*models.MaxAccount, int64, error) {
	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	
	// Find with pagination
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.M{"created_at": -1})
	
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	
	var accounts []*models.MaxAccount
	for cursor.Next(ctx) {
		var account models.MaxAccount
		if err := cursor.Decode(&account); err != nil {
			continue
		}
		
		// Decrypt sensitive fields
		if account.VKAccessToken != "" {
			if decrypted, err := r.encryptor.Decrypt(account.VKAccessToken); err == nil {
				account.VKAccessToken = decrypted
			}
		}
		
		if account.Phone != "" {
			if decrypted, err := r.encryptor.Decrypt(account.Phone); err == nil {
				account.Phone = decrypted
			}
		}
		
		accounts = append(accounts, &account)
	}
	
	return accounts, total, nil
}

// UpdateAccountStatus updates account status
func (r *AccountRepository) UpdateAccountStatus(ctx context.Context, id primitive.ObjectID, status models.AccountStatus, errorMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}
	
	if errorMsg != "" {
		update["$set"].(bson.M)["error_message"] = errorMsg
	}
	
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// UpdateAccountFullCredentials updates account credentials
func (r *AccountRepository) UpdateAccountFullCredentials(ctx context.Context, id primitive.ObjectID, phone, password, cookies, vkUserID, vkAccessToken, maxSessionToken string, status models.AccountStatus) error {
	update := bson.M{
		"status":     status,
		"updated_at": time.Now(),
	}

	if phone != "" {
		encrypted, err := r.encryptor.Encrypt(phone)
		if err != nil {
			return fmt.Errorf("failed to encrypt phone: %w", err)
		}
		update["phone"] = encrypted
	}

	if password != "" {
		encrypted, err := r.encryptor.Encrypt(password)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		update["password"] = encrypted
	}

	if cookies != "" {
		encrypted, err := r.encryptor.Encrypt(cookies)
		if err != nil {
			return fmt.Errorf("failed to encrypt cookies: %w", err)
		}
		update["cookies"] = encrypted
	}

	if vkUserID != "" {
		update["vk_user_id"] = vkUserID
	}

	if vkAccessToken != "" {
		encrypted, err := r.encryptor.Encrypt(vkAccessToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt vk access token: %w", err)
		}
		update["vk_access_token"] = encrypted
	}

	if maxSessionToken != "" {
		encrypted, err := r.encryptor.Encrypt(maxSessionToken)
		if err != nil {
			return fmt.Errorf("failed to encrypt max session token: %w", err)
		}
		update["max_session_token"] = encrypted
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

// UpdateVKLink updates VK account link status
func (r *AccountRepository) UpdateVKLink(ctx context.Context, id primitive.ObjectID, vkAccountID string, isLinked bool) error {
	update := bson.M{
		"is_vk_linked": isLinked,
		"updated_at":   time.Now(),
	}

	if vkAccountID != "" {
		update["vk_account_id"] = vkAccountID
	}

	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

// IncrementRetryCount increments retry count
func (r *AccountRepository) IncrementRetryCount(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{
			"$inc": bson.M{"retry_count": 1},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)
	return err
}

// Delete soft deletes an account
func (r *AccountRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
				"updated_at": time.Now(),
			},
		},
	)
	return err
}

// GetStatistics returns account statistics
func (r *AccountRepository) GetStatistics(ctx context.Context) (*models.AccountStatistics, error) {
	stats := &models.AccountStatistics{
		AccountsByStatus: make(map[string]int64),
	}
	
	// Total accounts
	total, err := r.collection.CountDocuments(ctx, bson.M{"deleted_at": nil})
	if err != nil {
		return nil, err
	}
	stats.TotalAccounts = total
	
	// Accounts by status
	statuses := []models.AccountStatus{
		models.AccountStatusCreating,
		models.AccountStatusCreated,
		models.AccountStatusWarming,
		models.AccountStatusReady,
		models.AccountStatusBanned,
		models.AccountStatusError,
		models.AccountStatusSuspended,
	}
	
	for _, status := range statuses {
		count, err := r.collection.CountDocuments(ctx, bson.M{
			"status":     status,
			"deleted_at": nil,
		})
		if err != nil {
			continue
		}
		stats.AccountsByStatus[string(status)] = count
	}
	
	// Success rate
	success := stats.AccountsByStatus[string(models.AccountStatusCreated)] +
		stats.AccountsByStatus[string(models.AccountStatusWarming)] +
		stats.AccountsByStatus[string(models.AccountStatusReady)]
	
	if stats.TotalAccounts > 0 {
		stats.SuccessRate = float64(success) / float64(stats.TotalAccounts)
	}
	
	// Average retries
	pipeline := []bson.M{
		{"$match": bson.M{"deleted_at": nil}},
		{"$group": bson.M{
			"_id": nil,
			"avg_retries": bson.M{"$avg": "$retry_count"},
		}},
	}
	
	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err == nil && cursor.Next(ctx) {
		var result struct {
			AvgRetries float64 `bson:"avg_retries"`
		}
		cursor.Decode(&result)
		stats.AverageRetries = result.AvgRetries
	}
	
	// Last hour
	stats.LastHour, _ = r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": time.Now().Add(-time.Hour)},
		"deleted_at": nil,
	})
	
	// Last 24 hours
	stats.Last24Hours, _ = r.collection.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": time.Now().Add(-24 * time.Hour)},
		"deleted_at": nil,
	})
	
	return stats, nil
}

// CreateIndexes creates database indexes
func (r *AccountRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.M{"status": 1},
		},
		{
			Keys: bson.M{"created_at": -1},
		},
		{
			Keys: bson.M{"proxy_id": 1},
		},
		{
			Keys: bson.M{"activation_id": 1},
		},
		{
			Keys: bson.M{"deleted_at": 1},
		},
	}
	
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}
