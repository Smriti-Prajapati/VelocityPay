package analytics

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes analytics endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs an analytics Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Dashboard godoc
// GET /api/analytics/dashboard
func (h *Handler) Dashboard(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	summary, err := h.svc.GetDashboard(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("dashboard failed", zap.Error(err))
		response.InternalError(c, "could not load dashboard")
		return
	}

	response.OK(c, summary)
}

// UserStats godoc
// GET /api/analytics/stats
func (h *Handler) UserStats(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	stats, err := h.svc.GetUserStats(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "could not load stats")
		return
	}

	response.OK(c, stats)
}

// MonthlySpend godoc
// GET /api/analytics/monthly?months=6
func (h *Handler) MonthlySpend(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	months := queryInt(c, "months", 6)

	data, err := h.svc.GetMonthlySpend(c.Request.Context(), userID, months)
	if err != nil {
		response.InternalError(c, "could not load monthly spend")
		return
	}

	response.OK(c, gin.H{"data": data, "months": months})
}

// DailyVolume godoc
// GET /api/analytics/daily?days=30
func (h *Handler) DailyVolume(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	days := queryInt(c, "days", 30)

	data, err := h.svc.GetDailyVolume(c.Request.Context(), userID, days)
	if err != nil {
		response.InternalError(c, "could not load daily volume")
		return
	}

	response.OK(c, gin.H{"data": data, "days": days})
}

// PlatformStats godoc
// GET /api/analytics/platform
func (h *Handler) PlatformStats(c *gin.Context) {
	stats, err := h.svc.GetPlatformStats(c.Request.Context())
	if err != nil {
		h.log.Error("platform stats failed", zap.Error(err))
		response.InternalError(c, "could not load platform stats")
		return
	}

	response.OK(c, stats)
}

// TopSenders godoc
// GET /api/analytics/top-senders?limit=10
func (h *Handler) TopSenders(c *gin.Context) {
	limit := queryInt(c, "limit", 10)

	senders, err := h.svc.GetTopSenders(c.Request.Context(), limit)
	if err != nil {
		response.InternalError(c, "could not load top senders")
		return
	}

	response.OK(c, gin.H{"data": senders, "total": len(senders)})
}

// WalletGrowth godoc
// GET /api/analytics/wallet-growth?days=30
func (h *Handler) WalletGrowth(c *gin.Context) {
	days := queryInt(c, "days", 30)

	data, err := h.svc.GetWalletGrowth(c.Request.Context(), days)
	if err != nil {
		response.InternalError(c, "could not load wallet growth")
		return
	}

	response.OK(c, gin.H{"data": data, "days": days})
}

// RegisterRoutes mounts analytics handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	ag := api.Group("/analytics", authMiddleware)
	{
		ag.GET("/dashboard", h.Dashboard)
		ag.GET("/stats", h.UserStats)
		ag.GET("/monthly", h.MonthlySpend)
		ag.GET("/daily", h.DailyVolume)
		ag.GET("/platform", h.PlatformStats)
		ag.GET("/top-senders", h.TopSenders)
		ag.GET("/wallet-growth", h.WalletGrowth)
	}
}

// queryInt reads an integer query param with a fallback default.
func queryInt(c *gin.Context, key string, defaultVal int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultVal
	}
	return v
}
