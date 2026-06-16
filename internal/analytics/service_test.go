package analytics_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/analytics"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubRepo struct{}

func (r *stubRepo) GetUserStats(_ context.Context, _ uuid.UUID) (*analytics.TransactionStats, error) {
	return &analytics.TransactionStats{
		TotalTransactions:        10,
		CompletedCount:           8,
		FailedCount:              2,
		TotalAmountSent:          5000.00,
		TotalAmountReceived:      3000.00,
		AverageTransactionAmount: 625.00,
		LargestTransaction:       2000.00,
	}, nil
}

func (r *stubRepo) GetMonthlySpend(_ context.Context, _ uuid.UUID, _ int) ([]*analytics.MonthlySpend, error) {
	return []*analytics.MonthlySpend{
		{Year: 2026, Month: 6, Amount: 2500.00, Count: 5},
		{Year: 2026, Month: 5, Amount: 1800.00, Count: 3},
	}, nil
}

func (r *stubRepo) GetDailyVolume(_ context.Context, _ uuid.UUID, _ int) ([]*analytics.DailyVolume, error) {
	return []*analytics.DailyVolume{
		{Date: "2026-06-15", Sent: 500.00, Received: 250.00},
		{Date: "2026-06-14", Sent: 100.00, Received: 750.00},
	}, nil
}

func (r *stubRepo) GetPlatformStats(_ context.Context) (*analytics.PlatformStats, error) {
	return &analytics.PlatformStats{
		TotalUsers:        3,
		TotalWallets:      3,
		TotalTransactions: 15,
		TotalVolume:       12500.00,
		TodayTransactions: 5,
		TodayVolume:       1250.00,
		ActiveUsersToday:  2,
	}, nil
}

func (r *stubRepo) GetTopSenders(_ context.Context, limit int) ([]*analytics.TopSender, error) {
	senders := []*analytics.TopSender{
		{UserID: uuid.New().String(), Name: "Alice", Total: 5000.00, Count: 8},
		{UserID: uuid.New().String(), Name: "Bob", Total: 1200.00, Count: 3},
	}
	if len(senders) > limit {
		return senders[:limit], nil
	}
	return senders, nil
}

func (r *stubRepo) GetWalletGrowth(_ context.Context, _ int) ([]*analytics.WalletGrowth, error) {
	return []*analytics.WalletGrowth{
		{Date: "2026-06-15", Count: 2},
		{Date: "2026-06-14", Count: 1},
	}, nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAnalytics_UserStats(t *testing.T) {
	repo := &stubRepo{}
	stats, err := repo.GetUserStats(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.TotalTransactions != 10 {
		t.Errorf("expected 10 transactions, got %d", stats.TotalTransactions)
	}
	if stats.CompletedCount != 8 {
		t.Errorf("expected 8 completed, got %d", stats.CompletedCount)
	}
	if stats.TotalAmountSent != 5000.00 {
		t.Errorf("expected 5000 sent, got %.2f", stats.TotalAmountSent)
	}
}

func TestAnalytics_MonthlySpend(t *testing.T) {
	repo := &stubRepo{}
	data, err := repo.GetMonthlySpend(context.Background(), uuid.New(), 6)
	if err != nil {
		t.Fatalf("monthly spend: %v", err)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 months, got %d", len(data))
	}
	if data[0].Amount != 2500.00 {
		t.Errorf("expected 2500 for June, got %.2f", data[0].Amount)
	}
}

func TestAnalytics_DailyVolume(t *testing.T) {
	repo := &stubRepo{}
	data, err := repo.GetDailyVolume(context.Background(), uuid.New(), 30)
	if err != nil {
		t.Fatalf("daily volume: %v", err)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 days, got %d", len(data))
	}
	if data[0].Sent != 500.00 {
		t.Errorf("expected 500 sent on June 15, got %.2f", data[0].Sent)
	}
}

func TestAnalytics_PlatformStats(t *testing.T) {
	repo := &stubRepo{}
	stats, err := repo.GetPlatformStats(context.Background())
	if err != nil {
		t.Fatalf("platform stats: %v", err)
	}
	if stats.TotalUsers != 3 {
		t.Errorf("expected 3 users, got %d", stats.TotalUsers)
	}
	if stats.TotalVolume != 12500.00 {
		t.Errorf("expected 12500 volume, got %.2f", stats.TotalVolume)
	}
	if stats.ActiveUsersToday != 2 {
		t.Errorf("expected 2 active users today, got %d", stats.ActiveUsersToday)
	}
}

func TestAnalytics_TopSenders(t *testing.T) {
	repo := &stubRepo{}

	senders, err := repo.GetTopSenders(context.Background(), 10)
	if err != nil {
		t.Fatalf("top senders: %v", err)
	}
	if len(senders) == 0 {
		t.Error("expected at least one top sender")
	}
	if senders[0].Name != "Alice" {
		t.Errorf("expected Alice as top sender, got %s", senders[0].Name)
	}
	if senders[0].Total != 5000.00 {
		t.Errorf("expected 5000 total for Alice, got %.2f", senders[0].Total)
	}
}

func TestAnalytics_TopSenders_LimitRespected(t *testing.T) {
	repo := &stubRepo{}
	senders, err := repo.GetTopSenders(context.Background(), 1)
	if err != nil {
		t.Fatalf("top senders: %v", err)
	}
	if len(senders) > 1 {
		t.Errorf("expected at most 1 sender, got %d", len(senders))
	}
}

func TestAnalytics_WalletGrowth(t *testing.T) {
	repo := &stubRepo{}
	data, err := repo.GetWalletGrowth(context.Background(), 30)
	if err != nil {
		t.Fatalf("wallet growth: %v", err)
	}
	if len(data) != 2 {
		t.Errorf("expected 2 days of growth data, got %d", len(data))
	}
	if data[0].Date != "2026-06-15" {
		t.Errorf("expected 2026-06-15, got %s", data[0].Date)
	}
}

func TestAnalytics_DomainFields(t *testing.T) {
	summary := analytics.DashboardSummary{
		UserID:            uuid.New().String(),
		TotalSent:         1000,
		TotalReceived:     500,
		TotalTransactions: 5,
		WalletBalance:     750,
		Currency:          "INR",
	}
	if summary.Currency != "INR" {
		t.Error("currency should be INR")
	}
	if summary.WalletBalance != 750 {
		t.Error("balance mismatch")
	}
}
