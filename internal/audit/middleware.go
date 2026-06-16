package audit

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FromContext extracts request metadata for audit log entries.
func FromContext(c *gin.Context) (ipAddress, userAgent string) {
	return c.ClientIP(), c.Request.UserAgent()
}

// LogFromHTTP builds a LogRequest enriched with HTTP context.
func LogFromHTTP(c *gin.Context, userID uuid.UUID, action Action, entityType, entityID string, metadata map[string]interface{}) LogRequest {
	ip, ua := FromContext(c)
	return LogRequest{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		IPAddress:  ip,
		UserAgent:  ua,
		Metadata:   metadata,
	}
}
