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

// AlertRepository репозиторий для работы с алертами
type AlertRepository struct {
	rulesCollection  *mongo.Collection
	eventsCollection *mongo.Collection
}

// NewAlertRepository создает новый репозиторий алертов
func NewAlertRepository(db *mongo.Database) *AlertRepository {
	return &AlertRepository{
		rulesCollection:  db.Collection("alert_rules"),
		eventsCollection: db.Collection("alert_events"),
	}
}

// CreateRule создает новое правило алерта
func (r *AlertRepository) CreateRule(ctx context.Context, rule *models.AlertRule) error {
	rule.ID = primitive.NewObjectID()
	_, err := r.rulesCollection.InsertOne(ctx, rule)
	return err
}

// GetRuleByID получает правило по ID
func (r *AlertRepository) GetRuleByID(ctx context.Context, id primitive.ObjectID) (*models.AlertRule, error) {
	var rule models.AlertRule
	err := r.rulesCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetRuleByName получает правило по имени
func (r *AlertRepository) GetRuleByName(ctx context.Context, name string) (*models.AlertRule, error) {
	var rule models.AlertRule
	err := r.rulesCollection.FindOne(ctx, bson.M{"name": name}).Decode(&rule)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetActiveRules получает активные правила
func (r *AlertRepository) GetActiveRules(ctx context.Context) ([]models.AlertRule, error) {
	filter := bson.M{"enabled": true}

	cursor, err := r.rulesCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.AlertRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// GetAllRules получает все правила
func (r *AlertRepository) GetAllRules(ctx context.Context) ([]models.AlertRule, error) {
	cursor, err := r.rulesCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rules []models.AlertRule
	if err := cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	return rules, nil
}

// UpdateRule обновляет правило
func (r *AlertRepository) UpdateRule(ctx context.Context, rule *models.AlertRule) error {
	_, err := r.rulesCollection.ReplaceOne(
		ctx,
		bson.M{"_id": rule.ID},
		rule,
	)
	return err
}

// UpdateRuleField обновляет поле правила
func (r *AlertRepository) UpdateRuleField(ctx context.Context, id primitive.ObjectID, field string, value interface{}) error {
	_, err := r.rulesCollection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{field: value}},
	)
	return err
}

// DeleteRule удаляет правило
func (r *AlertRepository) DeleteRule(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.rulesCollection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// SaveAlertEvent сохраняет событие алерта
func (r *AlertRepository) SaveAlertEvent(ctx context.Context, event *models.AlertEvent) error {
	event.ID = primitive.NewObjectID()
	_, err := r.eventsCollection.InsertOne(ctx, event)
	return err
}

// GetAlertEventByID получает событие по ID
func (r *AlertRepository) GetAlertEventByID(ctx context.Context, id primitive.ObjectID) (*models.AlertEvent, error) {
	var event models.AlertEvent
	err := r.eventsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetActiveAlerts получает активные (не подтвержденные) алерты
func (r *AlertRepository) GetActiveAlerts(ctx context.Context) ([]models.AlertEvent, error) {
	filter := bson.M{"acknowledged": false}

	opts := options.Find().SetSort(bson.D{{Key: "fired_at", Value: -1}})

	cursor, err := r.eventsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.AlertEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// GetAlerts получает алерты с фильтрами
func (r *AlertRepository) GetAlerts(ctx context.Context, unacknowledgedOnly bool, severity string) ([]models.AlertEvent, error) {
	filter := bson.M{}

	// Добавляем фильтр по acknowledged если требуется
	if unacknowledgedOnly {
		filter["acknowledged"] = false
	}

	// Добавляем фильтр по severity если указан
	if severity != "" {
		filter["severity"] = severity
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "fired_at", Value: -1}}).
		SetLimit(100) // Ограничиваем количество результатов

	cursor, err := r.eventsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.AlertEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// GetAlertsBySeverity получает алерты по уровню критичности
func (r *AlertRepository) GetAlertsBySeverity(ctx context.Context, severity string, limit int) ([]models.AlertEvent, error) {
	filter := bson.M{"severity": severity}

	opts := options.Find().
		SetSort(bson.D{{Key: "fired_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := r.eventsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.AlertEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// GetRecentAlerts получает последние алерты
func (r *AlertRepository) GetRecentAlerts(ctx context.Context, hours int) ([]models.AlertEvent, error) {
	filter := bson.M{
		"fired_at": bson.M{"$gte": time.Now().Add(-time.Duration(hours) * time.Hour)},
	}

	opts := options.Find().SetSort(bson.D{{Key: "fired_at", Value: -1}})

	cursor, err := r.eventsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []models.AlertEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

// AcknowledgeAlert подтверждает алерт
func (r *AlertRepository) AcknowledgeAlert(ctx context.Context, id primitive.ObjectID, acknowledgedBy string) error {
	now := time.Now()
	_, err := r.eventsCollection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"acknowledged": true,
			"acknowledged_at": now,
			"acknowledged_by": acknowledgedBy,
		}},
	)
	return err
}

// GetAlertSummary получает сводку по алертам
func (r *AlertRepository) GetAlertSummary(ctx context.Context) (*models.AlertSummary, error) {
	// Подсчет общего количества
	totalCount, err := r.eventsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	// Подсчет не подтвержденных
	unacknowledgedCount, err := r.eventsCollection.CountDocuments(ctx, bson.M{"acknowledged": false})
	if err != nil {
		return nil, err
	}

	// Группировка по severity
	severityPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": "$severity",
			"count": bson.M{"$sum": 1},
		}}},
	}

	severityCursor, err := r.eventsCollection.Aggregate(ctx, severityPipeline)
	if err != nil {
		return nil, err
	}
	defer severityCursor.Close(ctx)

	bySeverity := make(map[string]int64)
	for severityCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := severityCursor.Decode(&result); err != nil {
			continue
		}
		bySeverity[result.ID] = result.Count
	}

	// Группировка по платформе
	platformPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": "$platform",
			"count": bson.M{"$sum": 1},
		}}},
	}

	platformCursor, err := r.eventsCollection.Aggregate(ctx, platformPipeline)
	if err != nil {
		return nil, err
	}
	defer platformCursor.Close(ctx)

	byPlatform := make(map[string]int64)
	for platformCursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := platformCursor.Decode(&result); err != nil {
			continue
		}
		byPlatform[result.ID] = result.Count
	}

	// Получение последних алертов
	recentAlerts, _ := r.GetRecentAlerts(ctx, 24)

	return &models.AlertSummary{
		TotalAlerts:         totalCount,
		UnacknowledgedCount: unacknowledgedCount,
		BySeverity:          bySeverity,
		ByPlatform:          byPlatform,
		RecentAlerts:        recentAlerts,
	}, nil
}

// DeleteOldAlerts удаляет старые алерты
func (r *AlertRepository) DeleteOldAlerts(ctx context.Context, olderThan time.Time) error {
	_, err := r.eventsCollection.DeleteMany(ctx, bson.M{
		"fired_at": bson.M{"$lt": olderThan},
	})
	return err
}