package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	// Cache TTL for different types of data
	AccountStatsCacheTTL  = 5 * time.Minute
	WarmingStatsCacheTTL  = 10 * time.Minute
	ProxyStatsCacheTTL    = 10 * time.Minute
	SMSStatsCacheTTL      = 15 * time.Minute
	OverallStatsCacheTTL  = 3 * time.Minute
	DetailedStatsCacheTTL = 5 * time.Minute
)

// CacheHelper provides methods for caching stats data
type CacheHelper struct {
	client *redis.Client
}

func NewCacheHelper(client *redis.Client) *CacheHelper {
	return &CacheHelper{client: client}
}

// SetAccountStats caches account statistics for a platform
func (c *CacheHelper) SetAccountStats(ctx context.Context, platform string, stats *AccountStats) error {
	key := fmt.Sprintf("stats:account:%s", platform)
	return c.setCache(ctx, key, stats, AccountStatsCacheTTL)
}

// GetAccountStats retrieves cached account statistics for a platform
func (c *CacheHelper) GetAccountStats(ctx context.Context, platform string) (*AccountStats, error) {
	key := fmt.Sprintf("stats:account:%s", platform)
	var stats AccountStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetWarmingStats caches warming statistics
func (c *CacheHelper) SetWarmingStats(ctx context.Context, platform string, stats *WarmingStats) error {
	key := fmt.Sprintf("stats:warming:%s", platform)
	return c.setCache(ctx, key, stats, WarmingStatsCacheTTL)
}

// GetWarmingStats retrieves cached warming statistics
func (c *CacheHelper) GetWarmingStats(ctx context.Context, platform string) (*WarmingStats, error) {
	key := fmt.Sprintf("stats:warming:%s", platform)
	var stats WarmingStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetProxyStats caches proxy statistics
func (c *CacheHelper) SetProxyStats(ctx context.Context, stats *ProxyStats) error {
	key := "stats:proxy"
	return c.setCache(ctx, key, stats, ProxyStatsCacheTTL)
}

// GetProxyStats retrieves cached proxy statistics
func (c *CacheHelper) GetProxyStats(ctx context.Context) (*ProxyStats, error) {
	key := "stats:proxy"
	var stats ProxyStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetSMSStats caches SMS statistics
func (c *CacheHelper) SetSMSStats(ctx context.Context, stats *SMSStats) error {
	key := "stats:sms"
	return c.setCache(ctx, key, stats, SMSStatsCacheTTL)
}

// GetSMSStats retrieves cached SMS statistics
func (c *CacheHelper) GetSMSStats(ctx context.Context) (*SMSStats, error) {
	key := "stats:sms"
	var stats SMSStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetOverallStats caches overall statistics
func (c *CacheHelper) SetOverallStats(ctx context.Context, stats *OverallStats) error {
	key := "stats:overall"
	return c.setCache(ctx, key, stats, OverallStatsCacheTTL)
}

// GetOverallStats retrieves cached overall statistics
func (c *CacheHelper) GetOverallStats(ctx context.Context) (*OverallStats, error) {
	key := "stats:overall"
	var stats OverallStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetDetailedStats caches detailed statistics for a platform
func (c *CacheHelper) SetDetailedStats(ctx context.Context, platform string, stats *DetailedStats) error {
	key := fmt.Sprintf("stats:detailed:%s", platform)
	return c.setCache(ctx, key, stats, DetailedStatsCacheTTL)
}

// GetDetailedStats retrieves cached detailed statistics for a platform
func (c *CacheHelper) GetDetailedStats(ctx context.Context, platform string) (*DetailedStats, error) {
	key := fmt.Sprintf("stats:detailed:%s", platform)
	var stats DetailedStats
	if err := c.getCache(ctx, key, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// InvalidateAllStats clears all cached statistics
func (c *CacheHelper) InvalidateAllStats(ctx context.Context) error {
	pattern := "stats:*"
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for pattern %s: %w", pattern, err)
	}

	if len(keys) > 0 {
		_, err = c.client.Del(ctx, keys...).Result()
		if err != nil {
			return fmt.Errorf("failed to delete cached stats: %w", err)
		}
	}

	return nil
}

// Helper methods for generic cache operations
func (c *CacheHelper) setCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *CacheHelper) getCache(ctx context.Context, key string, dest interface{}) error {
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss")
		}
		return fmt.Errorf("failed to get cache data: %w", err)
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return nil
}
