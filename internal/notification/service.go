package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles notification creation and retrieval.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService wires up the notification service.
func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Send creates and persists a notification for a user.
func (s *Service) Send(ctx context.Context, userID uuid.UUID, ntype Type, title, message, relatedID string) error {
	n := &Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      ntype,
		Title:     title,
		Message:   message,
		IsRead:    false,
		RelatedID: relatedID,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return fmt.Errorf("send notification: %w", err)
	}

	s.log.Debug("notification sent",
		zap.String("user_id", userID.String()),
		zap.String("type", string(ntype)),
		zap.String("title", title),
	)
	return nil
}

// List returns all notifications for a user with unread count.
func (s *Service) List(ctx context.Context, userID uuid.UUID) (*ListResponse, error) {
	notifications, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}

	unread, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count unread: %w", err)
	}

	if notifications == nil {
		notifications = []*Notification{}
	}

	return &ListResponse{
		Notifications: notifications,
		Total:         len(notifications),
		UnreadCount:   unread,
	}, nil
}

// MarkRead marks a single notification as read.
func (s *Service) MarkRead(ctx context.Context, userID uuid.UUID, notificationID uuid.UUID) error {
	return s.repo.MarkRead(ctx, notificationID, userID)
}

// MarkAllRead marks every notification for the user as read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.repo.MarkAllRead(ctx, userID)
}
