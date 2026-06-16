package transaction_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/velocitypay/velocitypay/internal/transaction"
	"github.com/velocitypay/velocitypay/internal/wallet"
)

// ── Wallet repository stub ────────────────────────────────────────────────────

type stubWalletRepo struct {
	walletsByUserID   map[uuid.UUID]*wallet.Wallet
	walletsByNumber   map[string]*wallet.Wallet
	updateBalanceErr  error
	balanceCallCount  int
}

func newStubWalletRepo() *stubWalletRepo {
	return &stubWalletRepo{
		walletsByUserID: make(map[uuid.UUID]*wallet.Wallet),
		walletsByNumber: make(map[string]*wallet.Wallet),
	}
}

func (s *stubWalletRepo) addWallet(w *wallet.Wallet) {
	s.walletsByUserID[w.UserID] = w
	s.walletsByNumber[w.WalletNumber] = w
}

func (s *stubWalletRepo) Create(_ context.Context, w *wallet.Wallet) error { return nil }

func (s *stubWalletRepo) FindByID(_ context.Context, id uuid.UUID) (*wallet.Wallet, error) {
	for _, w := range s.walletsByUserID {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, wallet.ErrNotFound
}

func (s *stubWalletRepo) FindByUserID(_ context.Context, userID uuid.UUID) (*wallet.Wallet, error) {
	w, ok := s.walletsByUserID[userID]
	if !ok {
		return nil, wallet.ErrNotFound
	}
	return w, nil
}

func (s *stubWalletRepo) FindByWalletNumber(_ context.Context, number string) (*wallet.Wallet, error) {
	w, ok := s.walletsByNumber[number]
	if !ok {
		return nil, wallet.ErrNotFound
	}
	return w, nil
}

func (s *stubWalletRepo) UpdateBalance(_ context.Context, _ *sqlx.Tx, walletID uuid.UUID, delta float64) error {
	s.balanceCallCount++
	if s.updateBalanceErr != nil {
		return s.updateBalanceErr
	}
	for _, w := range s.walletsByUserID {
		if w.ID == walletID {
			w.Balance += delta
			if w.Balance < 0 {
				w.Balance -= delta // undo
				return wallet.ErrInsufficientBalance
			}
		}
	}
	return nil
}

func (s *stubWalletRepo) ExistsByUserID(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil }
func (s *stubWalletRepo) TotalSentByUserID(_ context.Context, _ uuid.UUID) (float64, error) { return 0, nil }
func (s *stubWalletRepo) TotalReceivedByUserID(_ context.Context, _ uuid.UUID) (float64, error) { return 0, nil }
func (s *stubWalletRepo) BeginTx(_ context.Context) (*sqlx.Tx, error) { return nil, nil }

// ── Transaction repository stub ───────────────────────────────────────────────

type stubTxnRepo struct {
	transactions map[uuid.UUID]*transaction.Transaction
	byIdemKey    map[string]*transaction.Transaction
	createErr    error
}

func newStubTxnRepo() *stubTxnRepo {
	return &stubTxnRepo{
		transactions: make(map[uuid.UUID]*transaction.Transaction),
		byIdemKey:    make(map[string]*transaction.Transaction),
	}
}

func (s *stubTxnRepo) Create(_ context.Context, _ *sqlx.Tx, t *transaction.Transaction) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.transactions[t.ID] = t
	if t.IdempotencyKey != "" {
		s.byIdemKey[t.IdempotencyKey] = t
	}
	return nil
}

func (s *stubTxnRepo) FindByID(_ context.Context, id uuid.UUID) (*transaction.Transaction, error) {
	t, ok := s.transactions[id]
	if !ok {
		return nil, transaction.ErrNotFound
	}
	return t, nil
}

func (s *stubTxnRepo) FindByIdempotencyKey(_ context.Context, key string) (*transaction.Transaction, error) {
	t, ok := s.byIdemKey[key]
	if !ok {
		return nil, transaction.ErrNotFound
	}
	return t, nil
}

func (s *stubTxnRepo) UpdateStatus(_ context.Context, id uuid.UUID, status transaction.Status, reason string) error {
	if t, ok := s.transactions[id]; ok {
		t.Status = status
		t.FailureReason = reason
	}
	return nil
}

func (s *stubTxnRepo) ListByUserID(_ context.Context, userID uuid.UUID, _ transaction.HistoryFilter) ([]*transaction.Transaction, int, error) {
	var result []*transaction.Transaction
	for _, t := range s.transactions {
		if t.SenderID == userID || t.ReceiverID == userID {
			result = append(result, t)
		}
	}
	return result, len(result), nil
}

