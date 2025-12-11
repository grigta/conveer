package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

// MockSMSActivateServer is a mock HTTP server for SMS-Activate API
type MockSMSActivateServer struct {
	Server            *httptest.Server
	Activations       map[string]*MockActivation
	Balance           float64
	mu                sync.RWMutex
	RequestLog        []MockRequest
	DefaultCountryCode string
	ShouldFail        bool
	FailureMessage    string
}

// MockActivation represents a mock SMS activation
type MockActivation struct {
	ID          string
	PhoneNumber string
	Service     string
	Country     string
	Status      string
	SMSCode     string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// MockRequest logs incoming requests
type MockRequest struct {
	Method    string
	Path      string
	Query     map[string]string
	Timestamp time.Time
}

// NewMockSMSActivateServer creates a new mock SMS-Activate server
func NewMockSMSActivateServer() *MockSMSActivateServer {
	mock := &MockSMSActivateServer{
		Activations:        make(map[string]*MockActivation),
		Balance:            1000.0,
		DefaultCountryCode: "0", // Russia
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

func (m *MockSMSActivateServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.RequestLog = append(m.RequestLog, MockRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     parseQuery(r),
		Timestamp: time.Now(),
	})
	m.mu.Unlock()

	if m.ShouldFail {
		http.Error(w, m.FailureMessage, http.StatusInternalServerError)
		return
	}

	action := r.URL.Query().Get("action")

	switch action {
	case "getNumber":
		m.handleGetNumber(w, r)
	case "getStatus":
		m.handleGetStatus(w, r)
	case "setStatus":
		m.handleSetStatus(w, r)
	case "getBalance":
		m.handleGetBalance(w, r)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

func (m *MockSMSActivateServer) handleGetNumber(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	service := r.URL.Query().Get("service")
	country := r.URL.Query().Get("country")
	if country == "" {
		country = m.DefaultCountryCode
	}

	id := fmt.Sprintf("mock_%d", time.Now().UnixNano())
	phone := fmt.Sprintf("+7999%07d", time.Now().UnixNano()%10000000)

	activation := &MockActivation{
		ID:          id,
		PhoneNumber: phone,
		Service:     service,
		Country:     country,
		Status:      "STATUS_WAIT_CODE",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(20 * time.Minute),
	}

	m.Activations[id] = activation
	m.Balance -= 10.0 // Mock cost

	// Response format: ACCESS_NUMBER:id:phone
	w.Write([]byte(fmt.Sprintf("ACCESS_NUMBER:%s:%s", id, phone)))
}

func (m *MockSMSActivateServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.URL.Query().Get("id")
	activation, exists := m.Activations[id]

	if !exists {
		w.Write([]byte("NO_ACTIVATION"))
		return
	}

	if activation.SMSCode != "" {
		w.Write([]byte(fmt.Sprintf("STATUS_OK:%s", activation.SMSCode)))
		return
	}

	w.Write([]byte(activation.Status))
}

func (m *MockSMSActivateServer) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.URL.Query().Get("id")
	status := r.URL.Query().Get("status")

	activation, exists := m.Activations[id]
	if !exists {
		w.Write([]byte("NO_ACTIVATION"))
		return
	}

	switch status {
	case "8": // Cancel
		activation.Status = "STATUS_CANCEL"
		m.Balance += 10.0 // Refund
	case "1": // Ready
		activation.Status = "STATUS_WAIT_CODE"
	case "3": // Another code
		activation.Status = "STATUS_WAIT_RETRY"
	case "6": // Complete
		activation.Status = "STATUS_OK"
	}

	w.Write([]byte("ACCESS_" + status))
}

func (m *MockSMSActivateServer) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Write([]byte(fmt.Sprintf("ACCESS_BALANCE:%.2f", m.Balance)))
}

// SetSMSCode sets the SMS code for an activation
func (m *MockSMSActivateServer) SetSMSCode(activationID, code string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if activation, exists := m.Activations[activationID]; exists {
		activation.SMSCode = code
		activation.Status = "STATUS_OK"
	}
}

// SetBalance sets the account balance
func (m *MockSMSActivateServer) SetBalance(balance float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Balance = balance
}

// SetShouldFail configures the server to return errors
func (m *MockSMSActivateServer) SetShouldFail(shouldFail bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShouldFail = shouldFail
	m.FailureMessage = message
}

// GetRequestLog returns all logged requests
func (m *MockSMSActivateServer) GetRequestLog() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MockRequest{}, m.RequestLog...)
}

