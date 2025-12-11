// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/grigta/conveer/pkg/testutil"
	"github.com/grigta/conveer/services/proxy-service/internal/models"
	"github.com/grigta/conveer/services/proxy-service/internal/repository"
	"github.com/grigta/conveer/services/proxy-service/internal/service"
)

// ProxyServiceIntegrationSuite tests proxy service with real MongoDB, Redis, and RabbitMQ
type ProxyServiceIntegrationSuite struct {
	suite.Suite
	ctx            context.Context
	cancel         context.CancelFunc
	mongoContainer *testutil.MongoContainer
	redisContainer *testutil.RedisContainer
	rabbitContainer *testutil.RabbitMQContainer
	proxyRepo      repository.ProxyRepository
	providerRepo   repository.ProviderRepository
	svc            *service.ProxyService
}

func (s *ProxyServiceIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 5*time.Minute)

	var err error

	// Start MongoDB
	s.mongoContainer, err = testutil.NewMongoContainer(s.ctx)
	s.Require().NoError(err, "Failed to start MongoDB container")

	// Start Redis
	s.redisContainer, err = testutil.NewRedisContainer(s.ctx)
	s.Require().NoError(err, "Failed to start Redis container")

	// Start RabbitMQ
	s.rabbitContainer, err = testutil.NewRabbitMQContainer(s.ctx)
	s.Require().NoError(err, "Failed to start RabbitMQ container")

	// Initialize repositories with real connections
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")

	s.proxyRepo = repository.NewProxyRepository(db)
	s.providerRepo = repository.NewProviderRepository(db)

	// Note: Full service initialization requires all dependencies
	// For integration tests, we test repository and service interactions
}

func (s *ProxyServiceIntegrationSuite) TearDownSuite() {
	s.cancel()

	if s.mongoContainer != nil {
		_ = s.mongoContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(context.Background())
	}
	if s.rabbitContainer != nil {
		_ = s.rabbitContainer.Terminate(context.Background())
	}
}

func (s *ProxyServiceIntegrationSuite) SetupTest() {
	// Clean up collections before each test
	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")
	_ = db.Collection("proxies").Drop(s.ctx)
	_ = db.Collection("providers").Drop(s.ctx)
}

// TestProxyRepository_CRUD tests Create, Read, Update, Delete operations
func (s *ProxyServiceIntegrationSuite) TestProxyRepository_CRUD() {
	ctx := s.ctx

	// Create proxy
	proxy := &model.Proxy{
		Host:     "192.168.1.1",
		Port:     8080,
		Protocol: "http",
		Username: "user",
		Password: "pass",
		Country:  "RU",
		Provider: "test-provider",
		Status:   model.ProxyStatusAvailable,
	}

	err := s.proxyRepo.Create(ctx, proxy)
	s.Require().NoError(err)
	s.NotEmpty(proxy.ID)

	// Read proxy
	found, err := s.proxyRepo.GetByID(ctx, proxy.ID)
	s.Require().NoError(err)
	s.Equal(proxy.Host, found.Host)
	s.Equal(proxy.Port, found.Port)

	// Update proxy
	proxy.Status = model.ProxyStatusAllocated
	proxy.AccountID = "account-123"
	err = s.proxyRepo.Update(ctx, proxy)
	s.Require().NoError(err)

	// Verify update
	found, err = s.proxyRepo.GetByID(ctx, proxy.ID)
	s.Require().NoError(err)
	s.Equal(model.ProxyStatusAllocated, found.Status)
	s.Equal("account-123", found.AccountID)

	// Delete proxy
	err = s.proxyRepo.Delete(ctx, proxy.ID)
	s.Require().NoError(err)

	// Verify delete
	found, err = s.proxyRepo.GetByID(ctx, proxy.ID)
	s.Error(err)
	s.Nil(found)
}

// TestProxyRepository_FindAvailable tests finding available proxies
func (s *ProxyServiceIntegrationSuite) TestProxyRepository_FindAvailable() {
	ctx := s.ctx

	// Create proxies with different statuses
	proxies := []*model.Proxy{
		{Host: "1.1.1.1", Port: 8080, Status: model.ProxyStatusAvailable, Country: "RU"},
		{Host: "2.2.2.2", Port: 8080, Status: model.ProxyStatusAvailable, Country: "US"},
		{Host: "3.3.3.3", Port: 8080, Status: model.ProxyStatusAllocated, Country: "RU"},
		{Host: "4.4.4.4", Port: 8080, Status: model.ProxyStatusBanned, Country: "RU"},
	}

	for _, p := range proxies {
		err := s.proxyRepo.Create(ctx, p)
		s.Require().NoError(err)
	}

	// Find all available
	available, err := s.proxyRepo.FindAvailable(ctx)
	s.Require().NoError(err)
	s.Len(available, 2)

	// Find available by country
	availableRU, err := s.proxyRepo.FindAvailableByCountry(ctx, "RU")
	s.Require().NoError(err)
	s.Len(availableRU, 1)
	s.Equal("1.1.1.1", availableRU[0].Host)
}

