package notification

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	"go.uber.org/zap"
)

const queueName = "velocitypay.notifications"

// Consumer listens to RabbitMQ events and creates notifications.
type Consumer struct {
	consumer *rabbitmq.Consumer
	svc      *Service
	log      *zap.Logger
}

// NewConsumer sets up the RabbitMQ consumer with all relevant binding keys.
func NewConsumer(conn *rabbitmq.Connection, svc *Service, log *zap.Logger) (*Consumer, error) {
	bindingKeys := []string{
		rabbitmq.EventTransactionCompleted,
		rabbitmq.EventTransactionFailed,
		rabbitmq.EventRefundCreated,
		rabbitmq.EventRefundCompleted,
		rabbitmq.EventWalletCreated,
		rabbitmq.EventUserRegistered,
	}

	c, err := rabbitmq.NewConsumer(conn, queueName, bindingKeys, log)
	if err != nil {
		return nil, fmt.Errorf("create notification consumer: %w", err)
	}

	return &Consumer{consumer: c, svc: svc, log: log}, nil
}

// Start begins consuming events. Blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	return c.consumer.Consume(ctx, c.handle)
}

// handle routes each delivery to the appropriate handler by routing key.
func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) error {
	c.log.Debug("notification consumer received event",
		zap.String("routing_key", d.RoutingKey),
	)

	switch d.RoutingKey {
	case rabbitmq.EventTransactionCompleted:
		return c.handleTransactionCompleted(ctx, d.Body)
	case rabbitmq.EventTransactionFailed:
		return c.handleTransactionFailed(ctx, d.Body)
	case rabbitmq.EventRefundCreated:
		return c.handleRefundCreated(ctx, d.Body)
	case rabbitmq.EventRefundCompleted:
		return c.handleRefundCompleted(ctx, d.Body)
	case rabbitmq.EventWalletCreated:
		return c.handleWalletCreated(ctx, d.Body)
	case rabbitmq.EventUserRegistered:
		return c.handleUserRegistered(ctx, d.Body)
	default:
		c.log.Warn("unhandled routing key", zap.String("key", d.RoutingKey))
		return nil
	}
}

func (c *Consumer) handleTransactionCompleted(ctx context.Context, body []byte) error {
	var evt TransactionEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal transaction event: %w", err)
	}

	senderID, err := uuid.Parse(evt.SenderID)
	if err != nil {
		return fmt.Errorf("parse sender_id: %w", err)
	}
	receiverID, err := uuid.Parse(evt.ReceiverID)
	if err != nil {
		return fmt.Errorf("parse receiver_id: %w", err)
	}

	// Notify sender
	if err := c.svc.Send(ctx, senderID, TypeTransactionSent,
		"Transfer Successful",
		fmt.Sprintf("You sent ₹%.2f successfully. Transaction ID: %s", evt.Amount, evt.TransactionID),
		evt.TransactionID,
	); err != nil {
		c.log.Error("notify sender failed", zap.Error(err))
	}

	// Notify receiver
	if err := c.svc.Send(ctx, receiverID, TypeTransactionReceived,
		"Money Received",
		fmt.Sprintf("You received ₹%.2f. Transaction ID: %s", evt.Amount, evt.TransactionID),
		evt.TransactionID,
	); err != nil {
		c.log.Error("notify receiver failed", zap.Error(err))
	}

	return nil
}

func (c *Consumer) handleTransactionFailed(ctx context.Context, body []byte) error {
	var evt TransactionEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal transaction failed event: %w", err)
	}

	senderID, err := uuid.Parse(evt.SenderID)
	if err != nil {
		return fmt.Errorf("parse sender_id: %w", err)
	}

	return c.svc.Send(ctx, senderID, TypeTransactionFailed,
		"Transfer Failed",
		fmt.Sprintf("Your transfer of ₹%.2f failed. Reason: %s", evt.Amount, evt.Reason),
		evt.TransactionID,
	)
}

func (c *Consumer) handleRefundCreated(ctx context.Context, body []byte) error {
	var evt RefundEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal refund created event: %w", err)
	}

	requesterID, err := uuid.Parse(evt.RequestedBy)
	if err != nil {
		return fmt.Errorf("parse requested_by: %w", err)
	}

	return c.svc.Send(ctx, requesterID, TypeRefundRequested,
		"Refund Requested",
		fmt.Sprintf("Your refund request for ₹%.2f has been submitted and is under review.", evt.Amount),
		evt.RefundID,
	)
}

func (c *Consumer) handleRefundCompleted(ctx context.Context, body []byte) error {
	var evt RefundEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal refund completed event: %w", err)
	}

	senderID, err := uuid.Parse(evt.SenderID)
	if err != nil {
		return fmt.Errorf("parse sender_id: %w", err)
	}

	return c.svc.Send(ctx, senderID, TypeRefundCompleted,
		"Refund Completed",
		fmt.Sprintf("Your refund of ₹%.2f has been processed and credited to your wallet.", evt.Amount),
		evt.RefundID,
	)
}

func (c *Consumer) handleWalletCreated(ctx context.Context, body []byte) error {
	var evt WalletEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal wallet event: %w", err)
	}

	userID, err := uuid.Parse(evt.UserID)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}

	return c.svc.Send(ctx, userID, TypeWalletCreated,
		"Wallet Created",
		fmt.Sprintf("Your %s wallet has been created successfully. Wallet ID: %s", evt.Currency, evt.WalletID),
		evt.WalletID,
	)
}

func (c *Consumer) handleUserRegistered(ctx context.Context, body []byte) error {
	var evt UserEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal user event: %w", err)
	}

	userID, err := uuid.Parse(evt.UserID)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}

	return c.svc.Send(ctx, userID, TypeUserWelcome,
		"Welcome to VelocityPay!",
		fmt.Sprintf("Hi %s! Your account is ready. Create a wallet to start sending and receiving money.", evt.Name),
		evt.UserID,
	)
}
