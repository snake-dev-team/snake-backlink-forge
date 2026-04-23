// Package db provides pgxpool initialization for the API service.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
)

// NewPool creates and validates a pgxpool.Pool using the provided config.
// Pool parameters are tuned for Fly.io shared-cpu-1x (1 GB RAM):
//   - MaxConns 20 — ~5–10 MB overhead per connection, stays within budget
//   - MinConns 2  — keep two connections warm to avoid cold-start latency
//   - MaxConnLifetime 30m — recycle before Fly's 60 min idle-close
//   - MaxConnIdleTime 5m  — free idle connections quickly on low traffic
//
// Returns a non-nil error if the connection string is malformed or the initial
// ping fails. Callers may treat a nil pool as "DB unavailable" (Phase 1 lenient
// boot) as long as they guard every pool usage with a nil-check.
func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	poolCfg.MaxConns = 20
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	return pool, nil
}
