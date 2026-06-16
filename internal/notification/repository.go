package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrNotFound is returned when a notification cannot be located.
var ErrNotFound = errors.New("notification not found")

// Repository defines the data access contract for notifications.
type Repository interface {
	Create(ctx context.Context, n *Notification) error
	FindByID(ctx context.Context, id uuid.UUID) (*Notification, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Notification, error)
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	CountUnread(ctx context.Context, userID uuid.UUID) (int, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed notification repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, n *Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, type, title, message, is_read, related_id, created_at)
		VALUES (:id, :user_id, :type, :title, :message, :is_read, :related_id, :created_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, n); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*Notification, error) {
	n := &Notification{}
	err := r.db.GetContext(ctx, n, `SELECT * FROM notifications WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find notification: %w", err)
	}
	return n, nil
}

func (r *postgresRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*Notification, error) {
	rows, err := r.db.QueryxContext(ctx,
		`SELECT * FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		n := &Notification{}
		if err := rows.StructScan(n); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func (r *postgresRepository) MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET is_read = TRUE WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

func (r *postgresRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return count, nil
}
