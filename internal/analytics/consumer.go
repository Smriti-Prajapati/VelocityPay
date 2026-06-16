package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	"go.uber.org/zap"
)

const analyticsQueueName = "velocitypay.analytics"

// Consumer listens to transaction events and updates analytics counters.
type Consumer struct {
	consumer *rabbitmq.Consumer
	svc      *Service
	log      *zap.Logger
}

// NewConsumer sets up the analytics RabbitMQ consumer.
func NewConsumer(conn *rabbitmq.Connection, svc *Service, log *zap.Logger) (*Consumer, error) {
	bindingKeys := []string{
		rabbitmq.EventTransactionCompleted,
		rabbitmq.EventTransactionFailed,
		rabbitmq.EventUserRegistered,
		rabbitmq.EventWalletCreated,
	}

	c, err := rabbitmq.NewConsumer(conn, analyticsQueueName, bindingKeys, log)
	if err != nil {
		return nil, fmt.Errorf("create analytics consumer: %w", err)
	}

	return &Consumer{consumer: c, svc: svc, log: log}, nil
}

// Start begins consuming analytics events. Blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	return c.consumer.Consume(ctx, c.handle)
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) error {
	c.log.Debug("analytics event received", zap.String("routing_key", d.RoutingKey))

	switch d.RoutingKey {
	case rabbitmq.EventTransactionCompleted:
		return c.handleTransactionCompleted(d.Body)
	case rabbitmq.EventTransactionFailed:
		return c.handleTransactionFailed(d.Body)
	case rabbitmq.EventUserRegistered:
		return c.handleUserRegistered(d.Body)
	case rabbitmq.EventWalletCreated:
		return c.handleWalletCreated(d.Body)
	}
	return nil
}

type txnEvent struct {
	TransactionID string  `json:"transaction_id"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	Amount        float64 `json:"amount"`
}

func (c *Consumer) handleTransactionCompleted(body []byte) error {
	var evt txnEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal completed event: %w", err)
	}
	c.log.Info("analytics: transaction completed",
		zap.String("txn_id", evt.TransactionID),
		zap.Float64("amount", evt.Amount),
	)
	return nil
}

func (c *Consumer) handleTransactionFailed(body []byte) error {
	var evt txnEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal failed event: %w", err)
	}
	c.log.Info("analytics: transaction failed",
		zap.String("txn_id", evt.TransactionID),
	)
	return nil
}

func (c *Consumer) handleUserRegistered(body []byte) error {
	var evt struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal user event: %w", err)
	}
	c.log.Info("analytics: user registered", zap.String("user_id", evt.UserID))
	return nil
}

func (c *Consumer) handleWalletCreated(body []byte) error {
	var evt struct {
		WalletID string `json:"wallet_id"`
		UserID   string `json:"user_id"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal wallet event: %w", err)
	}
	c.log.Info("analytics: wallet created",
		zap.String("wallet_id", evt.WalletID),
		zap.String("currency", evt.Currency),
	)
	return nil
}
