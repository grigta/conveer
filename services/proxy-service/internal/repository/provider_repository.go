package repository

import (
	"context"
	"errors"
	"time"

	"conveer/pkg/database"
	"conveer/services/proxy-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/sirupsen/logrus"
)

type ProviderRepository struct {
	db     *database.MongoDB
	logger *logrus.Logger
}

func NewProviderRepository(db *database.MongoDB, logger *logrus.Logger) *ProviderRepository {
	return &ProviderRepository{
		db:     db,
		logger: logger,
	}
}

func (r *ProviderRepository) GetProviderConfig(ctx context.Context, name string) (*models.ProxyProvider, error) {
	var provider models.ProxyProvider
	err := r.db.Collection("proxy_providers").FindOne(ctx, bson.M{"name": name}).Decode(&provider)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("provider not found")
		}
		r.logger.WithError(err).Error("Failed to get provider config")
		return nil, err
	}

	return &provider, nil
}

func (r *ProviderRepository) ListActiveProviders(ctx context.Context) ([]models.ProxyProvider, error) {
	filter := bson.M{"enabled": true}
	opts := options.Find().SetSort(bson.D{{Key: "priority", Value: 1}})

	cursor, err := r.db.Collection("proxy_providers").Find(ctx, filter, opts)
	if err != nil {
		r.logger.WithError(err).Error("Failed to list active providers")
		return nil, err
	}
	defer cursor.Close(ctx)

	var providers []models.ProxyProvider
	for cursor.Next(ctx) {
		var provider models.ProxyProvider
		if err := cursor.Decode(&provider); err != nil {
			r.logger.WithError(err).Error("Failed to decode provider")
			continue
		}
		providers = append(providers, provider)
	}

	return providers, nil
}

func (r *ProviderRepository) UpdateProviderStats(ctx context.Context, name string, stats *models.ProviderStats) error {
	stats.ProviderName = name
	stats.LastRequestTime = time.Now()

	filter := bson.M{"provider_name": name}
	opts := options.Replace().SetUpsert(true)

	_, err := r.db.Collection("provider_stats").ReplaceOne(ctx, filter, stats, opts)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update provider stats")
		return err
	}

	return nil
}

func (r *ProviderRepository) GetProviderStats(ctx context.Context, name string) (*models.ProviderStats, error) {
	var stats models.ProviderStats
	err := r.db.Collection("provider_stats").FindOne(ctx, bson.M{"provider_name": name}).Decode(&stats)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &models.ProviderStats{
				ProviderName: name,
			}, nil
		}
		r.logger.WithError(err).Error("Failed to get provider stats")
		return nil, err
	}

	return &stats, nil
}

func (r *ProviderRepository) IncrementProviderCounter(ctx context.Context, name string, counterType string) error {
	update := bson.M{
		"$inc": bson.M{
			counterType: 1,
		},
		"$set": bson.M{
			"provider_name": name,
			"last_request_time": time.Now(),
		},
	}

	if counterType == "total_allocated" || counterType == "total_rotated" {
		update["$set"].(bson.M)["last_success_time"] = time.Now()
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.db.Collection("provider_stats").UpdateOne(
		ctx,
		bson.M{"provider_name": name},
		update,
		opts,
	)

	if err != nil {
		r.logger.WithError(err).Error("Failed to increment provider counter")
		return err
	}

	return nil
}

func (r *ProviderRepository) UpdateProviderActiveProxies(ctx context.Context, name string, count int64) error {
	update := bson.M{
		"$set": bson.M{
			"active_proxies": count,
			"provider_name": name,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.db.Collection("provider_stats").UpdateOne(
		ctx,
		bson.M{"provider_name": name},
		update,
		opts,
	)

	if err != nil {
		r.logger.WithError(err).Error("Failed to update provider active proxies")
		return err
	}

	return nil
}

func (r *ProviderRepository) GetAllProviderStats(ctx context.Context) ([]models.ProviderStats, error) {
	cursor, err := r.db.Collection("provider_stats").Find(ctx, bson.M{})
	if err != nil {
		r.logger.WithError(err).Error("Failed to get all provider stats")
		return nil, err
	}
	defer cursor.Close(ctx)

	var stats []models.ProviderStats
	for cursor.Next(ctx) {
		var stat models.ProviderStats
		if err := cursor.Decode(&stat); err != nil {
			r.logger.WithError(err).Error("Failed to decode provider stats")
			continue
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

func (r *ProviderRepository) SaveProviderConfig(ctx context.Context, provider *models.ProxyProvider) error {
	filter := bson.M{"name": provider.Name}
	opts := options.Replace().SetUpsert(true)

	_, err := r.db.Collection("proxy_providers").ReplaceOne(ctx, filter, provider, opts)
	if err != nil {
		r.logger.WithError(err).Error("Failed to save provider config")
		return err
	}

	return nil
}

func (r *ProviderRepository) CreateIndexes(ctx context.Context) error {
	providerIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "enabled", Value: 1}, {Key: "priority", Value: 1}},
		},
	}

	_, err := r.db.Collection("proxy_providers").Indexes().CreateMany(ctx, providerIndexes)
	if err != nil {
		r.logger.WithError(err).Error("Failed to create proxy_providers indexes")
		return err
	}

	statsIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "provider_name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err = r.db.Collection("provider_stats").Indexes().CreateMany(ctx, statsIndexes)
	if err != nil {
		r.logger.WithError(err).Error("Failed to create provider_stats indexes")
		return err
	}

	return nil
}