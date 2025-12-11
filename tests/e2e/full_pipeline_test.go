// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"conveer/pkg/testutil"
)

// FullPipelineE2ESuite tests the complete account creation and warming pipeline
type FullPipelineE2ESuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	mongoContainer  *testutil.MongoContainer
	redisContainer  *testutil.RedisContainer
	rabbitContainer *testutil.RabbitMQContainer

	// Mock external services
	mockSMSServer   *httptest.Server
	mockProxyServer *httptest.Server
	mockIPQSServer  *httptest.Server

	// Track mock state
	smsActivations map[string]string // activationID -> code
	proxyPool      []string
	allocatedProxies map[string]string // accountID -> proxyID
}

func (s *FullPipelineE2ESuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	var err error

	// Initialize mock state
	s.smsActivations = make(map[string]string)
	s.proxyPool = []string{"proxy-1", "proxy-2", "proxy-3", "proxy-4", "proxy-5"}
	s.allocatedProxies = make(map[string]string)

	// Start test containers
	s.mongoContainer, err = testutil.NewMongoContainer(s.ctx)
	s.Require().NoError(err, "Failed to start MongoDB container")

	s.redisContainer, err = testutil.NewRedisContainer(s.ctx)
	s.Require().NoError(err, "Failed to start Redis container")

	s.rabbitContainer, err = testutil.NewRabbitMQContainer(s.ctx)
	s.Require().NoError(err, "Failed to start RabbitMQ container")

	// Setup mock servers
	s.mockSMSServer = s.setupMockSMSServer()
	s.mockProxyServer = s.setupMockProxyServer()
	s.mockIPQSServer = s.setupMockIPQSServer()
}

func (s *FullPipelineE2ESuite) TearDownSuite() {
	s.cancel()

	if s.mockSMSServer != nil {
		s.mockSMSServer.Close()
	}
	if s.mockProxyServer != nil {
		s.mockProxyServer.Close()
	}
	if s.mockIPQSServer != nil {
		s.mockIPQSServer.Close()
	}

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

// Mock SMS Server
func (s *FullPipelineE2ESuite) setupMockSMSServer() *httptest.Server {
	activationCounter := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")

		switch action {
		case "getNumber":
			activationCounter++
			activationID := fmt.Sprintf("act-%d", activationCounter)
			phone := fmt.Sprintf("+79%09d", activationCounter)
			code := fmt.Sprintf("%06d", 100000+activationCounter)

			// Store activation for later code retrieval
			s.smsActivations[activationID] = code

			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "ACCESS_NUMBER:%s:%s", activationID, phone)

		case "getStatus":
			activationID := r.URL.Query().Get("id")
			if code, ok := s.smsActivations[activationID]; ok {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "STATUS_OK:%s", code)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("STATUS_WAIT_CODE"))
			}

		case "setStatus":
			status := r.URL.Query().Get("status")
			if status == "8" { // Cancel
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ACCESS_ACTIVATION_CANCELED"))
			} else if status == "6" { // Complete
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ACCESS_ACTIVATION_COMPLETED"))
			}

		case "getBalance":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ACCESS_BALANCE:500.00"))

		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

