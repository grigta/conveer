package repository

import (
	"context"
	"errors"
	"time"

	"conveer/pkg/crypto"
	"conveer/pkg/database"
	"conveer/services/proxy-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/sirupsen/logrus"
)

type ProxyRepository struct {
	db        *database.MongoDB
	encryptor *crypto.Encryptor
	logger    *logrus.Logger
}

func NewProxyRepository(db *database.MongoDB, encryptor *crypto.Encryptor, logger *logrus.Logger) *ProxyRepository {
	return &ProxyRepository{
		db:        db,
		encryptor: encryptor,
		logger:    logger,
	}
}

func (r *ProxyRepository) CreateProxy(ctx context.Context, proxy *models.Proxy) error {
	if proxy.Password != "" {
		encryptedPassword, err := r.encryptor.Encrypt(proxy.Password)
		if err != nil {
			r.logger.WithError(err).Error("Failed to encrypt proxy password")
			return err
		}
		proxy.Password = encryptedPassword
	}

	proxy.CreatedAt = time.Now()
	proxy.LastChecked = time.Now()

	result, err := r.db.Collection("proxies").InsertOne(ctx, proxy)
	if err != nil {
		r.logger.WithError(err).Error("Failed to insert proxy")
		return err
	}

	proxy.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *ProxyRepository) GetProxyByID(ctx context.Context, id primitive.ObjectID) (*models.Proxy, error) {
	var proxy models.Proxy
	err := r.db.Collection("proxies").FindOne(ctx, bson.M{"_id": id}).Decode(&proxy)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("proxy not found")
		}
		r.logger.WithError(err).Error("Failed to get proxy by ID")
		return nil, err
	}

	if proxy.Password != "" {
		decryptedPassword, err := r.encryptor.Decrypt(proxy.Password)
		if err != nil {
			r.logger.WithError(err).Error("Failed to decrypt proxy password")
			return nil, err
		}
		proxy.Password = decryptedPassword
	}

	return &proxy, nil
}

func (r *ProxyRepository) GetAvailableProxies(ctx context.Context, filters models.ProxyFilters) ([]models.Proxy, error) {
	// First, get list of occupied proxy IDs
	occupiedIDs, err := r.getOccupiedProxyIDs(ctx)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get occupied proxy IDs")
		return nil, err
	}

	filter := bson.M{"status": models.ProxyStatusActive}

	// Exclude occupied proxies
	if len(occupiedIDs) > 0 {
		filter["_id"] = bson.M{"$nin": occupiedIDs}
	}

	if filters.Type != "" {
		filter["type"] = filters.Type
	}
	if filters.Country != "" {
		filter["country"] = filters.Country
	}
	if filters.Provider != "" {
		filter["provider"] = filters.Provider
	}

	cursor, err := r.db.Collection("proxies").Find(ctx, filter)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get available proxies")
		return nil, err
	}
	defer cursor.Close(ctx)

	var proxies []models.Proxy
	for cursor.Next(ctx) {
		var proxy models.Proxy
		if err := cursor.Decode(&proxy); err != nil {
			r.logger.WithError(err).Error("Failed to decode proxy")
			continue
		}

		if proxy.Password != "" {
			decryptedPassword, err := r.encryptor.Decrypt(proxy.Password)
			if err != nil {
				r.logger.WithError(err).Error("Failed to decrypt proxy password")
				continue
			}
			proxy.Password = decryptedPassword
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

func (r *ProxyRepository) UpdateProxyStatus(ctx context.Context, id primitive.ObjectID, status models.ProxyStatus) error {
	update := bson.M{
		"$set": bson.M{
			"status": status,
			"last_checked": time.Now(),
		},
	}

	result, err := r.db.Collection("proxies").UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update proxy status")
		return err
	}

	if result.ModifiedCount == 0 {
		return errors.New("proxy not found")
	}

	return nil
}

func (r *ProxyRepository) UpdateProxyHealth(ctx context.Context, id primitive.ObjectID, health *models.ProxyHealth) error {
	health.ProxyID = id
	health.LastCheck = time.Now()

	opts := options.Replace().SetUpsert(true)
	filter := bson.M{"proxy_id": id}

	_, err := r.db.Collection("proxy_health").ReplaceOne(ctx, filter, health, opts)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update proxy health")
		return err
	}

	return nil
}

