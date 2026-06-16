package fraud

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service exposes fraud operations to the rest of the application.
type Service struct {
	checker *Checker
	repo    Repository
	log     *zap.Logger
}

// NewService wires up the fraud service.
func NewService(checker *Checker, repo Repository, log *zap.Logger) *Service {
	return &Service{checker: checker, repo: repo, log: log}
}

// Check runs the fraud evaluation for a transfer. Called by the transaction
// service before funds are moved.
func (s *Service) Check(ctx context.Context, req CheckRequest) CheckResult {
	return s.checker.Evaluate(ctx, req)
}

// GetAlertsByUser returns all fraud alerts for a given user.
func (s *Service) GetAlertsByUser(ctx context.Context, userID uuid.UUID) ([]*FraudAlert, error) {
	alerts, err := s.repo.ListAlertsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get fraud alerts: %w", err)
	}
	if alerts == nil {
		alerts = []*FraudAlert{}
	}
	return alerts, nil
}

// GetUnreviewed returns all unreviewed high-risk alerts (for admin dashboard).
func (s *Service) GetUnreviewed(ctx context.Context) ([]*FraudAlert, error) {
	alerts, err := s.repo.ListUnreviewed(ctx)
	if err != nil {
		return nil, fmt.Errorf("get unreviewed alerts: %w", err)
	}
	if alerts == nil {
		alerts = []*FraudAlert{}
	}
	return alerts, nil
}

// MarkReviewed marks an alert as reviewed by an admin.
func (s *Service) MarkReviewed(ctx context.Context, alertID uuid.UUID) error {
	return s.repo.MarkReviewed(ctx, alertID)
}

// newID generates a new UUID for alert records.
func newID() uuid.UUID {
	return uuid.New()
}

// BuildCheckRequest constructs a CheckRequest from transaction context.
func BuildCheckRequest(
	userID uuid.UUID,
	txnID uuid.UUID,
	amount float64,
	walletBalance float64,
	accountCreatedAt time.Time,
) CheckRequest {
	now := time.Now().UTC()
	return CheckRequest{
		UserID:        userID,
		TransactionID: txnID,
		Amount:        amount,
		WalletBalance: walletBalance,
		AccountAge:    now.Sub(accountCreatedAt),
		TransferHour:  now.Hour(),
	}
}
