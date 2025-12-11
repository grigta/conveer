package database

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/grigta/conveer/pkg/logger"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
	timeout  time.Duration
}

func NewMongoDB(uri string, dbName string, timeout time.Duration) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetMaxPoolSize(50)
	clientOptions.SetMinPoolSize(10)
	clientOptions.SetMaxConnIdleTime(5 * time.Minute)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	logger.Info("Connected to MongoDB", logger.Field{Key: "database", Value: dbName})

	return &MongoDB{
		client:   client,
		database: client.Database(dbName),
		timeout:  timeout,
	}, nil
}

func (m *MongoDB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	return m.client.Disconnect(ctx)
}

func (m *MongoDB) Client() *mongo.Client {
	return m.client
}

func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.database
}

func (m *MongoDB) GetCollection(name string) *mongo.Collection {
	return m.database.Collection(name)
}

func (m *MongoDB) CreateIndexes(collection string, indexes []mongo.IndexModel) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	_, err := m.GetCollection(collection).Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func (m *MongoDB) FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error {
	coll := m.GetCollection(collection)
	err := coll.FindOne(ctx, filter).Decode(result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrNotFound
		}
		return fmt.Errorf("failed to find document: %w", err)
	}
	return nil
}

func (m *MongoDB) Find(ctx context.Context, collection string, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	coll := m.GetCollection(collection)
	return coll.Find(ctx, filter, opts...)
}

func (m *MongoDB) InsertOne(ctx context.Context, collection string, document interface{}) (*mongo.InsertOneResult, error) {
	coll := m.GetCollection(collection)
	return coll.InsertOne(ctx, document)
}

func (m *MongoDB) InsertMany(ctx context.Context, collection string, documents []interface{}) (*mongo.InsertManyResult, error) {
	coll := m.GetCollection(collection)
	return coll.InsertMany(ctx, documents)
}

func (m *MongoDB) UpdateOne(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	coll := m.GetCollection(collection)
	return coll.UpdateOne(ctx, filter, update, opts...)
}

func (m *MongoDB) UpdateMany(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.UpdateOptions) (*mongo.UpdateResult, error) {
	coll := m.GetCollection(collection)
	return coll.UpdateMany(ctx, filter, update, opts...)
}

func (m *MongoDB) DeleteOne(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	coll := m.GetCollection(collection)
	return coll.DeleteOne(ctx, filter)
}

func (m *MongoDB) DeleteMany(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	coll := m.GetCollection(collection)
	return coll.DeleteMany(ctx, filter)
}

func (m *MongoDB) CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error) {
	coll := m.GetCollection(collection)
	return coll.CountDocuments(ctx, filter)
}

func (m *MongoDB) Aggregate(ctx context.Context, collection string, pipeline interface{}, opts ...*options.AggregateOptions) (*mongo.Cursor, error) {
	coll := m.GetCollection(collection)
	return coll.Aggregate(ctx, pipeline, opts...)
}

func (m *MongoDB) WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) (interface{}, error)) (interface{}, error) {
	session, err := m.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	result, err := session.WithTransaction(ctx, fn)
	if err != nil {
		return nil, fmt.Errorf("transaction failed: %w", err)
	}

	return result, nil
}

func (m *MongoDB) CreateTextIndex(collection string, fields []string) error {
	indexModel := mongo.IndexModel{
		Keys: bson.D{},
	}

	for _, field := range fields {
		indexModel.Keys = append(indexModel.Keys.(bson.D), bson.E{Key: field, Value: "text"})
	}

	return m.CreateIndexes(collection, []mongo.IndexModel{indexModel})
}

func (m *MongoDB) CreateUniqueIndex(collection string, fields []string) error {
	indexModel := mongo.IndexModel{
		Keys:    bson.D{},
		Options: options.Index().SetUnique(true),
	}

	for _, field := range fields {
		indexModel.Keys = append(indexModel.Keys.(bson.D), bson.E{Key: field, Value: 1})
	}

	return m.CreateIndexes(collection, []mongo.IndexModel{indexModel})
}

func (m *MongoDB) CreateCompoundIndex(collection string, fields map[string]int) error {
	indexModel := mongo.IndexModel{
		Keys: bson.D{},
	}

	for field, order := range fields {
		indexModel.Keys = append(indexModel.Keys.(bson.D), bson.E{Key: field, Value: order})
	}

	return m.CreateIndexes(collection, []mongo.IndexModel{indexModel})
}
