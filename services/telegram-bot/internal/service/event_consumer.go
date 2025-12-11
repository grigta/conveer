package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/grigta/conveer/services/telegram-bot/internal/models"
	"github.com/grigta/conveer/pkg/messaging"
)

type EventConsumer interface {
	Start(ctx context.Context) error
	Stop() error
}

type eventConsumer struct {
	rabbitmq   *messaging.RabbitMQ
	botService BotService
	authService AuthService
}

func NewEventConsumer(rabbitmq *messaging.RabbitMQ, botService BotService, authService AuthService) EventConsumer {
	return &eventConsumer{
		rabbitmq:   rabbitmq,
		botService: botService,
		authService: authService,
	}
}

func (c *eventConsumer) Start(ctx context.Context) error {
	// Setup bot topology
	if err := c.setupTopology(); err != nil {
		return fmt.Errorf("failed to setup topology: %w", err)
	}

	// Start consuming events
	go c.consumeEvents(ctx)

	return nil
}

func (c *eventConsumer) setupTopology() error {
	// Declare bot.events exchange
	if err := c.rabbitmq.DeclareExchange("bot.events", "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare bot.events exchange: %w", err)
	}

	// Declare bot.alerts queue
	if _, err := c.rabbitmq.DeclareQueue("bot.alerts", true, false, false); err != nil {
		return fmt.Errorf("failed to declare bot.alerts queue: %w", err)
	}

	// Bind queue to exchange with routing keys
	routingKeys := []string{
		"*.manual_intervention",
		"*.account.banned",
		"*.task.failed",
		"*.health_failed",
		"sms.balance.low",
		"proxy.rotation.failed",
		"analytics.alert.*",
		"analytics.manual_intervention",
	}

	for _, key := range routingKeys {
		if err := c.rabbitmq.BindQueue("bot.alerts", key, "bot.events"); err != nil {
			return fmt.Errorf("failed to bind queue with routing key %s: %w", key, err)
		}
	}

	// Also bind to platform-specific exchanges for events
	platforms := []string{"vk", "telegram", "mail", "max", "warming", "proxy", "sms"}
	for _, platform := range platforms {
		exchange := fmt.Sprintf("%s.events", platform)

		// Try to declare exchange (might already exist)
		c.rabbitmq.DeclareExchange(exchange, "topic", true, false)

		// Bind our queue to each platform's events
		for _, key := range routingKeys {
			if err := c.rabbitmq.BindQueue("bot.alerts", key, exchange); err != nil {
				log.Printf("Warning: failed to bind to %s exchange with key %s: %v", exchange, key, err)
			}
		}
	}

	return nil
}

func (c *eventConsumer) consumeEvents(ctx context.Context) {
	handler := func(message []byte) error {
		var event models.Event
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("Failed to unmarshal event: %v", err)
			return nil // Don't requeue malformed messages
		}

		// Determine priority
		priority := c.determinePriority(event.Type)
		event.Priority = priority

		// Format alert message
		alertMessage := c.formatAlert(&event)

		// Get admin users
		admins, err := c.authService.ListUsers(ctx, map[string]interface{}{
			"role": models.RoleAdmin,
			"is_active": true,
		})
		if err != nil {
			log.Printf("Failed to get admin users: %v", err)
			return nil
		}

		// Send alert to all admins
		for _, admin := range admins {
			if err := c.botService.SendAlert(ctx, admin.TelegramID, alertMessage); err != nil {
				log.Printf("Failed to send alert to admin %d: %v", admin.TelegramID, err)
			}
		}

		// For critical alerts, also send to operators
		if priority == "critical" {
			operators, err := c.authService.ListUsers(ctx, map[string]interface{}{
				"role": models.RoleOperator,
				"is_active": true,
			})
			if err == nil {
				for _, operator := range operators {
					c.botService.SendAlert(ctx, operator.TelegramID, alertMessage)
				}
			}
		}

		return nil
	}

	if err := c.rabbitmq.ConsumeWithHandler(ctx, "bot.alerts", "telegram-bot-alerts", handler); err != nil {
		log.Printf("Error consuming events: %v", err)
	}
}

func (c *eventConsumer) determinePriority(eventType string) string {
	// Check for analytics alerts first
	if strings.Contains(eventType, "analytics.alert.") {
		parts := strings.Split(eventType, ".")
		if len(parts) >= 3 {
			severity := parts[2]
			if severity == "critical" || severity == "high" {
				return "critical"
			}
			if severity == "warning" || severity == "medium" {
				return "warning"
			}
		}
	}

	if strings.Contains(eventType, "banned") ||
	   strings.Contains(eventType, "failed") ||
	   strings.Contains(eventType, "balance.low") {
		return "critical"
	}

	if strings.Contains(eventType, "manual_intervention") ||
	   strings.Contains(eventType, "health_failed") {
		return "warning"
	}

	return "info"
}

func (c *eventConsumer) formatAlert(event *models.Event) string {
	var emoji string
	switch event.Priority {
	case "critical":
		emoji = "🚨"
	case "warning":
		emoji = "⚠️"
	default:
		emoji = "ℹ️"
	}

	message := fmt.Sprintf("%s [%s] %s\n", emoji, strings.ToUpper(event.Priority), event.Type)

	if event.Platform != "" {
		message += fmt.Sprintf("Platform: %s\n", event.Platform)
	}

	if event.AccountID != "" {
		message += fmt.Sprintf("Account: %s\n", event.AccountID)
	}

	if event.TaskID != "" {
		message += fmt.Sprintf("Task: %s\n", event.TaskID)
	}

	if event.Message != "" {
		message += fmt.Sprintf("Message: %s\n", event.Message)
	}

	if event.Error != "" {
		message += fmt.Sprintf("Error: %s\n", event.Error)
	}

	return message
}

func (c *eventConsumer) Stop() error {
	// Stop consuming
	// This would need to be implemented in the RabbitMQ wrapper
	return nil
}