func (r *ProxyRepository) GetProxiesByStatus(ctx context.Context, status models.ProxyStatus) ([]models.Proxy, error) {
	cursor, err := r.db.Collection("proxies").Find(ctx, bson.M{"status": status})
	if err != nil {
		r.logger.WithError(err).Error("Failed to get proxies by status")
		return nil, err
	}
	defer cursor.Close(ctx)

	var proxies []models.Proxy
	for cursor.Next(ctx) {
		var proxy models.Proxy
		if err := cursor.Decode(&proxy); err != nil {
			r.logger.WithError(err).Error("Failed to decode proxy")
			continue
		}

		if proxy.Password != "" {
			decryptedPassword, err := r.encryptor.Decrypt(proxy.Password)
			if err != nil {
				r.logger.WithError(err).Error("Failed to decrypt proxy password")
				continue
			}
			proxy.Password = decryptedPassword
		}

		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

func (r *ProxyRepository) GetExpiredProxies(ctx context.Context) ([]models.Proxy, error) {
	filter := bson.M{
		"expires_at": bson.M{"$lte": time.Now()},
		"status": bson.M{"$ne": models.ProxyStatusReleased},
	}

	cursor, err := r.db.Collection("proxies").Find(ctx, filter)
	if err != nil {
		r.logger.WithError(err).Error("Failed to get expired proxies")
		return nil, err
	}
	defer cursor.Close(ctx)

	var proxies []models.Proxy
	for cursor.Next(ctx) {
		var proxy models.Proxy
		if err := cursor.Decode(&proxy); err != nil {
			r.logger.WithError(err).Error("Failed to decode proxy")
			continue
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
}

func (r *ProxyRepository) BindProxyToAccount(ctx context.Context, proxyID primitive.ObjectID, accountID string) error {
	session, err := r.db.Client.StartSession()
	if err != nil {
		r.logger.WithError(err).Error("Failed to start session")
		return err
	}
	defer session.EndSession(ctx)

	err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		if err := session.StartTransaction(); err != nil {
			return err
		}

		existingBinding := bson.M{
			"account_id": accountID,
			"status": bson.M{"$ne": models.BindingStatusReleased},
		}

		update := bson.M{
			"$set": bson.M{
				"status": models.BindingStatusReleased,
			},
		}

		_, err := r.db.Collection("proxy_bindings").UpdateMany(sc, existingBinding, update)
		if err != nil {
			return err
		}

		binding := models.ProxyBinding{
			ProxyID:    proxyID,
			AccountID:  accountID,
			BoundAt:    time.Now(),
			LastUsedAt: time.Now(),
			Status:     models.BindingStatusActive,
		}

		_, err = r.db.Collection("proxy_bindings").InsertOne(sc, binding)
		if err != nil {
			return err
		}

		proxyUpdate := bson.M{
			"$set": bson.M{
				"status": models.ProxyStatusActive,
			},
		}

		_, err = r.db.Collection("proxies").UpdateOne(sc, bson.M{"_id": proxyID}, proxyUpdate)
		if err != nil {
			return err
		}

		return session.CommitTransaction(sc)
	})

	if err != nil {
		r.logger.WithError(err).Error("Failed to bind proxy to account")
		return err
	}

	return nil
}

func (r *ProxyRepository) ReleaseProxyBinding(ctx context.Context, proxyID primitive.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"status": models.BindingStatusReleased,
		},
	}

	_, err := r.db.Collection("proxy_bindings").UpdateOne(ctx, bson.M{"proxy_id": proxyID, "status": models.BindingStatusActive}, update)
	if err != nil {
		r.logger.WithError(err).Error("Failed to release proxy binding")
		return err
	}

	proxyUpdate := bson.M{
		"$set": bson.M{
			"status": models.ProxyStatusReleased,
		},
	}

	_, err = r.db.Collection("proxies").UpdateOne(ctx, bson.M{"_id": proxyID}, proxyUpdate)
	if err != nil {
		r.logger.WithError(err).Error("Failed to update proxy status to released")
		return err
	}

	return nil
}

func (r *ProxyRepository) GetProxyByAccountID(ctx context.Context, accountID string) (*models.Proxy, error) {
	var binding models.ProxyBinding
	err := r.db.Collection("proxy_bindings").FindOne(ctx, bson.M{
		"account_id": accountID,
		"status": models.BindingStatusActive,
	}).Decode(&binding)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		r.logger.WithError(err).Error("Failed to get proxy binding")
		return nil, err
	}

	return r.GetProxyByID(ctx, binding.ProxyID)
}

func (r *ProxyRepository) CreateIndexes(ctx context.Context) error {
	proxiesIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "ip", Value: 1}, {Key: "port", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "type", Value: 1}, {Key: "country", Value: 1}},
		},
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(86400), // 24 hours after expiration
		},
		{
			Keys: bson.D{{Key: "provider", Value: 1}},
		},
	}

	_, err := r.db.Collection("proxies").Indexes().CreateMany(ctx, proxiesIndexes)
	if err != nil {
		r.logger.WithError(err).Error("Failed to create proxies indexes")
		return err
	}

	healthIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "proxy_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "last_check", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "fraud_score", Value: 1}},
		},
	}

	_, err = r.db.Collection("proxy_health").Indexes().CreateMany(ctx, healthIndexes)
	if err != nil {
		r.logger.WithError(err).Error("Failed to create proxy_health indexes")
		return err
	}

	bindingIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "account_id", Value: 1}},
			Options: options.Index().SetUnique(false),
		},
		{
			Keys: bson.D{{Key: "proxy_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			// Unique compound index to ensure only one active binding per proxy
			Keys: bson.D{{Key: "proxy_id", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"status": models.BindingStatusActive}),
		},
	}

	_, err = r.db.Collection("proxy_bindings").Indexes().CreateMany(ctx, bindingIndexes)
	if err != nil {
		r.logger.WithError(err).Error("Failed to create proxy_bindings indexes")
		return err
	}

	return nil
}