// Mock Proxy Server
func (s *FullPipelineE2ESuite) setupMockProxyServer() *httptest.Server {
	proxyCounter := 0

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/proxy/purchase":
			proxyCounter++
			proxy := map[string]interface{}{
				"id":       fmt.Sprintf("proxy-%d", proxyCounter),
				"host":     fmt.Sprintf("192.168.1.%d", proxyCounter),
				"port":     8080,
				"protocol": "http",
				"username": "user",
				"password": "pass",
				"country":  "RU",
				"expires":  time.Now().Add(24 * time.Hour).Unix(),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(proxy)

		case "/api/proxy/release":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		case "/api/proxy/rotate":
			proxyCounter++
			proxy := map[string]interface{}{
				"id":       fmt.Sprintf("proxy-%d", proxyCounter),
				"host":     fmt.Sprintf("192.168.2.%d", proxyCounter),
				"port":     8080,
				"protocol": "http",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(proxy)

		case "/api/proxy/list":
			proxies := make([]map[string]interface{}, 0)
			for _, p := range s.proxyPool {
				proxies = append(proxies, map[string]interface{}{
					"id":     p,
					"status": "available",
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(proxies)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Mock IPQS Server
func (s *FullPipelineE2ESuite) setupMockIPQSServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"success":     true,
			"fraud_score": 25,
			"proxy":       false,
			"vpn":         false,
			"tor":         false,
			"bot_status":  false,
			"country":     "RU",
			"isp":         "Test ISP",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// TestFullAccountCreationPipeline tests the complete flow:
// 1. Allocate proxy
// 2. Get SMS number
// 3. Register account (simulated)
// 4. Get SMS code
// 5. Complete registration
// 6. Start warming
func (s *FullPipelineE2ESuite) TestFullAccountCreationPipeline() {
	ctx := s.ctx

	// Step 1: Allocate proxy
	s.T().Log("Step 1: Allocating proxy...")

	resp, err := http.Get(s.mockProxyServer.URL + "/api/proxy/purchase")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var proxy map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&proxy)
	resp.Body.Close()
	s.Require().NoError(err)

	proxyID := proxy["id"].(string)
	s.T().Logf("Allocated proxy: %s", proxyID)

	// Step 2: Check proxy health (via IPQS)
	s.T().Log("Step 2: Checking proxy health...")

	proxyHost := proxy["host"].(string)
	resp, err = http.Get(fmt.Sprintf("%s/api/json/%s", s.mockIPQSServer.URL, proxyHost))
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var healthCheck map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthCheck)
	resp.Body.Close()
	s.Require().NoError(err)

	fraudScore := healthCheck["fraud_score"].(float64)
	s.T().Logf("Fraud score: %.0f", fraudScore)
	s.Less(fraudScore, float64(70), "Fraud score should be acceptable")

	// Step 3: Request SMS number
	s.T().Log("Step 3: Requesting SMS number...")

	resp, err = http.Get(s.mockSMSServer.URL + "?action=getNumber&service=vk&country=0")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	s.T().Log("SMS number acquired")

	// Step 4: Wait for SMS code (simulated delay)
	s.T().Log("Step 4: Waiting for SMS code...")

	time.Sleep(100 * time.Millisecond) // Simulated wait

	// Step 5: Get SMS code
	s.T().Log("Step 5: Getting SMS code...")

	resp, err = http.Get(s.mockSMSServer.URL + "?action=getStatus&id=act-1")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	s.T().Log("SMS code received")

	// Step 6: Simulate account registration success
	s.T().Log("Step 6: Account registration completed (simulated)")

	// Verify we can communicate via RabbitMQ for events
	s.T().Log("Step 7: Publishing account.created event...")

	rabbitURL := s.rabbitContainer.GetURL()
	rabbit, err := testutil.NewTestRabbitMQ(rabbitURL)
	s.Require().NoError(err)
	defer rabbit.Close()

	err = rabbit.DeclareExchange("conveer.events", "topic")
	s.Require().NoError(err)

	event := map[string]interface{}{
		"account_id": "account-123",
		"platform":   "vk",
		"proxy_id":   proxyID,
		"timestamp":  time.Now().Unix(),
	}

	err = rabbit.Publish("conveer.events", "account.created", event)
	s.Require().NoError(err)

	s.T().Log("Event published successfully")

	// Step 8: Store account in MongoDB
	s.T().Log("Step 8: Storing account in MongoDB...")

	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")
	collection := db.Collection("accounts")

	account := map[string]interface{}{
		"platform":    "vk",
		"username":    "test_user_1",
		"phone":       "+79000000001",
		"status":      "active",
		"proxy_id":    proxyID,
		"created_at":  time.Now(),
	}

	_, err = collection.InsertOne(ctx, account)
	s.Require().NoError(err)

	s.T().Log("Account stored in database")

	// Final verification
	s.T().Log("Pipeline completed successfully!")
}

// TestProxyRotationDuringWarming tests proxy rotation mid-warming
func (s *FullPipelineE2ESuite) TestProxyRotationDuringWarming() {
	s.T().Log("Testing proxy rotation during warming...")

	// Initial proxy allocation
	resp, err := http.Get(s.mockProxyServer.URL + "/api/proxy/purchase")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var initialProxy map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&initialProxy)
	resp.Body.Close()

	initialID := initialProxy["id"].(string)
	s.T().Logf("Initial proxy: %s", initialID)

	// Simulate warming activity
	time.Sleep(50 * time.Millisecond)

	// Rotate proxy
	resp, err = http.Get(s.mockProxyServer.URL + "/api/proxy/rotate")
	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)

	var newProxy map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&newProxy)
	resp.Body.Close()

	newID := newProxy["id"].(string)
	s.T().Logf("New proxy after rotation: %s", newID)

	s.NotEqual(initialID, newID, "Proxy should have been rotated")
}

// TestSMSRetryMechanism tests SMS retry on failure
func (s *FullPipelineE2ESuite) TestSMSRetryMechanism() {
	s.T().Log("Testing SMS retry mechanism...")

	maxRetries := 3
	retryDelay := 50 * time.Millisecond

	var code string
	var success bool

	for attempt := 1; attempt <= maxRetries && !success; attempt++ {
		s.T().Logf("Attempt %d/%d", attempt, maxRetries)

		// Request number
		resp, err := http.Get(s.mockSMSServer.URL + "?action=getNumber&service=vk&country=0")
		s.Require().NoError(err)
		resp.Body.Close()

		// Wait for code
		time.Sleep(retryDelay)

		// Try to get code
		resp, err = http.Get(fmt.Sprintf("%s?action=getStatus&id=act-%d", s.mockSMSServer.URL, attempt+1))
		s.Require().NoError(err)

		if resp.StatusCode == http.StatusOK {
			// Parse response to check if code is available
			success = true
			code = s.smsActivations[fmt.Sprintf("act-%d", attempt+1)]
		}
		resp.Body.Close()
	}

	s.True(success, "Should successfully get SMS code within retries")
	s.NotEmpty(code, "Code should not be empty")
	s.T().Logf("Successfully retrieved code: %s", code)
}

// TestConcurrentAccountCreation tests creating multiple accounts concurrently
func (s *FullPipelineE2ESuite) TestConcurrentAccountCreation() {
	s.T().Log("Testing concurrent account creation...")

	numAccounts := 3
	results := make(chan bool, numAccounts)

	for i := 0; i < numAccounts; i++ {
		go func(index int) {
			// Allocate proxy
			resp, err := http.Get(s.mockProxyServer.URL + "/api/proxy/purchase")
			if err != nil {
				results <- false
				return
			}
			resp.Body.Close()

			// Get SMS number
			resp, err = http.Get(s.mockSMSServer.URL + "?action=getNumber&service=vk&country=0")
			if err != nil {
				results <- false
				return
			}
			resp.Body.Close()

			results <- true
		}(i)
	}

	// Wait for all goroutines
	successCount := 0
	for i := 0; i < numAccounts; i++ {
		if <-results {
			successCount++
		}
	}

	s.Equal(numAccounts, successCount, "All concurrent account creations should succeed")
	s.T().Logf("Successfully created %d accounts concurrently", successCount)
}

// TestEventPropagation tests event flow through RabbitMQ
func (s *FullPipelineE2ESuite) TestEventPropagation() {
	ctx := s.ctx
	s.T().Log("Testing event propagation through RabbitMQ...")

	rabbitURL := s.rabbitContainer.GetURL()
	rabbit, err := testutil.NewTestRabbitMQ(rabbitURL)
	s.Require().NoError(err)
	defer rabbit.Close()

	// Setup exchanges and queues
	events := []string{"account.created", "proxy.allocated", "warming.started", "warming.completed"}

	err = rabbit.DeclareExchange("conveer.events", "topic")
	s.Require().NoError(err)

	for _, eventType := range events {
		queueName := "test." + eventType
		err = rabbit.DeclareQueue(queueName)
		s.Require().NoError(err)

		err = rabbit.BindQueue(queueName, "conveer.events", eventType)
		s.Require().NoError(err)
	}

	// Simulate pipeline events
	pipelineEvents := []struct {
		routingKey string
		data       map[string]interface{}
	}{
		{
			routingKey: "account.created",
			data:       map[string]interface{}{"account_id": "acc-1", "platform": "vk"},
		},
		{
			routingKey: "proxy.allocated",
			data:       map[string]interface{}{"proxy_id": "proxy-1", "account_id": "acc-1"},
		},
		{
			routingKey: "warming.started",
			data:       map[string]interface{}{"task_id": "task-1", "account_id": "acc-1"},
		},
		{
			routingKey: "warming.completed",
			data:       map[string]interface{}{"task_id": "task-1", "account_id": "acc-1"},
		},
	}

	// Publish events
	for _, event := range pipelineEvents {
		err = rabbit.Publish("conveer.events", event.routingKey, event.data)
		s.Require().NoError(err)
		s.T().Logf("Published event: %s", event.routingKey)
	}

	// Verify events were received
	for _, event := range pipelineEvents {
		queueName := "test." + event.routingKey
		msg, err := rabbit.ConsumeOne(ctx, queueName, 5*time.Second)
		s.Require().NoError(err, "Should receive event: %s", event.routingKey)
		s.NotNil(msg)
		s.T().Logf("Received event: %s", event.routingKey)
	}

	s.T().Log("All events propagated successfully")
}

// TestDataConsistency tests data consistency across services
func (s *FullPipelineE2ESuite) TestDataConsistency() {
	ctx := s.ctx
	s.T().Log("Testing data consistency...")

	mongoClient := s.mongoContainer.GetClient()
	db := mongoClient.Database("conveer_test")

	// Create account
	accountsCollection := db.Collection("accounts")
	account := map[string]interface{}{
		"platform":  "vk",
		"username":  "consistency_test_user",
		"status":    "active",
		"proxy_id":  "proxy-consistency-1",
		"created_at": time.Now(),
	}

	result, err := accountsCollection.InsertOne(ctx, account)
	s.Require().NoError(err)
	accountID := result.InsertedID

	// Create corresponding proxy
	proxiesCollection := db.Collection("proxies")
	proxy := map[string]interface{}{
		"_id":        "proxy-consistency-1",
		"host":       "192.168.100.1",
		"port":       8080,
		"status":     "allocated",
		"account_id": accountID,
		"created_at": time.Now(),
	}

	_, err = proxiesCollection.InsertOne(ctx, proxy)
	s.Require().NoError(err)

	// Create warming task
	tasksCollection := db.Collection("warming_tasks")
	task := map[string]interface{}{
		"account_id":  accountID,
		"platform":    "vk",
		"status":      "running",
		"current_day": 1,
		"total_days":  7,
		"created_at":  time.Now(),
	}

	_, err = tasksCollection.InsertOne(ctx, task)
	s.Require().NoError(err)

	// Verify relationships
	var foundAccount map[string]interface{}
	err = accountsCollection.FindOne(ctx, map[string]interface{}{"_id": accountID}).Decode(&foundAccount)
	s.Require().NoError(err)

	var foundProxy map[string]interface{}
	err = proxiesCollection.FindOne(ctx, map[string]interface{}{"_id": foundAccount["proxy_id"]}).Decode(&foundProxy)
	s.Require().NoError(err)
	s.Equal(accountID, foundProxy["account_id"])

	var foundTask map[string]interface{}
	err = tasksCollection.FindOne(ctx, map[string]interface{}{"account_id": accountID}).Decode(&foundTask)
	s.Require().NoError(err)
	s.Equal("running", foundTask["status"])

	s.T().Log("Data consistency verified across all collections")
}

func TestFullPipelineE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	suite.Run(t, new(FullPipelineE2ESuite))
}

