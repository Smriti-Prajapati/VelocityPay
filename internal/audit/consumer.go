package audit

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	"go.uber.org/zap"
)

const auditQueueName = "velocitypay.audit"

// Consumer listens to all platform events and writes audit logs.
type Consumer struct {
	consumer *rabbitmq.Consumer
	svc      *Service
	log      *zap.Logger
}

// NewConsumer creates the audit event consumer bound to all event types.
func NewConsumer(conn *rabbitmq.Connection, svc *Service, log *zap.Logger) (*Consumer, error) {
	bindingKeys := []string{
		rabbitmq.EventUserRegistered,
		rabbitmq.EventWalletCreated,
		rabbitmq.EventTransactionCreated,
		rabbitmq.EventTransactionCompleted,
		rabbitmq.EventTransactionFailed,
		rabbitmq.EventRefundCreated,
		rabbitmq.EventRefundCompleted,
	}

	c, err := rabbitmq.NewConsumer(conn, auditQueueName, bindingKeys, log)
	if err != nil {
		return nil, fmt.Errorf("create audit consumer: %w", err)
	}

	return &Consumer{consumer: c, svc: svc, log: log}, nil
}

// Start begins consuming. Blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	return c.consumer.Consume(ctx, c.handle)
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) error {
	switch d.RoutingKey {
	case rabbitmq.EventUserRegistered:
		return c.auditUserRegistered(ctx, d.Body)
	case rabbitmq.EventWalletCreated:
		return c.auditWalletCreated(ctx, d.Body)
	case rabbitmq.EventTransactionCompleted:
		return c.auditTransactionCompleted(ctx, d.Body)
	case rabbitmq.EventTransactionFailed:
		return c.auditTransactionFailed(ctx, d.Body)
	case rabbitmq.EventRefundCreated:
		return c.auditRefundCreated(ctx, d.Body)
	case rabbitmq.EventRefundCompleted:
		return c.auditRefundCompleted(ctx, d.Body)
	}
	return nil
}

func (c *Consumer) auditUserRegistered(ctx context.Context, body []byte) error {
	var evt struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal user event: %w", err)
	}
	userID, _ := uuid.Parse(evt.UserID)
	c.svc.Log(ctx, LogRequest{
		UserID:     userID,
		Action:     ActionUserRegistered,
		EntityType: "user",
		EntityID:   evt.UserID,
		Metadata:   map[string]interface{}{"email": evt.Email, "name": evt.Name},
	})
	return nil
}

func (c *Consumer) auditWalletCreated(ctx context.Context, body []byte) error {
	var evt struct {
		WalletID string `json:"wallet_id"`
		UserID   string `json:"user_id"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal wallet event: %w", err)
	}
	userID, _ := uuid.Parse(evt.UserID)
	c.svc.Log(ctx, LogRequest{
		UserID:     userID,
		Action:     ActionWalletCreated,
		EntityType: "wallet",
		EntityID:   evt.WalletID,
		Metadata:   map[string]interface{}{"currency": evt.Currency},
	})
	return nil
}

func (c *Consumer) auditTransactionCompleted(ctx context.Context, body []byte) error {
	var evt struct {
		TransactionID string  `json:"transaction_id"`
		SenderID      string  `json:"sender_id"`
		ReceiverID    string  `json:"receiver_id"`
		Amount        float64 `json:"amount"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal tx completed: %w", err)
	}
	senderID, _ := uuid.Parse(evt.SenderID)
	c.svc.Log(ctx, LogRequest{
		UserID:     senderID,
		Action:     ActionTransferCompleted,
		EntityType: "transaction",
		EntityID:   evt.TransactionID,
		Metadata: map[string]interface{}{
			"amount":      evt.Amount,
			"receiver_id": evt.ReceiverID,
		},
	})
	return nil
}

func (c *Consumer) auditTransactionFailed(ctx context.Context, body []byte) error {
	var evt struct {
		TransactionID string  `json:"transaction_id"`
		SenderID      string  `json:"sender_id"`
		Amount        float64 `json:"amount"`
		Reason        string  `json:"reason"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal tx failed: %w", err)
	}
	senderID, _ := uuid.Parse(evt.SenderID)
	c.svc.Log(ctx, LogRequest{
		UserID:     senderID,
		Action:     ActionTransferFailed,
		EntityType: "transaction",
		EntityID:   evt.TransactionID,
		Metadata:   map[string]interface{}{"amount": evt.Amount, "reason": evt.Reason},
	})
	return nil
}

func (c *Consumer) auditRefundCreated(ctx context.Context, body []byte) error {
	var evt struct {
		RefundID    string  `json:"refund_id"`
		RequestedBy string  `json:"requested_by"`
		Amount      float64 `json:"amount"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal refund created: %w", err)
	}
	userID, _ := uuid.Parse(evt.RequestedBy)
	c.svc.Log(ctx, LogRequest{
		UserID:     userID,
		Action:     ActionRefundRequested,
		EntityType: "refund",
		EntityID:   evt.RefundID,
		Metadata:   map[string]interface{}{"amount": evt.Amount},
	})
	return nil
}

func (c *Consumer) auditRefundCompleted(ctx context.Context, body []byte) error {
	var evt struct {
		RefundID   string  `json:"refund_id"`
		SenderID   string  `json:"sender_id"`
		Amount     float64 `json:"amount"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return fmt.Errorf("unmarshal refund completed: %w", err)
	}
	senderID, _ := uuid.Parse(evt.SenderID)
	c.svc.Log(ctx, LogRequest{
		UserID:     senderID,
		Action:     ActionRefundCompleted,
		EntityType: "refund",
		EntityID:   evt.RefundID,
		Metadata:   map[string]interface{}{"amount": evt.Amount},
	})
	return nil
}
