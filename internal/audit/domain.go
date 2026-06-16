package audit

import (
	"time"

	"github.com/google/uuid"
)

// Action classifies what happened in the system.
type Action string

const (
	// Auth
	ActionUserRegistered  Action = "user.registered"
	ActionUserLoggedIn    Action = "user.logged_in"
	ActionPasswordChanged Action = "user.password_changed"
	ActionProfileUpdated  Action = "user.profile_updated"

	// Wallet
	ActionWalletCreated Action = "wallet.created"
	ActionMoneyAdded    Action = "wallet.money_added"

	// Transactions
	ActionTransferInitiated  Action = "transaction.initiated"
	ActionTransferCompleted  Action = "transaction.completed"
	ActionTransferFailed     Action = "transaction.failed"

	// Refunds
	ActionRefundRequested Action = "refund.requested"
	ActionRefundCompleted Action = "refund.completed"
	ActionRefundRejected  Action = "refund.rejected"

	// Fraud
	ActionFraudAlertCreated  Action = "fraud.alert_created"
	ActionFraudAlertReviewed Action = "fraud.alert_reviewed"
)

// AuditLog is a single immutable record of an action in the system.
type AuditLog struct {
	ID         uuid.UUID `db:"id"          json:"id"`
	UserID     uuid.UUID `db:"user_id"     json:"user_id"`
	Action     Action    `db:"action"      json:"action"`
	EntityType string    `db:"entity_type" json:"entity_type"` // "user", "wallet", "transaction"
	EntityID   string    `db:"entity_id"   json:"entity_id"`
	IPAddress  string    `db:"ip_address"  json:"ip_address,omitempty"`
	UserAgent  string    `db:"user_agent"  json:"user_agent,omitempty"`
	Metadata   string    `db:"metadata"    json:"metadata,omitempty"` // JSON string
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// LogRequest is used internally to create audit log entries.
type LogRequest struct {
	UserID     uuid.UUID
	Action     Action
	EntityType string
	EntityID   string
	IPAddress  string
	UserAgent  string
	Metadata   map[string]interface{}
}

// ListFilter controls pagination for audit log queries.
type ListFilter struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Action   string `form:"action"`
}

// ListResponse is the paginated audit log response.
type ListResponse struct {
	Logs       []*AuditLog `json:"logs"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
