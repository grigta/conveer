package service

import (
	"context"
	"fmt"
	"time"

	"github.com/conveer/telegram-bot/internal/models"
	"github.com/conveer/pkg/messaging"
)

type CommandService interface {
	StartRegistration(ctx context.Context, platform string, count int) error
	StartWarming(ctx context.Context, accountID, platform, scenario string, duration int) error
	PauseWarming(ctx context.Context, taskID string) error
	ResumeWarming(ctx context.Context, taskID string) error
	StopWarming(ctx context.Context, taskID string) error
	AllocateProxy(ctx context.Context, accountID, proxyType string) error
	ReleaseProxy(ctx context.Context, accountID string) error
	PurchaseNumber(ctx context.Context, service, country string) error
	CancelActivation(ctx context.Context, activationID string) error
}

type commandService struct {
	rabbitmq *messaging.RabbitMQ
}

func NewCommandService(rabbitmq *messaging.RabbitMQ) CommandService {
	return &commandService{
		rabbitmq: rabbitmq,
	}
}

func (s *commandService) StartRegistration(ctx context.Context, platform string, count int) error {
	cmd := models.RegisterCommand{
		Count:       count,
		InitiatedBy: "telegram_bot",
	}

	exchange := fmt.Sprintf("%s.commands", platform)
	routingKey := fmt.Sprintf("%s.register", platform)

	if err := s.rabbitmq.Publish(exchange, routingKey, cmd); err != nil {
		return fmt.Errorf("failed to publish registration command: %w", err)
	}

	return nil
}

func (s *commandService) StartWarming(ctx context.Context, accountID, platform, scenario string, duration int) error {
	cmd := models.WarmingCommand{
		AccountID:    accountID,
		Platform:     platform,
		Scenario:     scenario,
		DurationDays: duration,
		InitiatedBy:  "telegram_bot",
	}

	exchange := "warming.commands"
	routingKey := "start"

	if err := s.rabbitmq.Publish(exchange, routingKey, cmd); err != nil {
		return fmt.Errorf("failed to publish warming command: %w", err)
	}

	return nil
}

func (s *commandService) PauseWarming(ctx context.Context, taskID string) error {
	cmd := models.Command{
		Command:     "pause",
		Params:      map[string]interface{}{"task_id": taskID},
		InitiatedBy: "telegram_bot",
		Timestamp:   time.Now(),
	}

	if err := s.rabbitmq.Publish("warming.commands", "pause", cmd); err != nil {
		return fmt.Errorf("failed to publish pause command: %w", err)
	}

	return nil
}

func (s *commandService) ResumeWarming(ctx context.Context, taskID string) error {
	cmd := models.Command{
		Command:     "resume",
		Params:      map[string]interface{}{"task_id": taskID},
		InitiatedBy: "telegram_bot",
		Timestamp:   time.Now(),
	}

	if err := s.rabbitmq.Publish("warming.commands", "resume", cmd); err != nil {
		return fmt.Errorf("failed to publish resume command: %w", err)
	}

	return nil
}

func (s *commandService) StopWarming(ctx context.Context, taskID string) error {
	cmd := models.Command{
		Command:     "stop",
		Params:      map[string]interface{}{"task_id": taskID},
		InitiatedBy: "telegram_bot",
		Timestamp:   time.Now(),
	}

	if err := s.rabbitmq.Publish("warming.commands", "stop", cmd); err != nil {
		return fmt.Errorf("failed to publish stop command: %w", err)
	}

	return nil
}

func (s *commandService) AllocateProxy(ctx context.Context, accountID, proxyType string) error {
	cmd := models.ProxyCommand{
		AccountID:   accountID,
		Type:        proxyType,
		Action:      "allocate",
		InitiatedBy: "telegram_bot",
	}

	if err := s.rabbitmq.Publish("proxy.commands", "allocate", cmd); err != nil {
		return fmt.Errorf("failed to publish proxy allocate command: %w", err)
	}

	return nil
}

func (s *commandService) ReleaseProxy(ctx context.Context, accountID string) error {
	cmd := models.ProxyCommand{
		AccountID:   accountID,
		Action:      "release",
		InitiatedBy: "telegram_bot",
	}

	if err := s.rabbitmq.Publish("proxy.commands", "release", cmd); err != nil {
		return fmt.Errorf("failed to publish proxy release command: %w", err)
	}

	return nil
}

func (s *commandService) PurchaseNumber(ctx context.Context, service, country string) error {
	cmd := models.SMSCommand{
		Service:     service,
		Country:     country,
		Action:      "purchase",
		InitiatedBy: "telegram_bot",
	}

	if err := s.rabbitmq.Publish("sms.commands", "purchase", cmd); err != nil {
		return fmt.Errorf("failed to publish SMS purchase command: %w", err)
	}

	return nil
}

func (s *commandService) CancelActivation(ctx context.Context, activationID string) error {
	cmd := models.SMSCommand{
		ActivationID: activationID,
		Action:       "cancel",
		InitiatedBy:  "telegram_bot",
	}

	if err := s.rabbitmq.Publish("sms.commands", "cancel", cmd); err != nil {
		return fmt.Errorf("failed to publish SMS cancel command: %w", err)
	}

	return nil
}