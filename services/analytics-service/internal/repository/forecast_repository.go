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

// ForecastRepository репозиторий для работы с прогнозами
type ForecastRepository struct {
	collection *mongo.Collection
}

// NewForecastRepository создает новый репозиторий прогнозов
func NewForecastRepository(db *mongo.Database) *ForecastRepository {
	return &ForecastRepository{
		collection: db.Collection("forecasts"),
	}
}

// Save сохраняет прогноз
func (r *ForecastRepository) Save(ctx context.Context, forecast *models.ForecastResult) error {
	forecast.ID = primitive.NewObjectID()
	_, err := r.collection.InsertOne(ctx, forecast)
	return err
}

// GetLatestByType получает последний прогноз по типу
func (r *ForecastRepository) GetLatestByType(ctx context.Context, forecastType, platform string) (*models.ForecastResult, error) {
	filter := bson.M{
		"type": forecastType,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	if platform != "" {
		filter["platform"] = platform
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var forecast models.ForecastResult
	err := r.collection.FindOne(ctx, filter, opts).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	return &forecast, nil
}

// GetExpenseForecast получает прогноз расходов
func (r *ForecastRepository) GetExpenseForecast(ctx context.Context, period string) (*models.ForecastResult, error) {
	filter := bson.M{
		"type": "expense",
		"expense_forecast.period": period,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var forecast models.ForecastResult
	err := r.collection.FindOne(ctx, filter, opts).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	return &forecast, nil
}

// GetReadinessForecast получает прогноз готовности аккаунта
func (r *ForecastRepository) GetReadinessForecast(ctx context.Context, accountID string) (*models.ForecastResult, error) {
	filter := bson.M{
		"type": "readiness",
		"readiness_forecast.account_id": accountID,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var forecast models.ForecastResult
	err := r.collection.FindOne(ctx, filter, opts).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	return &forecast, nil
}

// GetOptimalTimeForecast получает прогноз оптимального времени
func (r *ForecastRepository) GetOptimalTimeForecast(ctx context.Context, platform string) (*models.ForecastResult, error) {
	filter := bson.M{
		"type": "optimal_time",
		"platform": platform,
		"valid_until": bson.M{"$gte": time.Now()},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "generated_at", Value: -1}})

	var forecast models.ForecastResult
	err := r.collection.FindOne(ctx, filter, opts).Decode(&forecast)
	if err != nil {
		return nil, err
	}

	return &forecast, nil
}

// GetAllValidForecasts получает все актуальные прогнозы
func (r *ForecastRepository) GetAllValidForecasts(ctx context.Context) ([]models.ForecastResult, error) {
	filter := bson.M{
		"valid_until": bson.M{"$gte": time.Now()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var forecasts []models.ForecastResult
	if err := cursor.All(ctx, &forecasts); err != nil {
		return nil, err
	}

	return forecasts, nil
}

// UpdateForecast обновляет существующий прогноз
func (r *ForecastRepository) UpdateForecast(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	return err
}

// DeleteExpiredForecasts удаляет устаревшие прогнозы
func (r *ForecastRepository) DeleteExpiredForecasts(ctx context.Context) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"valid_until": bson.M{"$lt": time.Now()},
	})
	return err
}

// GetForecastHistory получает историю прогнозов
func (r *ForecastRepository) GetForecastHistory(ctx context.Context, forecastType string, limit int) ([]models.ForecastResult, error) {
	filter := bson.M{
		"type": forecastType,
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "generated_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var forecasts []models.ForecastResult
	if err := cursor.All(ctx, &forecasts); err != nil {
		return nil, err
	}

	return forecasts, nil
}
