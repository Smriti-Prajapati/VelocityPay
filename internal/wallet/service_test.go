package wallet_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/velocitypay/velocitypay/internal/wallet"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubWalletRepo struct {
	walletsByUserID map[uuid.UUID]*wallet.Wallet
	walletsByNumber map[string]*wallet.Wallet
}

func newStub() *stubWalletRepo {
	return &stubWalletRepo{
		walletsByUserID: make(map[uuid.UUID]*wallet.Wallet),
		walletsByNumber: make(map[string]*wallet.Wallet),
	}
}

func (r *stubWalletRepo) Create(_ context.Context, w *wallet.Wallet) error {
	r.walletsByUserID[w.UserID] = w
	r.walletsByNumber[w.WalletNumber] = w
	return nil
}

func (r *stubWalletRepo) FindByID(_ context.Context, id uuid.UUID) (*wallet.Wallet, error) {
	for _, w := range r.walletsByUserID {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, wallet.ErrNotFound
}

func (r *stubWalletRepo) FindByUserID(_ context.Context, userID uuid.UUID) (*wallet.Wallet, error) {
	w, ok := r.walletsByUserID[userID]
	if !ok {
		return nil, wallet.ErrNotFound
	}
	return w, nil
}

func (r *stubWalletRepo) FindByWalletNumber(_ context.Context, number string) (*wallet.Wallet, error) {
	w, ok := r.walletsByNumber[number]
	if !ok {
		return nil, wallet.ErrNotFound
	}
	return w, nil
}

func (r *stubWalletRepo) UpdateBalance(_ context.Context, _ *sqlx.Tx, walletID uuid.UUID, delta float64) error {
	for _, w := range r.walletsByUserID {
		if w.ID == walletID {
			newBal := w.Balance + delta
			if newBal < 0 {
				return wallet.ErrInsufficientBalance
			}
			w.Balance = newBal
			return nil
		}
	}
	return wallet.ErrNotFound
}

func (r *stubWalletRepo) ExistsByUserID(_ context.Context, userID uuid.UUID) (bool, error) {
	_, ok := r.walletsByUserID[userID]
	return ok, nil
}

func (r *stubWalletRepo) TotalSentByUserID(_ context.Context, _ uuid.UUID) (float64, error) {
	return 150.00, nil
}

func (r *stubWalletRepo) TotalReceivedByUserID(_ context.Context, _ uuid.UUID) (float64, error) {
	return 300.00, nil
}

func (r *stubWalletRepo) BeginTx(_ context.Context) (*sqlx.Tx, error) { return nil, nil }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestWalletRepo_CreateAndFind(t *testing.T) {
	repo := newStub()
	ctx := context.Background()

	userID := uuid.New()
	w := &wallet.Wallet{
		ID:           uuid.New(),
		UserID:       userID,
		Balance:      0,
		WalletNumber: "VPY000000099",
		Currency:     "USD",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := repo.Create(ctx, w); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != w.ID {
		t.Errorf("id mismatch: want %v, got %v", w.ID, found.ID)
	}
}

func TestWalletRepo_DuplicatePrevented(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	w := &wallet.Wallet{ID: uuid.New(), UserID: userID, WalletNumber: "VPY000000100", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.Create(ctx, w)

	exists, err := repo.ExistsByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("wallet should exist")
	}
}

func TestWalletRepo_UpdateBalance_Debit(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	w := &wallet.Wallet{
		ID: uuid.New(), UserID: userID, Balance: 500.00,
		WalletNumber: "VPY000000200", Currency: "USD", IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = repo.Create(ctx, w)

	if err := repo.UpdateBalance(ctx, nil, w.ID, -200.00); err != nil {
		t.Fatalf("debit: %v", err)
	}
	if w.Balance != 300.00 {
		t.Errorf("expected 300.00, got %.2f", w.Balance)
	}
}

func TestWalletRepo_UpdateBalance_InsufficientFunds(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	w := &wallet.Wallet{
		ID: uuid.New(), UserID: userID, Balance: 50.00,
		WalletNumber: "VPY000000300", Currency: "USD",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = repo.Create(ctx, w)

	err := repo.UpdateBalance(ctx, nil, w.ID, -200.00)
	if !errors.Is(err, wallet.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestWalletRepo_FindByWalletNumber(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	w := &wallet.Wallet{
		ID: uuid.New(), UserID: userID, WalletNumber: "VPY000000400",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	_ = repo.Create(ctx, w)

	found, err := repo.FindByWalletNumber(ctx, "VPY000000400")
	if err != nil {
		t.Fatalf("find by number: %v", err)
	}
	if found.ID != w.ID {
		t.Error("wallet number lookup returned wrong wallet")
	}
}

func TestWalletRepo_NotFound(t *testing.T) {
	repo := newStub()
	_, err := repo.FindByUserID(context.Background(), uuid.New())
	if !errors.Is(err, wallet.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
