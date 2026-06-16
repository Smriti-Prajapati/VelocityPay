package refund

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes HTTP endpoints for the refund service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs a refund Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Request godoc
// POST /api/refunds
func (h *Handler) Request(c *gin.Context) {
	requesterID := middleware.MustGetUserID(c)

	var req RequestRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	rf, err := h.svc.Request(c.Request.Context(), requesterID, &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyRequested):
			response.Conflict(c, err.Error())
		case err.Error() == "only the sender can request a refund":
			response.Forbidden(c, err.Error())
		case err.Error() == "transaction not found":
			response.NotFound(c, err.Error())
		default:
			h.log.Error("refund request failed", zap.Error(err))
			response.BadRequest(c, err.Error())
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": rf})
}

// GetByID godoc
// GET /api/refunds/:id
func (h *Handler) GetByID(c *gin.Context) {
	requesterID := middleware.MustGetUserID(c)

	refundID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid refund id")
		return
	}

	resp, err := h.svc.GetByID(c.Request.Context(), requesterID, refundID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "refund not found")
			return
		}
		if err.Error() == "access denied" {
			response.Forbidden(c, "you do not own this refund")
			return
		}
		response.InternalError(c, "could not fetch refund")
		return
	}

	response.OK(c, resp)
}

// ListMine godoc
// GET /api/refunds
func (h *Handler) ListMine(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	refunds, err := h.svc.ListMine(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "could not fetch refunds")
		return
	}

	if refunds == nil {
		refunds = []*Refund{}
	}

	response.OK(c, gin.H{"refunds": refunds, "total": len(refunds)})
}

// Process godoc
// PUT /api/refunds/:id/process  (admin-only in production)
func (h *Handler) Process(c *gin.Context) {
	reviewerID := middleware.MustGetUserID(c)

	refundID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid refund id")
		return
	}

	var body struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note" validate:"omitempty,max=500"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	rf, err := h.svc.Process(c.Request.Context(), reviewerID, refundID, body.Approve, body.Note)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "refund not found")
			return
		}
		h.log.Error("process refund failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}

	response.OK(c, rf)
}

// RegisterRoutes mounts refund handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	refundGroup := api.Group("/refunds", authMiddleware)
	{
		refundGroup.POST("", h.Request)
		refundGroup.GET("", h.ListMine)
		refundGroup.GET("/:id", h.GetByID)
		refundGroup.PUT("/:id/process", h.Process)
	}
}
