package transaction

import (
	"time"

	"github.com/google/uuid"
)

// Status values for a transaction lifecycle.
type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusReversed  Status = "reversed"
)

// Type classifies the nature of a transaction.
type Type string

const (
	TypeTransfer Type = "transfer"
	TypeDeposit  Type = "deposit"
	TypeRefund   Type = "refund"
)

// Transaction is the canonical record of every fund movement.
type Transaction struct {
	ID              uuid.UUID  `db:"id"               json:"id"`
	SenderID        uuid.UUID  `db:"sender_id"        json:"sender_id"`
	ReceiverID      uuid.UUID  `db:"receiver_id"      json:"receiver_id"`
	Amount          float64    `db:"amount"           json:"amount"`
	TransactionType Type       `db:"transaction_type" json:"transaction_type"`
	Status          Status     `db:"status"           json:"status"`
	Notes           string     `db:"notes"            json:"notes,omitempty"`
	IdempotencyKey  string     `db:"idempotency_key"  json:"idempotency_key,omitempty"`
	FailureReason   string     `db:"failure_reason"   json:"failure_reason,omitempty"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"       json:"updated_at"`
}

// TransferRequest is the payload for POST /api/transactions/transfer.
type TransferRequest struct {
	ReceiverWalletNumber string  `json:"receiver_wallet_number" validate:"required"`
	Amount               float64 `json:"amount"                 validate:"required,gt=0"`
	Notes                string  `json:"notes"                  validate:"omitempty,max=255"`
	// IdempotencyKey is supplied by the client to prevent duplicate transfers.
	// If omitted, one is auto-generated (less safe — client should always provide it).
	IdempotencyKey string `json:"idempotency_key" validate:"omitempty,uuid"`
}

// HistoryFilter controls pagination and filtering for transaction history.
type HistoryFilter struct {
	Page     int    `form:"page"      validate:"omitempty,min=1"`
	PageSize int    `form:"page_size" validate:"omitempty,min=1,max=100"`
	Status   string `form:"status"    validate:"omitempty,oneof=pending completed failed reversed"`
	Type     string `form:"type"      validate:"omitempty,oneof=transfer deposit refund"`
}

// HistoryResponse is the paginated transaction list.
type HistoryResponse struct {
	Transactions []*Transaction `json:"transactions"`
	Total        int            `json:"total"`
	Page         int            `json:"page"`
	PageSize     int            `json:"page_size"`
	TotalPages   int            `json:"total_pages"`
}

// TransferJob is enqueued into the worker pool for async processing.
type TransferJob struct {
	SenderID    uuid.UUID
	ReceiverID  uuid.UUID
	SenderWalletID   uuid.UUID
	ReceiverWalletID uuid.UUID
	Amount      float64
	Notes       string
	Transaction *Transaction
	ResultCh    chan<- TransferResult
}

// TransferResult carries the outcome back to the HTTP handler.
type TransferResult struct {
	Transaction *Transaction
	Err         error
}
