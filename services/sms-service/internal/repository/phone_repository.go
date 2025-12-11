package repository

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/models"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PhoneRepository struct {
	collection *mongo.Collection
	logger     *logrus.Logger
	encKey     []byte
}

func NewPhoneRepository(db *mongo.Database, logger *logrus.Logger) *PhoneRepository {
	encKeyStr := os.Getenv("ENCRYPTION_KEY")
	if encKeyStr == "" {
		encKeyStr = "default-32-byte-encryption-key!!" // 32 bytes for AES-256
	}

	return &PhoneRepository{
		collection: db.Collection("phones"),
		logger:     logger,
		encKey:     []byte(encKeyStr)[:32],
	}
}

func (r *PhoneRepository) Create(ctx context.Context, phone *models.Phone) error {
	phone.CreatedAt = time.Now()
	phone.UpdatedAt = time.Now()

	// Encrypt sensitive data
	encryptedPhone := *phone
	if err := r.encryptPhone(&encryptedPhone); err != nil {
		return fmt.Errorf("failed to encrypt phone: %w", err)
	}

	result, err := r.collection.InsertOne(ctx, encryptedPhone)
	if err != nil {
		return fmt.Errorf("failed to insert phone: %w", err)
	}

	phone.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *PhoneRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Phone, error) {
	var phone models.Phone
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&phone)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find phone: %w", err)
	}

	if phone.Encrypted {
		if err := r.decryptPhone(&phone); err != nil {
			return nil, fmt.Errorf("failed to decrypt phone: %w", err)
		}
	}

	return &phone, nil
}

func (r *PhoneRepository) FindByNumber(ctx context.Context, number string) (*models.Phone, error) {
	encryptedNumber, err := r.encrypt(number)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt number for search: %w", err)
	}

	var phone models.Phone
	err = r.collection.FindOne(ctx, bson.M{"number": encryptedNumber}).Decode(&phone)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find phone: %w", err)
	}

	if phone.Encrypted {
		if err := r.decryptPhone(&phone); err != nil {
			return nil, fmt.Errorf("failed to decrypt phone: %w", err)
		}
	}

	return &phone, nil
}

func (r *PhoneRepository) FindByUserAndService(ctx context.Context, userID, service string, status models.PhoneStatus) ([]*models.Phone, error) {
	filter := bson.M{
		"user_id": userID,
		"service": service,
		"status":  status,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find phones: %w", err)
	}
	defer cursor.Close(ctx)

	var phones []*models.Phone
	for cursor.Next(ctx) {
		var phone models.Phone
		if err := cursor.Decode(&phone); err != nil {
			return nil, fmt.Errorf("failed to decode phone: %w", err)
		}

		if phone.Encrypted {
			if err := r.decryptPhone(&phone); err != nil {
				r.logger.Errorf("Failed to decrypt phone %s: %v", phone.ID.Hex(), err)
				continue
			}
		}

		phones = append(phones, &phone)
	}

	return phones, nil
}

func (r *PhoneRepository) Update(ctx context.Context, phone *models.Phone) error {
	phone.UpdatedAt = time.Now()

	encryptedPhone := *phone
	if err := r.encryptPhone(&encryptedPhone); err != nil {
		return fmt.Errorf("failed to encrypt phone: %w", err)
	}

	filter := bson.M{"_id": phone.ID}
	update := bson.M{"$set": encryptedPhone}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update phone: %w", err)
	}

	return nil
}

func (r *PhoneRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status models.PhoneStatus) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update phone status: %w", err)
	}

	return nil
}

func (r *PhoneRepository) FindExpired(ctx context.Context) ([]*models.Phone, error) {
	filter := bson.M{
		"status": bson.M{"$in": []models.PhoneStatus{
			models.PhoneStatusActive,
			models.PhoneStatusAvailable,
		}},
		"expires_at": bson.M{"$lt": time.Now()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find expired phones: %w", err)
	}
	defer cursor.Close(ctx)

	var phones []*models.Phone
	for cursor.Next(ctx) {
		var phone models.Phone
		if err := cursor.Decode(&phone); err != nil {
			return nil, fmt.Errorf("failed to decode phone: %w", err)
		}

		if phone.Encrypted {
			if err := r.decryptPhone(&phone); err != nil {
				r.logger.Errorf("Failed to decrypt phone %s: %v", phone.ID.Hex(), err)
				continue
			}
		}

		phones = append(phones, &phone)
	}

	return phones, nil
}

func (r *PhoneRepository) CountByStatus(ctx context.Context, status models.PhoneStatus) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"status": status})
	if err != nil {
		return 0, fmt.Errorf("failed to count phones: %w", err)
	}

	return count, nil
}

func (r *PhoneRepository) GetStatistics(ctx context.Context, filter bson.M) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": nil,
			"total": bson.M{"$sum": 1},
			"totalPrice": bson.M{"$sum": "$price"},
			"avgPrice": bson.M{"$avg": "$price"},
			"byStatus": bson.M{
				"$push": bson.M{
					"status": "$status",
					"count":  1,
				},
			},
			"byService": bson.M{
				"$push": bson.M{
					"service": "$service",
					"count":   1,
				},
			},
		}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}
	defer cursor.Close(ctx)

	var result map[string]interface{}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode statistics: %w", err)
		}
	}

	return result, nil
}

func (r *PhoneRepository) CreateIndex(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "number", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "expires_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "activation_id", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func (r *PhoneRepository) encryptPhone(phone *models.Phone) error {
	var err error
	phone.Number, err = r.encrypt(phone.Number)
	if err != nil {
		return fmt.Errorf("failed to encrypt number: %w", err)
	}

	phone.Encrypted = true
	return nil
}

func (r *PhoneRepository) decryptPhone(phone *models.Phone) error {
	var err error
	phone.Number, err = r.decrypt(phone.Number)
	if err != nil {
		return fmt.Errorf("failed to decrypt number: %w", err)
	}

	phone.Encrypted = false
	return nil
}

func (r *PhoneRepository) encrypt(text string) (string, error) {
	block, err := aes.NewCipher(r.encKey)
	if err != nil {
		return "", err
	}

	plaintext := []byte(text)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (r *PhoneRepository) decrypt(cryptoText string) (string, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(r.encKey)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}
