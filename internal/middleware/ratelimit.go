package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"github.com/velocitypay/velocitypay/pkg/response"
)

// RateLimit limits requests per IP to maxRequests within window duration.
func RateLimit(cache *redisc.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("ratelimit:%s", c.ClientIP())
		ctx := context.Background()

		count, err := cache.Incr(ctx, key)
		if err != nil {
			// Fail open — don't block requests when Redis is unavailable.
			c.Next()
			return
		}

		if count == 1 {
			_ = cache.Expire(ctx, key, window)
		}

		if count > int64(maxRequests) {
			response.TooManyRequests(c, "rate limit exceeded")
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", maxRequests-int(count)))
		c.Next()
	}
}
