package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/velocitypay/velocitypay/internal/config"
	"go.uber.org/zap"
)

// Client wraps the Redis client with helper methods.
type Client struct {
	rdb *redis.Client
	log *zap.Logger
}

// NewClient creates and verifies a Redis connection.
// TLS is enabled automatically when addr contains "upstash.io"
// or when REDIS_TLS=true is explicitly set.
func NewClient(cfg *config.RedisConfig, log *zap.Logger) (*Client, error) {
	useTLS := cfg.TLS || strings.Contains(cfg.Addr, "upstash.io")

	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	if useTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		log.Info("Redis TLS enabled", zap.String("addr", cfg.Addr))
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info("connected to Redis", zap.String("addr", cfg.Addr))
	return &Client{rdb: rdb, log: log}, nil
}

// Set stores a key-value pair with an optional TTL.
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value by key. Returns redis.Nil if not found.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Del removes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists checks whether a key exists.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// Incr atomically increments a key and returns the new value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

// Expire sets a TTL on an existing key.
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// Close terminates the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Raw exposes the underlying redis.Client for advanced use cases.
func (c *Client) Raw() *redis.Client {
	return c.rdb
}
