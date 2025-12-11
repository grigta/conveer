package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/streadway/amqp"
	"github.com/grigta/conveer/pkg/logger"
)

type RabbitMQ struct {
	conn      *amqp.Connection
	channel   *amqp.Channel
	url       string
	consumers []ConsumerRegistration
	stopCh    chan struct{}
}

type ConsumerRegistration struct {
	QueueName    string
	ConsumerName string
	Handler      func([]byte) error
	Context      context.Context
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	logger.Info("Connected to RabbitMQ")

	rabbitmq := &RabbitMQ{
		conn:      conn,
		channel:   ch,
		url:       url,
		consumers: make([]ConsumerRegistration, 0),
		stopCh:    make(chan struct{}),
	}

	// Start connection monitor
	go rabbitmq.monitorConnection()

	return rabbitmq, nil
}

func (r *RabbitMQ) Close() error {
	// Stop monitoring
	close(r.stopCh)

	if err := r.channel.Close(); err != nil {
		return fmt.Errorf("failed to close channel: %w", err)
	}
	if err := r.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	return nil
}

func (r *RabbitMQ) DeclareExchange(name, kind string, durable, autoDelete bool) error {
	return r.channel.ExchangeDeclare(
		name,
		kind,
		durable,
		autoDelete,
		false,
		false,
		nil,
	)
}

func (r *RabbitMQ) DeclareQueue(name string, durable, autoDelete, exclusive bool) (amqp.Queue, error) {
	return r.channel.QueueDeclare(
		name,
		durable,
		autoDelete,
		exclusive,
		false,
		nil,
	)
}

func (r *RabbitMQ) BindQueue(queueName, routingKey, exchangeName string) error {
	return r.channel.QueueBind(
		queueName,
		routingKey,
		exchangeName,
		false,
		nil,
	)
}

func (r *RabbitMQ) Publish(exchange, routingKey string, message interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return r.channel.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)
}

func (r *RabbitMQ) PublishWithHeaders(exchange, routingKey string, message interface{}, headers map[string]interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return r.channel.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			Headers:     amqp.Table(headers),
			ContentType: "application/json",
			Body:        body,
			Timestamp:   time.Now(),
		},
	)
}

func (r *RabbitMQ) Consume(queueName, consumerName string, autoAck bool) (<-chan amqp.Delivery, error) {
	return r.channel.Consume(
		queueName,
		consumerName,
		autoAck,
		false,
		false,
		false,
		nil,
	)
}

func (r *RabbitMQ) ConsumeWithHandler(ctx context.Context, queueName, consumerName string, handler func([]byte) error) error {
	// Register consumer for auto-recovery
	r.consumers = append(r.consumers, ConsumerRegistration{
		QueueName:    queueName,
		ConsumerName: consumerName,
		Handler:      handler,
		Context:      ctx,
	})

	// Start consuming
	return r.startConsumer(ctx, queueName, consumerName, handler)
}

func (r *RabbitMQ) startConsumer(ctx context.Context, queueName, consumerName string, handler func([]byte) error) error {
	msgs, err := r.Consume(queueName, consumerName, false)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping consumer", logger.Field{Key: "queue", Value: queueName})
				return
			case msg, ok := <-msgs:
				if !ok {
					logger.Warn("Consumer channel closed", logger.Field{Key: "queue", Value: queueName})
					return
				}

				if err := handler(msg.Body); err != nil {
					logger.Error("Failed to process message",
						logger.Field{Key: "queue", Value: queueName},
						logger.Field{Key: "error", Value: err.Error()},
					)
					msg.Nack(false, true)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	logger.Info("Started consuming messages", logger.Field{Key: "queue", Value: queueName})
	return nil
}

func (r *RabbitMQ) SetQos(prefetchCount int) error {
	return r.channel.Qos(prefetchCount, 0, false)
}

func (r *RabbitMQ) Reconnect() error {
	if r.conn != nil && !r.conn.IsClosed() {
		r.conn.Close()
	}

	conn, err := amqp.Dial(r.url)
	if err != nil {
		return fmt.Errorf("failed to reconnect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to reopen channel: %w", err)
	}

	r.conn = conn
	r.channel = ch

	logger.Info("Reconnected to RabbitMQ")

	// Re-setup topology
	if err := r.SetupTopology(); err != nil {
		logger.Error("Failed to setup topology after reconnect", logger.Field{Key: "error", Value: err.Error()})
	}

	// Restart all registered consumers
	for _, consumer := range r.consumers {
		if err := r.startConsumer(consumer.Context, consumer.QueueName, consumer.ConsumerName, consumer.Handler); err != nil {
			logger.Error("Failed to restart consumer after reconnect",
				logger.Field{Key: "queue", Value: consumer.QueueName},
				logger.Field{Key: "error", Value: err.Error()},
			)
		} else {
			logger.Info("Restarted consumer after reconnect",
				logger.Field{Key: "queue", Value: consumer.QueueName},
			)
		}
	}

	return nil
}

func (r *RabbitMQ) monitorConnection() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			if r.conn != nil && r.conn.IsClosed() {
				logger.Warn("RabbitMQ connection lost, attempting to reconnect...")
				for i := 0; i < 5; i++ {
					if err := r.Reconnect(); err != nil {
						logger.Error("Failed to reconnect to RabbitMQ",
							logger.Field{Key: "attempt", Value: i + 1},
							logger.Field{Key: "error", Value: err.Error()},
						)
						time.Sleep(time.Duration(i+1) * time.Second)
					} else {
						break
					}
				}
			}
		}
	}
}

