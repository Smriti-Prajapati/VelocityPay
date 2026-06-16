package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Repository defines all analytics queries.
type Repository interface {
	GetUserStats(ctx context.Context, userID uuid.UUID) (*TransactionStats, error)
	GetMonthlySpend(ctx context.Context, userID uuid.UUID, months int) ([]*MonthlySpend, error)
	GetDailyVolume(ctx context.Context, userID uuid.UUID, days int) ([]*DailyVolume, error)
	GetPlatformStats(ctx context.Context) (*PlatformStats, error)
	GetTopSenders(ctx context.Context, limit int) ([]*TopSender, error)
	GetWalletGrowth(ctx context.Context, days int) ([]*WalletGrowth, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed analytics repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (*TransactionStats, error) {
	query := `
		SELECT
			COUNT(*)                                                       AS total_transactions,
			COUNT(*) FILTER (WHERE status = 'completed')                  AS completed_count,
			COUNT(*) FILTER (WHERE status = 'failed')                     AS failed_count,
			COALESCE(SUM(amount) FILTER (WHERE sender_id = $1 AND status = 'completed'), 0)   AS total_amount_sent,
			COALESCE(SUM(amount) FILTER (WHERE receiver_id = $1 AND status = 'completed'), 0) AS total_amount_received,
			COALESCE(AVG(amount) FILTER (WHERE status = 'completed'), 0)  AS avg_amount,
			COALESCE(MAX(amount) FILTER (WHERE status = 'completed'), 0)  AS largest_transaction
		FROM transactions
		WHERE sender_id = $1 OR receiver_id = $1
	`
	stats := &TransactionStats{}
	if err := r.db.GetContext(ctx, stats, query, userID); err != nil {
		return nil, fmt.Errorf("get user stats: %w", err)
	}
	return stats, nil
}

func (r *postgresRepository) GetMonthlySpend(ctx context.Context, userID uuid.UUID, months int) ([]*MonthlySpend, error) {
	query := `
		SELECT
			EXTRACT(YEAR  FROM created_at)::INT AS year,
			EXTRACT(MONTH FROM created_at)::INT AS month,
			COALESCE(SUM(amount), 0)            AS amount,
			COUNT(*)                            AS count
		FROM transactions
		WHERE sender_id = $1
		  AND status = 'completed'
		  AND created_at >= NOW() - ($2 || ' months')::INTERVAL
		GROUP BY year, month
		ORDER BY year DESC, month DESC
	`
	rows, err := r.db.QueryxContext(ctx, query, userID, months)
	if err != nil {
		return nil, fmt.Errorf("monthly spend: %w", err)
	}
	defer rows.Close()

	var result []*MonthlySpend
	for rows.Next() {
		m := &MonthlySpend{}
		if err := rows.StructScan(m); err != nil {
			return nil, fmt.Errorf("scan monthly spend: %w", err)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *postgresRepository) GetDailyVolume(ctx context.Context, userID uuid.UUID, days int) ([]*DailyVolume, error) {
	query := `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM-DD')                                     AS date,
			COALESCE(SUM(amount) FILTER (WHERE sender_id = $1), 0)                AS sent,
			COALESCE(SUM(amount) FILTER (WHERE receiver_id = $1), 0)              AS received
		FROM transactions
		WHERE (sender_id = $1 OR receiver_id = $1)
		  AND status = 'completed'
		  AND created_at >= NOW() - ($2 || ' days')::INTERVAL
		GROUP BY date
		ORDER BY date DESC
	`
	rows, err := r.db.QueryxContext(ctx, query, userID, days)
	if err != nil {
		return nil, fmt.Errorf("daily volume: %w", err)
	}
	defer rows.Close()

	var result []*DailyVolume
	for rows.Next() {
		d := &DailyVolume{}
		if err := rows.StructScan(d); err != nil {
			return nil, fmt.Errorf("scan daily volume: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (r *postgresRepository) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	query := `
		SELECT
			(SELECT COUNT(*) FROM users)        AS total_users,
			(SELECT COUNT(*) FROM wallets)      AS total_wallets,
			(SELECT COUNT(*) FROM transactions WHERE status = 'completed') AS total_transactions,
			(SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE status = 'completed') AS total_volume,
			(SELECT COUNT(*) FROM transactions
			  WHERE status = 'completed'
			    AND created_at >= CURRENT_DATE) AS today_transactions,
			(SELECT COALESCE(SUM(amount), 0) FROM transactions
			  WHERE status = 'completed'
			    AND created_at >= CURRENT_DATE) AS today_volume,
			(SELECT COUNT(DISTINCT sender_id) FROM transactions
			  WHERE created_at >= CURRENT_DATE) AS active_users_today
	`
	stats := &PlatformStats{}
	if err := r.db.GetContext(ctx, stats, query); err != nil {
		return nil, fmt.Errorf("platform stats: %w", err)
	}
	stats.GeneratedAt = time.Now().UTC()
	return stats, nil
}

func (r *postgresRepository) GetTopSenders(ctx context.Context, limit int) ([]*TopSender, error) {
	query := `
		SELECT
			t.sender_id                    AS user_id,
			u.name                         AS name,
			COALESCE(SUM(t.amount), 0)     AS total,
			COUNT(*)                       AS count
		FROM transactions t
		JOIN users u ON u.id = t.sender_id
		WHERE t.status = 'completed'
		GROUP BY t.sender_id, u.name
		ORDER BY total DESC
		LIMIT $1
	`
	rows, err := r.db.QueryxContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("top senders: %w", err)
	}
	defer rows.Close()

	var result []*TopSender
	for rows.Next() {
		s := &TopSender{}
		if err := rows.StructScan(s); err != nil {
			return nil, fmt.Errorf("scan top sender: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *postgresRepository) GetWalletGrowth(ctx context.Context, days int) ([]*WalletGrowth, error) {
	query := `
		SELECT
			TO_CHAR(created_at, 'YYYY-MM-DD') AS date,
			COUNT(*)                          AS count
		FROM wallets
		WHERE created_at >= NOW() - ($1 || ' days')::INTERVAL
		GROUP BY date
		ORDER BY date ASC
	`
	rows, err := r.db.QueryxContext(ctx, query, days)
	if err != nil {
		return nil, fmt.Errorf("wallet growth: %w", err)
	}
	defer rows.Close()

	var result []*WalletGrowth
	for rows.Next() {
		w := &WalletGrowth{}
		if err := rows.StructScan(w); err != nil {
			return nil, fmt.Errorf("scan wallet growth: %w", err)
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
