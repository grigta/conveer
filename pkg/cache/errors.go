package cache

import "errors"

var (
	ErrCacheMiss      = errors.New("cache miss")
	ErrCacheExpired   = errors.New("cache expired")
	ErrInvalidKey     = errors.New("invalid cache key")
	ErrInvalidValue   = errors.New("invalid cache value")
	ErrConnectionLost = errors.New("cache connection lost")
)