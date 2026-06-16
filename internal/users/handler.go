package users

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/middleware"
	"github.com/velocitypay/velocitypay/pkg/response"
	"go.uber.org/zap"
)

// Handler exposes HTTP handlers for the user service.
type Handler struct {
	svc *Service
	log *zap.Logger
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service, log *zap.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Register godoc
// POST /api/auth/register
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			response.Conflict(c, "email already in use")
			return
		}
		h.log.Error("register failed", zap.Error(err))
		response.InternalError(c, "registration failed")
		return
	}

	response.Created(c, resp)
}

// Login godoc
// POST /api/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) || err.Error() == "invalid credentials" {
			response.Unauthorized(c, "invalid email or password")
			return
		}
		h.log.Error("login failed", zap.Error(err))
		response.InternalError(c, "login failed")
		return
	}

	response.OK(c, resp)
}

// Logout godoc
// POST /api/auth/logout
// Since we use stateless JWTs, logout is handled client-side.
// In a production system you'd blacklist the token in Redis.
func (h *Handler) Logout(c *gin.Context) {
	response.OK(c, gin.H{"message": "logged out successfully"})
}

// GetProfile godoc
// GET /api/users/profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	user, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "could not fetch profile")
		return
	}

	response.OK(c, user)
}

// UpdateProfile godoc
// PUT /api/users/profile
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	user, err := h.svc.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "could not update profile")
		return
	}

	response.OK(c, user)
}

// ChangePassword godoc
// PUT /api/users/change-password
func (h *Handler) ChangePassword(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := validate.Struct(req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := h.svc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		if err.Error() == "current password is incorrect" {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "could not change password")
		return
	}

	response.OK(c, gin.H{"message": "password changed successfully"})
}

// RegisterRoutes mounts user handlers onto the router group.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	// Public auth routes
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/logout", authMiddleware, h.Logout)
	}

	// Protected user routes
	userGroup := api.Group("/users", authMiddleware)
	{
		userGroup.GET("/profile", h.GetProfile)
		userGroup.PUT("/profile", h.UpdateProfile)
		userGroup.PUT("/change-password", h.ChangePassword)
	}
}

// GetUserIDFromPath is a helper for admin routes that read :id from the URL.
func getUserIDFromPath(c *gin.Context) (uuid.UUID, bool) {
	raw := c.Param("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return uuid.Nil, false
	}
	return id, true
}
