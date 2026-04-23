// Package redis provides a go-redis v9 client initialized from config.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

// NewClient parses REDIS_URL, creates a go-redis Client, and validates
// connectivity with a Ping call.
// Pool parameters are tuned for Fly.io Redis 256 MB shared instance:
//   - PoolSize 5   — conservative; each connection ~50 KB overhead
//   - MaxRetries 3 — tolerate transient Fly network blips
//
// Returns (nil, err) on URL parse failure or Ping failure.
func NewClient(cfg *config.Config) (*goredis.Client, error) {
	opts, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse url: %w", err)
	}

	opts.PoolSize = 5
	opts.MaxRetries = 3

	client := goredis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	return client, nil
}
