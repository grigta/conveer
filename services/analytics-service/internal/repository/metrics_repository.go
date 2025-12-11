package repository

import (
	"context"
	"time"

	"github.com/conveer/conveer/services/analytics-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MetricsRepository репозиторий для работы с метриками
type MetricsRepository struct {
	collection *mongo.Collection
}

// NewMetricsRepository создает новый репозиторий метрик
func NewMetricsRepository(db *mongo.Database) *MetricsRepository {
	return &MetricsRepository{
		collection: db.Collection("aggregated_metrics"),
	}
}

// Save сохраняет агрегированные метрики
func (r *MetricsRepository) Save(ctx context.Context, metrics *models.AggregatedMetrics) error {
	metrics.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, metrics)
	return err
}

// GetLatest получает последние метрики для платформы
func (r *MetricsRepository) GetLatest(ctx context.Context, platform string) (*models.AggregatedMetrics, error) {
	filter := bson.M{}
	if platform != "" && platform != "all" {
		filter["platform"] = platform
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	var metrics models.AggregatedMetrics
	err := r.collection.FindOne(ctx, filter, opts).Decode(&metrics)
	if err != nil {
		return nil, err
	}

	return &metrics, nil
}

// GetByTimeRange получает метрики за период
func (r *MetricsRepository) GetByTimeRange(ctx context.Context, platform string, start, end time.Time) ([]models.AggregatedMetrics, error) {
	filter := bson.M{
		"timestamp": bson.M{
			"$gte": start,
			"$lte": end,
		},
	}

	if platform != "" && platform != "all" {
		filter["platform"] = platform
	}

	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var metrics []models.AggregatedMetrics
	if err := cursor.All(ctx, &metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

// GetTimeSeriesData получает данные временного ряда для метрики
func (r *MetricsRepository) GetTimeSeriesData(ctx context.Context, metricName string, duration time.Duration) ([]models.TimeSeriesData, error) {
	startTime := time.Now().Add(-duration)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"timestamp": bson.M{"$gte": startTime},
		}}},
		{{Key: "$project", Value: bson.M{
			"timestamp": 1,
			"platform": 1,
			"value": "$" + metricName,
		}}},
		{{Key: "$sort", Value: bson.M{"timestamp": 1}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var data []models.TimeSeriesData
	if err := cursor.All(ctx, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// GetAggregatedStats получает агрегированную статистику
func (r *MetricsRepository) GetAggregatedStats(ctx context.Context, platform string, period time.Duration) (map[string]interface{}, error) {
	startTime := time.Now().Add(-period)

	matchStage := bson.M{
		"timestamp": bson.M{"$gte": startTime},
	}

	if platform != "" && platform != "all" {
		matchStage["platform"] = platform
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchStage}},
		{{Key: "$group", Value: bson.M{
			"_id": nil,
			"avg_ban_rate": bson.M{"$avg": "$ban_rate"},
			"avg_success_rate": bson.M{"$avg": "$success_rate"},
			"total_spent": bson.M{"$sum": "$total_spent"},
			"avg_warming_days": bson.M{"$avg": "$avg_warming_days"},
			"total_errors": bson.M{"$sum": "$error_count"},
			"avg_error_rate": bson.M{"$avg": "$error_rate"},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) > 0 {
		return results[0], nil
	}

	return make(map[string]interface{}), nil
}

// GetTrends получает тренды метрик
func (r *MetricsRepository) GetTrends(ctx context.Context, platform string, days int) ([]models.AggregatedMetrics, error) {
	startTime := time.Now().AddDate(0, 0, -days)

	return r.GetByTimeRange(ctx, platform, startTime, time.Now())
}

// DeleteOldMetrics удаляет старые метрики
func (r *MetricsRepository) DeleteOldMetrics(ctx context.Context, olderThan time.Time) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"timestamp": bson.M{"$lt": olderThan},
	})
	return err
}