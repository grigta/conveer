package repository

import (
	"context"
	"time"

	"github.com/grigta/conveer/services/analytics-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RecommendationRepository репозиторий для работы с рекомендациями
type RecommendationRepository struct {
	collection *mongo.Collection
}

// NewRecommendationRepository создает новый репозиторий рекомендаций
func NewRecommendationRepository(db *mongo.Database) *RecommendationRepository {
	return &RecommendationRepository{
		collection: db.Collection("recommendations"),
	}
}

// Save сохраняет рекомендацию
func (r *RecommendationRepository) Save(ctx context.Context, recommendation *models.Recommendation) error {
	recommendation.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, recommendation)
	return err
}

// GetLatestByType получает последнюю рекомендацию по типу
func (r *RecommendationRepository) GetLatestByType(ctx context.Context, recType string) (*models.Recommendation, error) {
	filter := bson.M{
		"type": recType,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var recommendation models.Recommendation
	err := r.collection.FindOne(ctx, filter, opts).Decode(&recommendation)
	if err != nil {
		return nil, err
	}

	return &recommendation, nil
}

// GetByPriority получает рекомендации по приоритету
func (r *RecommendationRepository) GetByPriority(ctx context.Context, priority string) ([]models.Recommendation, error) {
	filter := bson.M{
		"priority": priority,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var recommendations []models.Recommendation
	if err := cursor.All(ctx, &recommendations); err != nil {
		return nil, err
	}

	return recommendations, nil
}

// GetActiveRecommendations получает все активные рекомендации
func (r *RecommendationRepository) GetActiveRecommendations(ctx context.Context, limit int) ([]models.Recommendation, error) {
	filter := bson.M{
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "priority", Value: 1}, // Сортировка: high -> medium -> low
			{Key: "generated_at", Value: -1},
		}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var recommendations []models.Recommendation
	if err := cursor.All(ctx, &recommendations); err != nil {
		return nil, err
	}

	return recommendations, nil
}

// GetProxyRatings получает рейтинг прокси-провайдеров
func (r *RecommendationRepository) GetProxyRatings(ctx context.Context) (*models.ProxyProviderRating, error) {
	recommendation, err := r.GetLatestByType(ctx, "proxy_provider")
	if err != nil {
		return nil, err
	}

	if recommendation.ProxyRating == nil {
		return &models.ProxyProviderRating{}, nil
	}

	return recommendation.ProxyRating, nil
}

// GetWarmingScenarioRecommendation получает рекомендацию по сценарию прогрева
func (r *RecommendationRepository) GetWarmingScenarioRecommendation(ctx context.Context, platform string) (*models.WarmingScenarioRecommendation, error) {
	filter := bson.M{
		"type": "warming_scenario",
		"warming_scenario.platform": platform,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var recommendation models.Recommendation
	err := r.collection.FindOne(ctx, filter, opts).Decode(&recommendation)
	if err != nil {
		return nil, err
	}

	if recommendation.WarmingScenario == nil {
		return &models.WarmingScenarioRecommendation{}, nil
	}

	return recommendation.WarmingScenario, nil
}

// GetErrorPatterns получает анализ паттернов ошибок
func (r *RecommendationRepository) GetErrorPatterns(ctx context.Context) (*models.ErrorPatternAnalysis, error) {
	recommendation, err := r.GetLatestByType(ctx, "error_pattern")
	if err != nil {
		return nil, err
	}

	if recommendation.ErrorPattern == nil {
		return &models.ErrorPatternAnalysis{}, nil
	}

	return recommendation.ErrorPattern, nil
}

// UpdateRecommendation обновляет рекомендацию
func (r *RecommendationRepository) UpdateRecommendation(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	return err
}

// DeleteExpiredRecommendations удаляет устаревшие рекомендации
func (r *RecommendationRepository) DeleteExpiredRecommendations(ctx context.Context) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"valid_until": bson.M{"$lt": time.Now()},
	})
	return err
}

// GetRecommendationHistory получает историю рекомендаций
func (r *RecommendationRepository) GetRecommendationHistory(ctx context.Context, recType string, limit int) ([]models.Recommendation, error) {
	filter := bson.M{
		"type": recType,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "generated_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var recommendations []models.Recommendation
	if err := cursor.All(ctx, &recommendations); err != nil {
		return nil, err
	}

	return recommendations, nil
}
