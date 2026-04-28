// Package api wires together the Fiber application, middleware, and route handlers.
package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/api/handlers"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/middleware"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Register mounts all API routes onto app.
// Both pool and rdb may be nil in Phase 1 (lenient boot).
func Register(app *fiber.App, pool *pgxpool.Pool, rdb *goredis.Client) {
	app.Get("/health", handlers.Health)
	app.Get("/ready", handlers.Ready(pool, rdb))
}

func RegisterV1(app *fiber.App, deps *handlers.ApiHandlerDeps) {
	v1 := app.Group("/api/v1")
	v1.Get("/health", handlers.Health)
	v1.Post("/auth/verify", middleware.NewAuthVerifyRateLimit(deps.Rdb, deps.Log), handlers.V1AuthVerify(deps))

	authed := v1.Group("", middleware.AuthAPIKey(deps.KeySvc, deps.AuditSvc, deps.Log))
	authed.Get("/me", handlers.V1Me(deps))
	authed.Get("/balance", handlers.V1Balance(deps))
	authed.Get("/transactions", handlers.V1Transactions(deps))
	authed.Get("/ledger", handlers.V1Ledger(deps))
	authed.Get("/wp-sites", handlers.V1WpSitesList(deps))
	authed.Post("/wp-sites", handlers.V1WpSitesCreate(deps))
	authed.Delete("/wp-sites/:id", handlers.V1WpSitesDelete(deps))
	authed.Post("/wp-sites/:id/revalidate", handlers.V1WpSitesRevalidate(deps))
}

// RegisterWebhook mounts the SePay webhook route with its middleware chain.
// Called from server.go after WebhookDeps are fully wired in main.go.
//
// Middleware order (outermost first):
//  1. NewRateLimitWebhook — [Q4] 20 req/sec/IP via Redis fixed-window; runs BEFORE auth.
//  2. SePayWebhook handler — auth, parse, CAS grant, notify goroutine.
//
// Body size cap: Fiber app-level BodyLimit (64KB) set in server.go covers this route.
func RegisterWebhook(app *fiber.App, deps *handlers.WebhookDeps, log *zap.Logger) {
	app.Post("/webhooks/sepay",
		middleware.NewRateLimitWebhook(deps.Rdb, 20, log),
		handlers.SePayWebhook(deps),
	)
}
