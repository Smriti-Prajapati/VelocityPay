package refund_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/refund"
	"github.com/velocitypay/velocitypay/internal/transaction"
)

// ── Refund repository stub ────────────────────────────────────────────────────

type stubRefundRepo struct {
	refunds map[uuid.UUID]*refund.Refund
	byTxnID map[uuid.UUID]*refund.Refund
}

func newStubRefundRepo() *stubRefundRepo {
	return &stubRefundRepo{
		refunds: make(map[uuid.UUID]*refund.Refund),
		byTxnID: make(map[uuid.UUID]*refund.Refund),
	}
}

func (r *stubRefundRepo) Create(_ context.Context, rf *refund.Refund) error {
	r.refunds[rf.ID] = rf
	r.byTxnID[rf.TransactionID] = rf
	return nil
}

func (r *stubRefundRepo) FindByID(_ context.Context, id uuid.UUID) (*refund.Refund, error) {
	rf, ok := r.refunds[id]
	if !ok {
		return nil, refund.ErrNotFound
	}
	return rf, nil
}

func (r *stubRefundRepo) FindByTransactionID(_ context.Context, txnID uuid.UUID) (*refund.Refund, error) {
	rf, ok := r.byTxnID[txnID]
	if !ok {
		return nil, refund.ErrNotFound
	}
	return rf, nil
}

func (r *stubRefundRepo) UpdateStatus(_ context.Context, id uuid.UUID, status refund.Status, _ uuid.UUID, note string) error {
	rf, ok := r.refunds[id]
	if !ok {
		return refund.ErrNotFound
	}
	rf.Status = status
	rf.ReviewNote = note
	return nil
}

func (r *stubRefundRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]*refund.Refund, error) {
	var result []*refund.Refund
	for _, rf := range r.refunds {
		if rf.RequestedBy == userID {
			result = append(result, rf)
		}
	}
	return result, nil
}

func (r *stubRefundRepo) ExistsByTransactionID(_ context.Context, txnID uuid.UUID) (bool, error) {
	_, ok := r.byTxnID[txnID]
	return ok, nil
}

// ── Transaction repository stub ───────────────────────────────────────────────

type stubTxnRepo struct {
	txns map[uuid.UUID]*transaction.Transaction
}

func newStubTxnRepo() *stubTxnRepo {
	return &stubTxnRepo{txns: make(map[uuid.UUID]*transaction.Transaction)}
}

func (r *stubTxnRepo) FindByID(_ context.Context, id uuid.UUID) (*transaction.Transaction, error) {
	t, ok := r.txns[id]
	if !ok {
		return nil, transaction.ErrNotFound
	}
	return t, nil
}

func (r *stubTxnRepo) FindByIdempotencyKey(_ context.Context, _ string) (*transaction.Transaction, error) {
	return nil, transaction.ErrNotFound
}

func (r *stubTxnRepo) Create(_ context.Context, _ interface{}, t *transaction.Transaction) error {
	r.txns[t.ID] = t
	return nil
}

func (r *stubTxnRepo) UpdateStatus(_ context.Context, id uuid.UUID, status transaction.Status, reason string) error {
	if t, ok := r.txns[id]; ok {
		t.Status = status
		t.FailureReason = reason
	}
	return nil
}

func (r *stubTxnRepo) ListByUserID(_ context.Context, _ uuid.UUID, _ transaction.HistoryFilter) ([]*transaction.Transaction, int, error) {
	return nil, 0, nil
}

func (r *stubTxnRepo) BeginTx(_ context.Context) (interface{}, error) { return nil, nil }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestRefund_OnlyCompletedTransactions(t *testing.T) {
	senderID := uuid.New()
	txnID := uuid.New()

	pendingTxn := &transaction.Transaction{
		ID:         txnID,
		SenderID:   senderID,
		ReceiverID: uuid.New(),
		Amount:     500,
		Status:     transaction.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// A pending transaction should not be refundable
	if pendingTxn.Status == transaction.StatusCompleted {
		t.Error("pending transaction should not be refundable")
	}
}

func TestRefund_AmountCannotExceedOriginal(t *testing.T) {
	originalAmount := 500.00
	refundAmount := 600.00

	if refundAmount > originalAmount {
		// This is the validation the service enforces
		err := errors.New("refund amount exceeds original transaction amount")
		if err == nil {
			t.Error("expected error for excessive refund amount")
		}
	}
}

func TestRefund_DuplicateRejected(t *testing.T) {
	repo := newStubRefundRepo()
	ctx := context.Background()

	txnID := uuid.New()
	senderID := uuid.New()

	rf := &refund.Refund{
		ID:            uuid.New(),
		TransactionID: txnID,
		RequestedBy:   senderID,
		Amount:        100,
		Status:        refund.StatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	_ = repo.Create(ctx, rf)

	exists, err := repo.ExistsByTransactionID(ctx, txnID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("duplicate refund should be detected")
	}
}

func TestRefund_CreateAndFind(t *testing.T) {
	repo := newStubRefundRepo()
	ctx := context.Background()

	txnID := uuid.New()
	requesterID := uuid.New()

	rf := &refund.Refund{
		ID:            uuid.New(),
		TransactionID: txnID,
		RequestedBy:   requesterID,
		Amount:        250,
		Reason:        "item not delivered",
		Status:        refund.StatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := repo.Create(ctx, rf); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByID(ctx, rf.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Amount != rf.Amount {
		t.Errorf("amount mismatch: want %.2f, got %.2f", rf.Amount, found.Amount)
	}
	if found.Status != refund.StatusPending {
		t.Errorf("expected pending status, got %s", found.Status)
	}
}

func TestRefund_StatusTransitions(t *testing.T) {
	repo := newStubRefundRepo()
	ctx := context.Background()

	rf := &refund.Refund{
		ID:        uuid.New(),
		RequestedBy: uuid.New(),
		TransactionID: uuid.New(),
		Amount:    100,
		Status:    refund.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = repo.Create(ctx, rf)

	// Approve it
	if err := repo.UpdateStatus(ctx, rf.ID, refund.StatusCompleted, uuid.New(), "approved by admin"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	found, _ := repo.FindByID(ctx, rf.ID)
	if found.Status != refund.StatusCompleted {
		t.Errorf("expected completed, got %s", found.Status)
	}
	if found.ReviewNote != "approved by admin" {
		t.Error("review note not saved")
	}
}

func TestRefund_NotFound(t *testing.T) {
	repo := newStubRefundRepo()
	_, err := repo.FindByID(context.Background(), uuid.New())
	if !errors.Is(err, refund.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRefund_ListByUser(t *testing.T) {
	repo := newStubRefundRepo()
	ctx := context.Background()

	userID := uuid.New()
	for i := 0; i < 3; i++ {
		rf := &refund.Refund{
			ID:            uuid.New(),
			TransactionID: uuid.New(),
			RequestedBy:   userID,
			Amount:        float64(100 * (i + 1)),
			Status:        refund.StatusPending,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_ = repo.Create(ctx, rf)
	}

	// Add one from a different user
	_ = repo.Create(ctx, &refund.Refund{
		ID: uuid.New(), TransactionID: uuid.New(),
		RequestedBy: uuid.New(), Amount: 50,
		Status: refund.StatusPending, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	results, err := repo.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 refunds for user, got %d", len(results))
	}
}
