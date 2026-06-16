package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

// Exchange names used across VelocityPay.
const (
	ExchangeEvents = "velocitypay.events"
)

// Event routing keys.
const (
	EventUserRegistered      = "user.registered"
	EventWalletCreated       = "wallet.created"
	EventTransactionCreated  = "transaction.created"
	EventTransactionCompleted = "transaction.completed"
	EventTransactionFailed   = "transaction.failed"
	EventRefundCreated       = "refund.created"
	EventRefundCompleted     = "refund.completed"
)

// Publisher publishes JSON-encoded events to RabbitMQ.
type Publisher struct {
	conn *Connection
	log  *zap.Logger
}

// NewPublisher creates a Publisher backed by the given connection.
func NewPublisher(conn *Connection, log *zap.Logger) (*Publisher, error) {
	p := &Publisher{conn: conn, log: log}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel for publisher: %w", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(
		ExchangeEvents,
		"topic",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return nil, fmt.Errorf("declare exchange: %w", err)
	}

	return p, nil
}

// Publish marshals payload to JSON and publishes it with the given routing key.
func (p *Publisher) Publish(ctx context.Context, routingKey string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	ch, err := p.conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	}

	if err := ch.PublishWithContext(ctx, ExchangeEvents, routingKey, false, false, msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	p.log.Debug("event published",
		zap.String("routing_key", routingKey),
		zap.Int("body_bytes", len(body)),
	)
	return nil
}
