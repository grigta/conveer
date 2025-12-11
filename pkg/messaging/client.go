package messaging

import (
	"context"

	"github.com/streadway/amqp"
)

// Client is the interface for messaging operations used by services
type Client interface {
	DeclareExchange(name, kind string, durable, autoDelete bool) error
	DeclareQueue(name string, durable, autoDelete, exclusive bool) (amqp.Queue, error)
	BindQueue(queueName, routingKey, exchangeName string) error
	PublishToQueue(queueName string, message interface{}) error
	PublishEvent(exchange, routingKey string, message interface{}) error
	ConsumeQueue(ctx context.Context, queueName string, handler func([]byte) error) error
	Close() error
}

// client wraps RabbitMQ to implement the Client interface
type client struct {
	rabbit *RabbitMQ
}

// NewClient creates a new messaging client
func NewClient(url string) (Client, error) {
	rabbit, err := NewRabbitMQ(url)
	if err != nil {
		return nil, err
	}

	return &client{
		rabbit: rabbit,
	}, nil
}

func (c *client) DeclareExchange(name, kind string, durable, autoDelete bool) error {
	return c.rabbit.DeclareExchange(name, kind, durable, autoDelete)
}

func (c *client) DeclareQueue(name string, durable, autoDelete, exclusive bool) (amqp.Queue, error) {
	return c.rabbit.DeclareQueue(name, durable, autoDelete, exclusive)
}

func (c *client) BindQueue(queueName, routingKey, exchangeName string) error {
	return c.rabbit.BindQueue(queueName, routingKey, exchangeName)
}

func (c *client) PublishToQueue(queueName string, message interface{}) error {
	// Publish directly to a queue (empty exchange)
	return c.rabbit.Publish("", queueName, message)
}

func (c *client) PublishEvent(exchange, routingKey string, message interface{}) error {
	return c.rabbit.Publish(exchange, routingKey, message)
}

func (c *client) ConsumeQueue(ctx context.Context, queueName string, handler func([]byte) error) error {
	// Use a default consumer name based on queue name
	consumerName := "consumer-" + queueName
	return c.rabbit.ConsumeWithHandler(ctx, queueName, consumerName, handler)
}

func (c *client) Close() error {
	return c.rabbit.Close()
}