// TestProxyRepository_UpdateHealth tests health status updates
func (s *ProxyServiceIntegrationSuite) TestProxyRepository_UpdateHealth() {
	ctx := s.ctx

	// Create proxy
	proxy := &model.Proxy{
		Host:   "192.168.1.1",
		Port:   8080,
		Status: model.ProxyStatusAvailable,
	}
	err := s.proxyRepo.Create(ctx, proxy)
	s.Require().NoError(err)

	// Update health
	health := &model.ProxyHealth{
		IsAvailable: true,
		LatencyMs:   150,
		FraudScore:  25.5,
		IsVPN:       false,
		LastCheck:   time.Now(),
	}

	err = s.proxyRepo.UpdateHealth(ctx, proxy.ID, health)
	s.Require().NoError(err)

	// Verify health update
	found, err := s.proxyRepo.GetByID(ctx, proxy.ID)
	s.Require().NoError(err)
	s.True(found.Health.IsAvailable)
	s.Equal(int64(150), found.Health.LatencyMs)
	s.InDelta(25.5, found.Health.FraudScore, 0.01)
}

// TestProxyRepository_CountByStatus tests counting proxies by status
func (s *ProxyServiceIntegrationSuite) TestProxyRepository_CountByStatus() {
	ctx := s.ctx

	// Create proxies
	statuses := []model.ProxyStatus{
		model.ProxyStatusAvailable,
		model.ProxyStatusAvailable,
		model.ProxyStatusAvailable,
		model.ProxyStatusAllocated,
		model.ProxyStatusAllocated,
		model.ProxyStatusBanned,
	}

	for i, status := range statuses {
		proxy := &model.Proxy{
			Host:   "192.168.1." + string(rune('1'+i)),
			Port:   8080,
			Status: status,
		}
		err := s.proxyRepo.Create(ctx, proxy)
		s.Require().NoError(err)
	}

	// Count by status
	availableCount, err := s.proxyRepo.CountByStatus(ctx, model.ProxyStatusAvailable)
	s.Require().NoError(err)
	s.Equal(int64(3), availableCount)

	allocatedCount, err := s.proxyRepo.CountByStatus(ctx, model.ProxyStatusAllocated)
	s.Require().NoError(err)
	s.Equal(int64(2), allocatedCount)

	bannedCount, err := s.proxyRepo.CountByStatus(ctx, model.ProxyStatusBanned)
	s.Require().NoError(err)
	s.Equal(int64(1), bannedCount)
}

// TestRedisCaching tests proxy caching in Redis
func (s *ProxyServiceIntegrationSuite) TestRedisCaching() {
	ctx := s.ctx

	redisClient := s.redisContainer.GetClient()

	// Test set and get
	key := "proxy:account:test-account-1"
	value := "proxy-123"

	err := redisClient.Set(ctx, key, value, 5*time.Minute).Err()
	s.Require().NoError(err)

	result, err := redisClient.Get(ctx, key).Result()
	s.Require().NoError(err)
	s.Equal(value, result)

	// Test TTL
	ttl, err := redisClient.TTL(ctx, key).Result()
	s.Require().NoError(err)
	s.True(ttl > 0 && ttl <= 5*time.Minute)

	// Test delete
	err = redisClient.Del(ctx, key).Err()
	s.Require().NoError(err)

	_, err = redisClient.Get(ctx, key).Result()
	s.Error(err) // Should be redis.Nil
}

// TestRabbitMQMessaging tests message publishing and consuming
func (s *ProxyServiceIntegrationSuite) TestRabbitMQMessaging() {
	ctx := s.ctx

	rabbitURL := s.rabbitContainer.GetURL()

	// Create RabbitMQ client
	rabbit, err := testutil.NewTestRabbitMQ(rabbitURL)
	s.Require().NoError(err)
	defer rabbit.Close()

	// Declare exchange and queue
	err = rabbit.DeclareExchange("conveer.events", "topic")
	s.Require().NoError(err)

	queueName := "test.proxy.allocated"
	err = rabbit.DeclareQueue(queueName)
	s.Require().NoError(err)

	err = rabbit.BindQueue(queueName, "conveer.events", "proxy.allocated")
	s.Require().NoError(err)

	// Publish message
	message := map[string]interface{}{
		"proxy_id":   "proxy-123",
		"account_id": "account-456",
		"timestamp":  time.Now().Unix(),
	}

	err = rabbit.Publish("conveer.events", "proxy.allocated", message)
	s.Require().NoError(err)

	// Consume message
	received := make(chan map[string]interface{}, 1)
	go func() {
		msg, err := rabbit.ConsumeOne(ctx, queueName, 5*time.Second)
		if err == nil {
			received <- msg
		}
	}()

	select {
	case msg := <-received:
		s.Equal("proxy-123", msg["proxy_id"])
		s.Equal("account-456", msg["account_id"])
	case <-time.After(10 * time.Second):
		s.Fail("Timeout waiting for message")
	}
}

func TestProxyServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, new(ProxyServiceIntegrationSuite))
}