type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func NewMessage(msgType string, data interface{}) *Message {
	return &Message{
		ID:        generateMessageID(),
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
		Metadata:  make(map[string]interface{}),
	}
}

func generateMessageID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

type Publisher interface {
	Publish(exchange, routingKey string, message interface{}) error
}

type Consumer interface {
	Consume(queueName, consumerName string, autoAck bool) (<-chan amqp.Delivery, error)
}

type MessageBroker interface {
	Publisher
	Consumer
	Close() error
}

func (r *RabbitMQ) PublishEvent(eventType string, data interface{}) error {
	message := NewMessage(eventType, data)
	return r.Publish("events", eventType, message)
}

func (r *RabbitMQ) PublishCommand(commandType string, data interface{}) error {
	message := NewMessage(commandType, data)
	return r.Publish("commands", commandType, message)
}

func (r *RabbitMQ) SetupTopology() error {
	if err := r.DeclareExchange("events", "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare events exchange: %w", err)
	}

	if err := r.DeclareExchange("commands", "direct", true, false); err != nil {
		return fmt.Errorf("failed to declare commands exchange: %w", err)
	}

	if err := r.DeclareExchange("dead-letter", "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare dead-letter exchange: %w", err)
	}

	// Setup SMS-specific topology
	if err := r.SetupSMSTopology(); err != nil {
		return fmt.Errorf("failed to setup SMS topology: %w", err)
	}

	return nil
}

func (r *RabbitMQ) SetupSMSTopology() error {
	// Declare SMS exchanges
	if err := r.DeclareExchange("sms.events", "topic", true, false); err != nil {
		return fmt.Errorf("failed to declare sms.events exchange: %w", err)
	}

	if err := r.DeclareExchange("sms.commands", "direct", true, false); err != nil {
		return fmt.Errorf("failed to declare sms.commands exchange: %w", err)
	}

	// Declare SMS queues
	smsQueues := []string{"sms.purchase", "sms.get_code", "sms.cancel", "sms.retry"}
	for _, queueName := range smsQueues {
		if _, err := r.DeclareQueue(queueName, true, false, false); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", queueName, err)
		}
	}

	// Bind queues to exchanges
	bindings := []struct {
		queue    string
		exchange string
		key      string
	}{
		{"sms.purchase", "sms.commands", "purchase"},
		{"sms.get_code", "sms.commands", "get_code"},
		{"sms.cancel", "sms.commands", "cancel"},
		{"sms.retry", "sms.commands", "retry"},
	}

	for _, binding := range bindings {
		if err := r.BindQueue(binding.queue, binding.key, binding.exchange); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w",
				binding.queue, binding.exchange, err)
		}
	}

	logger.Info("SMS topology setup completed")
	return nil
}

func (r *RabbitMQ) CreateDLQ(queueName string) error {
	dlqName := fmt.Sprintf("%s.dlq", queueName)

	_, err := r.channel.QueueDeclare(
		dlqName,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-message-ttl": int32(86400000),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	return r.BindQueue(dlqName, queueName, "dead-letter")
}
