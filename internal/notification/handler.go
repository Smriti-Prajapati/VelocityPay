package notification

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes HTTP endpoints for notifications.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs a notification Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// List godoc
// GET /api/notifications
func (h *Handler) List(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	resp, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("list notifications failed", zap.Error(err))
		response.InternalError(c, "could not fetch notifications")
		return
	}

	response.OK(c, resp)
}

// MarkRead godoc
// PUT /api/notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid notification id")
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), userID, notificationID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "notification not found")
			return
		}
		response.InternalError(c, "could not mark notification as read")
		return
	}

	response.OK(c, gin.H{"message": "notification marked as read"})
}

// MarkAllRead godoc
// PUT /api/notifications/read-all
func (h *Handler) MarkAllRead(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	if err := h.svc.MarkAllRead(c.Request.Context(), userID); err != nil {
		response.InternalError(c, "could not mark notifications as read")
		return
	}

	response.OK(c, gin.H{"message": "all notifications marked as read"})
}

// RegisterRoutes mounts notification handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	notifGroup := api.Group("/notifications", authMiddleware)
	{
		notifGroup.GET("", h.List)
		notifGroup.PUT("/read-all", h.MarkAllRead)
		notifGroup.PUT("/:id/read", h.MarkRead)
	}
}