func (r *ProxyRepository) GetProxyStatistics(ctx context.Context) (*models.ProxyStats, error) {
	stats := &models.ProxyStats{
		ProxiesByType:    make(map[string]int64),
		ProxiesByCountry: make(map[string]int64),
	}

	total, err := r.db.Collection("proxies").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats.TotalProxies = total

	active, err := r.db.Collection("proxies").CountDocuments(ctx, bson.M{"status": models.ProxyStatusActive})
	if err != nil {
		return nil, err
	}
	stats.ActiveProxies = active

	expired, err := r.db.Collection("proxies").CountDocuments(ctx, bson.M{"status": models.ProxyStatusExpired})
	if err != nil {
		return nil, err
	}
	stats.ExpiredProxies = expired

	banned, err := r.db.Collection("proxies").CountDocuments(ctx, bson.M{"status": models.ProxyStatusBanned})
	if err != nil {
		return nil, err
	}
	stats.BannedProxies = banned

	bindings, err := r.db.Collection("proxy_bindings").CountDocuments(ctx, bson.M{"status": models.BindingStatusActive})
	if err != nil {
		return nil, err
	}
	stats.TotalBindings = bindings

	pipeline := []bson.M{
		{"$group": bson.M{
			"_id": "$type",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := r.db.Collection("proxies").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&result); err == nil {
			stats.ProxiesByType[result.ID] = result.Count
		}
	}

	pipeline = []bson.M{
		{"$group": bson.M{
			"_id": "$country",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor2, err := r.db.Collection("proxies").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor2.Close(ctx)

	for cursor2.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor2.Decode(&result); err == nil {
			stats.ProxiesByCountry[result.ID] = result.Count
		}
	}

	healthPipeline := []bson.M{
		{"$group": bson.M{
			"_id": nil,
			"avg_fraud_score": bson.M{"$avg": "$fraud_score"},
			"avg_latency": bson.M{"$avg": "$latency"},
		}},
	}

	cursor3, err := r.db.Collection("proxy_health").Aggregate(ctx, healthPipeline)
	if err == nil && cursor3.Next(ctx) {
		var result struct {
			AvgFraudScore float64 `bson:"avg_fraud_score"`
			AvgLatency    float64 `bson:"avg_latency"`
		}
		if err := cursor3.Decode(&result); err == nil {
			stats.AvgFraudScore = result.AvgFraudScore
			stats.AvgLatency = result.AvgLatency
		}
		cursor3.Close(ctx)
	}

	return stats, nil
}

func (r *ProxyRepository) getOccupiedProxyIDs(ctx context.Context) ([]primitive.ObjectID, error) {
	filter := bson.M{"status": models.BindingStatusActive}

	cursor, err := r.db.Collection("proxy_bindings").Find(ctx, filter, options.Find().SetProjection(bson.M{"proxy_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var occupiedIDs []primitive.ObjectID
	for cursor.Next(ctx) {
		var binding struct {
			ProxyID primitive.ObjectID `bson:"proxy_id"`
		}
		if err := cursor.Decode(&binding); err == nil {
			occupiedIDs = append(occupiedIDs, binding.ProxyID)
		}
	}

	return occupiedIDs, nil
}

func (r *ProxyRepository) GetActiveBindingByProxyID(ctx context.Context, proxyID primitive.ObjectID) (*models.ProxyBinding, error) {
	var binding models.ProxyBinding
	err := r.db.Collection("proxy_bindings").FindOne(ctx, bson.M{
		"proxy_id": proxyID,
		"status": models.BindingStatusActive,
	}).Decode(&binding)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		r.logger.WithError(err).Error("Failed to get active binding by proxy ID")
		return nil, err
	}

	return &binding, nil
}

func (r *ProxyRepository) GetProxyHealthByID(ctx context.Context, proxyID primitive.ObjectID) (*models.ProxyHealth, error) {
	var health models.ProxyHealth
	err := r.db.Collection("proxy_health").FindOne(ctx, bson.M{
		"proxy_id": proxyID,
	}).Decode(&health)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		r.logger.WithError(err).Error("Failed to get proxy health by ID")
		return nil, err
	}

	return &health, nil
}
