// Package handlers contains HTTP handler functions for the API service.
package handlers

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// healthResponse is the JSON body for GET /health.
type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// readyResponse is the JSON body for GET /ready.
type readyResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
	Redis  string `json:"redis"`
}

// Health handles GET /health.
// Always returns 200 — no external dependencies; used by load-balancers to
// confirm the process is alive.
func Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(healthResponse{
		Status:  "ok",
		Version: "0.1.0",
	})
}

// Ready returns a factory handler for GET /ready.
// It probes both Postgres (pool) and Redis (rdb) with a 2-second timeout each.
// Both probes are nil-safe — a nil pool/rdb is treated as unavailable (503)
// without panicking. This matches the Phase 1 lenient-boot contract (C2 patch):
// the binary starts even if DB/Redis are not yet reachable.
func Ready(pool *pgxpool.Pool, rdb *goredis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		dbStatus := probeDB(pool)
		redisStatus := probeRedis(rdb)

		status := fiber.StatusOK
		overallStatus := "ready"

		if dbStatus != "ok" || redisStatus != "ok" {
			status = fiber.StatusServiceUnavailable
			overallStatus = "degraded"
		}

		return c.Status(status).JSON(readyResponse{
			Status: overallStatus,
			DB:     dbStatus,
			Redis:  redisStatus,
		})
	}
}

// probeDB pings Postgres with a 2-second deadline.
// Returns "ok" on success, "err" on failure or nil pool.
func probeDB(pool *pgxpool.Pool) string {
	if pool == nil {
		return "err"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		return "err"
	}
	return "ok"
}

// probeRedis pings Redis with a 2-second deadline.
// Returns "ok" on success, "err" on failure or nil client.
func probeRedis(rdb *goredis.Client) string {
	if rdb == nil {
		return "err"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return "err"
	}
	return "ok"
}