// Close closes the mock server
func (m *MockSMSActivateServer) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockSMSActivateServer) URL() string {
	return m.Server.URL
}

// MockProxyProviderServer is a mock HTTP server for proxy providers
type MockProxyProviderServer struct {
	Server      *httptest.Server
	Proxies     map[string]*MockProxy
	mu          sync.RWMutex
	RequestLog  []MockRequest
	ShouldFail  bool
	FailureCode int
}

// MockProxy represents a mock proxy
type MockProxy struct {
	ID        string    `json:"id"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Protocol  string    `json:"protocol"`
	Country   string    `json:"country"`
	City      string    `json:"city"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// NewMockProxyProviderServer creates a new mock proxy provider server
func NewMockProxyProviderServer() *MockProxyProviderServer {
	mock := &MockProxyProviderServer{
		Proxies: make(map[string]*MockProxy),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/purchase", mock.handlePurchase)
	mux.HandleFunc("/release/", mock.handleRelease)
	mux.HandleFunc("/rotate/", mock.handleRotate)
	mux.HandleFunc("/proxies", mock.handleList)
	mux.HandleFunc("/check/", mock.handleCheck)

	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockProxyProviderServer) handlePurchase(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r)

	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	id := fmt.Sprintf("proxy_%d", time.Now().UnixNano())
	proxy := &MockProxy{
		ID:        id,
		IP:        fmt.Sprintf("192.168.%d.%d", time.Now().Unix()%256, time.Now().UnixNano()%256),
		Port:      8080 + int(time.Now().UnixNano()%1000),
		Username:  "user_" + id,
		Password:  "pass_" + id,
		Protocol:  "http",
		Country:   "US",
		City:      "New York",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Active:    true,
	}

	m.mu.Lock()
	m.Proxies[id] = proxy
	m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxy)
}

func (m *MockProxyProviderServer) handleRelease(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r)

	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	// Extract proxy ID from path
	id := r.URL.Path[len("/release/"):]

	m.mu.Lock()
	delete(m.Proxies, id)
	m.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (m *MockProxyProviderServer) handleRotate(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r)

	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	id := r.URL.Path[len("/rotate/"):]

	m.mu.Lock()
	proxy, exists := m.Proxies[id]
	if exists {
		// Generate new IP
		proxy.IP = fmt.Sprintf("192.168.%d.%d", time.Now().Unix()%256, time.Now().UnixNano()%256)
		proxy.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	m.mu.Unlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxy)
}

func (m *MockProxyProviderServer) handleList(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r)

	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	m.mu.RLock()
	proxies := make([]*MockProxy, 0, len(m.Proxies))
	for _, p := range m.Proxies {
		proxies = append(proxies, p)
	}
	m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxies)
}

func (m *MockProxyProviderServer) handleCheck(w http.ResponseWriter, r *http.Request) {
	m.logRequest(r)

	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	id := r.URL.Path[len("/check/"):]

	m.mu.RLock()
	proxy, exists := m.Proxies[id]
	m.mu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"active":  proxy.Active,
		"healthy": proxy.Active,
	})
}

func (m *MockProxyProviderServer) logRequest(r *http.Request) {
	m.mu.Lock()
	m.RequestLog = append(m.RequestLog, MockRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     parseQuery(r),
		Timestamp: time.Now(),
	})
	m.mu.Unlock()
}

// SetShouldFail configures the server to return errors
func (m *MockProxyProviderServer) SetShouldFail(shouldFail bool, code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShouldFail = shouldFail
	m.FailureCode = code
}

// AddProxy adds a proxy to the mock server
func (m *MockProxyProviderServer) AddProxy(proxy *MockProxy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Proxies[proxy.ID] = proxy
}

