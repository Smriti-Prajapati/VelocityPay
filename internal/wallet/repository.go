package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrNotFound is returned when no wallet matches the query.
var ErrNotFound = errors.New("wallet not found")

// ErrAlreadyExists is returned when a user tries to create a second wallet.
var ErrAlreadyExists = errors.New("wallet already exists for this user")

// ErrInsufficientBalance is returned during transfers.
var ErrInsufficientBalance = errors.New("insufficient wallet balance")

// Repository defines the data access contract for wallets.
type Repository interface {
	Create(ctx context.Context, w *Wallet) error
	FindByID(ctx context.Context, id uuid.UUID) (*Wallet, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) (*Wallet, error)
	FindByWalletNumber(ctx context.Context, number string) (*Wallet, error)
	UpdateBalance(ctx context.Context, tx *sqlx.Tx, walletID uuid.UUID, delta float64) error
	ExistsByUserID(ctx context.Context, userID uuid.UUID) (bool, error)
	TotalSentByUserID(ctx context.Context, userID uuid.UUID) (float64, error)
	TotalReceivedByUserID(ctx context.Context, userID uuid.UUID) (float64, error)
	BeginTx(ctx context.Context) (*sqlx.Tx, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed wallet repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, w *Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, balance, wallet_number, currency, is_active, created_at, updated_at)
		VALUES (:id, :user_id, :balance, :wallet_number, :currency, :is_active, :created_at, :updated_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, w); err != nil {
		return fmt.Errorf("create wallet: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*Wallet, error) {
	w := &Wallet{}
	err := r.db.GetContext(ctx, w, `SELECT * FROM wallets WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find wallet by id: %w", err)
	}
	return w, nil
}

func (r *postgresRepository) FindByUserID(ctx context.Context, userID uuid.UUID) (*Wallet, error) {
	w := &Wallet{}
	err := r.db.GetContext(ctx, w, `SELECT * FROM wallets WHERE user_id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find wallet by user_id: %w", err)
	}
	return w, nil
}

func (r *postgresRepository) FindByWalletNumber(ctx context.Context, number string) (*Wallet, error) {
	w := &Wallet{}
	err := r.db.GetContext(ctx, w, `SELECT * FROM wallets WHERE wallet_number = $1`, number)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find wallet by number: %w", err)
	}
	return w, nil
}

// UpdateBalance adjusts the wallet balance by delta within a transaction.
// delta can be negative (debit) or positive (credit).
// Raises ErrInsufficientBalance if the resulting balance would be negative.
func (r *postgresRepository) UpdateBalance(ctx context.Context, tx *sqlx.Tx, walletID uuid.UUID, delta float64) error {
	query := `
		UPDATE wallets
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2 AND (balance + $1) >= 0
	`
	res, err := tx.ExecContext(ctx, query, delta, walletID)
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInsufficientBalance
	}
	return nil
}

func (r *postgresRepository) ExistsByUserID(ctx context.Context, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM wallets WHERE user_id = $1)`, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check wallet exists: %w", err)
	}
	return exists, nil
}

func (r *postgresRepository) TotalSentByUserID(ctx context.Context, userID uuid.UUID) (float64, error) {
	var total sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE sender_id = $1 AND status = 'completed'`,
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total sent: %w", err)
	}
	return total.Float64, nil
}

func (r *postgresRepository) TotalReceivedByUserID(ctx context.Context, userID uuid.UUID) (float64, error) {
	var total sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE receiver_id = $1 AND status = 'completed'`,
		userID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total received: %w", err)
	}
	return total.Float64, nil
}

func (r *postgresRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}
