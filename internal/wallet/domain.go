package wallet

import (
	"time"

	"github.com/google/uuid"
)

// Wallet represents a user's digital wallet.
type Wallet struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	UserID       uuid.UUID `db:"user_id"       json:"user_id"`
	Balance      float64   `db:"balance"       json:"balance"`
	WalletNumber string    `db:"wallet_number" json:"wallet_number"`
	Currency     string    `db:"currency"      json:"currency"`
	IsActive     bool      `db:"is_active"     json:"is_active"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// CreateWalletRequest is the payload for POST /api/wallet/create.
type CreateWalletRequest struct {
	Currency string `json:"currency" validate:"required,oneof=USD EUR GBP INR AED SGD"`
}

// AddMoneyRequest is the payload for POST /api/wallet/add-money.
type AddMoneyRequest struct {
	Amount float64 `json:"amount" validate:"required,gt=0"`
	Notes  string  `json:"notes"  validate:"omitempty,max=255"`
}

// WalletDetails is the enriched response returned for wallet queries.
type WalletDetails struct {
	*Wallet
	TotalSent     float64 `json:"total_sent"`
	TotalReceived float64 `json:"total_received"`
}