// GetRequestLog returns all logged requests
func (m *MockProxyProviderServer) GetRequestLog() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MockRequest{}, m.RequestLog...)
}

// Close closes the mock server
func (m *MockProxyProviderServer) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockProxyProviderServer) URL() string {
	return m.Server.URL
}

// MockIPQSServer is a mock HTTP server for IPQualityScore API
type MockIPQSServer struct {
	Server        *httptest.Server
	DefaultScore  float64
	IPScores      map[string]float64
	mu            sync.RWMutex
	ShouldFail    bool
	FailureCode   int
}

// NewMockIPQSServer creates a new mock IPQualityScore server
func NewMockIPQSServer() *MockIPQSServer {
	mock := &MockIPQSServer{
		DefaultScore: 25.0,
		IPScores:     make(map[string]float64),
	}

	mock.Server = httptest.NewServer(http.HandlerFunc(mock.handleRequest))
	return mock
}

func (m *MockIPQSServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if m.ShouldFail {
		w.WriteHeader(m.FailureCode)
		return
	}

	// Extract IP from path: /api/json/ip/{api_key}/{ip}
	parts := r.URL.Path
	// For simplicity, just return mock data
	
	m.mu.RLock()
	score := m.DefaultScore
	// In a real implementation, parse IP from path
	m.mu.RUnlock()

	response := map[string]interface{}{
		"success":         true,
		"fraud_score":     score,
		"country_code":    "US",
		"city":            "New York",
		"ISP":             "Test ISP",
		"mobile":          false,
		"proxy":           true,
		"vpn":             false,
		"tor":             false,
		"recent_abuse":    false,
		"connection_type": "Residential",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SetDefaultScore sets the default fraud score
func (m *MockIPQSServer) SetDefaultScore(score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DefaultScore = score
}

// SetIPScore sets a specific score for an IP
func (m *MockIPQSServer) SetIPScore(ip string, score float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IPScores[ip] = score
}

// SetShouldFail configures the server to return errors
func (m *MockIPQSServer) SetShouldFail(shouldFail bool, code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShouldFail = shouldFail
	m.FailureCode = code
}

// Close closes the mock server
func (m *MockIPQSServer) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockIPQSServer) URL() string {
	return m.Server.URL
}

// Helper function to parse query parameters
func parseQuery(r *http.Request) map[string]string {
	query := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}
	return query
}

// MockTelegramBotAPI is a mock HTTP server for Telegram Bot API
type MockTelegramBotAPI struct {
	Server      *httptest.Server
	Messages    []map[string]interface{}
	Updates     []map[string]interface{}
	mu          sync.RWMutex
	ShouldFail  bool
}

// NewMockTelegramBotAPI creates a new mock Telegram Bot API server
func NewMockTelegramBotAPI() *MockTelegramBotAPI {
	mock := &MockTelegramBotAPI{
		Messages: make([]map[string]interface{}, 0),
		Updates:  make([]map[string]interface{}, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/bot", mock.handleBot)
	mux.HandleFunc("/", mock.handleRequest)

	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockTelegramBotAPI) handleBot(w http.ResponseWriter, r *http.Request) {
	m.handleRequest(w, r)
}

func (m *MockTelegramBotAPI) handleRequest(w http.ResponseWriter, r *http.Request) {
	if m.ShouldFail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse the method from the path
	response := map[string]interface{}{
		"ok":     true,
		"result": map[string]interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AddUpdate adds a mock update to be returned by getUpdates
func (m *MockTelegramBotAPI) AddUpdate(update map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Updates = append(m.Updates, update)
}

// GetSentMessages returns all sent messages
func (m *MockTelegramBotAPI) GetSentMessages() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]map[string]interface{}{}, m.Messages...)
}

// SetShouldFail configures the server to return errors
func (m *MockTelegramBotAPI) SetShouldFail(shouldFail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShouldFail = shouldFail
}

// Close closes the mock server
func (m *MockTelegramBotAPI) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockTelegramBotAPI) URL() string {
	return m.Server.URL
}

