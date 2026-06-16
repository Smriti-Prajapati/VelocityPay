package fraud

import (
	"time"

	"github.com/google/uuid"
)

// RiskLevel classifies the severity of a fraud signal.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// AlertType identifies the fraud rule that triggered.
type AlertType string

const (
	AlertVelocityBreached   AlertType = "velocity_breach"    // too many transfers in short window
	AlertLargeTransaction   AlertType = "large_transaction"  // single transfer above threshold
	AlertRapidDepletion     AlertType = "rapid_depletion"    // balance drops >80% in one transfer
	AlertUnusualHour        AlertType = "unusual_hour"       // transfer at 1–5 AM local time
	AlertNewAccountTransfer AlertType = "new_account"        // account less than 24h old transferring
)

// FraudAlert is persisted whenever a rule fires.
type FraudAlert struct {
	ID            uuid.UUID `db:"id"             json:"id"`
	UserID        uuid.UUID `db:"user_id"        json:"user_id"`
	TransactionID uuid.UUID `db:"transaction_id" json:"transaction_id"`
	AlertType     AlertType `db:"alert_type"     json:"alert_type"`
	RiskLevel     RiskLevel `db:"risk_level"     json:"risk_level"`
	RiskScore     int       `db:"risk_score"     json:"risk_score"` // 0–100
	Details       string    `db:"details"        json:"details"`
	Reviewed      bool      `db:"reviewed"       json:"reviewed"`
	CreatedAt     time.Time `db:"created_at"     json:"created_at"`
}

// CheckRequest is the input to the fraud checker.
type CheckRequest struct {
	UserID        uuid.UUID
	TransactionID uuid.UUID
	Amount        float64
	WalletBalance float64
	AccountAge    time.Duration // time since account was created
	TransferHour  int           // 0–23 UTC
}

// CheckResult holds the outcome of a fraud evaluation.
type CheckResult struct {
	Allowed   bool
	RiskScore int
	Alerts    []FraudAlert
}

// Thresholds contains configurable fraud rule limits.
type Thresholds struct {
	MaxTransfersPerWindow  int           // default 10
	VelocityWindow         time.Duration // default 30s
	LargeTransactionAmount float64       // default 50000
	NewAccountAgeCutoff    time.Duration // default 24h
	RapidDepletionPercent  float64       // default 0.80 (80%)
}

// DefaultThresholds returns production-sensible defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MaxTransfersPerWindow:  10,
		VelocityWindow:         30 * time.Second,
		LargeTransactionAmount: 50_000,
		NewAccountAgeCutoff:    24 * time.Hour,
		RapidDepletionPercent:  0.80,
	}
}
