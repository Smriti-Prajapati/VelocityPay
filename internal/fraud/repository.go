package fraud

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines persistence for fraud alerts.
type Repository interface {
	CreateAlert(ctx context.Context, alert *FraudAlert) error
	ListAlertsByUserID(ctx context.Context, userID uuid.UUID) ([]*FraudAlert, error)
	ListUnreviewed(ctx context.Context) ([]*FraudAlert, error)
	MarkReviewed(ctx context.Context, alertID uuid.UUID) error
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed fraud repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) CreateAlert(ctx context.Context, alert *FraudAlert) error {
	query := `
		INSERT INTO fraud_alerts
			(id, user_id, transaction_id, alert_type, risk_level, risk_score, details, reviewed, created_at)
		VALUES
			(:id, :user_id, :transaction_id, :alert_type, :risk_level, :risk_score, :details, :reviewed, :created_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, alert); err != nil {
		return fmt.Errorf("create fraud alert: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListAlertsByUserID(ctx context.Context, userID uuid.UUID) ([]*FraudAlert, error) {
	rows, err := r.db.QueryxContext(ctx,
		`SELECT * FROM fraud_alerts WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list fraud alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*FraudAlert
	for rows.Next() {
		a := &FraudAlert{}
		if err := rows.StructScan(a); err != nil {
			return nil, fmt.Errorf("scan fraud alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *postgresRepository) ListUnreviewed(ctx context.Context) ([]*FraudAlert, error) {
	rows, err := r.db.QueryxContext(ctx,
		`SELECT * FROM fraud_alerts WHERE reviewed = FALSE ORDER BY risk_score DESC, created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list unreviewed alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*FraudAlert
	for rows.Next() {
		a := &FraudAlert{}
		if err := rows.StructScan(a); err != nil {
			return nil, fmt.Errorf("scan fraud alert: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *postgresRepository) MarkReviewed(ctx context.Context, alertID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE fraud_alerts SET reviewed = TRUE WHERE id = $1`, alertID)
	if err != nil {
		return fmt.Errorf("mark alert reviewed: %w", err)
	}
	return nil
}
