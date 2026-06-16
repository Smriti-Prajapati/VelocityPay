package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes audit log endpoints.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs an audit Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// MyLogs godoc
// GET /api/audit/me
func (h *Handler) MyLogs(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var filter ListFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.GetMyLogs(c.Request.Context(), userID, filter)
	if err != nil {
		h.log.Error("get my audit logs failed", zap.Error(err))
		response.InternalError(c, "could not fetch audit logs")
		return
	}

	response.OK(c, resp)
}

// AllLogs godoc
// GET /api/audit/all  (admin)
func (h *Handler) AllLogs(c *gin.Context) {
	var filter ListFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.svc.GetAllLogs(c.Request.Context(), filter)
	if err != nil {
		h.log.Error("get all audit logs failed", zap.Error(err))
		response.InternalError(c, "could not fetch audit logs")
		return
	}

	response.OK(c, resp)
}

// RegisterRoutes mounts audit handlers.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	auditGroup := api.Group("/audit", authMiddleware)
	{
		auditGroup.GET("/me", h.MyLogs)
		auditGroup.GET("/all", h.AllLogs)
	}
}
