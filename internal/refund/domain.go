package refund

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle of a refund request.
type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusCompleted Status = "completed"
)

// Refund is the canonical record of a refund request.
type Refund struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	TransactionID uuid.UUID `db:"transaction_id" json:"transaction_id"`
	RequestedBy   uuid.UUID `db:"requested_by"   json:"requested_by"`
	Amount        float64   `db:"amount"         json:"amount"`
	Reason        string    `db:"reason"         json:"reason"`
	Status        Status    `db:"status"         json:"status"`
	ReviewedBy    uuid.UUID `db:"reviewed_by"    json:"reviewed_by,omitempty"`
	ReviewNote    string    `db:"review_note"    json:"review_note,omitempty"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"     json:"updated_at"`
}

// RequestRefundRequest is the payload for POST /api/refunds.
type RequestRefundRequest struct {
	TransactionID string  `json:"transaction_id" validate:"required,uuid"`
	Amount        float64 `json:"amount"         validate:"required,gt=0"`
	Reason        string  `json:"reason"         validate:"required,min=10,max=500"`
}

// RefundResponse wraps a refund with the original transaction detail.
type RefundResponse struct {
	*Refund
	TransactionAmount float64 `json:"transaction_amount"`
	TransactionStatus string  `json:"transaction_status"`
}
