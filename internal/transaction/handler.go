package transaction

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/internal/wallet"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes HTTP endpoints for the transaction service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs a transaction Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Transfer godoc
// POST /api/transactions/transfer
func (h *Handler) Transfer(c *gin.Context) {
	senderID := middleware.MustGetUserID(c)

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	txn, err := h.svc.Transfer(c.Request.Context(), senderID, &req)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInsufficientBalance):
			response.UnprocessableEntity(c, "insufficient wallet balance")
		case errors.Is(err, wallet.ErrNotFound):
			response.NotFound(c, "wallet not found")
		case err.Error() == "cannot transfer to your own wallet":
			response.BadRequest(c, err.Error())
		case err.Error() == "transaction queue full, please retry shortly":
			response.TooManyRequests(c, err.Error())
		default:
			h.log.Error("transfer failed", zap.Error(err), zap.String("sender_id", senderID.String()))
			response.InternalError(c, "transfer could not be processed")
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": txn})
}

// GetHistory godoc
// GET /api/transactions/history
func (h *Handler) GetHistory(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var filter HistoryFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(filter); err != nil {
		response.ValidationError(c, err)
		return
	}

	resp, err := h.svc.GetHistory(c.Request.Context(), userID, filter)
	if err != nil {
		h.log.Error("get history failed", zap.Error(err))
		response.InternalError(c, "could not fetch transaction history")
		return
	}

	response.OK(c, resp)
}

// GetByID godoc
// GET /api/transactions/:id
func (h *Handler) GetByID(c *gin.Context) {
	requesterID := middleware.MustGetUserID(c)

	txnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid transaction id")
		return
	}

	txn, err := h.svc.GetByID(c.Request.Context(), requesterID, txnID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "transaction not found")
			return
		}
		if err.Error() == "access denied" {
			response.Forbidden(c, "you are not a party to this transaction")
			return
		}
		response.InternalError(c, "could not fetch transaction")
		return
	}

	response.OK(c, txn)
}

// RegisterRoutes mounts transaction handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	txGroup := api.Group("/transactions", authMiddleware)
	{
		txGroup.POST("/transfer", h.Transfer)
		txGroup.GET("/history", h.GetHistory)
		txGroup.GET("/:id", h.GetByID)
	}
}
