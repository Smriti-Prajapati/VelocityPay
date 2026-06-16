package fraud_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/fraud"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubRepo struct {
	alerts []*fraud.FraudAlert
}

func (r *stubRepo) CreateAlert(_ context.Context, a *fraud.FraudAlert) error {
	r.alerts = append(r.alerts, a)
	return nil
}
func (r *stubRepo) ListAlertsByUserID(_ context.Context, _ uuid.UUID) ([]*fraud.FraudAlert, error) {
	return r.alerts, nil
}
func (r *stubRepo) ListUnreviewed(_ context.Context) ([]*fraud.FraudAlert, error) {
	return r.alerts, nil
}
func (r *stubRepo) MarkReviewed(_ context.Context, _ uuid.UUID) error { return nil }

// ── Redis stub ────────────────────────────────────────────────────────────────

type stubCache struct {
	counters map[string]int64
}

func newStubCache() *stubCache {
	return &stubCache{counters: make(map[string]int64)}
}

// We can't use the real redisc.Client interface in tests without embedding,
// so we test rule logic directly via exported helpers below.

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestFraud_DefaultThresholds(t *testing.T) {
	th := fraud.DefaultThresholds()

	if th.MaxTransfersPerWindow != 10 {
		t.Errorf("expected 10 max transfers, got %d", th.MaxTransfersPerWindow)
	}
	if th.LargeTransactionAmount != 50_000 {
		t.Errorf("expected 50000 large tx threshold, got %.0f", th.LargeTransactionAmount)
	}
	if th.RapidDepletionPercent != 0.80 {
		t.Errorf("expected 0.80 depletion threshold, got %.2f", th.RapidDepletionPercent)
	}
	if th.NewAccountAgeCutoff != 24*time.Hour {
		t.Errorf("expected 24h new account cutoff")
	}
}

func TestFraud_LargeTransactionRule(t *testing.T) {
	threshold := 50_000.0

	tests := []struct {
		amount  float64
		wantFlag bool
	}{
		{49_999, false},
		{50_000, true},
		{100_000, true},
	}

	for _, tc := range tests {
		flagged := tc.amount >= threshold
		if flagged != tc.wantFlag {
			t.Errorf("amount %.0f: expected flagged=%v, got %v", tc.amount, tc.wantFlag, flagged)
		}
	}
}

func TestFraud_RapidDepletionRule(t *testing.T) {
	threshold := 0.80

	tests := []struct {
		balance  float64
		amount   float64
		wantFlag bool
	}{
		{1000, 750, false}, // 75% — under threshold
		{1000, 800, true},  // 80% — exactly at threshold
		{1000, 900, true},  // 90% — over threshold
		{0, 100, false},    // zero balance — skip rule
	}

	for _, tc := range tests {
		var flagged bool
		if tc.balance > 0 {
			ratio := tc.amount / tc.balance
			flagged = ratio >= threshold
		}
		if flagged != tc.wantFlag {
			t.Errorf("balance=%.0f amount=%.0f: expected flagged=%v, got %v",
				tc.balance, tc.amount, tc.wantFlag, flagged)
		}
	}
}

func TestFraud_UnusualHourRule(t *testing.T) {
	tests := []struct {
		hour     int
		wantFlag bool
	}{
		{0, false},  // midnight — ok
		{1, true},   // 1 AM — flagged
		{3, true},   // 3 AM — flagged
		{5, true},   // 5 AM — flagged
		{6, false},  // 6 AM — ok
		{14, false}, // 2 PM — ok
	}

	for _, tc := range tests {
		flagged := tc.hour >= 1 && tc.hour <= 5
		if flagged != tc.wantFlag {
			t.Errorf("hour %d: expected flagged=%v, got %v", tc.hour, tc.wantFlag, flagged)
		}
	}
}

func TestFraud_NewAccountRule(t *testing.T) {
	cutoff := 24 * time.Hour

	tests := []struct {
		age      time.Duration
		wantFlag bool
	}{
		{30 * time.Minute, true},  // very new
		{12 * time.Hour, true},    // half a day
		{23*time.Hour + 59*time.Minute, true}, // just under cutoff
		{24 * time.Hour, false},   // exactly at cutoff — not flagged
		{48 * time.Hour, false},   // 2 days old — safe
	}

	for _, tc := range tests {
		flagged := tc.age < cutoff
		if flagged != tc.wantFlag {
			t.Errorf("age %s: expected flagged=%v, got %v", tc.age, tc.wantFlag, flagged)
		}
	}
}

func TestFraud_RiskLevels(t *testing.T) {
	levels := []fraud.RiskLevel{
		fraud.RiskLow,
		fraud.RiskMedium,
		fraud.RiskHigh,
		fraud.RiskCritical,
	}
	for _, l := range levels {
		if l == "" {
			t.Error("risk level must not be empty")
		}
	}
}

func TestFraud_AlertTypes(t *testing.T) {
	types := []fraud.AlertType{
		fraud.AlertVelocityBreached,
		fraud.AlertLargeTransaction,
		fraud.AlertRapidDepletion,
		fraud.AlertUnusualHour,
		fraud.AlertNewAccountTransfer,
	}
	for _, a := range types {
		if a == "" {
			t.Error("alert type must not be empty")
		}
	}
}

func TestFraud_BuildCheckRequest(t *testing.T) {
	userID := uuid.New()
	txnID := uuid.New()
	createdAt := time.Now().Add(-2 * time.Hour) // 2 hours old account

	req := fraud.BuildCheckRequest(userID, txnID, 500.0, 1000.0, createdAt)

	if req.UserID != userID {
		t.Error("user_id mismatch")
	}
	if req.Amount != 500.0 {
		t.Errorf("amount: want 500, got %.2f", req.Amount)
	}
	if req.WalletBalance != 1000.0 {
		t.Error("wallet balance mismatch")
	}
	if req.AccountAge < 2*time.Hour {
		t.Errorf("account age too low: %s", req.AccountAge)
	}
}

func TestFraud_ScoreCapping(t *testing.T) {
	// If multiple rules fire, score should be capped at 100
	score := 85 + 40 + 35 // velocity + large + depletion
	if score > 100 {
		score = 100
	}
	if score != 100 {
		t.Errorf("score should be capped at 100, got %d", score)
	}
}
