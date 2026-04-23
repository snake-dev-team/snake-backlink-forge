// Package api wires together the Fiber application, middleware, and route handlers.
package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	goredis "github.com/redis/go-redis/v9"
)

// Register mounts all API routes onto app.
// Both pool and rdb may be nil in Phase 1 (lenient boot).
func Register(app *fiber.App, pool *pgxpool.Pool, rdb *goredis.Client) {
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready(pool, rdb))
}
