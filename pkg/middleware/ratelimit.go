package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	requests map[string]*bucket
	mu       sync.RWMutex
	rate     int
	window   time.Duration
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*bucket),
		rate:     rate,
		window:   window,
	}

	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.getKey(c)

		if !rl.allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": rl.window.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) getKey(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if exists {
		return fmt.Sprintf("user:%v", userID)
	}
	return fmt.Sprintf("ip:%s", c.ClientIP())
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.requests[key]
	now := time.Now()

	if !exists || now.Sub(b.lastReset) > rl.window {
		rl.requests[key] = &bucket{
			tokens:    rl.rate - 1,
			lastReset: now,
		}
		return true
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.requests {
			if now.Sub(b.lastReset) > rl.window*2 {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

type TokenBucketLimiter struct {
	capacity  int
	tokens    int
	refillRate time.Duration
	mu        sync.Mutex
	lastRefill time.Time
}

func NewTokenBucketLimiter(capacity int, refillRate time.Duration) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tbl *TokenBucketLimiter) Allow() bool {
	tbl.mu.Lock()
	defer tbl.mu.Unlock()

	tbl.refill()

	if tbl.tokens > 0 {
		tbl.tokens--
		return true
	}

	return false
}

func (tbl *TokenBucketLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(tbl.lastRefill)
	tokensToAdd := int(elapsed / tbl.refillRate)

	if tokensToAdd > 0 {
		tbl.tokens = min(tbl.tokens+tokensToAdd, tbl.capacity)
		tbl.lastRefill = now
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type IPRateLimiter struct {
	limiters map[string]*TokenBucketLimiter
	mu       sync.RWMutex
	capacity int
	refillRate time.Duration
}

func NewIPRateLimiter(capacity int, refillRate time.Duration) *IPRateLimiter {
	irl := &IPRateLimiter{
		limiters:   make(map[string]*TokenBucketLimiter),
		capacity:   capacity,
		refillRate: refillRate,
	}

	go irl.cleanup()

	return irl
}

func (irl *IPRateLimiter) GetLimiter(ip string) *TokenBucketLimiter {
	irl.mu.RLock()
	limiter, exists := irl.limiters[ip]
	irl.mu.RUnlock()

	if exists {
		return limiter
	}

	irl.mu.Lock()
	defer irl.mu.Unlock()

	limiter, exists = irl.limiters[ip]
	if !exists {
		limiter = NewTokenBucketLimiter(irl.capacity, irl.refillRate)
		irl.limiters[ip] = limiter
	}

	return limiter
}

func (irl *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		irl.mu.Lock()
		for ip, limiter := range irl.limiters {
			limiter.mu.Lock()
			if time.Since(limiter.lastRefill) > time.Hour {
				delete(irl.limiters, ip)
			}
			limiter.mu.Unlock()
		}
		irl.mu.Unlock()
	}
}

func (irl *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := irl.GetLimiter(ip)

		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": irl.refillRate.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}