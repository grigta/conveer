package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/grigta/conveer/services/sms-service/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

type RetryManager struct {
	channel *amqp.Channel
	logger  *logrus.Logger
}

func NewRetryManager(channel *amqp.Channel, logger *logrus.Logger) *RetryManager {
	return &RetryManager{
		channel: channel,
		logger:  logger,
	}
}

func (r *RetryManager) ScheduleRetry(ctx context.Context, activation *models.Activation, delay time.Duration) error {
	data, err := json.Marshal(activation)
	if err != nil {
		return err
	}

	// Publish to retry queue with delay
	return r.channel.Publish(
		"sms.commands",
		"retry",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         data,
			DeliveryMode: amqp.Persistent,
			Expiration:   delay.String(),
		},
	)
}

func (r *RetryManager) StartWorker(ctx context.Context, smsService *SMSService) {
	msgs, err := r.channel.Consume(
		"sms.retry",
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		r.logger.Errorf("Failed to start retry worker: %v", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgs:
			var activation models.Activation
			if err := json.Unmarshal(msg.Body, &activation); err != nil {
				r.logger.Errorf("Failed to unmarshal retry message: %v", err)
				continue
			}

			// Process retry
			_, _, err := smsService.GetSMSCode(ctx, activation.ActivationID, activation.UserID)
			if err != nil {
				r.logger.Debugf("Retry failed for activation %s: %v", activation.ActivationID, err)
			}
		}
	}
}
