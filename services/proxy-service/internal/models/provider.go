package models

import (
	"time"
)

type AuthType string

const (
	AuthTypeBearer AuthType = "bearer"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeAPIKey AuthType = "api_key"
)

type RotationType string

const (
	RotationTypeAPI       RotationType = "api"
	RotationTypeScheduled RotationType = "scheduled"
	RotationTypeManual    RotationType = "manual"
)

type ProxyProvider struct {
	Name         string                 `json:"name" yaml:"name"`
	Type         ProxyType              `json:"type" yaml:"type"`
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
	Priority     int                    `json:"priority" yaml:"priority"`
	API          ProviderAPI            `json:"api" yaml:"api"`
	Endpoints    ProviderEndpoints      `json:"endpoints" yaml:"endpoints"`
	Parameters   ProviderParameters     `json:"parameters" yaml:"parameters"`
	Pricing      ProviderPricing        `json:"pricing" yaml:"pricing"`
}

type ProviderAPI struct {
	BaseURL  string   `json:"base_url" yaml:"base_url"`
	AuthType AuthType `json:"auth_type" yaml:"auth_type"`
	AuthKey  string   `json:"auth_key" yaml:"auth_key"` // Encrypted
}

type ProviderEndpoints struct {
	List     string `json:"list" yaml:"list"`
	Purchase string `json:"purchase" yaml:"purchase"`
	Release  string `json:"release" yaml:"release"`
	Rotate   string `json:"rotate" yaml:"rotate"`
	Check    string `json:"check,omitempty" yaml:"check,omitempty"`
}

type ProviderParameters struct {
	Countries         []string        `json:"countries" yaml:"countries"`
	Protocols         []ProxyProtocol `json:"protocols" yaml:"protocols"`
	RotationType      RotationType    `json:"rotation_type" yaml:"rotation_type"`
	RotationInterval  string          `json:"rotation_interval" yaml:"rotation_interval"`
	MaxConcurrent     int             `json:"max_concurrent,omitempty" yaml:"max_concurrent,omitempty"`
	MinPoolSize       int             `json:"min_pool_size,omitempty" yaml:"min_pool_size,omitempty"`
}

type ProviderPricing struct {
	CostPerProxy float64 `json:"cost_per_proxy" yaml:"cost_per_proxy"`
	Currency     string  `json:"currency" yaml:"currency"`
}

type ProviderConfig struct {
	Providers []ProxyProvider `json:"providers" yaml:"providers"`
}

type ProviderStats struct {
	ProviderName     string    `json:"provider_name"`
	TotalAllocated   int64     `json:"total_allocated"`
	TotalReleased    int64     `json:"total_released"`
	TotalRotated     int64     `json:"total_rotated"`
	ActiveProxies    int64     `json:"active_proxies"`
	BannedProxies    int64     `json:"banned_proxies"`
	AvgResponseTime  float64   `json:"avg_response_time"` // milliseconds
	LastRequestTime  time.Time `json:"last_request_time"`
	LastSuccessTime  time.Time `json:"last_success_time"`
	FailureRate      float64   `json:"failure_rate"` // percentage
	TotalCost        float64   `json:"total_cost"`
}

type ProxyPurchaseParams struct {
	Provider string        `json:"provider"`
	Type     ProxyType     `json:"type"`
	Country  string        `json:"country,omitempty"`
	Protocol ProxyProtocol `json:"protocol,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Quantity int           `json:"quantity,omitempty"`
}

type ProxyResponse struct {
	IP       string        `json:"ip"`
	Port     int           `json:"port"`
	Username string        `json:"username"`
	Password string        `json:"password"`
	Protocol ProxyProtocol `json:"protocol"`
	Country  string        `json:"country,omitempty"`
	City     string        `json:"city,omitempty"`
	ExpireAt time.Time     `json:"expire_at,omitempty"`
}
