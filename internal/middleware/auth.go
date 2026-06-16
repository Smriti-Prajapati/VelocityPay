package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/auth"
	"github.com/velocitypay/velocitypay/pkg/response"
)

const contextKeyUserID = "user_id"
const contextKeyEmail  = "email"

// Authenticate validates the Bearer token and stores user context.
func Authenticate(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			response.Unauthorized(c, "authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			response.Unauthorized(c, "invalid authorization format")
			c.Abort()
			return
		}

		claims, err := tokens.Parse(parts[1])
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set(contextKeyUserID, claims.UserID)
		c.Set(contextKeyEmail, claims.Email)
		c.Next()
	}
}

// MustGetUserID extracts the authenticated user's UUID from context.
// It panics if the auth middleware was not applied — a programming error.
func MustGetUserID(c *gin.Context) uuid.UUID {
	v, exists := c.Get(contextKeyUserID)
	if !exists {
		panic("auth middleware not applied: user_id missing from context")
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		panic("user_id in context is not a uuid.UUID")
	}
	return id
}

// MustGetEmail extracts the authenticated user's email from context.
func MustGetEmail(c *gin.Context) string {
	v, _ := c.Get(contextKeyEmail)
	email, _ := v.(string)
	return email
}
