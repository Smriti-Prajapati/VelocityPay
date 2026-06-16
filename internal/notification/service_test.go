package notification_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/notification"
)

// ── Repository stub ───────────────────────────────────────────────────────────

type stubRepo struct {
	notifications map[uuid.UUID]*notification.Notification
}

func newStub() *stubRepo {
	return &stubRepo{notifications: make(map[uuid.UUID]*notification.Notification)}
}

func (r *stubRepo) Create(_ context.Context, n *notification.Notification) error {
	r.notifications[n.ID] = n
	return nil
}

func (r *stubRepo) FindByID(_ context.Context, id uuid.UUID) (*notification.Notification, error) {
	n, ok := r.notifications[id]
	if !ok {
		return nil, notification.ErrNotFound
	}
	return n, nil
}

func (r *stubRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]*notification.Notification, error) {
	var result []*notification.Notification
	for _, n := range r.notifications {
		if n.UserID == userID {
			result = append(result, n)
		}
	}
	return result, nil
}

func (r *stubRepo) MarkRead(_ context.Context, id uuid.UUID, userID uuid.UUID) error {
	n, ok := r.notifications[id]
	if !ok {
		return notification.ErrNotFound
	}
	if n.UserID != userID {
		return notification.ErrNotFound
	}
	n.IsRead = true
	return nil
}

func (r *stubRepo) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	for _, n := range r.notifications {
		if n.UserID == userID {
			n.IsRead = true
		}
	}
	return nil
}

func (r *stubRepo) CountUnread(_ context.Context, userID uuid.UUID) (int, error) {
	count := 0
	for _, n := range r.notifications {
		if n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestNotification_CreateAndFind(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	n := &notification.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      notification.TypeTransactionSent,
		Title:     "Transfer Successful",
		Message:   "You sent ₹250.00",
		IsRead:    false,
		CreatedAt: time.Now(),
	}

	if err := repo.Create(ctx, n); err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := repo.FindByID(ctx, n.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Title != n.Title {
		t.Errorf("title mismatch: want %q, got %q", n.Title, found.Title)
	}
	if found.IsRead {
		t.Error("new notification should be unread")
	}
}

func TestNotification_MarkRead(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	n := &notification.Notification{
		ID: uuid.New(), UserID: userID,
		Type: notification.TypeTransactionReceived,
		Title: "Money Received", Message: "₹500",
		IsRead: false, CreatedAt: time.Now(),
	}
	_ = repo.Create(ctx, n)

	if err := repo.MarkRead(ctx, n.ID, userID); err != nil {
		t.Fatalf("mark read: %v", err)
	}

	found, _ := repo.FindByID(ctx, n.ID)
	if !found.IsRead {
		t.Error("notification should be marked as read")
	}
}

func TestNotification_MarkRead_WrongUser(t *testing.T) {
	repo := newStub()
	ctx := context.Background()

	n := &notification.Notification{
		ID: uuid.New(), UserID: uuid.New(),
		Type: notification.TypeTransactionSent,
		Title: "T", Message: "M",
		IsRead: false, CreatedAt: time.Now(),
	}
	_ = repo.Create(ctx, n)

	// Different user tries to mark it read
	err := repo.MarkRead(ctx, n.ID, uuid.New())
	if !errors.Is(err, notification.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestNotification_MarkAllRead(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 5; i++ {
		_ = repo.Create(ctx, &notification.Notification{
			ID: uuid.New(), UserID: userID,
			Type: notification.TypeTransactionSent,
			Title: "T", Message: "M",
			IsRead: false, CreatedAt: time.Now(),
		})
	}

	if err := repo.MarkAllRead(ctx, userID); err != nil {
		t.Fatalf("mark all read: %v", err)
	}

	unread, _ := repo.CountUnread(ctx, userID)
	if unread != 0 {
		t.Errorf("expected 0 unread, got %d", unread)
	}
}

func TestNotification_UnreadCount(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		_ = repo.Create(ctx, &notification.Notification{
			ID: uuid.New(), UserID: userID,
			Type: notification.TypeTransactionSent,
			Title: "T", Message: "M",
			IsRead: false, CreatedAt: time.Now(),
		})
	}
	// One already read
	_ = repo.Create(ctx, &notification.Notification{
		ID: uuid.New(), UserID: userID,
		Type: notification.TypeTransactionSent,
		Title: "T", Message: "M",
		IsRead: true, CreatedAt: time.Now(),
	})

	count, err := repo.CountUnread(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 unread, got %d", count)
	}
}

func TestNotification_ListByUser(t *testing.T) {
	repo := newStub()
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 4; i++ {
		_ = repo.Create(ctx, &notification.Notification{
			ID: uuid.New(), UserID: userID,
			Type: notification.TypeWalletCreated,
			Title: "W", Message: "M",
			IsRead: false, CreatedAt: time.Now(),
		})
	}
	// Another user's notification
	_ = repo.Create(ctx, &notification.Notification{
		ID: uuid.New(), UserID: uuid.New(),
		Type: notification.TypeWalletCreated,
		Title: "W", Message: "M",
		IsRead: false, CreatedAt: time.Now(),
	})

	results, err := repo.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Errorf("expected 4 notifications, got %d", len(results))
	}
}

func TestNotification_TypeConstants(t *testing.T) {
	types := []notification.Type{
		notification.TypeTransactionSent,
		notification.TypeTransactionReceived,
		notification.TypeTransactionFailed,
		notification.TypeRefundRequested,
		notification.TypeRefundCompleted,
		notification.TypeWalletCreated,
		notification.TypeUserWelcome,
	}
	for _, tp := range types {
		if tp == "" {
			t.Error("notification type must not be empty")
		}
	}
}