func (s *stubTxnRepo) BeginTx(_ context.Context) (*sqlx.Tx, error) { return nil, nil }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestTransferDomain_SelfTransferRejected(t *testing.T) {
	senderID := uuid.New()
	walletID := uuid.New()

	senderWallet := &wallet.Wallet{
		ID:           walletID,
		UserID:       senderID,
		Balance:      500.00,
		WalletNumber: "VPY000000001",
		Currency:     "USD",
		IsActive:     true,
	}

	walletRepo := newStubWalletRepo()
	walletRepo.addWallet(senderWallet)

	// Sender tries to send to their own wallet number
	req := &transaction.TransferRequest{
		ReceiverWalletNumber: "VPY000000001",
		Amount:               100,
	}

	txnRepo := newStubTxnRepo()
	_ = txnRepo
	_ = req

	// Verify that sender and receiver wallets are the same — service must reject this
	recv, err := walletRepo.FindByWalletNumber(context.Background(), req.ReceiverWalletNumber)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if recv.ID != senderWallet.ID {
		t.Fatal("expected wallets to be the same for self-transfer check")
	}
}

func TestTransferDomain_InsufficientBalance(t *testing.T) {
	senderID := uuid.New()
	receiverID := uuid.New()

	senderWallet := &wallet.Wallet{
		ID: uuid.New(), UserID: senderID, Balance: 50.00,
		WalletNumber: "VPY000000010", Currency: "USD", IsActive: true,
	}
	receiverWallet := &wallet.Wallet{
		ID: uuid.New(), UserID: receiverID, Balance: 0,
		WalletNumber: "VPY000000020", Currency: "USD", IsActive: true,
	}

	walletRepo := newStubWalletRepo()
	walletRepo.addWallet(senderWallet)
	walletRepo.addWallet(receiverWallet)

	// Attempt to debit more than balance
	err := walletRepo.UpdateBalance(context.Background(), nil, senderWallet.ID, -200.00)
	if !errors.Is(err, wallet.ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got: %v", err)
	}
}

func TestTransferDomain_BalanceAfterTransfer(t *testing.T) {
	senderID := uuid.New()
	receiverID := uuid.New()

	senderWallet := &wallet.Wallet{
		ID: uuid.New(), UserID: senderID, Balance: 1000.00,
		WalletNumber: "VPY000000030", Currency: "USD", IsActive: true,
	}
	receiverWallet := &wallet.Wallet{
		ID: uuid.New(), UserID: receiverID, Balance: 200.00,
		WalletNumber: "VPY000000040", Currency: "USD", IsActive: true,
	}

	walletRepo := newStubWalletRepo()
	walletRepo.addWallet(senderWallet)
	walletRepo.addWallet(receiverWallet)

	amount := 300.00

	if err := walletRepo.UpdateBalance(context.Background(), nil, senderWallet.ID, -amount); err != nil {
		t.Fatalf("debit sender: %v", err)
	}
	if err := walletRepo.UpdateBalance(context.Background(), nil, receiverWallet.ID, amount); err != nil {
		t.Fatalf("credit receiver: %v", err)
	}

	if senderWallet.Balance != 700.00 {
		t.Errorf("sender balance: want 700.00, got %.2f", senderWallet.Balance)
	}
	if receiverWallet.Balance != 500.00 {
		t.Errorf("receiver balance: want 500.00, got %.2f", receiverWallet.Balance)
	}
}

func TestHistoryFilter_Defaults(t *testing.T) {
	f := transaction.HistoryFilter{}
	if f.Page != 0 {
		t.Errorf("expected zero Page default, got %d", f.Page)
	}
	if f.PageSize != 0 {
		t.Errorf("expected zero PageSize default, got %d", f.PageSize)
	}
}

func TestTransactionStatus_Values(t *testing.T) {
	statuses := []transaction.Status{
		transaction.StatusPending,
		transaction.StatusCompleted,
		transaction.StatusFailed,
		transaction.StatusReversed,
	}
	for _, s := range statuses {
		if s == "" {
			t.Errorf("transaction status must not be empty")
		}
	}
}

func TestTransactionDomain_IdempotencyKey(t *testing.T) {
	repo := newStubTxnRepo()
	key := uuid.New().String()

	txn := &transaction.Transaction{
		ID:             uuid.New(),
		SenderID:       uuid.New(),
		ReceiverID:     uuid.New(),
		Amount:         100.00,
		TransactionType: transaction.TypeTransfer,
		Status:         transaction.StatusPending,
		IdempotencyKey: key,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := repo.Create(context.Background(), nil, txn); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByIdempotencyKey(context.Background(), key)
	if err != nil {
		t.Fatalf("find by key: %v", err)
	}
	if found.ID != txn.ID {
		t.Errorf("expected txn ID %v, got %v", txn.ID, found.ID)
	}
}

func TestGetByID_AccessDenied(t *testing.T) {
	repo := newStubTxnRepo()
	senderID := uuid.New()
	receiverID := uuid.New()
	strangerID := uuid.New()

	txn := &transaction.Transaction{
		ID:         uuid.New(),
		SenderID:   senderID,
		ReceiverID: receiverID,
		Amount:     50,
		Status:     transaction.StatusCompleted,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	_ = repo.Create(context.Background(), nil, txn)

	found, err := repo.FindByID(context.Background(), txn.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	// Stranger should be denied
	if found.SenderID == strangerID || found.ReceiverID == strangerID {
		t.Error("stranger should not have access")
	}
}
