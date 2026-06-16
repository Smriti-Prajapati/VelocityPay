package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/audit"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubRepo struct {
	logs []*audit.AuditLog
}

func (r *stubRepo) Create(_ context.Context, l *audit.AuditLog) error {
	r.logs = append(r.logs, l)
	return nil
}

func (r *stubRepo) ListByUserID(_ context.Context, userID uuid.UUID, f audit.ListFilter) ([]*audit.AuditLog, int, error) {
	var result []*audit.AuditLog
	for _, l := range r.logs {
		if l.UserID == userID {
			result = append(result, l)
		}
	}
	return result, len(result), nil
}

func (r *stubRepo) ListAll(_ context.Context, _ audit.ListFilter) ([]*audit.AuditLog, int, error) {
	return r.logs, len(r.logs), nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAudit_CreateAndList(t *testing.T) {
	repo := &stubRepo{}
	ctx := context.Background()
	userID := uuid.New()

	entry := &audit.AuditLog{
		ID:         uuid.New(),
		UserID:     userID,
		Action:     audit.ActionTransferCompleted,
		EntityType: "transaction",
		EntityID:   uuid.New().String(),
		Metadata:   `{"amount":500}`,
		CreatedAt:  time.Now(),
	}

	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	logs, total, err := repo.ListByUserID(ctx, userID, audit.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Errorf("expected 1 log, got %d", total)
	}
	if logs[0].Action != audit.ActionTransferCompleted {
		t.Errorf("action mismatch: got %s", logs[0].Action)
	}
}

func TestAudit_ListAll(t *testing.T) {
	repo := &stubRepo{}
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Create(ctx, &audit.AuditLog{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			Action:    audit.ActionUserRegistered,
			CreatedAt: time.Now(),
		})
	}

	logs, total, err := repo.ListAll(ctx, audit.ListFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 5 {
		t.Errorf("expected 5 logs, got %d", total)
	}
	if len(logs) != 5 {
		t.Errorf("expected 5 logs returned, got %d", len(logs))
	}
}

func TestAudit_UserIsolation(t *testing.T) {
	repo := &stubRepo{}
	ctx := context.Background()

	userA := uuid.New()
	userB := uuid.New()

	for i := 0; i < 3; i++ {
		_ = repo.Create(ctx, &audit.AuditLog{
			ID: uuid.New(), UserID: userA,
			Action:    audit.ActionTransferCompleted,
			CreatedAt: time.Now(),
		})
	}
	_ = repo.Create(ctx, &audit.AuditLog{
		ID: uuid.New(), UserID: userB,
		Action:    audit.ActionWalletCreated,
		CreatedAt: time.Now(),
	})

	logs, total, _ := repo.ListByUserID(ctx, userA, audit.ListFilter{Page: 1, PageSize: 20})
	if total != 3 {
		t.Errorf("expected 3 logs for userA, got %d", total)
	}
	for _, l := range logs {
		if l.UserID != userA {
			t.Error("got log from wrong user")
		}
	}
}

func TestAudit_ActionConstants(t *testing.T) {
	actions := []audit.Action{
		audit.ActionUserRegistered,
		audit.ActionUserLoggedIn,
		audit.ActionPasswordChanged,
		audit.ActionProfileUpdated,
		audit.ActionWalletCreated,
		audit.ActionMoneyAdded,
		audit.ActionTransferInitiated,
		audit.ActionTransferCompleted,
		audit.ActionTransferFailed,
		audit.ActionRefundRequested,
		audit.ActionRefundCompleted,
		audit.ActionRefundRejected,
		audit.ActionFraudAlertCreated,
		audit.ActionFraudAlertReviewed,
	}
	for _, a := range actions {
		if a == "" {
			t.Errorf("action constant must not be empty")
		}
	}
}

func TestAudit_MetadataStored(t *testing.T) {
	repo := &stubRepo{}
	ctx := context.Background()
	userID := uuid.New()

	_ = repo.Create(ctx, &audit.AuditLog{
		ID:         uuid.New(),
		UserID:     userID,
		Action:     audit.ActionTransferCompleted,
		EntityType: "transaction",
		EntityID:   "tx-123",
		Metadata:   `{"amount":1500,"receiver_id":"abc"}`,
		CreatedAt:  time.Now(),
	})

	logs, _, _ := repo.ListByUserID(ctx, userID, audit.ListFilter{Page: 1, PageSize: 20})
	if len(logs) == 0 {
		t.Fatal("no logs found")
	}
	if logs[0].Metadata == "" || logs[0].Metadata == "{}" {
		t.Error("metadata should be stored")
	}
	if logs[0].EntityID != "tx-123" {
		t.Errorf("entity_id mismatch: got %s", logs[0].EntityID)
	}
}

func TestAudit_LogRequestIPAndUserAgent(t *testing.T) {
	req := audit.LogRequest{
		UserID:     uuid.New(),
		Action:     audit.ActionUserLoggedIn,
		EntityType: "user",
		EntityID:   uuid.New().String(),
		IPAddress:  "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		Metadata:   map[string]interface{}{"method": "email"},
	}

	if req.IPAddress != "192.168.1.1" {
		t.Error("IP address not set")
	}
	if req.UserAgent != "Mozilla/5.0" {
		t.Error("user agent not set")
	}
	if req.Action != audit.ActionUserLoggedIn {
		t.Error("action mismatch")
	}
}
