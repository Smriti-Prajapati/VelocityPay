package fraud

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes fraud alert endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs a fraud Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// MyAlerts godoc
// GET /api/fraud/alerts/me
func (h *Handler) MyAlerts(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	alerts, err := h.svc.GetAlertsByUser(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("get fraud alerts failed", zap.Error(err))
		response.InternalError(c, "could not fetch fraud alerts")
		return
	}

	response.OK(c, gin.H{"alerts": alerts, "total": len(alerts)})
}

// Unreviewed godoc
// GET /api/fraud/alerts/unreviewed  (admin)
func (h *Handler) Unreviewed(c *gin.Context) {
	alerts, err := h.svc.GetUnreviewed(c.Request.Context())
	if err != nil {
		response.InternalError(c, "could not fetch unreviewed alerts")
		return
	}

	response.OK(c, gin.H{"alerts": alerts, "total": len(alerts)})
}

// MarkReviewed godoc
// PUT /api/fraud/alerts/:id/review
func (h *Handler) MarkReviewed(c *gin.Context) {
	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid alert id")
		return
	}

	if err := h.svc.MarkReviewed(c.Request.Context(), alertID); err != nil {
		response.InternalError(c, "could not mark alert as reviewed")
		return
	}

	response.OK(c, gin.H{"message": "alert marked as reviewed"})
}

// RegisterRoutes mounts fraud handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	fraudGroup := api.Group("/fraud", authMiddleware)
	{
		fraudGroup.GET("/alerts/me", h.MyAlerts)
		fraudGroup.GET("/alerts/unreviewed", h.Unreviewed)
		fraudGroup.PUT("/alerts/:id/review", h.MarkReviewed)
	}
}
