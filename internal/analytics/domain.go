package analytics

import "time"

// DashboardSummary is the top-level overview returned for a user's dashboard.
type DashboardSummary struct {
	UserID          string  `json:"user_id"`
	TotalSent       float64 `json:"total_sent"`
	TotalReceived   float64 `json:"total_received"`
	TotalTransactions int   `json:"total_transactions"`
	WalletBalance   float64 `json:"wallet_balance"`
	Currency        string  `json:"currency"`
}

// MonthlySpend represents aggregated spending for a single calendar month.
type MonthlySpend struct {
	Year   int     `db:"year"  json:"year"`
	Month  int     `db:"month" json:"month"`
	Amount float64 `db:"amount" json:"amount"`
	Count  int     `db:"count"  json:"count"`
}

// DailyVolume represents transaction volume for a single day.
type DailyVolume struct {
	Date   string  `db:"date"   json:"date"`
	Sent   float64 `db:"sent"   json:"sent"`
	Received float64 `db:"received" json:"received"`
}

// TransactionStats holds overall transaction metrics for a user.
type TransactionStats struct {
	TotalTransactions   int     `db:"total_transactions"   json:"total_transactions"`
	CompletedCount      int     `db:"completed_count"      json:"completed_count"`
	FailedCount         int     `db:"failed_count"         json:"failed_count"`
	TotalAmountSent     float64 `db:"total_amount_sent"    json:"total_amount_sent"`
	TotalAmountReceived float64 `db:"total_amount_received" json:"total_amount_received"`
	AverageTransactionAmount float64 `db:"avg_amount"      json:"average_transaction_amount"`
	LargestTransaction  float64 `db:"largest_transaction"  json:"largest_transaction"`
}

// PlatformStats is the admin-level platform-wide analytics.
type PlatformStats struct {
	TotalUsers         int     `db:"total_users"          json:"total_users"`
	TotalWallets       int     `db:"total_wallets"        json:"total_wallets"`
	TotalTransactions  int     `db:"total_transactions"   json:"total_transactions"`
	TotalVolume        float64 `db:"total_volume"         json:"total_volume"`
	TodayTransactions  int     `db:"today_transactions"   json:"today_transactions"`
	TodayVolume        float64 `db:"today_volume"         json:"today_volume"`
	ActiveUsersToday   int     `db:"active_users_today"   json:"active_users_today"`
	GeneratedAt        time.Time                              `json:"generated_at"`
}

// TopSender is a user ranked by amount sent.
type TopSender struct {
	UserID string  `db:"user_id"  json:"user_id"`
	Name   string  `db:"name"     json:"name"`
	Total  float64 `db:"total"    json:"total"`
	Count  int     `db:"count"    json:"count"`
}

// WalletGrowth tracks the cumulative wallet count over time.
type WalletGrowth struct {
	Date  string `db:"date"  json:"date"`
	Count int    `db:"count" json:"count"`
}
