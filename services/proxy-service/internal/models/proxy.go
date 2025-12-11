package models

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProxyProtocol string

const (
	ProtocolHTTP   ProxyProtocol = "http"
	ProtocolHTTPS  ProxyProtocol = "https"
	ProtocolSOCKS5 ProxyProtocol = "socks5"
)

type ProxyType string

const (
	ProxyTypeMobile      ProxyType = "mobile"
	ProxyTypeResidential ProxyType = "residential"
)

type ProxyStatus string

const (
	ProxyStatusActive   ProxyStatus = "active"
	ProxyStatusExpired  ProxyStatus = "expired"
	ProxyStatusBanned   ProxyStatus = "banned"
	ProxyStatusChecking ProxyStatus = "checking"
	ProxyStatusRotating ProxyStatus = "rotating"
	ProxyStatusReleased ProxyStatus = "released"
)

type Proxy struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Provider     string             `bson:"provider" json:"provider"`
	IP           string             `bson:"ip" json:"ip"`
	Port         int                `bson:"port" json:"port"`
	Protocol     ProxyProtocol      `bson:"protocol" json:"protocol"`
	Username     string             `bson:"username" json:"username"`
	Password     string             `bson:"password" json:"password"` // Encrypted
	Type         ProxyType          `bson:"type" json:"type"`
	Country      string             `bson:"country" json:"country"`
	City         string             `bson:"city" json:"city"`
	Status       ProxyStatus        `bson:"status" json:"status"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt    time.Time          `bson:"expires_at" json:"expires_at"`
	LastChecked  time.Time          `bson:"last_checked" json:"last_checked"`
}

type ProxyHealth struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProxyID         primitive.ObjectID `bson:"proxy_id" json:"proxy_id"`
	Latency         int                `bson:"latency" json:"latency"` // milliseconds
	FraudScore      float64            `bson:"fraud_score" json:"fraud_score"` // 0-100
	IsVPN           bool               `bson:"is_vpn" json:"is_vpn"`
	IsProxy         bool               `bson:"is_proxy" json:"is_proxy"`
	IsTor           bool               `bson:"is_tor" json:"is_tor"`
	BlacklistStatus bool               `bson:"blacklist_status" json:"blacklist_status"`
	LastCheck       time.Time          `bson:"last_check" json:"last_check"`
	FailedChecks    int                `bson:"failed_checks" json:"failed_checks"`
}

type BindingStatus string

const (
	BindingStatusActive   BindingStatus = "active"
	BindingStatusRotating BindingStatus = "rotating"
	BindingStatusReleased BindingStatus = "released"
)

type ProxyBinding struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProxyID    primitive.ObjectID `bson:"proxy_id" json:"proxy_id"`
	AccountID  string             `bson:"account_id" json:"account_id"`
	BoundAt    time.Time          `bson:"bound_at" json:"bound_at"`
	LastUsedAt time.Time          `bson:"last_used_at" json:"last_used_at"`
	Status     BindingStatus      `bson:"status" json:"status"`
}

type ProxyFilters struct {
	Type    ProxyType   `json:"type,omitempty"`
	Country string      `json:"country,omitempty"`
	Status  ProxyStatus `json:"status,omitempty"`
	Provider string     `json:"provider,omitempty"`
}

type ProxyAllocationRequest struct {
	AccountID    string        `json:"account_id" binding:"required"`
	Type         ProxyType     `json:"type,omitempty"`
	Country      string        `json:"country,omitempty"`
	Protocol     ProxyProtocol `json:"protocol,omitempty"`
}

type ProxyStats struct {
	TotalProxies     int64              `json:"total_proxies"`
	ActiveProxies    int64              `json:"active_proxies"`
	ExpiredProxies   int64              `json:"expired_proxies"`
	BannedProxies    int64              `json:"banned_proxies"`
	TotalBindings    int64              `json:"total_bindings"`
	ProxiesByType    map[string]int64   `json:"proxies_by_type"`
	ProxiesByCountry map[string]int64   `json:"proxies_by_country"`
	AvgFraudScore    float64            `json:"avg_fraud_score"`
	AvgLatency       float64            `json:"avg_latency"`
}
