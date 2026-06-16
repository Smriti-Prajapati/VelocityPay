package wallet

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes HTTP endpoints for wallet operations.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs the wallet Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Create godoc
// POST /api/wallet/create
func (h *Handler) Create(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	w, err := h.svc.Create(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			response.Conflict(c, "wallet already exists for this account")
			return
		}
		h.log.Error("create wallet failed", zap.Error(err))
		response.InternalError(c, "could not create wallet")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": w})
}

// AddMoney godoc
// POST /api/wallet/add-money
func (h *Handler) AddMoney(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req AddMoneyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	w, err := h.svc.AddMoney(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "wallet not found")
			return
		}
		h.log.Error("add money failed", zap.Error(err))
		response.InternalError(c, "could not add money")
		return
	}

	response.OK(c, w)
}

// GetBalance godoc
// GET /api/wallet/balance
func (h *Handler) GetBalance(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	w, err := h.svc.GetBalance(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "wallet not found — please create one first")
			return
		}
		response.InternalError(c, "could not fetch balance")
		return
	}

	response.OK(c, gin.H{
		"wallet_id":     w.ID,
		"balance":       w.Balance,
		"currency":      w.Currency,
		"wallet_number": w.WalletNumber,
	})
}

// GetDetails godoc
// GET /api/wallet/details
func (h *Handler) GetDetails(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	details, err := h.svc.GetDetails(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "wallet not found")
			return
		}
		response.InternalError(c, "could not fetch wallet details")
		return
	}

	response.OK(c, details)
}

// RegisterRoutes mounts wallet handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	walletGroup := api.Group("/wallet", authMiddleware)
	{
		walletGroup.POST("/create", h.Create)
		walletGroup.POST("/add-money", h.AddMoney)
		walletGroup.GET("/balance", h.GetBalance)
		walletGroup.GET("/details", h.GetDetails)
	}
}
