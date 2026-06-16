package notification

import (
	"time"

	"github.com/google/uuid"
)

// Type classifies the notification so the frontend can render the right icon.
type Type string

const (
	TypeTransactionSent     Type = "transaction_sent"
	TypeTransactionReceived Type = "transaction_received"
	TypeTransactionFailed   Type = "transaction_failed"
	TypeRefundRequested     Type = "refund_requested"
	TypeRefundCompleted     Type = "refund_completed"
	TypeWalletCreated       Type = "wallet_created"
	TypeUserWelcome         Type = "user_welcome"
)

// Notification is a single in-app message for a user.
type Notification struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	UserID     uuid.UUID `db:"user_id"     json:"user_id"`
	Type       Type      `db:"type"        json:"type"`
	Title      string    `db:"title"       json:"title"`
	Message    string    `db:"message"     json:"message"`
	IsRead     bool      `db:"is_read"     json:"is_read"`
	RelatedID  string    `db:"related_id"  json:"related_id,omitempty"` // transaction_id, refund_id, etc.
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// ListResponse is the paginated response for GET /api/notifications.
type ListResponse struct {
	Notifications []*Notification `json:"notifications"`
	Total         int             `json:"total"`
	UnreadCount   int             `json:"unread_count"`
}

// Event payloads consumed from RabbitMQ -----------------------------------

// TransactionEvent is the payload shape for transaction.* events.
type TransactionEvent struct {
	TransactionID string  `json:"transaction_id"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	Reason        string  `json:"reason,omitempty"`
}

// RefundEvent is the payload shape for refund.* events.
type RefundEvent struct {
	RefundID      string  `json:"refund_id"`
	TransactionID string  `json:"transaction_id"`
	RequestedBy   string  `json:"requested_by"`
	SenderID      string  `json:"sender_id"`
	ReceiverID    string  `json:"receiver_id"`
	Amount        float64 `json:"amount"`
}

// WalletEvent is the payload shape for wallet.created events.
type WalletEvent struct {
	WalletID string `json:"wallet_id"`
	UserID   string `json:"user_id"`
	Currency string `json:"currency"`
}

// UserEvent is the payload shape for user.registered events.
type UserEvent struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}
