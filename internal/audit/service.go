package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles audit log creation and retrieval.
type Service struct {
	repo Repository
	log  *zap.Logger
}

// NewService wires up the audit service.
func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Log records an audit event. Non-fatal — errors are logged but never returned
// to callers so a failed audit write never breaks a business operation.
func (s *Service) Log(ctx context.Context, req LogRequest) {
	meta := "{}"
	if len(req.Metadata) > 0 {
		if b, err := json.Marshal(req.Metadata); err == nil {
			meta = string(b)
		}
	}

	entry := &AuditLog{
		ID:         uuid.New(),
		UserID:     req.UserID,
		Action:     req.Action,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
		Metadata:   meta,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		s.log.Error("failed to write audit log",
			zap.String("action", string(req.Action)),
			zap.String("user_id", req.UserID.String()),
			zap.Error(err),
		)
		return
	}

	s.log.Debug("audit log written",
		zap.String("action", string(req.Action)),
		zap.String("entity_type", req.EntityType),
		zap.String("entity_id", req.EntityID),
	)
}

// GetMyLogs returns paginated audit logs for the requesting user.
func (s *Service) GetMyLogs(ctx context.Context, userID uuid.UUID, filter ListFilter) (*ListResponse, error) {
	logs, total, err := s.repo.ListByUserID(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("get my logs: %w", err)
	}
	return buildResponse(logs, total, filter), nil
}

// GetAllLogs returns paginated platform-wide audit logs (admin only).
func (s *Service) GetAllLogs(ctx context.Context, filter ListFilter) (*ListResponse, error) {
	logs, total, err := s.repo.ListAll(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get all logs: %w", err)
	}
	return buildResponse(logs, total, filter), nil
}

func buildResponse(logs []*AuditLog, total int, f ListFilter) *ListResponse {
	if logs == nil {
		logs = []*AuditLog{}
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	totalPages := int(math.Ceil(float64(total) / float64(f.PageSize)))
	return &ListResponse{
		Logs:       logs,
		Total:      total,
		Page:       f.Page,
		PageSize:   f.PageSize,
		TotalPages: totalPages,
	}
}
