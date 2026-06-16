package rabbitmq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// HandlerFunc processes a single delivery. Returning an error triggers a nack.
type HandlerFunc func(ctx context.Context, delivery amqp.Delivery) error

// Consumer subscribes to a queue bound to the events exchange.
type Consumer struct {
	conn      *Connection
	queueName string
	log       *zap.Logger
}

// NewConsumer creates a Consumer and declares the queue + bindings.
func NewConsumer(conn *Connection, queueName string, bindingKeys []string, log *zap.Logger) (*Consumer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	// Ensure the exchange exists (idempotent).
	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	q, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("declare queue %q: %w", queueName, err)
	}

	for _, key := range bindingKeys {
		if err := ch.QueueBind(q.Name, key, ExchangeEvents, false, nil); err != nil {
			return nil, fmt.Errorf("bind queue to %q: %w", key, err)
		}
	}

	return &Consumer{conn: conn, queueName: queueName, log: log}, nil
}

// Consume starts consuming messages and dispatches each to handler.
// It blocks until ctx is cancelled.
func (c *Consumer) Consume(ctx context.Context, handler HandlerFunc) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	if err := ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(c.queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consume: %w", err)
	}

	c.log.Info("consumer started", zap.String("queue", c.queueName))

	for {
		select {
		case <-ctx.Done():
			c.log.Info("consumer stopping", zap.String("queue", c.queueName))
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("deliveries channel closed for queue %q", c.queueName)
			}
			if err := handler(ctx, d); err != nil {
				c.log.Error("handler error, nacking",
					zap.String("queue", c.queueName),
					zap.Error(err),
				)
				_ = d.Nack(false, true) // requeue
			} else {
				_ = d.Ack(false)
			}
		}
	}
}
