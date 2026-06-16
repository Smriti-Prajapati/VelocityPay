package audit

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines persistence for audit logs.
type Repository interface {
	Create(ctx context.Context, log *AuditLog) error
	ListByUserID(ctx context.Context, userID uuid.UUID, filter ListFilter) ([]*AuditLog, int, error)
	ListAll(ctx context.Context, filter ListFilter) ([]*AuditLog, int, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed audit repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, l *AuditLog) error {
	query := `
		INSERT INTO audit_logs
			(id, user_id, action, entity_type, entity_id, ip_address, user_agent, metadata, created_at)
		VALUES
			(:id, :user_id, :action, :entity_type, :entity_id, :ip_address, :user_agent, :metadata, :created_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, l); err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID, f ListFilter) ([]*AuditLog, int, error) {
	f = normaliseFilter(f)

	countQ := `SELECT COUNT(*) FROM audit_logs WHERE user_id = $1`
	args := []interface{}{userID}

	if f.Action != "" {
		countQ += fmt.Sprintf(" AND action = $%d", len(args)+1)
		args = append(args, f.Action)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	dataQ := `SELECT * FROM audit_logs WHERE user_id = $1`
	dataArgs := []interface{}{userID}

	if f.Action != "" {
		dataQ += fmt.Sprintf(" AND action = $%d", len(dataArgs)+1)
		dataArgs = append(dataArgs, f.Action)
	}

	dataQ += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		len(dataArgs)+1, len(dataArgs)+2)
	dataArgs = append(dataArgs, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.QueryxContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		if err := rows.StructScan(l); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (r *postgresRepository) ListAll(ctx context.Context, f ListFilter) ([]*AuditLog, int, error) {
	f = normaliseFilter(f)

	countQ := `SELECT COUNT(*) FROM audit_logs`
	var countArgs []interface{}

	if f.Action != "" {
		countQ += " WHERE action = $1"
		countArgs = append(countArgs, f.Action)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count all audit logs: %w", err)
	}

	dataQ := `SELECT * FROM audit_logs`
	dataArgs := []interface{}{}

	if f.Action != "" {
		dataQ += " WHERE action = $1"
		dataArgs = append(dataArgs, f.Action)
	}

	dataQ += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		len(dataArgs)+1, len(dataArgs)+2)
	dataArgs = append(dataArgs, f.PageSize, (f.Page-1)*f.PageSize)

	rows, err := r.db.QueryxContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list all audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		l := &AuditLog{}
		if err := rows.StructScan(l); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func normaliseFilter(f ListFilter) ListFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	return f
}
