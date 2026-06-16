package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrNotFound is returned when a transaction cannot be located.
var ErrNotFound = errors.New("transaction not found")

// ErrDuplicateIdempotencyKey is returned when the same key was already processed.
var ErrDuplicateIdempotencyKey = errors.New("duplicate transaction: idempotency key already used")

// Repository defines the data access contract for transactions.
type Repository interface {
	Create(ctx context.Context, tx *sqlx.Tx, t *Transaction) error
	FindByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	FindByIdempotencyKey(ctx context.Context, key string) (*Transaction, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status, failureReason string) error
	ListByUserID(ctx context.Context, userID uuid.UUID, filter HistoryFilter) ([]*Transaction, int, error)
	BeginTx(ctx context.Context) (*sqlx.Tx, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed transaction repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, tx *sqlx.Tx, t *Transaction) error {
	query := `
		INSERT INTO transactions
			(id, sender_id, receiver_id, amount, transaction_type, status, notes, idempotency_key, failure_reason, created_at, updated_at)
		VALUES
			(:id, :sender_id, :receiver_id, :amount, :transaction_type, :status, :notes, :idempotency_key, :failure_reason, :created_at, :updated_at)
	`
	stmt, err := tx.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("prepare create transaction: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.ExecContext(ctx, t); err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*Transaction, error) {
	t := &Transaction{}
	err := r.db.GetContext(ctx, t, `SELECT * FROM transactions WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transaction by id: %w", err)
	}
	return t, nil
}

func (r *postgresRepository) FindByIdempotencyKey(ctx context.Context, key string) (*Transaction, error) {
	t := &Transaction{}
	err := r.db.GetContext(ctx, t,
		`SELECT * FROM transactions WHERE idempotency_key = $1`, key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find transaction by idempotency key: %w", err)
	}
	return t, nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status Status, failureReason string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE transactions SET status = $1, failure_reason = $2, updated_at = NOW() WHERE id = $3`,
		status, failureReason, id,
	)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID, f HistoryFilter) ([]*Transaction, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	offset := (f.Page - 1) * f.PageSize

	// Count query
	countQuery := `SELECT COUNT(*) FROM transactions WHERE (sender_id = $1 OR receiver_id = $1)`
	args := []interface{}{userID}

	if f.Status != "" {
		countQuery += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, f.Status)
	}
	if f.Type != "" {
		countQuery += fmt.Sprintf(" AND transaction_type = $%d", len(args)+1)
		args = append(args, f.Type)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Data query
	dataQuery := `SELECT * FROM transactions WHERE (sender_id = $1 OR receiver_id = $1)`
	dataArgs := []interface{}{userID}

	if f.Status != "" {
		dataQuery += fmt.Sprintf(" AND status = $%d", len(dataArgs)+1)
		dataArgs = append(dataArgs, f.Status)
	}
	if f.Type != "" {
		dataQuery += fmt.Sprintf(" AND transaction_type = $%d", len(dataArgs)+1)
		dataArgs = append(dataArgs, f.Type)
	}

	dataQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(dataArgs)+1, len(dataArgs)+2)
	dataArgs = append(dataArgs, f.PageSize, offset)

	rows, err := r.db.QueryxContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txns []*Transaction
	for rows.Next() {
		t := &Transaction{}
		if err := rows.StructScan(t); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		txns = append(txns, t)
	}

	return txns, total, rows.Err()
}

func (r *postgresRepository) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return r.db.BeginTxx(ctx, nil)
}
