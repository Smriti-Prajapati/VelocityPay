package refund

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	"github.com/velocitypay/velocitypay/internal/transaction"
	"github.com/velocitypay/velocitypay/internal/wallet"
	"go.uber.org/zap"
)

// Service contains all business logic for refund operations.
type Service struct {
	repo        Repository
	txnRepo     transaction.Repository
	walletRepo  wallet.Repository
	publisher   *rabbitmq.Publisher
	log         *zap.Logger
}

// NewService wires up the refund service.
func NewService(
	repo Repository,
	txnRepo transaction.Repository,
	walletRepo wallet.Repository,
	publisher *rabbitmq.Publisher,
	log *zap.Logger,
) *Service {
	return &Service{
		repo:       repo,
		txnRepo:    txnRepo,
		walletRepo: walletRepo,
		publisher:  publisher,
		log:        log,
	}
}

// Request creates a new refund request for a completed transaction.
// Rules:
//   - Only the sender of the original transaction can request a refund.
//   - The transaction must be in "completed" status.
//   - Refund amount must not exceed the original transaction amount.
//   - Only one refund per transaction is allowed.
func (s *Service) Request(ctx context.Context, requesterID uuid.UUID, req *RequestRefundRequest) (*Refund, error) {
	txnID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		return nil, errors.New("invalid transaction id")
	}

	// 1. Load the original transaction
	txn, err := s.txnRepo.FindByID(ctx, txnID)
	if err != nil {
		if errors.Is(err, transaction.ErrNotFound) {
			return nil, errors.New("transaction not found")
		}
		return nil, fmt.Errorf("find transaction: %w", err)
	}

	// 2. Only the sender can request a refund
	if txn.SenderID != requesterID {
		return nil, errors.New("only the sender can request a refund")
	}

	// 3. Transaction must be completed
	if txn.Status != transaction.StatusCompleted {
		return nil, fmt.Errorf("cannot refund a transaction with status %q", txn.Status)
	}

	// 4. Refund amount cannot exceed original
	if req.Amount > txn.Amount {
		return nil, fmt.Errorf("refund amount (%.2f) exceeds original transaction amount (%.2f)", req.Amount, txn.Amount)
	}

	// 5. No duplicate refund for the same transaction
	exists, err := s.repo.ExistsByTransactionID(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("check duplicate refund: %w", err)
	}
	if exists {
		return nil, ErrAlreadyRequested
	}

	now := time.Now().UTC()
	rf := &Refund{
		ID:            uuid.New(),
		TransactionID: txnID,
		RequestedBy:   requesterID,
		Amount:        req.Amount,
		Reason:        req.Reason,
		Status:        StatusPending,
		ReviewedBy:    uuid.Nil,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.Create(ctx, rf); err != nil {
		return nil, fmt.Errorf("create refund: %w", err)
	}

	// Publish event for notification service
	_ = s.publisher.Publish(ctx, rabbitmq.EventRefundCreated, map[string]interface{}{
		"refund_id":      rf.ID,
		"transaction_id": txnID,
		"requested_by":   requesterID,
		"amount":         req.Amount,
	})

	s.log.Info("refund requested",
		zap.String("refund_id", rf.ID.String()),
		zap.String("transaction_id", txnID.String()),
		zap.Float64("amount", req.Amount),
	)

	return rf, nil
}

// GetByID returns a refund, enforcing that the requester owns it.
func (s *Service) GetByID(ctx context.Context, requesterID uuid.UUID, refundID uuid.UUID) (*RefundResponse, error) {
	rf, err := s.repo.FindByID(ctx, refundID)
	if err != nil {
		return nil, err
	}

	if rf.RequestedBy != requesterID {
		return nil, errors.New("access denied")
	}

	txn, err := s.txnRepo.FindByID(ctx, rf.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("find original transaction: %w", err)
	}

	return &RefundResponse{
		Refund:            rf,
		TransactionAmount: txn.Amount,
		TransactionStatus: string(txn.Status),
	}, nil
}

// ListMine returns all refund requests made by the authenticated user.
func (s *Service) ListMine(ctx context.Context, userID uuid.UUID) ([]*Refund, error) {
	return s.repo.ListByUserID(ctx, userID)
}

// Process approves or rejects a pending refund.
// When approved, it reverses the funds atomically:
//   receiver's wallet is debited → sender's wallet is credited → transaction marked reversed.
func (s *Service) Process(ctx context.Context, reviewerID uuid.UUID, refundID uuid.UUID, approve bool, note string) (*Refund, error) {
	rf, err := s.repo.FindByID(ctx, refundID)
	if err != nil {
		return nil, err
	}

	if rf.Status != StatusPending {
		return nil, fmt.Errorf("refund is already %s", rf.Status)
	}

	if !approve {
		if err := s.repo.UpdateStatus(ctx, refundID, StatusRejected, reviewerID, note); err != nil {
			return nil, fmt.Errorf("reject refund: %w", err)
		}
		rf.Status = StatusRejected
		rf.ReviewNote = note
		s.log.Info("refund rejected", zap.String("refund_id", refundID.String()))
		return rf, nil
	}

	// Load the original transaction to find sender and receiver wallets
	txn, err := s.txnRepo.FindByID(ctx, rf.TransactionID)
	if err != nil {
		return nil, fmt.Errorf("find original transaction: %w", err)
	}

	senderWallet, err := s.walletRepo.FindByUserID(ctx, txn.SenderID)
	if err != nil {
		return nil, fmt.Errorf("find sender wallet: %w", err)
	}

	receiverWallet, err := s.walletRepo.FindByUserID(ctx, txn.ReceiverID)
	if err != nil {
		return nil, fmt.Errorf("find receiver wallet: %w", err)
	}

	// Atomic reversal: debit receiver, credit sender
	dbTx, err := s.walletRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = dbTx.Rollback()
		}
	}()

	// Debit the receiver (they return the money)
	if err = s.walletRepo.UpdateBalance(ctx, dbTx, receiverWallet.ID, -rf.Amount); err != nil {
		return nil, fmt.Errorf("debit receiver for refund: %w", err)
	}

	// Credit the original sender
	if err = s.walletRepo.UpdateBalance(ctx, dbTx, senderWallet.ID, rf.Amount); err != nil {
		return nil, fmt.Errorf("credit sender for refund: %w", err)
	}

	if err = dbTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit refund: %w", err)
	}

	// Update refund and original transaction status
	if err = s.repo.UpdateStatus(ctx, refundID, StatusCompleted, reviewerID, note); err != nil {
		return nil, fmt.Errorf("complete refund: %w", err)
	}
	if err = s.txnRepo.UpdateStatus(ctx, txn.ID, transaction.StatusReversed, "refunded"); err != nil {
		s.log.Error("failed to mark transaction reversed", zap.Error(err))
	}

	rf.Status = StatusCompleted
	rf.ReviewNote = note

	// Publish completion event
	_ = s.publisher.Publish(ctx, rabbitmq.EventRefundCompleted, map[string]interface{}{
		"refund_id":      rf.ID,
		"transaction_id": txn.ID,
		"amount":         rf.Amount,
		"sender_id":      txn.SenderID,
		"receiver_id":    txn.ReceiverID,
	})

	s.log.Info("refund completed",
		zap.String("refund_id", rf.ID.String()),
		zap.Float64("amount", rf.Amount),
	)

	return rf, nil
}
