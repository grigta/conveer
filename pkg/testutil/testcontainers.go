package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ContainerConfig holds configuration for test containers
type ContainerConfig struct {
	MongoDBVersion  string
	RedisVersion    string
	RabbitMQVersion string
}

// DefaultContainerConfig returns default container versions
func DefaultContainerConfig() ContainerConfig {
	return ContainerConfig{
		MongoDBVersion:  "6.0",
		RedisVersion:    "7.0",
		RabbitMQVersion: "3.12-management",
	}
}

// MongoDBContainer represents a MongoDB test container
type MongoDBContainer struct {
	Container   testcontainers.Container
	URI         string
	Host        string
	Port        string
	DatabaseName string
}

// StartMongoContainer starts a MongoDB container for testing
func StartMongoContainer(ctx context.Context) (*MongoDBContainer, error) {
	return StartMongoContainerWithConfig(ctx, DefaultContainerConfig())
}

// StartMongoContainerWithConfig starts a MongoDB container with custom config
func StartMongoContainerWithConfig(ctx context.Context, config ContainerConfig) (*MongoDBContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("mongo:%s", config.MongoDBVersion),
		ExposedPorts: []string{"27017/tcp"},
		Env: map[string]string{
			"MONGO_INITDB_ROOT_USERNAME": "test",
			"MONGO_INITDB_ROOT_PASSWORD": "test",
			"MONGO_INITDB_DATABASE":      "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Waiting for connections"),
			wait.ForListeningPort("27017/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start MongoDB container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get MongoDB container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "27017")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get MongoDB container port: %w", err)
	}

	uri := fmt.Sprintf("mongodb://test:test@%s:%s/testdb?authSource=admin", host, port.Port())

	return &MongoDBContainer{
		Container:    container,
		URI:          uri,
		Host:         host,
		Port:         port.Port(),
		DatabaseName: "testdb",
	}, nil
}

// Close terminates the MongoDB container
func (m *MongoDBContainer) Close(ctx context.Context) error {
	if m.Container != nil {
		return m.Container.Terminate(ctx)
	}
	return nil
}

// RedisContainer represents a Redis test container
type RedisContainer struct {
	Container testcontainers.Container
	URI       string
	Host      string
	Port      string
}

// StartRedisContainer starts a Redis container for testing
func StartRedisContainer(ctx context.Context) (*RedisContainer, error) {
	return StartRedisContainerWithConfig(ctx, DefaultContainerConfig())
}

// StartRedisContainerWithConfig starts a Redis container with custom config
func StartRedisContainerWithConfig(ctx context.Context, config ContainerConfig) (*RedisContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("redis:%s", config.RedisVersion),
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Ready to accept connections"),
			wait.ForListeningPort("6379/tcp"),
		).WithDeadline(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Redis container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get Redis container port: %w", err)
	}

	uri := fmt.Sprintf("redis://%s:%s", host, port.Port())

	return &RedisContainer{
		Container: container,
		URI:       uri,
		Host:      host,
		Port:      port.Port(),
	}, nil
}

// Close terminates the Redis container
func (r *RedisContainer) Close(ctx context.Context) error {
	if r.Container != nil {
		return r.Container.Terminate(ctx)
	}
	return nil
}

// RabbitMQContainer represents a RabbitMQ test container
type RabbitMQContainer struct {
	Container      testcontainers.Container
	URI            string
	Host           string
	AMQPPort       string
	ManagementPort string
}

// StartRabbitMQContainer starts a RabbitMQ container for testing
func StartRabbitMQContainer(ctx context.Context) (*RabbitMQContainer, error) {
	return StartRabbitMQContainerWithConfig(ctx, DefaultContainerConfig())
}

// StartRabbitMQContainerWithConfig starts a RabbitMQ container with custom config
func StartRabbitMQContainerWithConfig(ctx context.Context, config ContainerConfig) (*RabbitMQContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        fmt.Sprintf("rabbitmq:%s", config.RabbitMQVersion),
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "test",
			"RABBITMQ_DEFAULT_PASS": "test",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server startup complete"),
			wait.ForListeningPort("5672/tcp"),
		).WithDeadline(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start RabbitMQ container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get RabbitMQ container host: %w", err)
	}

	amqpPort, err := container.MappedPort(ctx, "5672")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get RabbitMQ AMQP port: %w", err)
	}

	managementPort, err := container.MappedPort(ctx, "15672")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get RabbitMQ management port: %w", err)
	}

	uri := fmt.Sprintf("amqp://test:test@%s:%s/", host, amqpPort.Port())

	return &RabbitMQContainer{
		Container:      container,
		URI:            uri,
		Host:           host,
		AMQPPort:       amqpPort.Port(),
		ManagementPort: managementPort.Port(),
	}, nil
}

// Close terminates the RabbitMQ container
func (r *RabbitMQContainer) Close(ctx context.Context) error {
	if r.Container != nil {
		return r.Container.Terminate(ctx)
	}
	return nil
}

// TestInfrastructure holds all test containers
type TestInfrastructure struct {
	MongoDB  *MongoDBContainer
	Redis    *RedisContainer
	RabbitMQ *RabbitMQContainer
}

// StartTestInfrastructure starts all test containers
func StartTestInfrastructure(ctx context.Context) (*TestInfrastructure, error) {
	return StartTestInfrastructureWithConfig(ctx, DefaultContainerConfig())
}

// StartTestInfrastructureWithConfig starts all test containers with custom config
func StartTestInfrastructureWithConfig(ctx context.Context, config ContainerConfig) (*TestInfrastructure, error) {
	infra := &TestInfrastructure{}

	var err error

	// Start MongoDB
	infra.MongoDB, err = StartMongoContainerWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start MongoDB: %w", err)
	}

	// Start Redis
	infra.Redis, err = StartRedisContainerWithConfig(ctx, config)
	if err != nil {
		infra.MongoDB.Close(ctx)
		return nil, fmt.Errorf("failed to start Redis: %w", err)
	}

	// Start RabbitMQ
	infra.RabbitMQ, err = StartRabbitMQContainerWithConfig(ctx, config)
	if err != nil {
		infra.MongoDB.Close(ctx)
		infra.Redis.Close(ctx)
		return nil, fmt.Errorf("failed to start RabbitMQ: %w", err)
	}

	return infra, nil
}

// Close terminates all test containers
func (t *TestInfrastructure) Close(ctx context.Context) error {
	var errs []error

	if t.MongoDB != nil {
		if err := t.MongoDB.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.Redis != nil {
		if err := t.Redis.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.RabbitMQ != nil {
		if err := t.RabbitMQ.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing test infrastructure: %v", errs)
	}

	return nil
}

// GetConnectionStrings returns all connection strings
func (t *TestInfrastructure) GetConnectionStrings() map[string]string {
	return map[string]string{
		"mongodb":  t.MongoDB.URI,
		"redis":    t.Redis.URI,
		"rabbitmq": t.RabbitMQ.URI,
	}
}

