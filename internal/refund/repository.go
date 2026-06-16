package refund

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrNotFound is returned when a refund record cannot be located.
var ErrNotFound = errors.New("refund not found")

// ErrAlreadyRequested is returned when a refund for the same transaction already exists.
var ErrAlreadyRequested = errors.New("a refund for this transaction already exists")

// Repository defines the data access contract for refunds.
type Repository interface {
	Create(ctx context.Context, r *Refund) error
	FindByID(ctx context.Context, id uuid.UUID) (*Refund, error)
	FindByTransactionID(ctx context.Context, txnID uuid.UUID) (*Refund, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status, reviewedBy uuid.UUID, note string) error
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Refund, error)
	ExistsByTransactionID(ctx context.Context, txnID uuid.UUID) (bool, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed refund repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, rf *Refund) error {
	query := `
		INSERT INTO refunds
			(id, transaction_id, requested_by, amount, reason, status, reviewed_by, review_note, created_at, updated_at)
		VALUES
			(:id, :transaction_id, :requested_by, :amount, :reason, :status, :reviewed_by, :review_note, :created_at, :updated_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, rf); err != nil {
		return fmt.Errorf("create refund: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*Refund, error) {
	rf := &Refund{}
	err := r.db.GetContext(ctx, rf, `SELECT * FROM refunds WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by id: %w", err)
	}
	return rf, nil
}

func (r *postgresRepository) FindByTransactionID(ctx context.Context, txnID uuid.UUID) (*Refund, error) {
	rf := &Refund{}
	err := r.db.GetContext(ctx, rf, `SELECT * FROM refunds WHERE transaction_id = $1`, txnID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by transaction_id: %w", err)
	}
	return rf, nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status Status, reviewedBy uuid.UUID, note string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refunds SET status = $1, reviewed_by = $2, review_note = $3, updated_at = NOW() WHERE id = $4`,
		status, reviewedBy, note, id,
	)
	if err != nil {
		return fmt.Errorf("update refund status: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Refund, error) {
	rows, err := r.db.QueryxContext(ctx,
		`SELECT * FROM refunds WHERE requested_by = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list refunds: %w", err)
	}
	defer rows.Close()

	var refunds []*Refund
	for rows.Next() {
		rf := &Refund{}
		if err := rows.StructScan(rf); err != nil {
			return nil, fmt.Errorf("scan refund: %w", err)
		}
		refunds = append(refunds, rf)
	}
	return refunds, rows.Err()
}

func (r *postgresRepository) ExistsByTransactionID(ctx context.Context, txnID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM refunds WHERE transaction_id = $1)`, txnID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check refund exists: %w", err)
	}
	return exists, nil
}
