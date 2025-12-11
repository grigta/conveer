package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"conveer/sms-service/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type CacheService struct {
	client *redis.Client
	logger *logrus.Logger
}

func NewCacheService(client *redis.Client, logger *logrus.Logger) *CacheService {
	return &CacheService{
		client: client,
		logger: logger,
	}
}

func (s *CacheService) SetActivation(ctx context.Context, activationID string, activation *models.Activation, ttl time.Duration) error {
	data, err := json.Marshal(activation)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("activation:%s", activationID)
	return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *CacheService) GetActivation(ctx context.Context, activationID string) (*models.Activation, error) {
	key := fmt.Sprintf("activation:%s", activationID)
	data, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var activation models.Activation
	if err := json.Unmarshal([]byte(data), &activation); err != nil {
		return nil, err
	}

	return &activation, nil
}

func (s *CacheService) DeleteActivation(ctx context.Context, activationID string) error {
	key := fmt.Sprintf("activation:%s", activationID)
	return s.client.Del(ctx, key).Err()
}

func (s *CacheService) SetProviderBalance(ctx context.Context, provider string, balance float64, currency string, ttl time.Duration) error {
	key := fmt.Sprintf("provider:balance:%s", provider)
	value := fmt.Sprintf("%.2f:%s", balance, currency)
	return s.client.Set(ctx, key, value, ttl).Err()
}

func (s *CacheService) GetProviderBalance(ctx context.Context, provider string) (float64, string, error) {
	key := fmt.Sprintf("provider:balance:%s", provider)
	value, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return 0, "", err
	}

	var balance float64
	var currency string
	_, err = fmt.Sscanf(value, "%f:%s", &balance, &currency)
	if err != nil {
		return 0, "", err
	}

	return balance, currency, nil
}